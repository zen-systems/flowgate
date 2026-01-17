package adapter

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/zen-systems/flowgate/pkg/artifact"
)

// AnthropicAdapter implements the Adapter interface for Claude models.
type AnthropicAdapter struct {
	client anthropic.Client
}

// NewAnthropicAdapter creates a new Anthropic adapter.
func NewAnthropicAdapter(apiKey string) (*AnthropicAdapter, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic API key is required")
	}

	client := anthropic.NewClient()
	return &AnthropicAdapter{client: client}, nil
}

// Name returns the adapter identifier.
func (a *AnthropicAdapter) Name() string {
	return "anthropic"
}

// Models returns the list of supported Claude models.
func (a *AnthropicAdapter) Models() []string {
	return []string{
		"claude-sonnet-4-20250514",
		"claude-opus-4-20250514",
	}
}

// Generate sends a prompt to Claude and returns the response as an artifact.
func (a *AnthropicAdapter) Generate(ctx context.Context, model string, prompt string) (*artifact.Artifact, error) {
	resp, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 4096,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic API error: %w", err)
	}

	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return artifact.New(content, a.Name(), model, prompt), nil
}
