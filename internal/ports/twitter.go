package ports

import (
	"context"
	"xtools/internal/domain"
)

// TwitterClient abstracts Twitter interaction (API or Browser)
type TwitterClient interface {
	// Authentication
	Authenticate(ctx context.Context) error
	IsAuthenticated() bool
	GetAuthType() domain.AuthType

	// Search
	SearchTweets(ctx context.Context, query string, opts SearchOptions) (*domain.SearchResult, error)

	// Tweet Operations
	GetTweet(ctx context.Context, tweetID string) (*domain.Tweet, error)
	GetTweetThread(ctx context.Context, tweetID string) ([]domain.Tweet, error)
	PostReply(ctx context.Context, tweetID string, text string) (*domain.Reply, error)

	// Profile & Metrics
	GetProfile(ctx context.Context) (*domain.User, error)
	GetUser(ctx context.Context, username string) (*domain.User, error)
	GetTweetMetrics(ctx context.Context, tweetID string) (*domain.TweetMetrics, error)
	GetMyTweets(ctx context.Context, opts PaginationOptions) ([]domain.Tweet, error)

	// Rate Limiting
	GetRateLimitStatus() *domain.RateLimitStatus

	// Cleanup
	Close() error
}

// SearchOptions configures tweet search behavior
type SearchOptions struct {
	MaxResults     int
	SinceID        string
	Lang           string // "en" for English-only
	ExcludeReplies bool
	ExcludeRetweets bool
	MinLikes       int
	MinRetweets    int
	SortByViews    bool // prioritize high-view tweets
}

// PaginationOptions configures paginated requests
type PaginationOptions struct {
	Limit  int
	Cursor string
}

// TwitterClientFactory creates appropriate client based on config
type TwitterClientFactory interface {
	CreateClient(cfg domain.AccountConfig) (TwitterClient, error)
}
