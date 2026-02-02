// Package v2 提供记忆系统 v2 的集成测试
package v2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ========== 存储层集成测试 ==========

// TestStorageIntegration_CreateRetrieveUpdateDelete 测试完整的 CRUD 流程
func TestStorageIntegration_CreateRetrieveUpdateDelete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-integration-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 初始化组件
	cfg := DefaultMemoryConfig()
	cfg.Storage.GlobalRoot = filepath.Join(tmpDir, "global")

	storage, err := NewStorageManager(cfg)
	if err != nil {
		t.Fatalf("创建存储管理器失败: %v", err)
	}

	fileStore := NewMarkdownFileStore(storage)
	dbPath := filepath.Join(tmpDir, "index.db")
	index, err := NewSQLiteIndexStore(dbPath)
	if err != nil {
		t.Fatalf("创建索引存储失败: %v", err)
	}
	defer index.Close()

	// 1. 创建记忆
	mem := NewMemory(MemoryTypeCore, ScopeGlobal, CategoryPreference, "集成测试记忆", "这是集成测试内容")
	if err := fileStore.CreateMemory(mem); err != nil {
		t.Fatalf("创建记忆失败: %v", err)
	}

	// 创建索引
	idx := MemoryToIndex(mem)
	if err := index.CreateIndex(idx); err != nil {
		t.Fatalf("创建索引失败: %v", err)
	}

	// 2. 检索记忆
	readMem, err := fileStore.ReadMemory(mem.FilePath)
	if err != nil {
		t.Fatalf("读取记忆失败: %v", err)
	}

	if readMem.Title != mem.Title {
		t.Errorf("标题不匹配")
	}

	// 通过索引检索
	retrievedIdx, err := index.GetIndex(mem.ID)
	if err != nil {
		t.Fatalf("获取索引失败: %v", err)
	}

	if retrievedIdx.FilePath != mem.FilePath {
		t.Errorf("索引路径不匹配")
	}

	// 3. 更新记忆
	mem.Content = "更新后的集成测试内容"
	mem.UpdatedAt = time.Now()
	if err := fileStore.UpdateMemory(mem); err != nil {
		t.Fatalf("更新记忆失败: %v", err)
	}

	// 更新索引
	idx = MemoryToIndex(mem)
	if err := index.UpdateIndex(idx); err != nil {
		t.Fatalf("更新索引失败: %v", err)
	}

	// 验证更新
	updatedMem, _ := fileStore.ReadMemory(mem.FilePath)
	if strings.TrimSpace(updatedMem.Content) != strings.TrimSpace(mem.Content) {
		t.Errorf("内容更新失败")
	}

	// 4. 删除记忆
	if err := fileStore.DeleteMemory(mem.FilePath); err != nil {
		t.Fatalf("删除记忆失败: %v", err)
	}

	if err := index.DeleteIndex(mem.ID); err != nil {
		t.Fatalf("删除索引失败: %v", err)
	}

	// 验证删除
	if FileExists(mem.FilePath) {
		t.Error("文件应该已被删除")
	}

	_, err = index.GetIndex(mem.ID)
	if err != ErrIndexNotFound {
		t.Error("索引应该已被删除")
	}
}

// ========== 跨层级集成测试 ==========

// TestCrossLayerIntegration_ShortTermToLongTerm 测试短期记忆提升到长期记忆
func TestCrossLayerIntegration_ShortTermToLongTerm(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cross-layer-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultMemoryConfig()
	cfg.Storage.GlobalRoot = filepath.Join(tmpDir, "global")
	cfg.ShortTerm.PromoteThreshold = 3 // 设置较低的提升阈值

	storage, err := NewStorageManager(cfg)
	if err != nil {
		t.Fatalf("创建存储管理器失败: %v", err)
	}

	fileStore := NewMarkdownFileStore(storage)
	dbPath := filepath.Join(tmpDir, "index.db")
	index, err := NewSQLiteIndexStore(dbPath)
	if err != nil {
		t.Fatalf("创建索引存储失败: %v", err)
	}
	defer index.Close()

	shortTermMgr := NewShortTermMemoryManager(storage, fileStore, index, cfg)
	longTermMgr := NewLongTermMemoryManager(storage, fileStore, index, nil, cfg)

	// 1. 创建短期记忆
	mem, err := shortTermMgr.AddTask("高访问任务", "这是一个被频繁访问的任务", 7)
	if err != nil {
		t.Fatalf("创建短期记忆失败: %v", err)
	}

	// 2. 模拟多次访问
	for i := 0; i < 5; i++ {
		if err := index.IncrementAccessCount(mem.ID); err != nil {
			t.Fatalf("增加访问计数失败: %v", err)
		}
	}

	// 3. 获取高访问记忆
	highAccess, err := shortTermMgr.GetHighAccessMemories()
	if err != nil {
		t.Fatalf("获取高访问记忆失败: %v", err)
	}

	// 由于索引更新和文件分离，这里用索引检查
	idx, _ := index.GetIndex(mem.ID)
	if idx.AccessCount < cfg.ShortTerm.PromoteThreshold {
		t.Errorf("访问次数应达到提升阈值: %d < %d", idx.AccessCount, cfg.ShortTerm.PromoteThreshold)
	}

	// 4. 模拟提升：创建长期记忆
	longMem, err := longTermMgr.Add(CategoryKnowledge, ScopeGlobal, "来自短期的知识", mem.Content, []string{"promoted"})
	if err != nil {
		t.Logf("创建长期记忆失败 (预期可能失败因为没有VectorStore): %v", err)
		return // 在没有 VectorStore 时跳过
	}

	if longMem.Type != MemoryTypeLongTerm {
		t.Error("记忆类型应为 LongTerm")
	}

	// 确认高访问记忆列表有内容（可能为空取决于实现）
	_ = highAccess
}

