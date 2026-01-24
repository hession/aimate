package main

import (
	"fmt"
	"os"

	"github.com/hession/aimate/internal/cli"
	"github.com/hession/aimate/internal/config"
	"github.com/hession/aimate/internal/logger"
	"github.com/spf13/cobra"
)

var (
	version   = "0.1.0"
	configDir string // Configuration directory flag
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "aimate",
		Short: "AIMate - Your AI Work Companion",
		Long: `AIMate is an intelligent AI work companion that understands your intent and helps complete various tasks.

It can:
  • Have natural language conversations with you
  • Read and write files
  • Execute system commands
  • Search file contents
  • Remember information you tell it`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Set config directory if specified
			if configDir != "" {
				config.SetConfigDir(configDir)
			}

			// Initialize logger
			logDir := config.LogDir()
			if err := logger.Init(logger.Config{
				LogDir:     logDir,
				Level:      logger.INFO,
				MaxDays:    7,
				ConsoleOut: false, // Don't output to console
			}); err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Log configuration loaded
			logger.Info("Configuration loaded successfully")
			logConfigInfo(cfg)

			// Start CLI
			return cli.Run(cfg)
		},
	}

	// Add persistent flags
	rootCmd.PersistentFlags().StringVar(&configDir, "config-dir", "", "Configuration directory (default: ./config)")

	// config subcommand
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Show or manage configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Log config info to file
			logConfigInfo(cfg)

			// Show config file path only in terminal
			path, _ := config.ConfigPath()
			fmt.Printf("Config file path: %s\n", path)
			fmt.Printf("Log directory: %s\n", config.LogDir())
			return nil
		},
	}

	// version subcommand
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("AIMate v%s\n", version)
		},
	}

	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Close logger on exit
	logger.Close()
}

// logConfigInfo logs configuration information to log file
func logConfigInfo(cfg *config.Config) {
	apiKeyDisplay := "(not configured)"
	if cfg.Model.APIKey != "" {
		if len(cfg.Model.APIKey) > 8 {
			apiKeyDisplay = cfg.Model.APIKey[:8] + "..."
		} else {
			apiKeyDisplay = "***"
		}
	}

	logger.Info("AIMate Configuration:")
	logger.Info("  Model.APIKey: %s", apiKeyDisplay)
	logger.Info("  Model.BaseURL: %s", cfg.Model.BaseURL)
	logger.Info("  Model.Model: %s", cfg.Model.Model)
	logger.Info("  Model.Temperature: %.1f", cfg.Model.Temperature)
	logger.Info("  Model.MaxTokens: %d", cfg.Model.MaxTokens)
	logger.Info("  Memory.DBPath: %s", cfg.Memory.DBPath)
	logger.Info("  Memory.MaxContextMessages: %d", cfg.Memory.MaxContextMessages)
	logger.Info("  Safety.ConfirmDangerousOps: %v", cfg.Safety.ConfirmDangerousOps)
}
