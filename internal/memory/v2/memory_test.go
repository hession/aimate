// Package v2 提供记忆系统 v2 的测试
package v2

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ========== Frontmatter 解析测试 ==========

func TestFrontmatterParser_Parse(t *testing.T) {
	parser := NewFrontmatterParser()

	content := []byte(`---
id: test-123
type: core
scope: global
category: preference
title: 测试标题
status: active
importance: 3
access_count: 0
content_hash: abc123
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
accessed_at: 2024-01-01T00:00:00Z
---

这是测试内容
第二行内容
`)

	fm, body, err := parser.Parse(content)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if len(fm) == 0 {
		t.Error("frontmatter 为空")
	}

	expectedBody := "这是测试内容\n第二行内容\n"
	if string(body) != expectedBody {
		t.Errorf("body 不匹配: 期望 %q, 实际 %q", expectedBody, string(body))
	}
}

func TestFrontmatterParser_ParseNoFrontmatter(t *testing.T) {
	parser := NewFrontmatterParser()

	content := []byte("这是没有 frontmatter 的内容")

	fm, body, err := parser.Parse(content)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if len(fm) != 0 {
		t.Error("期望 frontmatter 为空")
	}

	if string(body) != string(content) {
		t.Errorf("body 不匹配")
	}
}

func TestFrontmatterParser_SerializeMemory(t *testing.T) {
	parser := NewFrontmatterParser()

	mem := NewMemory(MemoryTypeCore, ScopeGlobal, CategoryPreference, "测试标题", "测试内容")
	mem.ID = "test-id"

	content, err := parser.SerializeMemory(mem)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	// 验证内容
	if len(content) == 0 {
		t.Error("序列化内容为空")
	}

	// 验证可以重新解析
	parsedMem, err := parser.ParseMemory(content)
	if err != nil {
		t.Fatalf("重新解析失败: %v", err)
	}

	if parsedMem.ID != mem.ID {
		t.Errorf("ID 不匹配: 期望 %s, 实际 %s", mem.ID, parsedMem.ID)
	}

	if parsedMem.Title != mem.Title {
		t.Errorf("Title 不匹配: 期望 %s, 实际 %s", mem.Title, parsedMem.Title)
	}
}

func TestCalculateContentHash(t *testing.T) {
	content1 := []byte("Hello, World!")
	content2 := []byte("Hello, World!")
	content3 := []byte("Different content")

	hash1 := CalculateContentHash(content1)
	hash2 := CalculateContentHash(content2)
	hash3 := CalculateContentHash(content3)

	if hash1 != hash2 {
		t.Error("相同内容的哈希应该相同")
	}

	if hash1 == hash3 {
		t.Error("不同内容的哈希应该不同")
	}
}

// ========== 配置测试 ==========

func TestDefaultMemoryConfig(t *testing.T) {
	cfg := DefaultMemoryConfig()

	if cfg.Version != "2.0" {
		t.Errorf("版本不匹配: %s", cfg.Version)
	}

	if cfg.Core.MaxTokens <= 0 {
		t.Error("Core.MaxTokens 应该大于 0")
	}

	if cfg.Session.MaxTokens <= 0 {
		t.Error("Session.MaxTokens 应该大于 0")
	}

	// 验证上下文比例总和
	totalRatio := cfg.Context.CoreRatio + cfg.Context.SessionRatio +
		cfg.Context.ShortTermRatio + cfg.Context.LongTermRatio + cfg.Context.ReservedRatio

	if totalRatio < 0.99 || totalRatio > 1.01 {
		t.Errorf("上下文比例总和应为 1.0，实际为 %.2f", totalRatio)
	}
}

func TestValidateConfig(t *testing.T) {
	// 有效配置
	cfg := DefaultMemoryConfig()
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("有效配置验证失败: %v", err)
	}

	// 无效配置：空存储路径
	invalidCfg := DefaultMemoryConfig()
	invalidCfg.Storage.GlobalRoot = ""
	if err := ValidateConfig(invalidCfg); err == nil {
		t.Error("应该检测到无效的存储路径")
	}
}

