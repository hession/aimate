package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client LLM client
type Client struct {
	apiKey      string
	baseURL     string
	model       string
	temperature float64
	maxTokens   int
	httpClient  *http.Client
}

// Message message structure
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall tool call structure
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall function call details
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatResponse chat response
type ChatResponse struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// StreamHandler stream response handler
type StreamHandler func(content string)

// Tool tool definition (for Function Calling)
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction tool function definition
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// chatRequest chat request
type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
	Stream      bool      `json:"stream"`
}

// chatResponse API response
type chatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		Delta        Message `json:"delta"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// New creates a new LLM client
func New(apiKey, baseURL, model string, temperature float64, maxTokens int) *Client {
	return &Client{
		apiKey:      apiKey,
		baseURL:     strings.TrimSuffix(baseURL, "/"),
		model:       model,
		temperature: temperature,
		maxTokens:   maxTokens,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Chat sends a chat request
func (c *Client) Chat(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error) {
	return c.chat(ctx, messages, tools, false, nil)
}

// ChatStream sends a streaming chat request
func (c *Client) ChatStream(ctx context.Context, messages []Message, tools []Tool, handler StreamHandler) (*ChatResponse, error) {
	return c.chat(ctx, messages, tools, true, handler)
}

// chat internal chat implementation
func (c *Client) chat(ctx context.Context, messages []Message, tools []Tool, stream bool, handler StreamHandler) (*ChatResponse, error) {
	reqBody := chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: c.temperature,
		MaxTokens:   c.maxTokens,
		Stream:      stream,
	}

	if len(tools) > 0 {
		reqBody.Tools = tools
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned error (status %d): %s", resp.StatusCode, string(body))
	}

	if stream {
		return c.handleStreamResponse(resp.Body, handler)
	}

	return c.handleResponse(resp.Body)
}

// handleResponse handles normal response
func (c *Client) handleResponse(body io.Reader) (*ChatResponse, error) {
	var resp chatResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("API error: %s", resp.Error.Message)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("API returned empty response")
	}

	choice := resp.Choices[0]
	return &ChatResponse{
		Content:   choice.Message.Content,
		ToolCalls: choice.Message.ToolCalls,
	}, nil
}

// handleStreamResponse handles streaming response
func (c *Client) handleStreamResponse(body io.Reader, handler StreamHandler) (*ChatResponse, error) {
	reader := bufio.NewReader(body)
	var fullContent strings.Builder
	var toolCalls []ToolCall
	toolCallsMap := make(map[int]*ToolCall) // For merging streaming tool_calls

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to read streaming response: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var resp chatResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			continue // Ignore parse errors
		}

		if len(resp.Choices) == 0 {
			continue
		}

		delta := resp.Choices[0].Delta

		// Handle content
		if delta.Content != "" {
			fullContent.WriteString(delta.Content)
			if handler != nil {
				handler(delta.Content)
			}
		}

		// Handle tool_calls
		for _, tc := range delta.ToolCalls {
			idx := resp.Choices[0].Index
			if existing, ok := toolCallsMap[idx]; ok {
				// Append arguments
				existing.Function.Arguments += tc.Function.Arguments
			} else {
				// New tool_call
				newTC := ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
				toolCallsMap[idx] = &newTC
			}
		}
	}

	// Collect all tool_calls
	for _, tc := range toolCallsMap {
		toolCalls = append(toolCalls, *tc)
	}

	return &ChatResponse{
		Content:   fullContent.String(),
		ToolCalls: toolCalls,
	}, nil
}

// ChatWithRetry chat request with retry
func (c *Client) ChatWithRetry(ctx context.Context, messages []Message, tools []Tool, maxRetries int) (*ChatResponse, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		resp, err := c.Chat(ctx, messages, tools)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		// Wait before retry
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// ChatStreamWithRetry streaming chat request with retry
func (c *Client) ChatStreamWithRetry(ctx context.Context, messages []Message, tools []Tool, handler StreamHandler, maxRetries int) (*ChatResponse, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		resp, err := c.ChatStream(ctx, messages, tools, handler)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		// Wait before retry
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}
