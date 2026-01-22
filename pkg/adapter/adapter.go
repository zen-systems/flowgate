package adapter

import (
	"context"
)

// Adapter defines the interface for LLM provider adapters.
type Adapter interface {
	// Generate sends a prompt to the model and returns a response.
	Generate(ctx context.Context, model string, prompt string) (*Response, error)

	// Name returns the adapter's identifier.
	Name() string

	// Models returns the list of supported models.
	Models() []string
}

// AdapterInfo holds metadata about an adapter.
type AdapterInfo struct {
	Name   string
	Models []ModelInfo
}

// ModelInfo holds metadata about a model.
type ModelInfo struct {
	ID          string
	Description string
}