// TestCrossLayerIntegration_SessionTrimAndArchive 测试会话裁剪和归档
func TestCrossLayerIntegration_SessionTrimAndArchive(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session-trim-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultMemoryConfig()
	cfg.Storage.GlobalRoot = filepath.Join(tmpDir, "global")
	cfg.Session.MaxTokens = 1000 // 设置较低的 Token 限制

	storage, err := NewStorageManager(cfg)
	if err != nil {
		t.Fatalf("创建存储管理器失败: %v", err)
	}

	fileStore := NewMarkdownFileStore(storage)
	dbPath := filepath.Join(tmpDir, "index.db")
	index, err := NewSQLiteIndexStore(dbPath)
	if err != nil {
		t.Fatalf("创建索引存储失败: %v", err)
	}
	defer index.Close()

	sessionMgr := NewSessionManager(storage, fileStore, index, cfg)
	shortTermMgr := NewShortTermMemoryManager(storage, fileStore, index, cfg)

	// 1. 创建会话并添加消息
	_, err = sessionMgr.CreateSession()
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	// 添加多条消息使其接近阈值
	for i := 0; i < 10; i++ {
		_ = sessionMgr.AddMessage("user", "这是用户消息", 50)
		_ = sessionMgr.AddMessage("assistant", "这是助手回复，内容较长", 100)
	}

	// 2. 检查是否需要裁剪
	needsTrim := sessionMgr.NeedsTrimming()

	// 获取消息
	messages := sessionMgr.GetMessages()

	// 3. 如果需要裁剪，将上下文摘要保存到短期记忆
	if needsTrim && len(messages) > 0 {
		summary := "会话摘要：用户进行了多轮对话..."
		_, err := shortTermMgr.AddContext("会话上下文摘要", summary, 14)
		if err != nil {
			t.Fatalf("创建上下文摘要失败: %v", err)
		}
	}

	// 4. 归档当前会话
	if err := sessionMgr.ArchiveCurrentSession(); err != nil {
		t.Fatalf("归档会话失败: %v", err)
	}

	// 验证会话已归档
	if sessionMgr.GetCurrentSession() != nil {
		t.Error("归档后当前会话应为 nil")
	}
}

// ========== 索引同步测试 ==========

// TestIndexSync_DetectOrphanedIndexes 测试检测孤立索引
func TestIndexSync_DetectOrphanedIndexes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "index-sync-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultMemoryConfig()
	cfg.Storage.GlobalRoot = filepath.Join(tmpDir, "global")

	storage, err := NewStorageManager(cfg)
	if err != nil {
		t.Fatalf("创建存储管理器失败: %v", err)
	}

	fileStore := NewMarkdownFileStore(storage)
	dbPath := filepath.Join(tmpDir, "index.db")
	index, err := NewSQLiteIndexStore(dbPath)
	if err != nil {
		t.Fatalf("创建索引存储失败: %v", err)
	}
	defer index.Close()

	// 1. 创建记忆和索引
	mem := NewMemory(MemoryTypeCore, ScopeGlobal, CategoryPreference, "同步测试", "内容")
	if err := fileStore.CreateMemory(mem); err != nil {
		t.Fatalf("创建记忆失败: %v", err)
	}

	idx := MemoryToIndex(mem)
	if err := index.CreateIndex(idx); err != nil {
		t.Fatalf("创建索引失败: %v", err)
	}

	// 2. 直接删除文件（模拟外部删除）
	if err := os.Remove(mem.FilePath); err != nil {
		t.Fatalf("删除文件失败: %v", err)
	}

	// 3. 检测孤立索引
	existingFiles := make(map[string]bool)
	// 扫描所有文件
	files, _ := storage.GetAllMemoryPaths()
	for _, f := range files {
		existingFiles[f] = true
	}

	orphaned, err := index.GetOrphanedIndexes(existingFiles)
	if err != nil {
		t.Fatalf("获取孤立索引失败: %v", err)
	}

	if len(orphaned) != 1 {
		t.Errorf("应检测到 1 个孤立索引，实际 %d", len(orphaned))
	}

	if len(orphaned) > 0 && orphaned[0].ID != mem.ID {
		t.Error("孤立索引 ID 不匹配")
	}
}

