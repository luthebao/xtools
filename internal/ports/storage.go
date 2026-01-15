package ports

import (
	"context"
	"xtools/internal/domain"
)

// ConfigStore manages account configurations (YAML files)
type ConfigStore interface {
	// Account configuration
	LoadAccount(accountID string) (*domain.AccountConfig, error)
	SaveAccount(cfg domain.AccountConfig) error
	ListAccounts() ([]domain.AccountConfig, error)
	DeleteAccount(accountID string) error

	// Watch for external config file changes
	WatchChanges(ctx context.Context) <-chan ConfigChangeEvent

	// Reload configuration from disk
	ReloadAccount(accountID string) (*domain.AccountConfig, error)

	// Get config file path
	GetConfigPath(accountID string) string
}

// ConfigChangeEvent represents a config file change
type ConfigChangeEvent struct {
	AccountID string
	EventType string // "created", "modified", "deleted"
	Error     error
}

// MetricsStore persists metrics data (SQLite)
type MetricsStore interface {
	// Profile metrics
	SaveProfileSnapshot(snapshot domain.ProfileSnapshot) error
	GetProfileHistory(accountID string, days int) ([]domain.ProfileSnapshot, error)

	// Tweet metrics
	SaveTweetMetrics(metrics domain.TweetMetrics) error
	GetTweetMetricsHistory(tweetID string, days int) ([]domain.TweetMetrics, error)

	// Reply metrics
	SaveReplyMetrics(metrics domain.ReplyMetrics) error
	GetReplyPerformance(accountID string, days int) (*domain.ReplyPerformanceReport, error)

	// Daily stats
	SaveDailyStats(stats domain.DailyStats) error
	GetDailyStats(accountID string, days int) ([]domain.DailyStats, error)

	// Duplicate tracking
	IsReplied(accountID, tweetID string) (bool, error)
	MarkReplied(accountID, tweetID, replyID string) error

	// Cleanup
	Close() error
}

// ExcelExporter exports data to Excel files
type ExcelExporter interface {
	// Export tweets found for an account
	ExportTweets(accountID string, tweets []domain.Tweet, path string) error

	// Append tweets to existing file
	AppendTweets(accountID string, tweets []domain.Tweet) error

	// Export replies for an account
	ExportReplies(accountID string, replies []domain.Reply, path string) error

	// Export metrics report
	ExportMetrics(report domain.MetricsReport, path string) error

	// Get export file path for account
	GetExportPath(accountID string) string

	// Load tweets from Excel file
	LoadTweets(path string) ([]domain.Tweet, error)
}

// ReplyStore manages pending and sent replies
type ReplyStore interface {
	// Pending replies (approval queue)
	AddPendingReply(item domain.ApprovalQueueItem) error
	GetPendingReplies(accountID string) ([]domain.ApprovalQueueItem, error)
	UpdateReplyStatus(replyID string, status domain.ReplyStatus) error
	RemovePendingReply(replyID string) error

	// Reply history
	SaveReply(reply domain.Reply) error
	GetReplies(accountID string, limit int) ([]domain.Reply, error)
	GetReplyByID(replyID string) (*domain.Reply, error)
}
