package domain

import "time"

// ActivityType represents the type of activity
type ActivityType string

const (
	ActivityTypeSearch     ActivityType = "search"
	ActivityTypeReply      ActivityType = "reply"
	ActivityTypeAuth       ActivityType = "auth"
	ActivityTypeError      ActivityType = "error"
	ActivityTypeRateLimit  ActivityType = "rate_limit"
	ActivityTypeWorker     ActivityType = "worker"
	ActivityTypeConfig     ActivityType = "config"
)

// ActivityLevel represents the severity level
type ActivityLevel string

const (
	ActivityLevelInfo    ActivityLevel = "info"
	ActivityLevelSuccess ActivityLevel = "success"
	ActivityLevelWarning ActivityLevel = "warning"
	ActivityLevelError   ActivityLevel = "error"
)

// ActivityLog represents a single activity entry
type ActivityLog struct {
	ID        string        `json:"id"`
	AccountID string        `json:"accountId"`
	Type      ActivityType  `json:"type"`
	Level     ActivityLevel `json:"level"`
	Message   string        `json:"message"`
	Details   string        `json:"details,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}
