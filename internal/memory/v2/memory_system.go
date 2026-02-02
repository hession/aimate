// Package v2 提供 AIMate 记忆系统 v2 版本
// 实现四层记忆架构：核心记忆、会话记忆、短期记忆、长期记忆
package v2

import (
	"context"
	"fmt"
	"sync"
)

// MemorySystem 记忆系统主结构
// 整合所有记忆组件，提供统一的访问接口
type MemorySystem struct {
	// 配置
	config    *MemoryConfig
	configMgr *ConfigManager

	// 存储层
	storage   *StorageManager
	fileStore *MarkdownFileStore

	// 索引层
	index  *SQLiteIndexStore
	vector *SQLiteVectorStore

	// 记忆管理器
	coreMgr      *CoreMemoryManager
	sessionMgr   *SessionManager
	shortTermMgr *ShortTermMemoryManager
	longTermMgr  *LongTermMemoryManager

	// 辅助组件
	classifier     *MemoryClassifier
	embedding      *EmbeddingManager
	retriever      *HybridRetriever
	contextBuilder *ContextBuilder
	lifecycle      *LifecycleManager
	syncer         *IndexSyncer
	trimmer        *SessionTrimmer

	// 状态
	initialized bool
	mu          sync.RWMutex
}

// NewMemorySystem 创建记忆系统
func NewMemorySystem() (*MemorySystem, error) {
	return &MemorySystem{}, nil
}

// Initialize 初始化记忆系统
func (ms *MemorySystem) Initialize(apiKey string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.initialized {
		return nil
	}

	// 1. 加载配置
	configMgr, err := NewConfigManager()
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	ms.configMgr = configMgr
	ms.config = configMgr.GetGlobalConfig()

	// 2. 初始化存储层
	storage, err := NewStorageManager(ms.config)
	if err != nil {
		return fmt.Errorf("初始化存储失败: %w", err)
	}
	ms.storage = storage

	// 3. 初始化文件存储
	ms.fileStore = NewMarkdownFileStore(storage)

	// 4. 初始化索引
	indexPath := storage.GetGlobalIndexDBPath()
	index, err := NewSQLiteIndexStore(indexPath)
	if err != nil {
		return fmt.Errorf("初始化索引失败: %w", err)
	}
	ms.index = index

	// 5. 初始化向量存储
	vectorPath := storage.GetGlobalRoot() + "/vectors.db"
	vector, err := NewSQLiteVectorStore(vectorPath, ms.config.Embedding.Dimension)
	if err != nil {
		return fmt.Errorf("初始化向量存储失败: %w", err)
	}
	ms.vector = vector

	// 6. 初始化 Embedding（如果有 API Key）
	if apiKey != "" && ms.config.Embedding.Enabled {
		embeddingClient := NewEmbeddingClient(&ms.config.Embedding, apiKey)
		ms.embedding = NewEmbeddingManager(embeddingClient, vector, &ms.config.Embedding)
	}

	// 7. 初始化各层记忆管理器
	ms.coreMgr = NewCoreMemoryManager(storage, ms.fileStore, ms.index, ms.config)
	ms.sessionMgr = NewSessionManager(storage, ms.fileStore, ms.index, ms.config)
	ms.shortTermMgr = NewShortTermMemoryManager(storage, ms.fileStore, ms.index, ms.config)
	ms.longTermMgr = NewLongTermMemoryManager(storage, ms.fileStore, ms.index, ms.vector, ms.config)

	// 8. 初始化辅助组件
	ms.classifier = NewMemoryClassifier()

	ms.syncer = NewIndexSyncer(storage, ms.fileStore, ms.index, ms.vector)

	ms.retriever = NewHybridRetriever(ms.index, ms.vector, ms.fileStore, ms.embedding, &ms.config.Retrieval)

	ms.contextBuilder = NewContextBuilder(
		ms.coreMgr,
		ms.sessionMgr,
		ms.shortTermMgr,
		ms.longTermMgr,
		ms.retriever,
		ms.config,
	)

	ms.lifecycle = NewLifecycleManager(ms.shortTermMgr, ms.longTermMgr, ms.syncer, ms.config)

	// 创建默认摘要函数
	summarizeFunc := DefaultSummarizeFunc
	ms.trimmer = NewSessionTrimmer(ms.sessionMgr, ms.shortTermMgr, ms.config, summarizeFunc)

	// 9. 初始化默认核心记忆
	if err := ms.coreMgr.InitDefaultMemories(); err != nil {
		// 非致命错误，记录日志继续
		fmt.Printf("初始化默认核心记忆失败: %v\n", err)
	}

	// 10. 启动后台维护任务
	if ms.config.Maintenance.Enabled {
		ms.lifecycle.Start()
	}

	ms.initialized = true
	return nil
}

