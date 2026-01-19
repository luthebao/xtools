package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"xtools/internal/domain"
	"xtools/internal/ports"
)

// ActionService orchestrates tweet actions from Polymarket events
type ActionService struct {
	mu             sync.RWMutex
	accountSvc     *AccountService
	actionStore    ports.ActionStore
	agent          ports.ActionAgent
	screenshot     ports.ScreenshotCapture
	eventBus       ports.EventBus
	activityLogger ports.ActivityLogger
	stopCh         chan struct{}
	isRunning      bool
}

// NewActionService creates a new action service
func NewActionService(
	accountSvc *AccountService,
	actionStore ports.ActionStore,
	agent ports.ActionAgent,
	screenshot ports.ScreenshotCapture,
	eventBus ports.EventBus,
	activityLogger ports.ActivityLogger,
) *ActionService {
	return &ActionService{
		accountSvc:     accountSvc,
		actionStore:    actionStore,
		agent:          agent,
		screenshot:     screenshot,
		eventBus:       eventBus,
		activityLogger: activityLogger,
	}
}

// Start begins listening for Polymarket events and processing actions
func (s *ActionService) Start() {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return
	}
	s.stopCh = make(chan struct{})
	s.isRunning = true
	s.mu.Unlock()

	log.Println("[ActionService] Starting action service")

	// Subscribe to fresh wallet events
	s.eventBus.Subscribe("polymarket:fresh_wallet_detected", s.onFreshWalletDetected)
	s.eventBus.Subscribe("polymarket:event", s.onPolymarketEvent)

	// Start background worker for processing queue
	go s.processQueueWorker()

	// Start retry worker
	go s.retryWorker()
}

// Stop stops the action service
func (s *ActionService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	log.Println("[ActionService] Stopping action service")
	close(s.stopCh)
	s.isRunning = false
}

// onFreshWalletDetected handles fresh wallet detection events
func (s *ActionService) onFreshWalletDetected(data interface{}) {
	profile, ok := data.(domain.WalletProfile)
	if !ok {
		log.Println("[ActionService] Invalid fresh wallet event data")
		return
	}

	s.processWalletEvent(profile, nil)
}

// onPolymarketEvent handles all Polymarket trade events
func (s *ActionService) onPolymarketEvent(data interface{}) {
	event, ok := data.(domain.PolymarketEvent)
	if !ok {
		return
	}

	// Only process if we have wallet profile attached
	if event.WalletProfile == nil {
		return
	}

	s.processWalletEvent(*event.WalletProfile, &event)
}

func (s *ActionService) processWalletEvent(profile domain.WalletProfile, event *domain.PolymarketEvent) {
	// Get all accounts
	accounts, err := s.accountSvc.ListAccounts()
	if err != nil {
		log.Printf("[ActionService] Failed to list accounts: %v", err)
		return
	}

	for _, acc := range accounts {
		if !acc.Enabled || !acc.ActionsConfig.Enabled {
			continue
		}

		// Check if trigger matches
		if !s.shouldTrigger(acc.ActionsConfig, profile, event) {
			continue
		}

		// Check for duplicate (if we have event ID)
		if event != nil && event.ID > 0 {
			exists, _ := s.actionStore.HasActionForEvent(acc.ID, event.ID)
			if exists {
				continue
			}
		}

		// Create and enqueue action
		s.createAction(acc, profile, event)
	}
}

func (s *ActionService) shouldTrigger(cfg domain.ActionsConfig, profile domain.WalletProfile, event *domain.PolymarketEvent) bool {
	switch cfg.TriggerType {
	case domain.TriggerFreshInsider:
		return profile.FreshnessLevel == domain.FreshnessInsider

	case domain.TriggerFreshWallet:
		return profile.FreshnessLevel == domain.FreshnessInsider ||
			profile.FreshnessLevel == domain.FreshnessWallet

	case domain.TriggerBigTrade:
		if event == nil {
			return false
		}
		notional := parseNotional(event.Price, event.Size)
		return notional >= cfg.MinTradeSize

	case domain.TriggerAnyTrade:
		return event != nil

	case domain.TriggerCustomBetCount:
		return profile.BetCount <= cfg.CustomBetCount

	default:
		return false
	}
}

