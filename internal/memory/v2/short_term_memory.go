// Package v2 提供短期记忆管理功能
package v2

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ShortTermMemoryManager 短期记忆管理器
// 管理任务、笔记、上下文摘要等短期记忆
type ShortTermMemoryManager struct {
	storage   *StorageManager
	fileStore *MarkdownFileStore
	index     IndexStore
	config    *MemoryConfig
}

// NewShortTermMemoryManager 创建短期记忆管理器
func NewShortTermMemoryManager(
	storage *StorageManager,
	fileStore *MarkdownFileStore,
	index IndexStore,
	config *MemoryConfig,
) *ShortTermMemoryManager {
	return &ShortTermMemoryManager{
		storage:   storage,
		fileStore: fileStore,
		index:     index,
		config:    config,
	}
}

// Add 添加短期记忆
func (m *ShortTermMemoryManager) Add(category MemoryCategory, scope MemoryScope, title, content string, ttlDays int) (*Memory, error) {
	mem := &Memory{
		ID:         uuid.New().String(),
		Type:       MemoryTypeShortTerm,
		Scope:      scope,
		Category:   category,
		Title:      title,
		Content:    content,
		Status:     StatusActive,
		Importance: 3,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		AccessedAt: time.Now(),
	}

	// 设置 TTL
	if ttlDays > 0 {
		mem.SetTTL(time.Duration(ttlDays) * 24 * time.Hour)
	} else {
		// 使用默认 TTL
		defaultTTL := m.getCategoryTTL(category)
		mem.SetTTL(time.Duration(defaultTTL) * 24 * time.Hour)
	}

	// 设置项目路径
	if scope == ScopeProject {
		mem.ProjectPath = m.storage.GetCurrentProject()
	}

	// 生成文件路径
	mem.FilePath = m.storage.GenerateMemoryFilePath(mem)

	// 确保目录存在
	if err := EnsureDir(mem.FilePath); err != nil {
		return nil, err
	}

	// 创建文件
	if err := m.fileStore.CreateMemory(mem); err != nil {
		return nil, err
	}

	// 创建索引
	idx := MemoryToIndex(mem)
	if err := m.index.CreateIndex(idx); err != nil {
		_ = m.fileStore.DeleteMemory(mem.FilePath)
		return nil, err
	}

	return mem, nil
}

// AddTask 添加任务记忆
func (m *ShortTermMemoryManager) AddTask(title, content string, ttlDays int) (*Memory, error) {
	scope := ScopeProject
	if m.storage.GetCurrentProject() == "" {
		scope = ScopeGlobal
	}
	return m.Add(CategoryTask, scope, title, content, ttlDays)
}

// AddNote 添加笔记记忆
func (m *ShortTermMemoryManager) AddNote(title, content string, ttlDays int) (*Memory, error) {
	scope := ScopeProject
	if m.storage.GetCurrentProject() == "" {
		scope = ScopeGlobal
	}
	return m.Add(CategoryNote, scope, title, content, ttlDays)
}

// AddContext 添加上下文摘要记忆
func (m *ShortTermMemoryManager) AddContext(title, content string, ttlDays int) (*Memory, error) {
	scope := ScopeProject
	if m.storage.GetCurrentProject() == "" {
		scope = ScopeGlobal
	}
	return m.Add(CategoryContext, scope, title, content, ttlDays)
}

// getCategoryTTL 获取分类的默认 TTL
func (m *ShortTermMemoryManager) getCategoryTTL(category MemoryCategory) int {
	if ttl, ok := m.config.ShortTerm.CategoryTTL[string(category)]; ok {
		return ttl
	}
	return m.config.ShortTerm.DefaultTTLDays
}

// LoadAll 加载所有短期记忆
func (m *ShortTermMemoryManager) LoadAll() ([]*Memory, error) {
	var allMemories []*Memory

	// 加载全局短期记忆
	globalPath := m.storage.GetGlobalShortTermPath()
	globalMems, err := m.fileStore.ListMemories(globalPath)
	if err == nil {
		allMemories = append(allMemories, globalMems...)
	}

	// 加载项目短期记忆
	if m.storage.GetProjectRoot() != "" {
		projectPath := m.storage.GetProjectShortTermPath()
		projectMems, err := m.fileStore.ListMemories(projectPath)
		if err == nil {
			allMemories = append(allMemories, projectMems...)
		}
	}

	return allMemories, nil
}

// LoadByCategory 按分类加载短期记忆
func (m *ShortTermMemoryManager) LoadByCategory(category MemoryCategory) ([]*Memory, error) {
	allMemories, err := m.LoadAll()
	if err != nil {
		return nil, err
	}

	var result []*Memory
	for _, mem := range allMemories {
		if mem.Category == category {
			result = append(result, mem)
		}
	}

	return result, nil
}

