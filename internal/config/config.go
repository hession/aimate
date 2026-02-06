package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// configDir is the configuration directory path
	// Can be set via SetConfigDir before loading config
	configDir     string
	configDirInit bool
)

// SetConfigDir sets a custom configuration directory
// Must be called before any config loading functions
func SetConfigDir(dir string) {
	configDir = dir
	configDirInit = true
}

// GetConfigDir returns the configuration directory
// Priority: 1. Manually set via SetConfigDir, 2. ./config in current directory
func GetConfigDir() string {
	if !configDirInit {
		// Default to ./config in current working directory
		cwd, err := os.Getwd()
		if err == nil {
			configDir = filepath.Join(cwd, "config")
		}
		configDirInit = true
	}
	return configDir
}

// Config application configuration structure
type Config struct {
	Model     ModelConfig     `yaml:"model"`
	Memory    MemoryConfig    `yaml:"memory"`
	Safety    SafetyConfig    `yaml:"safety"`
	WebSearch WebSearchConfig `yaml:"web_search"`
}

// ModelConfig LLM model configuration
type ModelConfig struct {
	APIKey      string  `yaml:"api_key"`
	BaseURL     string  `yaml:"base_url"`
	Model       string  `yaml:"model"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
}

// MemoryConfig memory storage configuration
type MemoryConfig struct {
	DBPath             string `yaml:"db_path"`
	MaxContextMessages int    `yaml:"max_context_messages"`
}

// SafetyConfig safety configuration
type SafetyConfig struct {
	ConfirmDangerousOps bool `yaml:"confirm_dangerous_ops"`
}

// WebSearchConfig web search configuration
type WebSearchConfig struct {
	Provider       string `yaml:"provider"`
	BaseURL        string `yaml:"base_url"`
	APIKey         string `yaml:"api_key"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	DefaultLimit   int    `yaml:"default_limit"`
	UserAgent      string `yaml:"user_agent"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		Model: ModelConfig{
			APIKey:      "",
			BaseURL:     "https://api.deepseek.com",
			Model:       "deepseek-chat",
			Temperature: 0.7,
			MaxTokens:   4096,
		},
		Memory: MemoryConfig{
			DBPath:             filepath.Join(homeDir, ".aimate", "memory.db"),
			MaxContextMessages: 20,
		},
		Safety: SafetyConfig{
			ConfirmDangerousOps: true,
		},
		WebSearch: WebSearchConfig{
			Provider:       "duckduckgo",
			BaseURL:        "https://api.duckduckgo.com",
			APIKey:         "",
			TimeoutSeconds: 15,
			DefaultLimit:   5,
			UserAgent:      "AIMate/0.1",
		},
	}
}

// ConfigDir returns the configuration directory path
func ConfigDir() (string, error) {
	dir := GetConfigDir()
	if dir == "" {
		return "", fmt.Errorf("failed to determine config directory")
	}
	return dir, nil
}

// LogDir returns the log directory path
func LogDir() string {
	dir := GetConfigDir()
	if dir == "" {
		return "logs"
	}
	return filepath.Join(dir, "logs")
}

// ConfigPath returns the configuration file path
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load loads configuration from file and merges with secrets
func Load() (*Config, error) {
	configPath, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Config file doesn't exist, create default config
		cfg := DefaultConfig()

		// Load secrets and merge API key
		secrets, _ := LoadSecrets()
		if secrets != nil {
			if apiKey := secrets.GetDeepSeekAPIKey(); apiKey != "" {
				cfg.Model.APIKey = apiKey
			}
			if webKey := secrets.GetWebSearchAPIKey(); webKey != "" {
				cfg.WebSearch.APIKey = webKey
			}
		}

		if err := Save(cfg); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return cfg, nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse config
	cfg := DefaultConfig() // Use default values as base
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Load secrets and merge API key if not set in config
	secrets, _ := LoadSecrets()
	if secrets != nil {
		if cfg.Model.APIKey == "" {
			if apiKey := secrets.GetDeepSeekAPIKey(); apiKey != "" {
				cfg.Model.APIKey = apiKey
			}
		}
		if cfg.WebSearch.APIKey == "" {
			if webKey := secrets.GetWebSearchAPIKey(); webKey != "" {
				cfg.WebSearch.APIKey = webKey
			}
		}
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save saves configuration to file
func Save(cfg *Config) error {
	configPath, err := ConfigPath()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Serialize config
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	// Add header comment
	content := "# AIMate Configuration File\n# For more info: https://github.com/hession/aimate\n\n" + string(data)

	// Write file
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate model config
	if c.Model.BaseURL == "" {
		return fmt.Errorf("config error: model.base_url cannot be empty")
	}
	if c.Model.Model == "" {
		return fmt.Errorf("config error: model.model cannot be empty")
	}
	if c.Model.Temperature < 0 || c.Model.Temperature > 2 {
		return fmt.Errorf("config error: model.temperature must be between 0 and 2")
	}
	if c.Model.MaxTokens <= 0 {
		return fmt.Errorf("config error: model.max_tokens must be greater than 0")
	}

	// Validate memory config
	if c.Memory.DBPath == "" {
		return fmt.Errorf("config error: memory.db_path cannot be empty")
	}
	if c.Memory.MaxContextMessages <= 0 {
		return fmt.Errorf("config error: memory.max_context_messages must be greater than 0")
	}

	// Validate web search config
	provider := strings.ToLower(strings.TrimSpace(c.WebSearch.Provider))
	if provider == "" {
		provider = "duckduckgo"
	}
	if provider == "searxng" && strings.TrimSpace(c.WebSearch.BaseURL) == "" {
		return fmt.Errorf("config error: web_search.base_url cannot be empty for searxng provider")
	}
	if c.WebSearch.TimeoutSeconds <= 0 {
		return fmt.Errorf("config error: web_search.timeout_seconds must be greater than 0")
	}
	if c.WebSearch.DefaultLimit <= 0 {
		return fmt.Errorf("config error: web_search.default_limit must be greater than 0")
	}

	return nil
}

// IsAPIKeyConfigured checks if API key is configured
func (c *Config) IsAPIKeyConfigured() bool {
	return c.Model.APIKey != ""
}

// String returns string representation of config (hides sensitive info)
func (c *Config) String() string {
	apiKeyDisplay := "(not configured)"
	if c.Model.APIKey != "" {
		if len(c.Model.APIKey) > 8 {
			apiKeyDisplay = c.Model.APIKey[:8] + "..." // Only show first 8 chars
		} else {
			apiKeyDisplay = "***"
		}
	}

	return fmt.Sprintf(`AIMate Configuration:
  Model:
    API Key: %s
    Base URL: %s
    Model: %s
    Temperature: %.1f
    Max Tokens: %d
  Memory:
    DB Path: %s
    Max Context Messages: %d
  Safety:
    Confirm Dangerous Ops: %v
  Web Search:
    Provider: %s
    Base URL: %s
    API Key: %s
    Timeout Seconds: %d
    Default Limit: %d
    User Agent: %s`,
		apiKeyDisplay,
		c.Model.BaseURL,
		c.Model.Model,
		c.Model.Temperature,
		c.Model.MaxTokens,
		c.Memory.DBPath,
		c.Memory.MaxContextMessages,
		c.Safety.ConfirmDangerousOps,
		c.WebSearch.Provider,
		c.WebSearch.BaseURL,
		redactAPIKey(c.WebSearch.APIKey),
		c.WebSearch.TimeoutSeconds,
		c.WebSearch.DefaultLimit,
		c.WebSearch.UserAgent,
	)
}

func redactAPIKey(value string) string {
	if value == "" {
		return "(not configured)"
	}
	if len(value) > 8 {
		return value[:8] + "..."
	}
	return "***"
}
