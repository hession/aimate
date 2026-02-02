// Package v2 提供混合检索功能
package v2

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
)

// Retriever 检索器接口
type Retriever interface {
	// Search 执行混合检索
	Search(ctx context.Context, query string, opts *RetrievalOptions) ([]*MemorySearchResult, error)

	// SearchByVector 向量检索
	SearchByVector(ctx context.Context, queryVec []float32, topK int) ([]*MemorySearchResult, error)

	// SearchByKeyword 关键词检索
	SearchByKeyword(keyword string, topK int) ([]*MemorySearchResult, error)
}

// RetrievalOptions 检索选项
type RetrievalOptions struct {
	// 检索的记忆类型（空表示全部）
	MemoryTypes []MemoryType

	// 检索的作用域
	Scope MemoryScope

	// 最终返回数量
	TopK int

	// 是否启用向量检索
	UseVector bool

	// 是否启用关键词检索
	UseKeyword bool

	// 是否启用时间权重
	UseTimeWeight bool

	// 最小相似度
	MinSimilarity float64
}

// DefaultRetrievalOptions 默认检索选项
func DefaultRetrievalOptions() *RetrievalOptions {
	return &RetrievalOptions{
		MemoryTypes:   nil, // 全部
		TopK:          5,
		UseVector:     true,
		UseKeyword:    true,
		UseTimeWeight: true,
		MinSimilarity: 0.6,
	}
}

// HybridRetriever 混合检索器
type HybridRetriever struct {
	index     IndexStore
	vector    VectorStore
	fileStore *MarkdownFileStore
	embedding *EmbeddingManager
	config    *RetrievalConfig
}

// NewHybridRetriever 创建混合检索器
func NewHybridRetriever(
	index IndexStore,
	vector VectorStore,
	fileStore *MarkdownFileStore,
	embedding *EmbeddingManager,
	config *RetrievalConfig,
) *HybridRetriever {
	return &HybridRetriever{
		index:     index,
		vector:    vector,
		fileStore: fileStore,
		embedding: embedding,
		config:    config,
	}
}

// Search 执行混合检索
func (r *HybridRetriever) Search(ctx context.Context, query string, opts *RetrievalOptions) ([]*MemorySearchResult, error) {
	if opts == nil {
		opts = DefaultRetrievalOptions()
	}

	var allResults []*MemorySearchResult

	// 1. 向量检索
	if opts.UseVector && r.embedding != nil {
		vectorResults, err := r.searchByVector(ctx, query, r.config.VectorTopK)
		if err == nil {
			allResults = append(allResults, vectorResults...)
		}
	}

	// 2. 关键词检索
	if opts.UseKeyword {
		keywordResults, err := r.searchByKeyword(query, r.config.KeywordTopK)
		if err == nil {
			allResults = append(allResults, keywordResults...)
		}
	}

	// 3. 合并去重
	merged := r.mergeResults(allResults)

	// 4. 应用时间权重
	if opts.UseTimeWeight {
		r.applyTimeWeight(merged)
	}

	// 5. 过滤条件
	filtered := r.filterResults(merged, opts)

	// 6. 排序并截取
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Score > filtered[j].Score
	})

	topK := opts.TopK
	if topK <= 0 {
		topK = r.config.FinalTopK
	}
	if len(filtered) > topK {
		filtered = filtered[:topK]
	}

	return filtered, nil
}

// searchByVector 向量检索
func (r *HybridRetriever) searchByVector(ctx context.Context, query string, topK int) ([]*MemorySearchResult, error) {
	// 使用 embedding 管理器搜索
	vectorResults, err := r.embedding.SearchSimilar(ctx, query, topK)
	if err != nil {
		return nil, err
	}

	var results []*MemorySearchResult
	for _, vr := range vectorResults {
		// 获取记忆详情
		idx, err := r.index.GetIndex(vr.ID)
		if err != nil {
			continue
		}

		mem, err := r.fileStore.ReadMemory(idx.FilePath)
		if err != nil {
			continue
		}

		results = append(results, &MemorySearchResult{
			Memory:    mem,
			Score:     vr.Score,
			MatchType: "vector",
		})
	}

	return results, nil
}

