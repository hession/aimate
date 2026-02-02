// Package v2 提供上下文构建功能
package v2

import (
	"context"
	"fmt"
	"strings"
)

// ContextBuilder 上下文构建器
// 负责从各层记忆中构建 LLM 所需的上下文
type ContextBuilder struct {
	coreMgr      *CoreMemoryManager
	sessionMgr   *SessionManager
	shortTermMgr *ShortTermMemoryManager
	longTermMgr  *LongTermMemoryManager
	retriever    *HybridRetriever
	config       *MemoryConfig
}

// NewContextBuilder 创建上下文构建器
func NewContextBuilder(
	coreMgr *CoreMemoryManager,
	sessionMgr *SessionManager,
	shortTermMgr *ShortTermMemoryManager,
	longTermMgr *LongTermMemoryManager,
	retriever *HybridRetriever,
	config *MemoryConfig,
) *ContextBuilder {
	return &ContextBuilder{
		coreMgr:      coreMgr,
		sessionMgr:   sessionMgr,
		shortTermMgr: shortTermMgr,
		longTermMgr:  longTermMgr,
		retriever:    retriever,
		config:       config,
	}
}

// BuildContext 构建完整上下文
func (b *ContextBuilder) BuildContext(ctx context.Context, query string) (*BuiltContext, error) {
	budget := b.calculateBudget()
	result := &BuiltContext{
		Budget: budget,
	}

	var builder strings.Builder
	usedTokens := 0

	// 1. 核心记忆（最高优先级）
	if b.coreMgr != nil && budget.Core > 0 {
		coreContext, err := b.coreMgr.BuildContext()
		if err == nil && coreContext != "" {
			tokens := b.estimateTokens(coreContext)
			if tokens <= budget.Core {
				builder.WriteString(coreContext)
				builder.WriteString("\n")
				usedTokens += tokens
				result.CoreTokens = tokens
			}
		}
	}

	// 2. 相关记忆检索（基于查询）
	if query != "" && b.retriever != nil && budget.LongTerm+budget.ShortTerm > 0 {
		retrievalContext, retrievalTokens, err := b.buildRetrievalContext(ctx, query, budget.LongTerm+budget.ShortTerm)
		if err == nil && retrievalContext != "" {
			builder.WriteString(retrievalContext)
			builder.WriteString("\n")
			usedTokens += retrievalTokens
			result.RetrievalTokens = retrievalTokens
		}
	}

	// 3. 短期记忆
	if b.shortTermMgr != nil && budget.ShortTerm > result.RetrievalTokens/2 {
		remaining := budget.ShortTerm - result.RetrievalTokens/2
		if remaining > 0 {
			shortTermContext, err := b.shortTermMgr.BuildContext(remaining)
			if err == nil && shortTermContext != "" {
				tokens := b.estimateTokens(shortTermContext)
				builder.WriteString(shortTermContext)
				builder.WriteString("\n")
				usedTokens += tokens
				result.ShortTermTokens = tokens
			}
		}
	}

	// 4. 会话记忆（如果还有空间）
	// 会话记忆由 SessionManager 单独管理，这里只记录预算

	result.Content = builder.String()
	result.TotalTokens = usedTokens

	return result, nil
}

