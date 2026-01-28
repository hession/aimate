package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) (*SQLiteStore, func()) {
	tmpDir, err := os.MkdirTemp("", "aimate-memory-test")
	if err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

func TestCreateAndGetSession(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	// Create session
	sessionID, err := store.CreateSession()
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	if sessionID == "" {
		t.Error("Session ID should not be empty")
	}

	// Get session
	session, err := store.GetSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	if session == nil {
		t.Fatal("Session should not be nil")
	}
	if session.ID != sessionID {
		t.Errorf("Session ID mismatch: expected %s, got %s", sessionID, session.ID)
	}

	// Get non-existent session
	session, err = store.GetSession("not-exist")
	if err != nil {
		t.Fatalf("Getting non-existent session should not return error: %v", err)
	}
	if session != nil {
		t.Error("Non-existent session should return nil")
	}
}

func TestGetLatestSession(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	// When no sessions exist
	session, err := store.GetLatestSession()
	if err != nil {
		t.Fatalf("Failed to get latest session: %v", err)
	}
	if session != nil {
		t.Error("Should return nil when no sessions exist")
	}

	// Create multiple sessions
	session1, _ := store.CreateSession()
	session2, _ := store.CreateSession()

	// Get latest session (should be session2)
	latest, err := store.GetLatestSession()
	if err != nil {
		t.Fatalf("Failed to get latest session: %v", err)
	}
	if latest == nil {
		t.Fatal("Should return latest session")
	}
	if latest.ID != session2 {
		t.Errorf("Latest session should be %s, got %s (session1=%s)", session2, latest.ID, session1)
	}
}

func TestSaveAndGetMessages(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	sessionID, _ := store.CreateSession()

	// Save messages
	msgs := []*Message{
		{SessionID: sessionID, Role: "user", Content: "Hello"},
		{SessionID: sessionID, Role: "assistant", Content: "Hi! How can I help you?"},
		{SessionID: sessionID, Role: "user", Content: "Help me check files"},
	}

	for _, msg := range msgs {
		err := store.SaveMessage(sessionID, msg)
		if err != nil {
			t.Fatalf("Failed to save message: %v", err)
		}
	}

	// Get messages
	retrieved, err := store.GetMessages(sessionID, 10)
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}
	if len(retrieved) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(retrieved))
	}

	// Verify message order (should be chronological)
	if retrieved[0].Content != "Hello" {
		t.Errorf("First message content mismatch: %s", retrieved[0].Content)
	}
	if retrieved[2].Content != "Help me check files" {
		t.Errorf("Last message content mismatch: %s", retrieved[2].Content)
	}

	// Test limit
	limited, err := store.GetMessages(sessionID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(limited))
	}
}

func TestSaveAndGetToolMessages(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	sessionID, _ := store.CreateSession()

	// Save tool call message (assistant)
	toolCalls := `[{"id":"call_123","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"test.txt\"}"}}]`
	assistantMsg := &Message{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   "",
		ToolCalls: toolCalls,
	}
	if err := store.SaveMessage(sessionID, assistantMsg); err != nil {
		t.Fatalf("Failed to save assistant tool call message: %v", err)
	}

	// Save tool result message (tool)
	toolMsg := &Message{
		SessionID:  sessionID,
		Role:       "tool",
		Content:    "file content",
		ToolCallID: "call_123",
	}
	if err := store.SaveMessage(sessionID, toolMsg); err != nil {
		t.Fatalf("Failed to save tool message: %v", err)
	}

	// Get messages
	retrieved, err := store.GetMessages(sessionID, 10)
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}
	if len(retrieved) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(retrieved))
	}

	// Verify assistant message
	if retrieved[0].Role != "assistant" {
		t.Errorf("First message role mismatch: %s", retrieved[0].Role)
	}
	if retrieved[0].ToolCalls != toolCalls {
		t.Errorf("ToolCalls mismatch: expected %s, got %s", toolCalls, retrieved[0].ToolCalls)
	}

	// Verify tool message
	if retrieved[1].Role != "tool" {
		t.Errorf("Second message role mismatch: %s", retrieved[1].Role)
	}
	if retrieved[1].ToolCallID != "call_123" {
		t.Errorf("ToolCallID mismatch: expected call_123, got %s", retrieved[1].ToolCallID)
	}
}

func TestSaveAndSearchMemories(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	// Save memories
	err := store.SaveMemory("My project uses Go language", []string{"project", "Go", "language"})
	if err != nil {
		t.Fatalf("Failed to save memory: %v", err)
	}

	err = store.SaveMemory("I prefer VSCode editor", []string{"VSCode", "editor"})
	if err != nil {
		t.Fatal(err)
	}

	// Search memories
	memories, err := store.SearchMemories("Go", 10)
	if err != nil {
		t.Fatalf("Failed to search memories: %v", err)
	}
	if len(memories) != 1 {
		t.Errorf("Expected 1 memory, got %d", len(memories))
	}
	if memories[0].Content != "My project uses Go language" {
		t.Errorf("Memory content mismatch: %s", memories[0].Content)
	}

	// Search non-existent keyword
	memories, err = store.SearchMemories("Python", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 0 {
		t.Errorf("Expected 0 memories, got %d", len(memories))
	}
}

func TestGetAllMemories(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	// Save multiple memories
	store.SaveMemory("Memory 1", []string{"key1"})
	store.SaveMemory("Memory 2", []string{"key2"})
	store.SaveMemory("Memory 3", []string{"key3"})

	memories, err := store.GetAllMemories(10)
	if err != nil {
		t.Fatalf("Failed to get all memories: %v", err)
	}
	if len(memories) != 3 {
		t.Errorf("Expected 3 memories, got %d", len(memories))
	}
}

func TestClearSession(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	sessionID, _ := store.CreateSession()

	// Save messages
	store.SaveMessage(sessionID, &Message{SessionID: sessionID, Role: "user", Content: "test"})
	store.SaveMessage(sessionID, &Message{SessionID: sessionID, Role: "assistant", Content: "reply"})

	// Clear session
	err := store.ClearSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to clear session: %v", err)
	}

	// Verify messages are cleared
	msgs, _ := store.GetMessages(sessionID, 10)
	if len(msgs) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(msgs))
	}
}

func TestDeleteMemory(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	store.SaveMemory("Test memory", []string{"test"})

	memories, _ := store.GetAllMemories(10)
	if len(memories) != 1 {
		t.Fatal("Should have 1 memory")
	}

	memoryID := memories[0].ID

	err := store.DeleteMemory(memoryID)
	if err != nil {
		t.Fatalf("Failed to delete memory: %v", err)
	}

	memories, _ = store.GetAllMemories(10)
	if len(memories) != 0 {
		t.Errorf("Expected 0 memories after delete, got %d", len(memories))
	}
}
