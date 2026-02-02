// Package agent 提供 Agent 与记忆系统 v2 的集成
package agent

import (
	"context"

	v2 "github.com/hession/aimate/internal/memory/v2"
)

// MemoryV2Integration 记忆系统 v2 集成层
type MemoryV2Integration struct {
	memSys *v2.MemorySystem
	agent  *Agent
}

// NewMemoryV2Integration 创建记忆系统 v2 集成
func NewMemoryV2Integration(agent *Agent, apiKey string) (*MemoryV2Integration, error) {
	memSys, err := v2.NewMemorySystem()
	if err != nil {
		return nil, err
	}

	if err := memSys.Initialize(apiKey); err != nil {
		return nil, err
	}

	return &MemoryV2Integration{
		memSys: memSys,
		agent:  agent,
	}, nil
}

// SetProject 设置当前项目
func (m *MemoryV2Integration) SetProject(projectPath string) error {
	return m.memSys.SetProject(projectPath)
}

// GetMemorySystem 获取记忆系统
func (m *MemoryV2Integration) GetMemorySystem() *v2.MemorySystem {
	return m.memSys
}

// ProcessUserMessage 处理用户消息（自动识别并存储记忆）
func (m *MemoryV2Integration) ProcessUserMessage(ctx context.Context, message string) (*v2.ClassificationResult, error) {
	return m.memSys.ProcessUserInput(ctx, message)
}

// AddConversation 添加对话到会话记忆
func (m *MemoryV2Integration) AddConversation(role, content string, tokenCount int) error {
	return m.memSys.AddConversation(role, content, tokenCount)
}

// BuildEnrichedContext 构建增强的上下文
func (m *MemoryV2Integration) BuildEnrichedContext(ctx context.Context, query string) (string, error) {
	builtCtx, err := m.memSys.BuildContext(ctx, query)
	if err != nil {
		return "", err
	}
	return builtCtx.Content, nil
}

// CheckAndTrimSession 检查并在需要时裁剪会话
func (m *MemoryV2Integration) CheckAndTrimSession() (*v2.TrimResult, error) {
	return m.memSys.TrimSessionIfNeeded()
}

// GetSessionWarnings 获取会话警告
func (m *MemoryV2Integration) GetSessionWarnings() []string {
	return m.memSys.CheckSessionThreshold()
}

// GetContextWarnings 获取上下文警告
func (m *MemoryV2Integration) GetContextWarnings() []v2.ContextWarning {
	return m.memSys.CheckWarnings()
}

// NewSession 创建新会话
func (m *MemoryV2Integration) NewSession() (*v2.Session, error) {
	return m.memSys.NewSession()
}

// SearchMemories 搜索记忆
func (m *MemoryV2Integration) SearchMemories(ctx context.Context, query string, topK int) ([]*v2.Memory, error) {
	return m.memSys.Search(ctx, query, topK)
}

// GetStats 获取记忆系统统计
func (m *MemoryV2Integration) GetStats() *v2.MemorySystemStats {
	return m.memSys.GetStats()
}

// Close 关闭记忆系统
func (m *MemoryV2Integration) Close() error {
	return m.memSys.Close()
}

// AddCoreMemory 添加核心记忆
func (m *MemoryV2Integration) AddCoreMemory(category v2.MemoryCategory, title, content string) (*v2.Memory, error) {
	return m.memSys.Core().Add(category, title, content)
}

// AddShortTermMemory 添加短期记忆
func (m *MemoryV2Integration) AddShortTermMemory(category v2.MemoryCategory, title, content string, ttlDays int) (*v2.Memory, error) {
	scope := v2.ScopeGlobal
	return m.memSys.ShortTerm().Add(category, scope, title, content, ttlDays)
}

// AddLongTermMemory 添加长期记忆
func (m *MemoryV2Integration) AddLongTermMemory(category v2.MemoryCategory, title, content string, tags []string) (*v2.Memory, error) {
	scope := v2.ScopeGlobal
	return m.memSys.LongTerm().Add(category, scope, title, content, tags)
}

// SyncIndex 同步索引
func (m *MemoryV2Integration) SyncIndex() (*v2.SyncResult, error) {
	return m.memSys.SyncIndex()
}

// RunMaintenance 运行维护任务
func (m *MemoryV2Integration) RunMaintenance(ctx context.Context) *v2.MaintenanceResult {
	return m.memSys.RunMaintenance(ctx)
}

// EstimateTokens 估算文本的 Token 数
func EstimateTokens(text string) int {
	// 简单估算：英文约 4 字符/token，中文约 1.5 字符/token
	return len(text) / 3
}
