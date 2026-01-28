package memory

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore SQLite memory storage implementation
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite storage
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &SQLiteStore{db: db}

	// Initialize tables
	if err := store.initTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database tables: %w", err)
	}

	return store, nil
}

// initTables initializes database tables
func (s *SQLiteStore) initTables() error {
	queries := []string{
		// Sessions table
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Messages table (short-term memory)
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			tool_calls TEXT,
			tool_call_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		)`,
		// Long-term memory table
		`CREATE TABLE IF NOT EXISTS memories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT NOT NULL,
			keywords TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Create indexes
		`CREATE INDEX IF NOT EXISTS idx_messages_session_id ON messages(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_keywords ON memories(keywords)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute SQL: %s, error: %w", query, err)
		}
	}

	// Try to add columns if they don't exist (simple migration)
	migrationQueries := []string{
		`ALTER TABLE messages ADD COLUMN tool_calls TEXT`,
		`ALTER TABLE messages ADD COLUMN tool_call_id TEXT`,
	}
	for _, query := range migrationQueries {
		// Ignore errors (e.g. duplicate column name)
		_, _ = s.db.Exec(query)
	}

	return nil
}

// CreateSession creates a new session
func (s *SQLiteStore) CreateSession() (string, error) {
	id := uuid.New().String()
	now := time.Now()

	_, err := s.db.Exec(
		"INSERT INTO sessions (id, created_at, updated_at) VALUES (?, ?, ?)",
		id, now, now,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return id, nil
}

// GetSession gets a session by ID
func (s *SQLiteStore) GetSession(id string) (*Session, error) {
	var session Session
	err := s.db.QueryRow(
		"SELECT id, created_at, updated_at FROM sessions WHERE id = ?",
		id,
	).Scan(&session.ID, &session.CreatedAt, &session.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return &session, nil
}

// GetLatestSession gets the latest session
func (s *SQLiteStore) GetLatestSession() (*Session, error) {
	var session Session
	err := s.db.QueryRow(
		"SELECT id, created_at, updated_at FROM sessions ORDER BY updated_at DESC LIMIT 1",
	).Scan(&session.ID, &session.CreatedAt, &session.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest session: %w", err)
	}

	return &session, nil
}

// UpdateSessionTime updates the session timestamp
func (s *SQLiteStore) UpdateSessionTime(id string) error {
	_, err := s.db.Exec(
		"UPDATE sessions SET updated_at = ? WHERE id = ?",
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("failed to update session time: %w", err)
	}
	return nil
}

// SaveMessage saves a message
func (s *SQLiteStore) SaveMessage(sessionID string, msg *Message) error {
	result, err := s.db.Exec(
		"INSERT INTO messages (session_id, role, content, tool_calls, tool_call_id, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		sessionID, msg.Role, msg.Content, msg.ToolCalls, msg.ToolCallID, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}

	// Get inserted ID
	id, err := result.LastInsertId()
	if err == nil {
		msg.ID = id
	}

	// Update session time
	_ = s.UpdateSessionTime(sessionID)

	return nil
}

// GetMessages gets messages for a session
func (s *SQLiteStore) GetMessages(sessionID string, limit int) ([]*Message, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, role, content, tool_calls, tool_call_id, created_at 
		 FROM messages 
		 WHERE session_id = ? 
		 ORDER BY created_at DESC 
		 LIMIT ?`,
		sessionID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		var msg Message
		var toolCalls, toolCallID sql.NullString
		if err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &toolCalls, &toolCallID, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		if toolCalls.Valid {
			msg.ToolCalls = toolCalls.String
		}
		if toolCallID.Valid {
			msg.ToolCallID = toolCallID.String
		}
		messages = append(messages, &msg)
	}

	// Reverse order so messages are in chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// SaveMemory saves long-term memory
func (s *SQLiteStore) SaveMemory(content string, keywords []string) error {
	keywordsStr := strings.Join(keywords, ",")

	_, err := s.db.Exec(
		"INSERT INTO memories (content, keywords, created_at) VALUES (?, ?, ?)",
		content, keywordsStr, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to save memory: %w", err)
	}

	return nil
}

// SearchMemories searches memories by keyword
func (s *SQLiteStore) SearchMemories(keyword string, limit int) ([]*Memory, error) {
	rows, err := s.db.Query(
		`SELECT id, content, keywords, created_at 
		 FROM memories 
		 WHERE content LIKE ? OR keywords LIKE ?
		 ORDER BY created_at DESC 
		 LIMIT ?`,
		"%"+keyword+"%", "%"+keyword+"%", limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search memories: %w", err)
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		var mem Memory
		if err := rows.Scan(&mem.ID, &mem.Content, &mem.Keywords, &mem.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan memory: %w", err)
		}
		memories = append(memories, &mem)
	}

	return memories, nil
}

// GetAllMemories gets all memories
func (s *SQLiteStore) GetAllMemories(limit int) ([]*Memory, error) {
	rows, err := s.db.Query(
		`SELECT id, content, keywords, created_at 
		 FROM memories 
		 ORDER BY created_at DESC 
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get memories: %w", err)
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		var mem Memory
		if err := rows.Scan(&mem.ID, &mem.Content, &mem.Keywords, &mem.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan memory: %w", err)
		}
		memories = append(memories, &mem)
	}

	return memories, nil
}

// DeleteMemory deletes a memory by ID
func (s *SQLiteStore) DeleteMemory(id int64) error {
	_, err := s.db.Exec("DELETE FROM memories WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete memory: %w", err)
	}
	return nil
}

// ClearSession clears all messages in a session
func (s *SQLiteStore) ClearSession(sessionID string) error {
	_, err := s.db.Exec("DELETE FROM messages WHERE session_id = ?", sessionID)
	if err != nil {
		return fmt.Errorf("failed to clear session messages: %w", err)
	}
	return nil
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
