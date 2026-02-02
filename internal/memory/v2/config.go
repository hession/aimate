// Package v2 提供记忆系统 v2 配置管理
package v2

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// MemoryConfig 记忆系统总配置
type MemoryConfig struct {
	// 版本号
	Version string `yaml:"version"`

	// 存储配置
	Storage StorageConfig `yaml:"storage"`

	// 各层记忆配置
	Core      CoreMemoryConfig      `yaml:"core"`
	Session   SessionMemoryConfig   `yaml:"session"`
	ShortTerm ShortTermMemoryConfig `yaml:"short_term"`
	LongTerm  LongTermMemoryConfig  `yaml:"long_term"`

	// 检索配置
	Retrieval RetrievalConfig `yaml:"retrieval"`

	// 嵌入模型配置
	Embedding EmbeddingConfig `yaml:"embedding"`

	// 自治管理配置
	Maintenance MaintenanceConfig `yaml:"maintenance"`

	// 上下文构建配置
	Context ContextConfig `yaml:"context"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	// 全局存储根目录（默认：~/.aimate/memory）
	GlobalRoot string `yaml:"global_root"`

	// 项目存储目录名（默认：.aimate/memory）
	ProjectDirName string `yaml:"project_dir_name"`

	// 索引数据库文件名（默认：index.db）
	IndexDBName string `yaml:"index_db_name"`

	// 项目根目录标记文件
	ProjectMarkers []string `yaml:"project_markers"`
}

// CoreMemoryConfig 核心记忆配置
type CoreMemoryConfig struct {
	// 是否启用
	Enabled bool `yaml:"enabled"`

	// 最大 Token 数
	MaxTokens int `yaml:"max_tokens"`

	// 自动精炼阈值（超过此比例触发压缩）
	RefineThreshold float64 `yaml:"refine_threshold"`
}

// SessionMemoryConfig 会话记忆配置
type SessionMemoryConfig struct {
	// 是否启用
	Enabled bool `yaml:"enabled"`

	// 最大 Token 数
	MaxTokens int `yaml:"max_tokens"`

	// 警告阈值（占比）
	WarningThresholds []float64 `yaml:"warning_thresholds"`

	// 裁剪时保护的最近 N 轮对话
	ProtectedRounds int `yaml:"protected_rounds"`

	// 归档保留天数
	ArchiveRetentionDays int `yaml:"archive_retention_days"`
}

// ShortTermMemoryConfig 短期记忆配置
type ShortTermMemoryConfig struct {
	// 是否启用
	Enabled bool `yaml:"enabled"`

	// 默认 TTL（天）
	DefaultTTLDays int `yaml:"default_ttl_days"`

	// 各分类的 TTL 配置（天）
	CategoryTTL map[string]int `yaml:"category_ttl"`

	// 自动提升阈值（访问次数）
	PromoteThreshold int `yaml:"promote_threshold"`

	// 最大文件数
	MaxFiles int `yaml:"max_files"`
}

// LongTermMemoryConfig 长期记忆配置
type LongTermMemoryConfig struct {
	// 是否启用
	Enabled bool `yaml:"enabled"`

	// 相似度压缩阈值
	CompressionSimilarity float64 `yaml:"compression_similarity"`

	// 未访问归档天数
	InactiveArchiveDays int `yaml:"inactive_archive_days"`

	// 最大文件数
	MaxFiles int `yaml:"max_files"`
}

// RetrievalConfig 检索配置
type RetrievalConfig struct {
	// 向量检索 Top-K
	VectorTopK int `yaml:"vector_top_k"`

	// 关键词检索 Top-K
	KeywordTopK int `yaml:"keyword_top_k"`

	// 最终返回 Top-K
	FinalTopK int `yaml:"final_top_k"`

	// 最小相似度阈值
	MinSimilarity float64 `yaml:"min_similarity"`

	// 时间衰减因子
	TimeDecayFactor float64 `yaml:"time_decay_factor"`

	// 检索超时（毫秒）
	TimeoutMs int `yaml:"timeout_ms"`
}

// EmbeddingConfig 嵌入模型配置
type EmbeddingConfig struct {
	// 是否启用向量检索
	Enabled bool `yaml:"enabled"`

	// API 提供商：deepseek/openai
	Provider string `yaml:"provider"`

	// API Base URL
	BaseURL string `yaml:"base_url"`

	// 模型名称
	Model string `yaml:"model"`

	// 向量维度
	Dimension int `yaml:"dimension"`

	// 批量大小
	BatchSize int `yaml:"batch_size"`

	// 请求超时（秒）
	TimeoutSec int `yaml:"timeout_sec"`

	// 重试次数
	MaxRetries int `yaml:"max_retries"`
}

// MaintenanceConfig 自治管理配置
type MaintenanceConfig struct {
	// 是否启用自动维护
	Enabled bool `yaml:"enabled"`

	// 维护检查间隔（分钟）
	IntervalMinutes int `yaml:"interval_minutes"`

	// 过期清理是否启用
	CleanupExpired bool `yaml:"cleanup_expired"`

	// 索引同步是否启用
	SyncIndex bool `yaml:"sync_index"`

	// 记忆压缩是否启用
	CompressMemories bool `yaml:"compress_memories"`

	// 反思任务是否启用
	ReflectionEnabled bool `yaml:"reflection_enabled"`

	// 反思间隔（小时）
	ReflectionIntervalHours int `yaml:"reflection_interval_hours"`
}

// ContextConfig 上下文构建配置
type ContextConfig struct {
	// 总 Token 预算
	TotalBudget int `yaml:"total_budget"`

	// 各层预算分配比例
	CoreRatio      float64 `yaml:"core_ratio"`
	SessionRatio   float64 `yaml:"session_ratio"`
	ShortTermRatio float64 `yaml:"short_term_ratio"`
	LongTermRatio  float64 `yaml:"long_term_ratio"`
	ReservedRatio  float64 `yaml:"reserved_ratio"`

	// 是否启用跨项目检索
	CrossProjectSearch bool `yaml:"cross_project_search"`
}

// DefaultMemoryConfig 返回默认配置
func DefaultMemoryConfig() *MemoryConfig {
	homeDir, _ := os.UserHomeDir()

	return &MemoryConfig{
		Version: "2.0",
		Storage: StorageConfig{
			GlobalRoot:     filepath.Join(homeDir, ".aimate", "memory"),
			ProjectDirName: ".aimate/memory",
			IndexDBName:    "index.db",
			ProjectMarkers: []string{".git", ".aimate", "go.mod", "package.json", "Cargo.toml", "pom.xml"},
		},
		Core: CoreMemoryConfig{
			Enabled:         true,
			MaxTokens:       2000,
			RefineThreshold: 0.9,
		},
		Session: SessionMemoryConfig{
			Enabled:              true,
			MaxTokens:            32000,
			WarningThresholds:    []float64{0.7, 0.85},
			ProtectedRounds:      3,
			ArchiveRetentionDays: 30,
		},
		ShortTerm: ShortTermMemoryConfig{
			Enabled:          true,
			DefaultTTLDays:   7,
			CategoryTTL:      map[string]int{"task": 3, "note": 7, "context": 14},
			PromoteThreshold: 5,
			MaxFiles:         100,
		},
		LongTerm: LongTermMemoryConfig{
			Enabled:               true,
			CompressionSimilarity: 0.85,
			InactiveArchiveDays:   90,
			MaxFiles:              500,
		},
		Retrieval: RetrievalConfig{
			VectorTopK:      20,
			KeywordTopK:     10,
			FinalTopK:       5,
			MinSimilarity:   0.6,
			TimeDecayFactor: 0.95,
			TimeoutMs:       500,
		},
		Embedding: EmbeddingConfig{
			Enabled:    true,
			Provider:   "deepseek",
			BaseURL:    "https://api.deepseek.com",
			Model:      "deepseek-embed",
			Dimension:  1536,
			BatchSize:  10,
			TimeoutSec: 30,
			MaxRetries: 3,
		},
		Maintenance: MaintenanceConfig{
			Enabled:                 true,
			IntervalMinutes:         60,
			CleanupExpired:          true,
			SyncIndex:               true,
			CompressMemories:        true,
			ReflectionEnabled:       true,
			ReflectionIntervalHours: 24,
		},
		Context: ContextConfig{
			TotalBudget:        128000,
			CoreRatio:          0.05,
			SessionRatio:       0.50,
			ShortTermRatio:     0.15,
			LongTermRatio:      0.20,
			ReservedRatio:      0.10,
			CrossProjectSearch: false,
		},
	}
}

// ConfigManager 配置管理器
type ConfigManager struct {
	// 全局配置
	globalConfig *MemoryConfig

	// 项目配置缓存（项目路径 -> 配置）
	projectConfigs map[string]*MemoryConfig

	// 全局配置文件路径
	globalConfigPath string
}

// NewConfigManager 创建配置管理器
func NewConfigManager() (*ConfigManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户主目录失败: %w", err)
	}

	cm := &ConfigManager{
		projectConfigs:   make(map[string]*MemoryConfig),
		globalConfigPath: filepath.Join(homeDir, ".aimate", "memory", "config.yaml"),
	}

	// 加载全局配置
	if err := cm.loadGlobalConfig(); err != nil {
		return nil, err
	}

	return cm, nil
}

// loadGlobalConfig 加载全局配置
func (cm *ConfigManager) loadGlobalConfig() error {
	// 检查配置文件是否存在
	if _, err := os.Stat(cm.globalConfigPath); os.IsNotExist(err) {
		// 使用默认配置
		cm.globalConfig = DefaultMemoryConfig()
		// 保存默认配置
		return cm.saveGlobalConfig()
	}

	// 读取配置文件
	data, err := os.ReadFile(cm.globalConfigPath)
	if err != nil {
		return fmt.Errorf("读取全局配置失败: %w", err)
	}

	// 解析配置
	cfg := DefaultMemoryConfig() // 使用默认值作为基础
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("解析全局配置失败: %w", err)
	}

	cm.globalConfig = cfg
	return nil
}

// saveGlobalConfig 保存全局配置
func (cm *ConfigManager) saveGlobalConfig() error {
	// 确保目录存在
	dir := filepath.Dir(cm.globalConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	// 序列化配置
	data, err := yaml.Marshal(cm.globalConfig)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 添加注释头
	content := fmt.Sprintf("# AIMate 记忆系统配置文件\n# 生成时间: %s\n# 文档: https://github.com/hession/aimate\n\n%s",
		time.Now().Format(time.RFC3339), string(data))

	// 写入文件
	if err := os.WriteFile(cm.globalConfigPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// GetGlobalConfig 获取全局配置
func (cm *ConfigManager) GetGlobalConfig() *MemoryConfig {
	return cm.globalConfig
}

// GetProjectConfig 获取项目配置（合并全局配置和项目配置）
func (cm *ConfigManager) GetProjectConfig(projectPath string) (*MemoryConfig, error) {
	// 检查缓存
	if cfg, ok := cm.projectConfigs[projectPath]; ok {
		return cfg, nil
	}

	// 构造项目配置文件路径
	projectConfigPath := filepath.Join(projectPath, cm.globalConfig.Storage.ProjectDirName, "config.yaml")

	// 从全局配置复制一份作为基础
	cfg := *cm.globalConfig

	// 检查项目配置是否存在
	if _, err := os.Stat(projectConfigPath); err == nil {
		// 读取项目配置
		data, err := os.ReadFile(projectConfigPath)
		if err != nil {
			return nil, fmt.Errorf("读取项目配置失败: %w", err)
		}

		// 解析并合并配置
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("解析项目配置失败: %w", err)
		}
	}

	// 缓存配置
	cm.projectConfigs[projectPath] = &cfg
	return &cfg, nil
}

// SaveProjectConfig 保存项目配置
func (cm *ConfigManager) SaveProjectConfig(projectPath string, cfg *MemoryConfig) error {
	// 构造项目配置文件路径
	projectConfigPath := filepath.Join(projectPath, cm.globalConfig.Storage.ProjectDirName, "config.yaml")

	// 确保目录存在
	dir := filepath.Dir(projectConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建项目配置目录失败: %w", err)
	}

	// 序列化配置
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化项目配置失败: %w", err)
	}

	// 添加注释头
	content := fmt.Sprintf("# AIMate 项目记忆配置\n# 项目路径: %s\n# 生成时间: %s\n\n%s",
		projectPath, time.Now().Format(time.RFC3339), string(data))

	// 写入文件
	if err := os.WriteFile(projectConfigPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入项目配置失败: %w", err)
	}

	// 更新缓存
	cm.projectConfigs[projectPath] = cfg

	return nil
}

// ReloadConfig 重新加载配置
func (cm *ConfigManager) ReloadConfig() error {
	// 清除项目配置缓存
	cm.projectConfigs = make(map[string]*MemoryConfig)

	// 重新加载全局配置
	return cm.loadGlobalConfig()
}

// GetContextBudget 根据配置计算上下文预算分配
func (cm *ConfigManager) GetContextBudget(cfg *MemoryConfig) *ContextBudget {
	total := cfg.Context.TotalBudget
	return &ContextBudget{
		Total:     total,
		Core:      int(float64(total) * cfg.Context.CoreRatio),
		Session:   int(float64(total) * cfg.Context.SessionRatio),
		ShortTerm: int(float64(total) * cfg.Context.ShortTermRatio),
		LongTerm:  int(float64(total) * cfg.Context.LongTermRatio),
		Reserved:  int(float64(total) * cfg.Context.ReservedRatio),
	}
}

// ValidateConfig 验证配置有效性
func ValidateConfig(cfg *MemoryConfig) error {
	// 验证存储配置
	if cfg.Storage.GlobalRoot == "" {
		return fmt.Errorf("配置错误: storage.global_root 不能为空")
	}
	if cfg.Storage.ProjectDirName == "" {
		return fmt.Errorf("配置错误: storage.project_dir_name 不能为空")
	}

	// 验证 Token 限制
	if cfg.Core.MaxTokens <= 0 {
		return fmt.Errorf("配置错误: core.max_tokens 必须大于 0")
	}
	if cfg.Session.MaxTokens <= 0 {
		return fmt.Errorf("配置错误: session.max_tokens 必须大于 0")
	}

	// 验证上下文比例
	totalRatio := cfg.Context.CoreRatio + cfg.Context.SessionRatio +
		cfg.Context.ShortTermRatio + cfg.Context.LongTermRatio + cfg.Context.ReservedRatio
	if totalRatio < 0.99 || totalRatio > 1.01 {
		return fmt.Errorf("配置错误: 上下文预算比例总和必须为 1.0（当前: %.2f）", totalRatio)
	}

	// 验证检索配置
	if cfg.Retrieval.MinSimilarity < 0 || cfg.Retrieval.MinSimilarity > 1 {
		return fmt.Errorf("配置错误: retrieval.min_similarity 必须在 0-1 之间")
	}

	return nil
}
