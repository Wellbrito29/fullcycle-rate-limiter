package config_test

import (
	"testing"
	"time"

	"github.com/Wellbrito29/fullcycle-rate-limiter/internal/config"
)

func TestLoadUsaValoresPadraoQuandoNaoHaEnv(t *testing.T) {
	cfg, err := config.Load("testdata/vazio.env")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if cfg.ServerPort != "8080" {
		t.Fatalf("esperava porta 8080, obteve %q", cfg.ServerPort)
	}
	if cfg.IPLimit != 10 {
		t.Fatalf("esperava limite de ip 10, obteve %d", cfg.IPLimit)
	}
	if cfg.IPBlockDuration != 5*time.Minute {
		t.Fatalf("esperava bloqueio de 5m, obteve %s", cfg.IPBlockDuration)
	}
}

func TestLoadLeEnvEOverridesDeToken(t *testing.T) {
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("RATE_LIMIT_IP_MAX_REQUESTS", "5")
	t.Setenv("RATE_LIMIT_IP_BLOCK_DURATION", "30")
	t.Setenv("RATE_LIMIT_TOKEN_MAX_REQUESTS", "50")
	t.Setenv("RATE_LIMIT_TOKEN_BLOCK_DURATION", "2m")
	t.Setenv("RATE_LIMIT_TOKENS", "abc123:100, premium:1000:1m")

	cfg, err := config.Load("testdata/vazio.env")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if cfg.ServerPort != "9090" {
		t.Fatalf("esperava porta 9090, obteve %q", cfg.ServerPort)
	}
	if cfg.IPLimit != 5 || cfg.IPBlockDuration != 30*time.Second {
		t.Fatalf("configuracao de ip inesperada: %d / %s", cfg.IPLimit, cfg.IPBlockDuration)
	}
	if cfg.TokenLimit != 50 || cfg.TokenBlockDuration != 2*time.Minute {
		t.Fatalf("configuracao de token inesperada: %d / %s", cfg.TokenLimit, cfg.TokenBlockDuration)
	}

	abc, ok := cfg.TokenOverrides["abc123"]
	if !ok {
		t.Fatal("esperava override para abc123")
	}
	if abc.Limit != 100 || abc.BlockDuration != 2*time.Minute {
		t.Fatalf("override de abc123 inesperado: %+v", abc)
	}

	premium, ok := cfg.TokenOverrides["premium"]
	if !ok {
		t.Fatal("esperava override para premium")
	}
	if premium.Limit != 1000 || premium.BlockDuration != time.Minute {
		t.Fatalf("override de premium inesperado: %+v", premium)
	}
}

func TestLoadRecusaConfiguracaoInvalida(t *testing.T) {
	tests := map[string]map[string]string{
		"limite de ip zerado":      {"RATE_LIMIT_IP_MAX_REQUESTS": "0"},
		"limite de ip nao numero":  {"RATE_LIMIT_IP_MAX_REQUESTS": "dez"},
		"bloqueio invalido":        {"RATE_LIMIT_IP_BLOCK_DURATION": "cinco minutos"},
		"token sem limite":         {"RATE_LIMIT_TOKENS": "abc123"},
		"token com limite zerado":  {"RATE_LIMIT_TOKENS": "abc123:0"},
		"token com campos demais":  {"RATE_LIMIT_TOKENS": "abc123:10:1m:extra"},
		"token com nome em branco": {"RATE_LIMIT_TOKENS": ":10"},
	}

	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			for key, value := range env {
				t.Setenv(key, value)
			}
			if _, err := config.Load("testdata/vazio.env"); err == nil {
				t.Fatal("esperava erro de configuracao")
			}
		})
	}
}
