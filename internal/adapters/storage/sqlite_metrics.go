package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"xtools/internal/domain"
	"xtools/internal/ports"
)

// SQLiteMetricsStore implements MetricsStore using SQLite
type SQLiteMetricsStore struct {
	db *sql.DB
}

// NewSQLiteMetricsStore creates a new SQLite-based metrics store
func NewSQLiteMetricsStore(dbPath string) (*SQLiteMetricsStore, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &SQLiteMetricsStore{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteMetricsStore) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS profile_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			followers_count INTEGER,
			following_count INTEGER,
			tweet_count INTEGER,
			listed_count INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_profile_account ON profile_snapshots(account_id, timestamp)`,
		`CREATE TABLE IF NOT EXISTS tweet_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tweet_id TEXT NOT NULL,
			account_id TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			impressions INTEGER,
			engagements INTEGER,
			like_count INTEGER,
			retweet_count INTEGER,
			reply_count INTEGER,
			quote_count INTEGER,
			click_count INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tweet_metrics ON tweet_metrics(tweet_id, timestamp)`,
		`CREATE TABLE IF NOT EXISTS reply_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			reply_id TEXT NOT NULL,
			account_id TEXT NOT NULL,
			original_tweet_id TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			like_count INTEGER,
			retweet_count INTEGER,
			impressions INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_reply_metrics ON reply_metrics(account_id, timestamp)`,
		`CREATE TABLE IF NOT EXISTS daily_stats (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id TEXT NOT NULL,
			date DATE NOT NULL,
			tweets_searched INTEGER,
			replies_generated INTEGER,
			replies_sent INTEGER,
			replies_failed INTEGER,
			tokens_used INTEGER,
			UNIQUE(account_id, date)
		)`,
		`CREATE TABLE IF NOT EXISTS replied_tweets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id TEXT NOT NULL,
			tweet_id TEXT NOT NULL,
			reply_id TEXT NOT NULL,
			replied_at DATETIME NOT NULL,
			UNIQUE(account_id, tweet_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_replied ON replied_tweets(account_id, tweet_id)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// SaveProfileSnapshot saves a profile metrics snapshot
func (s *SQLiteMetricsStore) SaveProfileSnapshot(snapshot domain.ProfileSnapshot) error {
	_, err := s.db.Exec(`
		INSERT INTO profile_snapshots (account_id, timestamp, followers_count, following_count, tweet_count, listed_count)
		VALUES (?, ?, ?, ?, ?, ?)`,
		snapshot.AccountID, snapshot.Timestamp, snapshot.FollowersCount,
		snapshot.FollowingCount, snapshot.TweetCount, snapshot.ListedCount)
	return err
}

// GetProfileHistory returns profile history for days
func (s *SQLiteMetricsStore) GetProfileHistory(accountID string, days int) ([]domain.ProfileSnapshot, error) {
	since := time.Now().AddDate(0, 0, -days)
	rows, err := s.db.Query(`
		SELECT account_id, timestamp, followers_count, following_count, tweet_count, listed_count
		FROM profile_snapshots
		WHERE account_id = ? AND timestamp >= ?
		ORDER BY timestamp DESC`,
		accountID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []domain.ProfileSnapshot
	for rows.Next() {
		var snap domain.ProfileSnapshot
		if err := rows.Scan(&snap.AccountID, &snap.Timestamp, &snap.FollowersCount,
			&snap.FollowingCount, &snap.TweetCount, &snap.ListedCount); err != nil {
			continue
		}
		snapshots = append(snapshots, snap)
	}

	return snapshots, nil
}

// SaveTweetMetrics saves tweet metrics
func (s *SQLiteMetricsStore) SaveTweetMetrics(metrics domain.TweetMetrics) error {
	_, err := s.db.Exec(`
		INSERT INTO tweet_metrics (tweet_id, account_id, timestamp, impressions, engagements, like_count, retweet_count, reply_count, quote_count, click_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		metrics.TweetID, metrics.AccountID, metrics.Timestamp, metrics.Impressions,
		metrics.Engagements, metrics.LikeCount, metrics.RetweetCount, metrics.ReplyCount,
		metrics.QuoteCount, metrics.ClickCount)
	return err
}

// GetTweetMetricsHistory returns metrics history for a tweet
func (s *SQLiteMetricsStore) GetTweetMetricsHistory(tweetID string, days int) ([]domain.TweetMetrics, error) {
	since := time.Now().AddDate(0, 0, -days)
	rows, err := s.db.Query(`
		SELECT tweet_id, account_id, timestamp, impressions, engagements, like_count, retweet_count, reply_count, quote_count, click_count
		FROM tweet_metrics
		WHERE tweet_id = ? AND timestamp >= ?
		ORDER BY timestamp DESC`,
		tweetID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []domain.TweetMetrics
	for rows.Next() {
		var m domain.TweetMetrics
		if err := rows.Scan(&m.TweetID, &m.AccountID, &m.Timestamp, &m.Impressions,
			&m.Engagements, &m.LikeCount, &m.RetweetCount, &m.ReplyCount,
			&m.QuoteCount, &m.ClickCount); err != nil {
			continue
		}
		metrics = append(metrics, m)
	}

	return metrics, nil
}

// SaveReplyMetrics saves reply metrics
func (s *SQLiteMetricsStore) SaveReplyMetrics(metrics domain.ReplyMetrics) error {
	_, err := s.db.Exec(`
		INSERT INTO reply_metrics (reply_id, account_id, original_tweet_id, timestamp, like_count, retweet_count, impressions)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		metrics.ReplyID, metrics.AccountID, metrics.OriginalTweetID, metrics.Timestamp,
		metrics.LikeCount, metrics.RetweetCount, metrics.Impressions)
	return err
}

// GetReplyPerformance returns reply performance report
func (s *SQLiteMetricsStore) GetReplyPerformance(accountID string, days int) (*domain.ReplyPerformanceReport, error) {
	since := time.Now().AddDate(0, 0, -days)

	var report domain.ReplyPerformanceReport
	report.AccountID = accountID
	report.Period = fmt.Sprintf("%d days", days)

	// Get aggregate stats
	row := s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(AVG(like_count), 0), COALESCE(AVG(impressions), 0)
		FROM reply_metrics
		WHERE account_id = ? AND timestamp >= ?`,
		accountID, since)

	row.Scan(&report.TotalReplies, &report.AvgLikesPerReply, &report.AvgImpressionsPerReply)

	// Get top performing replies
	rows, err := s.db.Query(`
		SELECT reply_id, account_id, original_tweet_id, timestamp, like_count, retweet_count, impressions
		FROM reply_metrics
		WHERE account_id = ? AND timestamp >= ?
		ORDER BY like_count DESC
		LIMIT 5`,
		accountID, since)
	if err != nil {
		return &report, nil
	}
	defer rows.Close()

	for rows.Next() {
		var m domain.ReplyMetrics
		if err := rows.Scan(&m.ReplyID, &m.AccountID, &m.OriginalTweetID, &m.Timestamp,
			&m.LikeCount, &m.RetweetCount, &m.Impressions); err != nil {
			continue
		}
		report.TopPerformingReplies = append(report.TopPerformingReplies, m)
	}

	return &report, nil
}

// SaveDailyStats saves daily statistics
func (s *SQLiteMetricsStore) SaveDailyStats(stats domain.DailyStats) error {
	_, err := s.db.Exec(`
		INSERT INTO daily_stats (account_id, date, tweets_searched, replies_generated, replies_sent, replies_failed, tokens_used)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_id, date) DO UPDATE SET
			tweets_searched = tweets_searched + excluded.tweets_searched,
			replies_generated = replies_generated + excluded.replies_generated,
			replies_sent = replies_sent + excluded.replies_sent,
			replies_failed = replies_failed + excluded.replies_failed,
			tokens_used = tokens_used + excluded.tokens_used`,
		stats.AccountID, stats.Date.Format("2006-01-02"), stats.TweetsSearched,
		stats.RepliesGenerated, stats.RepliesSent, stats.RepliesFailed, stats.TokensUsed)
	return err
}

// GetDailyStats returns daily statistics
func (s *SQLiteMetricsStore) GetDailyStats(accountID string, days int) ([]domain.DailyStats, error) {
	since := time.Now().AddDate(0, 0, -days)
	rows, err := s.db.Query(`
		SELECT account_id, date, tweets_searched, replies_generated, replies_sent, replies_failed, tokens_used
		FROM daily_stats
		WHERE account_id = ? AND date >= ?
		ORDER BY date DESC`,
		accountID, since.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []domain.DailyStats
	for rows.Next() {
		var s domain.DailyStats
		var dateStr string
		if err := rows.Scan(&s.AccountID, &dateStr, &s.TweetsSearched,
			&s.RepliesGenerated, &s.RepliesSent, &s.RepliesFailed, &s.TokensUsed); err != nil {
			continue
		}
		s.Date, _ = time.Parse("2006-01-02", dateStr)
		stats = append(stats, s)
	}

	return stats, nil
}

// IsReplied checks if already replied to a tweet
func (s *SQLiteMetricsStore) IsReplied(accountID, tweetID string) (bool, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM replied_tweets WHERE account_id = ? AND tweet_id = ?`,
		accountID, tweetID).Scan(&count)
	return count > 0, err
}

// MarkReplied marks a tweet as replied
func (s *SQLiteMetricsStore) MarkReplied(accountID, tweetID, replyID string) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO replied_tweets (account_id, tweet_id, reply_id, replied_at)
		VALUES (?, ?, ?, ?)`,
		accountID, tweetID, replyID, time.Now())
	return err
}

// Close closes the database connection
func (s *SQLiteMetricsStore) Close() error {
	return s.db.Close()
}

// Ensure SQLiteMetricsStore implements MetricsStore interface
var _ ports.MetricsStore = (*SQLiteMetricsStore)(nil)