// BuildContextForNewSession 为新会话构建初始上下文
func (b *ContextBuilder) BuildContextForNewSession(ctx context.Context) (*BuiltContext, error) {
	budget := b.calculateBudget()
	result := &BuiltContext{
		Budget: budget,
	}

	var builder strings.Builder

	// 1. 核心记忆
	if b.coreMgr != nil {
		coreContext, err := b.coreMgr.BuildContext()
		if err == nil && coreContext != "" {
			builder.WriteString(coreContext)
			builder.WriteString("\n")
			result.CoreTokens = b.estimateTokens(coreContext)
		}
	}

	// 2. 最近的短期记忆
	if b.shortTermMgr != nil {
		recentMems, err := b.shortTermMgr.LoadRecent(7) // 最近7天
		if err == nil && len(recentMems) > 0 {
			builder.WriteString("## 最近记忆\n\n")
			tokens := 0
			for _, mem := range recentMems {
				if tokens >= budget.ShortTerm {
					break
				}
				entry := fmt.Sprintf("- **%s**: %s\n", mem.Title, truncateContent(mem.Content, 100))
				entryTokens := b.estimateTokens(entry)
				builder.WriteString(entry)
				tokens += entryTokens
			}
			builder.WriteString("\n")
			result.ShortTermTokens = tokens
		}
	}

	// 3. 重要的长期记忆
	if b.longTermMgr != nil {
		active, err := b.longTermMgr.LoadActive()
		if err == nil && len(active) > 0 {
			builder.WriteString("## 重要知识\n\n")
			tokens := 0
			for _, mem := range active {
				if tokens >= budget.LongTerm || mem.Importance < 4 {
					break
				}
				entry := fmt.Sprintf("- **%s**: %s\n", mem.Title, truncateContent(mem.Content, 100))
				entryTokens := b.estimateTokens(entry)
				builder.WriteString(entry)
				tokens += entryTokens
			}
			builder.WriteString("\n")
			result.LongTermTokens = tokens
		}
	}

	result.Content = builder.String()
	result.TotalTokens = result.CoreTokens + result.ShortTermTokens + result.LongTermTokens

	return result, nil
}

// buildRetrievalContext 构建检索上下文
func (b *ContextBuilder) buildRetrievalContext(ctx context.Context, query string, maxTokens int) (string, int, error) {
	if b.retriever == nil {
		return "", 0, nil
	}

	// 执行混合检索
	results, err := b.retriever.Search(ctx, query, &RetrievalOptions{
		TopK:          10,
		UseVector:     true,
		UseKeyword:    true,
		UseTimeWeight: true,
		MinSimilarity: 0.5,
	})
	if err != nil {
		return "", 0, err
	}

	if len(results) == 0 {
		return "", 0, nil
	}

	var builder strings.Builder
	builder.WriteString("## 相关记忆\n\n")

	totalTokens := 0
	for _, result := range results {
		if result.Memory == nil {
			continue
		}

		entry := fmt.Sprintf("### %s\n%s\n\n", result.Memory.Title, truncateContent(result.Memory.Content, 300))
		entryTokens := b.estimateTokens(entry)

		if totalTokens+entryTokens > maxTokens {
			break
		}

		builder.WriteString(entry)
		totalTokens += entryTokens
	}

	return builder.String(), totalTokens, nil
}

// calculateBudget 计算 Token 预算分配
func (b *ContextBuilder) calculateBudget() *ContextBudget {
	total := b.config.Context.TotalBudget
	return &ContextBudget{
		Total:     total,
		Core:      int(float64(total) * b.config.Context.CoreRatio),
		Session:   int(float64(total) * b.config.Context.SessionRatio),
		ShortTerm: int(float64(total) * b.config.Context.ShortTermRatio),
		LongTerm:  int(float64(total) * b.config.Context.LongTermRatio),
		Reserved:  int(float64(total) * b.config.Context.ReservedRatio),
	}
}

// estimateTokens 估算文本的 Token 数
func (b *ContextBuilder) estimateTokens(text string) int {
	// 简单估算：英文约 4 字符/token，中文约 1.5 字符/token
	// 这里使用保守估计
	return len(text) / 3
}

// BuiltContext 构建的上下文
type BuiltContext struct {
	Content         string         `json:"content"`
	TotalTokens     int            `json:"total_tokens"`
	CoreTokens      int            `json:"core_tokens"`
	SessionTokens   int            `json:"session_tokens"`
	ShortTermTokens int            `json:"short_term_tokens"`
	LongTermTokens  int            `json:"long_term_tokens"`
	RetrievalTokens int            `json:"retrieval_tokens"`
	Budget          *ContextBudget `json:"budget"`
}

// GetRemainingBudget 获取剩余预算
func (c *BuiltContext) GetRemainingBudget() int {
	return c.Budget.Total - c.TotalTokens
}

// IsOverBudget 检查是否超预算
func (c *BuiltContext) IsOverBudget() bool {
	return c.TotalTokens > c.Budget.Total
}

