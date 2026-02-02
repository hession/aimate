// Package v2 提供 AIMate 记忆系统 v2 版本实现
// 实现四层记忆架构：核心记忆、会话记忆、短期记忆、长期记忆
package v2

import (
	"time"
)

// MemoryType 记忆类型
type MemoryType string

const (
	// MemoryTypeCore 核心记忆 - 用户偏好、全局规则
	MemoryTypeCore MemoryType = "core"
	// MemoryTypeSession 会话记忆 - 当前对话上下文
	MemoryTypeSession MemoryType = "session"
	// MemoryTypeShortTerm 短期记忆 - 近期任务、笔记、上下文摘要
	MemoryTypeShortTerm MemoryType = "short_term"
	// MemoryTypeLongTerm 长期记忆 - 项目知识、决策记录
	MemoryTypeLongTerm MemoryType = "long_term"
)

// MemoryScope 记忆作用域
type MemoryScope string

const (
	// ScopeGlobal 全局作用域 - 跨项目共享
	ScopeGlobal MemoryScope = "global"
	// ScopeProject 项目作用域 - 仅限当前项目
	ScopeProject MemoryScope = "project"
)

// MemoryCategory 记忆分类（子类型）
type MemoryCategory string

const (
	// 核心记忆分类
	CategoryPreference MemoryCategory = "preference" // 用户偏好
	CategoryRule       MemoryCategory = "rule"       // 全局规则
	CategoryPersona    MemoryCategory = "persona"    // 角色设定

	// 短期记忆分类
	CategoryTask    MemoryCategory = "task"    // 任务相关
	CategoryNote    MemoryCategory = "note"    // 临时笔记
	CategoryContext MemoryCategory = "context" // 上下文摘要

	// 长期记忆分类
	CategoryProject   MemoryCategory = "project"   // 项目知识
	CategoryKnowledge MemoryCategory = "knowledge" // 通用知识
	CategoryDecision  MemoryCategory = "decision"  // 决策记录
)

// MemoryStatus 记忆状态
type MemoryStatus string

const (
	StatusActive   MemoryStatus = "active"   // 活跃状态
	StatusArchived MemoryStatus = "archived" // 已归档
	StatusExpired  MemoryStatus = "expired"  // 已过期
)

// Memory 记忆基础结构
// 所有类型的记忆都继承此基础结构
type Memory struct {
	// 唯一标识符（UUID）
	ID string `yaml:"id" json:"id"`

	// 记忆类型：core/session/short_term/long_term
	Type MemoryType `yaml:"type" json:"type"`

	// 作用域：global/project
	Scope MemoryScope `yaml:"scope" json:"scope"`

	// 分类（子类型）
	Category MemoryCategory `yaml:"category" json:"category"`

	// 标题（用于文件名和显示）
	Title string `yaml:"title" json:"title"`

	// 记忆内容（Markdown 格式）
	Content string `yaml:"-" json:"content"`

	// 标签列表
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`

	// 关联的其他记忆 ID
	Related []string `yaml:"related,omitempty" json:"related,omitempty"`

	// 来源信息（如：会话 ID、文件路径等）
	Source string `yaml:"source,omitempty" json:"source,omitempty"`

	// 所属项目路径（仅 project scope）
	ProjectPath string `yaml:"project_path,omitempty" json:"project_path,omitempty"`

	// 状态
	Status MemoryStatus `yaml:"status" json:"status"`

	// 重要程度（1-5，5 最重要）
	Importance int `yaml:"importance" json:"importance"`

	// 访问计数
	AccessCount int `yaml:"access_count" json:"access_count"`

	// 内容哈希（用于变更检测）
	ContentHash string `yaml:"content_hash" json:"content_hash"`

	// TTL 过期时间（仅短期记忆）
	ExpiresAt *time.Time `yaml:"expires_at,omitempty" json:"expires_at,omitempty"`

	// 时间戳
	CreatedAt  time.Time `yaml:"created_at" json:"created_at"`
	UpdatedAt  time.Time `yaml:"updated_at" json:"updated_at"`
	AccessedAt time.Time `yaml:"accessed_at" json:"accessed_at"`

	// 文件路径（Markdown 文件的实际存储路径）
	FilePath string `yaml:"-" json:"file_path"`
}

// Session 会话结构
type Session struct {
	// 会话 ID（UUID）
	ID string `yaml:"id" json:"id"`

	// 会话标题（可选，用于显示）
	Title string `yaml:"title,omitempty" json:"title,omitempty"`

	// 所属项目路径
	ProjectPath string `yaml:"project_path" json:"project_path"`

	// 状态：active/archived
	Status MemoryStatus `yaml:"status" json:"status"`

	// Token 使用统计
	TokenCount int `yaml:"token_count" json:"token_count"`

	// 消息计数
	MessageCount int `yaml:"message_count" json:"message_count"`

	// 时间戳
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at" json:"updated_at"`

	// 文件路径
	FilePath string `yaml:"-" json:"file_path"`
}

