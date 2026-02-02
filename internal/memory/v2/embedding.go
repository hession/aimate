// Package v2 提供 Embedding API 集成功能
package v2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// EmbeddingClient Embedding API 客户端接口
type EmbeddingClient interface {
	// Embed 生成单个文本的向量
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch 批量生成向量
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// GetDimension 获取向量维度
	GetDimension() int
}

// DeepSeekEmbeddingClient DeepSeek Embedding 客户端
type DeepSeekEmbeddingClient struct {
	baseURL    string
	apiKey     string
	model      string
	dimension  int
	httpClient *http.Client
	maxRetries int
}

// NewDeepSeekEmbeddingClient 创建 DeepSeek Embedding 客户端
func NewDeepSeekEmbeddingClient(config *EmbeddingConfig, apiKey string) *DeepSeekEmbeddingClient {
	return &DeepSeekEmbeddingClient{
		baseURL:   config.BaseURL,
		apiKey:    apiKey,
		model:     config.Model,
		dimension: config.Dimension,
		httpClient: &http.Client{
			Timeout: time.Duration(config.TimeoutSec) * time.Second,
		},
		maxRetries: config.MaxRetries,
	}
}

// EmbeddingRequest Embedding API 请求
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbeddingResponse Embedding API 响应
type EmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// Embed 生成单个文本的向量
func (c *DeepSeekEmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	vectors, err := c.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("embedding 返回结果为空")
	}
	return vectors[0], nil
}

// EmbedBatch 批量生成向量
func (c *DeepSeekEmbeddingClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	var lastErr error
	for retry := 0; retry <= c.maxRetries; retry++ {
		vectors, err := c.doEmbed(ctx, texts)
		if err == nil {
			return vectors, nil
		}
		lastErr = err

		// 指数退避
		if retry < c.maxRetries {
			time.Sleep(time.Duration(1<<retry) * time.Second)
		}
	}

	return nil, fmt.Errorf("embedding 请求失败（已重试 %d 次）: %w", c.maxRetries, lastErr)
}

// doEmbed 执行 embedding 请求
func (c *DeepSeekEmbeddingClient) doEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := EmbeddingRequest{
		Model: c.model,
		Input: texts,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	url := c.baseURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 返回错误状态码 %d: %s", resp.StatusCode, string(body))
	}

	var embResp EmbeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 按索引排序结果
	vectors := make([][]float32, len(texts))
	for _, data := range embResp.Data {
		if data.Index < len(vectors) {
			vectors[data.Index] = data.Embedding
		}
	}

	return vectors, nil
}

// GetDimension 获取向量维度
func (c *DeepSeekEmbeddingClient) GetDimension() int {
	return c.dimension
}

// OpenAIEmbeddingClient OpenAI Embedding 客户端
type OpenAIEmbeddingClient struct {
	baseURL    string
	apiKey     string
	model      string
	dimension  int
	httpClient *http.Client
	maxRetries int
}

// NewOpenAIEmbeddingClient 创建 OpenAI Embedding 客户端
func NewOpenAIEmbeddingClient(config *EmbeddingConfig, apiKey string) *OpenAIEmbeddingClient {
	return &OpenAIEmbeddingClient{
		baseURL:   config.BaseURL,
		apiKey:    apiKey,
		model:     config.Model,
		dimension: config.Dimension,
		httpClient: &http.Client{
			Timeout: time.Duration(config.TimeoutSec) * time.Second,
		},
		maxRetries: config.MaxRetries,
	}
}

// Embed 生成单个文本的向量
func (c *OpenAIEmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	vectors, err := c.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("embedding 返回结果为空")
	}
	return vectors[0], nil
}

// EmbedBatch 批量生成向量
func (c *OpenAIEmbeddingClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	var lastErr error
	for retry := 0; retry <= c.maxRetries; retry++ {
		vectors, err := c.doEmbed(ctx, texts)
		if err == nil {
			return vectors, nil
		}
		lastErr = err

		if retry < c.maxRetries {
			time.Sleep(time.Duration(1<<retry) * time.Second)
		}
	}

	return nil, fmt.Errorf("embedding 请求失败（已重试 %d 次）: %w", c.maxRetries, lastErr)
}

