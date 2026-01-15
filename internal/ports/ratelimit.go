package ports

import (
	"context"
	"time"
)

// RateLimiter controls the rate of operations
type RateLimiter interface {
	// Acquire attempts to acquire a token, blocking until available or context cancelled
	Acquire(ctx context.Context) error

	// TryAcquire attempts to acquire a token immediately, returns false if not available
	TryAcquire() bool

	// GetStatus returns current rate limit status
	GetStatus() RateLimitInfo

	// Reset resets the rate limiter
	Reset()

	// SetRate updates the rate limit
	SetRate(rate int, period time.Duration)
}

// RateLimitInfo provides current rate limit state
type RateLimitInfo struct {
	Remaining int           `json:"remaining"`
	Limit     int           `json:"limit"`
	ResetIn   time.Duration `json:"resetIn"`
	IsLimited bool          `json:"isLimited"`
}

// RateLimiterFactory creates rate limiters
type RateLimiterFactory interface {
	// CreateLimiter creates a rate limiter with the given rate and period
	CreateLimiter(rate int, period time.Duration) RateLimiter

	// CreateBurstLimiter creates a rate limiter with burst capacity
	CreateBurstLimiter(rate int, burst int, period time.Duration) RateLimiter
}

// MultiRateLimiter combines multiple rate limiters (e.g., hourly + daily)
type MultiRateLimiter interface {
	// AddLimiter adds a rate limiter with a name
	AddLimiter(name string, limiter RateLimiter)

	// Acquire acquires from all limiters
	Acquire(ctx context.Context) error

	// TryAcquire tries to acquire from all limiters
	TryAcquire() bool

	// GetLimiter returns a specific limiter by name
	GetLimiter(name string) RateLimiter

	// GetAllStatus returns status of all limiters
	GetAllStatus() map[string]RateLimitInfo
}
