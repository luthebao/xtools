package workers

import (
	"sync"
	"time"

	"xtools/internal/adapters/ratelimit"
	"xtools/internal/domain"
	"xtools/internal/ports"
	"xtools/internal/services"
)

// WorkerPool manages workers for all accounts
type WorkerPool struct {
	searchSvc      *services.SearchService
	replySvc       *services.ReplyService
	configStore    ports.ConfigStore
	eventBus       ports.EventBus
	activityLogger ports.ActivityLogger

	mu      sync.RWMutex
	workers map[string]*SearchWorker
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(
	searchSvc *services.SearchService,
	replySvc *services.ReplyService,
	configStore ports.ConfigStore,
	eventBus ports.EventBus,
	activityLogger ports.ActivityLogger,
) *WorkerPool {
	return &WorkerPool{
		searchSvc:      searchSvc,
		replySvc:       replySvc,
		configStore:    configStore,
		eventBus:       eventBus,
		activityLogger: activityLogger,
		workers:        make(map[string]*SearchWorker),
	}
}

// StartWorker starts a worker for an account
func (p *WorkerPool) StartWorker(accountID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if worker, exists := p.workers[accountID]; exists && worker.IsRunning() {
		return domain.ErrWorkerAlreadyRunning
	}

	cfg, err := p.configStore.LoadAccount(accountID)
	if err != nil {
		return err
	}

	// Create rate limiter for this account
	rateLimiter := ratelimit.NewTokenBucket(
		cfg.RateLimits.SearchesPerHour,
		time.Hour,
	)

	worker := NewSearchWorker(
		accountID,
		p.searchSvc,
		p.replySvc,
		p.configStore,
		rateLimiter,
		p.eventBus,
		p.activityLogger,
	)

	if err := worker.Start(); err != nil {
		return err
	}

	p.workers[accountID] = worker
	return nil
}

// StopWorker stops a worker for an account
func (p *WorkerPool) StopWorker(accountID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	worker, exists := p.workers[accountID]
	if !exists || !worker.IsRunning() {
		return domain.ErrWorkerNotRunning
	}

	return worker.Stop()
}

// IsRunning checks if a worker is running
func (p *WorkerPool) IsRunning(accountID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	worker, exists := p.workers[accountID]
	return exists && worker.IsRunning()
}

// GetStatus returns the status of all workers
func (p *WorkerPool) GetStatus() map[string]bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	status := make(map[string]bool)
	for id, worker := range p.workers {
		status[id] = worker.IsRunning()
	}
	return status
}

// StartAll starts workers for all enabled accounts
func (p *WorkerPool) StartAll() error {
	accounts, err := p.configStore.ListAccounts()
	if err != nil {
		return err
	}

	for _, acc := range accounts {
		if acc.Enabled {
			p.StartWorker(acc.ID)
		}
	}

	return nil
}

// StopAll stops all running workers
func (p *WorkerPool) StopAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, worker := range p.workers {
		if worker.IsRunning() {
			worker.Stop()
		}
	}
}

// RestartWorker restarts a worker
func (p *WorkerPool) RestartWorker(accountID string) error {
	p.StopWorker(accountID)
	return p.StartWorker(accountID)
}
