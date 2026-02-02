// Package v2 提供会话记忆动态裁剪功能
package v2

import (
	"fmt"
	"strings"
	"time"
)

// SessionTrimmer 会话裁剪器
type SessionTrimmer struct {
	sessionMgr    *SessionManager
	shortTermMgr  *ShortTermMemoryManager
	config        *MemoryConfig
	summarizeFunc SummarizeFunc
}

// SummarizeFunc 摘要生成函数类型
// 用于调用 LLM API 生成摘要
type SummarizeFunc func(content string) (string, error)

// TrimResult 裁剪结果
type TrimResult struct {
	// 裁剪时间
	TrimTime time.Time `json:"trim_time"`

	// 裁剪前消息数
	BeforeMessages int `json:"before_messages"`

	// 裁剪后消息数
	AfterMessages int `json:"after_messages"`

	// 裁剪前 Token 数
	BeforeTokens int `json:"before_tokens"`

	// 裁剪后 Token 数
	AfterTokens int `json:"after_tokens"`

	// 删除的消息数
	TrimmedMessages int `json:"trimmed_messages"`

	// 是否生成了摘要
	SummaryCreated bool `json:"summary_created"`

	// 摘要 ID（如果生成了）
	SummaryID string `json:"summary_id,omitempty"`

	// 错误信息
	Error string `json:"error,omitempty"`
}

// NewSessionTrimmer 创建会话裁剪器
func NewSessionTrimmer(
	sessionMgr *SessionManager,
	shortTermMgr *ShortTermMemoryManager,
	config *MemoryConfig,
	summarizeFunc SummarizeFunc,
) *SessionTrimmer {
	return &SessionTrimmer{
		sessionMgr:    sessionMgr,
		shortTermMgr:  shortTermMgr,
		config:        config,
		summarizeFunc: summarizeFunc,
	}
}

// Trim 执行裁剪
func (t *SessionTrimmer) Trim() (*TrimResult, error) {
	result := &TrimResult{
		TrimTime: time.Now(),
	}

	session := t.sessionMgr.GetCurrentSession()
	if session == nil {
		return result, nil
	}

	messages := t.sessionMgr.GetMessages()
	result.BeforeMessages = len(messages)
	result.BeforeTokens = session.TokenCount

	// 检查是否需要裁剪
	if !t.sessionMgr.NeedsTrimming() {
		result.AfterMessages = result.BeforeMessages
		result.AfterTokens = result.BeforeTokens
		return result, nil
	}

	// 计算需要保留的消息数
	protectedRounds := t.config.Session.ProtectedRounds
	protectedMessages := protectedRounds * 2 // 每轮包含用户消息和助手回复

	if len(messages) <= protectedMessages {
		// 消息数太少，无法裁剪
		result.AfterMessages = result.BeforeMessages
		result.AfterTokens = result.BeforeTokens
		return result, nil
	}

	// 分离需要裁剪的消息和保留的消息
	messagesToTrim := messages[:len(messages)-protectedMessages]
	messagesToKeep := messages[len(messages)-protectedMessages:]

	// 生成被裁剪消息的摘要
	if len(messagesToTrim) > 0 && t.summarizeFunc != nil {
		summary, err := t.generateSummary(messagesToTrim)
		if err != nil {
			result.Error = fmt.Sprintf("生成摘要失败: %v", err)
		} else if summary != "" {
			// 保存摘要到短期记忆
			if t.shortTermMgr != nil {
				summaryMem, err := t.shortTermMgr.AddContext(
					fmt.Sprintf("会话摘要 %s", time.Now().Format("2006-01-02 15:04")),
					summary,
					14, // 14天过期
				)
				if err == nil {
					result.SummaryCreated = true
					result.SummaryID = summaryMem.ID
				}
			}
		}
	}

	// 更新会话消息
	t.sessionMgr.messages = messagesToKeep

	// 重新计算 Token 数
	newTokenCount := 0
	for _, msg := range messagesToKeep {
		newTokenCount += msg.TokenCount
	}

	session.MessageCount = len(messagesToKeep)
	session.TokenCount = newTokenCount
	session.UpdatedAt = time.Now()

	// 保存更新
	if err := t.sessionMgr.fileStore.UpdateSession(session, messagesToKeep); err != nil {
		return nil, err
	}

	result.AfterMessages = len(messagesToKeep)
	result.AfterTokens = newTokenCount
	result.TrimmedMessages = len(messagesToTrim)

	return result, nil
}

// generateSummary 生成消息摘要
func (t *SessionTrimmer) generateSummary(messages []SessionMessage) (string, error) {
	if t.summarizeFunc == nil {
		return "", nil
	}

	// 构建需要摘要的内容
	var builder strings.Builder
	builder.WriteString("以下是需要摘要的对话内容：\n\n")

	for _, msg := range messages {
		roleDisplay := formatRoleDisplay(msg.Role)
		builder.WriteString(fmt.Sprintf("%s: %s\n\n", roleDisplay, msg.Content))
	}

	content := builder.String()

	// 调用摘要函数
	return t.summarizeFunc(content)
}

