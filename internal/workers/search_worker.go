package workers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"xtools/internal/domain"
	"xtools/internal/ports"
	"xtools/internal/services"
)

// SearchWorker runs periodic tweet searches for an account
type SearchWorker struct {
	accountID      string
	searchSvc      *services.SearchService
	replySvc       *services.ReplyService
	configStore    ports.ConfigStore
	rateLimiter    ports.RateLimiter
	eventBus       ports.EventBus
	activityLogger ports.ActivityLogger

	ctx        context.Context
	cancel     context.CancelFunc
	running    bool
	debugMode  bool
	mu         sync.Mutex
	lastSearch time.Time
}

// NewSearchWorker creates a new search worker
func NewSearchWorker(
	accountID string,
	searchSvc *services.SearchService,
	replySvc *services.ReplyService,
	configStore ports.ConfigStore,
	rateLimiter ports.RateLimiter,
	eventBus ports.EventBus,
	activityLogger ports.ActivityLogger,
) *SearchWorker {
	return &SearchWorker{
		accountID:      accountID,
		searchSvc:      searchSvc,
		replySvc:       replySvc,
		configStore:    configStore,
		rateLimiter:    rateLimiter,
		eventBus:       eventBus,
		activityLogger: activityLogger,
	}
}

// Start begins the worker loop
func (w *SearchWorker) Start() error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return domain.ErrWorkerAlreadyRunning
	}

	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.running = true
	w.mu.Unlock()

	go w.run()

	w.eventBus.Emit(ports.EventAccountStarted, ports.AccountStatusEvent{
		AccountID: w.accountID,
		Status:    "started",
		Message:   "Search worker started",
	})

	return nil
}

// Stop stops the worker
func (w *SearchWorker) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return domain.ErrWorkerNotRunning
	}

	w.cancel()
	w.running = false

	w.eventBus.Emit(ports.EventAccountStopped, ports.AccountStatusEvent{
		AccountID: w.accountID,
		Status:    "stopped",
		Message:   "Search worker stopped",
	})

	return nil
}

// IsRunning returns whether the worker is running
func (w *SearchWorker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

func (w *SearchWorker) run() {
	cfg, err := w.configStore.LoadAccount(w.accountID)
	if err != nil {
		w.emitError(err)
		return
	}

	interval := time.Duration(cfg.SearchConfig.IntervalSecs) * time.Second

	// Store debug mode for rate limit bypass
	w.debugMode = cfg.DebugMode

	// In debug mode, use shorter interval (10 seconds minimum)
	if cfg.DebugMode {
		interval = 10 * time.Second
		w.eventBus.Emit(ports.EventWorkerStatus, ports.WorkerStatusEvent{
			AccountID:  w.accountID,
			WorkerType: "search",
			IsRunning:  true,
		})
	} else {
		// Normal mode: enforce minimum 1 minute interval
		if interval < time.Minute {
			interval = time.Minute
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start
	w.doSearch()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.doSearch()
		}
	}
}

func (w *SearchWorker) doSearch() {
	// Skip rate limit check in debug mode
	if !w.debugMode {
		if !w.rateLimiter.TryAcquire() {
			w.activityLogger.Log(w.accountID, domain.ActivityTypeSearch, domain.ActivityLevelWarning, "Rate limit hit, skipping search", "")
			w.eventBus.Emit(ports.EventRateLimitHit, ports.AccountStatusEvent{
				AccountID: w.accountID,
				Status:    "rate_limited",
				Message:   "Search rate limit hit",
			})
			return
		}
	}

	w.activityLogger.Log(w.accountID, domain.ActivityTypeSearch, domain.ActivityLevelInfo, "Starting search", "")
	w.emitStatus("searching")

	// Separate context for search (60 seconds)
	searchCtx, searchCancel := context.WithTimeout(w.ctx, 60*time.Second)
	tweets, err := w.searchSvc.SearchTweets(searchCtx, w.accountID)
	searchCancel()

	if err != nil {
		w.activityLogger.Log(w.accountID, domain.ActivityTypeSearch, domain.ActivityLevelError, "Search failed", err.Error())
		w.emitError(err)
		return
	}

	w.lastSearch = time.Now()
	w.activityLogger.Log(w.accountID, domain.ActivityTypeSearch, domain.ActivityLevelSuccess, fmt.Sprintf("Found %d tweets", len(tweets)), "")
	w.emitStatus("idle")

	// Get delay setting from config
	delayBetweenReplies := 60 * time.Second // Default 60 seconds
	if cfg, err := w.configStore.LoadAccount(w.accountID); err == nil && cfg.RateLimits.MinDelayBetween > 0 {
		delayBetweenReplies = time.Duration(cfg.RateLimits.MinDelayBetween) * time.Second
	}

	// Process tweets for auto-reply with longer timeout per tweet (2 minutes each for LLM + posting)
	for i, tweet := range tweets {
		// Add delay between replies (skip for first tweet)
		if i > 0 {
			w.activityLogger.Log(w.accountID, domain.ActivityTypeWorker, domain.ActivityLevelInfo,
				fmt.Sprintf("Waiting %v before next reply", delayBetweenReplies), "")

			select {
			case <-w.ctx.Done():
				w.activityLogger.Log(w.accountID, domain.ActivityTypeWorker, domain.ActivityLevelWarning, "Worker stopped during delay", "")
				return
			case <-time.After(delayBetweenReplies):
				// Continue after delay
			}
		}

		w.activityLogger.Log(w.accountID, domain.ActivityTypeWorker, domain.ActivityLevelInfo, fmt.Sprintf("Processing tweet %d/%d from @%s", i+1, len(tweets), tweet.AuthorUsername), "")

		replyCtx, replyCancel := context.WithTimeout(w.ctx, 2*time.Minute)
		err := w.replySvc.ProcessAutoReply(replyCtx, w.accountID, tweet)
		replyCancel()

		if err != nil {
			// Error already logged by ReplyService
			continue
		}
	}
	w.activityLogger.Log(w.accountID, domain.ActivityTypeWorker, domain.ActivityLevelSuccess, fmt.Sprintf("Finished processing %d tweets", len(tweets)), "")
}

func (w *SearchWorker) emitStatus(status string) {
	w.eventBus.Emit(ports.EventWorkerStatus, ports.WorkerStatusEvent{
		AccountID:    w.accountID,
		WorkerType:   "search",
		IsRunning:    w.running,
		LastActivity: w.lastSearch.Format(time.RFC3339),
	})
}

func (w *SearchWorker) emitError(err error) {
	w.eventBus.Emit(ports.EventWorkerError, ports.AccountStatusEvent{
		AccountID: w.accountID,
		Status:    "error",
		Error:     err.Error(),
	})
}
