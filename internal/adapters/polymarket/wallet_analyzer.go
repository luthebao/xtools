package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"sync"
	"time"

	"xtools/internal/domain"
)

const (
	// Cache settings
	walletCacheTTL = 5 * time.Minute
	maxCacheSize   = 10000

	// Default fresh wallet thresholds
	defaultMinTradeSize        = 1000.0 // $1000 USDC
	defaultFreshWalletMaxNonce = 5
	defaultFreshWalletMaxAge   = 48.0 // hours

	// Confidence scoring constants
	baseConfidence      = 0.5
	brandNewBonus       = 0.2
	veryYoungBonus      = 0.1
	largeTradeBonus     = 0.1
	largeTradeThreshold = 10000.0 // $10,000
)

// Default RPC URLs for Polygon
var defaultRPCURLs = []string{
	"https://polygon-rpc.com",
	"https://rpc.ankr.com/polygon",
	"https://polygon.llamarpc.com",
}

// WalletAnalyzer analyzes wallet profiles for fresh wallet detection
type WalletAnalyzer struct {
	mu            sync.RWMutex
	rpcURLs       []string
	currentRPCIdx int
	httpClient    *http.Client
	cache         map[string]*cachedProfile
	config        domain.PolymarketConfig
}

type cachedProfile struct {
	profile   *domain.WalletProfile
	expiresAt time.Time
}

// NewWalletAnalyzer creates a new wallet analyzer
func NewWalletAnalyzer(config domain.PolymarketConfig) *WalletAnalyzer {
	// Build RPC URL list
	var rpcURLs []string

	// Add configured URLs first
	if len(config.PolygonRPCURLs) > 0 {
		rpcURLs = append(rpcURLs, config.PolygonRPCURLs...)
	}

	// Add legacy single URL if set
	if config.PolygonRPCURL != "" {
		// Check if not already in list
		found := false
		for _, u := range rpcURLs {
			if u == config.PolygonRPCURL {
				found = true
				break
			}
		}
		if !found {
			rpcURLs = append([]string{config.PolygonRPCURL}, rpcURLs...)
		}
	}

	// Fall back to defaults if no URLs configured
	if len(rpcURLs) == 0 {
		rpcURLs = defaultRPCURLs
	}

	return &WalletAnalyzer{
		rpcURLs:       rpcURLs,
		currentRPCIdx: 0,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		cache:  make(map[string]*cachedProfile),
		config: config,
	}
}

// AnalyzeWallet retrieves and analyzes a wallet's profile
func (a *WalletAnalyzer) AnalyzeWallet(ctx context.Context, address string) (*domain.WalletProfile, error) {
	if address == "" {
		return nil, fmt.Errorf("empty wallet address")
	}

	// Check cache first
	if profile := a.getFromCache(address); profile != nil {
		return profile, nil
	}

	// Get transaction count (nonce)
	nonce, err := a.getTransactionCount(ctx, address)
	if err != nil {
		log.Printf("[WalletAnalyzer] Failed to get nonce for %s: %v", shortenAddress(address), err)
		// Return a default profile with unknown nonce
		return &domain.WalletProfile{
			Address:        address,
			Nonce:          -1,
			IsFresh:        false,
			AnalyzedAt:     time.Now(),
			FreshThreshold: a.getFreshThreshold(),
		}, nil
	}

	profile := &domain.WalletProfile{
		Address:        address,
		Nonce:          nonce,
		IsFresh:        nonce <= a.getFreshThreshold(),
		IsBrandNew:     nonce == 0,
		TotalTxCount:   nonce,
		AnalyzedAt:     time.Now(),
		FreshThreshold: a.getFreshThreshold(),
	}

	// Cache the result
	a.addToCache(address, profile)

	return profile, nil
}

