// Package v2 提供记忆系统 v2 的错误定义
package v2

import (
	"errors"
	"fmt"
)

// 预定义错误
var (
	// 存储相关错误
	ErrMemoryNotFound    = errors.New("记忆不存在")
	ErrSessionNotFound   = errors.New("会话不存在")
	ErrFileNotFound      = errors.New("文件不存在")
	ErrInvalidFilePath   = errors.New("无效的文件路径")
	ErrFileAlreadyExists = errors.New("文件已存在")

	// 解析相关错误
	ErrInvalidFrontmatter  = errors.New("无效的 frontmatter")
	ErrFrontmatterNotFound = errors.New("frontmatter 不存在")
	ErrInvalidMemoryType   = errors.New("无效的记忆类型")
	ErrInvalidMemoryScope  = errors.New("无效的记忆作用域")

	// 索引相关错误
	ErrIndexNotFound   = errors.New("索引记录不存在")
	ErrIndexOutOfSync  = errors.New("索引与文件不同步")
	ErrVectorNotFound  = errors.New("向量记录不存在")
	ErrDatabaseError   = errors.New("数据库操作失败")
	ErrEmbeddingFailed = errors.New("向量嵌入失败")

	// 配置相关错误
	ErrInvalidConfig  = errors.New("无效的配置")
	ErrConfigNotFound = errors.New("配置文件不存在")

	// 上下文相关错误
	ErrTokenLimitExceeded = errors.New("Token 限制超出")
	ErrContextTooLarge    = errors.New("上下文过大")

	// 操作相关错误
	ErrOperationTimeout = errors.New("操作超时")
	ErrOperationFailed  = errors.New("操作失败")
)

// MemoryError 记忆系统错误（包含上下文信息）
type MemoryError struct {
	Op      string // 操作名称
	Path    string // 相关文件路径
	Err     error  // 原始错误
	Details string // 详细信息
}

func (e *MemoryError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("记忆系统错误 [%s] 路径=%s: %v", e.Op, e.Path, e.Err)
	}
	if e.Details != "" {
		return fmt.Sprintf("记忆系统错误 [%s]: %v (%s)", e.Op, e.Err, e.Details)
	}
	return fmt.Sprintf("记忆系统错误 [%s]: %v", e.Op, e.Err)
}

func (e *MemoryError) Unwrap() error {
	return e.Err
}

// NewMemoryError 创建记忆系统错误
func NewMemoryError(op string, err error) *MemoryError {
	return &MemoryError{Op: op, Err: err}
}

// NewMemoryErrorWithPath 创建带路径的记忆系统错误
func NewMemoryErrorWithPath(op, path string, err error) *MemoryError {
	return &MemoryError{Op: op, Path: path, Err: err}
}

// NewMemoryErrorWithDetails 创建带详情的记忆系统错误
func NewMemoryErrorWithDetails(op string, err error, details string) *MemoryError {
	return &MemoryError{Op: op, Err: err, Details: details}
}

// IsNotFound 检查是否是"未找到"类型的错误
func IsNotFound(err error) bool {
	return errors.Is(err, ErrMemoryNotFound) ||
		errors.Is(err, ErrSessionNotFound) ||
		errors.Is(err, ErrFileNotFound) ||
		errors.Is(err, ErrIndexNotFound) ||
		errors.Is(err, ErrVectorNotFound) ||
		errors.Is(err, ErrConfigNotFound)
}

// IsTokenLimit 检查是否是 Token 限制错误
func IsTokenLimit(err error) bool {
	return errors.Is(err, ErrTokenLimitExceeded) ||
		errors.Is(err, ErrContextTooLarge)
}

// IndexSyncError 索引同步错误
type IndexSyncError struct {
	FilePath    string
	IndexID     string
	FileHash    string
	IndexHash   string
	Description string
}

func (e *IndexSyncError) Error() string {
	return fmt.Sprintf("索引同步错误: 文件=%s, 索引ID=%s, 文件哈希=%s, 索引哈希=%s: %s",
		e.FilePath, e.IndexID, e.FileHash, e.IndexHash, e.Description)
}
