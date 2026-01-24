package main

import (
	"fmt"
	"os"

	"github.com/hession/aimate/internal/cli"
	"github.com/hession/aimate/internal/config"
	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Start CLI
			return cli.Run(cfg)
		},
	}

	// config subcommand
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Show or manage configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			fmt.Println(cfg.String())

			path, _ := config.ConfigPath()
			fmt.Printf("\nConfig file path: %s\n", path)
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
}
