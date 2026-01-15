package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"xtools/internal/domain"
	"xtools/internal/ports"
)

// SearchService handles tweet searching and filtering
type SearchService struct {
	accountSvc    *AccountService
	metricsStore  ports.MetricsStore
	excelExporter ports.ExcelExporter
	eventBus      ports.EventBus
}

// NewSearchService creates a new search service
func NewSearchService(
	accountSvc *AccountService,
	metricsStore ports.MetricsStore,
	excelExporter ports.ExcelExporter,
	eventBus ports.EventBus,
) *SearchService {
	return &SearchService{
		accountSvc:    accountSvc,
		metricsStore:  metricsStore,
		excelExporter: excelExporter,
		eventBus:      eventBus,
	}
}

// SearchTweets searches for tweets matching account's keywords
func (s *SearchService) SearchTweets(ctx context.Context, accountID string) ([]domain.Tweet, error) {
	cfg, err := s.accountSvc.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	client, err := s.accountSvc.GetClient(accountID)
	if err != nil {
		return nil, err
	}

	if len(cfg.SearchConfig.Keywords) == 0 {
		return nil, nil
	}

	// Build query based on auth type
	var query string
	if cfg.AuthType == domain.AuthTypeBrowser {
		// Browser query format: (keyword1 OR keyword2 OR keyword3) min_faves:10 until:YYYY-MM-DD
		query = s.buildBrowserSearchQuery(cfg)
	} else {
		// API query format: keyword1 OR keyword2 OR keyword3
		query = strings.Join(cfg.SearchConfig.Keywords, " OR ")
	}

	opts := ports.SearchOptions{
		MaxResults:      100,
		Lang:            "en",
		ExcludeRetweets: true,
		SortByViews:     true,
	}

	if cfg.SearchConfig.EnglishOnly {
		opts.Lang = "en"
	}

	result, err := client.SearchTweets(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	var allTweets []domain.Tweet

	// Add metadata to tweets and match keywords
	for i := range result.Tweets {
		tweet := &result.Tweets[i]
		tweet.AccountID = accountID
		tweet.DiscoveredAt = time.Now()

		// Find which keywords matched this tweet
		textLower := strings.ToLower(tweet.Text)
		for _, kw := range cfg.SearchConfig.Keywords {
			// Handle keywords with search operators (e.g., "insider -filter:replies")
			kwBase := strings.Split(kw, " ")[0] // Get first word before operators
			if strings.Contains(textLower, strings.ToLower(kwBase)) {
				tweet.MatchedKeywords = append(tweet.MatchedKeywords, kw)
			}
		}

		allTweets = append(allTweets, *tweet)
	}

	// Filter tweets
	filtered := s.filterTweets(cfg, allTweets)

	// Sort by view count (highest first)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ViewCount > filtered[j].ViewCount
	})

	// Save to Excel
	if len(filtered) > 0 {
		if err := s.excelExporter.AppendTweets(accountID, filtered); err != nil {
			// Log but don't fail
		}
	}

	// Emit events for found tweets
	for _, tweet := range filtered {
		s.eventBus.Emit(ports.EventTweetFound, ports.TweetFoundEvent{
			AccountID: accountID,
			Tweet:     tweet,
			Keyword:   strings.Join(tweet.MatchedKeywords, ", "),
		})
	}

	return filtered, nil
}

