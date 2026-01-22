package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// RoutingConfig holds the routing rules configuration.
type RoutingConfig struct {
	TaskTypes                     map[string]TaskType `yaml:"task_types"`
	Default                       RouteTarget         `yaml:"default"`
	Retry                         RetryConfig         `yaml:"retry,omitempty"`
	Fallback                      FallbackConfig      `yaml:"fallback,omitempty"`
	Pricing                       PricingConfig       `yaml:"pricing,omitempty"`
	ClassifierAdapter             string              `yaml:"classifier_adapter,omitempty"`
	ClassifierModel               string              `yaml:"classifier_model,omitempty"`
	ClassifierConfidenceThreshold float64             `yaml:"classifier_confidence_threshold,omitempty"`
	EnableLLMTieBreaker           *bool               `yaml:"enable_llm_tie_breaker,omitempty"`
}

// TaskType defines a category of tasks with routing rules.
type TaskType struct {
	Triggers []string `yaml:"triggers"`
	Adapter  string   `yaml:"adapter"`
	Model    string   `yaml:"model"`
}

// RouteTarget specifies an adapter and model combination.
type RouteTarget struct {
	Adapter string `yaml:"adapter"`
	Model   string `yaml:"model"`
}

// RetryConfig defines retry and backoff behavior.
type RetryConfig struct {
	MaxRetries    int `yaml:"max_retries,omitempty"`
	BaseBackoffMs int `yaml:"base_backoff_ms,omitempty"`
	MaxBackoffMs  int `yaml:"max_backoff_ms,omitempty"`
}

// FallbackConfig defines adapter/model fallbacks.
type FallbackConfig struct {
	AllowFallback bool                     `yaml:"allow_fallback,omitempty"`
	FallbackChain map[string][]RouteTarget `yaml:"fallback_chain,omitempty"`
}

// PricingConfig maps adapter -> model -> pricing.
type PricingConfig map[string]map[string]ModelPricing

// ModelPricing defines per-1k token pricing.
type ModelPricing struct {
	PromptPer1K     float64 `yaml:"prompt_per_1k,omitempty"`
	CompletionPer1K float64 `yaml:"completion_per_1k,omitempty"`
}

// LoadRoutingConfig reads routing configuration from a YAML file.
func LoadRoutingConfig(path string) (*RoutingConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg RoutingConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	applyRoutingDefaults(&cfg)
	return &cfg, nil
}

// DefaultRoutingConfig returns the default routing configuration.
func DefaultRoutingConfig() *RoutingConfig {
	cfg := &RoutingConfig{
		TaskTypes: map[string]TaskType{
			"research": {
				Triggers: []string{"research", "find", "look up", "what is", "compare"},
				Adapter:  "google",
				Model:    "gemini-2.0-pro",
			},
			"summarize": {
				Triggers: []string{"summarize", "tldr", "key points"},
				Adapter:  "openai",
				Model:    "gpt-5.2-thinking",
			},
			"outline": {
				Triggers: []string{"outline", "plan", "structure", "organize"},
				Adapter:  "openai",
				Model:    "gpt-5.2-instant",
			},
			"scaffold": {
				Triggers: []string{"scaffold", "boilerplate", "starter", "template"},
				Adapter:  "openai",
				Model:    "gpt-5.2-codex",
			},
			"implement": {
				Triggers: []string{"implement", "code", "write a function", "build", "create"},
				Adapter:  "anthropic",
				Model:    "claude-sonnet-4-20250514",
			},
			"refactor": {
				Triggers: []string{"refactor", "large refactor", "migrate", "rewrite"},
				Adapter:  "openai",
				Model:    "gpt-5.2-codex",
			},
			"debug": {
				Triggers: []string{"debug", "fix", "error", "bug", "failing"},
				Adapter:  "anthropic",
				Model:    "claude-sonnet-4-20250514",
			},
			"review": {
				Triggers: []string{"review", "check", "audit", "evaluate"},
				Adapter:  "anthropic",
				Model:    "claude-sonnet-4-20250514",
			},
			"math": {
				Triggers: []string{"calculate", "equation", "formula", "proof", "derive"},
				Adapter:  "openai",
				Model:    "gpt-5.2-pro",
			},
			"security_review": {
				Triggers: []string{"security", "vulnerability", "audit security", "penetration", "exploit"},
				Adapter:  "anthropic",
				Model:    "claude-opus-4-20250514",
			},
			"architecture": {
				Triggers: []string{"architect", "system design", "design review", "architecture"},
				Adapter:  "anthropic",
				Model:    "claude-opus-4-20250514",
			},
			"legal": {
				Triggers: []string{"contract", "legal", "compliance", "terms", "license"},
				Adapter:  "anthropic",
				Model:    "claude-opus-4-20250514",
			},
			"complex_debug": {
				Triggers: []string{"race condition", "memory leak", "deadlock", "concurrency bug"},
				Adapter:  "anthropic",
				Model:    "claude-opus-4-20250514",
			},
			"final_review": {
				Triggers: []string{"final review", "ship review", "pre-release", "production ready"},
				Adapter:  "anthropic",
				Model:    "claude-opus-4-20250514",
			},
			"bulk_code": {
				Triggers: []string{"bulk code", "generate multiple", "batch generate", "mass create", "generate all"},
				Adapter:  "deepseek",
				Model:    "deepseek-coder",
			},
			"reasoning": {
				Triggers: []string{"reason", "think through", "step by step", "logical", "deduce", "infer"},
				Adapter:  "deepseek",
				Model:    "deepseek-reasoner",
			},
		},
		Default: RouteTarget{
			Adapter: "anthropic",
			Model:   "claude-sonnet-4-20250514",
		},
	}

	applyRoutingDefaults(cfg)
	return cfg
}

func applyRoutingDefaults(cfg *RoutingConfig) {
	if cfg == nil {
		return
	}
	if cfg.Retry.MaxRetries == 0 {
		cfg.Retry.MaxRetries = 2
	}
	if cfg.Retry.BaseBackoffMs == 0 {
		cfg.Retry.BaseBackoffMs = 200
	}
	if cfg.Retry.MaxBackoffMs == 0 {
		cfg.Retry.MaxBackoffMs = 2000
	}
	if cfg.Retry.MaxBackoffMs < cfg.Retry.BaseBackoffMs {
		cfg.Retry.MaxBackoffMs = cfg.Retry.BaseBackoffMs
	}
	if cfg.ClassifierConfidenceThreshold == 0 {
		cfg.ClassifierConfidenceThreshold = 0.65
	}
	if cfg.EnableLLMTieBreaker == nil {
		enabled := true
		cfg.EnableLLMTieBreaker = &enabled
	}
}
