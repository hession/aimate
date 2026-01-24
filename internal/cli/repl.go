package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hession/aimate/internal/agent"
	"github.com/hession/aimate/internal/config"
	"github.com/hession/aimate/internal/llm"
	"github.com/hession/aimate/internal/memory"
	"github.com/hession/aimate/internal/tools"
)

const (
	Version = "0.1.0"

	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorRed    = "\033[31m"
	colorGray   = "\033[90m"
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

	memStore, err := memory.NewSQLiteStore(cfg.Memory.DBPath)
	if err != nil {
		return fmt.Errorf("failed to initialize memory store: %w", err)
	}
	defer memStore.Close()

	// Create tool registry
	registry := tools.NewDefaultRegistry(confirmDangerousOp)

	// Create Agent
	ag, err := agent.New(
		cfg, llmClient, memStore, registry,
		agent.WithStreamHandler(streamOutput),
		agent.WithToolCallHandler(toolCallOutput),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize Agent: %w", err)
	}

	// Start REPL
	return runREPL(ag)
}

// printWelcome prints welcome message
func printWelcome() {
	fmt.Printf("\n%s🤖 AIMate v%s%s - Your AI Work Companion\n", colorCyan, Version, colorReset)
	fmt.Printf("%sType /help for help, /exit to quit%s\n\n", colorGray, colorReset)
}

// promptAPIKey prompts user to configure API Key
func promptAPIKey(cfg *config.Config) error {
	fmt.Printf("%s⚠️  API Key not configured%s\n\n", colorYellow, colorReset)
	fmt.Printf("Please enter your DeepSeek API Key: ")

	reader := bufio.NewReader(os.Stdin)
	apiKey, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return fmt.Errorf("API Key cannot be empty")
	}

	cfg.Model.APIKey = apiKey
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\n%s✅ API Key saved%s\n\n", colorGreen, colorReset)

	// Restart
	return Run(cfg)
}

// runREPL runs the interactive REPL
func runREPL(ag *agent.Agent) error {
	reader := bufio.NewReader(os.Stdin)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Printf("\n\n%sGoodbye! 👋%s\n", colorCyan, colorReset)
		cancel()
		os.Exit(0)
	}()

	for {
		// Display prompt
		fmt.Printf("%sYou: %s", colorGreen, colorReset)

		// Read user input
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle built-in commands
		if strings.HasPrefix(input, "/") {
			if handleCommand(input, ag) {
				continue
			}
			return nil // /exit command
		}

		// Call Agent to process
		fmt.Printf("\n%sAIMate: %s", colorBlue, colorReset)

		_, err = ag.Chat(ctx, input)
		if err != nil {
			fmt.Printf("\n%s❌ Error: %v%s\n", colorRed, err, colorReset)
		}

		fmt.Println()
		fmt.Println()
	}
}

// handleCommand handles built-in commands, returns true to continue loop, false to exit
func handleCommand(cmd string, ag *agent.Agent) bool {
	switch strings.ToLower(cmd) {
	case "/help":
		printHelp()
		return true

	case "/clear":
		if err := ag.ClearSession(); err != nil {
			fmt.Printf("%s❌ Failed to clear session: %v%s\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("%s✅ Session cleared%s\n", colorGreen, colorReset)
		}
		return true

	case "/exit", "/quit", "/q":
		fmt.Printf("%sGoodbye! 👋%s\n", colorCyan, colorReset)
		return false

	case "/config":
		cfg, err := config.Load()
		if err != nil {
			fmt.Printf("%s❌ Failed to load config: %v%s\n", colorRed, err, colorReset)
		} else {
			fmt.Println(cfg.String())
		}
		return true

	case "/new":
		if err := ag.NewSession(); err != nil {
			fmt.Printf("%s❌ Failed to create new session: %v%s\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("%s✅ New session created%s\n", colorGreen, colorReset)
		}
		return true

	default:
		fmt.Printf("%s❓ Unknown command: %s%s\n", colorYellow, cmd, colorReset)
		fmt.Println("Type /help for available commands")
		return true
	}
}

// printHelp prints help information
func printHelp() {
	fmt.Printf(`
%s📚 AIMate Help%s

%sBuilt-in Commands:%s
  /help    - Show this help message
  /clear   - Clear current session history
  /new     - Create new session
  /config  - Show current configuration
  /exit    - Exit program

%sAvailable Tools:%s
  • read_file    - Read file content
  • write_file   - Write file content
  • list_dir     - List directory content
  • run_command  - Execute shell command
  • search_files - Search file content

%sExamples:%s
  "Show me the files in current directory"
  "Read the content of main.go"
  "Remember that my project uses Go"
  "Create a file hello.txt with content Hello World"

`, colorCyan, colorReset, colorYellow, colorReset, colorYellow, colorReset, colorYellow, colorReset)
}

// streamOutput handles stream output
func streamOutput(content string) {
	fmt.Print(content)
}

// toolCallOutput handles tool call output
func toolCallOutput(name string, args map[string]any, result string, err error) {
	fmt.Printf("\n\n%s🔧 Calling tool: %s%s\n", colorYellow, name, colorReset)

	// Display arguments
	if len(args) > 0 {
		fmt.Printf("%s   Args: %v%s\n", colorGray, args, colorReset)
	}

	// Display status
	if err != nil {
		fmt.Printf("%s   Status: ❌ Failed - %v%s\n", colorRed, err, colorReset)
	} else {
		fmt.Printf("%s   Status: ✅ Done%s\n", colorGreen, colorReset)
	}

	fmt.Println()
}

// confirmDangerousOp confirms dangerous operation
func confirmDangerousOp(command string) bool {
	fmt.Printf("\n%s⚠️  Dangerous Operation Warning%s\n", colorRed, colorReset)
	fmt.Printf("About to execute: %s\n", command)
	fmt.Printf("Confirm execution? (y/N): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes"
}