// ========== Memory 类型测试 ==========

func TestNewMemory(t *testing.T) {
	mem := NewMemory(MemoryTypeCore, ScopeGlobal, CategoryPreference, "测试", "内容")

	if mem.Type != MemoryTypeCore {
		t.Error("类型不匹配")
	}

	if mem.Scope != ScopeGlobal {
		t.Error("作用域不匹配")
	}

	if mem.Status != StatusActive {
		t.Error("状态应为 Active")
	}

	if mem.Importance != 3 {
		t.Error("默认重要性应为 3")
	}
}

func TestMemory_SetTTL(t *testing.T) {
	mem := NewMemory(MemoryTypeShortTerm, ScopeGlobal, CategoryTask, "任务", "内容")

	mem.SetTTL(7 * 24 * time.Hour) // 7天

	if mem.ExpiresAt == nil {
		t.Fatal("ExpiresAt 不应为 nil")
	}

	expectedExpiry := time.Now().Add(7 * 24 * time.Hour)
	diff := mem.ExpiresAt.Sub(expectedExpiry)
	if diff < -time.Second || diff > time.Second {
		t.Error("TTL 设置不正确")
	}
}

func TestMemory_IsExpired(t *testing.T) {
	mem := NewMemory(MemoryTypeShortTerm, ScopeGlobal, CategoryTask, "任务", "内容")

	// 未设置 TTL
	if mem.IsExpired() {
		t.Error("未设置 TTL 时不应过期")
	}

	// 设置过去的时间
	pastTime := time.Now().Add(-time.Hour)
	mem.ExpiresAt = &pastTime
	if !mem.IsExpired() {
		t.Error("过去的时间应该过期")
	}

	// 设置未来的时间
	futureTime := time.Now().Add(time.Hour)
	mem.ExpiresAt = &futureTime
	if mem.IsExpired() {
		t.Error("未来的时间不应过期")
	}
}

// ========== 分类器测试 ==========

func TestMemoryClassifier_Classify(t *testing.T) {
	classifier := NewMemoryClassifier()

	tests := []struct {
		name         string
		input        string
		shouldStore  bool
		expectedType MemoryType
		expectedCat  MemoryCategory
	}{
		{
			name:         "用户偏好",
			input:        "我习惯使用 vim 编辑器",
			shouldStore:  true,
			expectedType: MemoryTypeCore,
			expectedCat:  CategoryPreference,
		},
		{
			name:         "规则",
			input:        "以后不要在代码中添加注释",
			shouldStore:  true,
			expectedType: MemoryTypeCore,
			expectedCat:  CategoryRule,
		},
		{
			name:         "临时任务",
			input:        "今天要完成登录功能",
			shouldStore:  true,
			expectedType: MemoryTypeShortTerm,
			expectedCat:  CategoryTask,
		},
		{
			name:         "普通问题",
			input:        "什么是 Go 语言？",
			shouldStore:  false,
			expectedType: "",
			expectedCat:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.Classify(tt.input)

			if result.ShouldStore != tt.shouldStore {
				t.Errorf("ShouldStore: 期望 %v, 实际 %v", tt.shouldStore, result.ShouldStore)
			}

			if tt.shouldStore {
				if result.MemoryType != tt.expectedType {
					t.Errorf("MemoryType: 期望 %v, 实际 %v", tt.expectedType, result.MemoryType)
				}

				if result.Category != tt.expectedCat {
					t.Errorf("Category: 期望 %v, 实际 %v", tt.expectedCat, result.Category)
				}
			}
		})
	}
}

// ========== 向量测试 ==========