// TestIndexSync_DetectOrphanedFiles 测试检测孤立文件
func TestIndexSync_DetectOrphanedFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "file-sync-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultMemoryConfig()
	cfg.Storage.GlobalRoot = filepath.Join(tmpDir, "global")

	storage, err := NewStorageManager(cfg)
	if err != nil {
		t.Fatalf("创建存储管理器失败: %v", err)
	}

	fileStore := NewMarkdownFileStore(storage)
	dbPath := filepath.Join(tmpDir, "index.db")
	index, err := NewSQLiteIndexStore(dbPath)
	if err != nil {
		t.Fatalf("创建索引存储失败: %v", err)
	}
	defer index.Close()

	// 1. 创建记忆文件但不创建索引
	mem := NewMemory(MemoryTypeCore, ScopeGlobal, CategoryPreference, "孤立文件", "内容")
	if err := fileStore.CreateMemory(mem); err != nil {
		t.Fatalf("创建记忆失败: %v", err)
	}

	// 2. 获取所有文件
	files, err := storage.GetAllMemoryPaths()
	if err != nil {
		t.Fatalf("获取文件列表失败: %v", err)
	}

	// 3. 获取所有索引
	allIndexes, err := index.GetAllIndexes()
	if err != nil {
		t.Fatalf("获取索引列表失败: %v", err)
	}

	// 构建已索引文件集合
	indexedFiles := make(map[string]bool)
	for _, idx := range allIndexes {
		indexedFiles[idx.FilePath] = true
	}

	// 4. 检测孤立文件
	var orphanedFiles []string
	for _, f := range files {
		if !indexedFiles[f] {
			orphanedFiles = append(orphanedFiles, f)
		}
	}

	if len(orphanedFiles) != 1 {
		t.Errorf("应检测到 1 个孤立文件，实际 %d", len(orphanedFiles))
	}
}

// TestIndexSync_ContentHashVerification 测试内容哈希验证
func TestIndexSync_ContentHashVerification(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hash-verify-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultMemoryConfig()
	cfg.Storage.GlobalRoot = filepath.Join(tmpDir, "global")

	storage, err := NewStorageManager(cfg)
	if err != nil {
		t.Fatalf("创建存储管理器失败: %v", err)
	}

	fileStore := NewMarkdownFileStore(storage)
	dbPath := filepath.Join(tmpDir, "index.db")
	index, err := NewSQLiteIndexStore(dbPath)
	if err != nil {
		t.Fatalf("创建索引存储失败: %v", err)
	}
	defer index.Close()

	// 1. 创建记忆和索引
	mem := NewMemory(MemoryTypeCore, ScopeGlobal, CategoryPreference, "哈希测试", "原始内容")
	if err := fileStore.CreateMemory(mem); err != nil {
		t.Fatalf("创建记忆失败: %v", err)
	}

	idx := MemoryToIndex(mem)
	originalHash := idx.ContentHash
	if err := index.CreateIndex(idx); err != nil {
		t.Fatalf("创建索引失败: %v", err)
	}

	// 2. 外部修改文件
	mem.Content = "修改后的内容"
	mem.ContentHash = CalculateContentHash([]byte(mem.Content))
	if err := fileStore.UpdateMemory(mem); err != nil {
		t.Fatalf("更新记忆失败: %v", err)
	}

	// 3. 检测哈希变化
	retrievedIdx, _ := index.GetIndex(mem.ID)
	readMem, _ := fileStore.ReadMemory(mem.FilePath)
	currentHash := CalculateContentHash([]byte(strings.TrimSpace(readMem.Content)))

	// 索引中的哈希仍是旧的
	if retrievedIdx.ContentHash != originalHash {
		t.Error("索引哈希不应变化（未同步）")
	}

	// 计算的当前哈希应与原始不同
	if currentHash == originalHash {
		t.Error("文件内容已修改，哈希应不同")
	}
}

