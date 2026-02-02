// Package cli 提供记忆系统 v2 的 CLI 命令
package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	v2 "github.com/hession/aimate/internal/memory/v2"
)

// MemoryV2Commands 记忆系统 v2 CLI 命令处理器
type MemoryV2Commands struct {
	memSys *v2.MemorySystem
}

// NewMemoryV2Commands 创建记忆系统 v2 CLI 命令处理器
func NewMemoryV2Commands(memSys *v2.MemorySystem) *MemoryV2Commands {
	return &MemoryV2Commands{memSys: memSys}
}

// HandleCommand 处理记忆相关命令
// 返回: (是否处理了命令, 输出内容)
func (c *MemoryV2Commands) HandleCommand(cmd string) (bool, string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return false, ""
	}

	command := strings.ToLower(parts[0])

	switch command {
	// 会话管理命令
	case "/new":
		return true, c.newSession()
	case "/session":
		return true, c.handleSessionCommand(parts[1:])

	// 记忆管理命令
	case "/memory":
		return true, c.handleMemoryCommand(parts[1:])

	default:
		return false, ""
	}
}

// ========== 会话命令 ==========

// newSession 创建新会话
func (c *MemoryV2Commands) newSession() string {
	sess, err := c.memSys.NewSession()
	if err != nil {
		return fmt.Sprintf("❌ 创建新会话失败: %v", err)
	}
	return fmt.Sprintf("✅ 新会话已创建\n   会话 ID: %s\n   创建时间: %s",
		sess.ID[:8], sess.CreatedAt.Format("2006-01-02 15:04:05"))
}

// handleSessionCommand 处理会话子命令
func (c *MemoryV2Commands) handleSessionCommand(args []string) string {
	if len(args) == 0 {
		return c.sessionStatus()
	}

	subCmd := strings.ToLower(args[0])
	switch subCmd {
	case "status":
		return c.sessionStatus()
	case "list":
		return c.sessionList()
	case "restore":
		if len(args) < 2 {
			return "❌ 请指定会话 ID: /session restore <session_id>"
		}
		return c.sessionRestore(args[1])
	default:
		return c.sessionHelp()
	}
}

// sessionStatus 显示当前会话状态
func (c *MemoryV2Commands) sessionStatus() string {
	stats := c.memSys.Session().GetSessionStats()

	var builder strings.Builder
	builder.WriteString("📊 会话状态\n\n")

	if stats.CurrentSessionID != "" {
		builder.WriteString(fmt.Sprintf("当前会话: %s\n", stats.CurrentSessionID[:8]))
		builder.WriteString(fmt.Sprintf("消息数量: %d\n", stats.CurrentMessages))
		builder.WriteString(fmt.Sprintf("Token 使用: %d / %d (%.1f%%)\n",
			stats.CurrentTokens, stats.MaxTokens, stats.UsageRatio*100))

		// 显示警告
		if stats.UsageRatio >= 0.85 {
			builder.WriteString("\n⚠️ 会话上下文即将达到限制，建议使用 /new 开启新会话\n")
		} else if stats.UsageRatio >= 0.7 {
			builder.WriteString("\n⚠️ 会话上下文使用较高，请注意\n")
		}
	} else {
		builder.WriteString("当前无活跃会话\n")
	}

	builder.WriteString(fmt.Sprintf("\n总会话数: %d\n", stats.TotalSessions))
	builder.WriteString(fmt.Sprintf("活跃会话: %d\n", stats.ActiveSessions))

	return builder.String()
}

// sessionList 列出最近会话
func (c *MemoryV2Commands) sessionList() string {
	sessions, err := c.memSys.Session().ListRecentSessions(10)
	if err != nil {
		return fmt.Sprintf("❌ 获取会话列表失败: %v", err)
	}

	if len(sessions) == 0 {
		return "📋 暂无会话记录"
	}

	var builder strings.Builder
	builder.WriteString("📋 最近会话\n\n")

	for i, sess := range sessions {
		status := "📝"
		if sess.Status == v2.StatusArchived {
			status = "📦"
		}

		title := sess.Title
		if title == "" {
			title = "(无标题)"
		}

		builder.WriteString(fmt.Sprintf("%d. %s %s - %s\n",
			i+1, status, sess.ID[:8], title))
		builder.WriteString(fmt.Sprintf("   消息: %d, Token: %d, 创建: %s\n",
			sess.MessageCount, sess.TokenCount,
			sess.CreatedAt.Format("01-02 15:04")))
	}

	builder.WriteString("\n使用 /session restore <id> 恢复会话")
	return builder.String()
}