func TestVectorOperations(t *testing.T) {
	// 测试余弦相似度
	a := []float32{1.0, 0.0, 0.0}
	b := []float32{1.0, 0.0, 0.0}
	c := []float32{0.0, 1.0, 0.0}

	sim1 := CosineSimilarity(a, b)
	if sim1 < 0.99 {
		t.Errorf("相同向量的余弦相似度应为 1.0，实际为 %.4f", sim1)
	}

	sim2 := CosineSimilarity(a, c)
	if sim2 > 0.01 {
		t.Errorf("正交向量的余弦相似度应为 0.0，实际为 %.4f", sim2)
	}

	// 测试归一化
	v := []float32{3.0, 4.0}
	normalized := NormalizeVector(v)
	norm := calculateNorm(normalized)
	if norm < 0.99 || norm > 1.01 {
		t.Errorf("归一化向量的范数应为 1.0，实际为 %.4f", norm)
	}
}

// ========== 存储测试 ==========

func TestStorageManager(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "memory-test-*")
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

	// 测试全局路径
	globalRoot := storage.GetGlobalRoot()
	if globalRoot != cfg.Storage.GlobalRoot {
		t.Errorf("全局根目录不匹配")
	}

	// 测试目录创建
	corePath := storage.GetGlobalCorePath()
	if _, err := os.Stat(corePath); os.IsNotExist(err) {
		t.Error("核心记忆目录应该存在")
	}
}

// ========== 索引测试 ==========

func TestSQLiteIndexStore(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "index-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := NewSQLiteIndexStore(dbPath)
	if err != nil {
		t.Fatalf("创建索引存储失败: %v", err)
	}
	defer store.Close()

	// 测试创建索引
	idx := &MemoryIndex{
		ID:          "test-001",
		FilePath:    "/test/path/file.md",
		Type:        MemoryTypeCore,
		Scope:       ScopeGlobal,
		Category:    CategoryPreference,
		Title:       "测试索引",
		ContentHash: "hash123",
		Importance:  3,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		AccessedAt:  time.Now(),
	}

	if err := store.CreateIndex(idx); err != nil {
		t.Fatalf("创建索引失败: %v", err)
	}

	// 测试获取索引
	retrieved, err := store.GetIndex("test-001")
	if err != nil {
		t.Fatalf("获取索引失败: %v", err)
	}

	if retrieved.Title != idx.Title {
		t.Errorf("标题不匹配: 期望 %s, 实际 %s", idx.Title, retrieved.Title)
	}

	// 测试更新访问计数
	if err := store.IncrementAccessCount("test-001"); err != nil {
		t.Fatalf("更新访问计数失败: %v", err)
	}

	updated, _ := store.GetIndex("test-001")
	if updated.AccessCount != 1 {
		t.Errorf("访问计数应为 1，实际为 %d", updated.AccessCount)
	}

	// 测试删除
	if err := store.DeleteIndex("test-001"); err != nil {
		t.Fatalf("删除索引失败: %v", err)
	}

	_, err = store.GetIndex("test-001")
	if err != ErrIndexNotFound {
		t.Error("删除后应该找不到索引")
	}
}

// ========== 向量存储测试 ==========

func TestSQLiteVectorStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vector-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "vectors.db")
	store, err := NewSQLiteVectorStore(dbPath, 3)
	if err != nil {
		t.Fatalf("创建向量存储失败: %v", err)
	}
	defer store.Close()

	// 测试存储向量
	vector := []float32{1.0, 2.0, 3.0}
	if err := store.StoreVector("vec-001", vector); err != nil {
		t.Fatalf("存储向量失败: %v", err)
	}

	// 测试获取向量
	retrieved, err := store.GetVector("vec-001")
	if err != nil {
		t.Fatalf("获取向量失败: %v", err)
	}

	if len(retrieved) != len(vector) {
		t.Errorf("向量长度不匹配")
	}

	for i := range vector {
		if retrieved[i] != vector[i] {
			t.Errorf("向量元素 %d 不匹配: 期望 %f, 实际 %f", i, vector[i], retrieved[i])
		}
	}

	// 测试相似度搜索
	queryVec := []float32{1.0, 2.0, 3.0}
	results, err := store.SearchSimilar(queryVec, 10, 0.5)
	if err != nil {
		t.Fatalf("相似度搜索失败: %v", err)
	}

	if len(results) == 0 {
		t.Error("应该找到至少一个结果")
	}

	if results[0].ID != "vec-001" {
		t.Error("结果 ID 不匹配")
	}

	// 测试删除
	if err := store.DeleteVector("vec-001"); err != nil {
		t.Fatalf("删除向量失败: %v", err)
	}

	_, err = store.GetVector("vec-001")
	if err != ErrVectorNotFound {
		t.Error("删除后应该找不到向量")
	}
}

