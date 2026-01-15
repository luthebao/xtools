package activity

import (
	"fmt"
	"sync"
	"time"

	"xtools/internal/domain"
	"xtools/internal/ports"
)

const maxLogsPerAccount = 500
const maxTotalLogs = 2000

// InMemoryLogger stores activity logs in memory
type InMemoryLogger struct {
	mu       sync.RWMutex
	logs     []domain.ActivityLog
	eventBus ports.EventBus
	counter  int64
}

// NewInMemoryLogger creates a new in-memory activity logger
func NewInMemoryLogger(eventBus ports.EventBus) *InMemoryLogger {
	return &InMemoryLogger{
		logs:     make([]domain.ActivityLog, 0),
		eventBus: eventBus,
	}
}

// Log adds a new activity entry
func (l *InMemoryLogger) Log(accountID string, actType domain.ActivityType, level domain.ActivityLevel, message string, details string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.counter++
	entry := domain.ActivityLog{
		ID:        fmt.Sprintf("%d-%d", time.Now().UnixNano(), l.counter),
		AccountID: accountID,
		Type:      actType,
		Level:     level,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}

	// Prepend (newest first)
	l.logs = append([]domain.ActivityLog{entry}, l.logs...)

	// Trim if too many
	if len(l.logs) > maxTotalLogs {
		l.logs = l.logs[:maxTotalLogs]
	}

	// Emit event for real-time updates
	if l.eventBus != nil {
		l.eventBus.Emit("activity:new", entry)
	}

	// Also print to console for debugging
	fmt.Printf("[%s] [%s] %s: %s\n", level, actType, accountID, message)
}

// GetLogs retrieves logs for an account (newest first)
func (l *InMemoryLogger) GetLogs(accountID string, limit int) []domain.ActivityLog {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []domain.ActivityLog
	for _, log := range l.logs {
		if log.AccountID == accountID {
			result = append(result, log)
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

// GetAllLogs retrieves all logs across accounts (newest first)
func (l *InMemoryLogger) GetAllLogs(limit int) []domain.ActivityLog {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if limit <= 0 || limit > len(l.logs) {
		limit = len(l.logs)
	}

	result := make([]domain.ActivityLog, limit)
	copy(result, l.logs[:limit])
	return result
}

// ClearLogs clears logs for an account
func (l *InMemoryLogger) ClearLogs(accountID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var filtered []domain.ActivityLog
	for _, log := range l.logs {
		if log.AccountID != accountID {
			filtered = append(filtered, log)
		}
	}
	l.logs = filtered
}

// Ensure InMemoryLogger implements ActivityLogger
var _ ports.ActivityLogger = (*InMemoryLogger)(nil)
