package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/c-bata/go-prompt"
	"github.com/hession/aimate/internal/agent"
	"github.com/hession/aimate/internal/config"
	"github.com/hession/aimate/internal/llm"
	"github.com/hession/aimate/internal/tools"
)

const (
	Version = "0.1.0"
)

// Run starts the CLI interactive interface
func Run(cfg *config.Config) error {
	// Display welcome message
	printWelcome()

	// Check API Key
	if !cfg.IsAPIKeyConfigured() {
		return promptAPIKey(cfg)
	}

	// Initialize components
	llmClient := llm.New(
		cfg.Model.APIKey,
		cfg.Model.BaseURL,
		cfg.Model.Model,
		cfg.Model.Temperature,
		cfg.Model.MaxTokens,
	)

	// Initialize memory v2
	memV2, err := agent.NewMemoryV2Integration(nil, cfg.Model.APIKey)
	if err != nil {
		return fmt.Errorf("failed to initialize memory v2: %w", err)
	}
	defer memV2.Close()

	// Set project path
	cwd, _ := os.Getwd()
	if err := memV2.SetProject(cwd); err != nil {
		// Non-fatal: log warning but continue
		fmt.Printf("Warning: failed to set project path: %v\n", err)
	}

	// Create tool registry
	registry := tools.NewDefaultRegistry(confirmDangerousOp)

	// Create Agent
	ag, err := agent.New(
		cfg, llmClient, memV2, registry,
		agent.WithStreamHandler(streamOutput),
		agent.WithToolCallHandler(toolCallOutput),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize Agent: %w", err)
	}

	// Start REPL
	return runREPL(ag, cfg)
}

// RunPrompt runs in non-interactive prompt mode with a single prompt string
func RunPrompt(cfg *config.Config, promptText string) error {
	promptText = strings.TrimSpace(promptText)
	if promptText == "" {
		return fmt.Errorf("prompt is empty")
	}

	// Check API Key
	if !cfg.IsAPIKeyConfigured() {
		return fmt.Errorf("API Key not configured")
	}

	// Initialize components
	llmClient := llm.New(
		cfg.Model.APIKey,
		cfg.Model.BaseURL,
		cfg.Model.Model,
		cfg.Model.Temperature,
		cfg.Model.MaxTokens,
	)

	// Use in-memory mode for prompt mode - v2 will use temporary storage
	memV2, err := agent.NewMemoryV2Integration(nil, cfg.Model.APIKey)
	if err != nil {
		return fmt.Errorf("failed to initialize memory v2: %w", err)
	}
	defer memV2.Close()

	// Create tool registry (disable dangerous ops in prompt mode)
	registry := tools.NewDefaultRegistry(func(string) bool {
		return false
	})

	// Create Agent
	ag, err := agent.New(
		cfg, llmClient, memV2, registry,
		agent.WithStreamHandler(streamOutput),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize Agent: %w", err)
	}

	if _, err := ag.Chat(context.Background(), promptText); err != nil {
		return err
	}

	fmt.Println()
	return nil
}

// printWelcome prints welcome message
func printWelcome() {
	fmt.Printf("\n🤖 AIMate v%s - Your AI Work Companion\n", Version)
	fmt.Printf("Type /help for help, /exit to quit\n")
	fmt.Printf("For multi-line input: enter text, then press Enter twice to submit\n\n")
}

// promptAPIKey prompts user to configure API Key
func promptAPIKey(cfg *config.Config) error {
	fmt.Printf("⚠️  API Key not configured\n\n")

	// Use go-prompt for API key input
	line := prompt.Input("Please enter your DeepSeek API Key: ", emptyCompleter,
		prompt.OptionPrefixTextColor(prompt.Yellow),
	)
	apiKey := strings.TrimSpace(line)
	if apiKey == "" {
		return fmt.Errorf("API Key cannot be empty")
	}

	cfg.Model.APIKey = apiKey
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\n✅ API Key saved\n\n")

	// Restart
	return Run(cfg)
}

// getHistoryFilePath returns the history file path
func getHistoryFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	historyDir := filepath.Join(homeDir, ".aimate")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return ""
	}
	return filepath.Join(historyDir, "history")
}

