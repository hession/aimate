// Package v2 提供记忆系统 v2 的端到端场景测试
package v2

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ========== E2E 场景测试 ==========

// TestE2E_BasicMemoryFlow 测试基础记忆流程
// 场景：用户表达偏好 -> 系统识别并存储 -> 后续会话中检索
func TestE2E_BasicMemoryFlow(t *testing.T) {
	env := setupE2EEnvironment(t)
	defer env.Cleanup()

	// 1. 用户表达偏好
	userInput := "我习惯使用 vim 编辑器，请记住这个偏好"

	// 2. 分类器识别
	classifier := NewMemoryClassifier()
	result := classifier.Classify(userInput)

	if !result.ShouldStore {
		t.Fatal("应识别为需要存储的记忆")
	}

	if result.MemoryType != MemoryTypeCore {
		t.Errorf("应识别为核心记忆，实际为 %s", result.MemoryType)
	}

	// 3. 存储记忆
	coreMgr := NewCoreMemoryManager(env.Storage, env.FileStore, env.Index, env.Config)
	mem, err := coreMgr.Add(result.Category, result.Title, userInput)
	if err != nil {
		t.Fatalf("存储核心记忆失败: %v", err)
	}

	t.Logf("存储核心记忆: ID=%s, 标题=%s", mem.ID[:8], mem.Title)

	// 4. 验证持久化
	readMem, err := env.FileStore.ReadMemory(mem.FilePath)
	if err != nil {
		t.Fatalf("读取记忆失败: %v", err)
	}

	if !strings.Contains(readMem.Content, "vim") {
		t.Error("记忆内容应包含 vim")
	}

	// 5. 模拟新会话启动时的上下文构建
	coreContext, err := coreMgr.BuildContext()
	if err != nil {
		t.Fatalf("构建核心上下文失败: %v", err)
	}

	if !strings.Contains(coreContext, "vim") {
		t.Error("核心上下文应包含用户偏好")
	}

	t.Logf("核心上下文:\n%s", truncateForLog(coreContext, 200))
}

// TestE2E_CrossSessionMemoryPersistence 测试跨会话记忆保持
// 场景：会话1创建记忆 -> 会话归档 -> 会话2检索记忆
func TestE2E_CrossSessionMemoryPersistence(t *testing.T) {
	env := setupE2EEnvironment(t)
	defer env.Cleanup()

	sessionMgr := NewSessionManager(env.Storage, env.FileStore, env.Index, env.Config)
	shortTermMgr := NewShortTermMemoryManager(env.Storage, env.FileStore, env.Index, env.Config)

	// === 会话 1 ===
	t.Log("=== 会话 1 开始 ===")

	sess1, err := sessionMgr.CreateSession()
	if err != nil {
		t.Fatalf("创建会话1失败: %v", err)
	}
	t.Logf("会话1 ID: %s", sess1.ID[:8])

	// 添加对话
	_ = sessionMgr.AddMessage("user", "今天需要完成 API 文档编写", 20)
	_ = sessionMgr.AddMessage("assistant", "好的，我帮您记录这个任务", 15)

	// 创建短期任务记忆
	task, err := shortTermMgr.AddTask("完成 API 文档", "需要编写 REST API 的文档", 7)
	if err != nil {
		t.Fatalf("创建任务记忆失败: %v", err)
	}
	t.Logf("创建任务: %s", task.Title)

	// 归档会话1
	if err := sessionMgr.ArchiveCurrentSession(); err != nil {
		t.Fatalf("归档会话1失败: %v", err)
	}
	t.Log("会话1 已归档")

	// === 会话 2 ===
	t.Log("=== 会话 2 开始 ===")

	sess2, err := sessionMgr.CreateSession()
	if err != nil {
		t.Fatalf("创建会话2失败: %v", err)
	}
	t.Logf("会话2 ID: %s", sess2.ID[:8])

	// 验证会话2是新会话
	if sess2.ID == sess1.ID {
		t.Error("会话2应该是新会话")
	}

	// 搜索之前的任务
	results, err := env.Index.SearchByKeyword("API", 10)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	t.Logf("搜索 'API' 找到 %d 条结果", len(results))

	found := false
	for _, idx := range results {
		if strings.Contains(idx.Title, "API") {
			found = true
			t.Logf("找到相关记忆: %s", idx.Title)
		}
	}

	if !found {
		t.Error("应该能在新会话中找到之前创建的任务记忆")
	}

	// 加载活跃的短期记忆
	active, _ := shortTermMgr.LoadActive()
	t.Logf("活跃短期记忆数: %d", len(active))

	if len(active) == 0 {
		t.Error("应该有活跃的短期记忆")
	}
}

