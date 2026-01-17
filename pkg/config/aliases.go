package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// ModelAliases manages model alias resolution and validation.
type ModelAliases struct {
	Aliases   map[string]string   `yaml:"aliases"`
	Providers map[string][]string `yaml:"providers"`
}

// LoadAliases reads model aliases from a YAML file.
func LoadAliases(path string) (*ModelAliases, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var aliases ModelAliases
	if err := yaml.Unmarshal(data, &aliases); err != nil {
		return nil, err
	}

	// Initialize maps if nil
	if aliases.Aliases == nil {
		aliases.Aliases = make(map[string]string)
	}
	if aliases.Providers == nil {
		aliases.Providers = make(map[string][]string)
	}

	return &aliases, nil
}

// LoadAliasesWithFallback loads aliases from the user config dir, falling back to
// the provided default path if not found.
func LoadAliasesWithFallback(defaultPath string) (*ModelAliases, error) {
	// Try user config directory first
	home, err := os.UserHomeDir()
	if err == nil {
		userPath := filepath.Join(home, ".flowgate", "models.yaml")
		if _, err := os.Stat(userPath); err == nil {
			return LoadAliases(userPath)
		}
	}

	// Fall back to default path
	if defaultPath != "" {
		if _, err := os.Stat(defaultPath); err == nil {
			return LoadAliases(defaultPath)
		}
	}

	// Return empty aliases if no config found
	return &ModelAliases{
		Aliases:   make(map[string]string),
		Providers: make(map[string][]string),
	}, nil
}

// Resolve returns the canonical model name for an alias.
// If the input is not an alias, it returns the input unchanged.
func (a *ModelAliases) Resolve(modelOrAlias string) string {
	if a == nil || a.Aliases == nil {
		return modelOrAlias
	}
	if canonical, ok := a.Aliases[modelOrAlias]; ok {
		return canonical
	}
	return modelOrAlias
}

// IsAlias returns true if the given string is a known alias.
func (a *ModelAliases) IsAlias(name string) bool {
	if a == nil || a.Aliases == nil {
		return false
	}
	_, ok := a.Aliases[name]
	return ok
}

// ValidateModel checks if a model exists in the provider's list.
// Returns nil if valid, or an error describing the problem.
func (a *ModelAliases) ValidateModel(adapter, model string) error {
	if a == nil || a.Providers == nil {
		return nil // No validation possible without provider info
	}

	models, ok := a.Providers[adapter]
	if !ok {
		return fmt.Errorf("unknown adapter %q", adapter)
	}

	for _, m := range models {
		if m == model {
			return nil
		}
	}

	return fmt.Errorf("model %q not in %s provider list", model, adapter)
}

// ListAliases returns a copy of the aliases map.
func (a *ModelAliases) ListAliases() map[string]string {
	if a == nil || a.Aliases == nil {
		return make(map[string]string)
	}
	result := make(map[string]string, len(a.Aliases))
	for k, v := range a.Aliases {
		result[k] = v
	}
	return result
}

// ListProviders returns a sorted list of provider names.
func (a *ModelAliases) ListProviders() []string {
	if a == nil || a.Providers == nil {
		return nil
	}
	providers := make([]string, 0, len(a.Providers))
	for p := range a.Providers {
		providers = append(providers, p)
	}
	sort.Strings(providers)
	return providers
}

// GetProviderModels returns the models for a given provider.
func (a *ModelAliases) GetProviderModels(provider string) []string {
	if a == nil || a.Providers == nil {
		return nil
	}
	return a.Providers[provider]
}

// GetProviderForModel returns the provider name for a canonical model.
func (a *ModelAliases) GetProviderForModel(model string) string {
	if a == nil || a.Providers == nil {
		return ""
	}
	for provider, models := range a.Providers {
		for _, m := range models {
			if m == model {
				return provider
			}
		}
	}
	return ""
}

// ValidateRoutingConfig checks that all models in a routing config are valid.
// Returns a slice of validation errors (empty if all valid).
func (a *ModelAliases) ValidateRoutingConfig(cfg *RoutingConfig) []error {
	if a == nil || cfg == nil {
		return nil
	}

	var errors []error

	// Validate each task type
	for name, taskType := range cfg.TaskTypes {
		model := a.Resolve(taskType.Model)
		if err := a.ValidateModel(taskType.Adapter, model); err != nil {
			errors = append(errors, fmt.Errorf("task %q: %w", name, err))
		}
	}

	// Validate default
	model := a.Resolve(cfg.Default.Model)
	if err := a.ValidateModel(cfg.Default.Adapter, model); err != nil {
		errors = append(errors, fmt.Errorf("default: %w", err))
	}

	return errors
}

// DefaultAliases returns the default model aliases configuration.
func DefaultAliases() *ModelAliases {
	return &ModelAliases{
		Aliases: map[string]string{
			// OpenAI
			"fast":      "gpt-5.2-instant",
			"fast-code": "gpt-5.2-codex",
			"thinking":  "gpt-5.2-thinking",
			"math":      "gpt-5.2-pro",
			// Anthropic
			"quality":      "claude-sonnet-4-20250514",
			"quality-code": "claude-sonnet-4-20250514",
			"deep":         "claude-opus-4-20250514",
			// Google
			"research": "gemini-2.0-pro",
			// DeepSeek
			"cheap":      "deepseek-chat",
			"cheap-code": "deepseek-coder",
			"reason":     "deepseek-reasoner",
		},
		Providers: map[string][]string{
			"anthropic": {"claude-sonnet-4-20250514", "claude-opus-4-20250514"},
			"openai":    {"gpt-5.2-instant", "gpt-5.2-thinking", "gpt-5.2-codex", "gpt-5.2-pro"},
			"google":    {"gemini-2.0-pro"},
			"deepseek":  {"deepseek-chat", "deepseek-coder", "deepseek-reasoner"},
		},
	}
}
