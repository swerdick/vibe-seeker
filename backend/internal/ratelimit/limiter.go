package ratelimit

import (
	"context"
	"time"
)

// Limiter provides a simple ticker-based rate limiter that respects context cancellation.
type Limiter struct {
	ticker *time.Ticker
}

// NewLimiter creates a rate limiter that allows one operation per interval.
func NewLimiter(interval time.Duration) *Limiter {
	return &Limiter{ticker: time.NewTicker(interval)}
}

// Wait blocks until the next tick or the context is cancelled.
// Returns the context error if cancelled, nil otherwise.
func (l *Limiter) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l.ticker.C:
		return nil
	}
}

// Stop releases the underlying ticker resources.
func (l *Limiter) Stop() {
	l.ticker.Stop()
}
