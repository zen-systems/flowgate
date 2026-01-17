package adapter

import (
	"context"
	"fmt"

	"github.com/zen-systems/flowgate/pkg/artifact"
	"google.golang.org/genai"
)

// GoogleAdapter implements the Adapter interface for Gemini models.
type GoogleAdapter struct {
	client *genai.Client
}

// NewGoogleAdapter creates a new Google Gemini adapter.
func NewGoogleAdapter(apiKey string) (*GoogleAdapter, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("google API key is required")
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create google client: %w", err)
	}

	return &GoogleAdapter{
		client: client,
	}, nil
}

// Name returns the adapter identifier.
func (a *GoogleAdapter) Name() string {
	return "google"
}

// Models returns the list of supported Gemini models.
func (a *GoogleAdapter) Models() []string {
	return []string{
		"gemini-2.0-pro",
	}
}

// Generate sends a prompt to Gemini and returns the response as an artifact.
func (a *GoogleAdapter) Generate(ctx context.Context, model string, prompt string) (*artifact.Artifact, error) {
	resp, err := a.client.Models.GenerateContent(ctx, model, genai.Text(prompt), nil)
	if err != nil {
		return nil, fmt.Errorf("google API error: %w", err)
	}

	if resp == nil || len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("google returned no candidates")
	}

	var content string
	if resp.Candidates[0].Content != nil {
		for _, part := range resp.Candidates[0].Content.Parts {
			if part.Text != "" {
				content += part.Text
			}
		}
	}

	return artifact.New(content, a.Name(), model, prompt), nil
}
