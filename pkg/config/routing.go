package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// RoutingConfig holds the routing rules configuration.
type RoutingConfig struct {
	TaskTypes map[string]TaskType `yaml:"task_types"`
	Default   RouteTarget         `yaml:"default"`
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

	return &cfg, nil
}

// DefaultRoutingConfig returns the default routing configuration.
func DefaultRoutingConfig() *RoutingConfig {
	return &RoutingConfig{
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
}