// ========== Mock Embedding 测试 ==========

func TestMockEmbeddingClient(t *testing.T) {
	client := NewMockEmbeddingClient(128)

	if client.GetDimension() != 128 {
		t.Errorf("维度应为 128")
	}

	// 测试相同文本生成相同向量
	vec1, _ := client.Embed(nil, "Hello, World!")
	vec2, _ := client.Embed(nil, "Hello, World!")

	for i := range vec1 {
		if vec1[i] != vec2[i] {
			t.Error("相同文本应生成相同向量")
			break
		}
	}

	// 测试不同文本生成不同向量
	vec3, _ := client.Embed(nil, "Different text")
	same := true
	for i := range vec1 {
		if vec1[i] != vec3[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("不同文本应生成不同向量")
	}
}

// ========== MarkdownFileStore 测试 ==========

func TestMarkdownFileStore_CreateAndReadMemory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filestore-test-*")
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

	// 创建记忆
	mem := NewMemory(MemoryTypeCore, ScopeGlobal, CategoryPreference, "测试记忆", "这是测试内容")

	err = fileStore.CreateMemory(mem)
	if err != nil {
		t.Fatalf("创建记忆失败: %v", err)
	}

	// 验证文件存在
	if !FileExists(mem.FilePath) {
		t.Error("记忆文件应该存在")
	}

	// 读取记忆
	readMem, err := fileStore.ReadMemory(mem.FilePath)
	if err != nil {
		t.Fatalf("读取记忆失败: %v", err)
	}

	if readMem.Title != mem.Title {
		t.Errorf("标题不匹配: 期望 %s, 实际 %s", mem.Title, readMem.Title)
	}

	// 内容可能有尾部换行符的差异，使用 TrimSpace 比较
	if strings.TrimSpace(readMem.Content) != strings.TrimSpace(mem.Content) {
		t.Errorf("内容不匹配: 期望 %q, 实际 %q", mem.Content, readMem.Content)
	}
}

func TestMarkdownFileStore_UpdateMemory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filestore-update-test-*")
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

	// 创建记忆
	mem := NewMemory(MemoryTypeCore, ScopeGlobal, CategoryRule, "测试规则", "原始内容")
	if err := fileStore.CreateMemory(mem); err != nil {
		t.Fatalf("创建记忆失败: %v", err)
	}

	// 更新记忆
	mem.Content = "更新后的内容"
	if err := fileStore.UpdateMemory(mem); err != nil {
		t.Fatalf("更新记忆失败: %v", err)
	}

	// 验证更新
	readMem, err := fileStore.ReadMemory(mem.FilePath)
	if err != nil {
		t.Fatalf("读取记忆失败: %v", err)
	}

	// 内容可能有尾部换行符的差异，使用 TrimSpace 比较
	if strings.TrimSpace(readMem.Content) != strings.TrimSpace("更新后的内容") {
		t.Errorf("内容未更新: %q", readMem.Content)
	}
}

func TestMarkdownFileStore_DeleteMemory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filestore-delete-test-*")
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

	// 创建记忆
	mem := NewMemory(MemoryTypeCore, ScopeGlobal, CategoryRule, "待删除", "内容")
	if err := fileStore.CreateMemory(mem); err != nil {
		t.Fatalf("创建记忆失败: %v", err)
	}

	// 删除记忆
	if err := fileStore.DeleteMemory(mem.FilePath); err != nil {
		t.Fatalf("删除记忆失败: %v", err)
	}

	// 验证文件不存在
	if FileExists(mem.FilePath) {
		t.Error("删除后文件不应存在")
	}
}

// ========== SessionManager 测试 ==========