// SearchByVector 向量检索（公开方法）
func (r *HybridRetriever) SearchByVector(ctx context.Context, queryVec []float32, topK int) ([]*MemorySearchResult, error) {
	vectorResults, err := r.vector.SearchSimilar(queryVec, topK, r.config.MinSimilarity)
	if err != nil {
		return nil, err
	}

	var results []*MemorySearchResult
	for _, vr := range vectorResults {
		idx, err := r.index.GetIndex(vr.ID)
		if err != nil {
			continue
		}

		mem, err := r.fileStore.ReadMemory(idx.FilePath)
		if err != nil {
			continue
		}

		results = append(results, &MemorySearchResult{
			Memory:    mem,
			Score:     vr.Score,
			MatchType: "vector",
		})
	}

	return results, nil
}

// searchByKeyword 关键词检索
func (r *HybridRetriever) searchByKeyword(query string, topK int) ([]*MemorySearchResult, error) {
	// 提取关键词
	keywords := ExtractKeywords(query, 5)

	var allIndexes []*MemoryIndex

	// 对每个关键词进行搜索
	for _, keyword := range keywords {
		indexes, err := r.index.SearchByKeyword(keyword, topK)
		if err != nil {
			continue
		}
		allIndexes = append(allIndexes, indexes...)
	}

	// 也搜索原始查询
	indexes, err := r.index.SearchByKeyword(query, topK)
	if err == nil {
		allIndexes = append(allIndexes, indexes...)
	}

	// 去重
	seen := make(map[string]bool)
	var uniqueIndexes []*MemoryIndex
	for _, idx := range allIndexes {
		if !seen[idx.ID] {
			seen[idx.ID] = true
			uniqueIndexes = append(uniqueIndexes, idx)
		}
	}

	var results []*MemorySearchResult
	for i, idx := range uniqueIndexes {
		if i >= topK {
			break
		}

		mem, err := r.fileStore.ReadMemory(idx.FilePath)
		if err != nil {
			continue
		}

		// 计算关键词匹配分数
		score := r.calculateKeywordScore(query, mem)

		results = append(results, &MemorySearchResult{
			Memory:    mem,
			Score:     score,
			MatchType: "keyword",
		})
	}

	return results, nil
}

// SearchByKeyword 关键词检索（公开方法）
func (r *HybridRetriever) SearchByKeyword(keyword string, topK int) ([]*MemorySearchResult, error) {
	return r.searchByKeyword(keyword, topK)
}

// calculateKeywordScore 计算关键词匹配分数
func (r *HybridRetriever) calculateKeywordScore(query string, mem *Memory) float64 {
	queryLower := strings.ToLower(query)
	titleLower := strings.ToLower(mem.Title)
	contentLower := strings.ToLower(mem.Content)

	score := 0.0

	// 标题完全匹配
	if strings.Contains(titleLower, queryLower) {
		score += 0.5
	}

	// 内容匹配
	if strings.Contains(contentLower, queryLower) {
		score += 0.3
	}

	// 标签匹配
	for _, tag := range mem.Tags {
		if strings.Contains(strings.ToLower(tag), queryLower) {
			score += 0.2
			break
		}
	}

	// 关键词部分匹配
	keywords := strings.Fields(queryLower)
	matchCount := 0
	for _, kw := range keywords {
		if strings.Contains(titleLower, kw) || strings.Contains(contentLower, kw) {
			matchCount++
		}
	}
	if len(keywords) > 0 {
		score += float64(matchCount) / float64(len(keywords)) * 0.3
	}

	return math.Min(score, 1.0)
}

// mergeResults 合并去重结果
func (r *HybridRetriever) mergeResults(results []*MemorySearchResult) []*MemorySearchResult {
	// 按 ID 合并，保留最高分数
	mergedMap := make(map[string]*MemorySearchResult)

	for _, result := range results {
		if result.Memory == nil {
			continue
		}

		existing, ok := mergedMap[result.Memory.ID]
		if !ok {
			mergedMap[result.Memory.ID] = result
		} else {
			// 合并分数和匹配类型
			if result.Score > existing.Score {
				existing.Score = result.Score
			}
			if existing.MatchType != result.MatchType {
				existing.MatchType = "hybrid"
			}
		}
	}

	var merged []*MemorySearchResult
	for _, result := range mergedMap {
		merged = append(merged, result)
	}

	return merged
}

