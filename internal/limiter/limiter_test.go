package limiter_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wellbrito29/fullcycle-rate-limiter/internal/limiter"
	"github.com/Wellbrito29/fullcycle-rate-limiter/internal/storage/memory"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newLimiter(t *testing.T, opts limiter.Options) (*limiter.Limiter, *fakeClock) {
	t.Helper()

	clock := &fakeClock{now: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
	storage := memory.New().WithClock(clock.Now)

	l, err := limiter.New(storage, opts)
	if err != nil {
		t.Fatalf("erro inesperado ao criar limiter: %v", err)
	}
	return l, clock
}

func allow(t *testing.T, l *limiter.Limiter, req limiter.Request) limiter.Result {
	t.Helper()

	result, err := l.Allow(context.Background(), req)
	if err != nil {
		t.Fatalf("erro inesperado em Allow: %v", err)
	}
	return result
}

func TestAllowRecusaDepoisDeAtingirLimitePorIP(t *testing.T) {
	l, _ := newLimiter(t, limiter.Options{
		IP: limiter.Policy{Limit: 3, BlockDuration: time.Minute},
	})

	for i := 1; i <= 3; i++ {
		result := allow(t, l, limiter.Request{IP: "10.0.0.1"})
		if !result.Allowed {
			t.Fatalf("requisicao %d deveria ser permitida", i)
		}
		if result.Kind != limiter.KindIP {
			t.Fatalf("esperava limitacao por ip, obteve %q", result.Kind)
		}
		if want := 3 - i; result.Remaining != want {
			t.Fatalf("remaining na requisicao %d: esperava %d, obteve %d", i, want, result.Remaining)
		}
	}

	result := allow(t, l, limiter.Request{IP: "10.0.0.1"})
	if result.Allowed || !result.Blocked {
		t.Fatalf("quarta requisicao deveria ser bloqueada, obteve %+v", result)
	}
}

func TestAllowIsolaChavesDiferentes(t *testing.T) {
	l, _ := newLimiter(t, limiter.Options{
		IP: limiter.Policy{Limit: 1, BlockDuration: time.Minute},
	})

	allow(t, l, limiter.Request{IP: "10.0.0.1"})
	if result := allow(t, l, limiter.Request{IP: "10.0.0.1"}); result.Allowed {
		t.Fatal("segunda requisicao do mesmo ip deveria ser bloqueada")
	}
	if result := allow(t, l, limiter.Request{IP: "10.0.0.2"}); !result.Allowed {
		t.Fatal("outro ip nao deveria herdar o bloqueio")
	}
}

func TestAllowMantemBloqueioAteOFimDaPenalidade(t *testing.T) {
	l, clock := newLimiter(t, limiter.Options{
		IP: limiter.Policy{Limit: 1, BlockDuration: 5 * time.Minute},
	})

	allow(t, l, limiter.Request{IP: "10.0.0.1"})
	allow(t, l, limiter.Request{IP: "10.0.0.1"})

	clock.Advance(4 * time.Minute)
	if result := allow(t, l, limiter.Request{IP: "10.0.0.1"}); result.Allowed {
		t.Fatal("ip deveria continuar bloqueado antes do fim da penalidade")
	}

	clock.Advance(2 * time.Minute)
	if result := allow(t, l, limiter.Request{IP: "10.0.0.1"}); !result.Allowed {
		t.Fatal("ip deveria ser liberado depois do fim da penalidade")
	}
}

func TestAllowReiniciaContagemAFimDaJanela(t *testing.T) {
	l, clock := newLimiter(t, limiter.Options{
		Window: time.Second,
		IP:     limiter.Policy{Limit: 2, BlockDuration: time.Minute},
	})

	allow(t, l, limiter.Request{IP: "10.0.0.1"})
	allow(t, l, limiter.Request{IP: "10.0.0.1"})

	clock.Advance(1100 * time.Millisecond)

	if result := allow(t, l, limiter.Request{IP: "10.0.0.1"}); !result.Allowed {
		t.Fatal("nova janela deveria permitir requisicoes novamente")
	}
}

func TestAllowTokenTemPrecedenciaSobreIP(t *testing.T) {
	l, _ := newLimiter(t, limiter.Options{
		IP:           limiter.Policy{Limit: 2, BlockDuration: time.Minute},
		TokenDefault: limiter.Policy{Limit: 5, BlockDuration: time.Minute},
	})

	for i := 1; i <= 5; i++ {
		result := allow(t, l, limiter.Request{IP: "10.0.0.1", Token: "abc123"})
		if !result.Allowed {
			t.Fatalf("requisicao %d com token deveria ser permitida, limite do ip nao se aplica", i)
		}
		if result.Kind != limiter.KindToken {
			t.Fatalf("esperava limitacao por token, obteve %q", result.Kind)
		}
	}

	if result := allow(t, l, limiter.Request{IP: "10.0.0.1", Token: "abc123"}); result.Allowed {
		t.Fatal("sexta requisicao deveria estourar o limite do token")
	}
}

func TestAllowUsaLimiteEspecificoDoToken(t *testing.T) {
	l, _ := newLimiter(t, limiter.Options{
		IP:           limiter.Policy{Limit: 10, BlockDuration: time.Minute},
		TokenDefault: limiter.Policy{Limit: 2, BlockDuration: time.Minute},
		Tokens: map[string]limiter.Policy{
			"premium": {Limit: 4, BlockDuration: time.Minute},
		},
	})

	for i := 1; i <= 4; i++ {
		if result := allow(t, l, limiter.Request{IP: "10.0.0.1", Token: "premium"}); !result.Allowed {
			t.Fatalf("requisicao %d do token premium deveria ser permitida", i)
		}
	}
	if result := allow(t, l, limiter.Request{IP: "10.0.0.1", Token: "premium"}); result.Allowed {
		t.Fatal("token premium deveria estourar no limite proprio")
	}

	if result := allow(t, l, limiter.Request{IP: "10.0.0.1", Token: "comum"}); !result.Allowed {
		t.Fatal("token sem configuracao propria deveria usar o limite padrao")
	}
}

func TestAllowBloqueioDeTokenNaoAfetaOMesmoIP(t *testing.T) {
	l, _ := newLimiter(t, limiter.Options{
		IP:           limiter.Policy{Limit: 3, BlockDuration: time.Minute},
		TokenDefault: limiter.Policy{Limit: 1, BlockDuration: time.Minute},
	})

	allow(t, l, limiter.Request{IP: "10.0.0.1", Token: "abc123"})
	if result := allow(t, l, limiter.Request{IP: "10.0.0.1", Token: "abc123"}); result.Allowed {
		t.Fatal("token deveria estar bloqueado")
	}
	if result := allow(t, l, limiter.Request{IP: "10.0.0.1"}); !result.Allowed {
		t.Fatal("requisicao sem token deveria seguir a regra de ip")
	}
}

func TestNewSemStorage(t *testing.T) {
	if _, err := limiter.New(nil, limiter.Options{}); err == nil {
		t.Fatal("esperava erro ao criar limiter sem storage")
	}
}

func TestAllowConcorrenteRespeitaLimite(t *testing.T) {
	l, _ := newLimiter(t, limiter.Options{
		IP: limiter.Policy{Limit: 20, BlockDuration: time.Minute},
	})

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		allowed int
	)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := l.Allow(context.Background(), limiter.Request{IP: "10.0.0.1"})
			if err != nil {
				t.Errorf("erro inesperado: %v", err)
				return
			}
			if result.Allowed {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if allowed != 20 {
		t.Fatalf("esperava 20 requisicoes permitidas, obteve %d", allowed)
	}
}
