package memory

import (
	"context"
	"sync"
	"time"
)

type counter struct {
	hits      int64
	expiresAt time.Time
}

// Storage mantem os contadores em memoria. Serve como estrategia alternativa
// ao Redis e e usada nos testes do limiter.
type Storage struct {
	mu       sync.Mutex
	counters map[string]*counter
	blocks   map[string]time.Time
	now      func() time.Time
}

func New() *Storage {
	return &Storage{
		counters: map[string]*counter{},
		blocks:   map[string]time.Time{},
		now:      time.Now,
	}
}

// WithClock permite controlar o tempo nos testes.
func (s *Storage) WithClock(now func() time.Time) *Storage {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = now
	return s
}

func (s *Storage) Hit(_ context.Context, key string, window time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	current, ok := s.counters[key]
	if !ok || now.After(current.expiresAt) {
		s.counters[key] = &counter{hits: 1, expiresAt: now.Add(window)}
		return 1, nil
	}

	current.hits++
	return current.hits, nil
}

func (s *Storage) Block(_ context.Context, key string, duration time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.blocks[key] = s.now().Add(duration)
	return nil
}

func (s *Storage) IsBlocked(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	until, ok := s.blocks[key]
	if !ok {
		return false, nil
	}
	if s.now().After(until) {
		delete(s.blocks, key)
		return false, nil
	}
	return true, nil
}
