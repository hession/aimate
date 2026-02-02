// Package v2 提供会话记忆管理功能
package v2

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SessionManager 会话记忆管理器
type SessionManager struct {
	storage   *StorageManager
	fileStore *MarkdownFileStore
	index     IndexStore
	config    *MemoryConfig

	// 当前活跃会话
	currentSession *Session
	messages       []SessionMessage
}

// NewSessionManager 创建会话记忆管理器
func NewSessionManager(
	storage *StorageManager,
	fileStore *MarkdownFileStore,
	index IndexStore,
	config *MemoryConfig,
) *SessionManager {
	return &SessionManager{
		storage:   storage,
		fileStore: fileStore,
		index:     index,
		config:    config,
		messages:  []SessionMessage{},
	}
}

// CreateSession 创建新会话
func (m *SessionManager) CreateSession() (*Session, error) {
	// 如果有当前会话，先归档
	if m.currentSession != nil {
		if err := m.ArchiveCurrentSession(); err != nil {
			// 记录错误但继续
			fmt.Printf("归档当前会话失败: %v\n", err)
		}
	}

	sess := &Session{
		ID:           uuid.New().String(),
		ProjectPath:  m.storage.GetCurrentProject(),
		Status:       StatusActive,
		TokenCount:   0,
		MessageCount: 0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// 生成文件路径
	sess.FilePath = m.storage.GenerateSessionFilePath(sess)

	// 确保目录存在
	if err := EnsureDir(sess.FilePath); err != nil {
		return nil, err
	}

	// 创建会话文件
	if err := m.fileStore.CreateSession(sess); err != nil {
		return nil, err
	}

	m.currentSession = sess
	m.messages = []SessionMessage{}

	return sess, nil
}

// GetCurrentSession 获取当前会话
func (m *SessionManager) GetCurrentSession() *Session {
	return m.currentSession
}

// LoadSession 加载指定会话
func (m *SessionManager) LoadSession(sessionID string) error {
	// 查找会话文件
	sessPath, err := m.findSessionFile(sessionID)
	if err != nil {
		return err
	}

	sess, messages, err := m.fileStore.ReadSession(sessPath)
	if err != nil {
		return err
	}

	m.currentSession = sess
	m.messages = messages

	return nil
}

// LoadLatestSession 加载最新会话
func (m *SessionManager) LoadLatestSession() error {
	// 获取会话目录
	var sessDir string
	if m.storage.GetProjectRoot() != "" {
		sessDir = m.storage.GetProjectSessionsPath()
	} else {
		sessDir = m.storage.GetGlobalSessionsPath()
	}

	// 查找最新会话
	var latestPath string
	var latestTime time.Time

	err := filepath.Walk(sessDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			if info.ModTime().After(latestTime) {
				latestTime = info.ModTime()
				latestPath = path
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	if latestPath == "" {
		// 没有会话，创建新会话
		_, err := m.CreateSession()
		return err
	}

	// 加载会话
	sess, messages, err := m.fileStore.ReadSession(latestPath)
	if err != nil {
		return err
	}

	m.currentSession = sess
	m.messages = messages

	return nil
}

// AddMessage 添加消息到当前会话
func (m *SessionManager) AddMessage(role, content string, tokenCount int) error {
	if m.currentSession == nil {
		if _, err := m.CreateSession(); err != nil {
			return err
		}
	}

	msg := SessionMessage{
		Sequence:   len(m.messages) + 1,
		Role:       role,
		Content:    content,
		Timestamp:  time.Now(),
		TokenCount: tokenCount,
	}

	m.messages = append(m.messages, msg)
	m.currentSession.MessageCount = len(m.messages)
	m.currentSession.TokenCount += tokenCount
	m.currentSession.UpdatedAt = time.Now()

	// 保存到文件
	return m.fileStore.UpdateSession(m.currentSession, m.messages)
}

// AddToolMessage 添加工具调用消息
func (m *SessionManager) AddToolMessage(toolCalls string, toolCallID string, content string, tokenCount int) error {
	if m.currentSession == nil {
		if _, err := m.CreateSession(); err != nil {
			return err
		}
	}

	msg := SessionMessage{
		Sequence:   len(m.messages) + 1,
		Role:       "tool",
		Content:    content,
		ToolCalls:  toolCalls,
		ToolCallID: toolCallID,
		Timestamp:  time.Now(),
		TokenCount: tokenCount,
	}

	m.messages = append(m.messages, msg)
	m.currentSession.MessageCount = len(m.messages)
	m.currentSession.TokenCount += tokenCount
	m.currentSession.UpdatedAt = time.Now()

	return m.fileStore.UpdateSession(m.currentSession, m.messages)
}

// GetMessages 获取当前会话的消息
func (m *SessionManager) GetMessages() []SessionMessage {
	return m.messages
}

// GetRecentMessages 获取最近 N 条消息
func (m *SessionManager) GetRecentMessages(n int) []SessionMessage {
	if len(m.messages) <= n {
		return m.messages
	}
	return m.messages[len(m.messages)-n:]
}

// GetTokenUsage 获取 Token 使用情况
func (m *SessionManager) GetTokenUsage() (current, max int, ratio float64) {
	if m.currentSession == nil {
		return 0, m.config.Session.MaxTokens, 0
	}

	current = m.currentSession.TokenCount
	max = m.config.Session.MaxTokens
	ratio = float64(current) / float64(max)
	return
}

// CheckThreshold 检查是否达到警告阈值
func (m *SessionManager) CheckThreshold() (warnings []string) {
	_, _, ratio := m.GetTokenUsage()

	for _, threshold := range m.config.Session.WarningThresholds {
		if ratio >= threshold {
			warnings = append(warnings, fmt.Sprintf(
				"⚠️ 会话上下文已使用 %.0f%%，建议考虑开启新会话",
				ratio*100,
			))
		}
	}

	return
}

// ArchiveCurrentSession 归档当前会话
func (m *SessionManager) ArchiveCurrentSession() error {
	if m.currentSession == nil {
		return nil
	}

	m.currentSession.Status = StatusArchived
	err := m.fileStore.UpdateSession(m.currentSession, m.messages)

	m.currentSession = nil
	m.messages = nil

	return err
}

// ListSessions 列出所有会话
func (m *SessionManager) ListSessions() ([]*Session, error) {
	var sessions []*Session

	// 获取会话目录
	var sessDir string
	if m.storage.GetProjectRoot() != "" {
		sessDir = m.storage.GetProjectSessionsPath()
	} else {
		sessDir = m.storage.GetGlobalSessionsPath()
	}

	// 遍历会话文件
	err := filepath.Walk(sessDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			sess, _, err := m.fileStore.ReadSession(path)
			if err == nil {
				sessions = append(sessions, sess)
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// 按时间降序排序
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	return sessions, nil
}

// ListRecentSessions 列出最近 N 个会话
func (m *SessionManager) ListRecentSessions(n int) ([]*Session, error) {
	sessions, err := m.ListSessions()
	if err != nil {
		return nil, err
	}

	if len(sessions) <= n {
		return sessions, nil
	}
	return sessions[:n], nil
}

// RestoreSession 恢复指定会话
func (m *SessionManager) RestoreSession(sessionID string) error {
	// 归档当前会话
	if m.currentSession != nil {
		if err := m.ArchiveCurrentSession(); err != nil {
			return err
		}
	}

	// 加载目标会话
	return m.LoadSession(sessionID)
}

// BuildContext 构建会话上下文
func (m *SessionManager) BuildContext() (string, error) {
	if len(m.messages) == 0 {
		return "", nil
	}

	var builder strings.Builder
	builder.WriteString("## 当前会话\n\n")

	for _, msg := range m.messages {
		roleDisplay := formatRoleDisplay(msg.Role)
		builder.WriteString(fmt.Sprintf("### %s\n", roleDisplay))
		builder.WriteString(msg.Content)
		builder.WriteString("\n\n")
	}

	return builder.String(), nil
}

// BuildContextForLLM 构建用于 LLM 的上下文消息列表
func (m *SessionManager) BuildContextForLLM() []map[string]interface{} {
	var result []map[string]interface{}

	for _, msg := range m.messages {
		entry := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}

		if msg.ToolCalls != "" {
			var toolCalls []interface{}
			_ = json.Unmarshal([]byte(msg.ToolCalls), &toolCalls)
			entry["tool_calls"] = toolCalls
		}

		if msg.ToolCallID != "" {
			entry["tool_call_id"] = msg.ToolCallID
		}

		result = append(result, entry)
	}

	return result
}

// ClearMessages 清除当前会话消息（但保留会话）
func (m *SessionManager) ClearMessages() error {
	if m.currentSession == nil {
		return nil
	}

	m.messages = []SessionMessage{}
	m.currentSession.MessageCount = 0
	m.currentSession.TokenCount = 0
	m.currentSession.UpdatedAt = time.Now()

	return m.fileStore.UpdateSession(m.currentSession, m.messages)
}

// findSessionFile 查找会话文件
func (m *SessionManager) findSessionFile(sessionID string) (string, error) {
	var sessDir string
	if m.storage.GetProjectRoot() != "" {
		sessDir = m.storage.GetProjectSessionsPath()
	} else {
		sessDir = m.storage.GetGlobalSessionsPath()
	}

	var found string
	err := filepath.Walk(sessDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			// 检查文件名是否包含 sessionID
			if strings.Contains(filepath.Base(path), sessionID[:8]) {
				found = path
				return filepath.SkipAll
			}
		}
		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return "", err
	}

	if found == "" {
		return "", ErrSessionNotFound
	}

	return found, nil
}

// formatRoleDisplay 格式化角色显示
func formatRoleDisplay(role string) string {
	switch role {
	case "user":
		return "👤 用户"
	case "assistant":
		return "🤖 助手"
	case "system":
		return "⚙️ 系统"
	case "tool":
		return "🔧 工具"
	default:
		return role
	}
}

// GetSessionStats 获取会话统计
func (m *SessionManager) GetSessionStats() *SessionStats {
	stats := &SessionStats{}

	if m.currentSession != nil {
		stats.CurrentSessionID = m.currentSession.ID
		stats.CurrentTokens = m.currentSession.TokenCount
		stats.CurrentMessages = m.currentSession.MessageCount
		stats.MaxTokens = m.config.Session.MaxTokens
		stats.UsageRatio = float64(m.currentSession.TokenCount) / float64(m.config.Session.MaxTokens)
	}

	// 统计所有会话
	sessions, _ := m.ListSessions()
	stats.TotalSessions = len(sessions)

	activeSessions := 0
	for _, sess := range sessions {
		if sess.Status == StatusActive {
			activeSessions++
		}
	}
	stats.ActiveSessions = activeSessions

	return stats
}

// SessionStats 会话统计
type SessionStats struct {
	CurrentSessionID string  `json:"current_session_id"`
	CurrentTokens    int     `json:"current_tokens"`
	CurrentMessages  int     `json:"current_messages"`
	MaxTokens        int     `json:"max_tokens"`
	UsageRatio       float64 `json:"usage_ratio"`
	TotalSessions    int     `json:"total_sessions"`
	ActiveSessions   int     `json:"active_sessions"`
}

// NeedsTrimming 检查是否需要裁剪
func (m *SessionManager) NeedsTrimming() bool {
	if m.currentSession == nil {
		return false
	}

	ratio := float64(m.currentSession.TokenCount) / float64(m.config.Session.MaxTokens)
	// 达到 85% 时需要裁剪
	return ratio >= 0.85
}

// SetSessionTitle 设置会话标题
func (m *SessionManager) SetSessionTitle(title string) error {
	if m.currentSession == nil {
		return ErrSessionNotFound
	}

	m.currentSession.Title = title
	m.currentSession.UpdatedAt = time.Now()

	return m.fileStore.UpdateSession(m.currentSession, m.messages)
}
