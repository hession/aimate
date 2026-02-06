package cli

import (
	"testing"
	"time"

	v2 "github.com/hession/aimate/internal/memory/v2"
)

func TestVersion(t *testing.T) {
	if Version != "0.1.0" {
		t.Errorf("Expected Version to be '0.1.0', got '%s'", Version)
	}
}

func TestTruncateForDisplay(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxLen   int
		expected string
	}{
		{
			name:     "short text",
			text:     "Hello",
			maxLen:   10,
			expected: "Hello",
		},
		{
			name:     "exact length",
			text:     "Hello",
			maxLen:   5,
			expected: "Hello",
		},
		{
			name:     "truncate",
			text:     "Hello World",
			maxLen:   5,
			expected: "Hello...",
		},
		{
			name:     "with newlines",
			text:     "Hello\nWorld",
			maxLen:   20,
			expected: "Hello World",
		},
		{
			name:     "with carriage return",
			text:     "Hello\r\nWorld",
			maxLen:   20,
			expected: "Hello World",
		},
		{
			name:     "with leading/trailing spaces",
			text:     "  Hello  ",
			maxLen:   20,
			expected: "Hello",
		},
		{
			name:     "empty string",
			text:     "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateForDisplay(tt.text, tt.maxLen)
			if got != tt.expected {
				t.Errorf("truncateForDisplay(%q, %d) = %q, want %q", tt.text, tt.maxLen, got, tt.expected)
			}
		})
	}
}

func TestGetMemoryTypeIcon(t *testing.T) {
	tests := []struct {
		memType  v2.MemoryType
		expected string
	}{
		{v2.MemoryTypeCore, "\xf0\x9f\x93\x8c"},      // pushpin emoji
		{v2.MemoryTypeSession, "\xf0\x9f\x92\xac"},   // speech balloon emoji
		{v2.MemoryTypeShortTerm, "\xf0\x9f\x93\x9d"}, // memo emoji
		{v2.MemoryTypeLongTerm, "\xf0\x9f\x93\x9a"},  // books emoji
		{v2.MemoryType("unknown"), "\xf0\x9f\x93\x84"}, // page emoji for unknown
	}

	for _, tt := range tests {
		t.Run(string(tt.memType), func(t *testing.T) {
			got := getMemoryTypeIcon(tt.memType)
			if got != tt.expected {
				t.Errorf("getMemoryTypeIcon(%s) = %q, want %q", tt.memType, got, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "seconds",
			duration: 30 * time.Second,
			expected: "30秒",
		},
		{
			name:     "minutes",
			duration: 5 * time.Minute,
			expected: "5分钟",
		},
		{
			name:     "hours",
			duration: 3 * time.Hour,
			expected: "3小时",
		},
		{
			name:     "days",
			duration: 48 * time.Hour,
			expected: "2天",
		},
		{
			name:     "less than minute",
			duration: 45 * time.Second,
			expected: "45秒",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.duration)
			if got != tt.expected {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.duration, got, tt.expected)
			}
		})
	}
}

func TestGetMemoryV2CommandSuggestions(t *testing.T) {
	suggestions := GetMemoryV2CommandSuggestions()

	if len(suggestions) == 0 {
		t.Error("GetMemoryV2CommandSuggestions should return suggestions")
	}

	// Check for expected commands
	expectedCommands := []string{"/new", "/session", "/memory", "/memory search"}
	foundCommands := make(map[string]bool)

	for _, s := range suggestions {
		foundCommands[s.Text] = true
	}

	for _, cmd := range expectedCommands {
		if !foundCommands[cmd] {
			t.Errorf("Expected command '%s' in suggestions", cmd)
		}
	}

	// Verify all suggestions have descriptions
	for _, s := range suggestions {
		if s.Description == "" {
			t.Errorf("Suggestion '%s' has empty description", s.Text)
		}
	}
}

func TestCommandSuggestion(t *testing.T) {
	cs := CommandSuggestion{
		Text:        "/test",
		Description: "Test command",
	}

	if cs.Text != "/test" {
		t.Errorf("Expected Text '/test', got '%s'", cs.Text)
	}
	if cs.Description != "Test command" {
		t.Errorf("Expected Description 'Test command', got '%s'", cs.Description)
	}
}

func TestNewMemoryV2Commands(t *testing.T) {
	// Test with nil - should not panic
	cmd := NewMemoryV2Commands(nil)
	if cmd == nil {
		t.Error("NewMemoryV2Commands should not return nil")
	}
	if cmd.memSys != nil {
		t.Error("memSys should be nil when initialized with nil")
	}
}
