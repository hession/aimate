package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/chzyer/readline"
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
	return runREPL(ag, cfg)
}

// printWelcome prints welcome message
func printWelcome() {
	fmt.Printf("\n%s🤖 AIMate v%s%s - Your AI Work Companion\n", colorCyan, Version, colorReset)
	fmt.Printf("%sType /help for help, /exit to quit%s\n", colorGray, colorReset)
	fmt.Printf("%sFor multi-line input: enter text, then press Enter twice to submit%s\n\n", colorGray, colorReset)
}

// promptAPIKey prompts user to configure API Key
func promptAPIKey(cfg *config.Config) error {
	fmt.Printf("%s⚠️  API Key not configured%s\n\n", colorYellow, colorReset)

	// Create readline instance for API key input
	rl, err := readline.New("Please enter your DeepSeek API Key: ")
	if err != nil {
		return fmt.Errorf("failed to create readline: %w", err)
	}
	defer rl.Close()

	apiKey, err := rl.Readline()
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

// runREPL runs the interactive REPL with readline support
func runREPL(ag *agent.Agent, cfg *config.Config) error {
	// Configure readline
	rlConfig := &readline.Config{
		Prompt:          fmt.Sprintf("%sYou: %s", colorGreen, colorReset),
		HistoryFile:     getHistoryFilePath(),
		HistoryLimit:    1000,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",

		// Enable VIM mode (optional, can be removed)
		// VimMode:          false,

		HistorySearchFold:      true,
		DisableAutoSaveHistory: false,
	}

	rl, err := readline.NewEx(rlConfig)
	if err != nil {
		return fmt.Errorf("failed to create readline: %w", err)
	}
	defer rl.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Printf("\n\n%sGoodbye! 👋%s\n", colorCyan, colorReset)
		cancel()
		rl.Close()
		os.Exit(0)
	}()

	// Multi-line input mode
	var multiLineBuffer strings.Builder
	inMultiLine := false

	for {
		// Set prompt based on mode
		if inMultiLine {
			rl.SetPrompt(fmt.Sprintf("%s...  %s", colorGray, colorReset))
		} else {
			rl.SetPrompt(fmt.Sprintf("%sYou: %s", colorGreen, colorReset))
		}

		// Read user input with readline (supports backspace, arrow keys, history)
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				if inMultiLine {
					// Cancel multi-line mode
					multiLineBuffer.Reset()
					inMultiLine = false
					fmt.Println()
					continue
				}
				// Ctrl+C pressed, ask for confirmation
				fmt.Printf("\n%sPress Ctrl+C again or type /exit to quit%s\n", colorYellow, colorReset)
				continue
			}
			if err == io.EOF {
				fmt.Printf("\n%sGoodbye! 👋%s\n", colorCyan, colorReset)
				return nil
			}
			return fmt.Errorf("failed to read input: %w", err)
		}

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

		// Check if starting multi-line mode (ends with backslash or is a special command)
		if strings.HasSuffix(input, "\\") {
			// Start multi-line mode
			inMultiLine = true
			multiLineBuffer.WriteString(strings.TrimSuffix(input, "\\"))
			multiLineBuffer.WriteString("\n")
			fmt.Printf("%s(Multi-line mode: press Enter twice to submit, Ctrl+C to cancel)%s\n", colorGray, colorReset)
			continue
		}

		// Handle built-in commands
		if strings.HasPrefix(input, "/") {
			if handleCommand(input, ag, rl) {
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
	fmt.Printf("\n%sAIMate: %s", colorBlue, colorReset)

	_, err := ag.Chat(ctx, input)
	if err != nil {
		fmt.Printf("\n%s❌ Error: %v%s\n", colorRed, err, colorReset)
	}

	fmt.Println()
	fmt.Println()
	return nil
}

// handleCommand handles built-in commands, returns true to continue loop, false to exit
func handleCommand(cmd string, ag *agent.Agent, rl *readline.Instance) bool {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return true
	}

	command := strings.ToLower(parts[0])

	switch command {
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

	case "/history":
		// Clear command history
		if len(parts) > 1 && parts[1] == "clear" {
			historyFile := getHistoryFilePath()
			if historyFile != "" {
				if err := os.WriteFile(historyFile, []byte{}, 0644); err != nil {
					fmt.Printf("%s❌ Failed to clear history: %v%s\n", colorRed, err, colorReset)
				} else {
					fmt.Printf("%s✅ Command history cleared%s\n", colorGreen, colorReset)
				}
			}
		} else {
			fmt.Printf("%sUse Up/Down arrow keys to browse command history%s\n", colorGray, colorReset)
			fmt.Printf("%sUse /history clear to clear history%s\n", colorGray, colorReset)
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
  /help           - Show this help message
  /clear          - Clear current session history
  /new            - Create new session
  /config         - Show current configuration
  /history        - Show history usage tips
  /history clear  - Clear command history
  /exit           - Exit program

%sInput Tips:%s
  • Use Backspace to delete characters
  • Use Left/Right arrow keys to move cursor
  • Use Up/Down arrow keys to browse command history
  • Use Ctrl+A/Ctrl+E to jump to start/end of line
  • Use Ctrl+W to delete word before cursor
  • Use Ctrl+U to delete line before cursor
  • End line with \ for multi-line input
  • Press Enter twice to submit in multi-line mode
  • Press Ctrl+C to cancel current input

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

`, colorCyan, colorReset, colorYellow, colorReset, colorYellow, colorReset, colorYellow, colorReset, colorYellow, colorReset)
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

	rl, err := readline.New("Confirm execution? (y/N): ")
	if err != nil {
		return false
	}
	defer rl.Close()

	input, err := rl.Readline()
	if err != nil {
		return false
	}

	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes"
}
