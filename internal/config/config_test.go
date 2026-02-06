package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Model.BaseURL != "https://api.deepseek.com" {
		t.Errorf("Expected BaseURL to be https://api.deepseek.com, got %s", cfg.Model.BaseURL)
	}

	if cfg.Model.Model != "deepseek-chat" {
		t.Errorf("Expected Model to be deepseek-chat, got %s", cfg.Model.Model)
	}

	if cfg.Memory.MaxContextMessages != 20 {
		t.Errorf("Expected MaxContextMessages to be 20, got %d", cfg.Memory.MaxContextMessages)
	}

	if cfg.WebSearch.Provider != "duckduckgo" {
		t.Errorf("Expected WebSearch provider to be duckduckgo, got %s", cfg.WebSearch.Provider)
	}

	if !cfg.Safety.ConfirmDangerousOps {
		t.Error("Expected ConfirmDangerousOps to be true")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     DefaultConfig(),
			wantErr: false,
		},
		{
			name: "empty BaseURL",
			cfg: &Config{
				Model: ModelConfig{
					BaseURL:     "",
					Model:       "test",
					Temperature: 0.7,
					MaxTokens:   1000,
				},
				Memory: MemoryConfig{
					DBPath:             "/tmp/test.db",
					MaxContextMessages: 20,
				},
				WebSearch: WebSearchConfig{
					Provider:       "duckduckgo",
					BaseURL:        "https://api.duckduckgo.com",
					TimeoutSeconds: 15,
					DefaultLimit:   5,
					UserAgent:      "AIMate/0.1",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid Temperature",
			cfg: &Config{
				Model: ModelConfig{
					BaseURL:     "https://api.test.com",
					Model:       "test",
					Temperature: 3.0, // out of range
					MaxTokens:   1000,
				},
				Memory: MemoryConfig{
					DBPath:             "/tmp/test.db",
					MaxContextMessages: 20,
				},
				WebSearch: WebSearchConfig{
					Provider:       "duckduckgo",
					BaseURL:        "https://api.duckduckgo.com",
					TimeoutSeconds: 15,
					DefaultLimit:   5,
					UserAgent:      "AIMate/0.1",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "aimate-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Set config directory for test
	configTestDir := filepath.Join(tmpDir, "config")
	SetConfigDir(configTestDir)

	// Create and save config
	cfg := DefaultConfig()
	cfg.Model.APIKey = "test-api-key"

	err = Save(cfg)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(configTestDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file not created")
	}

	// Load config
	loadedCfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loadedCfg.Model.APIKey != cfg.Model.APIKey {
		t.Errorf("API Key mismatch: expected %s, got %s", cfg.Model.APIKey, loadedCfg.Model.APIKey)
	}
}

func TestIsAPIKeyConfigured(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.IsAPIKeyConfigured() {
		t.Error("Default config should not have API Key")
	}

	cfg.Model.APIKey = "test-key"
	if !cfg.IsAPIKeyConfigured() {
		t.Error("Should return true after setting API Key")
	}
}