// SetProject 设置当前项目
func (ms *MemorySystem) SetProject(projectPath string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if err := ms.storage.SetCurrentProject(projectPath); err != nil {
		return err
	}

	// 确保项目目录存在
	ms.shortTermMgr.EnsureDirectories()
	ms.longTermMgr.EnsureDirectories()

	return nil
}

// Close 关闭记忆系统
func (ms *MemorySystem) Close() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// 停止后台任务
	if ms.lifecycle != nil {
		ms.lifecycle.Stop()
	}

	// 关闭索引
	if ms.index != nil {
		ms.index.Close()
	}

	// 关闭向量存储
	if ms.vector != nil {
		ms.vector.Close()
	}

	ms.initialized = false
	return nil
}

// ========== 便捷访问方法 ==========

// Core 获取核心记忆管理器
func (ms *MemorySystem) Core() *CoreMemoryManager {
	return ms.coreMgr
}

// Session 获取会话记忆管理器
func (ms *MemorySystem) Session() *SessionManager {
	return ms.sessionMgr
}

// ShortTerm 获取短期记忆管理器
func (ms *MemorySystem) ShortTerm() *ShortTermMemoryManager {
	return ms.shortTermMgr
}

// LongTerm 获取长期记忆管理器
func (ms *MemorySystem) LongTerm() *LongTermMemoryManager {
	return ms.longTermMgr
}

// Classifier 获取分类器
func (ms *MemorySystem) Classifier() *MemoryClassifier {
	return ms.classifier
}

// Retriever 获取检索器
func (ms *MemorySystem) Retriever() *HybridRetriever {
	return ms.retriever
}

// Context 获取上下文构建器
func (ms *MemorySystem) Context() *ContextBuilder {
	return ms.contextBuilder
}

// Trimmer 获取会话裁剪器
func (ms *MemorySystem) Trimmer() *SessionTrimmer {
	return ms.trimmer
}

// ========== 核心操作方法 ==========

