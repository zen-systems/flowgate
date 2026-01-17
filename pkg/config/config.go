package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	AnthropicAPIKey string
	OpenAIAPIKey    string
	GoogleAPIKey    string
	DeepSeekAPIKey  string
	RoutingConfig   *RoutingConfig
	ConfigDir       string
}

// FileConfig represents the structure of ~/.flowgate/config.yaml
type FileConfig struct {
	APIKeys APIKeysConfig `yaml:"api_keys"`
}

// APIKeysConfig holds API key configuration from file.
type APIKeysConfig struct {
	Anthropic string `yaml:"anthropic"`
	OpenAI    string `yaml:"openai"`
	Google    string `yaml:"google"`
	DeepSeek  string `yaml:"deepseek"`
}

// Load reads configuration from config files and environment variables.
// Environment variables take precedence over file configuration.
func Load() (*Config, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}

	// Load file config first
	fileConfig := loadFileConfig(filepath.Join(configDir, "config.yaml"))

	// Build config with env vars taking precedence over file
	cfg := &Config{
		AnthropicAPIKey: getEnvOrDefault("ANTHROPIC_API_KEY", fileConfig.APIKeys.Anthropic),
		OpenAIAPIKey:    getEnvOrDefault("OPENAI_API_KEY", fileConfig.APIKeys.OpenAI),
		GoogleAPIKey:    getEnvOrDefault("GOOGLE_API_KEY", fileConfig.APIKeys.Google),
		DeepSeekAPIKey:  getEnvOrDefault("DEEPSEEK_API_KEY", fileConfig.APIKeys.DeepSeek),
		ConfigDir:       configDir,
	}

	// Load routing config
	routingPath := filepath.Join(configDir, "routing.yaml")
	if _, err := os.Stat(routingPath); err == nil {
		routing, err := LoadRoutingConfig(routingPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load routing config: %w", err)
		}
		cfg.RoutingConfig = routing
	} else {
		cfg.RoutingConfig = DefaultRoutingConfig()
	}

	return cfg, nil
}

// LoadWithRoutingFile loads config with a specific routing file.
func LoadWithRoutingFile(routingPath string) (*Config, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}

	// Load file config first
	fileConfig := loadFileConfig(filepath.Join(configDir, "config.yaml"))

	// Build config with env vars taking precedence over file
	cfg := &Config{
		AnthropicAPIKey: getEnvOrDefault("ANTHROPIC_API_KEY", fileConfig.APIKeys.Anthropic),
		OpenAIAPIKey:    getEnvOrDefault("OPENAI_API_KEY", fileConfig.APIKeys.OpenAI),
		GoogleAPIKey:    getEnvOrDefault("GOOGLE_API_KEY", fileConfig.APIKeys.Google),
		DeepSeekAPIKey:  getEnvOrDefault("DEEPSEEK_API_KEY", fileConfig.APIKeys.DeepSeek),
		ConfigDir:       configDir,
	}

	routing, err := LoadRoutingConfig(routingPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load routing config from %s: %w", routingPath, err)
	}
	cfg.RoutingConfig = routing

	return cfg, nil
}

// HasAdapter returns true if the API key for the given adapter is configured.
func (c *Config) HasAdapter(name string) bool {
	switch name {
	case "anthropic":
		return c.AnthropicAPIKey != ""
	case "openai":
		return c.OpenAIAPIKey != ""
	case "google":
		return c.GoogleAPIKey != ""
	case "deepseek":
		return c.DeepSeekAPIKey != ""
	default:
		return false
	}
}

// loadFileConfig reads the config file, returning empty config if not found.
func loadFileConfig(path string) *FileConfig {
	cfg := &FileConfig{}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg // Return empty config if file doesn't exist
	}

	_ = yaml.Unmarshal(data, cfg) // Ignore parse errors, use defaults
	return cfg
}

// getEnvOrDefault returns the environment variable value if set,
// otherwise returns the default value.
func getEnvOrDefault(envVar, defaultValue string) string {
	if val := os.Getenv(envVar); val != "" {
		return val
	}
	return defaultValue
}

func getConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(home, ".flowgate")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return configDir, nil
}
