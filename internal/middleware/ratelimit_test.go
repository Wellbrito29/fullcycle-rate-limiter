package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wellbrito29/fullcycle-rate-limiter/internal/limiter"
	"github.com/Wellbrito29/fullcycle-rate-limiter/internal/middleware"
	"github.com/Wellbrito29/fullcycle-rate-limiter/internal/storage/memory"
)

func newHandler(t *testing.T, opts limiter.Options) http.Handler {
	t.Helper()

	l, err := limiter.New(memory.New(), opts)
	if err != nil {
		t.Fatalf("erro inesperado ao criar limiter: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return middleware.RateLimit(l)(next)
}

func do(handler http.Handler, ip, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ip + ":54321"
	if token != "" {
		req.Header.Set(middleware.TokenHeader, token)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestMiddlewareRetorna429ComMensagemEsperada(t *testing.T) {
	handler := newHandler(t, limiter.Options{
		IP: limiter.Policy{Limit: 1, BlockDuration: time.Minute},
	})

	if rec := do(handler, "192.168.0.10", ""); rec.Code != http.StatusOK {
		t.Fatalf("primeira requisicao: esperava 200, obteve %d", rec.Code)
	}

	rec := do(handler, "192.168.0.10", "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("esperava 429, obteve %d", rec.Code)
	}
	if body := rec.Body.String(); body != middleware.BlockedBody {
		t.Fatalf("corpo inesperado: %q", body)
	}
}

func TestMiddlewareUsaTokenDoHeaderAPIKey(t *testing.T) {
	handler := newHandler(t, limiter.Options{
		IP:           limiter.Policy{Limit: 1, BlockDuration: time.Minute},
		TokenDefault: limiter.Policy{Limit: 3, BlockDuration: time.Minute},
	})

	for i := 1; i <= 3; i++ {
		rec := do(handler, "192.168.0.11", "abc123")
		if rec.Code != http.StatusOK {
			t.Fatalf("requisicao %d com token: esperava 200, obteve %d", i, rec.Code)
		}
		if got := rec.Header().Get("X-RateLimit-Policy"); got != string(limiter.KindToken) {
			t.Fatalf("esperava policy token, obteve %q", got)
		}
	}

	if rec := do(handler, "192.168.0.11", "abc123"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("esperava 429 ao estourar o limite do token, obteve %d", rec.Code)
	}
}

func TestMiddlewarePrecedenciaTokenSobreIPBloqueado(t *testing.T) {
	handler := newHandler(t, limiter.Options{
		IP:           limiter.Policy{Limit: 1, BlockDuration: time.Minute},
		TokenDefault: limiter.Policy{Limit: 5, BlockDuration: time.Minute},
	})

	do(handler, "192.168.0.12", "")
	if rec := do(handler, "192.168.0.12", ""); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ip deveria estar bloqueado, obteve %d", rec.Code)
	}

	rec := do(handler, "192.168.0.12", "abc123")
	if rec.Code != http.StatusOK {
		t.Fatalf("mesmo ip com token deveria passar, obteve %d", rec.Code)
	}
}

func TestMiddlewareUsaXForwardedFor(t *testing.T) {
	handler := newHandler(t, limiter.Options{
		IP: limiter.Policy{Limit: 1, BlockDuration: time.Minute},
	})

	first := httptest.NewRequest(http.MethodGet, "/", nil)
	first.RemoteAddr = "10.1.1.1:1111"
	first.Header.Set("X-Forwarded-For", "203.0.113.5, 10.1.1.1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, first)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d", rec.Code)
	}

	second := httptest.NewRequest(http.MethodGet, "/", nil)
	second.RemoteAddr = "10.2.2.2:2222"
	second.Header.Set("X-Forwarded-For", "203.0.113.5")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, second)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("esperava 429 para o mesmo ip repassado, obteve %d", rec.Code)
	}
}

func TestMiddlewarePublicaHeadersDeLimite(t *testing.T) {
	handler := newHandler(t, limiter.Options{
		IP: limiter.Policy{Limit: 2, BlockDuration: time.Minute},
	})

	rec := do(handler, "192.168.0.13", "")
	if got := rec.Header().Get("X-RateLimit-Limit"); got != "2" {
		t.Fatalf("esperava limite 2, obteve %q", got)
	}
	if got := rec.Header().Get("X-RateLimit-Remaining"); got != "1" {
		t.Fatalf("esperava remaining 1, obteve %q", got)
	}
}