func (s *SearchService) filterTweets(cfg *domain.AccountConfig, tweets []domain.Tweet) []domain.Tweet {
	var filtered []domain.Tweet
	seen := make(map[string]bool)

	for _, tweet := range tweets {
		// Skip duplicates
		if seen[tweet.ID] {
			continue
		}
		seen[tweet.ID] = true

		// Skip already replied tweets
		if replied, _ := s.metricsStore.IsReplied(cfg.ID, tweet.ID); replied {
			continue
		}

		// Skip blocklisted users
		if s.isBlocklisted(cfg.SearchConfig.Blocklist, tweet.AuthorUsername) {
			continue
		}

		// Skip tweets with excluded keywords
		if s.containsExcludedKeywords(cfg.SearchConfig.ExcludeKeywords, tweet.Text) {
			continue
		}

		// Check English only (basic check - assumes API already filtered)
		if cfg.SearchConfig.EnglishOnly && tweet.Language != "" && tweet.Language != "en" {
			continue
		}

		// Check minimum age
		if cfg.SearchConfig.MaxAgeMins > 0 {
			age := time.Since(tweet.CreatedAt)
			if age.Minutes() > float64(cfg.SearchConfig.MaxAgeMins) {
				continue
			}
		}

		filtered = append(filtered, tweet)
	}

	return filtered
}

func (s *SearchService) isBlocklisted(blocklist []string, username string) bool {
	username = strings.ToLower(username)
	for _, blocked := range blocklist {
		if strings.ToLower(blocked) == username {
			return true
		}
	}
	return false
}

func (s *SearchService) containsExcludedKeywords(excluded []string, text string) bool {
	textLower := strings.ToLower(text)
	for _, kw := range excluded {
		if strings.Contains(textLower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// buildBrowserSearchQuery builds a search query optimized for browser-based search
// Format: (keyword1 OR keyword2 OR keyword3) min_faves:N min_replies:N min_retweets:N until:UNIX_TIMESTAMP -filter:replies
func (s *SearchService) buildBrowserSearchQuery(cfg *domain.AccountConfig) string {
	// Build keywords with OR inside parentheses
	keywords := "(" + strings.Join(cfg.SearchConfig.Keywords, " OR ") + ")"

	// Build filters
	var filters []string

	// min_faves filter 
	minFaves := cfg.SearchConfig.MinFaves
	filters = append(filters, fmt.Sprintf("min_faves:%d", minFaves))

	// min_replies filter
	minReplies := cfg.SearchConfig.MinReplies
	filters = append(filters, fmt.Sprintf("min_replies:%d", minReplies))

	// min_retweets filter
	minRetweets := cfg.SearchConfig.MinRetweets
	filters = append(filters, fmt.Sprintf("min_retweets:%d", minRetweets))

	// until: current time minus 10 minutes (to avoid very recent tweets that may be spam)
	untilTime := time.Now().Add(-10 * time.Minute)
	filters = append(filters, fmt.Sprintf("until:%d", untilTime.Unix()))

	// Exclude replies
	filters = append(filters, "-filter:replies")

	query := fmt.Sprintf("%s %s", keywords, strings.Join(filters, " "))
	return query
}

// GetTweetDetails retrieves full tweet details including thread
func (s *SearchService) GetTweetDetails(ctx context.Context, accountID, tweetID string) (*domain.Tweet, []domain.Tweet, error) {
	client, err := s.accountSvc.GetClient(accountID)
	if err != nil {
		return nil, nil, err
	}

	tweet, err := client.GetTweet(ctx, tweetID)
	if err != nil {
		return nil, nil, err
	}

	thread, err := client.GetTweetThread(ctx, tweetID)
	if err != nil {
		return tweet, nil, nil
	}

	return tweet, thread, nil
}

// GetAuthorInfo retrieves author information for a tweet
func (s *SearchService) GetAuthorInfo(ctx context.Context, accountID, username string) (*domain.User, error) {
	client, err := s.accountSvc.GetClient(accountID)
	if err != nil {
		return nil, err
	}

	return client.GetUser(ctx, username)
}

// ManualSearch performs a manual search with custom query
func (s *SearchService) ManualSearch(ctx context.Context, accountID, query string, maxResults int) ([]domain.Tweet, error) {
	client, err := s.accountSvc.GetClient(accountID)
	if err != nil {
		return nil, err
	}

	opts := ports.SearchOptions{
		MaxResults:      maxResults,
		ExcludeRetweets: true,
		SortByViews:     true,
	}

	result, err := client.SearchTweets(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	return result.Tweets, nil
}
