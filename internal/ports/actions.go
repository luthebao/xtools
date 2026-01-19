package ports

import (
	"context"

	"xtools/internal/domain"
)

// ActionStore manages tweet action persistence and queue
type ActionStore interface {
	// Queue operations
	EnqueueAction(action domain.TweetAction) error
	DequeueActions(accountID string, limit int) ([]domain.TweetAction, error)
	UpdateActionStatus(actionID string, status domain.ActionStatus, errorMsg string) error
	UpdateAction(action domain.TweetAction) error

	// Query operations
	GetAction(actionID string) (*domain.TweetAction, error)
	GetPendingActions(accountID string) ([]domain.TweetAction, error)
	GetRetryableActions(maxRetries int) ([]domain.TweetAction, error)
	GetActionHistory(accountID string, limit int) ([]domain.TweetActionHistory, error)
	GetActionStats(accountID string) (*domain.ActionStats, error)

	// Deduplication
	HasActionForEvent(accountID string, eventID int64) (bool, error)
	MarkActionForEvent(accountID string, eventID int64, actionID string) error
}

// ActionAgent generates tweet content using multi-step LLM pipeline
type ActionAgent interface {
	// GenerateTweet performs the full generation pipeline:
	// 1. Build context from event/profile
	// 2. Load RAG examples (historical + curated)
	// 3. Generate initial draft
	// 4. Review and refine (if enabled)
	// 5. Return final tweet
	GenerateTweet(ctx context.Context, req domain.ActionGenerationRequest) (*domain.ActionGenerationResponse, error)

	// GenerateDraft generates initial tweet draft (single LLM call)
	GenerateDraft(ctx context.Context, req domain.ActionGenerationRequest) (string, int, error)

	// ReviewAndRefine improves the draft (single LLM call)
	ReviewAndRefine(ctx context.Context, draft string, req domain.ActionGenerationRequest) (string, int, error)
}

// ScreenshotCapture captures webpage screenshots
type ScreenshotCapture interface {
	// CaptureMarket captures the Polymarket market page
	CaptureMarket(ctx context.Context, marketSlug string) (imagePath string, err error)

	// CaptureProfile captures the Polymarket wallet profile page
	CaptureProfile(ctx context.Context, walletAddress string) (imagePath string, err error)

	// Close cleans up browser resources
	Close() error
}

// ContentFetcher fetches and parses web content for LLM context
type ContentFetcher interface {
	// FetchMarketContext fetches and extracts relevant context from market page
	FetchMarketContext(ctx context.Context, marketSlug string) (string, error)

	// FetchProfileContext fetches and extracts relevant context from profile page
	FetchProfileContext(ctx context.Context, walletAddress string) (string, error)
}

// TweetPoster posts tweets with optional media attachments
type TweetPoster interface {
	// PostTweet posts a new tweet (not a reply)
	PostTweet(ctx context.Context, text string) (tweetID string, err error)

	// PostTweetWithMedia posts a tweet with an image attachment
	PostTweetWithMedia(ctx context.Context, text string, mediaPath string) (tweetID string, err error)
}
