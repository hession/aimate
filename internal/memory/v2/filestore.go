// Package v2 提供 Markdown 记忆文件读写功能
package v2

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// FileStore Markdown 文件存储接口
type FileStore interface {
	// 记忆操作
	CreateMemory(mem *Memory) error
	ReadMemory(filePath string) (*Memory, error)
	UpdateMemory(mem *Memory) error
	DeleteMemory(filePath string) error
	MoveMemory(srcPath, dstPath string) error

	// 会话操作
	CreateSession(sess *Session) error
	ReadSession(filePath string) (*Session, []SessionMessage, error)
	UpdateSession(sess *Session, messages []SessionMessage) error
	AppendSessionMessage(filePath string, msg *SessionMessage) error

	// 批量操作
	ListMemories(dir string) ([]*Memory, error)
	ScanAllMemories() ([]*Memory, error)
}

// MarkdownFileStore Markdown 文件存储实现
type MarkdownFileStore struct {
	storage *StorageManager
	parser  *FrontmatterParser
}

// NewMarkdownFileStore 创建 Markdown 文件存储
func NewMarkdownFileStore(storage *StorageManager) *MarkdownFileStore {
	return &MarkdownFileStore{
		storage: storage,
		parser:  NewFrontmatterParser(),
	}
}

// ========== 记忆操作 ==========

// CreateMemory 创建记忆文件
func (fs *MarkdownFileStore) CreateMemory(mem *Memory) error {
	// 生成 ID
	if mem.ID == "" {
		mem.ID = uuid.New().String()
	}

	// 设置时间戳
	now := time.Now()
	if mem.CreatedAt.IsZero() {
		mem.CreatedAt = now
	}
	mem.UpdatedAt = now
	mem.AccessedAt = now

	// 生成文件路径
	filePath := fs.storage.GenerateMemoryFilePath(mem)
	mem.FilePath = filePath

	// 计算内容哈希
	mem.ContentHash = CalculateContentHash([]byte(mem.Content))

	// 确保目录存在
	if err := EnsureDir(filePath); err != nil {
		return NewMemoryErrorWithPath("CreateMemory", filePath, err)
	}

	// 检查文件是否已存在
	if FileExists(filePath) {
		return NewMemoryErrorWithPath("CreateMemory", filePath, ErrFileAlreadyExists)
	}

	// 序列化为 Markdown
	content, err := fs.parser.SerializeMemory(mem)
	if err != nil {
		return NewMemoryErrorWithPath("CreateMemory", filePath, err)
	}

	// 写入文件
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return NewMemoryErrorWithPath("CreateMemory", filePath, err)
	}

	return nil
}

// ReadMemory 读取记忆文件
func (fs *MarkdownFileStore) ReadMemory(filePath string) (*Memory, error) {
	// 检查文件是否存在
	if !FileExists(filePath) {
		return nil, NewMemoryErrorWithPath("ReadMemory", filePath, ErrFileNotFound)
	}

	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, NewMemoryErrorWithPath("ReadMemory", filePath, err)
	}

	// 解析 Markdown
	mem, err := fs.parser.ParseMemory(content)
	if err != nil {
		return nil, NewMemoryErrorWithPath("ReadMemory", filePath, err)
	}

	mem.FilePath = filePath
	return mem, nil
}

// UpdateMemory 更新记忆文件
func (fs *MarkdownFileStore) UpdateMemory(mem *Memory) error {
	if mem.FilePath == "" {
		return NewMemoryError("UpdateMemory", ErrInvalidFilePath)
	}

	// 检查文件是否存在
	if !FileExists(mem.FilePath) {
		return NewMemoryErrorWithPath("UpdateMemory", mem.FilePath, ErrFileNotFound)
	}

	// 更新时间戳
	mem.UpdatedAt = time.Now()

	// 重新计算内容哈希
	mem.ContentHash = CalculateContentHash([]byte(mem.Content))

	// 序列化为 Markdown
	content, err := fs.parser.SerializeMemory(mem)
	if err != nil {
		return NewMemoryErrorWithPath("UpdateMemory", mem.FilePath, err)
	}

	// 写入文件
	if err := os.WriteFile(mem.FilePath, content, 0644); err != nil {
		return NewMemoryErrorWithPath("UpdateMemory", mem.FilePath, err)
	}

	return nil
}

// DeleteMemory 删除记忆文件
func (fs *MarkdownFileStore) DeleteMemory(filePath string) error {
	if !FileExists(filePath) {
		return NewMemoryErrorWithPath("DeleteMemory", filePath, ErrFileNotFound)
	}

	if err := os.Remove(filePath); err != nil {
		return NewMemoryErrorWithPath("DeleteMemory", filePath, err)
	}

	return nil
}

