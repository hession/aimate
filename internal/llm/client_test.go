package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	client := New("test-api-key", "https://api.test.com", "test-model", 0.7, 1000)

	if client.apiKey != "test-api-key" {
		t.Errorf("Expected apiKey 'test-api-key', got '%s'", client.apiKey)
	}
	if client.baseURL != "https://api.test.com" {
		t.Errorf("Expected baseURL 'https://api.test.com', got '%s'", client.baseURL)
	}
	if client.model != "test-model" {
		t.Errorf("Expected model 'test-model', got '%s'", client.model)
	}
	if client.temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", client.temperature)
	}
	if client.maxTokens != 1000 {
		t.Errorf("Expected maxTokens 1000, got %d", client.maxTokens)
	}
}

func TestNew_TrimTrailingSlash(t *testing.T) {
	client := New("key", "https://api.test.com/", "model", 0.7, 1000)

	if client.baseURL != "https://api.test.com" {
		t.Errorf("Expected baseURL without trailing slash, got '%s'", client.baseURL)
	}
}

func TestClient_Chat(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("Expected path /v1/chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Expected Authorization header, got %s", r.Header.Get("Authorization"))
		}

		// Verify request body
		var reqBody chatRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("Failed to decode request body: %v", err)
		}
		if reqBody.Model != "test-model" {
			t.Errorf("Expected model 'test-model', got '%s'", reqBody.Model)
		}
		if len(reqBody.Messages) != 1 {
			t.Errorf("Expected 1 message, got %d", len(reqBody.Messages))
		}

		// Send response
		resp := chatResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "test-model",
			Choices: []struct {
				Index        int     `json:"index"`
				Message      Message `json:"message"`
				Delta        Message `json:"delta"`
				FinishReason string  `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: Message{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := New("test-key", server.URL, "test-model", 0.7, 1000)

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	resp, err := client.Chat(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.Content != "Hello! How can I help you?" {
		t.Errorf("Expected response content, got '%s'", resp.Content)
	}
}

func TestClient_Chat_WithTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody chatRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("Failed to decode request body: %v", err)
		}

		// Verify tools are passed
		if len(reqBody.Tools) != 1 {
			t.Errorf("Expected 1 tool, got %d", len(reqBody.Tools))
		}

		resp := chatResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "test-model",
			Choices: []struct {
				Index        int     `json:"index"`
				Message      Message `json:"message"`
				Delta        Message `json:"delta"`
				FinishReason string  `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: Message{
						Role: "assistant",
						ToolCalls: []ToolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: FunctionCall{
									Name:      "test_function",
									Arguments: `{"arg": "value"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := New("test-key", server.URL, "test-model", 0.7, 1000)

	messages := []Message{
		{Role: "user", Content: "Call a function"},
	}

	tools := []Tool{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "test_function",
				Description: "A test function",
				Parameters:  map[string]interface{}{"type": "object"},
			},
		},
	}

	resp, err := client.Chat(context.Background(), messages, tools)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Function.Name != "test_function" {
		t.Errorf("Expected function name 'test_function', got '%s'", resp.ToolCalls[0].Function.Name)
	}
}

func TestClient_Chat_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "Invalid request", "type": "invalid_request_error"}}`))
	}))
	defer server.Close()

	client := New("test-key", server.URL, "test-model", 0.7, 1000)

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	_, err := client.Chat(context.Background(), messages, nil)
	if err == nil {
		t.Error("Expected error for bad request")
	}
	if !strings.Contains(err.Error(), "status 400") {
		t.Errorf("Expected status 400 in error, got: %v", err)
	}
}

func TestClient_Chat_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			ID:      "test-id",
			Choices: []struct {
				Index        int     `json:"index"`
				Message      Message `json:"message"`
				Delta        Message `json:"delta"`
				FinishReason string  `json:"finish_reason"`
			}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := New("test-key", server.URL, "test-model", 0.7, 1000)

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	_, err := client.Chat(context.Background(), messages, nil)
	if err == nil {
		t.Error("Expected error for empty response")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("Expected 'empty response' in error, got: %v", err)
	}
}

func TestClient_Chat_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Error: &struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			}{
				Message: "Rate limit exceeded",
				Type:    "rate_limit_error",
				Code:    "rate_limit",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := New("test-key", server.URL, "test-model", 0.7, 1000)

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	_, err := client.Chat(context.Background(), messages, nil)
	if err == nil {
		t.Error("Expected error for API error response")
	}
	if !strings.Contains(err.Error(), "Rate limit exceeded") {
		t.Errorf("Expected 'Rate limit exceeded' in error, got: %v", err)
	}
}

