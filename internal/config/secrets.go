package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Secrets sensitive configuration loaded from .secrets file
type Secrets struct {
	values map[string]string
}

// NewSecrets creates a new Secrets instance
func NewSecrets() *Secrets {
	return &Secrets{
		values: make(map[string]string),
	}
}

// SecretsPath returns the secrets file path
func SecretsPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".secrets"), nil
}

// LoadSecrets loads secrets from the .secrets file
func LoadSecrets() (*Secrets, error) {
	secrets := NewSecrets()

	secretsPath, err := SecretsPath()
	if err != nil {
		return secrets, nil // Return empty secrets if path can't be determined
	}

	// Check if secrets file exists
	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		return secrets, nil // Return empty secrets if file doesn't exist
	}

	// Open and read the file
	file, err := os.Open(secretsPath)
	if err != nil {
		return secrets, nil // Return empty secrets on error
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key=value pairs
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			secrets.values[key] = value
		}
	}

	return secrets, scanner.Err()
}

// Get returns the value for a key
func (s *Secrets) Get(key string) string {
	if s == nil || s.values == nil {
		return ""
	}
	return s.values[key]
}

// GetOrDefault returns the value for a key, or the default value if not found
func (s *Secrets) GetOrDefault(key, defaultValue string) string {
	if s == nil || s.values == nil {
		return defaultValue
	}
	if value, ok := s.values[key]; ok && value != "" {
		return value
	}
	return defaultValue
}

// Has checks if a key exists
func (s *Secrets) Has(key string) bool {
	if s == nil || s.values == nil {
		return false
	}
	_, ok := s.values[key]
	return ok
}

// GetDeepSeekAPIKey returns the DeepSeek API key from secrets
func (s *Secrets) GetDeepSeekAPIKey() string {
	return s.Get("DEEPSEEK_API_KEY")
}

// GetWebSearchAPIKey returns the Web Search API key from secrets
func (s *Secrets) GetWebSearchAPIKey() string {
	return s.Get("WEB_SEARCH_API_KEY")
}
