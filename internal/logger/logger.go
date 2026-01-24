package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// LogLevel defines log level
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger is a simple logger with daily rotation
type Logger struct {
	mu          sync.Mutex
	level       LogLevel
	logDir      string
	maxDays     int
	currentFile *os.File
	currentDate string
	consoleOut  bool // Whether to output to console
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// Config logger configuration
type Config struct {
	LogDir     string   // Log directory
	Level      LogLevel // Log level
	MaxDays    int      // Max days to keep logs
	ConsoleOut bool     // Output to console as well
}

// Init initializes the default logger
func Init(cfg Config) error {
	var err error
	once.Do(func() {
		defaultLogger, err = NewLogger(cfg)
	})
	return err
}

// NewLogger creates a new logger instance
func NewLogger(cfg Config) (*Logger, error) {
	if cfg.MaxDays <= 0 {
		cfg.MaxDays = 7
	}

	// Ensure log directory exists
	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	l := &Logger{
		level:      cfg.Level,
		logDir:     cfg.LogDir,
		maxDays:    cfg.MaxDays,
		consoleOut: cfg.ConsoleOut,
	}

	// Open initial log file
	if err := l.rotateIfNeeded(); err != nil {
		return nil, err
	}

	return l, nil
}

// rotateIfNeeded checks if log rotation is needed and performs it
func (l *Logger) rotateIfNeeded() error {
	today := time.Now().Format("2006-01-02")
	if l.currentDate == today && l.currentFile != nil {
		return nil
	}

	// Close current file
	if l.currentFile != nil {
		l.currentFile.Close()
	}

	// Open new log file
	filename := filepath.Join(l.logDir, fmt.Sprintf("aimate-%s.log", today))
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	l.currentFile = f
	l.currentDate = today

	// Clean old log files
	go l.cleanOldLogs()

	return nil
}

// cleanOldLogs removes log files older than maxDays
func (l *Logger) cleanOldLogs() {
	files, err := filepath.Glob(filepath.Join(l.logDir, "aimate-*.log"))
	if err != nil {
		return
	}

	if len(files) <= l.maxDays {
		return
	}

	// Sort files by name (which is by date)
	sort.Strings(files)

	// Remove old files
	for i := 0; i < len(files)-l.maxDays; i++ {
		os.Remove(files[i])
	}
}

// log writes a log message
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if rotation is needed
	if err := l.rotateIfNeeded(); err != nil {
		fmt.Fprintf(os.Stderr, "Logger rotation error: %v\n", err)
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level.String(), message)

	// Write to file
	if l.currentFile != nil {
		l.currentFile.WriteString(logLine)
	}

	// Optionally write to console
	if l.consoleOut {
		fmt.Print(logLine)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Close closes the logger
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.currentFile != nil {
		return l.currentFile.Close()
	}
	return nil
}

// GetWriter returns an io.Writer for the logger at the specified level
func (l *Logger) GetWriter(level LogLevel) io.Writer {
	return &logWriter{logger: l, level: level}
}

// logWriter implements io.Writer interface
type logWriter struct {
	logger *Logger
	level  LogLevel
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		w.logger.log(w.level, "%s", msg)
	}
	return len(p), nil
}

// Package-level functions using the default logger

// Debug logs a debug message using the default logger
func Debug(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Debug(format, args...)
	}
}

// Info logs an info message using the default logger
func Info(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Info(format, args...)
	}
}

// Warn logs a warning message using the default logger
func Warn(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Warn(format, args...)
	}
}

// Error logs an error message using the default logger
func Error(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Error(format, args...)
	}
}

// Close closes the default logger
func Close() error {
	if defaultLogger != nil {
		return defaultLogger.Close()
	}
	return nil
}

// GetDefault returns the default logger
func GetDefault() *Logger {
	return defaultLogger
}
