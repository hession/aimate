// Package v2 提供双层文件存储架构实现
package v2

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StorageManager 存储管理器
// 负责管理全局存储和项目存储的双层架构
type StorageManager struct {
	config *MemoryConfig

	// 全局存储根目录
	globalRoot string

	// 当前项目路径
	currentProject string

	// 项目存储根目录
	projectRoot string
}

// NewStorageManager 创建存储管理器
func NewStorageManager(config *MemoryConfig) (*StorageManager, error) {
	sm := &StorageManager{
		config:     config,
		globalRoot: config.Storage.GlobalRoot,
	}

	// 确保全局存储目录存在
	if err := sm.ensureGlobalDirs(); err != nil {
		return nil, err
	}

	return sm, nil
}

// SetCurrentProject 设置当前项目
func (sm *StorageManager) SetCurrentProject(projectPath string) error {
	// 检测项目根目录
	projectRoot := sm.detectProjectRoot(projectPath)
	if projectRoot == "" {
		// 未检测到项目根目录，使用给定路径
		projectRoot = projectPath
	}

	sm.currentProject = projectRoot
	sm.projectRoot = filepath.Join(projectRoot, sm.config.Storage.ProjectDirName)

	// 确保项目存储目录存在
	return sm.ensureProjectDirs()
}

// GetCurrentProject 获取当前项目路径
func (sm *StorageManager) GetCurrentProject() string {
	return sm.currentProject
}

// detectProjectRoot 检测项目根目录
// 从给定路径向上查找，直到找到包含标记文件的目录
func (sm *StorageManager) detectProjectRoot(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return ""
	}

	// 向上查找直到根目录
	for {
		for _, marker := range sm.config.Storage.ProjectMarkers {
			markerPath := filepath.Join(absPath, marker)
			if _, err := os.Stat(markerPath); err == nil {
				return absPath
			}
		}

		parent := filepath.Dir(absPath)
		if parent == absPath {
			// 已到达根目录
			break
		}
		absPath = parent
	}

	return ""
}

// ensureGlobalDirs 确保全局存储目录结构存在
func (sm *StorageManager) ensureGlobalDirs() error {
	dirs := []string{
		sm.globalRoot,
		sm.GetGlobalCorePath(),
		sm.GetGlobalSessionsPath(),
		sm.GetGlobalShortTermPath(),
		sm.GetGlobalLongTermPath(),
		sm.GetGlobalArchivePath(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return NewMemoryErrorWithPath("ensureGlobalDirs", dir, err)
		}
	}

	return nil
}

// ensureProjectDirs 确保项目存储目录结构存在
func (sm *StorageManager) ensureProjectDirs() error {
	if sm.projectRoot == "" {
		return nil
	}

	dirs := []string{
		sm.projectRoot,
		sm.GetProjectSessionsPath(),
		sm.GetProjectShortTermPath(),
		sm.GetProjectLongTermPath(),
		sm.GetProjectArchivePath(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return NewMemoryErrorWithPath("ensureProjectDirs", dir, err)
		}
	}

	return nil
}

// ========== 全局存储路径 ==========

// GetGlobalRoot 获取全局存储根目录
func (sm *StorageManager) GetGlobalRoot() string {
	return sm.globalRoot
}

// GetGlobalCorePath 获取全局核心记忆目录
func (sm *StorageManager) GetGlobalCorePath() string {
	return filepath.Join(sm.globalRoot, "core")
}

// GetGlobalSessionsPath 获取全局会话目录
func (sm *StorageManager) GetGlobalSessionsPath() string {
	return filepath.Join(sm.globalRoot, "sessions")
}

// GetGlobalShortTermPath 获取全局短期记忆目录
func (sm *StorageManager) GetGlobalShortTermPath() string {
	return filepath.Join(sm.globalRoot, "short_term")
}

// GetGlobalLongTermPath 获取全局长期记忆目录
func (sm *StorageManager) GetGlobalLongTermPath() string {
	return filepath.Join(sm.globalRoot, "long_term")
}

