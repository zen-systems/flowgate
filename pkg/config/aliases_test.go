package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve(t *testing.T) {
	aliases := &ModelAliases{
		Aliases: map[string]string{
			"fast":    "gpt-5.2-instant",
			"quality": "claude-sonnet-4-20250514",
		},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "resolve known alias",
			input:    "fast",
			expected: "gpt-5.2-instant",
		},
		{
			name:     "resolve another alias",
			input:    "quality",
			expected: "claude-sonnet-4-20250514",
		},
		{
			name:     "unknown alias returns input unchanged",
			input:    "unknown-model",
			expected: "unknown-model",
		},
		{
			name:     "canonical model returns unchanged",
			input:    "gpt-5.2-instant",
			expected: "gpt-5.2-instant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aliases.Resolve(tt.input)
			if result != tt.expected {
				t.Errorf("Resolve(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolve_NilAliases(t *testing.T) {
	var aliases *ModelAliases
	result := aliases.Resolve("fast")
	if result != "fast" {
		t.Errorf("Resolve on nil should return input, got %q", result)
	}
}

func TestIsAlias(t *testing.T) {
	aliases := &ModelAliases{
		Aliases: map[string]string{
			"fast": "gpt-5.2-instant",
		},
	}

	if !aliases.IsAlias("fast") {
		t.Error("IsAlias should return true for known alias")
	}

	if aliases.IsAlias("unknown") {
		t.Error("IsAlias should return false for unknown alias")
	}

	if aliases.IsAlias("gpt-5.2-instant") {
		t.Error("IsAlias should return false for canonical model name")
	}
}

func TestValidateModel(t *testing.T) {
	aliases := &ModelAliases{
		Providers: map[string][]string{
			"openai":    {"gpt-5.2-instant", "gpt-5.2-pro"},
			"anthropic": {"claude-sonnet-4-20250514"},
		},
	}

	tests := []struct {
		name      string
		adapter   string
		model     string
		wantError bool
	}{
		{
			name:      "valid model for provider",
			adapter:   "openai",
			model:     "gpt-5.2-instant",
			wantError: false,
		},
		{
			name:      "another valid model",
			adapter:   "anthropic",
			model:     "claude-sonnet-4-20250514",
			wantError: false,
		},
		{
			name:      "invalid model for provider",
			adapter:   "openai",
			model:     "claude-sonnet-4-20250514",
			wantError: true,
		},
		{
			name:      "unknown adapter",
			adapter:   "unknown",
			model:     "some-model",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := aliases.ValidateModel(tt.adapter, tt.model)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateModel(%q, %q) error = %v, wantError %v",
					tt.adapter, tt.model, err, tt.wantError)
			}
		})
	}
}

