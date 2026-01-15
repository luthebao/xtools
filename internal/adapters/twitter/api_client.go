package twitter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"xtools/internal/domain"
	"xtools/internal/ports"
)

// decodeBearerToken handles URL-encoded bearer tokens
func decodeBearerToken(token string) string {
	if decoded, err := url.QueryUnescape(token); err == nil {
		return decoded
	}
	return token
}

const (
	baseURL         = "https://api.twitter.com/2"
	searchEndpoint  = "/tweets/search/recent"
	tweetEndpoint   = "/tweets"
	usersMe         = "/users/me"
)

// APIClient implements TwitterClient using Twitter API v2
type APIClient struct {
	credentials  domain.APICredentials
	httpClient   *http.Client
	rateLimitMu  sync.RWMutex
	rateLimit    *domain.RateLimitStatus
	authenticated bool
}

// NewAPIClient creates a new Twitter API v2 client
func NewAPIClient(creds domain.APICredentials) (*APIClient, error) {
	if creds.BearerToken == "" {
		return nil, fmt.Errorf("bearer token is required")
	}

	// Decode URL-encoded bearer token if needed
	creds.BearerToken = decodeBearerToken(creds.BearerToken)

	return &APIClient{
		credentials: creds,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimit: &domain.RateLimitStatus{},
	}, nil
}

// Authenticate verifies credentials are valid
func (c *APIClient) Authenticate(ctx context.Context) error {
	profile, err := c.GetProfile(ctx)
	if err != nil {
		fmt.Printf("[Twitter API] Authentication failed: %v\n", err)
		return fmt.Errorf("authentication failed: %w", err)
	}
	fmt.Printf("[Twitter API] Authenticated as @%s\n", profile.Username)
	c.authenticated = true
	return nil
}

// IsAuthenticated returns whether client is authenticated
func (c *APIClient) IsAuthenticated() bool {
	return c.authenticated
}

// GetAuthType returns the authentication type
func (c *APIClient) GetAuthType() domain.AuthType {
	return domain.AuthTypeAPI
}

// SearchTweets searches for tweets matching query
func (c *APIClient) SearchTweets(ctx context.Context, query string, opts ports.SearchOptions) (*domain.SearchResult, error) {
	params := url.Values{}
	params.Set("query", c.buildSearchQuery(query, opts))
	params.Set("max_results", fmt.Sprintf("%d", min(opts.MaxResults, 100)))
	params.Set("tweet.fields", "id,text,author_id,created_at,conversation_id,public_metrics,lang,referenced_tweets")
	params.Set("user.fields", "id,name,username,description,public_metrics,verified")
	params.Set("expansions", "author_id,referenced_tweets.id")

	if opts.SinceID != "" {
		params.Set("since_id", opts.SinceID)
	}

	endpoint := baseURL + searchEndpoint + "?" + params.Encode()
	fmt.Printf("[Twitter API] Searching: %s\n", c.buildSearchQuery(query, opts))

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		fmt.Printf("[Twitter API] Search error: %v\n", err)
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(searchResp.Errors) > 0 {
		return nil, fmt.Errorf("API error: %s", searchResp.Errors[0].Detail)
	}

	// Build user map for lookups
	users := make(map[string]UserData)
	if searchResp.Includes != nil {
		for _, u := range searchResp.Includes.Users {
			users[u.ID] = u
		}
	}

	// Convert to domain tweets
	tweets := make([]domain.Tweet, 0, len(searchResp.Data))
	for _, t := range searchResp.Data {
		tweets = append(tweets, t.ToDomainTweet(users))
	}

	// Sort by view count if requested
	if opts.SortByViews {
		sort.Slice(tweets, func(i, j int) bool {
			return tweets[i].ViewCount > tweets[j].ViewCount
		})
	}

	result := &domain.SearchResult{
		Tweets:       tweets,
		Query:        query,
		SearchedAt:   time.Now(),
		TotalResults: len(tweets),
	}
	if searchResp.Meta != nil {
		result.NextToken = searchResp.Meta.NextToken
	}

	return result, nil
}

func (c *APIClient) buildSearchQuery(query string, opts ports.SearchOptions) string {
	parts := []string{query}

	if opts.Lang != "" {
		parts = append(parts, fmt.Sprintf("lang:%s", opts.Lang))
	}
	if opts.ExcludeReplies {
		parts = append(parts, "-is:reply")
	}
	if opts.ExcludeRetweets {
		parts = append(parts, "-is:retweet")
	}
	if opts.MinLikes > 0 {
		parts = append(parts, fmt.Sprintf("min_faves:%d", opts.MinLikes))
	}
	if opts.MinRetweets > 0 {
		parts = append(parts, fmt.Sprintf("min_retweets:%d", opts.MinRetweets))
	}

	return strings.Join(parts, " ")
}