// SessionMessage 会话消息
type SessionMessage struct {
	// 消息序号
	Sequence int `json:"sequence"`

	// 角色：user/assistant/system/tool
	Role string `json:"role"`

	// 消息内容
	Content string `json:"content"`

	// 工具调用信息（JSON 字符串）
	ToolCalls string `json:"tool_calls,omitempty"`

	// 工具调用 ID（tool 角色时使用）
	ToolCallID string `json:"tool_call_id,omitempty"`

	// 时间戳
	Timestamp time.Time `json:"timestamp"`

	// Token 计数
	TokenCount int `json:"token_count"`
}

// MemoryFrontmatter Markdown 文件的 frontmatter 元数据
// 用于 YAML frontmatter 的序列化/反序列化
type MemoryFrontmatter struct {
	ID          string         `yaml:"id"`
	Type        MemoryType     `yaml:"type"`
	Scope       MemoryScope    `yaml:"scope"`
	Category    MemoryCategory `yaml:"category"`
	Title       string         `yaml:"title"`
	Tags        []string       `yaml:"tags,omitempty"`
	Related     []string       `yaml:"related,omitempty"`
	Source      string         `yaml:"source,omitempty"`
	ProjectPath string         `yaml:"project_path,omitempty"`
	Status      MemoryStatus   `yaml:"status"`
	Importance  int            `yaml:"importance"`
	AccessCount int            `yaml:"access_count"`
	ContentHash string         `yaml:"content_hash"`
	ExpiresAt   *time.Time     `yaml:"expires_at,omitempty"`
	CreatedAt   time.Time      `yaml:"created_at"`
	UpdatedAt   time.Time      `yaml:"updated_at"`
	AccessedAt  time.Time      `yaml:"accessed_at"`
}

// SessionFrontmatter 会话文件的 frontmatter 元数据
type SessionFrontmatter struct {
	ID           string       `yaml:"id"`
	Title        string       `yaml:"title,omitempty"`
	ProjectPath  string       `yaml:"project_path"`
	Status       MemoryStatus `yaml:"status"`
	TokenCount   int          `yaml:"token_count"`
	MessageCount int          `yaml:"message_count"`
	CreatedAt    time.Time    `yaml:"created_at"`
	UpdatedAt    time.Time    `yaml:"updated_at"`
}

// MemorySearchResult 记忆检索结果
type MemorySearchResult struct {
	Memory     *Memory `json:"memory"`
	Score      float64 `json:"score"`      // 相关性分数
	MatchType  string  `json:"match_type"` // 匹配类型：vector/keyword/time
	Highlights string  `json:"highlights"` // 高亮片段
}

// MemoryStats 记忆统计信息
type MemoryStats struct {
	// 各类型记忆数量
	CoreCount      int `json:"core_count"`
	SessionCount   int `json:"session_count"`
	ShortTermCount int `json:"short_term_count"`
	LongTermCount  int `json:"long_term_count"`

	// 各状态数量
	ActiveCount   int `json:"active_count"`
	ArchivedCount int `json:"archived_count"`
	ExpiredCount  int `json:"expired_count"`

	// 存储统计
	TotalFiles     int   `json:"total_files"`
	TotalSizeBytes int64 `json:"total_size_bytes"`

	// 索引统计
	IndexedCount  int `json:"indexed_count"`
	VectorCount   int `json:"vector_count"`
	OrphanedFiles int `json:"orphaned_files"`
	OrphanedIndex int `json:"orphaned_index"`

	// 时间统计
	OldestMemory *time.Time `json:"oldest_memory,omitempty"`
	NewestMemory *time.Time `json:"newest_memory,omitempty"`
}

