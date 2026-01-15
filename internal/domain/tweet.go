package domain

import "time"

// Tweet represents a Twitter post
type Tweet struct {
	ID             string    `json:"id"`
	AuthorID       string    `json:"authorId"`
	AuthorUsername string    `json:"authorUsername"`
	AuthorName     string    `json:"authorName"`
	AuthorBio      string    `json:"authorBio"`
	Text           string    `json:"text"`
	CreatedAt      time.Time `json:"createdAt"`
	Language       string    `json:"language"`

	// Engagement metrics at discovery time
	LikeCount    int `json:"likeCount"`
	RetweetCount int `json:"retweetCount"`
	ReplyCount   int `json:"replyCount"`
	ViewCount    int `json:"viewCount"`

	// Thread context
	ConversationID string  `json:"conversationId"`
	InReplyToID    string  `json:"inReplyToId,omitempty"`
	ThreadTweets   []Tweet `json:"threadTweets,omitempty"`

	// Search context
	MatchedKeywords []string  `json:"matchedKeywords"`
	DiscoveredAt    time.Time `json:"discoveredAt"`
	AccountID       string    `json:"accountId"`
}

// User represents a Twitter user profile
type User struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	Name           string `json:"name"`
	Bio            string `json:"bio"`
	FollowersCount int    `json:"followersCount"`
	FollowingCount int    `json:"followingCount"`
	TweetCount     int    `json:"tweetCount"`
	Verified       bool   `json:"verified"`
}

// ReplyStatus defines the status of a generated reply
type ReplyStatus string

const (
	ReplyStatusPending  ReplyStatus = "pending"
	ReplyStatusApproved ReplyStatus = "approved"
	ReplyStatusPosted   ReplyStatus = "posted"
	ReplyStatusRejected ReplyStatus = "rejected"
	ReplyStatusFailed   ReplyStatus = "failed"
)

// Reply represents a reply to a tweet
type Reply struct {
	ID            string      `json:"id"`
	TweetID       string      `json:"tweetId"`
	AccountID     string      `json:"accountId"`
	Text          string      `json:"text"`
	GeneratedAt   time.Time   `json:"generatedAt"`
	PostedAt      *time.Time  `json:"postedAt,omitempty"`
	Status        ReplyStatus `json:"status"`
	LLMTokensUsed int         `json:"llmTokensUsed"`
	ErrorMessage  string      `json:"errorMessage,omitempty"`
	PostedReplyID string      `json:"postedReplyId,omitempty"`
}

// ApprovalQueueItem represents an item in the approval queue
type ApprovalQueueItem struct {
	Reply         Reply     `json:"reply"`
	OriginalTweet Tweet     `json:"originalTweet"`
	QueuedAt      time.Time `json:"queuedAt"`
	ExpiresAt     time.Time `json:"expiresAt"`
}

// SearchResult represents search results with metadata
type SearchResult struct {
	Tweets       []Tweet   `json:"tweets"`
	Query        string    `json:"query"`
	SearchedAt   time.Time `json:"searchedAt"`
	NextToken    string    `json:"nextToken,omitempty"`
	TotalResults int       `json:"totalResults"`
}

// RateLimitStatus represents the current rate limit state
type RateLimitStatus struct {
	Remaining  int       `json:"remaining"`
	Limit      int       `json:"limit"`
	ResetAt    time.Time `json:"resetAt"`
	IsLimited  bool      `json:"isLimited"`
}
