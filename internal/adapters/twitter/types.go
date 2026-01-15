package twitter

import (
	"time"
	"xtools/internal/domain"
)

// API v2 response structures

// SearchResponse represents Twitter API v2 search response
type SearchResponse struct {
	Data     []TweetData         `json:"data"`
	Includes *SearchIncludes     `json:"includes,omitempty"`
	Meta     *SearchMeta         `json:"meta,omitempty"`
	Errors   []APIError          `json:"errors,omitempty"`
}

// TweetData represents a tweet from API v2
type TweetData struct {
	ID               string            `json:"id"`
	Text             string            `json:"text"`
	AuthorID         string            `json:"author_id"`
	CreatedAt        string            `json:"created_at"`
	ConversationID   string            `json:"conversation_id,omitempty"`
	InReplyToUserID  string            `json:"in_reply_to_user_id,omitempty"`
	Lang             string            `json:"lang,omitempty"`
	PublicMetrics    *PublicMetrics    `json:"public_metrics,omitempty"`
	ReferencedTweets []ReferencedTweet `json:"referenced_tweets,omitempty"`
}

// PublicMetrics represents tweet engagement metrics
type PublicMetrics struct {
	RetweetCount int `json:"retweet_count"`
	ReplyCount   int `json:"reply_count"`
	LikeCount    int `json:"like_count"`
	QuoteCount   int `json:"quote_count"`
	ViewCount    int `json:"impression_count,omitempty"`
}

// ReferencedTweet represents a referenced tweet
type ReferencedTweet struct {
	Type string `json:"type"` // "replied_to", "quoted", "retweeted"
	ID   string `json:"id"`
}

// SearchIncludes contains expanded data
type SearchIncludes struct {
	Users  []UserData  `json:"users,omitempty"`
	Tweets []TweetData `json:"tweets,omitempty"`
}

// UserData represents a user from API v2
type UserData struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Username        string        `json:"username"`
	Description     string        `json:"description,omitempty"`
	Verified        bool          `json:"verified,omitempty"`
	PublicMetrics   *UserMetrics  `json:"public_metrics,omitempty"`
}

// UserMetrics represents user profile metrics
type UserMetrics struct {
	FollowersCount int `json:"followers_count"`
	FollowingCount int `json:"following_count"`
	TweetCount     int `json:"tweet_count"`
	ListedCount    int `json:"listed_count"`
}

// SearchMeta contains pagination info
type SearchMeta struct {
	NewestID    string `json:"newest_id,omitempty"`
	OldestID    string `json:"oldest_id,omitempty"`
	ResultCount int    `json:"result_count"`
	NextToken   string `json:"next_token,omitempty"`
}

// APIError represents a Twitter API error
type APIError struct {
	Title   string `json:"title"`
	Detail  string `json:"detail"`
	Type    string `json:"type"`
	Status  int    `json:"status,omitempty"`
}

// CreateTweetRequest for posting tweets/replies
type CreateTweetRequest struct {
	Text  string        `json:"text"`
	Reply *ReplyOptions `json:"reply,omitempty"`
}

// ReplyOptions for replying to a tweet
type ReplyOptions struct {
	InReplyToTweetID string `json:"in_reply_to_tweet_id"`
}

// CreateTweetResponse from creating a tweet
type CreateTweetResponse struct {
	Data   *CreatedTweet `json:"data,omitempty"`
	Errors []APIError    `json:"errors,omitempty"`
}

// CreatedTweet represents a newly created tweet
type CreatedTweet struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Helper functions to convert API types to domain types

// ToDomainTweet converts TweetData to domain.Tweet
func (t *TweetData) ToDomainTweet(users map[string]UserData) domain.Tweet {
	tweet := domain.Tweet{
		ID:             t.ID,
		AuthorID:       t.AuthorID,
		Text:           t.Text,
		Language:       t.Lang,
		ConversationID: t.ConversationID,
	}

	// Parse created time
	if t.CreatedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, t.CreatedAt); err == nil {
			tweet.CreatedAt = parsed
		}
	}

	// Add metrics
	if t.PublicMetrics != nil {
		tweet.LikeCount = t.PublicMetrics.LikeCount
		tweet.RetweetCount = t.PublicMetrics.RetweetCount
		tweet.ReplyCount = t.PublicMetrics.ReplyCount
		tweet.ViewCount = t.PublicMetrics.ViewCount
	}

	// Add author info
	if author, ok := users[t.AuthorID]; ok {
		tweet.AuthorUsername = author.Username
		tweet.AuthorName = author.Name
		tweet.AuthorBio = author.Description
	}

	// Add reply reference
	for _, ref := range t.ReferencedTweets {
		if ref.Type == "replied_to" {
			tweet.InReplyToID = ref.ID
			break
		}
	}

	return tweet
}

// ToDomainUser converts UserData to domain.User
func (u *UserData) ToDomainUser() domain.User {
	user := domain.User{
		ID:       u.ID,
		Username: u.Username,
		Name:     u.Name,
		Bio:      u.Description,
		Verified: u.Verified,
	}

	if u.PublicMetrics != nil {
		user.FollowersCount = u.PublicMetrics.FollowersCount
		user.FollowingCount = u.PublicMetrics.FollowingCount
		user.TweetCount = u.PublicMetrics.TweetCount
	}

	return user
}
