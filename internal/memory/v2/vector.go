// Package v2 提供向量存储功能
package v2

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// VectorStore 向量存储接口
type VectorStore interface {
	// 向量操作
	StoreVector(id string, vector []float32) error
	GetVector(id string) ([]float32, error)
	DeleteVector(id string) error
	UpdateVector(id string, vector []float32) error

	// 相似度搜索
	SearchSimilar(queryVector []float32, topK int, minSimilarity float64) ([]*VectorSearchResult, error)

	// 批量操作
	BatchStoreVectors(items map[string][]float32) error

	// 统计
	GetVectorCount() (int, error)
	GetVectorStats() (*VectorStats, error)

	// 关闭
	Close() error
}

// VectorSearchResult 向量搜索结果
type VectorSearchResult struct {
	ID       string  `json:"id"`
	Score    float64 `json:"score"` // 余弦相似度
	Distance float64 `json:"distance"`
}

// VectorStats 向量统计
type VectorStats struct {
	TotalVectors int `json:"total_vectors"`
	Dimension    int `json:"dimension"`
}

// SQLiteVectorStore 基于 SQLite 的向量存储实现
// 使用 BLOB 存储向量，并在查询时计算余弦相似度
// 这是一个简化实现，适用于小规模数据（< 10000 条）
// 对于大规模数据，建议使用 sqlite-vec 扩展或外部向量数据库
type SQLiteVectorStore struct {
	db        *sql.DB
	dbPath    string
	dimension int
}

// NewSQLiteVectorStore 创建 SQLite 向量存储
func NewSQLiteVectorStore(dbPath string, dimension int) (*SQLiteVectorStore, error) {
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建向量存储目录失败: %w", err)
	}

	// 打开数据库
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("打开向量数据库失败: %w", err)
	}

	store := &SQLiteVectorStore{
		db:        db,
		dbPath:    dbPath,
		dimension: dimension,
	}

	// 初始化表结构
	if err := store.initTables(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initTables 初始化数据库表
func (s *SQLiteVectorStore) initTables() error {
	queries := []string{
		// 向量表
		`CREATE TABLE IF NOT EXISTS memory_vectors (
			id TEXT PRIMARY KEY,
			vector BLOB NOT NULL,
			dimension INTEGER NOT NULL,
			norm REAL NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,

		// 索引
		`CREATE INDEX IF NOT EXISTS idx_vectors_created_at ON memory_vectors(created_at)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("初始化向量表失败: %w", err)
		}
	}

	return nil
}

// StoreVector 存储向量
func (s *SQLiteVectorStore) StoreVector(id string, vector []float32) error {
	if len(vector) != s.dimension {
		return fmt.Errorf("向量维度不匹配: 期望 %d, 实际 %d", s.dimension, len(vector))
	}

	// 计算 L2 范数
	norm := calculateNorm(vector)

	// 将向量序列化为 BLOB
	blob := vectorToBlob(vector)

	now := time.Now()

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO memory_vectors (id, vector, dimension, norm, created_at, updated_at) 
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, blob, len(vector), norm, now, now,
	)
	if err != nil {
		return fmt.Errorf("存储向量失败: %w", err)
	}

	return nil
}

// GetVector 获取向量
func (s *SQLiteVectorStore) GetVector(id string) ([]float32, error) {
	var blob []byte
	err := s.db.QueryRow("SELECT vector FROM memory_vectors WHERE id = ?", id).Scan(&blob)
	if err == sql.ErrNoRows {
		return nil, ErrVectorNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("获取向量失败: %w", err)
	}

	return blobToVector(blob), nil
}

// DeleteVector 删除向量
func (s *SQLiteVectorStore) DeleteVector(id string) error {
	result, err := s.db.Exec("DELETE FROM memory_vectors WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("删除向量失败: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取删除行数失败: %w", err)
	}
	if rows == 0 {
		return ErrVectorNotFound
	}

	return nil
}

// UpdateVector 更新向量
func (s *SQLiteVectorStore) UpdateVector(id string, vector []float32) error {
	if len(vector) != s.dimension {
		return fmt.Errorf("向量维度不匹配: 期望 %d, 实际 %d", s.dimension, len(vector))
	}

	norm := calculateNorm(vector)
	blob := vectorToBlob(vector)

	result, err := s.db.Exec(
		`UPDATE memory_vectors SET vector = ?, norm = ?, updated_at = ? WHERE id = ?`,
		blob, norm, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("更新向量失败: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取更新行数失败: %w", err)
	}
	if rows == 0 {
		return ErrVectorNotFound
	}

	return nil
}

