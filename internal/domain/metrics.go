package domain

import "time"

// ProfileSnapshot represents a point-in-time profile metrics snapshot
type ProfileSnapshot struct {
	AccountID      string    `json:"accountId"`
	Timestamp      time.Time `json:"timestamp"`
	FollowersCount int       `json:"followersCount"`
	FollowingCount int       `json:"followingCount"`
	TweetCount     int       `json:"tweetCount"`
	ListedCount    int       `json:"listedCount"`
}

// TweetMetrics represents metrics for a single tweet
type TweetMetrics struct {
	TweetID      string    `json:"tweetId"`
	AccountID    string    `json:"accountId"`
	Timestamp    time.Time `json:"timestamp"`
	Impressions  int       `json:"impressions"`
	Engagements  int       `json:"engagements"`
	LikeCount    int       `json:"likeCount"`
	RetweetCount int       `json:"retweetCount"`
	ReplyCount   int       `json:"replyCount"`
	QuoteCount   int       `json:"quoteCount"`
	ClickCount   int       `json:"clickCount"`
}

// ReplyMetrics represents metrics for a reply
type ReplyMetrics struct {
	ReplyID         string    `json:"replyId"`
	AccountID       string    `json:"accountId"`
	OriginalTweetID string    `json:"originalTweetId"`
	Timestamp       time.Time `json:"timestamp"`
	LikeCount       int       `json:"likeCount"`
	RetweetCount    int       `json:"retweetCount"`
	Impressions     int       `json:"impressions"`
}

// ProfileGrowthMetrics represents follower growth over a period
type ProfileGrowthMetrics struct {
	FollowersGained int     `json:"followersGained"`
	FollowersLost   int     `json:"followersLost"`
	NetChange       int     `json:"netChange"`
	GrowthRate      float64 `json:"growthRate"`
}

// ReplyPerformanceReport represents reply analytics
type ReplyPerformanceReport struct {
	AccountID              string         `json:"accountId"`
	Period                 string         `json:"period"`
	TotalReplies           int            `json:"totalReplies"`
	SuccessfulReplies      int            `json:"successfulReplies"`
	FailedReplies          int            `json:"failedReplies"`
	PendingReplies         int            `json:"pendingReplies"`
	AvgLikesPerReply       float64        `json:"avgLikesPerReply"`
	AvgImpressionsPerReply float64        `json:"avgImpressionsPerReply"`
	TopPerformingReplies   []ReplyMetrics `json:"topPerformingReplies"`
}

// MetricsReport represents a comprehensive metrics report
type MetricsReport struct {
	AccountID        string                 `json:"accountId"`
	GeneratedAt      time.Time              `json:"generatedAt"`
	ProfileGrowth    ProfileGrowthMetrics   `json:"profileGrowth"`
	ReplyPerformance ReplyPerformanceReport `json:"replyPerformance"`
	TopTweets        []TweetMetrics         `json:"topTweets"`
}

// DailyStats represents daily statistics for an account
type DailyStats struct {
	AccountID       string    `json:"accountId"`
	Date            time.Time `json:"date"`
	TweetsSearched  int       `json:"tweetsSearched"`
	RepliesGenerated int       `json:"repliesGenerated"`
	RepliesSent     int       `json:"repliesSent"`
	RepliesFailed   int       `json:"repliesFailed"`
	TokensUsed      int       `json:"tokensUsed"`
}