func TestGetProviderForModel(t *testing.T) {
	aliases := &ModelAliases{
		Providers: map[string][]string{
			"openai":    {"gpt-5.2-instant"},
			"anthropic": {"claude-sonnet-4-20250514"},
		},
	}

	tests := []struct {
		model    string
		expected string
	}{
		{"gpt-5.2-instant", "openai"},
		{"claude-sonnet-4-20250514", "anthropic"},
		{"unknown-model", ""},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := aliases.GetProviderForModel(tt.model)
			if result != tt.expected {
				t.Errorf("GetProviderForModel(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

func TestLoadAliases(t *testing.T) {
	// Create a temp file with test config
	dir := t.TempDir()
	configPath := filepath.Join(dir, "models.yaml")

	content := `aliases:
  fast: gpt-5.2-instant
  quality: claude-sonnet-4-20250514

providers:
  openai:
    - gpt-5.2-instant
  anthropic:
    - claude-sonnet-4-20250514
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	aliases, err := LoadAliases(configPath)
	if err != nil {
		t.Fatalf("LoadAliases() error = %v", err)
	}

	// Test alias resolution
	if aliases.Resolve("fast") != "gpt-5.2-instant" {
		t.Error("alias 'fast' should resolve to 'gpt-5.2-instant'")
	}

	// Test provider lookup
	if aliases.GetProviderForModel("gpt-5.2-instant") != "openai" {
		t.Error("gpt-5.2-instant should be in openai provider")
	}
}

func TestLoadAliases_FileNotFound(t *testing.T) {
	_, err := LoadAliases("/nonexistent/path/models.yaml")
	if err == nil {
		t.Error("LoadAliases should error for nonexistent file")
	}
}

func TestLoadAliasesWithFallback(t *testing.T) {
	// Temporarily rename user config if it exists
	home, _ := os.UserHomeDir()
	userConfig := filepath.Join(home, ".flowgate", "models.yaml")
	backupConfig := userConfig + ".backup"

	if _, err := os.Stat(userConfig); err == nil {
		os.Rename(userConfig, backupConfig)
		defer os.Rename(backupConfig, userConfig)
	}

	// Create a temp file for fallback
	dir := t.TempDir()
	fallbackPath := filepath.Join(dir, "models.yaml")

	content := `aliases:
  test-alias: test-model
`
	if err := os.WriteFile(fallbackPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	aliases, err := LoadAliasesWithFallback(fallbackPath)
	if err != nil {
		t.Fatalf("LoadAliasesWithFallback() error = %v", err)
	}

	if aliases.Resolve("test-alias") != "test-model" {
		t.Error("fallback config should be loaded")
	}
}

func TestLoadAliasesWithFallback_NoFile(t *testing.T) {
	// Should return empty aliases without error
	aliases, err := LoadAliasesWithFallback("/nonexistent/path/models.yaml")
	if err != nil {
		t.Fatalf("LoadAliasesWithFallback() should not error, got %v", err)
	}

	// Should return input unchanged
	if aliases.Resolve("any") != "any" {
		t.Error("empty aliases should return input unchanged")
	}
}

func TestListAliases(t *testing.T) {
	aliases := &ModelAliases{
		Aliases: map[string]string{
			"fast":    "gpt-5.2-instant",
			"quality": "claude-sonnet-4-20250514",
		},
	}

	list := aliases.ListAliases()

	if len(list) != 2 {
		t.Errorf("expected 2 aliases, got %d", len(list))
	}

	if list["fast"] != "gpt-5.2-instant" {
		t.Error("ListAliases should include 'fast' alias")
	}

	// Verify it's a copy
	list["new"] = "value"
	if aliases.Aliases["new"] == "value" {
		t.Error("ListAliases should return a copy, not the original")
	}
}

func TestValidateRoutingConfig(t *testing.T) {
	aliases := &ModelAliases{
		Aliases: map[string]string{
			"fast": "gpt-5.2-instant",
		},
		Providers: map[string][]string{
			"openai":    {"gpt-5.2-instant"},
			"anthropic": {"claude-sonnet-4-20250514"},
		},
	}

	// Valid config
	validConfig := &RoutingConfig{
		TaskTypes: map[string]TaskType{
			"task1": {Adapter: "openai", Model: "fast"},  // alias
			"task2": {Adapter: "openai", Model: "gpt-5.2-instant"}, // canonical
		},
		Default: RouteTarget{Adapter: "anthropic", Model: "claude-sonnet-4-20250514"},
	}

	errors := aliases.ValidateRoutingConfig(validConfig)
	if len(errors) != 0 {
		t.Errorf("expected no errors for valid config, got %v", errors)
	}

	// Invalid config
	invalidConfig := &RoutingConfig{
		TaskTypes: map[string]TaskType{
			"task1": {Adapter: "openai", Model: "nonexistent-model"},
		},
		Default: RouteTarget{Adapter: "anthropic", Model: "claude-sonnet-4-20250514"},
	}

	errors = aliases.ValidateRoutingConfig(invalidConfig)
	if len(errors) != 1 {
		t.Errorf("expected 1 error for invalid config, got %d", len(errors))
	}
}

func TestDefaultAliases(t *testing.T) {
	aliases := DefaultAliases()

	if aliases == nil {
		t.Fatal("DefaultAliases should not return nil")
	}

	if len(aliases.Aliases) == 0 {
		t.Error("DefaultAliases should have aliases")
	}

	if len(aliases.Providers) == 0 {
		t.Error("DefaultAliases should have providers")
	}

	// Check a known alias
	if aliases.Resolve("fast") != "gpt-5.2-instant" {
		t.Error("'fast' alias should resolve to 'gpt-5.2-instant'")
	}
}