// MoveMemory 移动记忆文件（用于归档）
func (fs *MarkdownFileStore) MoveMemory(srcPath, dstPath string) error {
	if !FileExists(srcPath) {
		return NewMemoryErrorWithPath("MoveMemory", srcPath, ErrFileNotFound)
	}

	// 确保目标目录存在
	if err := EnsureDir(dstPath); err != nil {
		return NewMemoryErrorWithPath("MoveMemory", dstPath, err)
	}

	// 移动文件
	if err := os.Rename(srcPath, dstPath); err != nil {
		return NewMemoryErrorWithPath("MoveMemory", srcPath, err)
	}

	return nil
}

// ========== 会话操作 ==========

// CreateSession 创建会话文件
func (fs *MarkdownFileStore) CreateSession(sess *Session) error {
	// 生成 ID
	if sess.ID == "" {
		sess.ID = uuid.New().String()
	}

	// 设置时间戳
	now := time.Now()
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = now
	}
	sess.UpdatedAt = now

	// 生成文件路径
	filePath := fs.storage.GenerateSessionFilePath(sess)
	sess.FilePath = filePath

	// 确保目录存在
	if err := EnsureDir(filePath); err != nil {
		return NewMemoryErrorWithPath("CreateSession", filePath, err)
	}

	// 序列化为 Markdown（初始内容为空）
	content, err := fs.parser.SerializeSession(sess, []byte("## 对话记录\n\n"))
	if err != nil {
		return NewMemoryErrorWithPath("CreateSession", filePath, err)
	}

	// 写入文件
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return NewMemoryErrorWithPath("CreateSession", filePath, err)
	}

	return nil
}

// ReadSession 读取会话文件
func (fs *MarkdownFileStore) ReadSession(filePath string) (*Session, []SessionMessage, error) {
	if !FileExists(filePath) {
		return nil, nil, NewMemoryErrorWithPath("ReadSession", filePath, ErrFileNotFound)
	}

	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, NewMemoryErrorWithPath("ReadSession", filePath, err)
	}

	// 解析会话
	sess, body, err := fs.parser.ParseSession(content)
	if err != nil {
		return nil, nil, NewMemoryErrorWithPath("ReadSession", filePath, err)
	}

	sess.FilePath = filePath

	// 解析消息列表（从 body 中解析）
	messages := fs.parseSessionMessages(body)

	return sess, messages, nil
}

// UpdateSession 更新会话文件
func (fs *MarkdownFileStore) UpdateSession(sess *Session, messages []SessionMessage) error {
	if sess.FilePath == "" {
		return NewMemoryError("UpdateSession", ErrInvalidFilePath)
	}

	// 更新时间戳和统计
	sess.UpdatedAt = time.Now()
	sess.MessageCount = len(messages)

	// 计算总 token 数
	totalTokens := 0
	for _, msg := range messages {
		totalTokens += msg.TokenCount
	}
	sess.TokenCount = totalTokens

	// 生成消息内容
	body := fs.formatSessionMessages(messages)

	// 序列化为 Markdown
	content, err := fs.parser.SerializeSession(sess, body)
	if err != nil {
		return NewMemoryErrorWithPath("UpdateSession", sess.FilePath, err)
	}

	// 写入文件
	if err := os.WriteFile(sess.FilePath, content, 0644); err != nil {
		return NewMemoryErrorWithPath("UpdateSession", sess.FilePath, err)
	}

	return nil
}

// AppendSessionMessage 追加会话消息
func (fs *MarkdownFileStore) AppendSessionMessage(filePath string, msg *SessionMessage) error {
	// 读取现有会话
	sess, messages, err := fs.ReadSession(filePath)
	if err != nil {
		return err
	}

	// 设置消息序号
	msg.Sequence = len(messages) + 1
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	// 追加消息
	messages = append(messages, *msg)

	// 更新会话
	return fs.UpdateSession(sess, messages)
}

// parseSessionMessages 从 Markdown body 解析消息列表
func (fs *MarkdownFileStore) parseSessionMessages(body []byte) []SessionMessage {
	// 简化实现：按消息块解析
	// 实际消息格式：
	// ### [序号] 角色 (时间)
	// 内容
	//
	// 这里返回空列表，实际实现需要更复杂的解析逻辑

	var messages []SessionMessage
	// TODO: 实现完整的消息解析逻辑
	// 当前简化返回空列表，消息主要通过索引数据库查询
	_ = body
	return messages
}

// formatSessionMessages 格式化消息列表为 Markdown
func (fs *MarkdownFileStore) formatSessionMessages(messages []SessionMessage) []byte {
	var content string
	content = "## 对话记录\n\n"

	for _, msg := range messages {
		// 格式化角色显示
		roleDisplay := fs.formatRole(msg.Role)
		timeStr := msg.Timestamp.Format("2006-01-02 15:04:05")

		content += fmt.Sprintf("### [%d] %s (%s)\n\n", msg.Sequence, roleDisplay, timeStr)
		content += msg.Content
		if !endsWith(msg.Content, "\n") {
			content += "\n"
		}
		content += "\n"
	}

	return []byte(content)
}

