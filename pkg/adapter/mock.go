package adapter

import (
	"context"
	"fmt"

	"github.com/zen-systems/flowgate/pkg/artifact"
)

// MockAdapter returns deterministic responses for local runs and tests.
type MockAdapter struct {
	responses       map[string]string
	defaultResponse string
	Usage           *Usage
}

// NewMockAdapter creates a mock adapter with a default response.
func NewMockAdapter() *MockAdapter {
	return &MockAdapter{
		responses:       make(map[string]string),
		defaultResponse: "mock response:",
	}
}

// NewMockAdapterWithResponses creates a mock adapter with predefined responses.
func NewMockAdapterWithResponses(responses map[string]string, defaultResponse string) *MockAdapter {
	if defaultResponse == "" {
		defaultResponse = "mock response:"
	}
	return &MockAdapter{responses: responses, defaultResponse: defaultResponse}
}

// Name returns the adapter identifier.
func (a *MockAdapter) Name() string {
	return "mock"
}

// Models returns the list of supported mock models.
func (a *MockAdapter) Models() []string {
	return []string{"mock-1"}
}

// Generate returns a deterministic artifact for the prompt.
func (a *MockAdapter) Generate(_ context.Context, model string, prompt string) (*Response, error) {
	if model == "" {
		model = "mock-1"
	}
	if response, ok := a.responses[prompt]; ok {
		art := artifact.New(response, a.Name(), model, prompt)
		return &Response{Artifact: art, Usage: a.Usage}, nil
	}
	content := fmt.Sprintf("%s\n%s", a.defaultResponse, prompt)
	art := artifact.New(content, a.Name(), model, prompt)
	return &Response{Artifact: art, Usage: a.Usage}, nil
}
