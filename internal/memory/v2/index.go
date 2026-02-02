// Package v2 提供 SQLite 元数据索引功能
package v2

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// IndexStore 元数据索引存储接口
type IndexStore interface {
	// 索引 CRUD
	CreateIndex(idx *MemoryIndex) error
	GetIndex(id string) (*MemoryIndex, error)
	GetIndexByPath(filePath string) (*MemoryIndex, error)
	UpdateIndex(idx *MemoryIndex) error
	DeleteIndex(id string) error
	DeleteIndexByPath(filePath string) error

	// 查询方法
	GetRecentMemories(days int, memType MemoryType) ([]*MemoryIndex, error)
	GetExpiredMemories() ([]*MemoryIndex, error)
	GetMemoriesByType(memType MemoryType) ([]*MemoryIndex, error)
	GetMemoriesByScope(scope MemoryScope) ([]*MemoryIndex, error)
	GetMemoriesByCategory(category MemoryCategory) ([]*MemoryIndex, error)
	SearchByKeyword(keyword string, limit int) ([]*MemoryIndex, error)

	// 访问更新
	IncrementAccessCount(id string) error

	// 统计
	GetIndexStats() (*IndexStats, error)

	// 同步检查
	GetAllIndexes() ([]*MemoryIndex, error)
	GetOrphanedIndexes(existingFiles map[string]bool) ([]*MemoryIndex, error)

	// 关闭
	Close() error
}

