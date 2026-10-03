package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	counterPrefix = "ratelimit:counter:"
	blockPrefix   = "ratelimit:block:"
)

type Options struct {
	Addr     string
	Password string
	DB       int
}

type Storage struct {
	client *redis.Client
}

func New(opts Options) *Storage {
	return NewWithClient(redis.NewClient(&redis.Options{
		Addr:     opts.Addr,
		Password: opts.Password,
		DB:       opts.DB,
	}))
}

func NewWithClient(client *redis.Client) *Storage {
	return &Storage{client: client}
}

func (s *Storage) Ping(ctx context.Context) error {
	if err := s.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis indisponivel: %w", err)
	}
	return nil
}

func (s *Storage) Close() error {
	return s.client.Close()
}

// O TTL e aplicado junto do primeiro incremento, o que mantem a janela fixa
// mesmo com varias requisicoes concorrentes.
func (s *Storage) Hit(ctx context.Context, key string, window time.Duration) (int64, error) {
	pipe := s.client.TxPipeline()
	incr := pipe.Incr(ctx, counterPrefix+key)
	pipe.ExpireNX(ctx, counterPrefix+key, window)

	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

func (s *Storage) Block(ctx context.Context, key string, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	// O contador nao e removido: ele expira junto da janela e manter o valor
	// evita que requisicoes concorrentes reiniciem a contagem durante o bloqueio.
	return s.client.Set(ctx, blockPrefix+key, "1", duration).Err()
}

func (s *Storage) IsBlocked(ctx context.Context, key string) (bool, error) {
	err := s.client.Get(ctx, blockPrefix+key).Err()
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, redis.Nil):
		return false, nil
	default:
		return false, err
	}
}