// sessionRestore 恢复指定会话
func (c *MemoryV2Commands) sessionRestore(sessionID string) string {
	if err := c.memSys.Session().RestoreSession(sessionID); err != nil {
		return fmt.Sprintf("❌ 恢复会话失败: %v", err)
	}
	return fmt.Sprintf("✅ 会话已恢复: %s", sessionID)
}

// sessionHelp 显示会话命令帮助
func (c *MemoryV2Commands) sessionHelp() string {
	return `📖 会话命令帮助

/new                      - 创建新会话
/session                  - 显示当前会话状态
/session status           - 显示当前会话状态
/session list             - 列出最近会话
/session restore <id>     - 恢复指定会话`
}

// ========== 记忆管理命令 ==========

// handleMemoryCommand 处理记忆管理子命令
func (c *MemoryV2Commands) handleMemoryCommand(args []string) string {
	if len(args) == 0 {
		return c.memoryStats()
	}

	subCmd := strings.ToLower(args[0])
	switch subCmd {
	case "stats":
		return c.memoryStats()
	case "search":
		if len(args) < 2 {
			return "❌ 请指定搜索关键词: /memory search <keyword>"
		}
		return c.memorySearch(strings.Join(args[1:], " "))
	case "diagnose":
		return c.memoryDiagnose()
	case "reindex":
		return c.memoryReindex()
	case "sync":
		return c.memorySync()
	case "maintenance":
		return c.memoryMaintenance()
	case "core":
		return c.memoryCoreList()
	case "recent":
		return c.memoryRecent()
	default:
		return c.memoryHelp()
	}
}

// memoryStats 显示记忆统计
func (c *MemoryV2Commands) memoryStats() string {
	stats := c.memSys.GetStats()

	var builder strings.Builder
	builder.WriteString("📊 记忆系统统计\n\n")

	builder.WriteString("📌 核心记忆:\n")
	builder.WriteString(fmt.Sprintf("   Token 使用: %d\n", stats.CoreTokens))

	builder.WriteString("\n💬 会话记忆:\n")
	builder.WriteString(fmt.Sprintf("   当前消息: %d\n", stats.SessionMessages))
	builder.WriteString(fmt.Sprintf("   Token 使用: %d (%.1f%%)\n",
		stats.SessionTokens, stats.SessionUsageRatio*100))

	builder.WriteString("\n📝 短期记忆:\n")
	builder.WriteString(fmt.Sprintf("   总数: %d\n", stats.ShortTermCount))
	builder.WriteString(fmt.Sprintf("   已过期: %d\n", stats.ShortTermExpired))

	builder.WriteString("\n📚 长期记忆:\n")
	builder.WriteString(fmt.Sprintf("   总数: %d\n", stats.LongTermCount))

	builder.WriteString("\n🔍 索引统计:\n")
	builder.WriteString(fmt.Sprintf("   索引条目: %d\n", stats.IndexedCount))
	builder.WriteString(fmt.Sprintf("   向量条目: %d\n", stats.VectorCount))

	return builder.String()
}

// memorySearch 搜索记忆
func (c *MemoryV2Commands) memorySearch(keyword string) string {
	ctx := context.Background()
	memories, err := c.memSys.Search(ctx, keyword, 10)
	if err != nil {
		return fmt.Sprintf("❌ 搜索失败: %v", err)
	}

	if len(memories) == 0 {
		return fmt.Sprintf("🔍 未找到与 \"%s\" 相关的记忆", keyword)
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("🔍 搜索结果 (关键词: %s)\n\n", keyword))

	for i, mem := range memories {
		typeIcon := getMemoryTypeIcon(mem.Type)
		builder.WriteString(fmt.Sprintf("%d. %s [%s] %s\n",
			i+1, typeIcon, mem.Category, mem.Title))
		builder.WriteString(fmt.Sprintf("   %s\n", truncateForDisplay(mem.Content, 100)))
		builder.WriteString("\n")
	}

	return builder.String()
}