// applyTimeWeight 应用时间权重
func (r *HybridRetriever) applyTimeWeight(results []*MemorySearchResult) {
	now := time.Now()

	for _, result := range results {
		if result.Memory == nil {
			continue
		}

		// 计算时间衰减
		daysSinceUpdate := now.Sub(result.Memory.UpdatedAt).Hours() / 24
		timeWeight := math.Pow(r.config.TimeDecayFactor, daysSinceUpdate/30) // 每30天衰减

		// 应用时间权重
		result.Score *= timeWeight

		// 重要性加成
		importanceBoost := float64(result.Memory.Importance) / 5.0
		result.Score *= (0.8 + 0.2*importanceBoost)

		// 访问频率加成
		if result.Memory.AccessCount > 0 {
			accessBoost := math.Log10(float64(result.Memory.AccessCount)+1) / 3.0
			result.Score *= (1 + accessBoost*0.1)
		}
	}
}

// filterResults 过滤结果
func (r *HybridRetriever) filterResults(results []*MemorySearchResult, opts *RetrievalOptions) []*MemorySearchResult {
	var filtered []*MemorySearchResult

	for _, result := range results {
		if result.Memory == nil {
			continue
		}

		// 过滤分数
		if result.Score < opts.MinSimilarity {
			continue
		}

		// 过滤记忆类型
		if len(opts.MemoryTypes) > 0 {
			typeMatch := false
			for _, mt := range opts.MemoryTypes {
				if result.Memory.Type == mt {
					typeMatch = true
					break
				}
			}
			if !typeMatch {
				continue
			}
		}

		// 过滤作用域
		if opts.Scope != "" && result.Memory.Scope != opts.Scope {
			continue
		}

		// 过滤过期记忆
		if result.Memory.IsExpired() {
			continue
		}

		// 过滤非活跃记忆
		if result.Memory.Status != StatusActive {
			continue
		}

		filtered = append(filtered, result)
	}

	return filtered
}

// QuickSearch 快速搜索（简化版）
func (r *HybridRetriever) QuickSearch(ctx context.Context, query string, topK int) ([]*Memory, error) {
	opts := &RetrievalOptions{
		TopK:          topK,
		UseVector:     r.embedding != nil,
		UseKeyword:    true,
		UseTimeWeight: true,
		MinSimilarity: 0.5,
	}

	results, err := r.Search(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	var memories []*Memory
	for _, result := range results {
		if result.Memory != nil {
			memories = append(memories, result.Memory)
		}
	}

	return memories, nil
}

// SearchContext 搜索相关上下文
func (r *HybridRetriever) SearchContext(ctx context.Context, query string, maxTokens int) (string, error) {
	results, err := r.Search(ctx, query, &RetrievalOptions{
		TopK:          10,
		UseVector:     r.embedding != nil,
		UseKeyword:    true,
		UseTimeWeight: true,
		MinSimilarity: 0.5,
	})
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("## 相关记忆\n\n")

	currentTokens := 0
	for _, result := range results {
		if result.Memory == nil {
			continue
		}

		// 估算 Token
		entryTokens := len(result.Memory.Title)/4 + len(result.Memory.Content)/4 + 20
		if currentTokens+entryTokens > maxTokens {
			break
		}

		builder.WriteString("### ")
		builder.WriteString(result.Memory.Title)
		builder.WriteString("\n\n")
		builder.WriteString(truncateContent(result.Memory.Content, 300))
		builder.WriteString("\n\n")

		currentTokens += entryTokens
	}

	return builder.String(), nil
}

// GetRetrievalStats 获取检索统计
func (r *HybridRetriever) GetRetrievalStats() *RetrievalStats {
	stats := &RetrievalStats{}

	// 获取索引统计
	indexStats, err := r.index.GetIndexStats()
	if err == nil {
		stats.TotalIndexed = indexStats.TotalCount
	}

	// 获取向量统计
	if r.vector != nil {
		vectorStats, err := r.vector.GetVectorStats()
		if err == nil {
			stats.TotalVectors = vectorStats.TotalVectors
			stats.VectorDimension = vectorStats.Dimension
		}
	}

	// Embedding 缓存统计
	if r.embedding != nil {
		stats.EmbeddingCacheSize = r.embedding.GetCacheSize()
		stats.OfflineQueueSize = r.embedding.GetOfflineQueueSize()
	}

	return stats
}

// RetrievalStats 检索统计
type RetrievalStats struct {
	TotalIndexed       int `json:"total_indexed"`
	TotalVectors       int `json:"total_vectors"`
	VectorDimension    int `json:"vector_dimension"`
	EmbeddingCacheSize int `json:"embedding_cache_size"`
	OfflineQueueSize   int `json:"offline_queue_size"`
}