// LoadRecent 加载最近 N 天的短期记忆
func (m *ShortTermMemoryManager) LoadRecent(days int) ([]*Memory, error) {
	indexes, err := m.index.GetRecentMemories(days, MemoryTypeShortTerm)
	if err != nil {
		return nil, err
	}

	var memories []*Memory
	for _, idx := range indexes {
		mem, err := m.fileStore.ReadMemory(idx.FilePath)
		if err == nil {
			memories = append(memories, mem)
		}
	}

	return memories, nil
}

// LoadActive 加载所有活跃的短期记忆（未过期）
func (m *ShortTermMemoryManager) LoadActive() ([]*Memory, error) {
	allMemories, err := m.LoadAll()
	if err != nil {
		return nil, err
	}

	var active []*Memory
	for _, mem := range allMemories {
		if !mem.IsExpired() && mem.Status == StatusActive {
			active = append(active, mem)
		}
	}

	// 按更新时间降序排序
	sort.Slice(active, func(i, j int) bool {
		return active[i].UpdatedAt.After(active[j].UpdatedAt)
	})

	return active, nil
}

// GetExpired 获取已过期的记忆
func (m *ShortTermMemoryManager) GetExpired() ([]*Memory, error) {
	indexes, err := m.index.GetExpiredMemories()
	if err != nil {
		return nil, err
	}

	var expired []*Memory
	for _, idx := range indexes {
		if idx.Type == MemoryTypeShortTerm {
			mem, err := m.fileStore.ReadMemory(idx.FilePath)
			if err == nil {
				expired = append(expired, mem)
			}
		}
	}

	return expired, nil
}

// Update 更新短期记忆
func (m *ShortTermMemoryManager) Update(id string, content string) error {
	idx, err := m.index.GetIndex(id)
	if err != nil {
		return err
	}

	mem, err := m.fileStore.ReadMemory(idx.FilePath)
	if err != nil {
		return err
	}

	mem.Content = content
	mem.UpdatedAt = time.Now()

	if err := m.fileStore.UpdateMemory(mem); err != nil {
		return err
	}

	return m.index.UpdateIndex(MemoryToIndex(mem))
}

// Delete 删除短期记忆
func (m *ShortTermMemoryManager) Delete(id string) error {
	idx, err := m.index.GetIndex(id)
	if err != nil {
		return err
	}

	if err := m.fileStore.DeleteMemory(idx.FilePath); err != nil {
		return err
	}

	return m.index.DeleteIndex(id)
}

// Archive 归档短期记忆
func (m *ShortTermMemoryManager) Archive(id string) error {
	idx, err := m.index.GetIndex(id)
	if err != nil {
		return err
	}

	mem, err := m.fileStore.ReadMemory(idx.FilePath)
	if err != nil {
		return err
	}

	return m.fileStore.ArchiveMemory(mem)
}

// CleanExpired 清理过期记忆
func (m *ShortTermMemoryManager) CleanExpired() (int, error) {
	expired, err := m.GetExpired()
	if err != nil {
		return 0, err
	}

	cleaned := 0
	for _, mem := range expired {
		// 归档而不是删除
		if err := m.fileStore.ArchiveMemory(mem); err == nil {
			cleaned++
			// 更新索引状态
			mem.Status = StatusArchived
			_ = m.index.UpdateIndex(MemoryToIndex(mem))
		}
	}

	return cleaned, nil
}

// FindByTitle 按标题查找
func (m *ShortTermMemoryManager) FindByTitle(title string) (*Memory, error) {
	allMemories, err := m.LoadAll()
	if err != nil {
		return nil, err
	}

	for _, mem := range allMemories {
		if mem.Title == title {
			return mem, nil
		}
	}

	return nil, nil
}

// FindByID 按 ID 查找
func (m *ShortTermMemoryManager) FindByID(id string) (*Memory, error) {
	idx, err := m.index.GetIndex(id)
	if err != nil {
		return nil, err
	}

	return m.fileStore.ReadMemory(idx.FilePath)
}

// Search 搜索短期记忆
func (m *ShortTermMemoryManager) Search(keyword string, limit int) ([]*Memory, error) {
	indexes, err := m.index.SearchByKeyword(keyword, limit)
	if err != nil {
		return nil, err
	}

	var memories []*Memory
	for _, idx := range indexes {
		if idx.Type == MemoryTypeShortTerm {
			mem, err := m.fileStore.ReadMemory(idx.FilePath)
			if err == nil {
				memories = append(memories, mem)
			}
		}
	}

	return memories, nil
}

// GetHighAccessMemories 获取高访问频率的记忆（候选提升为长期记忆）
func (m *ShortTermMemoryManager) GetHighAccessMemories() ([]*Memory, error) {
	allMemories, err := m.LoadAll()
	if err != nil {
		return nil, err
	}

	var highAccess []*Memory
	for _, mem := range allMemories {
		if mem.AccessCount >= m.config.ShortTerm.PromoteThreshold {
			highAccess = append(highAccess, mem)
		}
	}

	return highAccess, nil
}

