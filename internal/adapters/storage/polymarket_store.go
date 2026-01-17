package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"xtools/internal/domain"
)

// PolymarketStore handles storage for Polymarket events
type PolymarketStore struct {
	db     *sql.DB
	dbPath string
}

// NewPolymarketStore creates a new Polymarket store
func NewPolymarketStore(dbPath string) (*PolymarketStore, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &PolymarketStore{db: db, dbPath: dbPath}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *PolymarketStore) migrate() error {
	migrations := []string{
		// Original table
		`CREATE TABLE IF NOT EXISTS polymarket_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_type TEXT NOT NULL,
			asset_id TEXT,
			market_slug TEXT,
			market_name TEXT,
			market_image TEXT,
			market_link TEXT,
			timestamp DATETIME NOT NULL,
			raw_data TEXT,
			price TEXT,
			size TEXT,
			side TEXT,
			best_bid TEXT,
			best_ask TEXT,
			fee_rate_bps INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_polymarket_timestamp ON polymarket_events(timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_polymarket_event_type ON polymarket_events(event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_polymarket_market_name ON polymarket_events(market_name)`,
	}

	// Add new columns for trade data (ignore errors if columns already exist)
	newColumns := []string{
		`ALTER TABLE polymarket_events ADD COLUMN trade_id TEXT`,
		`ALTER TABLE polymarket_events ADD COLUMN wallet_address TEXT`,
		`ALTER TABLE polymarket_events ADD COLUMN outcome TEXT`,
		`ALTER TABLE polymarket_events ADD COLUMN outcome_index INTEGER`,
		`ALTER TABLE polymarket_events ADD COLUMN event_slug TEXT`,
		`ALTER TABLE polymarket_events ADD COLUMN event_title TEXT`,
		`ALTER TABLE polymarket_events ADD COLUMN trader_name TEXT`,
		`ALTER TABLE polymarket_events ADD COLUMN condition_id TEXT`,
		`ALTER TABLE polymarket_events ADD COLUMN is_fresh_wallet INTEGER DEFAULT 0`,
		`ALTER TABLE polymarket_events ADD COLUMN wallet_nonce INTEGER`,
		`ALTER TABLE polymarket_events ADD COLUMN risk_score REAL DEFAULT 0`,
		`ALTER TABLE polymarket_events ADD COLUMN risk_signals TEXT`,
		`ALTER TABLE polymarket_events ADD COLUMN fresh_wallet_signal TEXT`,
	}

	// New indexes for fresh wallet queries
	newIndexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_polymarket_fresh_wallet ON polymarket_events(is_fresh_wallet) WHERE is_fresh_wallet = 1`,
		`CREATE INDEX IF NOT EXISTS idx_polymarket_wallet_address ON polymarket_events(wallet_address)`,
		`CREATE INDEX IF NOT EXISTS idx_polymarket_risk_score ON polymarket_events(risk_score DESC)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	// Add new columns (ignore "duplicate column" errors)
	for _, col := range newColumns {
		s.db.Exec(col) // Ignore errors for existing columns
	}

	// Add new indexes
	for _, idx := range newIndexes {
		s.db.Exec(idx) // Ignore errors if index exists
	}

	return nil
}

