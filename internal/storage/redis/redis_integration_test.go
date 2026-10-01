//go:build integration

package redis_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Wellbrito29/fullcycle-rate-limiter/internal/limiter"
	redisstorage "github.com/Wellbrito29/fullcycle-rate-limiter/internal/storage/redis"
)

func newStorage(t *testing.T) *redisstorage.Storage {
	t.Helper()

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	storage := redisstorage.New(redisstorage.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := storage.Ping(ctx); err != nil {
		t.Fatalf("redis indisponivel em %s: %v", addr, err)
	}

	t.Cleanup(func() { _ = storage.Close() })
	return storage
}

// Cada execucao usa chaves novas para nao herdar contadores de rodadas anteriores.
func uniqueKey(prefix string) string {
	return fmt.Sprintf("%s:%d", prefix, time.Now().UnixNano())
}

func TestRedisStorageContaDentroDaJanela(t *testing.T) {
	storage := newStorage(t)
	ctx := context.Background()
	key := uniqueKey(t.Name())

	for i := int64(1); i <= 3; i++ {
		total, err := storage.Hit(ctx, key, 2*time.Second)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if total != i {
			t.Fatalf("esperava total %d, obteve %d", i, total)
		}
	}

	time.Sleep(2100 * time.Millisecond)

	total, err := storage.Hit(ctx, key, time.Second)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if total != 1 {
		t.Fatalf("contagem deveria reiniciar na nova janela, obteve %d", total)
	}
}

func TestRedisStorageBloqueiaPorTempoDeterminado(t *testing.T) {
	storage := newStorage(t)
	ctx := context.Background()
	key := uniqueKey(t.Name())

	if err := storage.Block(ctx, key, 1500*time.Millisecond); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	blocked, err := storage.IsBlocked(ctx, key)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !blocked {
		t.Fatal("chave deveria estar bloqueada")
	}

	time.Sleep(1600 * time.Millisecond)

	blocked, err = storage.IsBlocked(ctx, key)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if blocked {
		t.Fatal("bloqueio deveria ter expirado")
	}
}

func TestLimiterComRedisAplicaPrecedenciaDeToken(t *testing.T) {
	storage := newStorage(t)
	ctx := context.Background()

	l, err := limiter.New(storage, limiter.Options{
		Window:       2 * time.Second,
		IP:           limiter.Policy{Limit: 1, BlockDuration: time.Second},
		TokenDefault: limiter.Policy{Limit: 3, BlockDuration: time.Second},
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	ip := uniqueKey("ip")
	token := uniqueKey("token")

	if result, _ := l.Allow(ctx, limiter.Request{IP: ip}); !result.Allowed {
		t.Fatal("primeira requisicao por ip deveria passar")
	}
	if result, _ := l.Allow(ctx, limiter.Request{IP: ip}); result.Allowed {
		t.Fatal("segunda requisicao por ip deveria ser bloqueada")
	}

	for i := 1; i <= 3; i++ {
		result, err := l.Allow(ctx, limiter.Request{IP: ip, Token: token})
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if !result.Allowed {
			t.Fatalf("requisicao %d com token deveria passar mesmo com o ip bloqueado", i)
		}
	}
	if result, _ := l.Allow(ctx, limiter.Request{IP: ip, Token: token}); result.Allowed {
		t.Fatal("quarta requisicao deveria estourar o limite do token")
	}
}