// memoryDiagnose 诊断记忆系统
func (c *MemoryV2Commands) memoryDiagnose() string {
	var builder strings.Builder
	builder.WriteString("🔧 记忆系统诊断\n\n")

	// 检查警告
	warnings := c.memSys.CheckWarnings()
	if len(warnings) > 0 {
		builder.WriteString("⚠️ 警告:\n")
		for _, w := range warnings {
			icon := "⚠️"
			if w.Level == "critical" {
				icon = "🚨"
			}
			builder.WriteString(fmt.Sprintf("   %s %s\n", icon, w.Message))
		}
		builder.WriteString("\n")
	} else {
		builder.WriteString("✅ 无警告\n\n")
	}

	// 显示统计
	stats := c.memSys.GetStats()
	builder.WriteString("📊 状态:\n")
	builder.WriteString(fmt.Sprintf("   索引条目: %d\n", stats.IndexedCount))
	builder.WriteString(fmt.Sprintf("   向量条目: %d\n", stats.VectorCount))
	builder.WriteString(fmt.Sprintf("   短期记忆: %d (过期: %d)\n",
		stats.ShortTermCount, stats.ShortTermExpired))
	builder.WriteString(fmt.Sprintf("   长期记忆: %d\n", stats.LongTermCount))

	return builder.String()
}

// memoryReindex 重建索引
func (c *MemoryV2Commands) memoryReindex() string {
	result, err := c.memSys.Reindex()
	if err != nil {
		return fmt.Sprintf("❌ 重建索引失败: %v", err)
	}

	return fmt.Sprintf("✅ 索引重建完成\n"+
		"   新建: %d\n"+
		"   更新: %d\n"+
		"   错误: %d\n"+
		"   耗时: %dms",
		result.Created, result.Updated, result.Errors, result.DurationMs)
}

// memorySync 同步索引
func (c *MemoryV2Commands) memorySync() string {
	result, err := c.memSys.SyncIndex()
	if err != nil {
		return fmt.Sprintf("❌ 同步索引失败: %v", err)
	}

	return fmt.Sprintf("✅ 索引同步完成\n"+
		"   新建: %d\n"+
		"   更新: %d\n"+
		"   删除: %d\n"+
		"   跳过: %d\n"+
		"   耗时: %dms",
		result.Created, result.Updated, result.Deleted, result.Skipped, result.DurationMs)
}

// memoryMaintenance 运行维护任务
func (c *MemoryV2Commands) memoryMaintenance() string {
	ctx := context.Background()
	result := c.memSys.RunMaintenance(ctx)

	var builder strings.Builder
	builder.WriteString("🔧 维护任务完成\n\n")
	builder.WriteString(fmt.Sprintf("   清理过期: %d\n", result.ExpiredCleaned))
	builder.WriteString(fmt.Sprintf("   归档不活跃: %d\n", result.InactiveArchived))
	builder.WriteString(fmt.Sprintf("   提升记忆: %d\n", result.Promoted))
	builder.WriteString(fmt.Sprintf("   同步索引: %d\n", result.IndexSynced))
	builder.WriteString(fmt.Sprintf("   清理孤立: %d\n", result.OrphanedCleaned))
	builder.WriteString(fmt.Sprintf("   耗时: %dms\n", result.DurationMs))

	if len(result.Errors) > 0 {
		builder.WriteString("\n⚠️ 错误:\n")
		for _, e := range result.Errors {
			builder.WriteString(fmt.Sprintf("   - %s\n", e))
		}
	}

	return builder.String()
}