// TestE2E_ContextTrimAndRecover 测试上下文裁剪与恢复
// 场景：会话 Token 超限 -> 触发裁剪 -> 生成摘要 -> 继续对话
func TestE2E_ContextTrimAndRecover(t *testing.T) {
	env := setupE2EEnvironment(t)
	defer env.Cleanup()

	// 设置较低的 Token 限制以触发裁剪
	env.Config.Session.MaxTokens = 500

	sessionMgr := NewSessionManager(env.Storage, env.FileStore, env.Index, env.Config)
	shortTermMgr := NewShortTermMemoryManager(env.Storage, env.FileStore, env.Index, env.Config)

	// 创建会话
	_, err := sessionMgr.CreateSession()
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	// 添加大量消息直到接近限制
	for i := 0; i < 10; i++ {
		_ = sessionMgr.AddMessage("user", "这是一条较长的用户消息，包含一些重要信息", 30)
		_ = sessionMgr.AddMessage("assistant", "这是助手的回复，同样包含较长的内容和建议", 40)
	}

	// 检查是否需要裁剪
	needsTrim := sessionMgr.NeedsTrimming()
	t.Logf("需要裁剪: %v", needsTrim)

	current, max, ratio := sessionMgr.GetTokenUsage()
	t.Logf("Token 使用: %d/%d (%.1f%%)", current, max, ratio*100)

	// 如果需要裁剪，执行裁剪流程
	if needsTrim {
		// 获取当前消息
		messages := sessionMgr.GetMessages()

		// 生成摘要（模拟）
		summary := "会话摘要：用户进行了多轮对话，讨论了重要信息和建议..."

		// 保存摘要到短期记忆
		_, err := shortTermMgr.AddContext("会话摘要", summary, 14)
		if err != nil {
			t.Fatalf("保存摘要失败: %v", err)
		}
		t.Logf("已保存会话摘要，原消息数: %d", len(messages))

		// 清除消息（模拟裁剪）
		_ = sessionMgr.ClearMessages()

		// 验证裁剪后状态
		newCurrent, _, _ := sessionMgr.GetTokenUsage()
		if newCurrent != 0 {
			t.Errorf("裁剪后 Token 应为 0，实际为 %d", newCurrent)
		}
	}

	// 继续对话
	_ = sessionMgr.AddMessage("user", "继续我们之前的讨论", 10)

	// 验证可以检索到之前的上下文摘要
	contexts, _ := shortTermMgr.ListContexts()
	t.Logf("上下文摘要数: %d", len(contexts))
}

// TestE2E_ProjectScopeIsolation 测试跨项目记忆隔离
// 场景：项目A的记忆不应出现在项目B中
func TestE2E_ProjectScopeIsolation(t *testing.T) {
	env := setupE2EEnvironment(t)
	defer env.Cleanup()

	// 创建两个模拟项目目录
	projectA := filepath.Join(env.TmpDir, "projectA")
	projectB := filepath.Join(env.TmpDir, "projectB")

	_ = os.MkdirAll(filepath.Join(projectA, ".git"), 0755)
	_ = os.MkdirAll(filepath.Join(projectB, ".git"), 0755)

	// === 在项目 A 中创建记忆 ===
	if err := env.Storage.SetCurrentProject(projectA); err != nil {
		t.Fatalf("设置项目A失败: %v", err)
	}

	shortTermMgr := NewShortTermMemoryManager(env.Storage, env.FileStore, env.Index, env.Config)

	// 创建项目A的记忆
	memA, err := shortTermMgr.Add(CategoryTask, ScopeProject, "项目A任务", "项目A的任务内容", 7)
	if err != nil {
		t.Fatalf("创建项目A记忆失败: %v", err)
	}
	t.Logf("项目A记忆: %s (路径: %s)", memA.Title, memA.FilePath)

	// === 切换到项目 B ===
	if err := env.Storage.SetCurrentProject(projectB); err != nil {
		t.Fatalf("设置项目B失败: %v", err)
	}

	// 创建新的 shortTermMgr（使用更新后的 storage）
	shortTermMgrB := NewShortTermMemoryManager(env.Storage, env.FileStore, env.Index, env.Config)

	// 创建项目B的记忆
	memB, err := shortTermMgrB.Add(CategoryTask, ScopeProject, "项目B任务", "项目B的任务内容", 7)
	if err != nil {
		t.Fatalf("创建项目B记忆失败: %v", err)
	}
	t.Logf("项目B记忆: %s (路径: %s)", memB.Title, memB.FilePath)

	// 验证文件路径不同
	if memA.FilePath == memB.FilePath {
		t.Error("项目A和项目B的记忆文件路径应该不同")
	}

	// 验证项目A的记忆文件在项目A目录下
	if !strings.Contains(memA.FilePath, "projectA") {
		t.Error("项目A的记忆应该存储在项目A目录")
	}

	// 验证项目B的记忆文件在项目B目录下
	if !strings.Contains(memB.FilePath, "projectB") {
		t.Error("项目B的记忆应该存储在项目B目录")
	}
}