// GetGlobalArchivePath 获取全局归档目录
func (sm *StorageManager) GetGlobalArchivePath() string {
	return filepath.Join(sm.globalRoot, "archive")
}

// GetGlobalIndexDBPath 获取全局索引数据库路径
func (sm *StorageManager) GetGlobalIndexDBPath() string {
	return filepath.Join(sm.globalRoot, sm.config.Storage.IndexDBName)
}

// ========== 项目存储路径 ==========

// GetProjectRoot 获取项目存储根目录
func (sm *StorageManager) GetProjectRoot() string {
	return sm.projectRoot
}

// GetProjectSessionsPath 获取项目会话目录
func (sm *StorageManager) GetProjectSessionsPath() string {
	if sm.projectRoot == "" {
		return ""
	}
	return filepath.Join(sm.projectRoot, "sessions")
}

// GetProjectShortTermPath 获取项目短期记忆目录
func (sm *StorageManager) GetProjectShortTermPath() string {
	if sm.projectRoot == "" {
		return ""
	}
	return filepath.Join(sm.projectRoot, "short_term")
}

// GetProjectLongTermPath 获取项目长期记忆目录
func (sm *StorageManager) GetProjectLongTermPath() string {
	if sm.projectRoot == "" {
		return ""
	}
	return filepath.Join(sm.projectRoot, "long_term")
}

// GetProjectArchivePath 获取项目归档目录
func (sm *StorageManager) GetProjectArchivePath() string {
	if sm.projectRoot == "" {
		return ""
	}
	return filepath.Join(sm.projectRoot, "archive")
}

// GetProjectIndexDBPath 获取项目索引数据库路径
func (sm *StorageManager) GetProjectIndexDBPath() string {
	if sm.projectRoot == "" {
		return ""
	}
	return filepath.Join(sm.projectRoot, sm.config.Storage.IndexDBName)
}

// ========== 短期记忆子目录 ==========

// GetShortTermTasksPath 获取短期记忆任务目录（根据 scope）
func (sm *StorageManager) GetShortTermTasksPath(scope MemoryScope) string {
	base := sm.GetShortTermBasePath(scope)
	return filepath.Join(base, "tasks")
}

// GetShortTermNotesPath 获取短期记忆笔记目录
func (sm *StorageManager) GetShortTermNotesPath(scope MemoryScope) string {
	base := sm.GetShortTermBasePath(scope)
	return filepath.Join(base, "notes")
}

// GetShortTermContextsPath 获取短期记忆上下文目录
func (sm *StorageManager) GetShortTermContextsPath(scope MemoryScope) string {
	base := sm.GetShortTermBasePath(scope)
	return filepath.Join(base, "contexts")
}

// GetShortTermBasePath 获取短期记忆基础路径
func (sm *StorageManager) GetShortTermBasePath(scope MemoryScope) string {
	if scope == ScopeProject && sm.projectRoot != "" {
		return sm.GetProjectShortTermPath()
	}
	return sm.GetGlobalShortTermPath()
}

// ========== 长期记忆子目录 ==========

// GetLongTermProjectsPath 获取长期记忆项目目录
func (sm *StorageManager) GetLongTermProjectsPath(scope MemoryScope) string {
	base := sm.GetLongTermBasePath(scope)
	return filepath.Join(base, "projects")
}

// GetLongTermKnowledgePath 获取长期记忆知识目录
func (sm *StorageManager) GetLongTermKnowledgePath(scope MemoryScope) string {
	base := sm.GetLongTermBasePath(scope)
	return filepath.Join(base, "knowledge")
}

// GetLongTermDecisionsPath 获取长期记忆决策目录
func (sm *StorageManager) GetLongTermDecisionsPath(scope MemoryScope) string {
	base := sm.GetLongTermBasePath(scope)
	return filepath.Join(base, "decisions")
}

// GetLongTermBasePath 获取长期记忆基础路径
func (sm *StorageManager) GetLongTermBasePath(scope MemoryScope) string {
	if scope == ScopeProject && sm.projectRoot != "" {
		return sm.GetProjectLongTermPath()
	}
	return sm.GetGlobalLongTermPath()
}