// doEmbed 执行 embedding 请求
func (c *OpenAIEmbeddingClient) doEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := EmbeddingRequest{
		Model: c.model,
		Input: texts,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	url := c.baseURL + "/v1/embeddings"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 返回错误状态码 %d: %s", resp.StatusCode, string(body))
	}

	var embResp EmbeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	vectors := make([][]float32, len(texts))
	for _, data := range embResp.Data {
		if data.Index < len(vectors) {
			vectors[data.Index] = data.Embedding
		}
	}

	return vectors, nil
}

// GetDimension 获取向量维度
func (c *OpenAIEmbeddingClient) GetDimension() int {
	return c.dimension
}

// EmbeddingManager Embedding 管理器
// 负责管理 embedding 的生成、缓存和队列
type EmbeddingManager struct {
	client      EmbeddingClient
	vectorStore VectorStore
	config      *EmbeddingConfig

	// 离线队列
	offlineQueue     []EmbeddingTask
	offlineQueueLock sync.Mutex

	// 缓存（内存缓存）
	cache     map[string][]float32
	cacheLock sync.RWMutex
}

// EmbeddingTask 离线 embedding 任务
type EmbeddingTask struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// NewEmbeddingManager 创建 Embedding 管理器
func NewEmbeddingManager(client EmbeddingClient, vectorStore VectorStore, config *EmbeddingConfig) *EmbeddingManager {
	return &EmbeddingManager{
		client:       client,
		vectorStore:  vectorStore,
		config:       config,
		offlineQueue: []EmbeddingTask{},
		cache:        make(map[string][]float32),
	}
}

// EmbedAndStore 生成 embedding 并存储
func (m *EmbeddingManager) EmbedAndStore(ctx context.Context, id, text string) error {
	// 检查缓存
	m.cacheLock.RLock()
	if vec, ok := m.cache[id]; ok {
		m.cacheLock.RUnlock()
		return m.vectorStore.StoreVector(id, vec)
	}
	m.cacheLock.RUnlock()

	// 生成 embedding
	vec, err := m.client.Embed(ctx, text)
	if err != nil {
		// 加入离线队列
		m.addToOfflineQueue(id, text)
		return fmt.Errorf("生成 embedding 失败，已加入离线队列: %w", err)
	}

	// 存储到向量数据库
	if err := m.vectorStore.StoreVector(id, vec); err != nil {
		return fmt.Errorf("存储向量失败: %w", err)
	}

	// 更新缓存
	m.cacheLock.Lock()
	m.cache[id] = vec
	m.cacheLock.Unlock()

	return nil
}

// EmbedBatchAndStore 批量生成 embedding 并存储
func (m *EmbeddingManager) EmbedBatchAndStore(ctx context.Context, items map[string]string) error {
	if len(items) == 0 {
		return nil
	}

	// 准备批量数据
	ids := make([]string, 0, len(items))
	texts := make([]string, 0, len(items))
	for id, text := range items {
		ids = append(ids, id)
		texts = append(texts, text)
	}

	// 分批处理
	batchSize := m.config.BatchSize
	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batchTexts := texts[i:end]
		batchIDs := ids[i:end]

		// 生成 embedding
		vectors, err := m.client.EmbedBatch(ctx, batchTexts)
		if err != nil {
			// 将失败的加入离线队列
			for j, id := range batchIDs {
				m.addToOfflineQueue(id, batchTexts[j])
			}
			continue
		}

		// 存储向量
		batchVectors := make(map[string][]float32)
		for j, id := range batchIDs {
			if j < len(vectors) && vectors[j] != nil {
				batchVectors[id] = vectors[j]
			}
		}

		if err := m.vectorStore.BatchStoreVectors(batchVectors); err != nil {
			return fmt.Errorf("批量存储向量失败: %w", err)
		}

		// 更新缓存
		m.cacheLock.Lock()
		for id, vec := range batchVectors {
			m.cache[id] = vec
		}
		m.cacheLock.Unlock()
	}

	return nil
}

