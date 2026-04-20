package bench_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/nitish/ratelimiter/internal/limiter"
	redislimiter "github.com/nitish/ratelimiter/internal/redis"
	"github.com/redis/go-redis/v9"
)

func redisClient(b *testing.B) *redis.Client {
	b.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		b.Skipf("Redis not available at %s: %v", addr, err)
	}
	b.Cleanup(func() { client.Close() })
	return client
}

var rule = limiter.Rule{Name: "bench", Limit: 1_000_000, Window: 60 * time.Second}

// BenchmarkAllow measures a single sequential rate limit decision.
// This is the baseline: one goroutine, one Redis round trip.
func BenchmarkAllow(b *testing.B) {
	lim := redislimiter.NewSlidingWindowLimiter(redisClient(b), "bench")
	ctx := context.Background()
	key := fmt.Sprintf("benchkey-%d", b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := lim.Allow(ctx, key, rule); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAllowParallel measures throughput under concurrent load.
// GOMAXPROCS goroutines all hitting the same key simultaneously.
func BenchmarkAllowParallel(b *testing.B) {
	lim := redislimiter.NewSlidingWindowLimiter(redisClient(b), "bench")
	ctx := context.Background()
	key := fmt.Sprintf("benchkey-parallel-%d", b.N)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := lim.Allow(ctx, key, rule); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkAllowDistinctKeys measures throughput with no key contention.
// Each goroutine uses its own key, so there is no ZSET-level lock contention.
func BenchmarkAllowDistinctKeys(b *testing.B) {
	lim := redislimiter.NewSlidingWindowLimiter(redisClient(b), "bench")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		key := fmt.Sprintf("benchkey-distinct-%d", b.N)
		for pb.Next() {
			k := fmt.Sprintf("%s-%d", key, i)
			if _, err := lim.Allow(ctx, k, rule); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}