// MemoryIndex 元数据索引记录
type MemoryIndex struct {
	ID          string         `json:"id"`
	FilePath    string         `json:"file_path"`
	Type        MemoryType     `json:"type"`
	Scope       MemoryScope    `json:"scope"`
	Category    MemoryCategory `json:"category"`
	Title       string         `json:"title"`
	Tags        string         `json:"tags"` // 逗号分隔
	ContentHash string         `json:"content_hash"`
	Importance  int            `json:"importance"`
	AccessCount int            `json:"access_count"`
	TokenCount  int            `json:"token_count"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	AccessedAt  time.Time      `json:"accessed_at"`
}

// IndexStats 索引统计
type IndexStats struct {
	TotalCount     int `json:"total_count"`
	CoreCount      int `json:"core_count"`
	SessionCount   int `json:"session_count"`
	ShortTermCount int `json:"short_term_count"`
	LongTermCount  int `json:"long_term_count"`
	GlobalCount    int `json:"global_count"`
	ProjectCount   int `json:"project_count"`
	ExpiredCount   int `json:"expired_count"`
}

// SQLiteIndexStore SQLite 索引存储实现
type SQLiteIndexStore struct {
	db         *sql.DB
	dbPath     string
	ftsEnabled bool // FTS5 是否启用
}

// NewSQLiteIndexStore 创建 SQLite 索引存储
func NewSQLiteIndexStore(dbPath string) (*SQLiteIndexStore, error) {
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建索引目录失败: %w", err)
	}

	// 打开数据库
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("打开索引数据库失败: %w", err)
	}

	store := &SQLiteIndexStore{
		db:     db,
		dbPath: dbPath,
	}

	// 初始化表结构
	if err := store.initTables(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initTables 初始化数据库表
func (s *SQLiteIndexStore) initTables() error {
	// 基础表和索引（必须成功）
	baseQueries := []string{
		// 记忆索引表
		`CREATE TABLE IF NOT EXISTS memory_index (
			id TEXT PRIMARY KEY,
			file_path TEXT UNIQUE NOT NULL,
			type TEXT NOT NULL,
			scope TEXT NOT NULL,
			category TEXT NOT NULL,
			title TEXT NOT NULL,
			tags TEXT,
			content_hash TEXT NOT NULL,
			importance INTEGER DEFAULT 3,
			access_count INTEGER DEFAULT 0,
			token_count INTEGER DEFAULT 0,
			expires_at DATETIME,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			accessed_at DATETIME NOT NULL
		)`,

		// 索引
		`CREATE INDEX IF NOT EXISTS idx_memory_type ON memory_index(type)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_scope ON memory_index(scope)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_category ON memory_index(category)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_created_at ON memory_index(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_updated_at ON memory_index(updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_expires_at ON memory_index(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_access_count ON memory_index(access_count)`,
		// 为标题和标签添加索引以支持 LIKE 搜索
		`CREATE INDEX IF NOT EXISTS idx_memory_title ON memory_index(title)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_tags ON memory_index(tags)`,
	}

	// 执行基础查询
	for _, query := range baseQueries {
		if _, err := s.db.Exec(query); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("执行初始化 SQL 失败: %w", err)
			}
		}
	}

	// FTS5 相关（可选，失败时降级到 LIKE 搜索）
	ftsQueries := []string{
		// 全文搜索虚拟表
		`CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
			id,
			title,
			tags,
			content='memory_index',
			content_rowid='rowid'
		)`,

		// 触发器：同步更新 FTS
		`CREATE TRIGGER IF NOT EXISTS memory_ai AFTER INSERT ON memory_index BEGIN
			INSERT INTO memory_fts(rowid, id, title, tags) VALUES (NEW.rowid, NEW.id, NEW.title, NEW.tags);
		END`,

		`CREATE TRIGGER IF NOT EXISTS memory_ad AFTER DELETE ON memory_index BEGIN
			INSERT INTO memory_fts(memory_fts, rowid, id, title, tags) VALUES('delete', OLD.rowid, OLD.id, OLD.title, OLD.tags);
		END`,

		`CREATE TRIGGER IF NOT EXISTS memory_au AFTER UPDATE ON memory_index BEGIN
			INSERT INTO memory_fts(memory_fts, rowid, id, title, tags) VALUES('delete', OLD.rowid, OLD.id, OLD.title, OLD.tags);
			INSERT INTO memory_fts(rowid, id, title, tags) VALUES (NEW.rowid, NEW.id, NEW.title, NEW.tags);
		END`,
	}

	// 尝试启用 FTS5（失败时静默降级）
	s.ftsEnabled = true
	for _, query := range ftsQueries {
		if _, err := s.db.Exec(query); err != nil {
			if strings.Contains(err.Error(), "no such module") {
				// FTS5 不可用，使用 LIKE 搜索作为降级方案
				s.ftsEnabled = false
				break
			}
			if !strings.Contains(err.Error(), "already exists") {
				// 其他错误也降级
				s.ftsEnabled = false
				break
			}
		}
	}

	return nil
}

// CreateIndex 创建索引记录
func (s *SQLiteIndexStore) CreateIndex(idx *MemoryIndex) error {
	query := `INSERT INTO memory_index 
		(id, file_path, type, scope, category, title, tags, content_hash, 
		 importance, access_count, token_count, expires_at, created_at, updated_at, accessed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.Exec(query,
		idx.ID, idx.FilePath, idx.Type, idx.Scope, idx.Category, idx.Title, idx.Tags, idx.ContentHash,
		idx.Importance, idx.AccessCount, idx.TokenCount, idx.ExpiresAt, idx.CreatedAt, idx.UpdatedAt, idx.AccessedAt,
	)
	if err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	return nil
}

// GetIndex 通过 ID 获取索引
func (s *SQLiteIndexStore) GetIndex(id string) (*MemoryIndex, error) {
	query := `SELECT id, file_path, type, scope, category, title, tags, content_hash,
		importance, access_count, token_count, expires_at, created_at, updated_at, accessed_at
		FROM memory_index WHERE id = ?`

	idx := &MemoryIndex{}
	var expiresAt sql.NullTime

	err := s.db.QueryRow(query, id).Scan(
		&idx.ID, &idx.FilePath, &idx.Type, &idx.Scope, &idx.Category, &idx.Title, &idx.Tags, &idx.ContentHash,
		&idx.Importance, &idx.AccessCount, &idx.TokenCount, &expiresAt, &idx.CreatedAt, &idx.UpdatedAt, &idx.AccessedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrIndexNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("获取索引失败: %w", err)
	}

	if expiresAt.Valid {
		idx.ExpiresAt = &expiresAt.Time
	}

	return idx, nil
}

// GetIndexByPath 通过文件路径获取索引
func (s *SQLiteIndexStore) GetIndexByPath(filePath string) (*MemoryIndex, error) {
	query := `SELECT id, file_path, type, scope, category, title, tags, content_hash,
		importance, access_count, token_count, expires_at, created_at, updated_at, accessed_at
		FROM memory_index WHERE file_path = ?`

	idx := &MemoryIndex{}
	var expiresAt sql.NullTime

	err := s.db.QueryRow(query, filePath).Scan(
		&idx.ID, &idx.FilePath, &idx.Type, &idx.Scope, &idx.Category, &idx.Title, &idx.Tags, &idx.ContentHash,
		&idx.Importance, &idx.AccessCount, &idx.TokenCount, &expiresAt, &idx.CreatedAt, &idx.UpdatedAt, &idx.AccessedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrIndexNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("获取索引失败: %w", err)
	}

	if expiresAt.Valid {
		idx.ExpiresAt = &expiresAt.Time
	}

	return idx, nil
}

// UpdateIndex 更新索引记录
func (s *SQLiteIndexStore) UpdateIndex(idx *MemoryIndex) error {
	query := `UPDATE memory_index SET 
		file_path = ?, type = ?, scope = ?, category = ?, title = ?, tags = ?, 
		content_hash = ?, importance = ?, access_count = ?, token_count = ?,
		expires_at = ?, updated_at = ?, accessed_at = ?
		WHERE id = ?`

	result, err := s.db.Exec(query,
		idx.FilePath, idx.Type, idx.Scope, idx.Category, idx.Title, idx.Tags,
		idx.ContentHash, idx.Importance, idx.AccessCount, idx.TokenCount,
		idx.ExpiresAt, idx.UpdatedAt, idx.AccessedAt, idx.ID,
	)
	if err != nil {
		return fmt.Errorf("更新索引失败: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取更新行数失败: %w", err)
	}
	if rows == 0 {
		return ErrIndexNotFound
	}

	return nil
}

// DeleteIndex 删除索引记录
func (s *SQLiteIndexStore) DeleteIndex(id string) error {
	result, err := s.db.Exec("DELETE FROM memory_index WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("删除索引失败: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取删除行数失败: %w", err)
	}
	if rows == 0 {
		return ErrIndexNotFound
	}

	return nil
}

// DeleteIndexByPath 通过文件路径删除索引
func (s *SQLiteIndexStore) DeleteIndexByPath(filePath string) error {
	result, err := s.db.Exec("DELETE FROM memory_index WHERE file_path = ?", filePath)
	if err != nil {
		return fmt.Errorf("删除索引失败: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取删除行数失败: %w", err)
	}
	if rows == 0 {
		return ErrIndexNotFound
	}

	return nil
}

// GetRecentMemories 获取最近 N 天的记忆
func (s *SQLiteIndexStore) GetRecentMemories(days int, memType MemoryType) ([]*MemoryIndex, error) {
	since := time.Now().AddDate(0, 0, -days)

	var query string
	var args []interface{}

	if memType != "" {
		query = `SELECT id, file_path, type, scope, category, title, tags, content_hash,
			importance, access_count, token_count, expires_at, created_at, updated_at, accessed_at
			FROM memory_index 
			WHERE created_at >= ? AND type = ?
			ORDER BY created_at DESC`
		args = []interface{}{since, memType}
	} else {
		query = `SELECT id, file_path, type, scope, category, title, tags, content_hash,
			importance, access_count, token_count, expires_at, created_at, updated_at, accessed_at
			FROM memory_index 
			WHERE created_at >= ?
			ORDER BY created_at DESC`
		args = []interface{}{since}
	}

	return s.queryIndexes(query, args...)
}

// GetExpiredMemories 获取已过期的记忆
func (s *SQLiteIndexStore) GetExpiredMemories() ([]*MemoryIndex, error) {
	query := `SELECT id, file_path, type, scope, category, title, tags, content_hash,
		importance, access_count, token_count, expires_at, created_at, updated_at, accessed_at
		FROM memory_index 
		WHERE expires_at IS NOT NULL AND expires_at <= ?
		ORDER BY expires_at ASC`

	return s.queryIndexes(query, time.Now())
}

// GetMemoriesByType 按类型获取记忆
func (s *SQLiteIndexStore) GetMemoriesByType(memType MemoryType) ([]*MemoryIndex, error) {
	query := `SELECT id, file_path, type, scope, category, title, tags, content_hash,
		importance, access_count, token_count, expires_at, created_at, updated_at, accessed_at
		FROM memory_index 
		WHERE type = ?
		ORDER BY updated_at DESC`

	return s.queryIndexes(query, memType)
}

// GetMemoriesByScope 按作用域获取记忆
func (s *SQLiteIndexStore) GetMemoriesByScope(scope MemoryScope) ([]*MemoryIndex, error) {
	query := `SELECT id, file_path, type, scope, category, title, tags, content_hash,
		importance, access_count, token_count, expires_at, created_at, updated_at, accessed_at
		FROM memory_index 
		WHERE scope = ?
		ORDER BY updated_at DESC`

	return s.queryIndexes(query, scope)
}

// GetMemoriesByCategory 按分类获取记忆
func (s *SQLiteIndexStore) GetMemoriesByCategory(category MemoryCategory) ([]*MemoryIndex, error) {
	query := `SELECT id, file_path, type, scope, category, title, tags, content_hash,
		importance, access_count, token_count, expires_at, created_at, updated_at, accessed_at
		FROM memory_index 
		WHERE category = ?
		ORDER BY updated_at DESC`

	return s.queryIndexes(query, category)
}

// SearchByKeyword 关键词搜索
func (s *SQLiteIndexStore) SearchByKeyword(keyword string, limit int) ([]*MemoryIndex, error) {
	if s.ftsEnabled {
		// 使用 FTS5 全文搜索
		query := `SELECT m.id, m.file_path, m.type, m.scope, m.category, m.title, m.tags, m.content_hash,
			m.importance, m.access_count, m.token_count, m.expires_at, m.created_at, m.updated_at, m.accessed_at
			FROM memory_index m
			JOIN memory_fts f ON m.id = f.id
			WHERE memory_fts MATCH ?
			ORDER BY rank
			LIMIT ?`

		// FTS5 查询语法
		ftsQuery := fmt.Sprintf("*%s*", keyword)
		return s.queryIndexes(query, ftsQuery, limit)
	}

	// 降级：使用 LIKE 搜索
	likePattern := "%" + keyword + "%"
	query := `SELECT id, file_path, type, scope, category, title, tags, content_hash,
		importance, access_count, token_count, expires_at, created_at, updated_at, accessed_at
		FROM memory_index
		WHERE title LIKE ? OR tags LIKE ?
		ORDER BY updated_at DESC
		LIMIT ?`

	return s.queryIndexes(query, likePattern, likePattern, limit)
}

// IncrementAccessCount 增加访问计数
func (s *SQLiteIndexStore) IncrementAccessCount(id string) error {
	query := `UPDATE memory_index SET 
		access_count = access_count + 1,
		accessed_at = ?
		WHERE id = ?`

	result, err := s.db.Exec(query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("更新访问计数失败: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取更新行数失败: %w", err)
	}
	if rows == 0 {
		return ErrIndexNotFound
	}

	return nil
}

// GetIndexStats 获取索引统计
func (s *SQLiteIndexStore) GetIndexStats() (*IndexStats, error) {
	stats := &IndexStats{}

	// 总数
	err := s.db.QueryRow("SELECT COUNT(*) FROM memory_index").Scan(&stats.TotalCount)
	if err != nil {
		return nil, fmt.Errorf("获取总数失败: %w", err)
	}

	// 按类型统计
	rows, err := s.db.Query("SELECT type, COUNT(*) FROM memory_index GROUP BY type")
	if err != nil {
		return nil, fmt.Errorf("按类型统计失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var memType string
		var count int
		if err := rows.Scan(&memType, &count); err != nil {
			continue
		}
		switch MemoryType(memType) {
		case MemoryTypeCore:
			stats.CoreCount = count
		case MemoryTypeSession:
			stats.SessionCount = count
		case MemoryTypeShortTerm:
			stats.ShortTermCount = count
		case MemoryTypeLongTerm:
			stats.LongTermCount = count
		}
	}

	// 按作用域统计
	rows2, err := s.db.Query("SELECT scope, COUNT(*) FROM memory_index GROUP BY scope")
	if err != nil {
		return nil, fmt.Errorf("按作用域统计失败: %w", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var scope string
		var count int
		if err := rows2.Scan(&scope, &count); err != nil {
			continue
		}
		switch MemoryScope(scope) {
		case ScopeGlobal:
			stats.GlobalCount = count
		case ScopeProject:
			stats.ProjectCount = count
		}
	}

	// 过期数量
	err = s.db.QueryRow(
		"SELECT COUNT(*) FROM memory_index WHERE expires_at IS NOT NULL AND expires_at <= ?",
		time.Now(),
	).Scan(&stats.ExpiredCount)
	if err != nil {
		return nil, fmt.Errorf("获取过期数量失败: %w", err)
	}

	return stats, nil
}

// GetAllIndexes 获取所有索引
func (s *SQLiteIndexStore) GetAllIndexes() ([]*MemoryIndex, error) {
	query := `SELECT id, file_path, type, scope, category, title, tags, content_hash,
		importance, access_count, token_count, expires_at, created_at, updated_at, accessed_at
		FROM memory_index
		ORDER BY created_at DESC`

	return s.queryIndexes(query)
}

// GetOrphanedIndexes 获取孤立索引（文件不存在）
func (s *SQLiteIndexStore) GetOrphanedIndexes(existingFiles map[string]bool) ([]*MemoryIndex, error) {
	allIndexes, err := s.GetAllIndexes()
	if err != nil {
		return nil, err
	}

	var orphaned []*MemoryIndex
	for _, idx := range allIndexes {
		if !existingFiles[idx.FilePath] {
			orphaned = append(orphaned, idx)
		}
	}

	return orphaned, nil
}

// queryIndexes 查询索引列表的通用方法
func (s *SQLiteIndexStore) queryIndexes(query string, args ...interface{}) ([]*MemoryIndex, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询索引失败: %w", err)
	}
	defer rows.Close()

	var indexes []*MemoryIndex
	for rows.Next() {
		idx := &MemoryIndex{}
		var expiresAt sql.NullTime

		if err := rows.Scan(
			&idx.ID, &idx.FilePath, &idx.Type, &idx.Scope, &idx.Category, &idx.Title, &idx.Tags, &idx.ContentHash,
			&idx.Importance, &idx.AccessCount, &idx.TokenCount, &expiresAt, &idx.CreatedAt, &idx.UpdatedAt, &idx.AccessedAt,
		); err != nil {
			continue
		}

		if expiresAt.Valid {
			idx.ExpiresAt = &expiresAt.Time
		}

		indexes = append(indexes, idx)
	}

	return indexes, nil
}

// Close 关闭数据库连接
func (s *SQLiteIndexStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// MemoryToIndex 将 Memory 转换为 MemoryIndex
func MemoryToIndex(mem *Memory) *MemoryIndex {
	return &MemoryIndex{
		ID:          mem.ID,
		FilePath:    mem.FilePath,
		Type:        mem.Type,
		Scope:       mem.Scope,
		Category:    mem.Category,
		Title:       mem.Title,
		Tags:        strings.Join(mem.Tags, ","),
		ContentHash: mem.ContentHash,
		Importance:  mem.Importance,
		AccessCount: mem.AccessCount,
		ExpiresAt:   mem.ExpiresAt,
		CreatedAt:   mem.CreatedAt,
		UpdatedAt:   mem.UpdatedAt,
		AccessedAt:  mem.AccessedAt,
	}
}

// IndexToMemoryPartial 将 MemoryIndex 转换为部分 Memory（不含 Content）
func IndexToMemoryPartial(idx *MemoryIndex) *Memory {
	var tags []string
	if idx.Tags != "" {
		tags = strings.Split(idx.Tags, ",")
	}

	return &Memory{
		ID:          idx.ID,
		FilePath:    idx.FilePath,
		Type:        idx.Type,
		Scope:       idx.Scope,
		Category:    idx.Category,
		Title:       idx.Title,
		Tags:        tags,
		ContentHash: idx.ContentHash,
		Importance:  idx.Importance,
		AccessCount: idx.AccessCount,
		ExpiresAt:   idx.ExpiresAt,
		CreatedAt:   idx.CreatedAt,
		UpdatedAt:   idx.UpdatedAt,
		AccessedAt:  idx.AccessedAt,
	}
}