// SearchSimilar 相似度搜索
// 使用余弦相似度进行排序
func (s *SQLiteVectorStore) SearchSimilar(queryVector []float32, topK int, minSimilarity float64) ([]*VectorSearchResult, error) {
	if len(queryVector) != s.dimension {
		return nil, fmt.Errorf("查询向量维度不匹配: 期望 %d, 实际 %d", s.dimension, len(queryVector))
	}

	queryNorm := calculateNorm(queryVector)
	if queryNorm == 0 {
		return nil, fmt.Errorf("查询向量范数为 0")
	}

	// 获取所有向量（小规模数据适用）
	rows, err := s.db.Query("SELECT id, vector, norm FROM memory_vectors")
	if err != nil {
		return nil, fmt.Errorf("查询向量失败: %w", err)
	}
	defer rows.Close()

	var results []*VectorSearchResult

	for rows.Next() {
		var id string
		var blob []byte
		var norm float64

		if err := rows.Scan(&id, &blob, &norm); err != nil {
			continue
		}

		if norm == 0 {
			continue
		}

		vector := blobToVector(blob)

		// 计算余弦相似度
		dotProduct := calculateDotProduct(queryVector, vector)
		similarity := dotProduct / (queryNorm * float64(norm))

		if similarity >= minSimilarity {
			results = append(results, &VectorSearchResult{
				ID:       id,
				Score:    similarity,
				Distance: 1 - similarity, // 余弦距离
			})
		}
	}

	// 按相似度降序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 取 Top-K
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// BatchStoreVectors 批量存储向量
func (s *SQLiteVectorStore) BatchStoreVectors(items map[string][]float32) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT OR REPLACE INTO memory_vectors (id, vector, dimension, norm, created_at, updated_at) 
		 VALUES (?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("准备语句失败: %w", err)
	}
	defer stmt.Close()

	now := time.Now()

	for id, vector := range items {
		if len(vector) != s.dimension {
			continue
		}

		norm := calculateNorm(vector)
		blob := vectorToBlob(vector)

		if _, err := stmt.Exec(id, blob, len(vector), norm, now, now); err != nil {
			return fmt.Errorf("批量存储向量失败: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}

// GetVectorCount 获取向量数量
func (s *SQLiteVectorStore) GetVectorCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM memory_vectors").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("获取向量数量失败: %w", err)
	}
	return count, nil
}

// GetVectorStats 获取向量统计
func (s *SQLiteVectorStore) GetVectorStats() (*VectorStats, error) {
	count, err := s.GetVectorCount()
	if err != nil {
		return nil, err
	}

	return &VectorStats{
		TotalVectors: count,
		Dimension:    s.dimension,
	}, nil
}

// Close 关闭数据库连接
func (s *SQLiteVectorStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// ========== 工具函数 ==========

// vectorToBlob 将 float32 切片转换为 BLOB
func vectorToBlob(vector []float32) []byte {
	blob := make([]byte, len(vector)*4)
	for i, v := range vector {
		binary.LittleEndian.PutUint32(blob[i*4:], math.Float32bits(v))
	}
	return blob
}

// blobToVector 将 BLOB 转换为 float32 切片
func blobToVector(blob []byte) []float32 {
	vector := make([]float32, len(blob)/4)
	for i := range vector {
		bits := binary.LittleEndian.Uint32(blob[i*4:])
		vector[i] = math.Float32frombits(bits)
	}
	return vector
}

// calculateNorm 计算 L2 范数
func calculateNorm(vector []float32) float64 {
	var sum float64
	for _, v := range vector {
		sum += float64(v) * float64(v)
	}
	return math.Sqrt(sum)
}

// calculateDotProduct 计算点积
func calculateDotProduct(a []float32, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var sum float64
	for i := range a {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}

// VectorJSON 向量 JSON 序列化（用于调试）
func VectorJSON(vector []float32) string {
	data, _ := json.Marshal(vector)
	return string(data)
}

// NormalizeVector 归一化向量
func NormalizeVector(vector []float32) []float32 {
	norm := calculateNorm(vector)
	if norm == 0 {
		return vector
	}

	normalized := make([]float32, len(vector))
	for i, v := range vector {
		normalized[i] = float32(float64(v) / norm)
	}
	return normalized
}

// CosineSimilarity 计算余弦相似度
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	dotProduct := calculateDotProduct(a, b)
	normA := calculateNorm(a)
	normB := calculateNorm(b)

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (normA * normB)
}

// EuclideanDistance 计算欧氏距离
func EuclideanDistance(a, b []float32) float64 {
	if len(a) != len(b) {
		return math.MaxFloat64
	}

	var sum float64
	for i := range a {
		diff := float64(a[i]) - float64(b[i])
		sum += diff * diff
	}
	return math.Sqrt(sum)
}
