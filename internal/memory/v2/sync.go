// Package v2 提供索引同步功能
package v2

import (
	"fmt"
	"os"
	"time"
)

// SyncResult 同步结果
type SyncResult struct {
	// 同步时间
	SyncTime time.Time `json:"sync_time"`

	// 新增索引数
	Created int `json:"created"`

	// 更新索引数
	Updated int `json:"updated"`

	// 删除索引数
	Deleted int `json:"deleted"`

	// 跳过数（无变化）
	Skipped int `json:"skipped"`

	// 错误数
	Errors int `json:"errors"`

	// 错误详情
	ErrorDetails []string `json:"error_details,omitempty"`

	// 耗时（毫秒）
	DurationMs int64 `json:"duration_ms"`
}

// IndexSyncer 索引同步器
type IndexSyncer struct {
	storage   *StorageManager
	fileStore *MarkdownFileStore
	index     IndexStore
	vector    VectorStore
	parser    *FrontmatterParser
}

// NewIndexSyncer 创建索引同步器
func NewIndexSyncer(
	storage *StorageManager,
	fileStore *MarkdownFileStore,
	index IndexStore,
	vector VectorStore,
) *IndexSyncer {
	return &IndexSyncer{
		storage:   storage,
		fileStore: fileStore,
		index:     index,
		vector:    vector,
		parser:    NewFrontmatterParser(),
	}
}

// SyncAll 同步所有记忆文件与索引
func (s *IndexSyncer) SyncAll() (*SyncResult, error) {
	startTime := time.Now()
	result := &SyncResult{
		SyncTime: startTime,
	}

	// 1. 获取所有文件
	files, err := s.storage.GetAllMemoryPaths()
	if err != nil {
		return nil, NewMemoryError("SyncAll", err)
	}

	// 2. 获取所有索引
	allIndexes, err := s.index.GetAllIndexes()
	if err != nil {
		return nil, NewMemoryError("SyncAll", err)
	}

	// 构建索引映射
	indexByPath := make(map[string]*MemoryIndex)
	for _, idx := range allIndexes {
		indexByPath[idx.FilePath] = idx
	}

	// 构建文件集合
	fileSet := make(map[string]bool)
	for _, f := range files {
		fileSet[f] = true
	}

	// 3. 处理每个文件
	for _, filePath := range files {
		err := s.syncFile(filePath, indexByPath[filePath], result)
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, fmt.Sprintf("%s: %v", filePath, err))
		}
	}

	// 4. 清理孤立索引（文件不存在的索引）
	for path, idx := range indexByPath {
		if !fileSet[path] {
			if err := s.index.DeleteIndex(idx.ID); err != nil {
				result.Errors++
				result.ErrorDetails = append(result.ErrorDetails, fmt.Sprintf("删除孤立索引 %s: %v", idx.ID, err))
			} else {
				result.Deleted++
			}

			// 同时删除向量
			if s.vector != nil {
				_ = s.vector.DeleteVector(idx.ID)
			}
		}
	}

	result.DurationMs = time.Since(startTime).Milliseconds()
	return result, nil
}

// syncFile 同步单个文件
func (s *IndexSyncer) syncFile(filePath string, existingIndex *MemoryIndex, result *SyncResult) error {
	// 读取文件
	mem, err := s.fileStore.ReadMemory(filePath)
	if err != nil {
		return err
	}

	// 计算当前内容哈希
	currentHash := CalculateContentHash([]byte(mem.Content))

	if existingIndex == nil {
		// 新文件，创建索引
		idx := MemoryToIndex(mem)
		idx.ContentHash = currentHash
		if err := s.index.CreateIndex(idx); err != nil {
			return err
		}
		result.Created++
	} else if existingIndex.ContentHash != currentHash {
		// 文件已更新，更新索引
		idx := MemoryToIndex(mem)
		idx.ContentHash = currentHash
		idx.AccessCount = existingIndex.AccessCount // 保留访问计数
		if err := s.index.UpdateIndex(idx); err != nil {
			return err
		}
		result.Updated++
	} else {
		// 无变化
		result.Skipped++
	}

	return nil
}

