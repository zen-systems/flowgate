package adapter

import "github.com/zen-systems/flowgate/pkg/artifact"

// Usage captures normalized token usage.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Cost captures normalized cost estimates.
type Cost struct {
	Currency     string  `json:"currency"`
	Amount       float64 `json:"amount"`
	IsEstimate   bool    `json:"is_estimate"`
	PricingModel string  `json:"pricing_model,omitempty"`
}

// CallReport captures adapter call metadata.
type CallReport struct {
	Adapter      string `json:"adapter"`
	Model        string `json:"model"`
	Usage        Usage  `json:"usage"`
	Cost         Cost   `json:"cost"`
	Retries      int    `json:"retries"`
	FallbackUsed bool   `json:"fallback_used"`
	Error        string `json:"error,omitempty"`
}

// Response wraps an adapter output and optional usage data.
type Response struct {
	Artifact *artifact.Artifact
	Usage    *Usage
}
