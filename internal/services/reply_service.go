package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"xtools/internal/domain"
	"xtools/internal/ports"
)

// ReplyService handles reply generation and posting
type ReplyService struct {
	accountSvc     *AccountService
	searchSvc      *SearchService
	llmFactory     ports.LLMProviderFactory
	replyStore     ports.ReplyStore
	metricsStore   ports.MetricsStore
	eventBus       ports.EventBus
	activityLogger ports.ActivityLogger
}

// NewReplyService creates a new reply service
func NewReplyService(
	accountSvc *AccountService,
	searchSvc *SearchService,
	llmFactory ports.LLMProviderFactory,
	replyStore ports.ReplyStore,
	metricsStore ports.MetricsStore,
	eventBus ports.EventBus,
	activityLogger ports.ActivityLogger,
) *ReplyService {
	return &ReplyService{
		accountSvc:     accountSvc,
		searchSvc:      searchSvc,
		llmFactory:     llmFactory,
		replyStore:     replyStore,
		metricsStore:   metricsStore,
		eventBus:       eventBus,
		activityLogger: activityLogger,
	}
}

// GenerateReply generates a reply for a tweet using LLM
func (s *ReplyService) GenerateReply(ctx context.Context, accountID string, tweet domain.Tweet) (*domain.Reply, error) {
	s.log(accountID, domain.ActivityLevelInfo, "Checking tweet eligibility", fmt.Sprintf("@%s: %s", tweet.AuthorUsername, truncate(tweet.Text, 50)))

	cfg, err := s.accountSvc.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	// Check if already replied
	if replied, _ := s.metricsStore.IsReplied(accountID, tweet.ID); replied {
		s.log(accountID, domain.ActivityLevelWarning, "Already replied to tweet", tweet.ID)
		return nil, domain.ErrAlreadyReplied
	}

	s.log(accountID, domain.ActivityLevelInfo, "Creating LLM provider", cfg.LLMConfig.Model)

	// Create LLM provider
	llm, err := s.llmFactory.CreateProvider(cfg.LLMConfig)
	if err != nil {
		s.log(accountID, domain.ActivityLevelError, "Failed to create LLM provider", err.Error())
		return nil, fmt.Errorf("failed to create LLM provider: %w", err)
	}
	defer llm.Close()

	// Skip slow operations for browser auth (thread context and author bio)
	// These require additional page navigations which are very slow
	var threadContext []domain.Tweet
	var authorBio string

	if cfg.AuthType == domain.AuthTypeAPI {
		// Only fetch thread context for API auth (it's fast)
		s.log(accountID, domain.ActivityLevelInfo, "Fetching thread context", "")
		_, thread, _ := s.searchSvc.GetTweetDetails(ctx, accountID, tweet.ID)
		if len(thread) > 0 {
			threadContext = thread
		}

		// Only fetch author bio for API auth
		s.log(accountID, domain.ActivityLevelInfo, "Fetching author info", tweet.AuthorUsername)
		if author, _ := s.searchSvc.GetAuthorInfo(ctx, accountID, tweet.AuthorUsername); author != nil {
			authorBio = author.Bio
		}
	} else {
		s.log(accountID, domain.ActivityLevelInfo, "Skipping thread/author fetch (browser mode)", "")
	}

	s.log(accountID, domain.ActivityLevelInfo, "Generating reply with LLM", "")

	// Generate reply
	req := ports.ReplyRequest{
		OriginalTweet:   tweet,
		ThreadContext:   threadContext,
		AuthorBio:       authorBio,
		AccountPersona:  cfg.LLMConfig.Persona,
		Keywords:        tweet.MatchedKeywords,
		MaxLength:       cfg.ReplyConfig.MaxReplyLength,
		Tone:            cfg.ReplyConfig.Tone,
		IncludeHashtags: cfg.ReplyConfig.IncludeHashtags,
	}

	resp, err := llm.GenerateReply(ctx, req)
	if err != nil {
		s.log(accountID, domain.ActivityLevelError, "LLM generation failed", err.Error())
		return nil, fmt.Errorf("failed to generate reply: %w", err)
	}

	s.log(accountID, domain.ActivityLevelSuccess, "Reply generated", truncate(resp.Text, 100))

	reply := &domain.Reply{
		ID:            uuid.New().String(),
		TweetID:       tweet.ID,
		AccountID:     accountID,
		Text:          resp.Text,
		GeneratedAt:   time.Now(),
		Status:        domain.ReplyStatusPending,
		LLMTokensUsed: resp.TokensUsed,
	}

	s.eventBus.Emit(ports.EventReplyGenerated, ports.ReplyEvent{
		AccountID: accountID,
		Reply:     reply,
		TweetID:   tweet.ID,
	})

	return reply, nil
}

