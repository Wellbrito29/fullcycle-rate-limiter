package limiter

import (
	"context"
	"time"
)

// Storage e a estrategia de persistencia do limiter. Trocar Redis por outro
// mecanismo significa apenas fornecer outra implementacao desta interface.
type Storage interface {
	Hit(ctx context.Context, key string, window time.Duration) (int64, error)
	Block(ctx context.Context, key string, duration time.Duration) error
	IsBlocked(ctx context.Context, key string) (bool, error)
}
