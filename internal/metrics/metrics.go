package metrics

import (
	"context"
	"time"

	"github.com/nitish/ratelimiter/internal/limiter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	decisions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ratelimit_decisions_total",
		Help: "Total rate limit decisions, partitioned by result and rule.",
	}, []string{"result", "rule"})

	decisionDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ratelimit_decision_duration_seconds",
		Help:    "Time spent evaluating a rate limit decision.",
		Buckets: []float64{.0005, .001, .0025, .005, .01, .025, .05},
	}, []string{"rule"})

	redisErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ratelimit_redis_errors_total",
		Help: "Total errors communicating with Redis.",
	})
)

// InstrumentedLimiter wraps any Limiter and records Prometheus metrics
// for every decision.
type InstrumentedLimiter struct {
	inner limiter.Limiter
}

func NewInstrumentedLimiter(inner limiter.Limiter) *InstrumentedLimiter {
	return &InstrumentedLimiter{inner: inner}
}

func (l *InstrumentedLimiter) Allow(ctx context.Context, key string, rule limiter.Rule) (*limiter.Decision, error) {
	start := time.Now()

	dec, err := l.inner.Allow(ctx, key, rule)

	duration := time.Since(start).Seconds()
	decisionDuration.WithLabelValues(rule.Name).Observe(duration)

	if err != nil {
		redisErrors.Inc()
		return dec, err
	}

	result := "allowed"
	if !dec.Allowed {
		result = "denied"
	}
	decisions.WithLabelValues(result, rule.Name).Inc()

	return dec, nil
}