// SaveEvent saves a Polymarket event to the database
func (s *PolymarketStore) SaveEvent(event domain.PolymarketEvent) error {
	// Serialize risk signals and fresh wallet signal
	var riskSignalsJSON, freshWalletSignalJSON string
	if len(event.RiskSignals) > 0 {
		if data, err := json.Marshal(event.RiskSignals); err == nil {
			riskSignalsJSON = string(data)
		}
	}
	if event.FreshWalletSignal != nil {
		if data, err := json.Marshal(event.FreshWalletSignal); err == nil {
			freshWalletSignalJSON = string(data)
		}
	}

	var walletNonce *int
	if event.WalletProfile != nil {
		walletNonce = &event.WalletProfile.Nonce
	}

	_, err := s.db.Exec(`
		INSERT INTO polymarket_events (
			event_type, asset_id, market_slug, market_name, market_image, market_link,
			timestamp, raw_data, price, size, side, best_bid, best_ask, fee_rate_bps,
			trade_id, wallet_address, outcome, outcome_index, event_slug, event_title,
			trader_name, condition_id, is_fresh_wallet, wallet_nonce, risk_score,
			risk_signals, fresh_wallet_signal
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.EventType, event.AssetID, event.MarketSlug, event.MarketName,
		event.MarketImage, event.MarketLink, event.Timestamp, event.RawData,
		event.Price, event.Size, event.Side, event.BestBid, event.BestAsk, event.FeeRateBps,
		event.TradeID, event.WalletAddress, event.Outcome, event.OutcomeIndex,
		event.EventSlug, event.EventTitle, event.TraderName, event.ConditionID,
		event.IsFreshWallet, walletNonce, event.RiskScore,
		riskSignalsJSON, freshWalletSignalJSON,
	)
	return err
}

// GetEvents retrieves events with optional filtering
func (s *PolymarketStore) GetEvents(filter domain.PolymarketEventFilter) ([]domain.PolymarketEvent, error) {
	var conditions []string
	var args []any

	if len(filter.EventTypes) > 0 {
		placeholders := make([]string, len(filter.EventTypes))
		for i, et := range filter.EventTypes {
			placeholders[i] = "?"
			args = append(args, et)
		}
		conditions = append(conditions, fmt.Sprintf("event_type IN (%s)", strings.Join(placeholders, ",")))
	}

	if filter.MarketName != "" {
		conditions = append(conditions, "(market_name LIKE ? OR event_title LIKE ?)")
		args = append(args, "%"+filter.MarketName+"%", "%"+filter.MarketName+"%")
	}

	if filter.MinPrice > 0 {
		conditions = append(conditions, "CAST(price AS REAL) >= ?")
		args = append(args, filter.MinPrice)
	}

	if filter.MaxPrice > 0 {
		conditions = append(conditions, "CAST(price AS REAL) <= ?")
		args = append(args, filter.MaxPrice)
	}

	if filter.Side != "" {
		conditions = append(conditions, "side = ?")
		args = append(args, filter.Side)
	}

	if filter.MinSize > 0 {
		// Filter by notional value (price * size) instead of just size
		conditions = append(conditions, "(CAST(price AS REAL) * CAST(size AS REAL)) >= ?")
		args = append(args, filter.MinSize)
	}

	if filter.FreshWalletsOnly {
		conditions = append(conditions, "is_fresh_wallet = 1")
	}

	if filter.MinRiskScore > 0 {
		conditions = append(conditions, "risk_score >= ?")
		args = append(args, filter.MinRiskScore)
	}

	if filter.MaxWalletNonce > 0 {
		conditions = append(conditions, "wallet_nonce IS NOT NULL AND wallet_nonce <= ?")
		args = append(args, filter.MaxWalletNonce)
	}

	query := `SELECT id, event_type, asset_id, market_slug, market_name, market_image, market_link,
		timestamp, raw_data, price, size, side, best_bid, best_ask, fee_rate_bps,
		trade_id, wallet_address, outcome, outcome_index, event_slug, event_title,
		trader_name, condition_id, is_fresh_wallet, wallet_nonce, risk_score,
		risk_signals, fresh_wallet_signal
		FROM polymarket_events`

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY timestamp DESC"

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	query += fmt.Sprintf(" LIMIT %d", limit)

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.PolymarketEvent
	for rows.Next() {
		var e domain.PolymarketEvent
		var assetID, marketSlug, marketName, marketImage, marketLink sql.NullString
		var rawData, price, size, side, bestBid, bestAsk sql.NullString
		var feeRateBps sql.NullInt64
		var tradeID, walletAddress, outcome, eventSlug, eventTitle, traderName, conditionID sql.NullString
		var outcomeIndex sql.NullInt64
		var isFreshWallet sql.NullBool
		var walletNonce sql.NullInt64
		var riskScore sql.NullFloat64
		var riskSignals, freshWalletSignal sql.NullString

		if err := rows.Scan(
			&e.ID, &e.EventType, &assetID, &marketSlug, &marketName,
			&marketImage, &marketLink, &e.Timestamp, &rawData,
			&price, &size, &side, &bestBid, &bestAsk, &feeRateBps,
			&tradeID, &walletAddress, &outcome, &outcomeIndex, &eventSlug, &eventTitle,
			&traderName, &conditionID, &isFreshWallet, &walletNonce, &riskScore,
			&riskSignals, &freshWalletSignal,
		); err != nil {
			continue
		}

		e.AssetID = assetID.String
		e.MarketSlug = marketSlug.String
		e.MarketName = marketName.String
		e.MarketImage = marketImage.String
		e.MarketLink = marketLink.String
		e.RawData = rawData.String
		e.Price = price.String
		e.Size = size.String
		e.Side = domain.OrderSide(side.String)
		e.BestBid = bestBid.String
		e.BestAsk = bestAsk.String
		e.FeeRateBps = int(feeRateBps.Int64)
		e.TradeID = tradeID.String
		e.WalletAddress = walletAddress.String
		e.Outcome = outcome.String
		e.OutcomeIndex = int(outcomeIndex.Int64)
		e.EventSlug = eventSlug.String
		e.EventTitle = eventTitle.String
		e.TraderName = traderName.String
		e.ConditionID = conditionID.String
		e.IsFreshWallet = isFreshWallet.Bool
		e.RiskScore = riskScore.Float64

		// Parse risk signals
		if riskSignals.String != "" {
			json.Unmarshal([]byte(riskSignals.String), &e.RiskSignals)
		}

		// Parse fresh wallet signal
		if freshWalletSignal.String != "" {
			var signal domain.FreshWalletSignal
			if json.Unmarshal([]byte(freshWalletSignal.String), &signal) == nil {
				e.FreshWalletSignal = &signal
			}
		}

		// Reconstruct wallet profile if we have data
		if walletNonce.Valid {
			e.WalletProfile = &domain.WalletProfile{
				Address:  walletAddress.String,
				Nonce:    int(walletNonce.Int64),
				IsFresh:  isFreshWallet.Bool,
			}
		}

		events = append(events, e)
	}

	return events, nil
}

// GetEventCount returns the total count of events
func (s *PolymarketStore) GetEventCount() (int64, error) {
	var count int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM polymarket_events").Scan(&count)
	return count, err
}

// GetFreshWalletCount returns count of fresh wallet events
func (s *PolymarketStore) GetFreshWalletCount() (int64, error) {
	var count int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM polymarket_events WHERE is_fresh_wallet = 1").Scan(&count)
	return count, err
}

// ClearEvents removes all Polymarket events
func (s *PolymarketStore) ClearEvents() error {
	_, err := s.db.Exec("DELETE FROM polymarket_events")
	if err != nil {
		return err
	}
	// Vacuum to reclaim space
	_, err = s.db.Exec("VACUUM")
	return err
}

// GetDatabaseInfo returns database statistics
func (s *PolymarketStore) GetDatabaseInfo() (*domain.DatabaseInfo, error) {
	info := &domain.DatabaseInfo{
		Path: s.dbPath,
	}

	// Get file size
	if stat, err := os.Stat(s.dbPath); err == nil {
		info.SizeBytes = stat.Size()
		info.SizeFormatted = formatBytes(stat.Size())
	}

	// Get event count
	count, err := s.GetEventCount()
	if err != nil {
		return info, err
	}
	info.EventCount = count

	return info, nil
}

// Close closes the database connection
func (s *PolymarketStore) Close() error {
	return s.db.Close()
}

// formatBytes converts bytes to human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return strconv.FormatInt(bytes, 10) + " B"
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
