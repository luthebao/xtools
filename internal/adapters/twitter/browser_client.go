package twitter

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"xtools/internal/domain"
	"xtools/internal/ports"
)

// BrowserClient implements TwitterClient using browser automation
type BrowserClient struct {
	auth          domain.BrowserAuth
	browser       *rod.Browser
	page          *rod.Page
	authenticated bool
	rateLimitMu   sync.RWMutex
	rateLimit     *domain.RateLimitStatus
}

// NewBrowserClient creates a new browser-based Twitter client
func NewBrowserClient(auth domain.BrowserAuth) (*BrowserClient, error) {
	return &BrowserClient{
		auth:      auth,
		rateLimit: &domain.RateLimitStatus{},
	}, nil
}

// Authenticate sets up browser with cookies and verifies login
func (c *BrowserClient) Authenticate(ctx context.Context) error {
	// Launch browser
	l := launcher.New().Headless(true)
	if c.auth.ProxyURL != "" {
		l = l.Proxy(c.auth.ProxyURL)
	}

	controlURL, err := l.Launch()
	if err != nil {
		return fmt.Errorf("failed to launch browser: %w", err)
	}

	c.browser = rod.New().ControlURL(controlURL)
	if err := c.browser.Connect(); err != nil {
		return fmt.Errorf("failed to connect to browser: %w", err)
	}

	c.page, err = c.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return fmt.Errorf("failed to create page: %w", err)
	}

	// Set user agent if provided
	if c.auth.UserAgent != "" {
		if err := c.page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
			UserAgent: c.auth.UserAgent,
		}); err != nil {
			return fmt.Errorf("failed to set user agent: %w", err)
		}
	}

	// Set cookies
	for _, cookie := range c.auth.Cookies {
		err := c.browser.SetCookies([]*proto.NetworkCookieParam{{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   cookie.Domain,
			Path:     cookie.Path,
			Expires:  proto.TimeSinceEpoch(cookie.Expires),
			Secure:   cookie.Secure,
			HTTPOnly: cookie.HttpOnly,
		}})
		if err != nil {
			return fmt.Errorf("failed to set cookie %s: %w", cookie.Name, err)
		}
	}

	// Navigate to X/Twitter and verify login
	if err := c.page.Navigate("https://x.com/home"); err != nil {
		return fmt.Errorf("failed to navigate: %w", err)
	}

	if err := c.page.WaitLoad(); err != nil {
		return fmt.Errorf("failed to wait for load: %w", err)
	}

	// Check if logged in by looking for compose tweet button or timeline
	fmt.Printf("[browser] Checking authentication...\n")
	_, err = c.page.Timeout(10 * time.Second).Element("[data-testid='tweetButtonInline'], [data-testid='primaryColumn']")
	if err != nil {
		// Try alternative selectors
		_, err2 := c.page.Timeout(5 * time.Second).Element("[data-testid='SideNav_AccountSwitcher_Button']")
		if err2 != nil {
			fmt.Printf("[browser] Auth verification failed\n")
			return fmt.Errorf("login verification failed - cookies may be expired: %w", err)
		}
	}

	fmt.Printf("[browser] Authentication successful\n")
	c.authenticated = true
	return nil
}

// IsAuthenticated returns whether client is authenticated
func (c *BrowserClient) IsAuthenticated() bool {
	return c.authenticated
}

// GetAuthType returns the authentication type
func (c *BrowserClient) GetAuthType() domain.AuthType {
	return domain.AuthTypeBrowser
}

