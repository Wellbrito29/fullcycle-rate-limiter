package limiter

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const DefaultWindow = time.Second

type Kind string

const (
	KindIP    Kind = "ip"
	KindToken Kind = "token"
)

var ErrNoStorage = errors.New("limiter: storage nao informado")

type Policy struct {
	Limit         int
	BlockDuration time.Duration
}

type Options struct {
	Window       time.Duration
	IP           Policy
	TokenDefault Policy
	Tokens       map[string]Policy
}

type Request struct {
	IP    string
	Token string
}

type Result struct {
	Allowed   bool
	Kind      Kind
	Key       string
	Limit     int
	Remaining int
	Blocked   bool
}

type Limiter struct {
	storage Storage
	opts    Options
}

func New(storage Storage, opts Options) (*Limiter, error) {
	if storage == nil {
		return nil, ErrNoStorage
	}
	if opts.Window <= 0 {
		opts.Window = DefaultWindow
	}
	if opts.Tokens == nil {
		opts.Tokens = map[string]Policy{}
	}
	return &Limiter{storage: storage, opts: opts}, nil
}

func (l *Limiter) Allow(ctx context.Context, req Request) (Result, error) {
	kind, key, policy := l.resolve(req)
	result := Result{Kind: kind, Key: key, Limit: policy.Limit}

	if policy.Limit <= 0 {
		result.Allowed = true
		return result, nil
	}

	blocked, err := l.storage.IsBlocked(ctx, key)
	if err != nil {
		return result, fmt.Errorf("limiter: falha ao checar bloqueio: %w", err)
	}
	if blocked {
		result.Blocked = true
		return result, nil
	}

	total, err := l.storage.Hit(ctx, key, l.opts.Window)
	if err != nil {
		return result, fmt.Errorf("limiter: falha ao contabilizar requisicao: %w", err)
	}

	if total > int64(policy.Limit) {
		if err := l.storage.Block(ctx, key, policy.BlockDuration); err != nil {
			return result, fmt.Errorf("limiter: falha ao bloquear chave: %w", err)
		}
		result.Blocked = true
		return result, nil
	}

	result.Allowed = true
	result.Remaining = policy.Limit - int(total)
	return result, nil
}

// A configuracao do token sempre se sobrepoe a configuracao do IP.
func (l *Limiter) resolve(req Request) (Kind, string, Policy) {
	if req.Token != "" {
		policy := l.opts.TokenDefault
		if override, ok := l.opts.Tokens[req.Token]; ok {
			policy = override
		}
		return KindToken, "token:" + req.Token, policy
	}
	return KindIP, "ip:" + req.IP, l.opts.IP
}