// AnalyzeTrade analyzes a trade event for fresh wallet signals
func (a *WalletAnalyzer) AnalyzeTrade(ctx context.Context, event *domain.PolymarketEvent) (*domain.FreshWalletSignal, error) {
	if event.WalletAddress == "" {
		return nil, nil
	}

	// Check minimum trade size
	tradeSize := a.parseTradeSize(event)
	if tradeSize < a.getMinTradeSize() {
		return nil, nil
	}

	// Analyze wallet
	profile, err := a.AnalyzeWallet(ctx, event.WalletAddress)
	if err != nil {
		return nil, err
	}

	// Check if wallet is fresh
	if !a.isWalletFresh(profile) {
		return nil, nil
	}

	// Calculate confidence score
	confidence, factors := a.calculateConfidence(profile, tradeSize)

	signal := &domain.FreshWalletSignal{
		Confidence: confidence,
		Factors:    factors,
		Triggered:  true,
	}

	// Update event with wallet info
	event.WalletProfile = profile
	event.IsFreshWallet = true
	event.FreshWalletSignal = signal
	event.RiskScore = confidence

	// Add risk signals
	event.RiskSignals = a.generateRiskSignals(profile, tradeSize)

	log.Printf("[WalletAnalyzer] Fresh wallet detected: %s nonce=%d confidence=%.2f trade=$%.2f",
		shortenAddress(event.WalletAddress), profile.Nonce, confidence, tradeSize)

	return signal, nil
}

func (a *WalletAnalyzer) isWalletFresh(profile *domain.WalletProfile) bool {
	if profile.Nonce < 0 {
		// Unknown nonce, can't determine freshness
		return false
	}

	// Must have few transactions
	if profile.Nonce > a.getFreshThreshold() {
		return false
	}

	// If age is known, must be recent
	if profile.AgeHours > 0 && profile.AgeHours > a.getMaxAge() {
		return false
	}

	return true
}

func (a *WalletAnalyzer) calculateConfidence(profile *domain.WalletProfile, tradeSize float64) (float64, map[string]float64) {
	factors := make(map[string]float64)
	confidence := baseConfidence
	factors["base"] = baseConfidence

	// Brand new wallet bonus (nonce == 0)
	if profile.IsBrandNew {
		factors["brand_new"] = brandNewBonus
		confidence += brandNewBonus
	}

	// Very young wallet bonus (nonce <= 2)
	if profile.Nonce <= 2 && profile.Nonce >= 0 {
		factors["very_young"] = veryYoungBonus
		confidence += veryYoungBonus
	}

	// Large trade bonus
	if tradeSize > largeTradeThreshold {
		factors["large_trade"] = largeTradeBonus
		confidence += largeTradeBonus
	}

	// Clamp confidence to [0, 1]
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0 {
		confidence = 0
	}

	return confidence, factors
}

func (a *WalletAnalyzer) generateRiskSignals(profile *domain.WalletProfile, tradeSize float64) []string {
	var signals []string

	if profile.IsBrandNew {
		signals = append(signals, "Brand New Wallet (0 transactions)")
	} else if profile.Nonce <= 2 {
		signals = append(signals, fmt.Sprintf("Very Fresh Wallet (%d transactions)", profile.Nonce))
	} else {
		signals = append(signals, fmt.Sprintf("Fresh Wallet (%d transactions)", profile.Nonce))
	}

	if tradeSize >= largeTradeThreshold {
		signals = append(signals, fmt.Sprintf("Large Position ($%.2f)", tradeSize))
	}

	return signals
}

func (a *WalletAnalyzer) parseTradeSize(event *domain.PolymarketEvent) float64 {
	if event.Size == "" || event.Price == "" {
		return 0
	}

	size, err := strconv.ParseFloat(event.Size, 64)
	if err != nil {
		return 0
	}

	price, err := strconv.ParseFloat(event.Price, 64)
	if err != nil {
		return 0
	}

	// Notional value = size * price
	return size * price
}