// log logs an activity if logger is available
func (s *ReplyService) log(accountID string, level domain.ActivityLevel, message, details string) {
	if s.activityLogger != nil {
		s.activityLogger.Log(accountID, domain.ActivityTypeReply, level, message, details)
	}
}

// truncate truncates a string to max length
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// QueueReply adds a reply to the approval queue
func (s *ReplyService) QueueReply(reply domain.Reply, tweet domain.Tweet) error {
	item := domain.ApprovalQueueItem{
		Reply:         reply,
		OriginalTweet: tweet,
		QueuedAt:      time.Now(),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}

	if err := s.replyStore.AddPendingReply(item); err != nil {
		return err
	}

	s.eventBus.Emit(ports.EventApprovalRequired, item)

	return nil
}

// GetPendingReplies returns pending replies for an account
func (s *ReplyService) GetPendingReplies(accountID string) ([]domain.ApprovalQueueItem, error) {
	return s.replyStore.GetPendingReplies(accountID)
}

// ApproveReply approves and posts a pending reply
func (s *ReplyService) ApproveReply(ctx context.Context, replyID string) error {
	// Get the reply from pending queue
	reply, err := s.replyStore.GetReplyByID(replyID)
	if err != nil {
		// Try pending replies
		pending, _ := s.replyStore.GetPendingReplies("")
		for _, p := range pending {
			if p.Reply.ID == replyID {
				reply = &p.Reply
				break
			}
		}
	}

	if reply == nil {
		return fmt.Errorf("reply not found")
	}

	reply.Status = domain.ReplyStatusApproved
	s.replyStore.UpdateReplyStatus(replyID, domain.ReplyStatusApproved)

	s.eventBus.Emit(ports.EventReplyApproved, ports.ReplyEvent{
		AccountID: reply.AccountID,
		Reply:     reply,
		TweetID:   reply.TweetID,
	})

	// Post the reply
	return s.PostReply(ctx, *reply)
}

// RejectReply rejects a pending reply
func (s *ReplyService) RejectReply(replyID string) error {
	if err := s.replyStore.UpdateReplyStatus(replyID, domain.ReplyStatusRejected); err != nil {
		return err
	}

	s.eventBus.Emit(ports.EventReplyRejected, ports.ReplyEvent{
		Reply: &domain.Reply{ID: replyID, Status: domain.ReplyStatusRejected},
	})

	return nil
}

// EditReply updates a pending reply's text
func (s *ReplyService) EditReply(replyID, newText string) error {
	reply, err := s.replyStore.GetReplyByID(replyID)
	if err != nil {
		return err
	}

	reply.Text = newText
	return s.replyStore.SaveReply(*reply)
}

