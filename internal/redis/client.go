package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/nitish/ratelimiter/internal/limiter"
	"github.com/redis/go-redis/v9"
)

type SlidingWindowLimiter struct {
	client *redis.Client
	prefix string
}

func NewSlidingWindowLimiter(client *redis.Client, prefix string) *SlidingWindowLimiter {
	if prefix == "" {
		prefix = "rl"
	}
	return &SlidingWindowLimiter{client: client, prefix: prefix}
}

func (s *SlidingWindowLimiter) Allow(ctx context.Context, key string, rule limiter.Rule) (*limiter.Decision, error) {
	redisKey := fmt.Sprintf("%s:%s", s.prefix, key)

	requestID, err := randomID()
	if err != nil {
		return nil, fmt.Errorf("generating request id: %w", err)
	}

	result, err := slidingWindowScript.Run(ctx, s.client,
		[]string{redisKey},
		rule.Window.Microseconds(),
		rule.Limit,
		requestID,
	).Int64Slice()
	if err != nil {
		return nil, fmt.Errorf("executing rate limit script: %w", err)
	}

	if len(result) < 4 {
		return nil, fmt.Errorf("unexpected script result length: %d", len(result))
	}

	return &limiter.Decision{
		Allowed:   result[0] == 1,
		Limit:     rule.Limit,
		Remaining: result[1],
		ResetAt:   time.Unix(result[2], 0),
		RetryAt:   time.Duration(result[3]) * time.Second,
	}, nil
}

func randomID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