func TestClient_ChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody chatRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("Failed to decode request body: %v", err)
		}

		if !reqBody.Stream {
			t.Error("Expected Stream to be true")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		// Send streaming response
		chunks := []string{
			`{"id":"test","choices":[{"delta":{"content":"Hello"}}]}`,
			`{"id":"test","choices":[{"delta":{"content":" World"}}]}`,
			`{"id":"test","choices":[{"delta":{"content":"!"}}]}`,
		}

		for _, chunk := range chunks {
			w.Write([]byte("data: " + chunk + "\n\n"))
			flusher.Flush()
		}
		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer server.Close()

	client := New("test-key", server.URL, "test-model", 0.7, 1000)

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	var receivedContent strings.Builder
	handler := func(content string) {
		receivedContent.WriteString(content)
	}

	resp, err := client.ChatStream(context.Background(), messages, nil, handler)
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	if resp.Content != "Hello World!" {
		t.Errorf("Expected 'Hello World!', got '%s'", resp.Content)
	}
	if receivedContent.String() != "Hello World!" {
		t.Errorf("Handler received '%s', expected 'Hello World!'", receivedContent.String())
	}
}

func TestClient_ChatStream_WithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		// Send streaming response with tool calls
		chunks := []string{
			`{"id":"test","choices":[{"delta":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"test_func","arguments":"{\"a\":"}}]}}]}`,
			`{"id":"test","choices":[{"delta":{"tool_calls":[{"id":"call_1","function":{"arguments":"1}"}}]}}]}`,
		}

		for _, chunk := range chunks {
			w.Write([]byte("data: " + chunk + "\n\n"))
			flusher.Flush()
		}
		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer server.Close()

	client := New("test-key", server.URL, "test-model", 0.7, 1000)

	messages := []Message{
		{Role: "user", Content: "Call a function"},
	}

	resp, err := client.ChatStream(context.Background(), messages, nil, nil)
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Function.Arguments != `{"a":1}` {
		t.Errorf("Expected merged arguments, got '%s'", resp.ToolCalls[0].Function.Arguments)
	}
}

func TestClient_ChatWithRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "Server error"}`))
			return
		}

		resp := chatResponse{
			ID: "test-id",
			Choices: []struct {
				Index        int     `json:"index"`
				Message      Message `json:"message"`
				Delta        Message `json:"delta"`
				FinishReason string  `json:"finish_reason"`
			}{
				{
					Index:   0,
					Message: Message{Role: "assistant", Content: "Success"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := New("test-key", server.URL, "test-model", 0.7, 1000)
	// Override timeout for faster test
	client.httpClient.Timeout = 100 * time.Millisecond

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	resp, err := client.ChatWithRetry(context.Background(), messages, nil, 5)
	if err != nil {
		t.Fatalf("ChatWithRetry failed: %v", err)
	}

	if resp.Content != "Success" {
		t.Errorf("Expected 'Success', got '%s'", resp.Content)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestClient_ChatWithRetry_AllFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Server error"}`))
	}))
	defer server.Close()

	client := New("test-key", server.URL, "test-model", 0.7, 1000)
	client.httpClient.Timeout = 100 * time.Millisecond

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	_, err := client.ChatWithRetry(context.Background(), messages, nil, 2)
	if err == nil {
		t.Error("Expected error after all retries fail")
	}
	if !strings.Contains(err.Error(), "2 retries") {
		t.Errorf("Expected '2 retries' in error, got: %v", err)
	}
}

func TestHandleResponse_InvalidJSON(t *testing.T) {
	client := New("test-key", "https://api.test.com", "test-model", 0.7, 1000)

	body := bytes.NewBufferString("invalid json")
	_, err := client.handleResponse(body)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestMessage_JSON(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	var parsed Message
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if parsed.Role != msg.Role || parsed.Content != msg.Content {
		t.Error("Message serialization mismatch")
	}
}

func TestMessage_WithToolCalls(t *testing.T) {
	msg := Message{
		Role: "assistant",
		ToolCalls: []ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: FunctionCall{
					Name:      "test",
					Arguments: `{"key": "value"}`,
				},
			},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	var parsed Message
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if len(parsed.ToolCalls) != 1 {
		t.Error("Tool calls not preserved")
	}
}

func TestMessage_WithToolCallID(t *testing.T) {
	msg := Message{
		Role:       "tool",
		Content:    "Tool result",
		ToolCallID: "call_1",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	var parsed Message
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if parsed.ToolCallID != "call_1" {
		t.Errorf("Expected tool_call_id 'call_1', got '%s'", parsed.ToolCallID)
	}
}
