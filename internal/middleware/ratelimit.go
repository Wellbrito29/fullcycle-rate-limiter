package middleware

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wellbrito29/fullcycle-rate-limiter/internal/limiter"
)

const (
	TokenHeader  = "API_KEY"
	BlockedBody  = "you have reached the maximum number of requests or actions allowed within a certain time frame"
	headerLimit  = "X-RateLimit-Limit"
	headerLeft   = "X-RateLimit-Remaining"
	headerPolicy = "X-RateLimit-Policy"
)

func RateLimit(l *limiter.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req := limiter.Request{
				IP:    clientIP(r),
				Token: strings.TrimSpace(r.Header.Get(TokenHeader)),
			}

			result, err := l.Allow(r.Context(), req)
			if err != nil {
				log.Printf("rate limiter indisponivel: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			w.Header().Set(headerPolicy, string(result.Kind))
			w.Header().Set(headerLimit, strconv.Itoa(result.Limit))

			if !result.Allowed {
				w.Header().Set(headerLeft, "0")
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(BlockedBody))
				return
			}

			w.Header().Set(headerLeft, strconv.Itoa(result.Remaining))
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if first := strings.TrimSpace(strings.Split(forwarded, ",")[0]); first != "" {
			return first
		}
	}
	if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); real != "" {
		return real
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
