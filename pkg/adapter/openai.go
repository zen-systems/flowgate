package adapter

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/zen-systems/flowgate/pkg/artifact"
)

// OpenAIAdapter implements the Adapter interface for OpenAI models.
type OpenAIAdapter struct {
	client openai.Client
}

// NewOpenAIAdapter creates a new OpenAI adapter.
func NewOpenAIAdapter(apiKey string) (*OpenAIAdapter, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("openai API key is required")
	}

	client := openai.NewClient()
	return &OpenAIAdapter{client: client}, nil
}

// Name returns the adapter identifier.
func (a *OpenAIAdapter) Name() string {
	return "openai"
}

// Models returns the list of supported OpenAI models.
func (a *OpenAIAdapter) Models() []string {
	return []string{
		"gpt-5.2-instant",
		"gpt-5.2-thinking",
		"gpt-5.2-codex",
		"gpt-5.2-pro",
	}
}

// Generate sends a prompt to OpenAI and returns the response as an artifact.
func (a *OpenAIAdapter) Generate(ctx context.Context, model string, prompt string) (*artifact.Artifact, error) {
	resp, err := a.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModel(model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
		MaxCompletionTokens: openai.Int(4096),
	})
	if err != nil {
		return nil, fmt.Errorf("openai API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai returned no choices")
	}

	content := resp.Choices[0].Message.Content
	return artifact.New(content, a.Name(), model, prompt), nil
}
