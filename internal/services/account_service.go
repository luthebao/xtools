package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"xtools/internal/domain"
	"xtools/internal/ports"
)

// AccountService manages Twitter accounts
type AccountService struct {
	configStore   ports.ConfigStore
	clientFactory ports.TwitterClientFactory
	eventBus      ports.EventBus

	mu      sync.RWMutex
	clients map[string]ports.TwitterClient
}

// NewAccountService creates a new account service
func NewAccountService(
	configStore ports.ConfigStore,
	clientFactory ports.TwitterClientFactory,
	eventBus ports.EventBus,
) *AccountService {
	return &AccountService{
		configStore:   configStore,
		clientFactory: clientFactory,
		eventBus:      eventBus,
		clients:       make(map[string]ports.TwitterClient),
	}
}

// CreateAccount creates a new account configuration
func (s *AccountService) CreateAccount(cfg domain.AccountConfig) error {
	if cfg.ID == "" {
		cfg.ID = uuid.New().String()[:8]
	}

	if err := s.validateConfig(cfg); err != nil {
		return err
	}

	if err := s.configStore.SaveAccount(cfg); err != nil {
		return fmt.Errorf("failed to save account: %w", err)
	}

	s.eventBus.Emit(ports.EventAccountUpdated, ports.AccountStatusEvent{
		AccountID: cfg.ID,
		Status:    "created",
		Message:   "Account created successfully",
	})

	return nil
}

// UpdateAccount updates an existing account configuration
func (s *AccountService) UpdateAccount(cfg domain.AccountConfig) error {
	if err := s.validateConfig(cfg); err != nil {
		return err
	}

	existing, err := s.configStore.LoadAccount(cfg.ID)
	if err != nil {
		return err
	}

	// If auth changed, close existing client
	if existing.AuthType != cfg.AuthType {
		s.closeClient(cfg.ID)
	}

	if err := s.configStore.SaveAccount(cfg); err != nil {
		return fmt.Errorf("failed to save account: %w", err)
	}

	s.eventBus.Emit(ports.EventAccountUpdated, ports.AccountStatusEvent{
		AccountID: cfg.ID,
		Status:    "updated",
		Message:   "Account updated successfully",
	})

	return nil
}

// DeleteAccount removes an account
func (s *AccountService) DeleteAccount(accountID string) error {
	s.closeClient(accountID)

	if err := s.configStore.DeleteAccount(accountID); err != nil {
		return err
	}

	s.eventBus.Emit(ports.EventAccountUpdated, ports.AccountStatusEvent{
		AccountID: accountID,
		Status:    "deleted",
		Message:   "Account deleted",
	})

	return nil
}

// GetAccount retrieves an account configuration
func (s *AccountService) GetAccount(accountID string) (*domain.AccountConfig, error) {
	return s.configStore.LoadAccount(accountID)
}

// ListAccounts returns all accounts
func (s *AccountService) ListAccounts() ([]domain.AccountConfig, error) {
	return s.configStore.ListAccounts()
}

// GetClient returns a Twitter client for an account
func (s *AccountService) GetClient(accountID string) (ports.TwitterClient, error) {
	s.mu.RLock()
	client, exists := s.clients[accountID]
	s.mu.RUnlock()

	if exists && client.IsAuthenticated() {
		return client, nil
	}

	return s.createAndAuthClient(accountID)
}

func (s *AccountService) createAndAuthClient(accountID string) (ports.TwitterClient, error) {
	cfg, err := s.configStore.LoadAccount(accountID)
	if err != nil {
		return nil, err
	}

	client, err := s.clientFactory.CreateClient(*cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30e9)
	defer cancel()

	if err := client.Authenticate(ctx); err != nil {
		client.Close()
		s.eventBus.Emit(ports.EventAccountError, ports.AccountStatusEvent{
			AccountID: accountID,
			Status:    "auth_failed",
			Error:     err.Error(),
		})
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	s.mu.Lock()
	s.clients[accountID] = client
	s.mu.Unlock()

	return client, nil
}

func (s *AccountService) closeClient(accountID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if client, exists := s.clients[accountID]; exists {
		client.Close()
		delete(s.clients, accountID)
	}
}

// ReloadAccount reloads account config from disk
func (s *AccountService) ReloadAccount(accountID string) (*domain.AccountConfig, error) {
	s.closeClient(accountID)
	return s.configStore.ReloadAccount(accountID)
}

// TestConnection tests the account's Twitter connection
func (s *AccountService) TestConnection(accountID string) error {
	client, err := s.GetClient(accountID)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10e9)
	defer cancel()

	_, err = client.GetProfile(ctx)
	return err
}

