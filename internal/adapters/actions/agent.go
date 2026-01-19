package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"xtools/internal/domain"
	"xtools/internal/ports"
)

// TweetAgent implements ports.ActionAgent using OpenAI-compatible API
type TweetAgent struct {
	config     domain.LLMConfig
	httpClient *http.Client
}

// NewTweetAgent creates a new tweet agent
func NewTweetAgent(config domain.LLMConfig) (*TweetAgent, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://api.openai.com/v1"
	}
	if config.Model == "" {
		config.Model = "gpt-4o-mini"
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 500
	}
	if config.Temperature == 0 {
		config.Temperature = 0.7
	}

	return &TweetAgent{
		config: config,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// GenerateTweet performs the full multi-step generation pipeline
func (a *TweetAgent) GenerateTweet(ctx context.Context, req domain.ActionGenerationRequest) (*domain.ActionGenerationResponse, error) {
	resp := &domain.ActionGenerationResponse{}

	// Step 1: Generate initial draft
	draft, tokens1, err := a.GenerateDraft(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("draft generation failed: %w", err)
	}
	resp.DraftText = draft
	resp.TokensUsed = tokens1

	// Step 2: Review and refine (if examples provided for guidance)
	if len(req.ExampleTweets) > 0 || len(req.HistoricalTweets) > 0 {
		reviewed, tokens2, err := a.ReviewAndRefine(ctx, draft, req)
		if err != nil {
			// If review fails, use draft as final
			resp.FinalText = draft
			resp.Reasoning = "Review failed, using draft"
		} else {
			resp.ReviewedText = reviewed
			resp.FinalText = reviewed
			resp.TokensUsed += tokens2
			resp.Reasoning = "Reviewed and refined"
		}
	} else {
		resp.FinalText = draft
		resp.Reasoning = "No examples provided, using draft"
	}

	// Ensure tweet is within limits
	resp.FinalText = a.truncateTweet(resp.FinalText, req.MaxLength)
	resp.Confidence = 1.0

	return resp, nil
}

// GenerateDraft generates the initial tweet draft
func (a *TweetAgent) GenerateDraft(ctx context.Context, req domain.ActionGenerationRequest) (string, int, error) {
	systemPrompt := a.buildSystemPrompt(req)
	userPrompt := a.buildDraftPrompt(req)

	content, tokens, err := a.callLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", 0, err
	}

	return strings.TrimSpace(content), tokens, nil
}

// ReviewAndRefine improves the draft using examples
func (a *TweetAgent) ReviewAndRefine(ctx context.Context, draft string, req domain.ActionGenerationRequest) (string, int, error) {
	systemPrompt := "You are a tweet editor. Improve the tweet for clarity, engagement, and accuracy. Keep it under 280 characters. Output ONLY the improved tweet, nothing else."
	userPrompt := a.buildReviewPrompt(draft, req)

	content, tokens, err := a.callLLM(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", 0, err
	}

	return strings.TrimSpace(content), tokens, nil
}

func (a *TweetAgent) buildSystemPrompt(req domain.ActionGenerationRequest) string {
	if req.SystemPrompt != "" {
		return req.SystemPrompt
	}

	return `You are a crypto trading analyst who tweets about insider activity on Polymarket.
Write engaging, informative tweets about fresh wallet trades.
Focus on: wallet freshness, trade details, market context, and potential significance.
Keep tweets under 280 characters. Be factual, concise, and use relevant emojis.
Include the market URL and wallet profile URL when relevant.
Output ONLY the tweet text, nothing else.`
}