// memoryCoreList 列出核心记忆
func (c *MemoryV2Commands) memoryCoreList() string {
	memories, err := c.memSys.Core().LoadAll()
	if err != nil {
		return fmt.Sprintf("❌ 加载核心记忆失败: %v", err)
	}

	if len(memories) == 0 {
		return "📌 暂无核心记忆"
	}

	var builder strings.Builder
	builder.WriteString("📌 核心记忆列表\n\n")

	for i, mem := range memories {
		builder.WriteString(fmt.Sprintf("%d. [%s] %s\n",
			i+1, mem.Category, mem.Title))
		builder.WriteString(fmt.Sprintf("   %s\n\n",
			truncateForDisplay(mem.Content, 80)))
	}

	return builder.String()
}

// memoryRecent 显示最近记忆
func (c *MemoryV2Commands) memoryRecent() string {
	memories, err := c.memSys.ShortTerm().LoadRecent(7)
	if err != nil {
		return fmt.Sprintf("❌ 加载最近记忆失败: %v", err)
	}

	if len(memories) == 0 {
		return "📝 最近 7 天无短期记忆"
	}

	var builder strings.Builder
	builder.WriteString("📝 最近 7 天的短期记忆\n\n")

	for i, mem := range memories {
		if i >= 10 {
			builder.WriteString(fmt.Sprintf("\n... 还有 %d 条记忆", len(memories)-10))
			break
		}

		builder.WriteString(fmt.Sprintf("%d. [%s] %s\n",
			i+1, mem.Category, mem.Title))
		builder.WriteString(fmt.Sprintf("   创建: %s",
			mem.CreatedAt.Format("01-02 15:04")))
		if mem.ExpiresAt != nil {
			builder.WriteString(fmt.Sprintf(", 过期: %s",
				mem.ExpiresAt.Format("01-02 15:04")))
		}
		builder.WriteString("\n\n")
	}

	return builder.String()
}

// memoryHelp 显示记忆命令帮助
func (c *MemoryV2Commands) memoryHelp() string {
	return `📖 记忆管理命令帮助

/memory                   - 显示记忆系统统计
/memory stats             - 显示记忆系统统计
/memory search <keyword>  - 搜索记忆
/memory core              - 列出核心记忆
/memory recent            - 显示最近短期记忆
/memory diagnose          - 诊断记忆系统
/memory sync              - 同步索引
/memory reindex           - 重建索引
/memory maintenance       - 运行维护任务`
}

// ========== 工具函数 ==========

// getMemoryTypeIcon 获取记忆类型图标
func getMemoryTypeIcon(memType v2.MemoryType) string {
	switch memType {
	case v2.MemoryTypeCore:
		return "📌"
	case v2.MemoryTypeSession:
		return "💬"
	case v2.MemoryTypeShortTerm:
		return "📝"
	case v2.MemoryTypeLongTerm:
		return "📚"
	default:
		return "📄"
	}
}

// truncateForDisplay 截断文本用于显示
func truncateForDisplay(text string, maxLen int) string {
	// 移除换行符
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", "")
	text = strings.TrimSpace(text)

	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

// GetCommandSuggestions 获取命令建议（用于自动补全）
func GetMemoryV2CommandSuggestions() []CommandSuggestion {
	return []CommandSuggestion{
		{Text: "/new", Description: "创建新会话"},
		{Text: "/session", Description: "显示会话状态"},
		{Text: "/session list", Description: "列出最近会话"},
		{Text: "/session restore", Description: "恢复指定会话"},
		{Text: "/memory", Description: "显示记忆统计"},
		{Text: "/memory stats", Description: "显示记忆统计"},
		{Text: "/memory search", Description: "搜索记忆"},
		{Text: "/memory core", Description: "列出核心记忆"},
		{Text: "/memory recent", Description: "显示最近记忆"},
		{Text: "/memory diagnose", Description: "诊断记忆系统"},
		{Text: "/memory sync", Description: "同步索引"},
		{Text: "/memory reindex", Description: "重建索引"},
		{Text: "/memory maintenance", Description: "运行维护任务"},
	}
}

// CommandSuggestion 命令建议
type CommandSuggestion struct {
	Text        string
	Description string
}

// FormatDuration 格式化时间间隔
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d秒", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d分钟", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d小时", int(d.Hours()))
	}
	return fmt.Sprintf("%d天", int(d.Hours()/24))
}