// TrimIfNeeded 如果需要则执行裁剪
func (t *SessionTrimmer) TrimIfNeeded() (*TrimResult, error) {
	if !t.sessionMgr.NeedsTrimming() {
		return nil, nil
	}
	return t.Trim()
}

// EstimateTrimCount 估算需要裁剪的消息数
func (t *SessionTrimmer) EstimateTrimCount() int {
	session := t.sessionMgr.GetCurrentSession()
	if session == nil {
		return 0
	}

	messages := t.sessionMgr.GetMessages()
	protectedMessages := t.config.Session.ProtectedRounds * 2

	if len(messages) <= protectedMessages {
		return 0
	}

	return len(messages) - protectedMessages
}

// GetTrimPreview 获取裁剪预览
func (t *SessionTrimmer) GetTrimPreview() *TrimPreview {
	session := t.sessionMgr.GetCurrentSession()
	if session == nil {
		return nil
	}

	messages := t.sessionMgr.GetMessages()
	protectedMessages := t.config.Session.ProtectedRounds * 2

	preview := &TrimPreview{
		CurrentMessages:   len(messages),
		CurrentTokens:     session.TokenCount,
		MaxTokens:         t.config.Session.MaxTokens,
		UsageRatio:        float64(session.TokenCount) / float64(t.config.Session.MaxTokens),
		ProtectedMessages: protectedMessages,
		WillTrim:          len(messages) > protectedMessages && t.sessionMgr.NeedsTrimming(),
	}

	if preview.WillTrim {
		preview.MessagesToTrim = len(messages) - protectedMessages

		// 估算裁剪后的 Token 数
		trimmedTokens := 0
		for i := 0; i < preview.MessagesToTrim && i < len(messages); i++ {
			trimmedTokens += messages[i].TokenCount
		}
		preview.EstimatedAfterTokens = session.TokenCount - trimmedTokens
	}

	return preview
}

// TrimPreview 裁剪预览
type TrimPreview struct {
	CurrentMessages      int     `json:"current_messages"`
	CurrentTokens        int     `json:"current_tokens"`
	MaxTokens            int     `json:"max_tokens"`
	UsageRatio           float64 `json:"usage_ratio"`
	ProtectedMessages    int     `json:"protected_messages"`
	WillTrim             bool    `json:"will_trim"`
	MessagesToTrim       int     `json:"messages_to_trim"`
	EstimatedAfterTokens int     `json:"estimated_after_tokens"`
}

// ForceTrim 强制裁剪（不检查阈值）
func (t *SessionTrimmer) ForceTrim(keepMessages int) (*TrimResult, error) {
	result := &TrimResult{
		TrimTime: time.Now(),
	}

	session := t.sessionMgr.GetCurrentSession()
	if session == nil {
		return result, nil
	}

	messages := t.sessionMgr.GetMessages()
	result.BeforeMessages = len(messages)
	result.BeforeTokens = session.TokenCount

	if len(messages) <= keepMessages {
		result.AfterMessages = result.BeforeMessages
		result.AfterTokens = result.BeforeTokens
		return result, nil
	}

	// 分离消息
	messagesToTrim := messages[:len(messages)-keepMessages]
	messagesToKeep := messages[len(messages)-keepMessages:]

	// 生成摘要
	if len(messagesToTrim) > 0 && t.summarizeFunc != nil {
		summary, err := t.generateSummary(messagesToTrim)
		if err != nil {
			result.Error = fmt.Sprintf("生成摘要失败: %v", err)
		} else if summary != "" {
			if t.shortTermMgr != nil {
				summaryMem, err := t.shortTermMgr.AddContext(
					fmt.Sprintf("会话摘要 %s", time.Now().Format("2006-01-02 15:04")),
					summary,
					14,
				)
				if err == nil {
					result.SummaryCreated = true
					result.SummaryID = summaryMem.ID
				}
			}
		}
	}

	// 更新会话
	t.sessionMgr.messages = messagesToKeep

	newTokenCount := 0
	for _, msg := range messagesToKeep {
		newTokenCount += msg.TokenCount
	}

	session.MessageCount = len(messagesToKeep)
	session.TokenCount = newTokenCount
	session.UpdatedAt = time.Now()

	if err := t.sessionMgr.fileStore.UpdateSession(session, messagesToKeep); err != nil {
		return nil, err
	}

	result.AfterMessages = len(messagesToKeep)
	result.AfterTokens = newTokenCount
	result.TrimmedMessages = len(messagesToTrim)

	return result, nil
}

// DefaultSummarizeFunc 默认摘要函数（返回简单摘要）
func DefaultSummarizeFunc(content string) (string, error) {
	// 简单实现：提取关键信息
	lines := strings.Split(content, "\n")
	var summary strings.Builder
	summary.WriteString("## 历史对话摘要\n\n")

	messageCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "👤") || strings.HasPrefix(line, "🤖") {
			messageCount++
		}
	}

	summary.WriteString(fmt.Sprintf("- 包含 %d 条历史消息\n", messageCount))
	summary.WriteString(fmt.Sprintf("- 摘要时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))

	// 提取用户消息的关键内容
	summary.WriteString("\n### 主要话题\n\n")
	summary.WriteString("（此处应由 LLM 生成详细摘要）\n")

	return summary.String(), nil
}