// SyncSingle 同步单个文件
func (s *IndexSyncer) SyncSingle(filePath string) error {
	// 检查文件是否存在
	if !FileExists(filePath) {
		// 文件不存在，删除索引
		return s.index.DeleteIndexByPath(filePath)
	}

	// 读取文件
	mem, err := s.fileStore.ReadMemory(filePath)
	if err != nil {
		return err
	}

	// 检查现有索引
	existingIndex, err := s.index.GetIndexByPath(filePath)
	if err != nil && !IsNotFound(err) {
		return err
	}

	// 计算内容哈希
	currentHash := CalculateContentHash([]byte(mem.Content))

	if existingIndex == nil {
		// 创建新索引
		idx := MemoryToIndex(mem)
		idx.ContentHash = currentHash
		return s.index.CreateIndex(idx)
	}

	// 检查是否需要更新
	if existingIndex.ContentHash != currentHash {
		idx := MemoryToIndex(mem)
		idx.ContentHash = currentHash
		idx.AccessCount = existingIndex.AccessCount
		return s.index.UpdateIndex(idx)
	}

	return nil
}

// CheckConsistency 检查索引一致性
func (s *IndexSyncer) CheckConsistency() (*ConsistencyReport, error) {
	report := &ConsistencyReport{
		CheckTime: time.Now(),
	}

	// 获取所有文件
	files, err := s.storage.GetAllMemoryPaths()
	if err != nil {
		return nil, err
	}

	// 获取所有索引
	allIndexes, err := s.index.GetAllIndexes()
	if err != nil {
		return nil, err
	}

	// 构建映射
	indexByPath := make(map[string]*MemoryIndex)
	for _, idx := range allIndexes {
		indexByPath[idx.FilePath] = idx
	}

	fileSet := make(map[string]bool)
	for _, f := range files {
		fileSet[f] = true
	}

	report.TotalFiles = len(files)
	report.TotalIndexes = len(allIndexes)

	// 检查每个文件
	for _, filePath := range files {
		idx := indexByPath[filePath]
		if idx == nil {
			report.OrphanedFiles = append(report.OrphanedFiles, filePath)
			continue
		}

		// 检查哈希是否匹配
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		_, body, err := s.parser.Parse(content)
		if err != nil {
			continue
		}

		currentHash := CalculateContentHash(body)
		if currentHash != idx.ContentHash {
			report.HashMismatches = append(report.HashMismatches, &HashMismatch{
				FilePath:  filePath,
				FileHash:  currentHash,
				IndexHash: idx.ContentHash,
			})
		}
	}

	// 检查孤立索引
	for path, idx := range indexByPath {
		if !fileSet[path] {
			report.OrphanedIndexes = append(report.OrphanedIndexes, idx.ID)
		}
	}

	report.IsConsistent = len(report.OrphanedFiles) == 0 &&
		len(report.OrphanedIndexes) == 0 &&
		len(report.HashMismatches) == 0

	return report, nil
}

// ConsistencyReport 一致性检查报告
type ConsistencyReport struct {
	CheckTime       time.Time       `json:"check_time"`
	IsConsistent    bool            `json:"is_consistent"`
	TotalFiles      int             `json:"total_files"`
	TotalIndexes    int             `json:"total_indexes"`
	OrphanedFiles   []string        `json:"orphaned_files,omitempty"`
	OrphanedIndexes []string        `json:"orphaned_indexes,omitempty"`
	HashMismatches  []*HashMismatch `json:"hash_mismatches,omitempty"`
}

// HashMismatch 哈希不匹配记录
type HashMismatch struct {
	FilePath  string `json:"file_path"`
	FileHash  string `json:"file_hash"`
	IndexHash string `json:"index_hash"`
}