// PostReply posts a reply to Twitter using the configured reply method (API or browser)
func (s *ReplyService) PostReply(ctx context.Context, reply domain.Reply) error {
	// Check if already replied to this tweet (prevent duplicate replies)
	if replied, _ := s.metricsStore.IsReplied(reply.AccountID, reply.TweetID); replied {
		s.log(reply.AccountID, domain.ActivityLevelWarning, "Already replied to this tweet", reply.TweetID)
		return domain.ErrAlreadyReplied
	}

	// Get account config to check reply method
	cfg, err := s.accountSvc.GetAccount(reply.AccountID)
	if err != nil {
		s.log(reply.AccountID, domain.ActivityLevelError, "Failed to get account config", err.Error())
		s.markReplyFailed(reply.ID, err)
		return err
	}

	// Determine which client to use based on reply method setting
	var client ports.TwitterClient
	replyMethod := cfg.ReplyConfig.ReplyMethod
	if replyMethod == "" {
		replyMethod = domain.ReplyMethodAPI // Default to API if not set
	}

	if replyMethod == domain.ReplyMethodBrowser {
		s.log(reply.AccountID, domain.ActivityLevelInfo, "Posting reply via Browser", truncate(reply.Text, 50))
		client, err = s.accountSvc.GetBrowserClientForPosting(reply.AccountID)
	} else {
		s.log(reply.AccountID, domain.ActivityLevelInfo, "Posting reply via API", truncate(reply.Text, 50))
		client, err = s.accountSvc.GetAPIClientForPosting(reply.AccountID)
	}

	if err != nil {
		s.log(reply.AccountID, domain.ActivityLevelError, "Failed to get client for posting", err.Error())
		s.markReplyFailed(reply.ID, err)
		return err
	}
	defer client.Close()

	// Retry loop for rate limiting
	maxRetries := 10
	for attempt := 0; attempt < maxRetries; attempt++ {
		postedReply, err := client.PostReply(ctx, reply.TweetID, reply.Text)
		if err == nil {
			// Success - update reply with posted info
			reply.Status = domain.ReplyStatusPosted
			reply.PostedAt = postedReply.PostedAt
			reply.PostedReplyID = postedReply.PostedReplyID

			s.replyStore.SaveReply(reply)
			s.metricsStore.MarkReplied(reply.AccountID, reply.TweetID, reply.ID)

			s.log(reply.AccountID, domain.ActivityLevelSuccess, "Reply posted successfully", reply.PostedReplyID)

			s.eventBus.Emit(ports.EventReplyPosted, ports.ReplyEvent{
				AccountID: reply.AccountID,
				Reply:     reply,
				TweetID:   reply.TweetID,
			})

			return nil
		}

		// Check if rate limited
		if err == domain.ErrRateLimited {
			// Get rate limit reset time
			rateLimit := client.GetRateLimitStatus()
			waitDuration := 60 * time.Second // Default 1 minute wait

			if rateLimit != nil && !rateLimit.ResetAt.IsZero() {
				waitDuration = time.Until(rateLimit.ResetAt) + time.Second // Add buffer
				if waitDuration < 0 {
					waitDuration = 60 * time.Second
				}
			}

			s.log(reply.AccountID, domain.ActivityLevelWarning,
				fmt.Sprintf("Rate limited, waiting %v (attempt %d/%d)", waitDuration.Round(time.Second), attempt+1, maxRetries),
				"")

			// Wait for rate limit to reset
			select {
			case <-ctx.Done():
				s.log(reply.AccountID, domain.ActivityLevelError, "Context cancelled while waiting for rate limit", "")
				s.markReplyFailed(reply.ID, ctx.Err())
				return ctx.Err()
			case <-time.After(waitDuration):
				// Continue to retry
				continue
			}
		}

		// Non-rate-limit error
		s.log(reply.AccountID, domain.ActivityLevelError, "Failed to post reply", err.Error())
		s.markReplyFailed(reply.ID, err)
		return err
	}

	// Max retries exceeded
	err = fmt.Errorf("max retries exceeded for rate limit")
	s.log(reply.AccountID, domain.ActivityLevelError, "Failed to post reply", err.Error())
	s.markReplyFailed(reply.ID, err)
	return err
}

func (s *ReplyService) markReplyFailed(replyID string, err error) {
	s.replyStore.UpdateReplyStatus(replyID, domain.ReplyStatusFailed)

	s.eventBus.Emit(ports.EventReplyFailed, ports.ReplyEvent{
		Reply: &domain.Reply{ID: replyID, Status: domain.ReplyStatusFailed},
		Error: err.Error(),
	})
}

// GetReplyHistory returns reply history for an account
func (s *ReplyService) GetReplyHistory(accountID string, limit int) ([]domain.Reply, error) {
	return s.replyStore.GetReplies(accountID, limit)
}

// ProcessAutoReply generates and optionally auto-posts a reply
func (s *ReplyService) ProcessAutoReply(ctx context.Context, accountID string, tweet domain.Tweet) error {
	s.log(accountID, domain.ActivityLevelInfo, "Processing tweet", fmt.Sprintf("@%s", tweet.AuthorUsername))

	cfg, err := s.accountSvc.GetAccount(accountID)
	if err != nil {
		s.log(accountID, domain.ActivityLevelError, "Failed to get account config", err.Error())
		return err
	}

	reply, err := s.GenerateReply(ctx, accountID, tweet)
	if err != nil {
		if err == domain.ErrAlreadyReplied {
			return nil // Not an error, just skip
		}
		return err
	}

	// In debug mode, always queue for approval (manual review)
	if cfg.DebugMode {
		s.log(accountID, domain.ActivityLevelInfo, "Queuing reply for approval (debug mode)", truncate(reply.Text, 50))
		return s.QueueReply(*reply, tweet)
	}

	if cfg.ReplyConfig.ApprovalMode == domain.ApprovalModeAuto {
		s.log(accountID, domain.ActivityLevelInfo, "Auto-posting reply (auto mode)", "")
		return s.PostReply(ctx, *reply)
	}

	s.log(accountID, domain.ActivityLevelInfo, "Queuing reply for approval", truncate(reply.Text, 50))
	return s.QueueReply(*reply, tweet)
}
