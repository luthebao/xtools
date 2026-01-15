package ports

import "xtools/internal/domain"

// ActivityLogger logs and retrieves activity entries
type ActivityLogger interface {
	// Log adds a new activity entry
	Log(accountID string, actType domain.ActivityType, level domain.ActivityLevel, message string, details string)

	// GetLogs retrieves logs for an account (newest first)
	GetLogs(accountID string, limit int) []domain.ActivityLog

	// GetAllLogs retrieves all logs across accounts (newest first)
	GetAllLogs(limit int) []domain.ActivityLog

	// ClearLogs clears logs for an account
	ClearLogs(accountID string)
}