// getTransactionCount gets the nonce (transaction count) for an address
// Tries multiple RPC URLs with fallback on failure
func (a *WalletAnalyzer) getTransactionCount(ctx context.Context, address string) (int, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "eth_getTransactionCount",
		"params":  []any{address, "latest"},
		"id":      1,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	// Try each RPC URL starting from current index
	a.mu.RLock()
	startIdx := a.currentRPCIdx
	urls := a.rpcURLs
	a.mu.RUnlock()

	var lastErr error
	for i := 0; i < len(urls); i++ {
		idx := (startIdx + i) % len(urls)
		rpcURL := urls[idx]

		nonce, err := a.tryRPCRequest(ctx, rpcURL, body)
		if err == nil {
			// Success - update current index to this working RPC
			if idx != startIdx {
				a.mu.Lock()
				a.currentRPCIdx = idx
				a.mu.Unlock()
				log.Printf("[WalletAnalyzer] Switched to RPC: %s", rpcURL)
			}
			return nonce, nil
		}

		lastErr = err
		log.Printf("[WalletAnalyzer] RPC %s failed: %v, trying next...", rpcURL, err)
	}

	return 0, fmt.Errorf("all RPC endpoints failed, last error: %v", lastErr)
}

// tryRPCRequest attempts a single RPC request
func (a *WalletAnalyzer) tryRPCRequest(ctx context.Context, rpcURL string, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", rpcURL, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(newBytesReader(body))
	req.ContentLength = int64(len(body))

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result struct {
		Result string `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if result.Error != nil {
		return 0, fmt.Errorf("RPC error: %s", result.Error.Message)
	}

	// Parse hex nonce
	nonce := new(big.Int)
	if _, ok := nonce.SetString(result.Result, 0); !ok {
		return 0, fmt.Errorf("invalid nonce: %s", result.Result)
	}

	return int(nonce.Int64()), nil
}

// GetRPCURLs returns the configured RPC URLs
func (a *WalletAnalyzer) GetRPCURLs() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rpcURLs
}

func (a *WalletAnalyzer) getFromCache(address string) *domain.WalletProfile {
	a.mu.RLock()
	defer a.mu.RUnlock()

	cached, ok := a.cache[address]
	if !ok {
		return nil
	}

	if time.Now().After(cached.expiresAt) {
		return nil
	}

	return cached.profile
}

func (a *WalletAnalyzer) addToCache(address string, profile *domain.WalletProfile) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Evict old entries if cache is too large
	if len(a.cache) >= maxCacheSize {
		// Remove expired entries
		now := time.Now()
		for k, v := range a.cache {
			if now.After(v.expiresAt) {
				delete(a.cache, k)
			}
		}
		// If still too large, clear half
		if len(a.cache) >= maxCacheSize {
			count := 0
			for k := range a.cache {
				delete(a.cache, k)
				count++
				if count >= maxCacheSize/2 {
					break
				}
			}
		}
	}

	a.cache[address] = &cachedProfile{
		profile:   profile,
		expiresAt: time.Now().Add(walletCacheTTL),
	}
}

func (a *WalletAnalyzer) getFreshThreshold() int {
	if a.config.FreshWalletMaxNonce > 0 {
		return a.config.FreshWalletMaxNonce
	}
	return defaultFreshWalletMaxNonce
}

func (a *WalletAnalyzer) getMinTradeSize() float64 {
	if a.config.MinTradeSize > 0 {
		return a.config.MinTradeSize
	}
	return defaultMinTradeSize
}

func (a *WalletAnalyzer) getMaxAge() float64 {
	if a.config.FreshWalletMaxAge > 0 {
		return a.config.FreshWalletMaxAge
	}
	return defaultFreshWalletMaxAge
}

// bytesReader wraps a byte slice for http request body
type bytesReader struct {
	*byteSliceReader
}

type byteSliceReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{&byteSliceReader{data: data}}
}

func (r *byteSliceReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
