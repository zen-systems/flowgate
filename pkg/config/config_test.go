package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestConfigIgnoresFileAPIKeys(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)

	configDir := filepath.Join(home, ".flowgate")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	data := []byte("api_keys:\n  anthropic: file-ant\n  openai: file-openai\n  google: file-google\n  deepseek: file-deepseek\n")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("DEEPSEEK_API_KEY", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.AnthropicAPIKey != "" || cfg.OpenAIAPIKey != "" || cfg.GoogleAPIKey != "" || cfg.DeepSeekAPIKey != "" {
		t.Fatalf("expected file API keys to be ignored")
	}
}

func TestConfigUsesEnvAPIKeys(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)

	t.Setenv("ANTHROPIC_API_KEY", "env-ant")
	t.Setenv("OPENAI_API_KEY", "env-openai")
	t.Setenv("GOOGLE_API_KEY", "env-google")
	t.Setenv("DEEPSEEK_API_KEY", "env-deepseek")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.AnthropicAPIKey != "env-ant" || cfg.OpenAIAPIKey != "env-openai" || cfg.GoogleAPIKey != "env-google" || cfg.DeepSeekAPIKey != "env-deepseek" {
		t.Fatalf("expected env API keys to be used")
	}
}

func setHomeEnv(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
}
