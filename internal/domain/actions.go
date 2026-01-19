package domain

import "time"

// ActionTriggerType defines when to trigger a tweet action
type ActionTriggerType string

const (
	TriggerFreshInsider   ActionTriggerType = "fresh_insider"    // bet_count <= 3
	TriggerFreshWallet    ActionTriggerType = "fresh_wallet"     // bet_count <= 10
	TriggerBigTrade       ActionTriggerType = "big_trade"        // size >= threshold
	TriggerAnyTrade       ActionTriggerType = "any_trade"        // any trade matching filter
	TriggerCustomBetCount ActionTriggerType = "custom_bet_count" // bet_count <= custom threshold
)

// ActionScreenshotMode defines what to screenshot
type ActionScreenshotMode string

const (
	ScreenshotNone    ActionScreenshotMode = "none"
	ScreenshotMarket  ActionScreenshotMode = "market"
	ScreenshotProfile ActionScreenshotMode = "profile"
)

// ActionStatus tracks the lifecycle of a tweet action
type ActionStatus string

const (
	ActionStatusPending    ActionStatus = "pending"
	ActionStatusFetching   ActionStatus = "fetching"   // Fetching context URLs
	ActionStatusGenerating ActionStatus = "generating" // LLM generating content
	ActionStatusReviewing  ActionStatus = "reviewing"  // LLM reviewing/refining
	ActionStatusCapturing  ActionStatus = "capturing"  // Taking screenshot
	ActionStatusPosting    ActionStatus = "posting"    // Posting to Twitter
	ActionStatusCompleted  ActionStatus = "completed"
	ActionStatusFailed     ActionStatus = "failed"
	ActionStatusQueued     ActionStatus = "queued" // Moved to manual queue after max retries
)

// ActionsConfig holds per-account tweet action settings
type ActionsConfig struct {
	Enabled          bool                 `yaml:"enabled" json:"enabled"`
	TriggerType      ActionTriggerType    `yaml:"trigger_type" json:"triggerType"`
	CustomBetCount   int                  `yaml:"custom_bet_count" json:"customBetCount"`       // For custom_bet_count trigger
	MinTradeSize     float64              `yaml:"min_trade_size" json:"minTradeSize"`           // For big_trade trigger (in USDC)
	ScreenshotMode   ActionScreenshotMode `yaml:"screenshot_mode" json:"screenshotMode"`       // none, market, profile
	CustomPrompt     string               `yaml:"custom_prompt" json:"customPrompt"`           // Override system prompt
	ExampleTweets    []string             `yaml:"example_tweets" json:"exampleTweets"`         // Curated example tweets for RAG
	UseHistorical    bool                 `yaml:"use_historical" json:"useHistorical"`         // Use past tweets as RAG examples
	ReviewEnabled    bool                 `yaml:"review_enabled" json:"reviewEnabled"`         // Enable LLM review step
	MaxRetries       int                  `yaml:"max_retries" json:"maxRetries"`               // Retry attempts before queueing
	RetryBackoffSecs int                  `yaml:"retry_backoff_secs" json:"retryBackoffSecs"` // Base backoff in seconds
}

// DefaultActionsConfig returns a sensible default configuration
func DefaultActionsConfig() ActionsConfig {
	return ActionsConfig{
		Enabled:          false,
		TriggerType:      TriggerFreshInsider,
		CustomBetCount:   5,
		MinTradeSize:     100.0,
		ScreenshotMode:   ScreenshotNone,
		CustomPrompt:     "",
		ExampleTweets:    []string{},
		UseHistorical:    true,
		ReviewEnabled:    true,
		MaxRetries:       3,
		RetryBackoffSecs: 60,
	}
}

// TweetAction represents a queued tweet action
type TweetAction struct {
	ID             string          `json:"id"`
	AccountID      string          `json:"accountId"`
	TriggerType    ActionTriggerType `json:"triggerType"`
	WalletAddress  string          `json:"walletAddress"`
	WalletProfile  *WalletProfile  `json:"walletProfile,omitempty"`
	TradeEvent     *PolymarketEvent `json:"tradeEvent,omitempty"`
	MarketURL      string          `json:"marketUrl"`
	ProfileURL     string          `json:"profileUrl"`
	Status         ActionStatus    `json:"status"`
	DraftText      string          `json:"draftText"`
	ReviewedText   string          `json:"reviewedText"`
	FinalText      string          `json:"finalText"`
	ScreenshotPath string          `json:"screenshotPath"`
	PostedTweetID  string          `json:"postedTweetId"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	ProcessedAt    *time.Time      `json:"processedAt,omitempty"`
	RetryCount     int             `json:"retryCount"`
	NextRetryAt    *time.Time      `json:"nextRetryAt,omitempty"`
	ErrorMessage   string          `json:"errorMessage,omitempty"`
}

// TweetActionHistory represents a simplified view for history display
type TweetActionHistory struct {
	ID            string       `json:"id"`
	AccountID     string       `json:"accountId"`
	WalletAddress string       `json:"walletAddress"`
	MarketName    string       `json:"marketName"`
	TweetText     string       `json:"tweetText"`
	PostedTweetID string       `json:"postedTweetId"`
	Status        ActionStatus `json:"status"`
	CreatedAt     time.Time    `json:"createdAt"`
	ProcessedAt   *time.Time   `json:"processedAt,omitempty"`
	ErrorMessage  string       `json:"errorMessage,omitempty"`
}

// ActionStats provides statistics about tweet actions
type ActionStats struct {
	TotalActions    int `json:"totalActions"`
	PendingCount    int `json:"pendingCount"`
	CompletedCount  int `json:"completedCount"`
	FailedCount     int `json:"failedCount"`
	QueuedCount     int `json:"queuedCount"`
	TotalTokensUsed int `json:"totalTokensUsed"`
}

// ActionGenerationRequest contains context for LLM tweet generation
type ActionGenerationRequest struct {
	WalletProfile  *WalletProfile
	TradeEvent     *PolymarketEvent
	MarketURL      string
	ProfileURL     string
	MarketContext  string // Fetched content from market URL
	ProfileContext string // Fetched content from profile URL
	SystemPrompt   string
	ExampleTweets  []string // Curated examples
	HistoricalTweets []string // Past tweets from this account
	MaxLength      int
}

// ActionGenerationResponse contains LLM generation results
type ActionGenerationResponse struct {
	DraftText    string  `json:"draftText"`
	ReviewedText string  `json:"reviewedText"`
	FinalText    string  `json:"finalText"`
	Reasoning    string  `json:"reasoning"`
	Confidence   float64 `json:"confidence"`
	TokensUsed   int     `json:"tokensUsed"`
}
