package capabilityd

import (
	"context"
	"sync"
	"time"
)

const (
	mattermostRequestsPerSecond = 50.0
	mattermostRequestBurst      = 50.0
)

var defaultMattermostRateLimiter = newMattermostRateLimiter(mattermostRequestsPerSecond, mattermostRequestBurst)

type mattermostRateLimiter struct {
	mutex           sync.Mutex
	availableTokens float64
	maximumTokens   float64
	refillPerSecond float64
	lastRefillAt    time.Time
}

func newMattermostRateLimiter(refillPerSecond float64, maximumTokens float64) *mattermostRateLimiter {
	return &mattermostRateLimiter{
		availableTokens: maximumTokens,
		maximumTokens:   maximumTokens,
		refillPerSecond: refillPerSecond,
		lastRefillAt:    time.Now(),
	}
}

func (limiter *mattermostRateLimiter) wait(ctx context.Context) error {
	waitDuration := limiter.reserveToken()
	if waitDuration <= 0 {
		return nil
	}
	timer := time.NewTimer(waitDuration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (limiter *mattermostRateLimiter) reserveToken() time.Duration {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	now := time.Now()
	refilledTokens := limiter.availableTokens + now.Sub(limiter.lastRefillAt).Seconds()*limiter.refillPerSecond
	if refilledTokens > limiter.maximumTokens {
		refilledTokens = limiter.maximumTokens
	}
	limiter.lastRefillAt = now
	limiter.availableTokens = refilledTokens - 1
	if limiter.availableTokens >= 0 {
		return 0
	}
	return time.Duration(-limiter.availableTokens / limiter.refillPerSecond * float64(time.Second))
}
