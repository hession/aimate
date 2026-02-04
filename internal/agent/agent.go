package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hession/aimate/internal/config"
	"github.com/hession/aimate/internal/llm"
	v2 "github.com/hession/aimate/internal/memory/v2"
	"github.com/hession/aimate/internal/tools"
)

const (
	// MaxToolIterations maximum number of tool call iterations
	MaxToolIterations = 10
)

// Agent AI agent core
type Agent struct {
	config          *config.Config
	promptConfig    *config.PromptConfig
	llm             *llm.Client
	memoryV2        *MemoryV2Integration
	registry        *tools.Registry
	maxContextMsgs  int
	streamHandler   func(content string)
	toolCallHandler func(name string, args map[string]any, result string, err error)
}

// Option agent configuration option
type Option func(*Agent)

// WithStreamHandler sets the stream output handler
func WithStreamHandler(handler func(content string)) Option {
	return func(a *Agent) {
		a.streamHandler = handler
	}
}

// WithToolCallHandler sets the tool call handler
func WithToolCallHandler(handler func(name string, args map[string]any, result string, err error)) Option {
	return func(a *Agent) {
		a.toolCallHandler = handler
	}
}

// New creates a new Agent instance
func New(cfg *config.Config, llmClient *llm.Client, memV2 *MemoryV2Integration, reg *tools.Registry, opts ...Option) (*Agent, error) {
	// Load prompt configuration
	promptCfg, err := config.LoadPromptConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load prompt config: %w", err)
	}

	agent := &Agent{
		config:         cfg,
		promptConfig:   promptCfg,
		llm:            llmClient,
		memoryV2:       memV2,
		registry:       reg,
		maxContextMsgs: cfg.Memory.MaxContextMessages,
	}

	// Apply options
	for _, opt := range opts {
		opt(agent)
	}

	// Load or create session (v2 handles this internally)
	if err := agent.memoryV2.GetMemorySystem().Session().LoadLatestSession(); err != nil {
		// If loading fails, create a new session
		if _, err := agent.memoryV2.NewSession(); err != nil {
			return nil, fmt.Errorf("failed to initialize session: %w", err)
		}
	}

	return agent, nil
}