// SearchTweets searches for tweets using browser automation
func (c *BrowserClient) SearchTweets(ctx context.Context, query string, opts ports.SearchOptions) (*domain.SearchResult, error) {
	if !c.authenticated {
		return nil, domain.ErrNotAuthenticated
	}

	// Build search URL
	searchURL := c.buildSearchURL(query, opts)
	fmt.Printf("[browser] Navigating to: %s\n", searchURL)

	if err := c.page.Navigate(searchURL); err != nil {
		return nil, fmt.Errorf("failed to navigate to search: %w", err)
	}

	if err := c.page.WaitLoad(); err != nil {
		return nil, err
	}

	// Wait for tweets to load - try to find tweet elements with timeout
	fmt.Printf("[browser] Page loaded, waiting for tweets...\n")

	// Wait for tweet elements to appear (up to 15 seconds)
	var tweetEl *rod.Element
	for i := 0; i < 15; i++ {
		tweetEl, _ = c.page.Timeout(1 * time.Second).Element("[data-testid='tweet']")
		if tweetEl != nil {
			fmt.Printf("[browser] Tweets appeared after %d seconds\n", i+1)
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Additional wait for more tweets to load
	time.Sleep(2 * time.Second)

	// Try to find any content on page for debugging
	html, _ := c.page.HTML()
	if len(html) > 500 {
		fmt.Printf("[browser] Page HTML length: %d\n", len(html))
	}

	// Extract tweets from page
	tweets, err := c.extractTweets(opts.MaxResults)
	if err != nil {
		return nil, fmt.Errorf("failed to extract tweets: %w", err)
	}

	fmt.Printf("[browser] Extracted %d tweets for query: %s\n", len(tweets), query)

	return &domain.SearchResult{
		Tweets:       tweets,
		Query:        query,
		SearchedAt:   time.Now(),
		TotalResults: len(tweets),
	}, nil
}

func (c *BrowserClient) buildSearchURL(query string, opts ports.SearchOptions) string {
	params := url.Values{}
	params.Set("q", query)
	params.Set("f", "live") // Latest tweets

	if opts.Lang != "" {
		params.Set("lang", opts.Lang)
	}

	return "https://x.com/search?" + params.Encode()
}

func (c *BrowserClient) extractTweets(maxResults int) ([]domain.Tweet, error) {
	var tweets []domain.Tweet

	// Find tweet articles
	elements, err := c.page.Elements("[data-testid='tweet']")
	if err != nil {
		fmt.Printf("[browser] Error finding tweet elements: %v\n", err)
		return nil, err
	}

	fmt.Printf("[browser] Found %d tweet elements\n", len(elements))

	for i, el := range elements {
		if i >= maxResults {
			break
		}

		tweet, err := c.parseTweetElement(el)
		if err != nil {
			fmt.Printf("[browser] Failed to parse tweet %d: %v\n", i, err)
			continue // Skip tweets that can't be parsed
		}
		tweets = append(tweets, tweet)
	}

	return tweets, nil
}

func (c *BrowserClient) parseTweetElement(el *rod.Element) (domain.Tweet, error) {
	tweet := domain.Tweet{
		DiscoveredAt: time.Now(),
		CreatedAt:    time.Now(), // Set to now since we're searching live/recent tweets
	}

	// Extract tweet text
	textEl, err := el.Element("[data-testid='tweetText']")
	if err == nil {
		tweet.Text, _ = textEl.Text()
	}

	// Extract author username from link
	userLink, err := el.Element("[data-testid='User-Name'] a[href^='/']")
	if err == nil {
		href, _ := userLink.Attribute("href")
		if href != nil && *href != "" {
			parts := strings.Split(*href, "/")
			if len(parts) > 1 {
				tweet.AuthorUsername = parts[1]
			}
		}
	}

	// Extract tweet ID from status link
	statusLink, err := el.Element("a[href*='/status/']")
	if err == nil {
		href, _ := statusLink.Attribute("href")
		if href != nil {
			re := regexp.MustCompile(`/status/(\d+)`)
			matches := re.FindStringSubmatch(*href)
			if len(matches) > 1 {
				tweet.ID = matches[1]
			}
		}
	}

	// Extract metrics
	tweet.LikeCount = c.extractMetric(el, "[data-testid='like']")
	tweet.RetweetCount = c.extractMetric(el, "[data-testid='retweet']")
	tweet.ReplyCount = c.extractMetric(el, "[data-testid='reply']")

	if tweet.ID == "" {
		return tweet, fmt.Errorf("could not extract tweet ID")
	}

	return tweet, nil
}

func (c *BrowserClient) extractMetric(el *rod.Element, selector string) int {
	metricEl, err := el.Element(selector)
	if err != nil {
		return 0
	}

	text, err := metricEl.Text()
	if err != nil {
		return 0
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	// Handle K/M suffixes
	text = strings.ToUpper(text)
	multiplier := 1
	if strings.HasSuffix(text, "K") {
		multiplier = 1000
		text = strings.TrimSuffix(text, "K")
	} else if strings.HasSuffix(text, "M") {
		multiplier = 1000000
		text = strings.TrimSuffix(text, "M")
	}

	val, _ := strconv.ParseFloat(text, 64)
	return int(val * float64(multiplier))
}

// GetTweet retrieves a single tweet by ID
func (c *BrowserClient) GetTweet(ctx context.Context, tweetID string) (*domain.Tweet, error) {
	// Navigate to tweet page and extract
	url := fmt.Sprintf("https://x.com/i/web/status/%s", tweetID)
	if err := c.page.Navigate(url); err != nil {
		return nil, err
	}
	if err := c.page.WaitLoad(); err != nil {
		return nil, err
	}

	time.Sleep(2 * time.Second)

	el, err := c.page.Element("[data-testid='tweet']")
	if err != nil {
		return nil, domain.ErrTweetNotFound
	}

	tweet, err := c.parseTweetElement(el)
	if err != nil {
		return nil, err
	}

	return &tweet, nil
}

// GetTweetThread retrieves conversation thread
func (c *BrowserClient) GetTweetThread(ctx context.Context, tweetID string) ([]domain.Tweet, error) {
	tweet, err := c.GetTweet(ctx, tweetID)
	if err != nil {
		return nil, err
	}
	return []domain.Tweet{*tweet}, nil
}

// PostReply posts a reply to a tweet
func (c *BrowserClient) PostReply(ctx context.Context, tweetID string, text string) (*domain.Reply, error) {
	// Navigate to tweet
	url := fmt.Sprintf("https://x.com/i/web/status/%s", tweetID)
	if err := c.page.Navigate(url); err != nil {
		return nil, err
	}
	if err := c.page.WaitLoad(); err != nil {
		return nil, err
	}

	time.Sleep(2 * time.Second)

	// Click reply button
	replyBtn, err := c.page.Element("[data-testid='reply']")
	if err != nil {
		return nil, fmt.Errorf("reply button not found: %w", err)
	}
	if err := replyBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return nil, err
	}

	time.Sleep(1 * time.Second)

	// Type reply text
	textBox, err := c.page.Element("[data-testid='tweetTextarea_0']")
	if err != nil {
		return nil, fmt.Errorf("reply text box not found: %w", err)
	}
	if err := textBox.Input(text); err != nil {
		return nil, err
	}

	time.Sleep(500 * time.Millisecond)

	// Click post button
	postBtn, err := c.page.Element("[data-testid='tweetButton']")
	if err != nil {
		return nil, fmt.Errorf("post button not found: %w", err)
	}
	if err := postBtn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return nil, err
	}

	time.Sleep(2 * time.Second)

	now := time.Now()
	return &domain.Reply{
		TweetID:  tweetID,
		Text:     text,
		Status:   domain.ReplyStatusPosted,
		PostedAt: &now,
	}, nil
}

// GetProfile returns the authenticated user's profile
func (c *BrowserClient) GetProfile(ctx context.Context) (*domain.User, error) {
	// This would navigate to profile and scrape
	return nil, fmt.Errorf("GetProfile not implemented for browser client")
}

// GetUser retrieves a user by username
func (c *BrowserClient) GetUser(ctx context.Context, username string) (*domain.User, error) {
	url := fmt.Sprintf("https://x.com/%s", username)
	if err := c.page.Navigate(url); err != nil {
		return nil, err
	}
	if err := c.page.WaitLoad(); err != nil {
		return nil, err
	}

	time.Sleep(2 * time.Second)

	user := &domain.User{Username: username}

	// Extract follower count
	// This is simplified - actual implementation would need proper selectors

	return user, nil
}

// GetTweetMetrics retrieves metrics for a tweet
func (c *BrowserClient) GetTweetMetrics(ctx context.Context, tweetID string) (*domain.TweetMetrics, error) {
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

// GetMyTweets retrieves tweets from authenticated user
func (c *BrowserClient) GetMyTweets(ctx context.Context, opts ports.PaginationOptions) ([]domain.Tweet, error) {
	return nil, fmt.Errorf("GetMyTweets not implemented for browser client")
}

// GetRateLimitStatus returns current rate limit status
func (c *BrowserClient) GetRateLimitStatus() *domain.RateLimitStatus {
	c.rateLimitMu.RLock()
	defer c.rateLimitMu.RUnlock()
	return c.rateLimit
}

// Close cleans up browser resources
func (c *BrowserClient) Close() error {
	if c.page != nil {
		c.page.Close()
	}
	if c.browser != nil {
		return c.browser.Close()
	}
	return nil
}

// Ensure BrowserClient implements TwitterClient interface
var _ ports.TwitterClient = (*BrowserClient)(nil)
