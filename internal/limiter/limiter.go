package limiter

import (
	"context"
	"time"
)

type Rule struct {
	Name   string
	Limit  int64
	Window time.Duration
}

type Decision struct {
	Allowed   bool
	Limit     int64
	Remaining int64
	ResetAt   time.Time
	RetryAt   time.Duration
}

type Limiter interface {
	Allow(ctx context.Context, key string, rule Rule) (*Decision, error)
}