// NewSession creates a new session
func (a *Agent) NewSession() error {
	_, err := a.memoryV2.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

// ClearSession clears the current session
func (a *Agent) ClearSession() error {
	// Clear messages in current session
	if err := a.memoryV2.GetMemorySystem().Session().ClearMessages(); err != nil {
		return err
	}
	return nil
}

// Chat processes user message and returns response
func (a *Agent) Chat(ctx context.Context, userMessage string) (string, error) {
	// Save user message to v2 session
	tokenCount := EstimateTokens(userMessage)
	if err := a.memoryV2.AddConversation("user", userMessage, tokenCount); err != nil {
		return "", fmt.Errorf("failed to save user message: %w", err)
	}

	// Build message list
	messages, err := a.buildMessages(userMessage)
	if err != nil {
		return "", fmt.Errorf("failed to build messages: %w", err)
	}

	// Get tool schemas
	toolSchemas := a.registry.GetSchemas()
	llmTools := make([]llm.Tool, len(toolSchemas))
	for i, schema := range toolSchemas {
		llmTools[i] = llm.Tool{
			Type: schema.Type,
			Function: llm.ToolFunction{
				Name:        schema.Function.Name,
				Description: schema.Function.Description,
				Parameters:  schema.Function.Parameters,
			},
		}
	}

	// Agent loop
	var finalResponse string
	for i := 0; i < MaxToolIterations; i++ {
		// Call LLM
		var resp *llm.ChatResponse
		var err error

		if a.streamHandler != nil {
			resp, err = a.llm.ChatStream(ctx, messages, llmTools, a.streamHandler)
		} else {
			resp, err = a.llm.Chat(ctx, messages, llmTools)
		}

		if err != nil {
			return "", fmt.Errorf("failed to call LLM: %w", err)
		}

		// If no tool calls, return final response
		if len(resp.ToolCalls) == 0 {
			finalResponse = resp.Content
			break
		}

		// Process tool calls
		// Add assistant message (with tool calls)
		assistantMsg := llm.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		messages = append(messages, assistantMsg)

		// Save assistant message with tool calls to v2 session
		toolCallsJSON, _ := json.Marshal(resp.ToolCalls)
		assistantTokens := EstimateTokens(resp.Content) + EstimateTokens(string(toolCallsJSON))
		if err := a.memoryV2.GetMemorySystem().Session().AddToolMessage(
			string(toolCallsJSON), "", resp.Content, assistantTokens,
		); err != nil {
			return "", fmt.Errorf("failed to save assistant tool call message: %w", err)
		}

		// Execute each tool call
		for _, toolCall := range resp.ToolCalls {
			result, toolErr := a.executeTool(toolCall)

			// Notify tool call status
			if a.toolCallHandler != nil {
				var args map[string]any
				_ = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
				a.toolCallHandler(toolCall.Function.Name, args, result, toolErr)
			}

			// Add tool response message
			toolResultContent := result
			if toolErr != nil {
				toolResultContent = fmt.Sprintf("%s: %v", a.promptConfig.GetErrorPrefix(), toolErr)
			}

			toolMsg := llm.Message{
				Role:       "tool",
				Content:    toolResultContent,
				ToolCallID: toolCall.ID,
			}
			messages = append(messages, toolMsg)

			// Save tool message to v2 session
			toolTokens := EstimateTokens(toolResultContent)
			if err := a.memoryV2.GetMemorySystem().Session().AddToolMessage(
				"", toolCall.ID, toolResultContent, toolTokens,
			); err != nil {
				return "", fmt.Errorf("failed to save tool message: %w", err)
			}
		}
	}

	// Check if we need to save long-term memory
	a.checkAndSaveMemory(ctx, userMessage, finalResponse)

	// Save assistant response (only if it's not empty)
	if finalResponse != "" {
		assistantTokens := EstimateTokens(finalResponse)
		if err := a.memoryV2.AddConversation("assistant", finalResponse, assistantTokens); err != nil {
			return "", fmt.Errorf("failed to save assistant message: %w", err)
		}
	}

	return finalResponse, nil
}

// buildMessages builds the message list
func (a *Agent) buildMessages(userMessage string) ([]llm.Message, error) {
	// Get system prompt from config
	systemPrompt := a.promptConfig.GetSystemPrompt()

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
	}

	// Load relevant long-term memories
	memories, err := a.searchRelevantMemories(context.Background(), userMessage)
	if err == nil && len(memories) > 0 {
		var memoryContent strings.Builder
		memoryContent.WriteString(a.promptConfig.GetMemoryContext() + "\n")
		for _, mem := range memories {
			memoryContent.WriteString(fmt.Sprintf("- %s\n", mem.Content))
		}
		messages = append(messages, llm.Message{
			Role:    "system",
			Content: memoryContent.String(),
		})
	}

	// Load history messages from v2 session
	historyMsgs := a.memoryV2.GetMemorySystem().Session().GetMessages()

	// Limit to maxContextMsgs if needed
	if len(historyMsgs) > a.maxContextMsgs && a.maxContextMsgs > 0 {
		historyMsgs = historyMsgs[len(historyMsgs)-a.maxContextMsgs:]
	}

	// Convert history message format (exclude current message as it will be added at the end)
	expectedToolCalls := map[string]bool{}
	for i := 0; i < len(historyMsgs); i++ {
		msg := historyMsgs[i]
		// Skip the just-saved user message
		if msg.Role == "user" && msg.Content == userMessage {
			continue
		}

		// Skip invalid assistant messages (no content and no tool_calls)
		if msg.Role == "assistant" && msg.Content == "" && msg.ToolCalls == "" {
			continue
		}

		// Validate assistant tool call blocks (must be followed by tool responses)
		if msg.Role == "assistant" && msg.ToolCalls != "" {
			var toolCalls []llm.ToolCall
			if err := json.Unmarshal([]byte(msg.ToolCalls), &toolCalls); err != nil {
				continue
			}

			required := make(map[string]bool)
			for _, tc := range toolCalls {
				required[tc.ID] = true
			}

			found := make(map[string]bool)
			j := i + 1
			for j < len(historyMsgs) && historyMsgs[j].Role == "tool" {
				if historyMsgs[j].ToolCallID != "" {
					found[historyMsgs[j].ToolCallID] = true
				}
				j++
			}

			complete := true
			for id := range required {
				if !found[id] {
					complete = false
					break
				}
			}

			if !complete {
				// Skip incomplete tool call block to avoid invalid history
				i = j - 1
				continue
			}

			llmMsg := llm.Message{
				Role:      "assistant",
				Content:   msg.Content,
				ToolCalls: toolCalls,
			}
			messages = append(messages, llmMsg)

			expectedToolCalls = required
			continue
		}

		// Only include tool messages that match the last assistant tool_calls
		if msg.Role == "tool" {
			if len(expectedToolCalls) == 0 || msg.ToolCallID == "" {
				continue
			}
			if !expectedToolCalls[msg.ToolCallID] {
				continue
			}

			llmMsg := llm.Message{
				Role:       "tool",
				Content:    msg.Content,
				ToolCallID: msg.ToolCallID,
			}
			messages = append(messages, llmMsg)
			delete(expectedToolCalls, msg.ToolCallID)
			continue
		}

		if len(expectedToolCalls) > 0 {
			expectedToolCalls = map[string]bool{}
		}

		llmMsg := llm.Message{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}

		// Parse tool calls if present
		if msg.ToolCalls != "" {
			var toolCalls []llm.ToolCall
			if err := json.Unmarshal([]byte(msg.ToolCalls), &toolCalls); err == nil {
				llmMsg.ToolCalls = toolCalls
			}
		}

		messages = append(messages, llmMsg)
	}

	// Add current user message
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: userMessage,
	})

	return messages, nil
}

// executeTool executes a tool
func (a *Agent) executeTool(toolCall llm.ToolCall) (string, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	return a.registry.Execute(toolCall.Function.Name, args)
}

// searchRelevantMemories searches for relevant memories using v2
func (a *Agent) searchRelevantMemories(ctx context.Context, userMessage string) ([]*v2.Memory, error) {
	// Use v2 semantic search
	memories, err := a.memoryV2.SearchMemories(ctx, userMessage, 5)
	if err != nil {
		return nil, err
	}
	return memories, nil
}

// checkAndSaveMemory checks if we need to save long-term memory using v2
func (a *Agent) checkAndSaveMemory(ctx context.Context, userMessage, response string) {
	// Use v2 automatic classification and storage
	_, _ = a.memoryV2.ProcessUserMessage(ctx, userMessage)
}

// SessionID returns the current session ID
func (a *Agent) SessionID() string {
	sess := a.memoryV2.GetMemorySystem().Session().GetCurrentSession()
	if sess != nil {
		return sess.ID
	}
	return ""
}

// GetMemoryV2 returns the v2 memory integration
func (a *Agent) GetMemoryV2() *MemoryV2Integration {
	return a.memoryV2
}
