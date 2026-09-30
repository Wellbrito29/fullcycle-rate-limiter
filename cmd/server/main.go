package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/Wellbrito29/fullcycle-rate-limiter/internal/config"
	"github.com/Wellbrito29/fullcycle-rate-limiter/internal/limiter"
	"github.com/Wellbrito29/fullcycle-rate-limiter/internal/middleware"
	redisstorage "github.com/Wellbrito29/fullcycle-rate-limiter/internal/storage/redis"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuracao invalida: %v", err)
	}

	storage := redisstorage.New(redisstorage.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer storage.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := storage.Ping(ctx); err != nil {
		log.Fatalf("nao foi possivel conectar no redis: %v", err)
	}

	rateLimiter, err := limiter.New(storage, limiter.Options{
		Window:       limiter.DefaultWindow,
		IP:           limiter.Policy{Limit: cfg.IPLimit, BlockDuration: cfg.IPBlockDuration},
		TokenDefault: limiter.Policy{Limit: cfg.TokenLimit, BlockDuration: cfg.TokenBlockDuration},
		Tokens:       tokenPolicies(cfg),
	})
	if err != nil {
		log.Fatalf("nao foi possivel iniciar o limiter: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	server := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           middleware.RateLimit(rateLimiter)(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("servidor ouvindo na porta %s (ip=%d req/s, token=%d req/s)", cfg.ServerPort, cfg.IPLimit, cfg.TokenLimit)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("servidor encerrado: %v", err)
	}
}

func tokenPolicies(cfg *config.Config) map[string]limiter.Policy {
	policies := make(map[string]limiter.Policy, len(cfg.TokenOverrides))
	for token, override := range cfg.TokenOverrides {
		policies[token] = limiter.Policy{Limit: override.Limit, BlockDuration: override.BlockDuration}
	}
	return policies
}
