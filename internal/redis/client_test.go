package redis_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nitish/ratelimiter/internal/limiter"
	rl "github.com/nitish/ratelimiter/internal/redis"
	"github.com/redis/go-redis/v9"
)

func redisClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available at %s: %v", addr, err)
	}
	return client
}

// uniqueKey returns a key scoped to this test so parallel tests don't collide.
func uniqueKey(t *testing.T, suffix string) string {
	return fmt.Sprintf("test:%s:%s:%d", t.Name(), suffix, time.Now().UnixNano())
}

func TestAllow_UnderLimit(t *testing.T) {
	client := redisClient(t)
	ctx := context.Background()
	lim := rl.NewSlidingWindowLimiter(client, "test")

	rule := limiter.Rule{Name: "test", Limit: 5, Window: 10 * time.Second}
	key := uniqueKey(t, "under")

	for i := 0; i < 5; i++ {
		dec, err := lim.Allow(ctx, key, rule)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		if !dec.Allowed {
			t.Fatalf("request %d: expected allowed, got denied", i)
		}
		if dec.Remaining != int64(5-i-1) {
			t.Errorf("request %d: expected remaining=%d, got %d", i, 5-i-1, dec.Remaining)
		}
	}
}

func TestAllow_AtLimit(t *testing.T) {
	client := redisClient(t)
	ctx := context.Background()
	lim := rl.NewSlidingWindowLimiter(client, "test")

	rule := limiter.Rule{Name: "test", Limit: 3, Window: 10 * time.Second}
	key := uniqueKey(t, "atlimit")

	// Use up the limit.
	for i := 0; i < 3; i++ {
		dec, err := lim.Allow(ctx, key, rule)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		if !dec.Allowed {
			t.Fatalf("request %d: should be allowed", i)
		}
	}

	// Next request should be denied.
	dec, err := lim.Allow(ctx, key, rule)
	if err != nil {
		t.Fatalf("request 4: %v", err)
	}
	if dec.Allowed {
		t.Fatal("request 4: expected denied, got allowed")
	}
	if dec.Remaining != 0 {
		t.Errorf("expected remaining=0, got %d", dec.Remaining)
	}
	if dec.RetryAt <= 0 {
		t.Errorf("expected positive retry_after, got %v", dec.RetryAt)
	}
}

func TestAllow_WindowExpiry(t *testing.T) {
	client := redisClient(t)
	ctx := context.Background()
	lim := rl.NewSlidingWindowLimiter(client, "test")

	// Short window: 1 second.
	rule := limiter.Rule{Name: "test", Limit: 2, Window: 1 * time.Second}
	key := uniqueKey(t, "expiry")

	// Fill the limit.
	for i := 0; i < 2; i++ {
		if _, err := lim.Allow(ctx, key, rule); err != nil {
			t.Fatal(err)
		}
	}

	// Should be denied now.
	dec, _ := lim.Allow(ctx, key, rule)
	if dec.Allowed {
		t.Fatal("expected denied before window expires")
	}

	// Wait for the window to pass.
	time.Sleep(1100 * time.Millisecond)

	// Should be allowed again.
	dec, err := lim.Allow(ctx, key, rule)
	if err != nil {
		t.Fatal(err)
	}
	if !dec.Allowed {
		t.Fatal("expected allowed after window expired")
	}
}

func TestAllow_ConcurrentRequests(t *testing.T) {
	client := redisClient(t)
	ctx := context.Background()
	lim := rl.NewSlidingWindowLimiter(client, "test")

	rule := limiter.Rule{Name: "test", Limit: 50, Window: 10 * time.Second}
	key := uniqueKey(t, "concurrent")

	// Fire 100 concurrent requests with a limit of 50.
	// Exactly 50 should be allowed. This is the core correctness property.
	var allowed atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dec, err := lim.Allow(ctx, key, rule)
			if err != nil {
				t.Errorf("concurrent request error: %v", err)
				return
			}
			if dec.Allowed {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()

	got := allowed.Load()
	if got != 50 {
		t.Errorf("expected exactly 50 allowed, got %d", got)
	}
}

func TestAllow_DifferentKeysIndependent(t *testing.T) {
	client := redisClient(t)
	ctx := context.Background()
	lim := rl.NewSlidingWindowLimiter(client, "test")

	rule := limiter.Rule{Name: "test", Limit: 2, Window: 10 * time.Second}
	key1 := uniqueKey(t, "user1")
	key2 := uniqueKey(t, "user2")

	// Fill key1's limit.
	for i := 0; i < 2; i++ {
		lim.Allow(ctx, key1, rule)
	}

	// key2 should still be allowed.
	dec, err := lim.Allow(ctx, key2, rule)
	if err != nil {
		t.Fatal(err)
	}
	if !dec.Allowed {
		t.Fatal("key2 should not be affected by key1's limit")
	}
}