func (a *TweetAgent) buildDraftPrompt(req domain.ActionGenerationRequest) string {
	var sb strings.Builder

	sb.WriteString("## Fresh Wallet Trade Detected\n\n")

	// Trade details
	if req.TradeEvent != nil {
		sb.WriteString(fmt.Sprintf("**Market**: %s\n", req.TradeEvent.MarketName))
		sb.WriteString(fmt.Sprintf("**Event**: %s\n", req.TradeEvent.EventTitle))
		sb.WriteString(fmt.Sprintf("**Trade**: %s %s @ %s (Size: %s USDC)\n",
			req.TradeEvent.Side, req.TradeEvent.Outcome, req.TradeEvent.Price, req.TradeEvent.Size))
	}

	// Wallet details
	if req.WalletProfile != nil {
		sb.WriteString(fmt.Sprintf("**Wallet**: %s\n", shortenAddress(req.WalletProfile.Address)))
		sb.WriteString(fmt.Sprintf("**Bet Count**: %d (Freshness: %s)\n",
			req.WalletProfile.BetCount, req.WalletProfile.FreshnessLevel))
		if req.WalletProfile.JoinDate != "" {
			sb.WriteString(fmt.Sprintf("**Joined**: %s\n", req.WalletProfile.JoinDate))
		}
	}

	// URLs
	sb.WriteString(fmt.Sprintf("\n**Market URL**: %s\n", req.MarketURL))
	sb.WriteString(fmt.Sprintf("**Wallet URL**: %s\n\n", req.ProfileURL))

	// Context from fetched pages (if available)
	if req.MarketContext != "" {
		sb.WriteString("## Market Context\n")
		sb.WriteString(truncateText(req.MarketContext, 500))
		sb.WriteString("\n\n")
	}

	// Historical tweets for style reference
	if len(req.HistoricalTweets) > 0 {
		sb.WriteString("## Past Successful Tweets (for style reference)\n")
		for i, tweet := range req.HistoricalTweets {
			if i >= 3 {
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", tweet))
		}
		sb.WriteString("\n")
	}

	// Curated examples
	if len(req.ExampleTweets) > 0 {
		sb.WriteString("## Example Tweets (best practices)\n")
		for _, ex := range req.ExampleTweets {
			sb.WriteString(fmt.Sprintf("- %s\n", ex))
		}
		sb.WriteString("\n")
	}

	maxLen := req.MaxLength
	if maxLen <= 0 {
		maxLen = 280
	}
	sb.WriteString(fmt.Sprintf("Write a tweet about this fresh wallet trade in %d characters or less.\n", maxLen))
	sb.WriteString("Include relevant emojis and the market/wallet URLs when helpful.\n")
	sb.WriteString("Output ONLY the tweet text, nothing else.")

	return sb.String()
}

func (a *TweetAgent) buildReviewPrompt(draft string, req domain.ActionGenerationRequest) string {
	var sb strings.Builder

	sb.WriteString("## Original Draft Tweet\n")
	sb.WriteString(fmt.Sprintf("\"%s\"\n\n", draft))

	sb.WriteString("## Context\n")
	if req.TradeEvent != nil {
		sb.WriteString(fmt.Sprintf("- Market: %s\n", req.TradeEvent.MarketName))
		sb.WriteString(fmt.Sprintf("- Trade: %s %s @ %s\n", req.TradeEvent.Side, req.TradeEvent.Outcome, req.TradeEvent.Price))
	}
	if req.WalletProfile != nil {
		sb.WriteString(fmt.Sprintf("- Wallet: %s (Bets: %d, Freshness: %s)\n",
			shortenAddress(req.WalletProfile.Address), req.WalletProfile.BetCount, req.WalletProfile.FreshnessLevel))
	}
	sb.WriteString(fmt.Sprintf("- Market URL: %s\n", req.MarketURL))
	sb.WriteString(fmt.Sprintf("- Wallet URL: %s\n\n", req.ProfileURL))

	// Add examples for style guidance
	if len(req.ExampleTweets) > 0 || len(req.HistoricalTweets) > 0 {
		sb.WriteString("## Reference Tweets (match this style)\n")
		for i, tweet := range req.ExampleTweets {
			if i >= 2 {
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", tweet))
		}
		for i, tweet := range req.HistoricalTweets {
			if i >= 2 {
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", tweet))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Task\n")
	sb.WriteString("Review and improve this tweet for:\n")
	sb.WriteString("1. Clarity and accuracy\n")
	sb.WriteString("2. Engagement (hooks, emojis)\n")
	sb.WriteString("3. Keep under 280 characters\n")
	sb.WriteString("4. Match the style of reference tweets\n\n")
	sb.WriteString("Output ONLY the improved tweet, nothing else.")

	return sb.String()
}

func (a *TweetAgent) callLLM(ctx context.Context, systemPrompt, userPrompt string) (string, int, error) {
	reqBody := map[string]any{
		"model":       a.config.Model,
		"max_tokens":  a.config.MaxTokens,
		"temperature": a.config.Temperature,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := strings.TrimSuffix(a.config.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+a.config.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return "", 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read response: %w", err)
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", 0, fmt.Errorf("failed to parse response: %w", err)
	}

	if chatResp.Error != nil {
		return "", 0, fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", 0, fmt.Errorf("no response generated")
	}

	tokens := 0
	if chatResp.Usage != nil {
		tokens = chatResp.Usage.TotalTokens
	}

	return chatResp.Choices[0].Message.Content, tokens, nil
}

func (a *TweetAgent) truncateTweet(text string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 280
	}
	if len(text) <= maxLen {
		return text
	}

	// Try to truncate at word boundary
	truncated := text[:maxLen-3]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxLen/2 {
		truncated = truncated[:lastSpace]
	}
	return truncated + "..."
}

func shortenAddress(addr string) string {
	if len(addr) <= 10 {
		return addr
	}
	return addr[:6] + "..." + addr[len(addr)-4:]
}

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

// Ensure TweetAgent implements ports.ActionAgent
var _ ports.ActionAgent = (*TweetAgent)(nil)