func (s *ActionService) createAction(acc domain.AccountConfig, profile domain.WalletProfile, event *domain.PolymarketEvent) {
	action := domain.TweetAction{
		ID:            uuid.New().String(),
		AccountID:     acc.ID,
		TriggerType:   acc.ActionsConfig.TriggerType,
		WalletAddress: profile.Address,
		WalletProfile: &profile,
		TradeEvent:    event,
		Status:        domain.ActionStatusPending,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Build URLs
	if event != nil && event.MarketSlug != "" {
		action.MarketURL = fmt.Sprintf("https://polymarket.com/event/%s", event.MarketSlug)
	}
	action.ProfileURL = fmt.Sprintf("https://polymarket.com/profile/%s", profile.Address)

	// Enqueue action
	if err := s.actionStore.EnqueueAction(action); err != nil {
		log.Printf("[ActionService] Failed to enqueue action: %v", err)
		return
	}

	// Mark event as processed (if we have event ID)
	if event != nil && event.ID > 0 {
		s.actionStore.MarkActionForEvent(acc.ID, event.ID, action.ID)
	}

	s.log(acc.ID, domain.ActivityLevelInfo, "Tweet action queued",
		fmt.Sprintf("Wallet: %s, Trigger: %s", shortenAddr(profile.Address), acc.ActionsConfig.TriggerType))

	s.eventBus.Emit("action:queued", action)
}

// processQueueWorker processes pending actions
func (s *ActionService) processQueueWorker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.processPendingActions()
		}
	}
}

func (s *ActionService) processPendingActions() {
	accounts, err := s.accountSvc.ListAccounts()
	if err != nil {
		return
	}

	for _, acc := range accounts {
		if !acc.Enabled || !acc.ActionsConfig.Enabled {
			continue
		}

		// Get one pending action per account
		actions, err := s.actionStore.DequeueActions(acc.ID, 1)
		if err != nil || len(actions) == 0 {
			continue
		}

		for _, action := range actions {
			s.processAction(acc, action)
		}
	}
}

func (s *ActionService) processAction(acc domain.AccountConfig, action domain.TweetAction) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	s.log(acc.ID, domain.ActivityLevelInfo, "Processing tweet action", action.ID)

	// Update status: Fetching context
	action.Status = domain.ActionStatusFetching
	s.actionStore.UpdateAction(action)

	// Build generation request
	req := s.buildGenerationRequest(acc, action)

	// Update status: Generating
	action.Status = domain.ActionStatusGenerating
	s.actionStore.UpdateAction(action)
	s.eventBus.Emit("action:generating", action)

	// Generate tweet
	resp, err := s.agent.GenerateTweet(ctx, req)
	if err != nil {
		s.handleActionError(acc, &action, fmt.Errorf("generation failed: %w", err))
		return
	}

	action.DraftText = resp.DraftText
	action.ReviewedText = resp.ReviewedText
	action.FinalText = resp.FinalText
	s.actionStore.UpdateAction(action)

	// Optional: Capture screenshot
	if acc.ActionsConfig.ScreenshotMode != domain.ScreenshotNone && s.screenshot != nil {
		action.Status = domain.ActionStatusCapturing
		s.actionStore.UpdateAction(action)

		screenshotPath := s.captureScreenshot(ctx, acc.ActionsConfig, action)
		if screenshotPath != "" {
			action.ScreenshotPath = screenshotPath
			s.actionStore.UpdateAction(action)
		}
	}

	// Post tweet
	action.Status = domain.ActionStatusPosting
	s.actionStore.UpdateAction(action)
	s.eventBus.Emit("action:posting", action)

	tweetID, err := s.postTweet(ctx, acc, action.FinalText, action.ScreenshotPath)
	if err != nil {
		s.handleActionError(acc, &action, fmt.Errorf("posting failed: %w", err))
		return
	}

	// Success
	now := time.Now()
	action.Status = domain.ActionStatusCompleted
	action.PostedTweetID = tweetID
	action.ProcessedAt = &now
	action.UpdatedAt = now
	s.actionStore.UpdateAction(action)

	s.log(acc.ID, domain.ActivityLevelSuccess, "Tweet posted successfully", tweetID)
	s.eventBus.Emit("action:completed", action)
}

func (s *ActionService) buildGenerationRequest(acc domain.AccountConfig, action domain.TweetAction) domain.ActionGenerationRequest {
	req := domain.ActionGenerationRequest{
		WalletProfile: action.WalletProfile,
		TradeEvent:    action.TradeEvent,
		MarketURL:     action.MarketURL,
		ProfileURL:    action.ProfileURL,
		SystemPrompt:  acc.ActionsConfig.CustomPrompt,
		ExampleTweets: acc.ActionsConfig.ExampleTweets,
		MaxLength:     280,
	}

	// Load historical tweets if enabled
	if acc.ActionsConfig.UseHistorical {
		history, _ := s.actionStore.GetActionHistory(acc.ID, 5)
		for _, h := range history {
			if h.Status == domain.ActionStatusCompleted && h.TweetText != "" {
				req.HistoricalTweets = append(req.HistoricalTweets, h.TweetText)
			}
		}
	}

	return req
}

