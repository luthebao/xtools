package services

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"xtools/internal/adapters/polymarket"
	"xtools/internal/adapters/storage"
	"xtools/internal/domain"
	"xtools/internal/ports"
)

// PolymarketService handles Polymarket event watching and storage
type PolymarketService struct {
	mu             sync.RWMutex
	store          *storage.PolymarketStore
	client         *polymarket.WebSocketClient
	walletAnalyzer *polymarket.WalletAnalyzer
	eventBus       ports.EventBus
	dbPath         string
	config         domain.PolymarketConfig
	saveFilter     domain.PolymarketEventFilter // Filter for saving events to DB

	// Async analysis
	analysisCh chan *domain.PolymarketEvent
	stopCh     chan struct{}
}

// NewPolymarketService creates a new Polymarket service
func NewPolymarketService(store *storage.PolymarketStore, eventBus ports.EventBus, dbPath string) *PolymarketService {
	config := domain.DefaultPolymarketConfig()

	svc := &PolymarketService{
		store:          store,
		eventBus:       eventBus,
		dbPath:         dbPath,
		config:         config,
		walletAnalyzer: polymarket.NewWalletAnalyzer(config),
		analysisCh:     make(chan *domain.PolymarketEvent, 1000),
		saveFilter: domain.PolymarketEventFilter{
			MinSize: 100, // Default $100 minimum
		},
	}

	// Create WebSocket client with event callback
	svc.client = polymarket.NewWebSocketClient(svc.onEvent)

	return svc
}

// Start begins watching Polymarket events
func (s *PolymarketService) Start() error {
	s.mu.Lock()
	if s.stopCh != nil {
		s.mu.Unlock()
		return nil // Already running
	}
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	// Start the wallet analysis worker
	go s.walletAnalysisWorker()

	// Connect returns immediately and runs in the background
	return s.client.Connect()
}

// Stop stops watching Polymarket events
func (s *PolymarketService) Stop() {
	s.mu.Lock()
	if s.stopCh != nil {
		close(s.stopCh)
		s.stopCh = nil
	}
	s.mu.Unlock()

	if s.client != nil {
		s.client.Disconnect()
	}
}

// GetStatus returns the current watcher status
func (s *PolymarketService) GetStatus() domain.PolymarketWatcherStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.client == nil {
		return domain.PolymarketWatcherStatus{}
	}

	return s.client.GetStatus()
}

// GetEvents retrieves events with optional filtering
func (s *PolymarketService) GetEvents(filter domain.PolymarketEventFilter) ([]domain.PolymarketEvent, error) {
	return s.store.GetEvents(filter)
}

// ClearEvents removes all stored events
func (s *PolymarketService) ClearEvents() error {
	return s.store.ClearEvents()
}

// GetDatabaseInfo returns database statistics
func (s *PolymarketService) GetDatabaseInfo() (*domain.DatabaseInfo, error) {
	return s.store.GetDatabaseInfo()
}

// onEvent is called when a new event is received from WebSocket
func (s *PolymarketService) onEvent(event domain.PolymarketEvent) {
	s.mu.RLock()
	filter := s.saveFilter
	s.mu.RUnlock()

	// Check basic filters first (doesn't require wallet analysis)
	if !s.matchesBasicFilter(event, filter) {
		return
	}

	// Check if fresh wallet filters are active
	hasFreshWalletFilter := filter.FreshWalletsOnly || filter.MinRiskScore > 0 || filter.MaxWalletNonce > 0

	// If fresh wallet filters are active, do synchronous wallet analysis
	if hasFreshWalletFilter && event.WalletAddress != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		signal, err := s.walletAnalyzer.AnalyzeTrade(ctx, &event)
		cancel()

		if err != nil {
			log.Printf("[PolymarketService] Wallet analysis error: %v", err)
			return // Skip if analysis fails when filter is active
		}

		// Update fresh wallet counters if detected
		if signal != nil && signal.Triggered {
			s.client.IncrementFreshWalletsFound()

			if signal.Confidence >= s.config.AlertThreshold {
				log.Printf("[PolymarketService] HIGH RISK ALERT: Fresh wallet %s made $%.2f trade on %s (confidence: %.2f)",
					shortenAddress(event.WalletAddress),
					parseNotionalValue(event.Price, event.Size),
					event.MarketSlug,
					signal.Confidence)
			}
		}

		// Now check fresh wallet filters with analyzed data
		if !s.matchesFreshWalletFilter(event, filter) {
			return
		}

		// Save and emit (wallet analysis already done)
		s.saveAndEmit(event)

		// Emit fresh wallet alert if applicable
		if event.IsFreshWallet && event.RiskScore >= s.config.AlertThreshold {
			s.eventBus.Emit("polymarket:fresh_wallet", event)
		}
	} else {
		// No fresh wallet filter - save and emit immediately
		s.saveAndEmit(event)

		// Queue for background wallet analysis (non-blocking)
		if event.WalletAddress != "" {
			eventCopy := event
			select {
			case s.analysisCh <- &eventCopy:
			default:
			}
		}
	}
}