// ContextBudget 上下文 Token 预算分配
type ContextBudget struct {
	Total     int `json:"total"`      // 总 Token 预算
	Core      int `json:"core"`       // 核心记忆预算
	Session   int `json:"session"`    // 会话记忆预算
	ShortTerm int `json:"short_term"` // 短期记忆预算
	LongTerm  int `json:"long_term"`  // 长期记忆预算
	Reserved  int `json:"reserved"`   // 保留预算（用于响应）
}

// NewMemory 创建新的记忆实例
func NewMemory(memType MemoryType, scope MemoryScope, category MemoryCategory, title, content string) *Memory {
	now := time.Now()
	return &Memory{
		Type:        memType,
		Scope:       scope,
		Category:    category,
		Title:       title,
		Content:     content,
		Status:      StatusActive,
		Importance:  3, // 默认中等重要性
		AccessCount: 0,
		CreatedAt:   now,
		UpdatedAt:   now,
		AccessedAt:  now,
	}
}

// NewSession 创建新的会话实例
func NewSession(projectPath string) *Session {
	now := time.Now()
	return &Session{
		ProjectPath:  projectPath,
		Status:       StatusActive,
		TokenCount:   0,
		MessageCount: 0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// ToFrontmatter 将 Memory 转换为 Frontmatter
func (m *Memory) ToFrontmatter() *MemoryFrontmatter {
	return &MemoryFrontmatter{
		ID:          m.ID,
		Type:        m.Type,
		Scope:       m.Scope,
		Category:    m.Category,
		Title:       m.Title,
		Tags:        m.Tags,
		Related:     m.Related,
		Source:      m.Source,
		ProjectPath: m.ProjectPath,
		Status:      m.Status,
		Importance:  m.Importance,
		AccessCount: m.AccessCount,
		ContentHash: m.ContentHash,
		ExpiresAt:   m.ExpiresAt,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
		AccessedAt:  m.AccessedAt,
	}
}

// FromFrontmatter 从 Frontmatter 恢复 Memory
func (m *Memory) FromFrontmatter(fm *MemoryFrontmatter) {
	m.ID = fm.ID
	m.Type = fm.Type
	m.Scope = fm.Scope
	m.Category = fm.Category
	m.Title = fm.Title
	m.Tags = fm.Tags
	m.Related = fm.Related
	m.Source = fm.Source
	m.ProjectPath = fm.ProjectPath
	m.Status = fm.Status
	m.Importance = fm.Importance
	m.AccessCount = fm.AccessCount
	m.ContentHash = fm.ContentHash
	m.ExpiresAt = fm.ExpiresAt
	m.CreatedAt = fm.CreatedAt
	m.UpdatedAt = fm.UpdatedAt
	m.AccessedAt = fm.AccessedAt
}

// ToFrontmatter 将 Session 转换为 Frontmatter
func (s *Session) ToFrontmatter() *SessionFrontmatter {
	return &SessionFrontmatter{
		ID:           s.ID,
		Title:        s.Title,
		ProjectPath:  s.ProjectPath,
		Status:       s.Status,
		TokenCount:   s.TokenCount,
		MessageCount: s.MessageCount,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

// FromFrontmatter 从 Frontmatter 恢复 Session
func (s *Session) FromFrontmatter(fm *SessionFrontmatter) {
	s.ID = fm.ID
	s.Title = fm.Title
	s.ProjectPath = fm.ProjectPath
	s.Status = fm.Status
	s.TokenCount = fm.TokenCount
	s.MessageCount = fm.MessageCount
	s.CreatedAt = fm.CreatedAt
	s.UpdatedAt = fm.UpdatedAt
}

// IsExpired 检查记忆是否已过期
func (m *Memory) IsExpired() bool {
	if m.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*m.ExpiresAt)
}

// SetTTL 设置 TTL 过期时间
func (m *Memory) SetTTL(duration time.Duration) {
	expiresAt := time.Now().Add(duration)
	m.ExpiresAt = &expiresAt
}

// IncrementAccess 增加访问计数
func (m *Memory) IncrementAccess() {
	m.AccessCount++
	m.AccessedAt = time.Now()
}
