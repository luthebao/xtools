package twitter

import (
	"fmt"
	"xtools/internal/domain"
	"xtools/internal/ports"
)

// ClientFactory creates Twitter clients based on auth type
type ClientFactory struct{}

// NewClientFactory creates a new factory instance
func NewClientFactory() *ClientFactory {
	return &ClientFactory{}
}

// CreateClient creates the appropriate Twitter client based on config
func (f *ClientFactory) CreateClient(cfg domain.AccountConfig) (ports.TwitterClient, error) {
	switch cfg.AuthType {
	case domain.AuthTypeAPI:
		return f.createAPIClient(cfg)
	case domain.AuthTypeBrowser:
		return f.createBrowserClient(cfg)
	default:
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidAuthType, cfg.AuthType)
	}
}

func (f *ClientFactory) createAPIClient(cfg domain.AccountConfig) (ports.TwitterClient, error) {
	if cfg.APICredentials == nil {
		return nil, fmt.Errorf("API credentials required for API auth type")
	}

	return NewAPIClient(*cfg.APICredentials)
}

func (f *ClientFactory) createBrowserClient(cfg domain.AccountConfig) (ports.TwitterClient, error) {
	if cfg.BrowserAuth == nil {
		return nil, fmt.Errorf("browser auth config required for browser auth type")
	}

	return NewBrowserClient(*cfg.BrowserAuth)
}

// Ensure factory implements interface
var _ ports.TwitterClientFactory = (*ClientFactory)(nil)
