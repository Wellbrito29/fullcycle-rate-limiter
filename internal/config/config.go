package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	IPLimit         int
	IPBlockDuration time.Duration

	TokenLimit         int
	TokenBlockDuration time.Duration

	TokenOverrides map[string]TokenPolicy
}

// TokenPolicy guarda o limite individual de um token, quando ele existe.
type TokenPolicy struct {
	Limit         int
	BlockDuration time.Duration
}

func Load(envFiles ...string) (*Config, error) {
	if len(envFiles) == 0 {
		envFiles = []string{".env"}
	}
	_ = godotenv.Load(envFiles...)

	cfg := &Config{
		ServerPort:    getString("SERVER_PORT", "8080"),
		RedisAddr:     getString("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getString("REDIS_PASSWORD", ""),
	}

	var err error
	if cfg.RedisDB, err = getInt("REDIS_DB", 0, 0); err != nil {
		return nil, err
	}
	if cfg.IPLimit, err = getInt("RATE_LIMIT_IP_MAX_REQUESTS", 10, 1); err != nil {
		return nil, err
	}
	if cfg.IPBlockDuration, err = getDuration("RATE_LIMIT_IP_BLOCK_DURATION", 5*time.Minute); err != nil {
		return nil, err
	}
	if cfg.TokenLimit, err = getInt("RATE_LIMIT_TOKEN_MAX_REQUESTS", 100, 1); err != nil {
		return nil, err
	}
	if cfg.TokenBlockDuration, err = getDuration("RATE_LIMIT_TOKEN_BLOCK_DURATION", 5*time.Minute); err != nil {
		return nil, err
	}
	if cfg.TokenOverrides, err = parseTokenOverrides(os.Getenv("RATE_LIMIT_TOKENS"), cfg.TokenBlockDuration); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getString(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback, min int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s invalido: %w", key, err)
	}
	if v < min {
		return 0, fmt.Errorf("%s deve ser maior ou igual a %d", key, min)
	}
	return v, nil
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		return time.Duration(seconds) * time.Second, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s invalido: %w", key, err)
	}
	return v, nil
}

// Formato aceito: token:limite[:bloqueio],token:limite[:bloqueio]
func parseTokenOverrides(raw string, defaultBlock time.Duration) (map[string]TokenPolicy, error) {
	overrides := make(map[string]TokenPolicy)
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		parts := strings.Split(entry, ":")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, fmt.Errorf("RATE_LIMIT_TOKENS invalido em %q", entry)
		}

		token := strings.TrimSpace(parts[0])
		if token == "" {
			return nil, fmt.Errorf("RATE_LIMIT_TOKENS com token vazio em %q", entry)
		}

		limit, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || limit <= 0 {
			return nil, fmt.Errorf("RATE_LIMIT_TOKENS com limite invalido em %q", entry)
		}

		policy := TokenPolicy{Limit: limit, BlockDuration: defaultBlock}
		if len(parts) == 3 {
			if policy.BlockDuration, err = parseDurationValue(strings.TrimSpace(parts[2])); err != nil {
				return nil, fmt.Errorf("RATE_LIMIT_TOKENS com bloqueio invalido em %q", entry)
			}
		}

		overrides[token] = policy
	}
	return overrides, nil
}

func parseDurationValue(raw string) (time.Duration, error) {
	if seconds, err := strconv.Atoi(raw); err == nil {
		return time.Duration(seconds) * time.Second, nil
	}
	return time.ParseDuration(raw)
}
