package main

import (
	"testing"

	"github.com/hession/aimate/internal/config"
)

func TestLogConfigInfo(t *testing.T) {
	// Test with full API key (> 8 chars)
	cfg := &config.Config{
		Model: config.ModelConfig{
			APIKey:      "test-api-key-12345",
			BaseURL:     "https://api.test.com",
			Model:       "test-model",
			Temperature: 0.7,
			MaxTokens:   1000,
		},
		Memory: config.MemoryConfig{
			DBPath:             "/tmp/test.db",
			MaxContextMessages: 20,
		},
		Safety: config.SafetyConfig{
			ConfirmDangerousOps: true,
		},
	}

	// Should not panic
	logConfigInfo(cfg)
}

func TestLogConfigInfo_ShortAPIKey(t *testing.T) {
	// Test with short API key (<= 8 chars)
	cfg := &config.Config{
		Model: config.ModelConfig{
			APIKey:      "short",
			BaseURL:     "https://api.test.com",
			Model:       "test-model",
			Temperature: 0.7,
			MaxTokens:   1000,
		},
		Memory: config.MemoryConfig{
			DBPath:             "/tmp/test.db",
			MaxContextMessages: 20,
		},
		Safety: config.SafetyConfig{
			ConfirmDangerousOps: false,
		},
	}

	// Should not panic
	logConfigInfo(cfg)
}

func TestLogConfigInfo_EmptyAPIKey(t *testing.T) {
	// Test with empty API key
	cfg := &config.Config{
		Model: config.ModelConfig{
			APIKey:      "",
			BaseURL:     "https://api.test.com",
			Model:       "test-model",
			Temperature: 0.7,
			MaxTokens:   1000,
		},
		Memory: config.MemoryConfig{
			DBPath:             "/tmp/test.db",
			MaxContextMessages: 20,
		},
		Safety: config.SafetyConfig{
			ConfirmDangerousOps: true,
		},
	}

	// Should not panic
	logConfigInfo(cfg)
}

func TestVersion(t *testing.T) {
	if version != "0.1.0" {
		t.Errorf("Expected version '0.1.0', got '%s'", version)
	}
}
