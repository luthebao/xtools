package ports

import (
	"context"
	"xtools/internal/domain"
)

// LLMProvider generates replies using AI
type LLMProvider interface {
	// GenerateReply creates a reply for a tweet
	GenerateReply(ctx context.Context, req ReplyRequest) (*ReplyResponse, error)

	// ValidateConfig checks if the LLM configuration is valid
	ValidateConfig() error

	// GetModelInfo returns information about the model
	GetModelInfo() ModelInfo

	// Close cleans up any resources
	Close() error
}

// ReplyRequest contains the context needed to generate a reply
type ReplyRequest struct {
	OriginalTweet   domain.Tweet
	ThreadContext   []domain.Tweet // Previous tweets in thread
	AuthorBio       string         // Tweet author's bio
	AccountPersona  string         // Replying account's persona
	Keywords        []string       // Matched keywords for context
	MaxLength       int            // Maximum reply length
	Tone            string         // "professional", "casual", "witty"
	IncludeHashtags bool
}

// ReplyResponse contains the generated reply
type ReplyResponse struct {
	Text        string  `json:"text"`
	Confidence  float64 `json:"confidence"`
	TokensUsed  int     `json:"tokensUsed"`
	Model       string  `json:"model"`
	Fallback    bool    `json:"fallback"` // True if used fallback mechanism
	Reason      string  `json:"reason,omitempty"`
}

// ModelInfo provides details about the LLM model
type ModelInfo struct {
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	MaxTokens   int    `json:"maxTokens"`
	Temperature float64 `json:"temperature"`
}

// LLMProviderFactory creates LLM providers from config
type LLMProviderFactory interface {
	CreateProvider(cfg domain.LLMConfig) (LLMProvider, error)
}