// Reindex 重建索引
func (s *IndexSyncer) Reindex() (*SyncResult, error) {
	startTime := time.Now()
	result := &SyncResult{
		SyncTime: startTime,
	}

	// 获取所有文件
	files, err := s.storage.GetAllMemoryPaths()
	if err != nil {
		return nil, err
	}

	// 对每个文件创建/更新索引
	for _, filePath := range files {
		mem, err := s.fileStore.ReadMemory(filePath)
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, fmt.Sprintf("%s: %v", filePath, err))
			continue
		}

		idx := MemoryToIndex(mem)
		idx.ContentHash = CalculateContentHash([]byte(mem.Content))

		// 尝试获取现有索引
		existing, _ := s.index.GetIndex(idx.ID)
		if existing != nil {
			idx.AccessCount = existing.AccessCount // 保留访问计数
			if err := s.index.UpdateIndex(idx); err != nil {
				result.Errors++
				result.ErrorDetails = append(result.ErrorDetails, fmt.Sprintf("更新索引 %s: %v", idx.ID, err))
			} else {
				result.Updated++
			}
		} else {
			if err := s.index.CreateIndex(idx); err != nil {
				result.Errors++
				result.ErrorDetails = append(result.ErrorDetails, fmt.Sprintf("创建索引 %s: %v", idx.ID, err))
			} else {
				result.Created++
			}
		}
	}

	result.DurationMs = time.Since(startTime).Milliseconds()
	return result, nil
}

// CleanOrphanedIndexes 清理孤立索引
func (s *IndexSyncer) CleanOrphanedIndexes() (int, error) {
	// 获取所有文件
	files, err := s.storage.GetAllMemoryPaths()
	if err != nil {
		return 0, err
	}

	fileSet := make(map[string]bool)
	for _, f := range files {
		fileSet[f] = true
	}

	// 获取孤立索引
	orphaned, err := s.index.GetOrphanedIndexes(fileSet)
	if err != nil {
		return 0, err
	}

	// 删除孤立索引
	deleted := 0
	for _, idx := range orphaned {
		if err := s.index.DeleteIndex(idx.ID); err == nil {
			deleted++
			// 同时删除向量
			if s.vector != nil {
				_ = s.vector.DeleteVector(idx.ID)
			}
		}
	}

	return deleted, nil
}

// IndexOrphanedFiles 为孤立文件创建索引
func (s *IndexSyncer) IndexOrphanedFiles() (int, error) {
	// 获取所有文件
	files, err := s.storage.GetAllMemoryPaths()
	if err != nil {
		return 0, err
	}

	// 获取所有索引
	allIndexes, err := s.index.GetAllIndexes()
	if err != nil {
		return 0, err
	}

	// 构建已索引文件集合
	indexedFiles := make(map[string]bool)
	for _, idx := range allIndexes {
		indexedFiles[idx.FilePath] = true
	}

	// 为孤立文件创建索引
	indexed := 0
	for _, filePath := range files {
		if indexedFiles[filePath] {
			continue
		}

		mem, err := s.fileStore.ReadMemory(filePath)
		if err != nil {
			continue
		}

		idx := MemoryToIndex(mem)
		idx.ContentHash = CalculateContentHash([]byte(mem.Content))

		if err := s.index.CreateIndex(idx); err == nil {
			indexed++
		}
	}

	return indexed, nil
}

// WatchFileChange 监听文件变更（用于增量同步）
// 返回需要同步的文件列表
func (s *IndexSyncer) WatchFileChange(since time.Time) ([]string, error) {
	files, err := s.storage.GetAllMemoryPaths()
	if err != nil {
		return nil, err
	}

	var changedFiles []string
	for _, filePath := range files {
		modTime, err := GetFileModTime(filePath)
		if err != nil {
			continue
		}

		if modTime.After(since) {
			changedFiles = append(changedFiles, filePath)
		}
	}

	return changedFiles, nil
}

// IncrementalSync 增量同步
func (s *IndexSyncer) IncrementalSync(since time.Time) (*SyncResult, error) {
	startTime := time.Now()
	result := &SyncResult{
		SyncTime: startTime,
	}

	// 获取变更文件
	changedFiles, err := s.WatchFileChange(since)
	if err != nil {
		return nil, err
	}

	// 同步变更文件
	for _, filePath := range changedFiles {
		if err := s.SyncSingle(filePath); err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, fmt.Sprintf("%s: %v", filePath, err))
		} else {
			result.Updated++
		}
	}

	result.DurationMs = time.Since(startTime).Milliseconds()
	return result, nil
}