// ========== 文件路径生成 ==========

// GenerateMemoryFilePath 生成记忆文件路径
func (sm *StorageManager) GenerateMemoryFilePath(mem *Memory) string {
	var basePath string

	switch mem.Type {
	case MemoryTypeCore:
		// 核心记忆只存在全局
		basePath = sm.GetGlobalCorePath()

	case MemoryTypeSession:
		// 会话记忆根据项目存储
		if mem.Scope == ScopeProject && sm.projectRoot != "" {
			basePath = sm.GetProjectSessionsPath()
		} else {
			basePath = sm.GetGlobalSessionsPath()
		}

	case MemoryTypeShortTerm:
		basePath = sm.getShortTermCategoryPath(mem.Category, mem.Scope)

	case MemoryTypeLongTerm:
		basePath = sm.getLongTermCategoryPath(mem.Category, mem.Scope)

	default:
		basePath = sm.GetGlobalLongTermPath()
	}

	// 生成文件名
	fileName := sm.generateFileName(mem)
	return filepath.Join(basePath, fileName)
}

// getShortTermCategoryPath 根据分类获取短期记忆路径
func (sm *StorageManager) getShortTermCategoryPath(category MemoryCategory, scope MemoryScope) string {
	switch category {
	case CategoryTask:
		return sm.GetShortTermTasksPath(scope)
	case CategoryNote:
		return sm.GetShortTermNotesPath(scope)
	case CategoryContext:
		return sm.GetShortTermContextsPath(scope)
	default:
		return sm.GetShortTermBasePath(scope)
	}
}

// getLongTermCategoryPath 根据分类获取长期记忆路径
func (sm *StorageManager) getLongTermCategoryPath(category MemoryCategory, scope MemoryScope) string {
	switch category {
	case CategoryProject:
		return sm.GetLongTermProjectsPath(scope)
	case CategoryKnowledge:
		return sm.GetLongTermKnowledgePath(scope)
	case CategoryDecision:
		return sm.GetLongTermDecisionsPath(scope)
	default:
		return sm.GetLongTermBasePath(scope)
	}
}

// generateFileName 生成文件名
// 格式：YYYYMMDD_category_title.md
func (sm *StorageManager) generateFileName(mem *Memory) string {
	date := mem.CreatedAt.Format("20060102")
	title := sanitizeFileName(mem.Title)
	if title == "" {
		title = mem.ID[:8] // 使用 ID 前 8 位
	}

	// 限制文件名长度
	if len(title) > 50 {
		title = title[:50]
	}

	return fmt.Sprintf("%s_%s_%s.md", date, string(mem.Category), title)
}

// GenerateSessionFilePath 生成会话文件路径
func (sm *StorageManager) GenerateSessionFilePath(sess *Session) string {
	var basePath string
	if sm.projectRoot != "" {
		basePath = sm.GetProjectSessionsPath()
	} else {
		basePath = sm.GetGlobalSessionsPath()
	}

	// 按日期组织子目录
	date := sess.CreatedAt.Format("2006-01")
	dateDir := filepath.Join(basePath, date)

	// 生成文件名
	fileName := fmt.Sprintf("%s_%s.md", sess.CreatedAt.Format("20060102_150405"), sess.ID[:8])
	return filepath.Join(dateDir, fileName)
}

// GetArchivePath 获取归档路径
func (sm *StorageManager) GetArchivePath(mem *Memory) string {
	var basePath string
	if mem.Scope == ScopeProject && sm.projectRoot != "" {
		basePath = sm.GetProjectArchivePath()
	} else {
		basePath = sm.GetGlobalArchivePath()
	}

	// 按年月组织
	date := time.Now().Format("2006-01")
	return filepath.Join(basePath, date, string(mem.Type))
}

// ========== 工具函数 ==========