// formatRole 格式化角色显示
func (fs *MarkdownFileStore) formatRole(role string) string {
	switch role {
	case "user":
		return "👤 用户"
	case "assistant":
		return "🤖 助手"
	case "system":
		return "⚙️ 系统"
	case "tool":
		return "🔧 工具"
	default:
		return role
	}
}

// ========== 批量操作 ==========

// ListMemories 列出指定目录下的所有记忆
func (fs *MarkdownFileStore) ListMemories(dir string) ([]*Memory, error) {
	files, err := fs.storage.ListMemoryFiles(dir)
	if err != nil {
		return nil, err
	}

	var memories []*Memory
	for _, file := range files {
		mem, err := fs.ReadMemory(file)
		if err != nil {
			// 跳过无法读取的文件，记录警告
			continue
		}
		memories = append(memories, mem)
	}

	return memories, nil
}

// ScanAllMemories 扫描所有记忆文件
func (fs *MarkdownFileStore) ScanAllMemories() ([]*Memory, error) {
	files, err := fs.storage.GetAllMemoryPaths()
	if err != nil {
		return nil, err
	}

	var memories []*Memory
	for _, file := range files {
		mem, err := fs.ReadMemory(file)
		if err != nil {
			// 跳过无法读取的文件
			continue
		}
		memories = append(memories, mem)
	}

	return memories, nil
}

// ========== 工具函数 ==========

// endsWith 检查字符串是否以指定后缀结束
func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// ReadMemoryByID 通过 ID 读取记忆（需要索引支持）
func (fs *MarkdownFileStore) ReadMemoryByID(id string) (*Memory, error) {
	// 此方法需要索引支持，暂时返回错误
	return nil, NewMemoryErrorWithDetails("ReadMemoryByID", ErrOperationFailed, "需要索引支持")
}

// UpdateMemoryAccess 更新记忆访问信息
func (fs *MarkdownFileStore) UpdateMemoryAccess(filePath string) error {
	mem, err := fs.ReadMemory(filePath)
	if err != nil {
		return err
	}

	mem.IncrementAccess()
	return fs.UpdateMemory(mem)
}

// ArchiveMemory 归档记忆
func (fs *MarkdownFileStore) ArchiveMemory(mem *Memory) error {
	if mem.FilePath == "" {
		return NewMemoryError("ArchiveMemory", ErrInvalidFilePath)
	}

	// 获取归档路径
	archivePath := fs.storage.GetArchivePath(mem)
	archiveDir := filepath.Dir(archivePath)
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return NewMemoryErrorWithPath("ArchiveMemory", archiveDir, err)
	}

	// 生成归档文件名
	fileName := filepath.Base(mem.FilePath)
	dstPath := filepath.Join(archivePath, fileName)

	// 更新状态
	mem.Status = StatusArchived
	if err := fs.UpdateMemory(mem); err != nil {
		return err
	}

	// 移动文件
	return fs.MoveMemory(mem.FilePath, dstPath)
}

// GetMemoryStats 获取记忆文件统计
func (fs *MarkdownFileStore) GetMemoryStats() (*MemoryStats, error) {
	stats := &MemoryStats{}

	// 扫描所有记忆
	memories, err := fs.ScanAllMemories()
	if err != nil {
		return nil, err
	}

	for _, mem := range memories {
		// 统计类型
		switch mem.Type {
		case MemoryTypeCore:
			stats.CoreCount++
		case MemoryTypeSession:
			stats.SessionCount++
		case MemoryTypeShortTerm:
			stats.ShortTermCount++
		case MemoryTypeLongTerm:
			stats.LongTermCount++
		}

		// 统计状态
		switch mem.Status {
		case StatusActive:
			stats.ActiveCount++
		case StatusArchived:
			stats.ArchivedCount++
		case StatusExpired:
			stats.ExpiredCount++
		}

		stats.TotalFiles++

		// 获取文件大小
		if info, err := os.Stat(mem.FilePath); err == nil {
			stats.TotalSizeBytes += info.Size()
		}

		// 时间统计
		if stats.OldestMemory == nil || mem.CreatedAt.Before(*stats.OldestMemory) {
			t := mem.CreatedAt
			stats.OldestMemory = &t
		}
		if stats.NewestMemory == nil || mem.CreatedAt.After(*stats.NewestMemory) {
			t := mem.CreatedAt
			stats.NewestMemory = &t
		}
	}

	return stats, nil
}
