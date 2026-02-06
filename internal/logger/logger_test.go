package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{LogLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("LogLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aimate-logger-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := Config{
		LogDir:     tmpDir,
		Level:      INFO,
		MaxDays:    7,
		ConsoleOut: false,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	if logger.level != INFO {
		t.Errorf("Expected level INFO, got %v", logger.level)
	}
	if logger.maxDays != 7 {
		t.Errorf("Expected maxDays 7, got %d", logger.maxDays)
	}
	if logger.logDir != tmpDir {
		t.Errorf("Expected logDir %s, got %s", tmpDir, logger.logDir)
	}
}

func TestNewLogger_DefaultMaxDays(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aimate-logger-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := Config{
		LogDir:     tmpDir,
		Level:      INFO,
		MaxDays:    0, // Should default to 7
		ConsoleOut: false,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	if logger.maxDays != 7 {
		t.Errorf("Expected default maxDays 7, got %d", logger.maxDays)
	}
}

func TestNewLogger_CreateLogDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aimate-logger-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logDir := filepath.Join(tmpDir, "logs", "subdir")
	cfg := Config{
		LogDir:     logDir,
		Level:      INFO,
		MaxDays:    7,
		ConsoleOut: false,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Verify directory was created
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Error("Log directory was not created")
	}
}

func TestLogger_LogLevels(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aimate-logger-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := Config{
		LogDir:     tmpDir,
		Level:      DEBUG,
		MaxDays:    7,
		ConsoleOut: false,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Log messages at different levels
	logger.Debug("debug message %d", 1)
	logger.Info("info message %s", "test")
	logger.Warn("warn message")
	logger.Error("error message")

	logger.Close()

	// Read the log file
	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(tmpDir, "aimate-"+today+".log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	if !strings.Contains(logContent, "[DEBUG] debug message 1") {
		t.Error("Log should contain DEBUG message")
	}
	if !strings.Contains(logContent, "[INFO] info message test") {
		t.Error("Log should contain INFO message")
	}
	if !strings.Contains(logContent, "[WARN] warn message") {
		t.Error("Log should contain WARN message")
	}
	if !strings.Contains(logContent, "[ERROR] error message") {
		t.Error("Log should contain ERROR message")
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aimate-logger-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := Config{
		LogDir:     tmpDir,
		Level:      WARN, // Only WARN and ERROR should be logged
		MaxDays:    7,
		ConsoleOut: false,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	logger.Close()

	// Read the log file
	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(tmpDir, "aimate-"+today+".log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	if strings.Contains(logContent, "[DEBUG]") {
		t.Error("DEBUG messages should be filtered out")
	}
	if strings.Contains(logContent, "[INFO]") {
		t.Error("INFO messages should be filtered out")
	}
	if !strings.Contains(logContent, "[WARN]") {
		t.Error("WARN messages should be logged")
	}
	if !strings.Contains(logContent, "[ERROR]") {
		t.Error("ERROR messages should be logged")
	}
}

func TestLogger_GetWriter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aimate-logger-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := Config{
		LogDir:     tmpDir,
		Level:      INFO,
		MaxDays:    7,
		ConsoleOut: false,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	writer := logger.GetWriter(INFO)
	if writer == nil {
		t.Fatal("GetWriter should return a writer")
	}

	// Write to the logger via io.Writer
	n, err := writer.Write([]byte("test message via writer\n"))
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if n != 24 { // "test message via writer\n" is 24 bytes
		t.Errorf("Expected to write 24 bytes, wrote %d", n)
	}
}

func TestLogger_Close(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aimate-logger-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := Config{
		LogDir:     tmpDir,
		Level:      INFO,
		MaxDays:    7,
		ConsoleOut: false,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Close should not return error
	err = logger.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Note: Closing again will return error because file is already closed
	// This is expected behavior from os.File.Close()
}

func TestPackageLevelFunctions_WithNilLogger(t *testing.T) {
	// Save and clear the default logger
	savedLogger := defaultLogger
	defaultLogger = nil
	defer func() { defaultLogger = savedLogger }()

	// These should not panic when default logger is nil
	Debug("test")
	Info("test")
	Warn("test")
	Error("test")

	err := Close()
	if err != nil {
		t.Errorf("Close with nil logger returned error: %v", err)
	}
}

func TestGetDefault(t *testing.T) {
	// Save and clear the default logger
	savedLogger := defaultLogger
	defaultLogger = nil
	defer func() { defaultLogger = savedLogger }()

	if GetDefault() != nil {
		t.Error("GetDefault should return nil when no logger is initialized")
	}
}
