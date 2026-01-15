package ratelimit

import (
	"context"
	"sync"
	"time"

	"xtools/internal/ports"
)

// TokenBucket implements a token bucket rate limiter
type TokenBucket struct {
	mu          sync.Mutex
	tokens      int
	maxTokens   int
	refillRate  int
	refillPeriod time.Duration
	lastRefill  time.Time
}

// NewTokenBucket creates a new token bucket rate limiter
func NewTokenBucket(rate int, period time.Duration) *TokenBucket {
	return &TokenBucket{
		tokens:       rate,
		maxTokens:    rate,
		refillRate:   rate,
		refillPeriod: period,
		lastRefill:   time.Now(),
	}
}

// NewBurstTokenBucket creates a token bucket with burst capacity
func NewBurstTokenBucket(rate int, burst int, period time.Duration) *TokenBucket {
	return &TokenBucket{
		tokens:       burst,
		maxTokens:    burst,
		refillRate:   rate,
		refillPeriod: period,
		lastRefill:   time.Now(),
	}
}

// refill adds tokens based on elapsed time
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	// Calculate tokens to add based on elapsed time
	periodsElapsed := int(elapsed / tb.refillPeriod)
	if periodsElapsed > 0 {
		tokensToAdd := periodsElapsed * tb.refillRate
		tb.tokens = min(tb.tokens+tokensToAdd, tb.maxTokens)
		tb.lastRefill = tb.lastRefill.Add(time.Duration(periodsElapsed) * tb.refillPeriod)
	}
}

// Acquire blocks until a token is available or context is cancelled
func (tb *TokenBucket) Acquire(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if tb.TryAcquire() {
				return nil
			}
			// Wait before retrying
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// TryAcquire attempts to acquire a token immediately
func (tb *TokenBucket) TryAcquire() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}

// GetStatus returns the current rate limit status
func (tb *TokenBucket) GetStatus() ports.RateLimitInfo {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	return ports.RateLimitInfo{
		Remaining: tb.tokens,
		Limit:     tb.maxTokens,
		ResetIn:   tb.refillPeriod - time.Since(tb.lastRefill),
		IsLimited: tb.tokens == 0,
	}
}

// Reset resets the rate limiter to full capacity
func (tb *TokenBucket) Reset() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.tokens = tb.maxTokens
	tb.lastRefill = time.Now()
}

// SetRate updates the rate limit parameters
func (tb *TokenBucket) SetRate(rate int, period time.Duration) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refillRate = rate
	tb.maxTokens = rate
	tb.refillPeriod = period
}

// MultiLimiter combines multiple rate limiters
type MultiLimiter struct {
	mu       sync.RWMutex
	limiters map[string]ports.RateLimiter
}

// NewMultiLimiter creates a new multi-limiter
func NewMultiLimiter() *MultiLimiter {
	return &MultiLimiter{
		limiters: make(map[string]ports.RateLimiter),
	}
}

// AddLimiter adds a rate limiter with a name
func (m *MultiLimiter) AddLimiter(name string, limiter ports.RateLimiter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.limiters[name] = limiter
}

// Acquire acquires from all limiters
func (m *MultiLimiter) Acquire(ctx context.Context) error {
	m.mu.RLock()
	limiters := make([]ports.RateLimiter, 0, len(m.limiters))
	for _, l := range m.limiters {
		limiters = append(limiters, l)
	}
	m.mu.RUnlock()

	for _, l := range limiters {
		if err := l.Acquire(ctx); err != nil {
			return err
		}
	}
	return nil
}

// TryAcquire tries to acquire from all limiters
func (m *MultiLimiter) TryAcquire() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, l := range m.limiters {
		if !l.TryAcquire() {
			return false
		}
	}
	return true
}

// GetLimiter returns a specific limiter by name
func (m *MultiLimiter) GetLimiter(name string) ports.RateLimiter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.limiters[name]
}

// GetAllStatus returns status of all limiters
func (m *MultiLimiter) GetAllStatus() map[string]ports.RateLimitInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]ports.RateLimitInfo, len(m.limiters))
	for name, l := range m.limiters {
		status[name] = l.GetStatus()
	}
	return status
}

// Factory creates rate limiters
type Factory struct{}

// NewFactory creates a new rate limiter factory
func NewFactory() *Factory {
	return &Factory{}
}

// CreateLimiter creates a rate limiter with the given rate and period
func (f *Factory) CreateLimiter(rate int, period time.Duration) ports.RateLimiter {
	return NewTokenBucket(rate, period)
}

// CreateBurstLimiter creates a rate limiter with burst capacity
func (f *Factory) CreateBurstLimiter(rate int, burst int, period time.Duration) ports.RateLimiter {
	return NewBurstTokenBucket(rate, burst, period)
}

// Ensure implementations satisfy interfaces
var _ ports.RateLimiter = (*TokenBucket)(nil)
var _ ports.MultiRateLimiter = (*MultiLimiter)(nil)
var _ ports.RateLimiterFactory = (*Factory)(nil)