// GetAccountStatus returns the current status of an account
func (s *AccountService) GetAccountStatus(accountID string) domain.AccountStatus {
	s.mu.RLock()
	client, exists := s.clients[accountID]
	s.mu.RUnlock()

	status := domain.AccountStatus{}
	if exists && client != nil {
		status.IsActive = client.IsAuthenticated()
		if rateLimit := client.GetRateLimitStatus(); rateLimit != nil {
			status.RateLimitReset = rateLimit.ResetAt
		}
	}

	return status
}

// GetAPIClientForPosting returns an API client for posting replies
// This always uses API credentials, even if the account uses browser for searching
func (s *AccountService) GetAPIClientForPosting(accountID string) (ports.TwitterClient, error) {
	cfg, err := s.configStore.LoadAccount(accountID)
	if err != nil {
		return nil, err
	}

	if cfg.APICredentials == nil {
		return nil, fmt.Errorf("API credentials required for posting replies via API")
	}

	// Check if OAuth credentials are available for posting
	if cfg.APICredentials.APIKey == "" || cfg.APICredentials.AccessToken == "" {
		return nil, fmt.Errorf("OAuth credentials (API Key, Access Token) required for posting replies via API")
	}

	// Create a fresh API client for posting
	client, err := s.clientFactory.CreateClient(domain.AccountConfig{
		AuthType:       domain.AuthTypeAPI,
		APICredentials: cfg.APICredentials,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create API client for posting: %w", err)
	}

	return client, nil
}

// GetBrowserClientForPosting returns a browser client for posting replies
func (s *AccountService) GetBrowserClientForPosting(accountID string) (ports.TwitterClient, error) {
	cfg, err := s.configStore.LoadAccount(accountID)
	if err != nil {
		return nil, err
	}

	if cfg.BrowserAuth == nil || len(cfg.BrowserAuth.Cookies) == 0 {
		return nil, fmt.Errorf("browser cookies required for posting replies via browser")
	}

	// Create a fresh browser client for posting
	client, err := s.clientFactory.CreateClient(domain.AccountConfig{
		AuthType:    domain.AuthTypeBrowser,
		BrowserAuth: cfg.BrowserAuth,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create browser client for posting: %w", err)
	}

	// Authenticate the browser client (launches browser and sets up page)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := client.Authenticate(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("browser authentication failed: %w", err)
	}

	return client, nil
}

// Close closes all clients
func (s *AccountService) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, client := range s.clients {
		client.Close()
		delete(s.clients, id)
	}
}

func (s *AccountService) validateConfig(cfg domain.AccountConfig) error {
	if cfg.Username == "" {
		return fmt.Errorf("username is required")
	}

	switch cfg.AuthType {
	case domain.AuthTypeAPI:
		if cfg.APICredentials == nil || cfg.APICredentials.BearerToken == "" {
			return fmt.Errorf("API credentials required for API auth type")
		}
	case domain.AuthTypeBrowser:
		if cfg.BrowserAuth == nil || len(cfg.BrowserAuth.Cookies) == 0 {
			return fmt.Errorf("browser cookies required for browser auth type")
		}
	default:
		return fmt.Errorf("invalid auth type: %s", cfg.AuthType)
	}

	// Validate credentials based on reply method
	replyMethod := cfg.ReplyConfig.ReplyMethod
	if replyMethod == "" {
		replyMethod = domain.ReplyMethodAPI // Default to API
	}

	if replyMethod == domain.ReplyMethodBrowser {
		// Browser reply method requires cookies
		if cfg.BrowserAuth == nil || len(cfg.BrowserAuth.Cookies) == 0 {
			return fmt.Errorf("browser cookies required for posting replies via browser")
		}
	} else {
		// API reply method requires OAuth credentials
		if cfg.APICredentials == nil ||
			cfg.APICredentials.APIKey == "" ||
			cfg.APICredentials.AccessToken == "" {
			return fmt.Errorf("API credentials (API Key, Access Token) required for posting replies via API")
		}
	}

	if cfg.LLMConfig.APIKey == "" {
		return fmt.Errorf("LLM API key is required")
	}

	return nil
}