func (s *ActionService) captureScreenshot(ctx context.Context, cfg domain.ActionsConfig, action domain.TweetAction) string {
	var path string
	var err error

	switch cfg.ScreenshotMode {
	case domain.ScreenshotMarket:
		if action.TradeEvent != nil && action.TradeEvent.MarketSlug != "" {
			path, err = s.screenshot.CaptureMarket(ctx, action.TradeEvent.MarketSlug)
		}
	case domain.ScreenshotProfile:
		path, err = s.screenshot.CaptureProfile(ctx, action.WalletAddress)
	}

	if err != nil {
		log.Printf("[ActionService] Screenshot capture failed (non-fatal): %v", err)
		return ""
	}
	return path
}

func (s *ActionService) postTweet(ctx context.Context, acc domain.AccountConfig, text, mediaPath string) (string, error) {
	client, err := s.accountSvc.GetAPIClientForPosting(acc.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get Twitter client: %w", err)
	}
	defer client.Close()

	// For now, post without media (Twitter API media upload requires additional implementation)
	// TODO: Implement media upload via TwitterClient.PostTweetWithMedia
	reply, err := client.PostReply(ctx, "", text) // Empty tweet ID = new tweet (need to extend interface)
	if err != nil {
		return "", err
	}

	return reply.PostedReplyID, nil
}

func (s *ActionService) handleActionError(acc domain.AccountConfig, action *domain.TweetAction, err error) {
	action.RetryCount++
	action.ErrorMessage = err.Error()
	action.UpdatedAt = time.Now()

	s.log(acc.ID, domain.ActivityLevelError, "Tweet action failed", err.Error())

	maxRetries := acc.ActionsConfig.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	if action.RetryCount <= maxRetries {
		// Schedule retry with exponential backoff
		backoffSecs := acc.ActionsConfig.RetryBackoffSecs
		if backoffSecs <= 0 {
			backoffSecs = 60
		}
		delay := time.Duration(backoffSecs) * time.Second * time.Duration(1<<(action.RetryCount-1))
		nextRetry := time.Now().Add(delay)
		action.NextRetryAt = &nextRetry
		action.Status = domain.ActionStatusPending

		s.log(acc.ID, domain.ActivityLevelWarning, "Scheduling retry",
			fmt.Sprintf("Attempt %d/%d in %s", action.RetryCount, maxRetries, delay))
	} else {
		// Max retries exceeded
		action.Status = domain.ActionStatusFailed
		s.log(acc.ID, domain.ActivityLevelError, "Max retries exceeded", "")
		s.eventBus.Emit("action:failed", *action)
	}

	s.actionStore.UpdateAction(*action)
}

// retryWorker processes actions that need retry
func (s *ActionService) retryWorker() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.processRetries()
		}
	}
}

func (s *ActionService) processRetries() {
	actions, err := s.actionStore.GetRetryableActions(5)
	if err != nil || len(actions) == 0 {
		return
	}

	for _, action := range actions {
		acc, err := s.accountSvc.GetAccount(action.AccountID)
		if err != nil || acc == nil {
			continue
		}
		s.processAction(*acc, action)
	}
}

func (s *ActionService) log(accountID string, level domain.ActivityLevel, message, details string) {
	if s.activityLogger != nil {
		s.activityLogger.Log(accountID, "action", level, message, details)
	}
}

// GetPendingActions returns pending actions for an account
func (s *ActionService) GetPendingActions(accountID string) ([]domain.TweetAction, error) {
	return s.actionStore.GetPendingActions(accountID)
}

// GetActionHistory returns action history for an account
func (s *ActionService) GetActionHistory(accountID string, limit int) ([]domain.TweetActionHistory, error) {
	return s.actionStore.GetActionHistory(accountID, limit)
}

// GetActionStats returns action statistics for an account
func (s *ActionService) GetActionStats(accountID string) (*domain.ActionStats, error) {
	return s.actionStore.GetActionStats(accountID)
}

// TestAction manually triggers a test action (for debugging)
func (s *ActionService) TestAction(accountID string, profile domain.WalletProfile, event *domain.PolymarketEvent) error {
	acc, err := s.accountSvc.GetAccount(accountID)
	if err != nil {
		return err
	}

	s.createAction(*acc, profile, event)
	return nil
}

func shortenAddr(addr string) string {
	if len(addr) <= 10 {
		return addr
	}
	return addr[:6] + "..." + addr[len(addr)-4:]
}

func parseNotional(price, size string) float64 {
	if price == "" || size == "" {
		return 0
	}
	var p, s float64
	parseFloatVal(price, &p)
	parseFloatVal(size, &s)
	return p * s
}

func parseFloatVal(str string, v *float64) {
	if str == "" {
		*v = 0
		return
	}
	val := 0.0
	multiplier := 1.0
	decimal := false
	decimalPlace := 0.1

	for _, c := range str {
		if c == '-' {
			multiplier = -1
		} else if c == '.' {
			decimal = true
		} else if c >= '0' && c <= '9' {
			digit := float64(c - '0')
			if decimal {
				val += digit * decimalPlace
				decimalPlace /= 10
			} else {
				val = val*10 + digit
			}
		}
	}
	*v = val * multiplier
}
