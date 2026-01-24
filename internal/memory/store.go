package memory

import (
	"time"
)

// Store memory storage interface
type Store interface {
	// Session management
	CreateSession() (string, error)
	GetSession(id string) (*Session, error)
	GetLatestSession() (*Session, error)
	UpdateSessionTime(id string) error
	ClearSession(sessionID string) error

	// Short-term memory (messages)
	SaveMessage(sessionID string, msg *Message) error
	GetMessages(sessionID string, limit int) ([]*Message, error)

	// Long-term memory
	SaveMemory(content string, keywords []string) error
	SearchMemories(keyword string, limit int) ([]*Memory, error)
	GetAllMemories(limit int) ([]*Memory, error)
	DeleteMemory(id int64) error

	// Close connection
	Close() error
}

// Session session structure
type Session struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message message structure
type Message struct {
	ID        int64
	SessionID string
	Role      string // "user" | "assistant" | "system"
	Content   string
	CreatedAt time.Time
}

// Memory long-term memory structure
type Memory struct {
	ID        int64
	Content   string
	Keywords  string
	CreatedAt time.Time
}