func TestSessionManager_CreateSession(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session-test-*")
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

	// 创建会话
	sess, err := sessionMgr.CreateSession()
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	if sess.ID == "" {
		t.Error("会话 ID 不应为空")
	}

	if sess.Status != StatusActive {
		t.Error("会话状态应为 Active")
	}

	// 验证当前会话
	current := sessionMgr.GetCurrentSession()
	if current == nil {
		t.Error("当前会话不应为 nil")
	}

	if current.ID != sess.ID {
		t.Error("当前会话 ID 不匹配")
	}
}

func TestSessionManager_AddMessage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session-msg-test-*")
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

	// 创建会话并添加消息
	_, err = sessionMgr.CreateSession()
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	// 添加用户消息
	err = sessionMgr.AddMessage("user", "你好", 10)
	if err != nil {
		t.Fatalf("添加用户消息失败: %v", err)
	}

	// 添加助手消息
	err = sessionMgr.AddMessage("assistant", "你好！有什么可以帮助你的？", 20)
	if err != nil {
		t.Fatalf("添加助手消息失败: %v", err)
	}

	// 验证消息
	messages := sessionMgr.GetMessages()
	if len(messages) != 2 {
		t.Errorf("消息数量应为 2，实际为 %d", len(messages))
	}

	if messages[0].Role != "user" {
		t.Errorf("第一条消息角色应为 user，实际为 %s", messages[0].Role)
	}
}

func TestSessionManager_TokenUsage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session-token-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultMemoryConfig()
	cfg.Storage.GlobalRoot = filepath.Join(tmpDir, "global")
	cfg.Session.MaxTokens = 1000

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

	_, _ = sessionMgr.CreateSession()
	_ = sessionMgr.AddMessage("user", "消息1", 100)
	_ = sessionMgr.AddMessage("assistant", "消息2", 200)

	current, max, ratio := sessionMgr.GetTokenUsage()

	if current != 300 {
		t.Errorf("当前 Token 数应为 300，实际为 %d", current)
	}

	if max != 1000 {
		t.Errorf("最大 Token 数应为 1000，实际为 %d", max)
	}

	expectedRatio := 0.3
	if ratio < expectedRatio-0.01 || ratio > expectedRatio+0.01 {
		t.Errorf("Token 使用比例应约为 0.3，实际为 %.2f", ratio)
	}
}

// ========== ShortTermMemoryManager 测试 ==========

func TestShortTermMemoryManager_Add(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shortterm-test-*")
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

	// 添加任务记忆
	mem, err := shortTermMgr.AddTask("测试任务", "任务内容", 3)
	if err != nil {
		t.Fatalf("添加任务记忆失败: %v", err)
	}

	if mem.Type != MemoryTypeShortTerm {
		t.Error("记忆类型应为 ShortTerm")
	}

	if mem.Category != CategoryTask {
		t.Error("分类应为 Task")
	}

	if mem.ExpiresAt == nil {
		t.Error("TTL 应该被设置")
	}
}

func TestShortTermMemoryManager_AddNote(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shortterm-note-test-*")
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

	// 添加笔记
	mem, err := shortTermMgr.AddNote("测试笔记", "笔记内容", 7)
	if err != nil {
		t.Fatalf("添加笔记失败: %v", err)
	}

	if mem.Category != CategoryNote {
		t.Error("分类应为 Note")
	}
}

func TestShortTermMemoryManager_LoadActive(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shortterm-active-test-*")
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

	// 添加多个记忆
	_, _ = shortTermMgr.AddTask("任务1", "内容1", 3)
	_, _ = shortTermMgr.AddNote("笔记1", "内容2", 7)

	// 加载活跃记忆
	active, err := shortTermMgr.LoadActive()
	if err != nil {
		t.Fatalf("加载活跃记忆失败: %v", err)
	}

	if len(active) != 2 {
		t.Errorf("活跃记忆数量应为 2，实际为 %d", len(active))
	}
}

// ========== 分类器额外测试 ==========

