package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/zen-systems/flowgate/pkg/artifact"
)

const deepseekBaseURL = "https://api.deepseek.com/v1"

// DeepSeekAdapter implements the Adapter interface for DeepSeek models.
// DeepSeek uses an OpenAI-compatible API format.
type DeepSeekAdapter struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// deepseekRequest represents the OpenAI-compatible request format.
type deepseekRequest struct {
	Model       string            `json:"model"`
	Messages    []deepseekMessage `json:"messages"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
}

// deepseekMessage represents a chat message.
type deepseekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// deepseekResponse represents the OpenAI-compatible response format.
type deepseekResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// NewDeepSeekAdapter creates a new DeepSeek adapter.
func NewDeepSeekAdapter(apiKey string) (*DeepSeekAdapter, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("deepseek API key is required")
	}

	return &DeepSeekAdapter{
		apiKey:     apiKey,
		baseURL:    deepseekBaseURL,
		httpClient: &http.Client{},
	}, nil
}

// Name returns the adapter identifier.
func (a *DeepSeekAdapter) Name() string {
	return "deepseek"
}

// Models returns the list of supported DeepSeek models.
func (a *DeepSeekAdapter) Models() []string {
	return []string{
		"deepseek-chat",
		"deepseek-coder",
		"deepseek-reasoner",
	}
}

// Generate sends a prompt to DeepSeek and returns the response as an artifact.
func (a *DeepSeekAdapter) Generate(ctx context.Context, model string, prompt string) (*artifact.Artifact, error) {
	reqBody := deepseekRequest{
		Model: model,
		Messages: []deepseekMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens: 4096,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("deepseek API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var deepseekResp deepseekResponse
	if err := json.Unmarshal(body, &deepseekResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if deepseekResp.Error != nil {
		return nil, fmt.Errorf("deepseek API error: %s (type: %s, code: %s)",
			deepseekResp.Error.Message, deepseekResp.Error.Type, deepseekResp.Error.Code)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deepseek API returned status %d: %s", resp.StatusCode, string(body))
	}

	if len(deepseekResp.Choices) == 0 {
		return nil, fmt.Errorf("deepseek returned no choices")
	}

	content := deepseekResp.Choices[0].Message.Content
	return artifact.New(content, a.Name(), model, prompt), nil
}