// BuildSystemPrompt 构建系统提示词
func (b *ContextBuilder) BuildSystemPrompt(ctx context.Context, basePrompt string) (string, error) {
	// 构建初始上下文
	builtCtx, err := b.BuildContextForNewSession(ctx)
	if err != nil {
		return basePrompt, err
	}

	if builtCtx.Content == "" {
		return basePrompt, nil
	}

	// 将记忆上下文添加到系统提示词
	return fmt.Sprintf("%s\n\n%s", basePrompt, builtCtx.Content), nil
}

// EnrichQuery 丰富查询上下文
func (b *ContextBuilder) EnrichQuery(ctx context.Context, query string) (string, error) {
	if b.retriever == nil {
		return query, nil
	}

	// 搜索相关记忆
	results, err := b.retriever.QuickSearch(ctx, query, 3)
	if err != nil || len(results) == 0 {
		return query, nil
	}

	// 构建丰富的上下文
	var builder strings.Builder
	builder.WriteString("用户问题: ")
	builder.WriteString(query)
	builder.WriteString("\n\n相关背景:\n")

	for _, mem := range results {
		builder.WriteString(fmt.Sprintf("- %s: %s\n", mem.Title, truncateContent(mem.Content, 100)))
	}

	return builder.String(), nil
}

// GetContextStats 获取上下文统计
func (b *ContextBuilder) GetContextStats() *ContextStats {
	stats := &ContextStats{
		Budget: b.calculateBudget(),
	}

	// 核心记忆统计
	if b.coreMgr != nil {
		coreTokens, _ := b.coreMgr.GetTotalTokens()
		stats.CoreUsed = coreTokens
	}

	// 会话统计
	if b.sessionMgr != nil {
		current, max, ratio := b.sessionMgr.GetTokenUsage()
		stats.SessionUsed = current
		stats.SessionMax = max
		stats.SessionRatio = ratio
	}

	// 短期记忆统计
	if b.shortTermMgr != nil {
		shortStats, err := b.shortTermMgr.GetStats()
		if err == nil {
			stats.ShortTermCount = shortStats.Total
		}
	}

	// 长期记忆统计
	if b.longTermMgr != nil {
		longStats, err := b.longTermMgr.GetStats()
		if err == nil {
			stats.LongTermCount = longStats.Total
		}
	}

	return stats
}

// ContextStats 上下文统计
type ContextStats struct {
	Budget         *ContextBudget `json:"budget"`
	CoreUsed       int            `json:"core_used"`
	SessionUsed    int            `json:"session_used"`
	SessionMax     int            `json:"session_max"`
	SessionRatio   float64        `json:"session_ratio"`
	ShortTermCount int            `json:"short_term_count"`
	LongTermCount  int            `json:"long_term_count"`
}

// ContextWarning 上下文警告
type ContextWarning struct {
	Level   string `json:"level"` // info, warning, critical
	Message string `json:"message"`
}

// CheckContextWarnings 检查上下文警告
func (b *ContextBuilder) CheckContextWarnings() []ContextWarning {
	var warnings []ContextWarning

	// 检查会话 Token 使用
	if b.sessionMgr != nil {
		_, _, ratio := b.sessionMgr.GetTokenUsage()
		if ratio >= 0.85 {
			warnings = append(warnings, ContextWarning{
				Level:   "critical",
				Message: fmt.Sprintf("会话上下文已使用 %.0f%%，建议开启新会话", ratio*100),
			})
		} else if ratio >= 0.7 {
			warnings = append(warnings, ContextWarning{
				Level:   "warning",
				Message: fmt.Sprintf("会话上下文已使用 %.0f%%，接近限制", ratio*100),
			})
		}
	}

	// 检查核心记忆
	if b.coreMgr != nil {
		overLimit, _ := b.coreMgr.IsOverLimit()
		if overLimit {
			warnings = append(warnings, ContextWarning{
				Level:   "warning",
				Message: "核心记忆超出 Token 限制，建议精炼",
			})
		}
	}

	return warnings
}