func TestMemoryClassifier_ClassifyComplex(t *testing.T) {
	classifier := NewMemoryClassifier()

	tests := []struct {
		name        string
		input       string
		shouldStore bool
		reason      string
	}{
		{
			name:        "短文本",
			input:       "OK",
			shouldStore: false,
			reason:      "文本过短",
		},
		{
			name:        "我希望你的风格",
			input:       "我希望你写代码时风格简洁一些",
			shouldStore: true,
			reason:      "应检测为用户偏好",
		},
		{
			name:        "项目知识",
			input:       "这个项目使用 Go 语言开发，使用 SQLite 作为数据库",
			shouldStore: true,
			reason:      "应检测为项目知识",
		},
		{
			name:        "时间相关任务",
			input:       "今天下午要完成登录功能的开发",
			shouldStore: true,
			reason:      "应检测为临时任务",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.Classify(tt.input)
			if result.ShouldStore != tt.shouldStore {
				t.Errorf("%s: ShouldStore 不匹配，期望 %v，实际 %v (原因: %s)",
					tt.reason, tt.shouldStore, result.ShouldStore, result.Reason)
			}
		})
	}
}

func TestMemoryClassifier_DetermineScope(t *testing.T) {
	classifier := NewMemoryClassifier()

	// 有项目上下文
	scope1 := classifier.DetermineScope("这个项目的架构设计", true)
	if scope1 != ScopeProject {
		t.Error("应检测为项目作用域")
	}

	// 明确全局
	scope2 := classifier.DetermineScope("我在所有项目中都习惯用 vim", true)
	if scope2 != ScopeGlobal {
		t.Error("应检测为全局作用域")
	}

	// 无项目上下文
	scope3 := classifier.DetermineScope("项目相关内容", false)
	if scope3 != ScopeGlobal {
		t.Error("无项目上下文时应为全局作用域")
	}
}

func TestMemoryClassifier_ExtractImportance(t *testing.T) {
	classifier := NewMemoryClassifier()

	// 高重要性
	importance1 := classifier.ExtractImportance("这个非常重要，必须记住")
	if importance1 != 5 {
		t.Errorf("高重要性应为 5，实际为 %d", importance1)
	}

	// 低重要性
	importance2 := classifier.ExtractImportance("这个可能有用")
	if importance2 != 2 {
		t.Errorf("低重要性应为 2，实际为 %d", importance2)
	}

	// 默认重要性
	importance3 := classifier.ExtractImportance("普通内容")
	if importance3 != 3 {
		t.Errorf("默认重要性应为 3，实际为 %d", importance3)
	}
}

// ========== 上下文管理测试 ==========

func TestContextBudget(t *testing.T) {
	cfg := DefaultMemoryConfig()
	budget := &ContextBudget{
		Total:     cfg.Context.TotalBudget,
		Core:      int(float64(cfg.Context.TotalBudget) * cfg.Context.CoreRatio),
		Session:   int(float64(cfg.Context.TotalBudget) * cfg.Context.SessionRatio),
		ShortTerm: int(float64(cfg.Context.TotalBudget) * cfg.Context.ShortTermRatio),
		LongTerm:  int(float64(cfg.Context.TotalBudget) * cfg.Context.LongTermRatio),
		Reserved:  int(float64(cfg.Context.TotalBudget) * cfg.Context.ReservedRatio),
	}

	// 验证各部分之和约等于总预算
	sum := budget.Core + budget.Session + budget.ShortTerm + budget.LongTerm + budget.Reserved
	if sum < budget.Total-100 || sum > budget.Total+100 {
		t.Errorf("预算分配之和应约等于总预算: 总=%d, 和=%d", budget.Total, sum)
	}
}

func TestBuiltContext_IsOverBudget(t *testing.T) {
	ctx := &BuiltContext{
		TotalTokens: 5000,
		Budget: &ContextBudget{
			Total: 10000,
		},
	}

	if ctx.IsOverBudget() {
		t.Error("未超预算时不应返回 true")
	}

	ctx.TotalTokens = 15000
	if !ctx.IsOverBudget() {
		t.Error("超预算时应返回 true")
	}
}

