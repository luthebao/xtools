package storage

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"xtools/internal/domain"
	"xtools/internal/ports"
)

// ActionStore implements ports.ActionStore using SQLite
type ActionStore struct {
	db *sql.DB
}

// NewActionStore creates a new action store
func NewActionStore(db *sql.DB) (*ActionStore, error) {
	store := &ActionStore{db: db}
	if err := store.migrate(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *ActionStore) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS tweet_actions (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			trigger_type TEXT NOT NULL,
			wallet_address TEXT NOT NULL,
			wallet_profile TEXT,
			trade_event TEXT,
			market_url TEXT,
			profile_url TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			draft_text TEXT,
			reviewed_text TEXT,
			final_text TEXT,
			screenshot_path TEXT,
			posted_tweet_id TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			processed_at DATETIME,
			retry_count INTEGER DEFAULT 0,
			next_retry_at DATETIME,
			error_message TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_actions_account ON tweet_actions(account_id)`,
		`CREATE INDEX IF NOT EXISTS idx_actions_status ON tweet_actions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_actions_retry ON tweet_actions(next_retry_at) WHERE next_retry_at IS NOT NULL`,
		`CREATE TABLE IF NOT EXISTS action_event_log (
			account_id TEXT NOT NULL,
			event_id INTEGER NOT NULL,
			action_id TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (account_id, event_id)
		)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			log.Printf("[ActionStore] Migration error: %v", err)
			return err
		}
	}
	return nil
}

// EnqueueAction adds a new action to the queue
func (s *ActionStore) EnqueueAction(action domain.TweetAction) error {
	walletProfileJSON, _ := json.Marshal(action.WalletProfile)
	tradeEventJSON, _ := json.Marshal(action.TradeEvent)

	_, err := s.db.Exec(`
		INSERT INTO tweet_actions (
			id, account_id, trigger_type, wallet_address, wallet_profile, trade_event,
			market_url, profile_url, status, draft_text, reviewed_text, final_text,
			screenshot_path, posted_tweet_id, created_at, updated_at, processed_at,
			retry_count, next_retry_at, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		action.ID, action.AccountID, action.TriggerType, action.WalletAddress,
		string(walletProfileJSON), string(tradeEventJSON),
		action.MarketURL, action.ProfileURL, action.Status,
		action.DraftText, action.ReviewedText, action.FinalText,
		action.ScreenshotPath, action.PostedTweetID,
		action.CreatedAt, action.UpdatedAt, action.ProcessedAt,
		action.RetryCount, action.NextRetryAt, action.ErrorMessage,
	)
	return err
}

// DequeueActions gets pending actions ready for processing
func (s *ActionStore) DequeueActions(accountID string, limit int) ([]domain.TweetAction, error) {
	query := `
		SELECT id, account_id, trigger_type, wallet_address, wallet_profile, trade_event,
			market_url, profile_url, status, draft_text, reviewed_text, final_text,
			screenshot_path, posted_tweet_id, created_at, updated_at, processed_at,
			retry_count, next_retry_at, error_message
		FROM tweet_actions
		WHERE account_id = ? AND status = 'pending'
			AND (next_retry_at IS NULL OR next_retry_at <= ?)
		ORDER BY created_at ASC
		LIMIT ?
	`
	return s.queryActions(query, accountID, time.Now(), limit)
}

// UpdateActionStatus updates the status of an action
func (s *ActionStore) UpdateActionStatus(actionID string, status domain.ActionStatus, errorMsg string) error {
	_, err := s.db.Exec(`
		UPDATE tweet_actions SET status = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`, status, errorMsg, time.Now(), actionID)
	return err
}

// UpdateAction updates all fields of an action
func (s *ActionStore) UpdateAction(action domain.TweetAction) error {
	walletProfileJSON, _ := json.Marshal(action.WalletProfile)
	tradeEventJSON, _ := json.Marshal(action.TradeEvent)

	_, err := s.db.Exec(`
		UPDATE tweet_actions SET
			trigger_type = ?, wallet_address = ?, wallet_profile = ?, trade_event = ?,
			market_url = ?, profile_url = ?, status = ?, draft_text = ?,
			reviewed_text = ?, final_text = ?, screenshot_path = ?, posted_tweet_id = ?,
			updated_at = ?, processed_at = ?, retry_count = ?, next_retry_at = ?, error_message = ?
		WHERE id = ?
	`,
		action.TriggerType, action.WalletAddress,
		string(walletProfileJSON), string(tradeEventJSON),
		action.MarketURL, action.ProfileURL, action.Status, action.DraftText,
		action.ReviewedText, action.FinalText, action.ScreenshotPath, action.PostedTweetID,
		time.Now(), action.ProcessedAt, action.RetryCount, action.NextRetryAt, action.ErrorMessage,
		action.ID,
	)
	return err
}

// GetAction retrieves a single action by ID
func (s *ActionStore) GetAction(actionID string) (*domain.TweetAction, error) {
	query := `
		SELECT id, account_id, trigger_type, wallet_address, wallet_profile, trade_event,
			market_url, profile_url, status, draft_text, reviewed_text, final_text,
			screenshot_path, posted_tweet_id, created_at, updated_at, processed_at,
			retry_count, next_retry_at, error_message
		FROM tweet_actions WHERE id = ?
	`
	actions, err := s.queryActions(query, actionID)
	if err != nil {
		return nil, err
	}
	if len(actions) == 0 {
		return nil, nil
	}
	return &actions[0], nil
}

// GetPendingActions returns all pending actions for an account
func (s *ActionStore) GetPendingActions(accountID string) ([]domain.TweetAction, error) {
	query := `
		SELECT id, account_id, trigger_type, wallet_address, wallet_profile, trade_event,
			market_url, profile_url, status, draft_text, reviewed_text, final_text,
			screenshot_path, posted_tweet_id, created_at, updated_at, processed_at,
			retry_count, next_retry_at, error_message
		FROM tweet_actions
		WHERE account_id = ? AND status IN ('pending', 'fetching', 'generating', 'reviewing', 'capturing', 'posting')
		ORDER BY created_at DESC
	`
	return s.queryActions(query, accountID)
}

// GetRetryableActions returns actions that need retry
func (s *ActionStore) GetRetryableActions(maxRetries int) ([]domain.TweetAction, error) {
	query := `
		SELECT id, account_id, trigger_type, wallet_address, wallet_profile, trade_event,
			market_url, profile_url, status, draft_text, reviewed_text, final_text,
			screenshot_path, posted_tweet_id, created_at, updated_at, processed_at,
			retry_count, next_retry_at, error_message
		FROM tweet_actions
		WHERE status = 'pending' AND retry_count > 0 AND retry_count <= ?
			AND next_retry_at IS NOT NULL AND next_retry_at <= ?
		ORDER BY next_retry_at ASC
	`
	return s.queryActions(query, maxRetries, time.Now())
}

// GetActionHistory returns recent actions for an account
func (s *ActionStore) GetActionHistory(accountID string, limit int) ([]domain.TweetActionHistory, error) {
	query := `
		SELECT id, account_id, wallet_address, market_url, final_text, posted_tweet_id,
			status, created_at, processed_at, error_message
		FROM tweet_actions
		WHERE account_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`
	rows, err := s.db.Query(query, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []domain.TweetActionHistory
	for rows.Next() {
		var h domain.TweetActionHistory
		var processedAt sql.NullTime
		var errorMsg sql.NullString

		if err := rows.Scan(
			&h.ID, &h.AccountID, &h.WalletAddress, &h.MarketName,
			&h.TweetText, &h.PostedTweetID, &h.Status,
			&h.CreatedAt, &processedAt, &errorMsg,
		); err != nil {
			return nil, err
		}

		if processedAt.Valid {
			h.ProcessedAt = &processedAt.Time
		}
		if errorMsg.Valid {
			h.ErrorMessage = errorMsg.String
		}
		history = append(history, h)
	}
	return history, nil
}

// GetActionStats returns statistics for an account
func (s *ActionStore) GetActionStats(accountID string) (*domain.ActionStats, error) {
	stats := &domain.ActionStats{}

	row := s.db.QueryRow(`
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed,
			SUM(CASE WHEN status = 'queued' THEN 1 ELSE 0 END) as queued
		FROM tweet_actions WHERE account_id = ?
	`, accountID)

	if err := row.Scan(&stats.TotalActions, &stats.PendingCount,
		&stats.CompletedCount, &stats.FailedCount, &stats.QueuedCount); err != nil {
		return nil, err
	}
	return stats, nil
}

// HasActionForEvent checks if an action already exists for an event
func (s *ActionStore) HasActionForEvent(accountID string, eventID int64) (bool, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM action_event_log WHERE account_id = ? AND event_id = ?
	`, accountID, eventID).Scan(&count)
	return count > 0, err
}