// emptyCompleter provides no auto-completion
func emptyCompleter(d prompt.Document) []prompt.Suggest {
	return []prompt.Suggest{}
}

// commandCompleter provides auto-completion for built-in commands
func commandCompleter(d prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{
		{Text: "/help", Description: "Show help message"},
		{Text: "/clear", Description: "Clear current session history"},
		{Text: "/new", Description: "Create new session"},
		{Text: "/config", Description: "Show current configuration"},
		{Text: "/history", Description: "Show history usage tips"},
		{Text: "/session", Description: "Show session status"},
		{Text: "/session list", Description: "List recent sessions"},
		{Text: "/session restore", Description: "Restore a session"},
		{Text: "/memory", Description: "Show memory statistics"},
		{Text: "/memory search", Description: "Search memories"},
		{Text: "/memory core", Description: "List core memories"},
		{Text: "/memory recent", Description: "Show recent memories"},
		{Text: "/memory diagnose", Description: "Diagnose memory system"},
		{Text: "/exit", Description: "Exit program"},
		{Text: "/quit", Description: "Exit program (alias)"},
	}
	return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
}

// runREPL runs the interactive REPL with go-prompt support
func runREPL(ag *agent.Agent, cfg *config.Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Printf("\n\nGoodbye! 👋\n")
		cancel()
		os.Exit(0)
	}()

	// Multi-line input mode
	var multiLineBuffer strings.Builder
	inMultiLine := false

	for {
		// Set prompt based on mode
		var promptText string
		var promptColor prompt.Color

		if inMultiLine {
			promptText = "...  "
			promptColor = prompt.DarkGray
		} else {
			promptText = "You: "
			promptColor = prompt.Green
		}

		// Use go-prompt for advanced input features
		line := prompt.Input(promptText, commandCompleter,
			prompt.OptionTitle("AIMate Interactive Prompt"),
			prompt.OptionPrefixTextColor(promptColor),
			prompt.OptionSuggestionTextColor(prompt.Blue),
			prompt.OptionSelectedSuggestionBGColor(prompt.LightGray),
			prompt.OptionSuggestionBGColor(prompt.DarkGray),
			prompt.OptionSuggestionTextColor(prompt.White),
			prompt.OptionDescriptionBGColor(prompt.Blue),
			prompt.OptionDescriptionTextColor(prompt.White),
		)

		// Handle multi-line input
		if inMultiLine {
			if line == "" {
				// Empty line ends multi-line input
				inMultiLine = false
				input := strings.TrimSpace(multiLineBuffer.String())
				multiLineBuffer.Reset()

				if input == "" {
					continue
				}

				// Process the input
				if err := processInput(ctx, ag, input); err != nil {
					return err
				}
				continue
			}
			// Add line to buffer
			multiLineBuffer.WriteString(line)
			multiLineBuffer.WriteString("\n")
			continue
		}

		// Single line mode
		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		// Check if starting multi-line mode (ends with backslash)
		if strings.HasSuffix(input, "\\") {
			// Start multi-line mode
			inMultiLine = true
			multiLineBuffer.WriteString(strings.TrimSuffix(input, "\\"))
			multiLineBuffer.WriteString("\n")
			fmt.Printf("(Multi-line mode: press Enter twice to submit, Ctrl+C to cancel)\n")
			continue
		}

		// Handle built-in commands
		if strings.HasPrefix(input, "/") {
			if handleCommand(input, ag) {
				continue
			}
			return nil // /exit command
		}

		// Process the input
		if err := processInput(ctx, ag, input); err != nil {
			return err
		}
	}
}

// processInput processes user input and calls agent
func processInput(ctx context.Context, ag *agent.Agent, input string) error {
	// Call Agent to process
	fmt.Printf("\nAIMate: ")

	_, err := ag.Chat(ctx, input)
	if err != nil {
		fmt.Printf("\n❌ Error: %v\n", err)
	}

	fmt.Println()
	fmt.Println()
	return nil
}