func TestBuiltContext_GetRemainingBudget(t *testing.T) {
	ctx := &BuiltContext{
		TotalTokens: 3000,
		Budget: &ContextBudget{
			Total: 10000,
		},
	}

	remaining := ctx.GetRemainingBudget()
	if remaining != 7000 {
		t.Errorf("剩余预算应为 7000，实际为 %d", remaining)
	}
}

// ========== 存储路径测试 ==========

func TestStorageManager_Paths(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-path-test-*")
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

	// 验证全局路径
	if !strings.HasSuffix(storage.GetGlobalCorePath(), "/core") {
		t.Error("核心记忆路径应以 /core 结尾")
	}

	if !strings.HasSuffix(storage.GetGlobalSessionsPath(), "/sessions") {
		t.Error("会话路径应以 /sessions 结尾")
	}

	if !strings.HasSuffix(storage.GetGlobalShortTermPath(), "/short_term") {
		t.Error("短期记忆路径应以 /short_term 结尾")
	}

	if !strings.HasSuffix(storage.GetGlobalLongTermPath(), "/long_term") {
		t.Error("长期记忆路径应以 /long_term 结尾")
	}
}

func TestStorageManager_GenerateMemoryFilePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-filepath-test-*")
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

	mem := NewMemory(MemoryTypeCore, ScopeGlobal, CategoryPreference, "测试标题", "内容")
	filePath := storage.GenerateMemoryFilePath(mem)

	if !strings.Contains(filePath, "/core/") {
		t.Error("核心记忆文件路径应包含 /core/")
	}

	if !strings.HasSuffix(filePath, ".md") {
		t.Error("记忆文件路径应以 .md 结尾")
	}
}

// ========== 索引转换测试 ==========

func TestMemoryToIndex(t *testing.T) {
	mem := NewMemory(MemoryTypeCore, ScopeGlobal, CategoryPreference, "测试", "内容")
	mem.ID = "test-id"
	mem.FilePath = "/test/path.md"
	mem.Tags = []string{"tag1", "tag2"}

	idx := MemoryToIndex(mem)

	if idx.ID != mem.ID {
		t.Error("ID 不匹配")
	}

	if idx.FilePath != mem.FilePath {
		t.Error("FilePath 不匹配")
	}

	if idx.Type != mem.Type {
		t.Error("Type 不匹配")
	}

	if idx.Tags != "tag1,tag2" {
		t.Errorf("Tags 应为 'tag1,tag2'，实际为 %s", idx.Tags)
	}
}

func TestIndexToMemoryPartial(t *testing.T) {
	idx := &MemoryIndex{
		ID:       "test-id",
		FilePath: "/test/path.md",
		Type:     MemoryTypeCore,
		Scope:    ScopeGlobal,
		Category: CategoryPreference,
		Title:    "测试",
		Tags:     "tag1,tag2",
	}

	mem := IndexToMemoryPartial(idx)

	if mem.ID != idx.ID {
		t.Error("ID 不匹配")
	}

	if len(mem.Tags) != 2 {
		t.Errorf("Tags 应有 2 个元素，实际有 %d 个", len(mem.Tags))
	}
}

// ========== Frontmatter 额外测试 ==========

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		content  string
		expected string
	}{
		{"# 标题\n内容", "标题"},
		{"普通文本", "普通文本"},
		{"", "无标题"},
		{"\n\n\n", "无标题"},
		{"一个非常非常非常非常非常非常非常非常非常非常非常长的标题", "一个非常非常非常非常非常非常非常非常非常非常非常长的标题..."},
	}

	for _, tt := range tests {
		title := ExtractTitle(tt.content)
		// 对于长标题，只检查前缀
		if len(tt.expected) > 50 {
			if !strings.HasPrefix(title, tt.expected[:50]) {
				t.Errorf("标题提取失败: 内容=%s, 期望前缀=%s, 实际=%s", tt.content, tt.expected[:50], title)
			}
		} else {
			if title != tt.expected {
				t.Errorf("标题提取失败: 内容=%s, 期望=%s, 实际=%s", tt.content, tt.expected, title)
			}
		}
	}
}

