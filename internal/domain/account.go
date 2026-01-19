package domain

import "time"

// AuthType defines the authentication method for Twitter
type AuthType string

const (
	AuthTypeAPI     AuthType = "api"
	AuthTypeBrowser AuthType = "browser"
)

// ApprovalMode defines how replies are handled
type ApprovalMode string

const (
	ApprovalModeAuto  ApprovalMode = "auto"
	ApprovalModeQueue ApprovalMode = "queue"
)

// ReplyMethod defines how replies are posted (api or browser)
type ReplyMethod string

const (
	ReplyMethodAPI     ReplyMethod = "api"
	ReplyMethodBrowser ReplyMethod = "browser"
)

// Account represents a Twitter account entity
type Account struct {
	ID          string
	Username    string
	DisplayName string
	AuthType    AuthType
	Status      AccountStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AccountStatus represents the current state of an account
type AccountStatus struct {
	IsActive       bool
	IsRunning      bool
	LastActivity   time.Time
	ErrorMessage   string
	RepliesSent    int
	RepliesQueued  int
	RateLimitReset time.Time
}

// AccountConfig represents the configuration for a Twitter account
type AccountConfig struct {
	ID        string   `yaml:"id" json:"id"`
	Username  string   `yaml:"username" json:"username"`
	Enabled   bool     `yaml:"enabled" json:"enabled"`
	AuthType  AuthType `yaml:"auth_type" json:"authType"`
	DebugMode bool     `yaml:"debug_mode" json:"debugMode"`

	// API Authentication (when auth_type = "api")
	APICredentials *APICredentials `yaml:"api_credentials,omitempty" json:"apiCredentials,omitempty"`

	// Browser Authentication (when auth_type = "browser")
	BrowserAuth *BrowserAuth `yaml:"browser_auth,omitempty" json:"browserAuth,omitempty"`

	// LLM Configuration (per-account)
	LLMConfig LLMConfig `yaml:"llm_config" json:"llmConfig"`

	// Search Configuration
	SearchConfig SearchConfig `yaml:"search_config" json:"searchConfig"`

	// Reply Configuration
	ReplyConfig ReplyConfig `yaml:"reply_config" json:"replyConfig"`

	// Rate Limiting
	RateLimits RateLimits `yaml:"rate_limits" json:"rateLimits"`

	// Actions Configuration (tweet actions for Polymarket events)
	ActionsConfig ActionsConfig `yaml:"actions_config" json:"actionsConfig"`
}

// APICredentials holds Twitter API v2 credentials
type APICredentials struct {
	APIKey       string `yaml:"api_key" json:"apiKey"`
	APISecret    string `yaml:"api_secret" json:"apiSecret"`
	AccessToken  string `yaml:"access_token" json:"accessToken"`
	AccessSecret string `yaml:"access_secret" json:"accessSecret"`
	BearerToken  string `yaml:"bearer_token" json:"bearerToken"`
}

// BrowserAuth holds browser automation credentials
type BrowserAuth struct {
	Cookies   []Cookie `yaml:"cookies" json:"cookies"`
	UserAgent string   `yaml:"user_agent" json:"userAgent"`
	ProxyURL  string   `yaml:"proxy_url,omitempty" json:"proxyUrl,omitempty"`
}

// Cookie represents a browser cookie
type Cookie struct {
	Name     string `yaml:"name" json:"name"`
	Value    string `yaml:"value" json:"value"`
	Domain   string `yaml:"domain" json:"domain"`
	Path     string `yaml:"path" json:"path"`
	Expires  int64  `yaml:"expires" json:"expires"`
	Secure   bool   `yaml:"secure" json:"secure"`
	HttpOnly bool   `yaml:"http_only" json:"httpOnly"`
}

// LLMConfig holds LLM API configuration
type LLMConfig struct {
	BaseURL     string  `yaml:"base_url" json:"baseUrl"`
	APIKey      string  `yaml:"api_key" json:"apiKey"`
	Model       string  `yaml:"model" json:"model"`
	Temperature float64 `yaml:"temperature" json:"temperature"`
	MaxTokens   int     `yaml:"max_tokens" json:"maxTokens"`
	Persona     string  `yaml:"persona" json:"persona"`
}

// SearchConfig holds tweet search settings
type SearchConfig struct {
	Keywords        []string `yaml:"keywords" json:"keywords"`
	ExcludeKeywords []string `yaml:"exclude_keywords" json:"excludeKeywords"`
	Blocklist       []string `yaml:"blocklist" json:"blocklist"`
	EnglishOnly     bool     `yaml:"english_only" json:"englishOnly"`
	MinFaves        int      `yaml:"min_faves" json:"minFaves"`
	MinReplies      int      `yaml:"min_replies" json:"minReplies"`
	MinRetweets     int      `yaml:"min_retweets" json:"minRetweets"`
	MaxAgeMins      int      `yaml:"max_age_mins" json:"maxAgeMins"`
	IntervalSecs    int      `yaml:"interval_secs" json:"intervalSecs"`
}

// ReplyConfig holds reply behavior settings
type ReplyConfig struct {
	ApprovalMode    ApprovalMode `yaml:"approval_mode" json:"approvalMode"`
	ReplyMethod     ReplyMethod  `yaml:"reply_method" json:"replyMethod"`
	MaxReplyLength  int          `yaml:"max_reply_length" json:"maxReplyLength"`
	Tone            string       `yaml:"tone" json:"tone"`
	IncludeHashtags bool         `yaml:"include_hashtags" json:"includeHashtags"`
	SignatureText   string       `yaml:"signature_text,omitempty" json:"signatureText,omitempty"`
}

// RateLimits holds rate limiting configuration
type RateLimits struct {
	SearchesPerHour int `yaml:"searches_per_hour" json:"searchesPerHour"`
	RepliesPerHour  int `yaml:"replies_per_hour" json:"repliesPerHour"`
	RepliesPerDay   int `yaml:"replies_per_day" json:"repliesPerDay"`
	MinDelayBetween int `yaml:"min_delay_between_secs" json:"minDelayBetween"`
}