// TestE2E_ManualFileEditSync 测试手动编辑文件后的同步
// 场景：用户手动编辑 Markdown 文件 -> 系统检测变化 -> 重新索引
func TestE2E_ManualFileEditSync(t *testing.T) {
	env := setupE2EEnvironment(t)
	defer env.Cleanup()

	// 创建记忆
	mem := NewMemory(MemoryTypeCore, ScopeGlobal, CategoryPreference, "手动编辑测试", "原始内容")
	if err := env.FileStore.CreateMemory(mem); err != nil {
		t.Fatalf("创建记忆失败: %v", err)
	}

	idx := MemoryToIndex(mem)
	originalHash := idx.ContentHash
	if err := env.Index.CreateIndex(idx); err != nil {
		t.Fatalf("创建索引失败: %v", err)
	}

	t.Logf("原始哈希: %s", originalHash[:16])

	// 模拟用户手动编辑文件
	mem.Content = "用户手动修改后的内容"
	if err := env.FileStore.UpdateMemory(mem); err != nil {
		t.Fatalf("更新记忆失败: %v", err)
	}

	// 读取文件并计算新哈希
	readMem, _ := env.FileStore.ReadMemory(mem.FilePath)
	newHash := CalculateContentHash([]byte(strings.TrimSpace(readMem.Content)))
	t.Logf("新哈希: %s", newHash[:16])

	// 验证哈希变化
	if newHash == originalHash {
		t.Error("文件修改后哈希应该变化")
	}

	// 模拟同步过程：检测到哈希变化，更新索引
	idx.ContentHash = newHash
	idx.UpdatedAt = time.Now()
	if err := env.Index.UpdateIndex(idx); err != nil {
		t.Fatalf("更新索引失败: %v", err)
	}

	// 验证索引已更新
	updatedIdx, _ := env.Index.GetIndex(mem.ID)
	if updatedIdx.ContentHash != newHash {
		t.Error("索引哈希应已更新")
	}
}

// TestE2E_MemoryLifecycleComplete 测试记忆完整生命周期
// 场景：创建 -> 访问 -> 过期 -> 归档
func TestE2E_MemoryLifecycleComplete(t *testing.T) {
	env := setupE2EEnvironment(t)
	defer env.Cleanup()

	shortTermMgr := NewShortTermMemoryManager(env.Storage, env.FileStore, env.Index, env.Config)

	// 1. 创建短期记忆（设置很短的 TTL 用于测试）
	mem, err := shortTermMgr.Add(CategoryTask, ScopeGlobal, "临时任务", "需要尽快完成", 0)
	if err != nil {
		t.Fatalf("创建记忆失败: %v", err)
	}

	// 手动设置为过去的过期时间
	pastTime := time.Now().Add(-time.Hour)
	mem.ExpiresAt = &pastTime
	_ = env.FileStore.UpdateMemory(mem)

	idx := MemoryToIndex(mem)
	_ = env.Index.UpdateIndex(idx)

	t.Logf("创建记忆: %s, 过期时间: %v", mem.Title, mem.ExpiresAt)

	// 2. 模拟访问（增加访问计数）
	for i := 0; i < 3; i++ {
		_ = env.Index.IncrementAccessCount(mem.ID)
	}

	updatedIdx, _ := env.Index.GetIndex(mem.ID)
	t.Logf("访问计数: %d", updatedIdx.AccessCount)

	// 3. 检测过期
	if !mem.IsExpired() {
		t.Error("记忆应已过期")
	}

	// 4. 获取过期记忆
	expired, _ := env.Index.GetExpiredMemories()
	t.Logf("过期记忆数: %d", len(expired))

	found := false
	for _, e := range expired {
		if e.ID == mem.ID {
			found = true
		}
	}

	if !found {
		t.Error("应检测到过期记忆")
	}

	// 5. 归档过期记忆
	cleaned, err := shortTermMgr.CleanExpired()
	if err != nil {
		t.Fatalf("清理过期记忆失败: %v", err)
	}

	t.Logf("清理过期记忆: %d", cleaned)
}

