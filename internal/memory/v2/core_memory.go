// Package v2 提供核心记忆管理功能
package v2

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CoreMemoryManager 核心记忆管理器
// 管理用户偏好、全局规则、角色设定等核心记忆
type CoreMemoryManager struct {
	storage   *StorageManager
	fileStore *MarkdownFileStore
	index     IndexStore
	config    *MemoryConfig
}

// NewCoreMemoryManager 创建核心记忆管理器
func NewCoreMemoryManager(
	storage *StorageManager,
	fileStore *MarkdownFileStore,
	index IndexStore,
	config *MemoryConfig,
) *CoreMemoryManager {
	return &CoreMemoryManager{
		storage:   storage,
		fileStore: fileStore,
		index:     index,
		config:    config,
	}
}

// LoadAll 加载所有核心记忆
func (m *CoreMemoryManager) LoadAll() ([]*Memory, error) {
	corePath := m.storage.GetGlobalCorePath()

	// 确保目录存在
	if err := os.MkdirAll(corePath, 0755); err != nil {
		return nil, NewMemoryErrorWithPath("LoadAll", corePath, err)
	}

	// 列出所有核心记忆文件
	return m.fileStore.ListMemories(corePath)
}

// LoadByCategory 按分类加载核心记忆
func (m *CoreMemoryManager) LoadByCategory(category MemoryCategory) ([]*Memory, error) {
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

// GetPreferences 获取用户偏好
func (m *CoreMemoryManager) GetPreferences() ([]*Memory, error) {
	return m.LoadByCategory(CategoryPreference)
}

// GetRules 获取全局规则
func (m *CoreMemoryManager) GetRules() ([]*Memory, error) {
	return m.LoadByCategory(CategoryRule)
}

// GetPersonas 获取角色设定
func (m *CoreMemoryManager) GetPersonas() ([]*Memory, error) {
	return m.LoadByCategory(CategoryPersona)
}

// Add 添加核心记忆
func (m *CoreMemoryManager) Add(category MemoryCategory, title, content string) (*Memory, error) {
	// 检查是否已存在相同标题的记忆
	existing, _ := m.FindByTitle(title)
	if existing != nil {
		return nil, NewMemoryErrorWithDetails("Add", ErrFileAlreadyExists,
			fmt.Sprintf("已存在同名核心记忆: %s", title))
	}

	mem := &Memory{
		ID:         uuid.New().String(),
		Type:       MemoryTypeCore,
		Scope:      ScopeGlobal,
		Category:   category,
		Title:      title,
		Content:    content,
		Status:     StatusActive,
		Importance: 5, // 核心记忆默认最高重要度
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		AccessedAt: time.Now(),
	}

	// 生成文件路径
	mem.FilePath = m.generateFilePath(mem)

	// 创建文件
	if err := m.fileStore.CreateMemory(mem); err != nil {
		return nil, err
	}

	// 创建索引
	idx := MemoryToIndex(mem)
	if err := m.index.CreateIndex(idx); err != nil {
		// 索引创建失败，尝试删除文件
		_ = m.fileStore.DeleteMemory(mem.FilePath)
		return nil, err
	}

	return mem, nil
}

// AddPreference 添加用户偏好
func (m *CoreMemoryManager) AddPreference(title, content string) (*Memory, error) {
	return m.Add(CategoryPreference, title, content)
}

// AddRule 添加全局规则
func (m *CoreMemoryManager) AddRule(title, content string) (*Memory, error) {
	return m.Add(CategoryRule, title, content)
}

// AddPersona 添加角色设定
func (m *CoreMemoryManager) AddPersona(title, content string) (*Memory, error) {
	return m.Add(CategoryPersona, title, content)
}

// Update 更新核心记忆
func (m *CoreMemoryManager) Update(id string, content string) error {
	// 从索引获取记忆信息
	idx, err := m.index.GetIndex(id)
	if err != nil {
		return err
	}

	// 读取记忆
	mem, err := m.fileStore.ReadMemory(idx.FilePath)
	if err != nil {
		return err
	}

	// 更新内容
	mem.Content = content
	mem.UpdatedAt = time.Now()

	// 保存文件
	if err := m.fileStore.UpdateMemory(mem); err != nil {
		return err
	}

	// 更新索引
	newIdx := MemoryToIndex(mem)
	return m.index.UpdateIndex(newIdx)
}

// Delete 删除核心记忆
func (m *CoreMemoryManager) Delete(id string) error {
	// 从索引获取记忆信息
	idx, err := m.index.GetIndex(id)
	if err != nil {
		return err
	}

	// 删除文件
	if err := m.fileStore.DeleteMemory(idx.FilePath); err != nil {
		return err
	}

	// 删除索引
	return m.index.DeleteIndex(id)
}

// FindByTitle 按标题查找核心记忆
func (m *CoreMemoryManager) FindByTitle(title string) (*Memory, error) {
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

// FindByID 按 ID 查找核心记忆
func (m *CoreMemoryManager) FindByID(id string) (*Memory, error) {
	idx, err := m.index.GetIndex(id)
	if err != nil {
		return nil, err
	}

	return m.fileStore.ReadMemory(idx.FilePath)
}

// GetTotalTokens 获取核心记忆总 Token 数（估算）
func (m *CoreMemoryManager) GetTotalTokens() (int, error) {
	allMemories, err := m.LoadAll()
	if err != nil {
		return 0, err
	}

	totalTokens := 0
	for _, mem := range allMemories {
		// 简单估算：每 4 个字符约 1 个 token
		totalTokens += len(mem.Content) / 4
	}

	return totalTokens, nil
}

// IsOverLimit 检查是否超出 Token 限制
func (m *CoreMemoryManager) IsOverLimit() (bool, error) {
	tokens, err := m.GetTotalTokens()
	if err != nil {
		return false, err
	}

	return tokens > m.config.Core.MaxTokens, nil
}

// NeedsRefine 检查是否需要精炼
func (m *CoreMemoryManager) NeedsRefine() (bool, error) {
	tokens, err := m.GetTotalTokens()
	if err != nil {
		return false, err
	}

	threshold := int(float64(m.config.Core.MaxTokens) * m.config.Core.RefineThreshold)
	return tokens > threshold, nil
}

// BuildContext 构建核心记忆上下文
func (m *CoreMemoryManager) BuildContext() (string, error) {
	allMemories, err := m.LoadAll()
	if err != nil {
		return "", err
	}

	if len(allMemories) == 0 {
		return "", nil
	}

	var builder strings.Builder
	builder.WriteString("## 核心记忆\n\n")

	// 按类别组织
	categories := map[MemoryCategory][]*Memory{
		CategoryPreference: {},
		CategoryRule:       {},
		CategoryPersona:    {},
	}

	for _, mem := range allMemories {
		if _, ok := categories[mem.Category]; ok {
			categories[mem.Category] = append(categories[mem.Category], mem)
		}
	}

	// 输出用户偏好
	if prefs := categories[CategoryPreference]; len(prefs) > 0 {
		builder.WriteString("### 用户偏好\n\n")
		for _, mem := range prefs {
			builder.WriteString(fmt.Sprintf("- **%s**: %s\n", mem.Title, truncateContent(mem.Content, 200)))
		}
		builder.WriteString("\n")
	}

	// 输出角色设定
	if personas := categories[CategoryPersona]; len(personas) > 0 {
		builder.WriteString("### 角色设定\n\n")
		for _, mem := range personas {
			builder.WriteString(fmt.Sprintf("- **%s**: %s\n", mem.Title, truncateContent(mem.Content, 200)))
		}
		builder.WriteString("\n")
	}

	// 输出全局规则
	if rules := categories[CategoryRule]; len(rules) > 0 {
		builder.WriteString("### 全局规则\n\n")
		for _, mem := range rules {
			builder.WriteString(fmt.Sprintf("- **%s**: %s\n", mem.Title, truncateContent(mem.Content, 200)))
		}
		builder.WriteString("\n")
	}

	return builder.String(), nil
}

// generateFilePath 生成核心记忆文件路径
func (m *CoreMemoryManager) generateFilePath(mem *Memory) string {
	corePath := m.storage.GetGlobalCorePath()
	fileName := fmt.Sprintf("%s_%s.md", string(mem.Category), sanitizeFileName(mem.Title))
	return filepath.Join(corePath, fileName)
}

// truncateContent 截断内容
func truncateContent(content string, maxLen int) string {
	// 移除换行符
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.ReplaceAll(content, "\r", "")

	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

// InitDefaultMemories 初始化默认核心记忆
func (m *CoreMemoryManager) InitDefaultMemories() error {
	// 检查是否已有核心记忆
	existing, err := m.LoadAll()
	if err != nil {
		return err
	}

	if len(existing) > 0 {
		return nil // 已有记忆，不初始化
	}

	// 创建默认角色设定
	_, err = m.AddPersona("助手身份", `你是 AIMate，一个智能编程助手。
你的目标是帮助用户解决编程问题，提供代码建议和技术指导。
你应该：
- 提供准确、高质量的代码
- 解释清楚技术概念
- 遵循最佳实践
- 尊重用户的编程风格偏好`)

	return err
}

// MergeMemory 合并相似的核心记忆
func (m *CoreMemoryManager) MergeMemory(id1, id2, mergedTitle, mergedContent string) (*Memory, error) {
	// 获取两个记忆
	mem1, err := m.FindByID(id1)
	if err != nil {
		return nil, err
	}

	mem2, err := m.FindByID(id2)
	if err != nil {
		return nil, err
	}

	// 验证类型相同
	if mem1.Category != mem2.Category {
		return nil, NewMemoryErrorWithDetails("MergeMemory", ErrOperationFailed, "只能合并相同类型的记忆")
	}

	// 创建合并后的记忆
	merged, err := m.Add(mem1.Category, mergedTitle, mergedContent)
	if err != nil {
		return nil, err
	}

	// 删除原记忆
	_ = m.Delete(id1)
	_ = m.Delete(id2)

	return merged, nil
}
