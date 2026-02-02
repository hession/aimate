// Package v2 提供长期记忆管理功能
package v2

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LongTermMemoryManager 长期记忆管理器
// 管理项目知识、通用知识、决策记录等长期记忆
type LongTermMemoryManager struct {
	storage   *StorageManager
	fileStore *MarkdownFileStore
	index     IndexStore
	vector    VectorStore
	config    *MemoryConfig
}

// NewLongTermMemoryManager 创建长期记忆管理器
func NewLongTermMemoryManager(
	storage *StorageManager,
	fileStore *MarkdownFileStore,
	index IndexStore,
	vector VectorStore,
	config *MemoryConfig,
) *LongTermMemoryManager {
	return &LongTermMemoryManager{
		storage:   storage,
		fileStore: fileStore,
		index:     index,
		vector:    vector,
		config:    config,
	}
}

// Add 添加长期记忆
func (m *LongTermMemoryManager) Add(category MemoryCategory, scope MemoryScope, title, content string, tags []string) (*Memory, error) {
	mem := &Memory{
		ID:         uuid.New().String(),
		Type:       MemoryTypeLongTerm,
		Scope:      scope,
		Category:   category,
		Title:      title,
		Content:    content,
		Tags:       tags,
		Status:     StatusActive,
		Importance: 3,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		AccessedAt: time.Now(),
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

// AddProjectKnowledge 添加项目知识
func (m *LongTermMemoryManager) AddProjectKnowledge(title, content string, tags []string) (*Memory, error) {
	scope := ScopeProject
	if m.storage.GetCurrentProject() == "" {
		scope = ScopeGlobal
	}
	return m.Add(CategoryProject, scope, title, content, tags)
}

// AddKnowledge 添加通用知识
func (m *LongTermMemoryManager) AddKnowledge(title, content string, tags []string) (*Memory, error) {
	return m.Add(CategoryKnowledge, ScopeGlobal, title, content, tags)
}

// AddDecision 添加决策记录
func (m *LongTermMemoryManager) AddDecision(title, content string, tags []string) (*Memory, error) {
	scope := ScopeProject
	if m.storage.GetCurrentProject() == "" {
		scope = ScopeGlobal
	}
	return m.Add(CategoryDecision, scope, title, content, tags)
}

// LoadAll 加载所有长期记忆
func (m *LongTermMemoryManager) LoadAll() ([]*Memory, error) {
	var allMemories []*Memory

	// 加载全局长期记忆
	globalPath := m.storage.GetGlobalLongTermPath()
	globalMems, err := m.fileStore.ListMemories(globalPath)
	if err == nil {
		allMemories = append(allMemories, globalMems...)
	}

	// 加载项目长期记忆
	if m.storage.GetProjectRoot() != "" {
		projectPath := m.storage.GetProjectLongTermPath()
		projectMems, err := m.fileStore.ListMemories(projectPath)
		if err == nil {
			allMemories = append(allMemories, projectMems...)
		}
	}

	return allMemories, nil
}

// LoadByCategory 按分类加载长期记忆
func (m *LongTermMemoryManager) LoadByCategory(category MemoryCategory) ([]*Memory, error) {
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

// LoadByScope 按作用域加载长期记忆
func (m *LongTermMemoryManager) LoadByScope(scope MemoryScope) ([]*Memory, error) {
	allMemories, err := m.LoadAll()
	if err != nil {
		return nil, err
	}

	var result []*Memory
	for _, mem := range allMemories {
		if mem.Scope == scope {
			result = append(result, mem)
		}
	}

	return result, nil
}

// LoadActive 加载所有活跃的长期记忆
func (m *LongTermMemoryManager) LoadActive() ([]*Memory, error) {
	allMemories, err := m.LoadAll()
	if err != nil {
		return nil, err
	}

	var active []*Memory
	for _, mem := range allMemories {
		if mem.Status == StatusActive {
			active = append(active, mem)
		}
	}

	// 按重要性和更新时间排序
	sort.Slice(active, func(i, j int) bool {
		if active[i].Importance != active[j].Importance {
			return active[i].Importance > active[j].Importance
		}
		return active[i].UpdatedAt.After(active[j].UpdatedAt)
	})

	return active, nil
}

// Update 更新长期记忆
func (m *LongTermMemoryManager) Update(id string, content string) error {
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

// UpdateWithTags 更新长期记忆（包含标签）
func (m *LongTermMemoryManager) UpdateWithTags(id string, content string, tags []string) error {
	idx, err := m.index.GetIndex(id)
	if err != nil {
		return err
	}

	mem, err := m.fileStore.ReadMemory(idx.FilePath)
	if err != nil {
		return err
	}

	mem.Content = content
	mem.Tags = tags
	mem.UpdatedAt = time.Now()

	if err := m.fileStore.UpdateMemory(mem); err != nil {
		return err
	}

	return m.index.UpdateIndex(MemoryToIndex(mem))
}

// Delete 删除长期记忆
func (m *LongTermMemoryManager) Delete(id string) error {
	idx, err := m.index.GetIndex(id)
	if err != nil {
		return err
	}

	if err := m.fileStore.DeleteMemory(idx.FilePath); err != nil {
		return err
	}

	// 删除向量
	if m.vector != nil {
		_ = m.vector.DeleteVector(id)
	}

	return m.index.DeleteIndex(id)
}

// Archive 归档长期记忆
func (m *LongTermMemoryManager) Archive(id string) error {
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

// FindByID 按 ID 查找
func (m *LongTermMemoryManager) FindByID(id string) (*Memory, error) {
	idx, err := m.index.GetIndex(id)
	if err != nil {
		return nil, err
	}

	return m.fileStore.ReadMemory(idx.FilePath)
}

// FindByTitle 按标题查找
func (m *LongTermMemoryManager) FindByTitle(title string) (*Memory, error) {
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

// SearchByKeyword 关键词搜索
func (m *LongTermMemoryManager) SearchByKeyword(keyword string, limit int) ([]*Memory, error) {
	indexes, err := m.index.SearchByKeyword(keyword, limit)
	if err != nil {
		return nil, err
	}

	var memories []*Memory
	for _, idx := range indexes {
		if idx.Type == MemoryTypeLongTerm {
			mem, err := m.fileStore.ReadMemory(idx.FilePath)
			if err == nil {
				memories = append(memories, mem)
			}
		}
	}

	return memories, nil
}

// SearchByTags 按标签搜索
func (m *LongTermMemoryManager) SearchByTags(tags []string, limit int) ([]*Memory, error) {
	allMemories, err := m.LoadAll()
	if err != nil {
		return nil, err
	}

	var result []*Memory
	for _, mem := range allMemories {
		if hasAnyTag(mem.Tags, tags) {
			result = append(result, mem)
			if len(result) >= limit {
				break
			}
		}
	}

	return result, nil
}

// hasAnyTag 检查是否包含任意标签
func hasAnyTag(memTags []string, searchTags []string) bool {
	for _, st := range searchTags {
		for _, mt := range memTags {
			if strings.EqualFold(mt, st) {
				return true
			}
		}
	}
	return false
}

// GetInactive 获取长期未访问的记忆
func (m *LongTermMemoryManager) GetInactive() ([]*Memory, error) {
	allMemories, err := m.LoadAll()
	if err != nil {
		return nil, err
	}

	threshold := time.Now().AddDate(0, 0, -m.config.LongTerm.InactiveArchiveDays)

	var inactive []*Memory
	for _, mem := range allMemories {
		if mem.AccessedAt.Before(threshold) {
			inactive = append(inactive, mem)
		}
	}

	return inactive, nil
}

// ArchiveInactive 归档长期未访问的记忆
func (m *LongTermMemoryManager) ArchiveInactive() (int, error) {
	inactive, err := m.GetInactive()
	if err != nil {
		return 0, err
	}

	archived := 0
	for _, mem := range inactive {
		if err := m.fileStore.ArchiveMemory(mem); err == nil {
			archived++
			mem.Status = StatusArchived
			_ = m.index.UpdateIndex(MemoryToIndex(mem))
		}
	}

	return archived, nil
}

// AddRelation 添加记忆关联
func (m *LongTermMemoryManager) AddRelation(id, relatedID string) error {
	idx, err := m.index.GetIndex(id)
	if err != nil {
		return err
	}

	mem, err := m.fileStore.ReadMemory(idx.FilePath)
	if err != nil {
		return err
	}

	// 检查是否已存在关联
	for _, r := range mem.Related {
		if r == relatedID {
			return nil // 已存在
		}
	}

	mem.Related = append(mem.Related, relatedID)
	mem.UpdatedAt = time.Now()

	if err := m.fileStore.UpdateMemory(mem); err != nil {
		return err
	}

	return m.index.UpdateIndex(MemoryToIndex(mem))
}

// RemoveRelation 移除记忆关联
func (m *LongTermMemoryManager) RemoveRelation(id, relatedID string) error {
	idx, err := m.index.GetIndex(id)
	if err != nil {
		return err
	}

	mem, err := m.fileStore.ReadMemory(idx.FilePath)
	if err != nil {
		return err
	}

	// 移除关联
	var newRelated []string
	for _, r := range mem.Related {
		if r != relatedID {
			newRelated = append(newRelated, r)
		}
	}
	mem.Related = newRelated
	mem.UpdatedAt = time.Now()

	if err := m.fileStore.UpdateMemory(mem); err != nil {
		return err
	}

	return m.index.UpdateIndex(MemoryToIndex(mem))
}

// GetRelated 获取关联的记忆
func (m *LongTermMemoryManager) GetRelated(id string) ([]*Memory, error) {
	idx, err := m.index.GetIndex(id)
	if err != nil {
		return nil, err
	}

	mem, err := m.fileStore.ReadMemory(idx.FilePath)
	if err != nil {
		return nil, err
	}

	var related []*Memory
	for _, relatedID := range mem.Related {
		relatedMem, err := m.FindByID(relatedID)
		if err == nil {
			related = append(related, relatedMem)
		}
	}

	return related, nil
}

// SetImportance 设置记忆重要性
func (m *LongTermMemoryManager) SetImportance(id string, importance int) error {
	if importance < 1 || importance > 5 {
		return fmt.Errorf("重要性必须在 1-5 之间")
	}

	idx, err := m.index.GetIndex(id)
	if err != nil {
		return err
	}

	mem, err := m.fileStore.ReadMemory(idx.FilePath)
	if err != nil {
		return err
	}

	mem.Importance = importance
	mem.UpdatedAt = time.Now()

	if err := m.fileStore.UpdateMemory(mem); err != nil {
		return err
	}

	return m.index.UpdateIndex(MemoryToIndex(mem))
}

// BuildContext 构建长期记忆上下文
func (m *LongTermMemoryManager) BuildContext(maxTokens int) (string, error) {
	active, err := m.LoadActive()
	if err != nil {
		return "", err
	}

	if len(active) == 0 {
		return "", nil
	}

	var builder strings.Builder
	builder.WriteString("## 长期记忆\n\n")

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

		if len(mem.Tags) > 0 {
			builder.WriteString(fmt.Sprintf("*标签: %s*\n", strings.Join(mem.Tags, ", ")))
		}

		builder.WriteString("\n")
		builder.WriteString(truncateContent(mem.Content, 500))
		builder.WriteString("\n\n")

		currentTokens += entryTokens
	}

	return builder.String(), nil
}

// getCategoryName 获取分类显示名称
func (m *LongTermMemoryManager) getCategoryName(category MemoryCategory) string {
	switch category {
	case CategoryProject:
		return "项目知识"
	case CategoryKnowledge:
		return "通用知识"
	case CategoryDecision:
		return "决策记录"
	default:
		return string(category)
	}
}

// GetStats 获取长期记忆统计
func (m *LongTermMemoryManager) GetStats() (*LongTermStats, error) {
	allMemories, err := m.LoadAll()
	if err != nil {
		return nil, err
	}

	stats := &LongTermStats{}

	for _, mem := range allMemories {
		stats.Total++

		switch mem.Category {
		case CategoryProject:
			stats.ProjectCount++
		case CategoryKnowledge:
			stats.KnowledgeCount++
		case CategoryDecision:
			stats.DecisionCount++
		}

		switch mem.Scope {
		case ScopeGlobal:
			stats.GlobalCount++
		case ScopeProject:
			stats.ProjectScopeCount++
		}

		if mem.Status == StatusArchived {
			stats.ArchivedCount++
		}
	}

	return stats, nil
}

// LongTermStats 长期记忆统计
type LongTermStats struct {
	Total             int `json:"total"`
	ProjectCount      int `json:"project_count"`
	KnowledgeCount    int `json:"knowledge_count"`
	DecisionCount     int `json:"decision_count"`
	GlobalCount       int `json:"global_count"`
	ProjectScopeCount int `json:"project_scope_count"`
	ArchivedCount     int `json:"archived_count"`
}

// EnsureDirectories 确保长期记忆目录存在
func (m *LongTermMemoryManager) EnsureDirectories() error {
	dirs := []string{
		m.storage.GetLongTermProjectsPath(ScopeGlobal),
		m.storage.GetLongTermKnowledgePath(ScopeGlobal),
		m.storage.GetLongTermDecisionsPath(ScopeGlobal),
	}

	if m.storage.GetProjectRoot() != "" {
		dirs = append(dirs,
			m.storage.GetLongTermProjectsPath(ScopeProject),
			m.storage.GetLongTermKnowledgePath(ScopeProject),
			m.storage.GetLongTermDecisionsPath(ScopeProject),
		)
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// PromoteFromShortTerm 从短期记忆提升为长期记忆
func (m *LongTermMemoryManager) PromoteFromShortTerm(shortTermMem *Memory) (*Memory, error) {
	// 创建长期记忆
	longTermMem, err := m.Add(
		CategoryKnowledge,
		shortTermMem.Scope,
		shortTermMem.Title,
		shortTermMem.Content,
		shortTermMem.Tags,
	)
	if err != nil {
		return nil, err
	}

	// 保留访问计数
	longTermMem.AccessCount = shortTermMem.AccessCount

	return longTermMem, nil
}

// ListProjectKnowledge 列出项目知识
func (m *LongTermMemoryManager) ListProjectKnowledge() ([]*Memory, error) {
	return m.LoadByCategory(CategoryProject)
}

// ListKnowledge 列出通用知识
func (m *LongTermMemoryManager) ListKnowledge() ([]*Memory, error) {
	return m.LoadByCategory(CategoryKnowledge)
}

// ListDecisions 列出决策记录
func (m *LongTermMemoryManager) ListDecisions() ([]*Memory, error) {
	return m.LoadByCategory(CategoryDecision)
}
