package agent

import (
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "short text",
			text:     "Hello",
			expected: 1, // 5 / 3 = 1
		},
		{
			name:     "medium text",
			text:     "Hello World, this is a test message.",
			expected: 12, // 36 / 3 = 12
		},
		{
			name:     "chinese text",
			text:     "你好世界",
			expected: 4, // 12 bytes (3 bytes per character) / 3 = 4
		},
		{
			name:     "mixed text",
			text:     "Hello 世界",
			expected: 4, // 12 / 3 = 4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.text)
			if got != tt.expected {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.text, got, tt.expected)
			}
		})
	}
}

func TestWithStreamHandler(t *testing.T) {
	var handlerCalled bool
	handler := func(content string) {
		handlerCalled = true
	}

	agent := &Agent{}
	opt := WithStreamHandler(handler)
	opt(agent)

	if agent.streamHandler == nil {
		t.Error("streamHandler should be set")
	}

	// Call the handler to verify it works
	agent.streamHandler("test")
	if !handlerCalled {
		t.Error("streamHandler should have been called")
	}
}

func TestWithToolCallHandler(t *testing.T) {
	var handlerCalled bool
	handler := func(name string, args map[string]any, result string, err error) {
		handlerCalled = true
	}

	agent := &Agent{}
	opt := WithToolCallHandler(handler)
	opt(agent)

	if agent.toolCallHandler == nil {
		t.Error("toolCallHandler should be set")
	}

	// Call the handler to verify it works
	agent.toolCallHandler("test", nil, "", nil)
	if !handlerCalled {
		t.Error("toolCallHandler should have been called")
	}
}

func TestMaxToolIterations(t *testing.T) {
	if MaxToolIterations != 10 {
		t.Errorf("MaxToolIterations should be 10, got %d", MaxToolIterations)
	}
}