// ProcessUserInput 处理用户输入（自动识别并存储记忆）
func (ms *MemorySystem) ProcessUserInput(ctx context.Context, userMessage string) (*ClassificationResult, error) {
	// 分类
	result := ms.classifier.Classify(userMessage)

	if !result.ShouldStore {
		return result, nil
	}

	// 根据分类结果存储
	switch result.MemoryType {
	case MemoryTypeCore:
		_, err := ms.coreMgr.Add(result.Category, result.Title, userMessage)
		if err != nil {
			return result, err
		}

	case MemoryTypeShortTerm:
		_, err := ms.shortTermMgr.Add(result.Category, result.Scope, result.Title, userMessage, result.TTLDays)
		if err != nil {
			return result, err
		}

	case MemoryTypeLongTerm:
		_, err := ms.longTermMgr.Add(result.Category, result.Scope, result.Title, userMessage, result.Tags)
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

// AddConversation 添加对话到会话记忆
func (ms *MemorySystem) AddConversation(role, content string, tokenCount int) error {
	return ms.sessionMgr.AddMessage(role, content, tokenCount)
}

// BuildContext 构建上下文
func (ms *MemorySystem) BuildContext(ctx context.Context, query string) (*BuiltContext, error) {
	return ms.contextBuilder.BuildContext(ctx, query)
}

// Search 搜索记忆
func (ms *MemorySystem) Search(ctx context.Context, query string, topK int) ([]*Memory, error) {
	return ms.retriever.QuickSearch(ctx, query, topK)
}

// NewSession 创建新会话
func (ms *MemorySystem) NewSession() (*Session, error) {
	return ms.sessionMgr.CreateSession()
}

// CheckSessionThreshold 检查会话阈值
func (ms *MemorySystem) CheckSessionThreshold() []string {
	return ms.sessionMgr.CheckThreshold()
}

// TrimSessionIfNeeded 如果需要则裁剪会话
func (ms *MemorySystem) TrimSessionIfNeeded() (*TrimResult, error) {
	return ms.trimmer.TrimIfNeeded()
}

// ========== 状态和统计 ==========

// GetStats 获取记忆系统统计
func (ms *MemorySystem) GetStats() *MemorySystemStats {
	stats := &MemorySystemStats{}

	// 核心记忆
	coreTokens, _ := ms.coreMgr.GetTotalTokens()
	stats.CoreTokens = coreTokens

	// 会话
	sessionStats := ms.sessionMgr.GetSessionStats()
	stats.SessionTokens = sessionStats.CurrentTokens
	stats.SessionMessages = sessionStats.CurrentMessages
	stats.SessionUsageRatio = sessionStats.UsageRatio

	// 短期记忆
	shortTermStats, _ := ms.shortTermMgr.GetStats()
	if shortTermStats != nil {
		stats.ShortTermCount = shortTermStats.Total
		stats.ShortTermExpired = shortTermStats.ExpiredCount
	}

	// 长期记忆
	longTermStats, _ := ms.longTermMgr.GetStats()
	if longTermStats != nil {
		stats.LongTermCount = longTermStats.Total
	}

	// 索引
	indexStats, _ := ms.index.GetIndexStats()
	if indexStats != nil {
		stats.IndexedCount = indexStats.TotalCount
	}

	// 向量
	vectorStats, _ := ms.vector.GetVectorStats()
	if vectorStats != nil {
		stats.VectorCount = vectorStats.TotalVectors
	}

	return stats
}

// MemorySystemStats 记忆系统统计
type MemorySystemStats struct {
	CoreTokens        int     `json:"core_tokens"`
	SessionTokens     int     `json:"session_tokens"`
	SessionMessages   int     `json:"session_messages"`
	SessionUsageRatio float64 `json:"session_usage_ratio"`
	ShortTermCount    int     `json:"short_term_count"`
	ShortTermExpired  int     `json:"short_term_expired"`
	LongTermCount     int     `json:"long_term_count"`
	IndexedCount      int     `json:"indexed_count"`
	VectorCount       int     `json:"vector_count"`
}

// CheckWarnings 检查警告
func (ms *MemorySystem) CheckWarnings() []ContextWarning {
	return ms.contextBuilder.CheckContextWarnings()
}

// SyncIndex 同步索引
func (ms *MemorySystem) SyncIndex() (*SyncResult, error) {
	return ms.syncer.SyncAll()
}

// Reindex 重建索引
func (ms *MemorySystem) Reindex() (*SyncResult, error) {
	return ms.syncer.Reindex()
}

// RunMaintenance 运行维护任务
func (ms *MemorySystem) RunMaintenance(ctx context.Context) *MaintenanceResult {
	return ms.lifecycle.RunMaintenance(ctx)
}

// IsInitialized 检查是否已初始化
func (ms *MemorySystem) IsInitialized() bool {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.initialized
}

// GetConfig 获取配置
func (ms *MemorySystem) GetConfig() *MemoryConfig {
	return ms.config
}