// matchesBasicFilter checks basic filter criteria (doesn't require wallet analysis)
func (s *PolymarketService) matchesBasicFilter(event domain.PolymarketEvent, filter domain.PolymarketEventFilter) bool {
	// Check minimum notional value (price * size)
	notional := parseNotionalValue(event.Price, event.Size)
	minSize := filter.MinSize
	if minSize <= 0 {
		minSize = s.config.MinTradeSize
	}
	if notional < minSize {
		return false
	}

	// Check event types
	if len(filter.EventTypes) > 0 {
		found := false
		for _, et := range filter.EventTypes {
			if et == event.EventType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check side
	if filter.Side != "" && string(event.Side) != string(filter.Side) {
		return false
	}

	// Check price range
	if event.Price != "" {
		var price float64
		parseFloat(event.Price, &price)
		if filter.MinPrice > 0 && price < filter.MinPrice {
			return false
		}
		if filter.MaxPrice > 0 && price > filter.MaxPrice {
			return false
		}
	}

	// Check market name (partial match)
	if filter.MarketName != "" {
		marketName := strings.ToLower(filter.MarketName)
		eventMarket := strings.ToLower(event.MarketName)
		eventTitle := strings.ToLower(event.EventTitle)
		if !strings.Contains(eventMarket, marketName) && !strings.Contains(eventTitle, marketName) {
			return false
		}
	}

	return true
}

// matchesFreshWalletFilter checks fresh wallet specific filters (requires wallet analysis to be done)
func (s *PolymarketService) matchesFreshWalletFilter(event domain.PolymarketEvent, filter domain.PolymarketEventFilter) bool {
	// Check fresh wallets only
	if filter.FreshWalletsOnly && !event.IsFreshWallet {
		return false
	}

	// Check min risk score
	if filter.MinRiskScore > 0 && event.RiskScore < filter.MinRiskScore {
		return false
	}

	// Check max wallet nonce
	if filter.MaxWalletNonce > 0 {
		if event.WalletProfile == nil || event.WalletProfile.Nonce > filter.MaxWalletNonce {
			return false
		}
	}

	return true
}

// walletAnalysisWorker processes events and analyzes wallets in background
func (s *PolymarketService) walletAnalysisWorker() {
	log.Println("[PolymarketService] Starting wallet analysis worker")

	for {
		select {
		case <-s.stopCh:
			log.Println("[PolymarketService] Wallet analysis worker stopped")
			return
		case event := <-s.analysisCh:
			if event == nil {
				continue
			}

			// Analyze for fresh wallet (with timeout)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			signal, err := s.walletAnalyzer.AnalyzeTrade(ctx, event)
			cancel()

			if err != nil {
				log.Printf("[PolymarketService] Wallet analysis error: %v", err)
				continue
			}

			// If fresh wallet detected, update counters and emit alert
			if signal != nil && signal.Triggered {
				s.client.IncrementFreshWalletsFound()

				// Log high-confidence alerts
				if signal.Confidence >= s.config.AlertThreshold {
					log.Printf("[PolymarketService] HIGH RISK ALERT: Fresh wallet %s made $%.2f trade on %s (confidence: %.2f)",
						shortenAddress(event.WalletAddress),
						parseNotionalValue(event.Price, event.Size),
						event.MarketSlug,
						signal.Confidence)

					// Emit fresh wallet alert event
					s.eventBus.Emit("polymarket:fresh_wallet", *event)
				}
			}
		}
	}
}

func (s *PolymarketService) saveAndEmit(event domain.PolymarketEvent) {
	// Save to database (async to avoid blocking)
	go func(e domain.PolymarketEvent) {
		if err := s.store.SaveEvent(e); err != nil {
			log.Printf("[PolymarketService] Failed to save event: %v", err)
		}
	}(event)

	// Emit to frontend for real-time updates
	s.eventBus.Emit("polymarket:event", event)
}

// IsRunning returns whether the watcher is currently running
func (s *PolymarketService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.client == nil {
		return false
	}

	return s.client.IsConnected()
}

// Close shuts down the service
func (s *PolymarketService) Close() {
	s.Stop()
	if s.store != nil {
		s.store.Close()
	}
}

// UpdateConfig updates the service configuration
func (s *PolymarketService) UpdateConfig(config domain.PolymarketConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config
	s.walletAnalyzer = polymarket.NewWalletAnalyzer(config)
}

// GetConfig returns the current configuration
func (s *PolymarketService) GetConfig() domain.PolymarketConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// SetSaveFilter sets the filter for saving events to database
func (s *PolymarketService) SetSaveFilter(filter domain.PolymarketEventFilter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveFilter = filter
	log.Printf("[PolymarketService] Save filter updated: minSize=%.0f, side=%s, freshWalletsOnly=%v",
		filter.MinSize, filter.Side, filter.FreshWalletsOnly)
}

// GetSaveFilter returns the current save filter
func (s *PolymarketService) GetSaveFilter() domain.PolymarketEventFilter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.saveFilter
}

// Helper functions

func shortenAddress(addr string) string {
	if len(addr) <= 10 {
		return addr
	}
	return addr[:6] + "..." + addr[len(addr)-4:]
}

func parseNotionalValue(price, size string) float64 {
	if price == "" || size == "" {
		return 0
	}

	var p, s float64
	if _, err := parseFloat(price, &p); err != nil {
		return 0
	}
	if _, err := parseFloat(size, &s); err != nil {
		return 0
	}
	return p * s
}

func parseFloat(str string, v *float64) (bool, error) {
	if str == "" {
		return false, nil
	}
	var val float64
	_, err := formatScan(str, &val)
	if err != nil {
		return false, err
	}
	*v = val
	return true, nil
}

func formatScan(str string, v *float64) (int, error) {
	// Simple float parser
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
	return 1, nil
}