// SearchSimilar 搜索相似向量
func (m *EmbeddingManager) SearchSimilar(ctx context.Context, text string, topK int) ([]*VectorSearchResult, error) {
	// 生成查询向量
	queryVec, err := m.client.Embed(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("生成查询向量失败: %w", err)
	}

	// 搜索相似向量
	return m.vectorStore.SearchSimilar(queryVec, topK, m.config.MinSimilarity())
}

// MinSimilarity 获取最小相似度配置（扩展 EmbeddingConfig）
func (c *EmbeddingConfig) MinSimilarity() float64 {
	return 0.6 // 默认值，可以从配置中读取
}

// addToOfflineQueue 添加到离线队列
func (m *EmbeddingManager) addToOfflineQueue(id, text string) {
	m.offlineQueueLock.Lock()
	defer m.offlineQueueLock.Unlock()

	m.offlineQueue = append(m.offlineQueue, EmbeddingTask{
		ID:        id,
		Text:      text,
		CreatedAt: time.Now(),
	})
}

// ProcessOfflineQueue 处理离线队列
func (m *EmbeddingManager) ProcessOfflineQueue(ctx context.Context) (int, error) {
	m.offlineQueueLock.Lock()
	queue := m.offlineQueue
	m.offlineQueue = []EmbeddingTask{}
	m.offlineQueueLock.Unlock()

	if len(queue) == 0 {
		return 0, nil
	}

	processed := 0
	var failedTasks []EmbeddingTask

	for _, task := range queue {
		vec, err := m.client.Embed(ctx, task.Text)
		if err != nil {
			failedTasks = append(failedTasks, task)
			continue
		}

		if err := m.vectorStore.StoreVector(task.ID, vec); err != nil {
			failedTasks = append(failedTasks, task)
			continue
		}

		processed++
	}

	// 将失败的任务重新加入队列
	if len(failedTasks) > 0 {
		m.offlineQueueLock.Lock()
		m.offlineQueue = append(m.offlineQueue, failedTasks...)
		m.offlineQueueLock.Unlock()
	}

	return processed, nil
}

// GetOfflineQueueSize 获取离线队列大小
func (m *EmbeddingManager) GetOfflineQueueSize() int {
	m.offlineQueueLock.Lock()
	defer m.offlineQueueLock.Unlock()
	return len(m.offlineQueue)
}

// ClearCache 清除缓存
func (m *EmbeddingManager) ClearCache() {
	m.cacheLock.Lock()
	m.cache = make(map[string][]float32)
	m.cacheLock.Unlock()
}

// GetCacheSize 获取缓存大小
func (m *EmbeddingManager) GetCacheSize() int {
	m.cacheLock.RLock()
	defer m.cacheLock.RUnlock()
	return len(m.cache)
}

// NewEmbeddingClient 根据配置创建 Embedding 客户端
func NewEmbeddingClient(config *EmbeddingConfig, apiKey string) EmbeddingClient {
	switch config.Provider {
	case "openai":
		return NewOpenAIEmbeddingClient(config, apiKey)
	case "deepseek":
		fallthrough
	default:
		return NewDeepSeekEmbeddingClient(config, apiKey)
	}
}

// MockEmbeddingClient 模拟 Embedding 客户端（用于测试）
type MockEmbeddingClient struct {
	dimension int
}

// NewMockEmbeddingClient 创建模拟客户端
func NewMockEmbeddingClient(dimension int) *MockEmbeddingClient {
	return &MockEmbeddingClient{dimension: dimension}
}

// Embed 生成模拟向量
func (c *MockEmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	// 生成基于文本哈希的确定性向量
	vec := make([]float32, c.dimension)
	hash := 0
	for _, ch := range text {
		hash = hash*31 + int(ch)
	}
	for i := 0; i < c.dimension; i++ {
		vec[i] = float32(hash%1000) / 1000.0
		hash = hash*31 + i
	}
	return NormalizeVector(vec), nil
}

// EmbedBatch 批量生成模拟向量
func (c *MockEmbeddingClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for i, text := range texts {
		vec, err := c.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		vectors[i] = vec
	}
	return vectors, nil
}

// GetDimension 获取向量维度
func (c *MockEmbeddingClient) GetDimension() int {
	return c.dimension
}