// GetTweet retrieves a single tweet by ID
func (c *APIClient) GetTweet(ctx context.Context, tweetID string) (*domain.Tweet, error) {
	params := url.Values{}
	params.Set("tweet.fields", "id,text,author_id,created_at,conversation_id,public_metrics,lang")
	params.Set("user.fields", "id,name,username,description,public_metrics")
	params.Set("expansions", "author_id")

	endpoint := baseURL + tweetEndpoint + "/" + tweetID + "?" + params.Encode()
	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data     TweetData       `json:"data"`
		Includes *SearchIncludes `json:"includes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	users := make(map[string]UserData)
	if result.Includes != nil {
		for _, u := range result.Includes.Users {
			users[u.ID] = u
		}
	}

	tweet := result.Data.ToDomainTweet(users)
	return &tweet, nil
}

// GetTweetThread retrieves the conversation thread for a tweet
func (c *APIClient) GetTweetThread(ctx context.Context, tweetID string) ([]domain.Tweet, error) {
	tweet, err := c.GetTweet(ctx, tweetID)
	if err != nil {
		return nil, err
	}

	if tweet.ConversationID == "" {
		return []domain.Tweet{*tweet}, nil
	}

	// Search for tweets in the conversation
	query := fmt.Sprintf("conversation_id:%s", tweet.ConversationID)
	result, err := c.SearchTweets(ctx, query, ports.SearchOptions{MaxResults: 50})
	if err != nil {
		return []domain.Tweet{*tweet}, nil // Return just the tweet if thread fetch fails
	}

	return result.Tweets, nil
}

// PostReply posts a reply to a tweet
func (c *APIClient) PostReply(ctx context.Context, tweetID string, text string) (*domain.Reply, error) {
	reqBody := CreateTweetRequest{
		Text: text,
		Reply: &ReplyOptions{
			InReplyToTweetID: tweetID,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(ctx, "POST", baseURL+tweetEndpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to post reply: %w", err)
	}
	defer resp.Body.Close()

	var result CreateTweetResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("reply failed: %s", result.Errors[0].Detail)
	}

	now := time.Now()
	return &domain.Reply{
		ID:            result.Data.ID,
		TweetID:       tweetID,
		Text:          text,
		Status:        domain.ReplyStatusPosted,
		PostedAt:      &now,
		PostedReplyID: result.Data.ID,
	}, nil
}

// GetProfile returns the authenticated user's profile
func (c *APIClient) GetProfile(ctx context.Context) (*domain.User, error) {
	params := url.Values{}
	params.Set("user.fields", "id,name,username,description,public_metrics,verified")

	endpoint := baseURL + usersMe + "?" + params.Encode()
	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data UserData `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	user := result.Data.ToDomainUser()
	return &user, nil
}

// GetUser retrieves a user by username
func (c *APIClient) GetUser(ctx context.Context, username string) (*domain.User, error) {
	params := url.Values{}
	params.Set("user.fields", "id,name,username,description,public_metrics,verified")

	endpoint := baseURL + "/users/by/username/" + username + "?" + params.Encode()
	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data UserData `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	user := result.Data.ToDomainUser()
	return &user, nil
}

// GetTweetMetrics retrieves metrics for a tweet
func (c *APIClient) GetTweetMetrics(ctx context.Context, tweetID string) (*domain.TweetMetrics, error) {
	tweet, err := c.GetTweet(ctx, tweetID)
	if err != nil {
		return nil, err
	}

	return &domain.TweetMetrics{
		TweetID:      tweetID,
		Timestamp:    time.Now(),
		LikeCount:    tweet.LikeCount,
		RetweetCount: tweet.RetweetCount,
		ReplyCount:   tweet.ReplyCount,
	}, nil
}

// GetMyTweets retrieves tweets from the authenticated user
func (c *APIClient) GetMyTweets(ctx context.Context, opts ports.PaginationOptions) ([]domain.Tweet, error) {
	profile, err := c.GetProfile(ctx)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("max_results", fmt.Sprintf("%d", min(opts.Limit, 100)))
	params.Set("tweet.fields", "id,text,author_id,created_at,public_metrics")
	if opts.Cursor != "" {
		params.Set("pagination_token", opts.Cursor)
	}

	endpoint := baseURL + "/users/" + profile.ID + "/tweets?" + params.Encode()
	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	tweets := make([]domain.Tweet, 0, len(result.Data))
	for _, t := range result.Data {
		tweets = append(tweets, t.ToDomainTweet(nil))
	}

	return tweets, nil
}

// GetRateLimitStatus returns current rate limit status
func (c *APIClient) GetRateLimitStatus() *domain.RateLimitStatus {
	c.rateLimitMu.RLock()
	defer c.rateLimitMu.RUnlock()
	return c.rateLimit
}

// Close cleans up resources
func (c *APIClient) Close() error {
	return nil
}

func (c *APIClient) doRequest(ctx context.Context, method, reqURL string, body []byte) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		return nil, err
	}

	// Use OAuth 1.0a for POST requests if we have user credentials
	if method == "POST" && hasOAuthCredentials(c.credentials) {
		authHeader := generateOAuthHeader(c.credentials, method, reqURL)
		req.Header.Set("Authorization", authHeader)
		fmt.Printf("[Twitter API] Using OAuth 1.0a for POST request\n")
	} else {
		req.Header.Set("Authorization", "Bearer "+c.credentials.BearerToken)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Update rate limit info from headers
	c.updateRateLimit(resp)

	if resp.StatusCode == 429 {
		return nil, domain.ErrRateLimited
	}

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Provide helpful error messages
		switch resp.StatusCode {
		case 401:
			return nil, fmt.Errorf("authentication failed: invalid or expired credentials")
		case 403:
			return nil, fmt.Errorf("access denied: your API access level may not support this endpoint")
		default:
			return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
		}
	}

	return resp, nil
}

func (c *APIClient) updateRateLimit(resp *http.Response) {
	c.rateLimitMu.Lock()
	defer c.rateLimitMu.Unlock()

	// Parse rate limit headers
	// x-rate-limit-remaining, x-rate-limit-limit, x-rate-limit-reset
}

// Ensure APIClient implements TwitterClient interface
var _ ports.TwitterClient = (*APIClient)(nil)