// MarkActionForEvent records that an action was created for an event
func (s *ActionStore) MarkActionForEvent(accountID string, eventID int64, actionID string) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO action_event_log (account_id, event_id, action_id) VALUES (?, ?, ?)
	`, accountID, eventID, actionID)
	return err
}

func (s *ActionStore) queryActions(query string, args ...interface{}) ([]domain.TweetAction, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []domain.TweetAction
	for rows.Next() {
		var a domain.TweetAction
		var walletProfileJSON, tradeEventJSON sql.NullString
		var processedAt, nextRetryAt sql.NullTime
		var errorMsg sql.NullString

		if err := rows.Scan(
			&a.ID, &a.AccountID, &a.TriggerType, &a.WalletAddress,
			&walletProfileJSON, &tradeEventJSON,
			&a.MarketURL, &a.ProfileURL, &a.Status, &a.DraftText,
			&a.ReviewedText, &a.FinalText, &a.ScreenshotPath, &a.PostedTweetID,
			&a.CreatedAt, &a.UpdatedAt, &processedAt,
			&a.RetryCount, &nextRetryAt, &errorMsg,
		); err != nil {
			return nil, err
		}

		if walletProfileJSON.Valid {
			json.Unmarshal([]byte(walletProfileJSON.String), &a.WalletProfile)
		}
		if tradeEventJSON.Valid {
			json.Unmarshal([]byte(tradeEventJSON.String), &a.TradeEvent)
		}
		if processedAt.Valid {
			a.ProcessedAt = &processedAt.Time
		}
		if nextRetryAt.Valid {
			a.NextRetryAt = &nextRetryAt.Time
		}
		if errorMsg.Valid {
			a.ErrorMessage = errorMsg.String
		}
		actions = append(actions, a)
	}
	return actions, nil
}

// Ensure ActionStore implements ports.ActionStore
var _ ports.ActionStore = (*ActionStore)(nil)