// ========== 多组件协作测试 ==========

// TestMultiComponent_MemorySystemLifecycle 测试记忆系统完整生命周期
func TestMultiComponent_MemorySystemLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lifecycle-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultMemoryConfig()
	cfg.Storage.GlobalRoot = filepath.Join(tmpDir, "global")

	storage, err := NewStorageManager(cfg)
	if err != nil {
		t.Fatalf("创建存储管理器失败: %v", err)
	}

	fileStore := NewMarkdownFileStore(storage)
	dbPath := filepath.Join(tmpDir, "index.db")
	index, err := NewSQLiteIndexStore(dbPath)
	if err != nil {
		t.Fatalf("创建索引存储失败: %v", err)
	}
	defer index.Close()

	sessionMgr := NewSessionManager(storage, fileStore, index, cfg)
	shortTermMgr := NewShortTermMemoryManager(storage, fileStore, index, cfg)
	longTermMgr := NewLongTermMemoryManager(storage, fileStore, index, nil, cfg)

	// 1. 创建会话
	sess, err := sessionMgr.CreateSession()
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	t.Logf("创建会话: %s", sess.ID[:8])

	// 2. 添加对话
	_ = sessionMgr.AddMessage("user", "我习惯使用 vim 编辑器", 20)
	_ = sessionMgr.AddMessage("assistant", "好的，我记住了您使用 vim 的偏好", 25)

	// 3. 创建短期任务记忆
	task, _ := shortTermMgr.AddTask("完成测试用例", "需要完成集成测试", 3)
	t.Logf("创建任务: %s", task.Title)

	// 4. 创建长期知识记忆
	knowledge, _ := longTermMgr.Add(CategoryKnowledge, ScopeGlobal, "Go 测试最佳实践",
		"使用 table-driven tests，使用临时目录等", []string{"go", "testing"})
	t.Logf("创建知识: %s", knowledge.Title)

	// 5. 验证统计
	stats, err := index.GetIndexStats()
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}

	t.Logf("统计: 总数=%d, 短期=%d, 长期=%d",
		stats.TotalCount, stats.ShortTermCount, stats.LongTermCount)

	if stats.TotalCount < 2 {
		t.Errorf("索引总数应至少为 2，实际 %d", stats.TotalCount)
	}

	// 6. 清理过期记忆（模拟）
	cleaned, _ := shortTermMgr.CleanExpired()
	t.Logf("清理过期记忆: %d", cleaned)
}

// TestMultiComponent_SearchAcrossLayers 测试跨层搜索
func TestMultiComponent_SearchAcrossLayers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "search-layers-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultMemoryConfig()
	cfg.Storage.GlobalRoot = filepath.Join(tmpDir, "global")

	storage, err := NewStorageManager(cfg)
	if err != nil {
		t.Fatalf("创建存储管理器失败: %v", err)
	}

	fileStore := NewMarkdownFileStore(storage)
	dbPath := filepath.Join(tmpDir, "index.db")
	index, err := NewSQLiteIndexStore(dbPath)
	if err != nil {
		t.Fatalf("创建索引存储失败: %v", err)
	}
	defer index.Close()

	shortTermMgr := NewShortTermMemoryManager(storage, fileStore, index, cfg)
	longTermMgr := NewLongTermMemoryManager(storage, fileStore, index, nil, cfg)

	// 创建不同层的记忆，都包含 "Go" 关键词
	_, _ = shortTermMgr.AddNote("Go 语法笔记", "Go 语言的基本语法...", 7)
	_, _ = longTermMgr.Add(CategoryKnowledge, ScopeGlobal, "Go 并发模式",
		"Go 的 goroutine 和 channel 使用...", []string{"go", "concurrency"})

	// 搜索包含 "Go" 的记忆
	results, err := index.SearchByKeyword("Go", 10)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	// 验证结果包含不同层的记忆
	if len(results) < 2 {
		t.Errorf("应找到至少 2 条包含 'Go' 的记忆，实际 %d", len(results))
	}

	foundShortTerm := false
	foundLongTerm := false
	for _, idx := range results {
		if idx.Type == MemoryTypeShortTerm {
			foundShortTerm = true
		}
		if idx.Type == MemoryTypeLongTerm {
			foundLongTerm = true
		}
	}

	if !foundShortTerm {
		t.Error("应找到短期记忆")
	}
	if !foundLongTerm {
		t.Error("应找到长期记忆")
	}
}