// BuildContext 构建短期记忆上下文
func (m *ShortTermMemoryManager) BuildContext(maxTokens int) (string, error) {
	active, err := m.LoadActive()
	if err != nil {
		return "", err
	}

	if len(active) == 0 {
		return "", nil
	}

	var builder strings.Builder
	builder.WriteString("## 短期记忆\n\n")

	currentTokens := 0
	for _, mem := range active {
		// 估算 Token 数
		entryTokens := len(mem.Title)/4 + len(mem.Content)/4 + 20

		if currentTokens+entryTokens > maxTokens {
			break
		}

		// 格式化记忆条目
		categoryName := m.getCategoryName(mem.Category)
		builder.WriteString(fmt.Sprintf("### [%s] %s\n", categoryName, mem.Title))
		builder.WriteString(fmt.Sprintf("*创建于 %s*\n\n", mem.CreatedAt.Format("2006-01-02")))
		builder.WriteString(truncateContent(mem.Content, 300))
		builder.WriteString("\n\n")

		currentTokens += entryTokens
	}

	return builder.String(), nil
}

// getCategoryName 获取分类显示名称
func (m *ShortTermMemoryManager) getCategoryName(category MemoryCategory) string {
	switch category {
	case CategoryTask:
		return "任务"
	case CategoryNote:
		return "笔记"
	case CategoryContext:
		return "上下文"
	default:
		return string(category)
	}
}

// GetStats 获取短期记忆统计
func (m *ShortTermMemoryManager) GetStats() (*ShortTermStats, error) {
	allMemories, err := m.LoadAll()
	if err != nil {
		return nil, err
	}

	stats := &ShortTermStats{}

	for _, mem := range allMemories {
		stats.Total++

		switch mem.Category {
		case CategoryTask:
			stats.TaskCount++
		case CategoryNote:
			stats.NoteCount++
		case CategoryContext:
			stats.ContextCount++
		}

		if mem.IsExpired() {
			stats.ExpiredCount++
		}

		if mem.AccessCount >= m.config.ShortTerm.PromoteThreshold {
			stats.HighAccessCount++
		}
	}

	return stats, nil
}

// ShortTermStats 短期记忆统计
type ShortTermStats struct {
	Total           int `json:"total"`
	TaskCount       int `json:"task_count"`
	NoteCount       int `json:"note_count"`
	ContextCount    int `json:"context_count"`
	ExpiredCount    int `json:"expired_count"`
	HighAccessCount int `json:"high_access_count"`
}

// ExtendTTL 延长记忆的 TTL
func (m *ShortTermMemoryManager) ExtendTTL(id string, additionalDays int) error {
	idx, err := m.index.GetIndex(id)
	if err != nil {
		return err
	}

	mem, err := m.fileStore.ReadMemory(idx.FilePath)
	if err != nil {
		return err
	}

	// 计算新的过期时间
	var newExpiresAt time.Time
	if mem.ExpiresAt != nil {
		newExpiresAt = mem.ExpiresAt.Add(time.Duration(additionalDays) * 24 * time.Hour)
	} else {
		newExpiresAt = time.Now().Add(time.Duration(additionalDays) * 24 * time.Hour)
	}
	mem.ExpiresAt = &newExpiresAt
	mem.UpdatedAt = time.Now()

	if err := m.fileStore.UpdateMemory(mem); err != nil {
		return err
	}

	return m.index.UpdateIndex(MemoryToIndex(mem))
}

// EnsureDirectories 确保短期记忆目录存在
func (m *ShortTermMemoryManager) EnsureDirectories() error {
	dirs := []string{
		m.storage.GetShortTermTasksPath(ScopeGlobal),
		m.storage.GetShortTermNotesPath(ScopeGlobal),
		m.storage.GetShortTermContextsPath(ScopeGlobal),
	}

	if m.storage.GetProjectRoot() != "" {
		dirs = append(dirs,
			m.storage.GetShortTermTasksPath(ScopeProject),
			m.storage.GetShortTermNotesPath(ScopeProject),
			m.storage.GetShortTermContextsPath(ScopeProject),
		)
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// ListTasks 列出所有任务
func (m *ShortTermMemoryManager) ListTasks() ([]*Memory, error) {
	return m.LoadByCategory(CategoryTask)
}

// ListNotes 列出所有笔记
func (m *ShortTermMemoryManager) ListNotes() ([]*Memory, error) {
	return m.LoadByCategory(CategoryNote)
}

// ListContexts 列出所有上下文摘要
func (m *ShortTermMemoryManager) ListContexts() ([]*Memory, error) {
	return m.LoadByCategory(CategoryContext)
}

// GetTodayMemories 获取今天创建的记忆
func (m *ShortTermMemoryManager) GetTodayMemories() ([]*Memory, error) {
	return m.LoadRecent(1)
}

// GetThisWeekMemories 获取本周创建的记忆
func (m *ShortTermMemoryManager) GetThisWeekMemories() ([]*Memory, error) {
	return m.LoadRecent(7)
}