// handleCommand handles built-in commands, returns true to continue loop, false to exit
func handleCommand(cmd string, ag *agent.Agent) bool {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return true
	}

	command := strings.ToLower(parts[0])

	// Check v2 memory commands first
	memV2 := ag.GetMemoryV2()
	if memV2 != nil {
		memCommands := NewMemoryV2Commands(memV2.GetMemorySystem())
		handled, output := memCommands.HandleCommand(cmd)
		if handled {
			fmt.Println(output)
			return true
		}
	}

	switch command {
	case "/help":
		printHelp()
		return true

	case "/clear":
		if err := ag.ClearSession(); err != nil {
			fmt.Printf("❌ Failed to clear session: %v\n", err)
		} else {
			fmt.Printf("✅ Session cleared\n")
		}
		return true

	case "/exit", "/quit", "/q":
		fmt.Printf("Goodbye! 👋\n")
		return false

	case "/config":
		cfg, err := config.Load()
		if err != nil {
			fmt.Printf("❌ Failed to load config: %v\n", err)
		} else {
			fmt.Println(cfg.String())
		}
		return true

	case "/history":
		// Clear command history
		if len(parts) > 1 && parts[1] == "clear" {
			historyFile := getHistoryFilePath()
			if historyFile != "" {
				if err := os.WriteFile(historyFile, []byte{}, 0644); err != nil {
					fmt.Printf("❌ Failed to clear history: %v\n", err)
				} else {
					fmt.Printf("✅ Command history cleared\n")
				}
			}
		} else {
			fmt.Printf("Use Up/Down arrow keys to browse command history\n")
			fmt.Printf("Use /history clear to clear history\n")
		}
		return true

	default:
		fmt.Printf("❓ Unknown command: %s\n", cmd)
		fmt.Println("Type /help for available commands")
		return true
	}
}

// printHelp prints help information
func printHelp() {
	fmt.Printf(`
📚 AIMate Help

Built-in Commands:
  /help           - Show this help message
  /clear          - Clear current session history
  /new            - Create new session
  /config         - Show current configuration
  /history        - Show history usage tips
  /history clear  - Clear command history
  /exit           - Exit program

Session Commands:
  /session        - Show current session status
  /session list   - List recent sessions
  /session restore <id> - Restore a session

Memory Commands:
  /memory         - Show memory statistics
  /memory search <keyword> - Search memories
  /memory core    - List core memories
  /memory recent  - Show recent short-term memories
  /memory diagnose - Diagnose memory system
  /memory sync    - Sync index
  /memory reindex - Rebuild index
  /memory maintenance - Run maintenance tasks

Input Tips:
  • Use Backspace to delete characters
  • Use Left/Right arrow keys to move cursor
  • Use Up/Down arrow keys to browse command history
  • Use Ctrl+A/Ctrl+E to jump to start/end of line
  • Use Ctrl+W to delete word before cursor
  • Use Ctrl+U to delete line before cursor
  • Use Ctrl+K to delete line after cursor
  • Use Tab for auto-completion
  • End line with \\ for multi-line input
  • Press Enter twice to submit in multi-line mode
  • Press Ctrl+C to cancel current input

Available Tools:
  • read_file    - Read file content
  • write_file   - Write file content
  • list_dir     - List directory content
  • run_command  - Execute shell command
  • search_files - Search file content

Examples:
  "Show me the files in current directory"
  "Read the content of main.go"
  "Remember that my project uses Go"
  "Create a file hello.txt with content Hello World"

`)
}

// streamOutput handles stream output
func streamOutput(content string) {
	fmt.Print(content)
}

// toolCallOutput handles tool call output
func toolCallOutput(name string, args map[string]any, result string, err error) {
	fmt.Printf("\n\n🔧 Calling tool: %s\n", name)

	// Display arguments
	if len(args) > 0 {
		fmt.Printf("   Args: %v\n", args)
	}

	// Display status
	if err != nil {
		fmt.Printf("   Status: ❌ Failed - %v\n", err)
	} else {
		fmt.Printf("   Status: ✅ Done\n")
	}

	fmt.Println()
}

// confirmDangerousOp confirms dangerous operation
func confirmDangerousOp(command string) bool {
	fmt.Printf("\n⚠️  Dangerous Operation Warning\n")
	fmt.Printf("About to execute: %s\n", command)

	input := prompt.Input("Confirm execution? (y/N): ", emptyCompleter,
		prompt.OptionPrefixTextColor(prompt.Red),
	)
	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes"
}