func TestExtractKeywords(t *testing.T) {
	content := "Go 语言是一种静态类型的编程语言，Go 非常适合并发编程"
	keywords := ExtractKeywords(content, 3)

	if len(keywords) == 0 {
		t.Error("应该提取到关键词")
	}

	if len(keywords) > 3 {
		t.Errorf("关键词数量不应超过 3，实际为 %d", len(keywords))
	}
}

// ========== 向量操作额外测试 ==========

func TestEuclideanDistance(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{3, 4, 0}

	dist := EuclideanDistance(a, b)
	if dist < 4.99 || dist > 5.01 {
		t.Errorf("欧氏距离应为 5，实际为 %f", dist)
	}
}

func TestVectorToBlob(t *testing.T) {
	original := []float32{1.0, 2.0, 3.0, 4.0}
	blob := vectorToBlob(original)
	recovered := blobToVector(blob)

	if len(recovered) != len(original) {
		t.Fatalf("向量长度不匹配: %d vs %d", len(recovered), len(original))
	}

	for i := range original {
		if recovered[i] != original[i] {
			t.Errorf("向量元素 %d 不匹配: %f vs %f", i, recovered[i], original[i])
		}
	}
}

// ========== 检索选项测试 ==========

func TestDefaultRetrievalOptions(t *testing.T) {
	opts := DefaultRetrievalOptions()

	if opts.TopK != 5 {
		t.Errorf("默认 TopK 应为 5，实际为 %d", opts.TopK)
	}

	if !opts.UseVector {
		t.Error("默认应启用向量检索")
	}

	if !opts.UseKeyword {
		t.Error("默认应启用关键词检索")
	}

	if !opts.UseTimeWeight {
		t.Error("默认应启用时间权重")
	}

	if opts.MinSimilarity != 0.6 {
		t.Errorf("默认最小相似度应为 0.6，实际为 %f", opts.MinSimilarity)
	}
}

// ========== 辅助函数测试 ==========

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal_name", "normal_name"},
		{"name with spaces", "name_with_spaces"},
		{"name/with/slashes", "name_with_slashes"},
		{"name:with:colons", "name_with_colons"},
		{"  trim__extra__underscores  ", "trim_extra_underscores"},
	}

	for _, tt := range tests {
		result := sanitizeFileName(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeFileName(%s) = %s, 期望 %s", tt.input, result, tt.expected)
		}
	}
}

func TestTruncateContent(t *testing.T) {
	short := "短文本"
	long := strings.Repeat("长文本", 100)

	// 短文本不截断
	result1 := truncateContent(short, 100)
	if result1 != short {
		t.Error("短文本不应被截断")
	}

	// 长文本截断
	result2 := truncateContent(long, 100)
	if len(result2) > 110 { // 允许一些余量
		t.Errorf("长文本应被截断到约 100 字符，实际长度 %d", len(result2))
	}
}

// ========== 时间相关测试 ==========

func TestMemory_IsExpired_EdgeCases(t *testing.T) {
	mem := NewMemory(MemoryTypeShortTerm, ScopeGlobal, CategoryTask, "测试", "内容")

	// 刚好过期
	exactNow := time.Now()
	mem.ExpiresAt = &exactNow
	// 休眠一小段时间确保过期
	time.Sleep(time.Millisecond * 10)
	if !mem.IsExpired() {
		t.Error("刚过期的时间应该返回过期")
	}
}

// ========== Mock 测试 ==========

func TestMockEmbeddingClient_EmbedBatch(t *testing.T) {
	client := NewMockEmbeddingClient(128)

	texts := []string{"文本1", "文本2", "文本3"}
	vectors, err := client.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("批量嵌入失败: %v", err)
	}

	if len(vectors) != len(texts) {
		t.Errorf("向量数量应为 %d，实际为 %d", len(texts), len(vectors))
	}

	for i, vec := range vectors {
		if len(vec) != 128 {
			t.Errorf("向量 %d 维度应为 128，实际为 %d", i, len(vec))
		}
	}
}