// sanitizeFileName 清理文件名中的非法字符
func sanitizeFileName(name string) string {
	// 替换非法字符
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	name = replacer.Replace(name)

	// 移除连续的下划线
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}

	// 移除首尾下划线
	name = strings.Trim(name, "_")

	return name
}

// IsGlobalPath 检查路径是否在全局存储中
func (sm *StorageManager) IsGlobalPath(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return strings.HasPrefix(absPath, sm.globalRoot)
}

// IsProjectPath 检查路径是否在项目存储中
func (sm *StorageManager) IsProjectPath(path string) bool {
	if sm.projectRoot == "" {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return strings.HasPrefix(absPath, sm.projectRoot)
}

// GetScopeFromPath 从路径推断作用域
func (sm *StorageManager) GetScopeFromPath(path string) MemoryScope {
	if sm.IsProjectPath(path) {
		return ScopeProject
	}
	return ScopeGlobal
}

// GetMemoryTypeFromPath 从路径推断记忆类型
func (sm *StorageManager) GetMemoryTypeFromPath(path string) MemoryType {
	if strings.Contains(path, "/core/") {
		return MemoryTypeCore
	}
	if strings.Contains(path, "/sessions/") {
		return MemoryTypeSession
	}
	if strings.Contains(path, "/short_term/") {
		return MemoryTypeShortTerm
	}
	if strings.Contains(path, "/long_term/") {
		return MemoryTypeLongTerm
	}
	return MemoryTypeLongTerm // 默认
}

// ListMemoryFiles 列出指定目录下的所有记忆文件
func (sm *StorageManager) ListMemoryFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, NewMemoryErrorWithPath("ListMemoryFiles", dir, err)
	}

	return files, nil
}

// GetAllMemoryPaths 获取所有记忆文件路径（全局+项目）
func (sm *StorageManager) GetAllMemoryPaths() ([]string, error) {
	var allFiles []string

	// 全局记忆
	globalDirs := []string{
		sm.GetGlobalCorePath(),
		sm.GetGlobalShortTermPath(),
		sm.GetGlobalLongTermPath(),
	}

	for _, dir := range globalDirs {
		files, err := sm.ListMemoryFiles(dir)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		allFiles = append(allFiles, files...)
	}

	// 项目记忆
	if sm.projectRoot != "" {
		projectDirs := []string{
			sm.GetProjectShortTermPath(),
			sm.GetProjectLongTermPath(),
		}

		for _, dir := range projectDirs {
			files, err := sm.ListMemoryFiles(dir)
			if err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			allFiles = append(allFiles, files...)
		}
	}

	return allFiles, nil
}

// GetStorageStats 获取存储统计信息
func (sm *StorageManager) GetStorageStats() (*StorageStats, error) {
	stats := &StorageStats{
		GlobalPath:  sm.globalRoot,
		ProjectPath: sm.projectRoot,
	}

	// 统计全局存储
	globalFiles, err := sm.GetAllMemoryPaths()
	if err != nil {
		return nil, err
	}

	for _, file := range globalFiles {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		stats.TotalFiles++
		stats.TotalSizeBytes += info.Size()

		if sm.IsGlobalPath(file) {
			stats.GlobalFiles++
			stats.GlobalSizeBytes += info.Size()
		} else {
			stats.ProjectFiles++
			stats.ProjectSizeBytes += info.Size()
		}
	}

	return stats, nil
}

// StorageStats 存储统计信息
type StorageStats struct {
	GlobalPath       string `json:"global_path"`
	ProjectPath      string `json:"project_path"`
	TotalFiles       int    `json:"total_files"`
	TotalSizeBytes   int64  `json:"total_size_bytes"`
	GlobalFiles      int    `json:"global_files"`
	GlobalSizeBytes  int64  `json:"global_size_bytes"`
	ProjectFiles     int    `json:"project_files"`
	ProjectSizeBytes int64  `json:"project_size_bytes"`
}

// EnsureDir 确保目录存在
func EnsureDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}

// FileExists 检查文件是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetFileModTime 获取文件修改时间
func GetFileModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
