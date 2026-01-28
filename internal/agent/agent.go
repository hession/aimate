package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hession/aimate/internal/config"
	"github.com/hession/aimate/internal/llm"
	"github.com/hession/aimate/internal/memory"
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
	memory          memory.Store
	registry        *tools.Registry
	sessionID       string
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
func New(cfg *config.Config, llmClient *llm.Client, mem memory.Store, reg *tools.Registry, opts ...Option) (*Agent, error) {
	// Load prompt configuration
	promptCfg, err := config.LoadPromptConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load prompt config: %w", err)
	}

	agent := &Agent{
		config:         cfg,
		promptConfig:   promptCfg,
		llm:            llmClient,
		memory:         mem,
		registry:       reg,
		maxContextMsgs: cfg.Memory.MaxContextMessages,
	}

	// Apply options
	for _, opt := range opts {
		opt(agent)
	}

	// Initialize session
	if err := agent.initSession(); err != nil {
		return nil, err
	}

	return agent, nil
}

// initSession initializes the session
func (a *Agent) initSession() error {
	// Try to get the latest session
	session, err := a.memory.GetLatestSession()
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	if session != nil {
		a.sessionID = session.ID
		return nil
	}

	// Create new session
	sessionID, err := a.memory.CreateSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	a.sessionID = sessionID
	return nil
}

// NewSession creates a new session
func (a *Agent) NewSession() error {
	sessionID, err := a.memory.CreateSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	a.sessionID = sessionID
	return nil
}

// ClearSession clears the current session
func (a *Agent) ClearSession() error {
	if err := a.memory.ClearSession(a.sessionID); err != nil {
		return err
	}
	return a.NewSession()
}

// Chat processes user message and returns response
func (a *Agent) Chat(ctx context.Context, userMessage string) (string, error) {
	// Save user message
	if err := a.memory.SaveMessage(a.sessionID, &memory.Message{
		SessionID: a.sessionID,
		Role:      "user",
		Content:   userMessage,
	}); err != nil {
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

		// Save assistant message with tool calls
		toolCallsJSON, _ := json.Marshal(resp.ToolCalls)
		if err := a.memory.SaveMessage(a.sessionID, &memory.Message{
			SessionID: a.sessionID,
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: string(toolCallsJSON),
		}); err != nil {
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

			// Save tool message
			if err := a.memory.SaveMessage(a.sessionID, &memory.Message{
				SessionID:  a.sessionID,
				Role:       "tool",
				Content:    toolResultContent,
				ToolCallID: toolCall.ID,
			}); err != nil {
				return "", fmt.Errorf("failed to save tool message: %w", err)
			}
		}
	}

	// Check if we need to save long-term memory
	a.checkAndSaveMemory(userMessage, finalResponse)

	// Save assistant response (only if it's not empty or different from the last tool call response)
	// Note: If the loop finished naturally, finalResponse is set.
	// If we broke out because of no tool calls, finalResponse is set.
	if finalResponse != "" {
		if err := a.memory.SaveMessage(a.sessionID, &memory.Message{
			SessionID: a.sessionID,
			Role:      "assistant",
			Content:   finalResponse,
		}); err != nil {
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
	memories, err := a.searchRelevantMemories(userMessage)
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

	// Load history messages
	historyMsgs, err := a.memory.GetMessages(a.sessionID, a.maxContextMsgs)
	if err != nil {
		return nil, fmt.Errorf("failed to get history messages: %w", err)
	}

	// Convert history message format (exclude current message as it will be added at the end)
	for _, msg := range historyMsgs {
		// Skip the just-saved user message
		if msg.Role == "user" && msg.Content == userMessage {
			continue
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

// searchRelevantMemories searches for relevant memories
func (a *Agent) searchRelevantMemories(userMessage string) ([]*memory.Memory, error) {
	// Simple keyword extraction (can be optimized later)
	keywords := extractKeywords(userMessage)
	if len(keywords) == 0 {
		return nil, nil
	}

	var allMemories []*memory.Memory
	seen := make(map[int64]bool)

	for _, keyword := range keywords {
		memories, err := a.memory.SearchMemories(keyword, 5)
		if err != nil {
			continue
		}
		for _, mem := range memories {
			if !seen[mem.ID] {
				seen[mem.ID] = true
				allMemories = append(allMemories, mem)
			}
		}
	}

	// Limit return count
	if len(allMemories) > 5 {
		allMemories = allMemories[:5]
	}

	return allMemories, nil
}

// checkAndSaveMemory checks if we need to save long-term memory
func (a *Agent) checkAndSaveMemory(userMessage, response string) {
	lowerMsg := strings.ToLower(userMessage)

	// Detect "remember" intent
	memoryTriggers := []string{"记住", "记下", "remember", "记一下", "帮我记"}
	for _, trigger := range memoryTriggers {
		if strings.Contains(lowerMsg, trigger) {
			// Extract content to remember
			content := extractMemoryContent(userMessage)
			if content != "" {
				keywords := extractKeywords(content)
				_ = a.memory.SaveMemory(content, keywords)
			}
			break
		}
	}
}

// extractKeywords extracts keywords (simple implementation)
func extractKeywords(text string) []string {
	// Remove common stop words, extract meaningful words
	stopWords := map[string]bool{
		"的": true, "是": true, "在": true, "我": true, "你": true,
		"他": true, "她": true, "它": true, "这": true, "那": true,
		"有": true, "和": true, "与": true, "或": true, "但": true,
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "can": true,
	}

	// Simple tokenization
	words := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == '?' || r == '!' ||
			r == '：' || r == '，' || r == '。' || r == '？' || r == '！'
	})

	var keywords []string
	for _, word := range words {
		word = strings.ToLower(strings.TrimSpace(word))
		if len(word) > 1 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	// Limit keyword count
	if len(keywords) > 10 {
		keywords = keywords[:10]
	}

	return keywords
}

// extractMemoryContent extracts content to remember
func extractMemoryContent(userMessage string) string {
	// Try to extract content after "remember"
	triggers := []string{"记住", "记下", "remember", "记一下", "帮我记"}
	lowerMsg := strings.ToLower(userMessage)

	for _, trigger := range triggers {
		if idx := strings.Index(lowerMsg, trigger); idx != -1 {
			// Extract content after trigger word
			content := userMessage[idx+len(trigger):]
			content = strings.TrimSpace(content)
			// Remove common prefixes
			content = strings.TrimPrefix(content, "：")
			content = strings.TrimPrefix(content, ":")
			content = strings.TrimPrefix(content, "，")
			content = strings.TrimPrefix(content, ",")
			content = strings.TrimSpace(content)
			if content != "" {
				return content
			}
		}
	}

	return userMessage
}

// SessionID returns the current session ID
func (a *Agent) SessionID() string {
	return a.sessionID
}