// TestE2E_ContextBuilder 测试完整的上下文构建流程
func TestE2E_ContextBuilder(t *testing.T) {
	env := setupE2EEnvironment(t)
	defer env.Cleanup()

	// 创建各层记忆管理器
	coreMgr := NewCoreMemoryManager(env.Storage, env.FileStore, env.Index, env.Config)
	sessionMgr := NewSessionManager(env.Storage, env.FileStore, env.Index, env.Config)
	shortTermMgr := NewShortTermMemoryManager(env.Storage, env.FileStore, env.Index, env.Config)
	longTermMgr := NewLongTermMemoryManager(env.Storage, env.FileStore, env.Index, nil, env.Config)

	// 1. 添加核心记忆
	_, _ = coreMgr.Add(CategoryPreference, "编辑器偏好", "我使用 VS Code 进行开发")

	// 2. 创建会话并添加消息
	_, _ = sessionMgr.CreateSession()
	_ = sessionMgr.AddMessage("user", "帮我优化这段代码", 10)

	// 3. 添加短期记忆
	_, _ = shortTermMgr.AddTask("代码优化", "优化性能问题", 3)

	// 4. 添加长期记忆
	_, _ = longTermMgr.Add(CategoryKnowledge, ScopeGlobal, "性能优化技巧",
		"使用缓存、减少数据库查询等", []string{"performance"})

	// 5. 创建上下文构建器
	builder := NewContextBuilder(coreMgr, sessionMgr, shortTermMgr, longTermMgr, nil, env.Config)

	// 6. 构建上下文
	ctx, err := builder.BuildContextForNewSession(context.Background())
	if err != nil {
		t.Fatalf("构建上下文失败: %v", err)
	}

	t.Logf("构建上下文:\n总Token: %d\n核心Token: %d\n短期Token: %d\n长期Token: %d",
		ctx.TotalTokens, ctx.CoreTokens, ctx.ShortTermTokens, ctx.LongTermTokens)

	// 7. 验证上下文包含各层信息
	if ctx.Content == "" {
		t.Error("上下文内容不应为空")
	}

	// 8. 检查预算
	if ctx.IsOverBudget() {
		t.Error("上下文不应超预算")
	}

	remaining := ctx.GetRemainingBudget()
	t.Logf("剩余预算: %d", remaining)
}

// ========== 测试辅助设施 ==========

// E2ETestEnvironment E2E 测试环境
type E2ETestEnvironment struct {
	TmpDir    string
	Config    *MemoryConfig
	Storage   *StorageManager
	FileStore *MarkdownFileStore
	Index     IndexStore
	cleanup   func()
}

// Cleanup 清理测试环境
func (e *E2ETestEnvironment) Cleanup() {
	if e.cleanup != nil {
		e.cleanup()
	}
}

// setupE2EEnvironment 设置 E2E 测试环境
func setupE2EEnvironment(t *testing.T) *E2ETestEnvironment {
	tmpDir, err := os.MkdirTemp("", "e2e-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	cfg := DefaultMemoryConfig()
	cfg.Storage.GlobalRoot = filepath.Join(tmpDir, "global")

	storage, err := NewStorageManager(cfg)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("创建存储管理器失败: %v", err)
	}

	fileStore := NewMarkdownFileStore(storage)

	dbPath := filepath.Join(tmpDir, "index.db")
	index, err := NewSQLiteIndexStore(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("创建索引存储失败: %v", err)
	}

	return &E2ETestEnvironment{
		TmpDir:    tmpDir,
		Config:    cfg,
		Storage:   storage,
		FileStore: fileStore,
		Index:     index,
		cleanup: func() {
			index.Close()
			os.RemoveAll(tmpDir)
		},
	}
}

// truncateForLog 截断字符串用于日志输出
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
