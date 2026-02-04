# AIMate 记忆系统核心设计文档

> 版本：v2.0 | 更新时间：2024-02-04
> 面向开发者的技术设计文档，帮助快速理解 AIMate 记忆系统的核心架构与实现

---

## 目录

- [系统概览](#系统概览)
- [设计目标](#设计目标)
- [四层记忆架构](#四层记忆架构)
- [技术架构](#技术架构)
- [存储设计](#存储设计)
- [索引与检索](#索引与检索)
- [自治运行机制](#自治运行机制)
- [跨项目隔离](#跨项目隔离)
- [核心组件](#核心组件)
- [数据流与生命周期](#数据流与生命周期)
- [性能与可观测性](#性能与可观测性)
- [开发指南](#开发指南)

---

## 系统概览

AIMate v2 记忆系统是一个**零负担、自治运行、可追溯**的本地化记忆管理系统，为 AI 编程助手提供长期上下文管理能力。

### 核心特性

- **四层记忆架构**：核心记忆、会话记忆、短期记忆、长期记忆，分层管理不同类型的信息
- **混合存储**：Markdown 文件（内容） + SQLite（索引） + 向量数据库（语义检索）
- **自动化治理**：智能裁剪、自动过期、提升归档，用户零负担
- **语义检索**：基于向量相似度的智能检索，理解用户意图
- **双层隔离**：全局记忆 + 项目记忆，避免跨项目污染
- **Git 友好**：Markdown 存储，支持版本控制和人工审查

### 设计哲学

```
用户无需关心记忆系统的存在
系统在后台默默工作
需要时精准召回相关信息
```

---

## 设计目标

### 用户视角

1. **零负担**：无需手动管理记忆，系统自动识别、存储、检索
2. **智能召回**：提问时自动加载相关记忆，无需显式调用
3. **跨会话连续性**：重新打开项目时自动恢复上下文

### 开发者视角

1. **可维护性**：代码模块化，职责清晰
2. **可扩展性**：支持新增记忆类型和检索策略
3. **可观测性**：完善的统计和诊断工具
4. **可测试性**：组件解耦，易于单元测试

### 技术目标

| 指标 | 目标 | 说明 |
|------|------|------|
| 检索延迟（P90） | < 200ms | 语义检索 + 文件读取 |
| 向量检索延迟 | < 50ms | sqlite-vec 查询 |
| 时间范围查询 | < 20ms | SQLite 索引查询 |
| 支持记忆数量 | 10万+ | 大规模项目支持 |
| 上下文构建 | < 100ms | 新会话启动时间 |

---

## 四层记忆架构

```
┌─────────────────────────────────────────────────────┐
│                  核心记忆 (Core)                     │
│          用户偏好、全局规则、角色设定                 │
│               永久有效 | 最高优先级                   │
└─────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────┐
│                  会话记忆 (Session)                  │
│            当前对话的完整上下文窗口                   │
│          实时持久化 | 自动裁剪 | 可恢复               │
└─────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────┐
│                短期记忆 (Short-term)                 │
│         临时任务、笔记、上下文摘要                    │
│      TTL 过期 | 高频提升 | 跨会话存活                │
└─────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────┐
│                长期记忆 (Long-term)                  │
│       项目知识、代码架构、历史决策                    │
│        持久化存储 | 语义检索 | 关联引用              │
└─────────────────────────────────────────────────────┘
```

### 各层特性对比

| 层级 | 责任 | 作用域 | 生命周期 | 存储方式 |
|------|------|--------|----------|----------|
| **Core** | 约束与偏好 | 全局 | 长期 | Markdown |
| **Session** | 对话上下文 | 会话 | 会话内 | Markdown + 实时追加 |
| **Short-term** | 临时信息 | 全局/项目 | 7-30天 TTL | Markdown + 索引 |
| **Long-term** | 稳定知识 | 全局/项目 | 长期 | Markdown + 向量索引 |

### 关键差异：会话记忆 vs 短期记忆

| 维度 | 会话记忆 | 短期记忆 |
|------|----------|----------|
| **边界** | 以会话为边界 | 以时间 TTL 为边界 |
| **跨会话** | ❌ 不自动加载 | ✅ 自动加载 |
| **容量管理** | Token 限制 + 自动裁剪 | 数量限制 + 过期清理 |
| **检索方式** | 顺序遍历（当前会话） | 语义检索 + 时间范围 |
| **典型内容** | 对话历史、工具调用结果 | 临时笔记、任务、摘要 |
| **清除时机** | 新会话开启时归档 | TTL 到期自动归档 |

---

## 技术架构

### 整体分层

```
┌─────────────────────────────────────────────────────┐
│                  应用层 (Agent)                      │
│         对话处理 | 记忆识别 | 上下文构建              │
└─────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────┐
│                 记忆管理层 (Managers)                │
│  CoreMemoryMgr | SessionMgr | ShortTermMgr | ...    │
└─────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────┐
│              辅助组件层 (Helpers)                    │
│  Classifier | Retriever | Trimmer | Lifecycle       │
└─────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────┐
│                存储与索引层 (Storage)                │
│  FileStore | IndexStore | VectorStore | Embedding   │
└─────────────────────────────────────────────────────┘
```

### 核心类图

```go
// 记忆系统入口
type MemorySystem struct {
    config    *MemoryConfig
    storage   *StorageManager
    fileStore *MarkdownFileStore
    index     *SQLiteIndexStore
    vector    *SQLiteVectorStore

    // 四层记忆管理器
    coreMgr      *CoreMemoryManager
    sessionMgr   *SessionManager
    shortTermMgr *ShortTermMemoryManager
    longTermMgr  *LongTermMemoryManager

    // 辅助组件
    classifier     *MemoryClassifier
    embedding      *EmbeddingManager
    retriever      *HybridRetriever
    contextBuilder *ContextBuilder
    lifecycle      *LifecycleManager
    trimmer        *SessionTrimmer
}
```

### 数据模型

```go
// 基础记忆结构
type Memory struct {
    ID          string       // UUID
    Type        MemoryType   // core/session/short_term/long_term
    Scope       MemoryScope  // global/project
    Category    MemoryCategory // preference/task/knowledge...
    Title       string
    Content     string       // Markdown 内容
    Tags        []string
    Related     []string     // 关联记忆 ID

    Status      MemoryStatus // active/archived/expired
    Importance  int          // 1-5
    AccessCount int
    ContentHash string       // 用于变更检测
    ExpiresAt   *time.Time   // TTL（仅短期记忆）

    CreatedAt   time.Time
    UpdatedAt   time.Time
    AccessedAt  time.Time
    FilePath    string
}
```

---

## 存储设计

### 混合存储架构

AIMate 采用**混合存储架构**，充分发挥各存储方式的优势：

```
内容存储 → Markdown 文件（Git 友好，人类可读）
元数据索引 → SQLite（快速查询，结构化）
语义索引 → 向量数据库（相似度检索）
```

### 目录结构

```
~/.aimate/                          # 全局存储
├── memory/
│   ├── core/                       # 核心记忆
│   │   ├── preferences.md
│   │   └── rules.md
│   ├── sessions/                   # 全局会话
│   │   └── archive/
│   ├── short_term/                 # 短期记忆
│   │   ├── tasks/
│   │   ├── notes/
│   │   └── contexts/
│   ├── long_term/                  # 长期记忆
│   │   ├── projects/
│   │   ├── knowledge/
│   │   └── decisions/
│   └── archive/                    # 归档
├── index.db                        # 元数据索引
└── vectors.db                      # 向量索引

{project}/.aimate/                  # 项目存储
├── memory/
│   ├── sessions/                   # 项目会话
│   │   ├── 2024-02/
│   │   │   └── 20240201_103000_a1b2c3d4.md
│   │   └── current.md
│   ├── short_term/                 # 项目短期记忆
│   └── long_term/                  # 项目长期记忆
├── index.db
└── vectors.db
```

### Markdown 文件格式

所有记忆文件采用统一格式：**YAML frontmatter + Markdown 内容**

```markdown
---
id: "abc123..."
type: "long_term"
scope: "project"
category: "knowledge"
title: "AIMate 记忆系统架构"
tags:
  - architecture
  - memory
created_at: "2024-02-01T10:30:00+08:00"
updated_at: "2024-02-01T15:30:00+08:00"
accessed_at: "2024-02-01T15:30:00+08:00"
access_count: 5
importance: 4
content_hash: "sha256:..."
related:
  - "../decisions/20240115_memory_design.md"
---

# AIMate 记忆系统架构

## 核心设计

AIMate 采用四层记忆架构...

## 技术选型

- 存储：Markdown + SQLite
- 向量检索：sqlite-vec
...
```

### 文件命名规范

```
格式：YYYYMMDD_category_title.md

示例：
20240201_task_login-refactor.md
20240201_knowledge_golang-patterns.md
20240115_decision_memory-design.md
```

---

## 索引与检索

### 三层索引架构

```
┌─────────────────────────────────────────────────────┐
│             1. 元数据索引 (SQLite)                   │
│         快速过滤：时间范围、类型、状态                │
└─────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────┐
│             2. 向量索引 (sqlite-vec)                 │
│              语义相似度检索 (Top-K)                  │
└─────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────┐
│             3. 全文检索 (FTS5/LIKE)                  │
│              关键词精确匹配 (降级方案)                │
└─────────────────────────────────────────────────────┘
```

### 元数据索引表结构

```sql
CREATE TABLE memory_index (
    id TEXT PRIMARY KEY,
    file_path TEXT NOT NULL UNIQUE,
    layer TEXT NOT NULL,              -- 记忆层级
    type TEXT,                        -- 记忆类型
    title TEXT,
    tags TEXT,                        -- JSON 数组
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    accessed_at DATETIME,
    access_count INTEGER DEFAULT 0,
    expires_at DATETIME,              -- 过期时间
    content_hash TEXT,                -- 用于同步检测

    INDEX idx_layer (layer),
    INDEX idx_created_at (created_at),
    INDEX idx_expires_at (expires_at)
);
```

### 混合检索策略

```go
func (r *HybridRetriever) Search(query string) []*Memory {
    // 1. 向量检索（语义相似度）
    vectorResults := r.searchByVector(query, topK=10)

    // 2. 关键词检索（精确匹配）
    keywordResults := r.searchByKeyword(query, topK=10)

    // 3. 合并去重
    merged := r.mergeResults(vectorResults, keywordResults)

    // 4. 应用时间权重
    r.applyTimeWeight(merged)

    // 5. 按分数排序
    sort.ByScore(merged)

    return merged[:finalTopK]
}
```

### 时间权重计算

```
最终分数 = 原始分数 × 时间衰减 × 重要性加成 × 访问频率加成

时间衰减 = decay_factor ^ (天数 / 30)
重要性加成 = 0.8 + 0.2 × (importance / 5)
访问频率加成 = 1 + log10(access_count + 1) × 0.1
```

### 索引同步机制

```
启动时自检 → 对比 content_hash → 检测文件变更
    ↓
发现不一致 → 更新向量索引 → 更新元数据索引
    ↓
清理孤立索引（文件不存在）
创建孤立文件的索引（索引中无记录）
```

---

## 自治运行机制

### 自动化流程总览

```
┌─────────────────────────────────────────────────────┐
│                用户对话（前台）                       │
└─────────────────────────────────────────────────────┘
                          │
        ┌─────────────────┼─────────────────┐
        ↓                 ↓                 ↓
  自动识别记忆      实时持久化会话      智能裁剪上下文
        │                 │                 │
        └─────────────────┼─────────────────┘
                          ↓
┌─────────────────────────────────────────────────────┐
│              后台维护任务（自动运行）                 │
│  • 过期清理   • 自动提升   • 索引同步   • 归档压缩  │
└─────────────────────────────────────────────────────┘
```

### 会话自动裁剪

**触发条件**：Token 数量达到 `context_max_tokens` 上限

**裁剪策略**：
1. 保护最近 N 轮对话（默认 3 轮）
2. 保护系统消息（System Prompt）
3. FIFO 删除最早的对话
4. 调用 LLM 生成摘要并存入短期记忆

```go
func (t *SessionTrimmer) TrimIfNeeded() (*TrimResult, error) {
    if !t.shouldTrim() {
        return nil, nil
    }

    // 1. 提取待裁剪内容
    toTrim := t.extractTrimContent()

    // 2. 调用 LLM 生成摘要
    summary := t.generateSummary(toTrim)

    // 3. 存入短期记忆
    t.shortTermMgr.Add("context", summary)

    // 4. 删除原始消息
    t.sessionMgr.RemoveMessages(toTrim)

    return result, nil
}
```

### 短期记忆自动管理

**过期清理**：
- 每次维护任务执行时检测 `expires_at`
- 过期文件移动到 `archive/` 而非直接删除
- 清理对应的向量索引条目

**自动提升**：
- 检测 `access_count >= N`（默认 5）的短期记忆
- 自动提升为长期记忆
- 删除原短期记忆文件

### 长期记忆自动归档

**归档条件**：
- 长期未访问（默认 90 天）
- 或存储空间超过阈值

**压缩合并**：
- 使用向量相似度检测相似记忆
- 调用 LLM 合并为一条精炼记忆
- 保留原始记忆的引用链接

### 后台维护任务

```go
type LifecycleManager struct {
    // 维护间隔：默认 1 小时
    intervalMinutes int
}

func (m *LifecycleManager) RunMaintenance() {
    // 1. 清理过期短期记忆
    m.shortTermMgr.CleanExpired()

    // 2. 归档长期未访问的记忆
    m.longTermMgr.ArchiveInactive()

    // 3. 自动提升高频访问记忆
    m.promoteHighAccessMemories()

    // 4. 同步索引
    m.syncer.SyncAll()
}
```

---

## 跨项目隔离

### 双层存储架构

```
全局存储 (~/.aimate/)         项目存储 ({project}/.aimate/)
     │                                │
     ├─ 核心记忆（用户偏好）           ├─ 会话记忆（项目对话）
     ├─ 全局知识（语言通用知识）       ├─ 短期记忆（项目任务）
     └─ 跨项目信息                     └─ 长期记忆（项目知识）
           │                                │
           └────────────┬───────────────────┘
                        ↓
                  记忆加载时合并
              全局核心记忆优先级最高
```

### 项目识别

系统从当前工作目录向上查找，直到发现以下标记文件之一：
- `.git/`
- `.aimate/`
- `go.mod`
- `package.json`
- `Cargo.toml`

```go
func (sm *StorageManager) detectProjectRoot(path string) string {
    for {
        for _, marker := range projectMarkers {
            if exists(path + "/" + marker) {
                return path // 找到项目根目录
            }
        }
        path = filepath.Dir(path) // 向上查找
    }
}
```

### 记忆归属判断

| 特征 | 归属 | 示例 |
|------|------|------|
| 用户偏好关键词 | 全局 | "我习惯用 Tab"、"我喜欢..." |
| 项目路径/文件引用 | 项目 | "这个项目的 main.go" |
| 技术通用知识 | 全局 | "Go error 处理最佳实践" |
| 项目架构决策 | 项目 | "我们决定使用 Redis" |
| 无法判断时 | 项目（默认） | 避免污染全局 |

### 记忆加载策略

**新会话启动时**：
1. 全局核心记忆（最高优先级）
2. 项目核心记忆
3. 项目长期记忆（相关）
4. 项目短期记忆（最近 7 天）
5. 全局记忆仅在检索时搜索，不自动加载

---

## 核心组件

### 1. MemorySystem（系统入口）

```go
// 初始化记忆系统
memSys := NewMemorySystem()
memSys.Initialize(apiKey)
memSys.SetProject(projectPath)

// 处理用户输入（自动识别记忆）
memSys.ProcessUserInput(ctx, userMessage)

// 添加对话到会话
memSys.AddConversation(role, content, tokenCount)

// 构建上下文
context := memSys.BuildContext(ctx, query)

// 检索记忆
results := memSys.Search(ctx, query, topK)
```

### 2. StorageManager（存储管理）

```go
// 管理双层存储结构
storage := NewStorageManager(config)
storage.SetCurrentProject(projectPath)

// 生成文件路径
filePath := storage.GenerateMemoryFilePath(memory)

// 获取存储统计
stats := storage.GetStorageStats()
```

### 3. HybridRetriever（混合检索）

```go
retriever := NewHybridRetriever(index, vector, fileStore, embedding, config)

// 执行混合检索
results := retriever.Search(ctx, query, &RetrievalOptions{
    TopK:          5,
    UseVector:     true,
    UseKeyword:    true,
    UseTimeWeight: true,
    MinSimilarity: 0.6,
})
```

### 4. ContextBuilder（上下文构建）

```go
builder := NewContextBuilder(coreMgr, sessionMgr, shortTermMgr, longTermMgr, retriever, config)

// 构建完整上下文
context := builder.BuildContext(ctx, query)

// 检查警告
warnings := builder.CheckContextWarnings()
// ["会话上下文已使用 87%，建议开启新会话"]
```

### 5. LifecycleManager（生命周期管理）

```go
lifecycle := NewLifecycleManager(shortTermMgr, longTermMgr, syncer, config)

// 启动后台维护
lifecycle.Start()

// 手动执行维护
result := lifecycle.RunMaintenance(ctx)
```

---

## 数据流与生命周期

### 记忆创建流程

```
用户输入 → MemoryClassifier.Classify()
    ↓
识别类型（核心/短期/长期）
    ↓
存储到 Markdown 文件 → 写入 frontmatter
    ↓
调用 Embedding API → 生成向量
    ↓
存入向量索引 (sqlite-vec)
    ↓
更新元数据索引 (SQLite)
    ↓
完成
```

### 记忆检索流程

```
用户查询 → HybridRetriever.Search()
    ↓
并行执行：
    1. 向量检索（语义）
    2. 关键词检索（精确）
    ↓
合并去重 → 应用时间权重
    ↓
过滤排序 → 返回 Top-K
    ↓
读取 Markdown 文件内容
    ↓
更新 access_count 和 accessed_at
    ↓
返回结果
```

### 会话记忆生命周期

```
创建会话 → 生成会话文件
    ↓
实时追加对话 → 更新 token_count
    ↓
检测阈值 → 超过 85% 发出警告
    ↓
达到上限 → 触发自动裁剪
    ↓
生成摘要 → 存入短期记忆
    ↓
新会话开启 → 归档旧会话
```

### 短期记忆生命周期

```
创建短期记忆 → 设置 TTL (14天)
    ↓
跨会话访问 → access_count++
    ↓
高频访问 (≥5次) → 自动提升为长期记忆
    ↓
TTL 到期 → 后台任务检测
    ↓
移动到 archive/ → 清理索引
```

---

## 性能与可观测性

### 性能监控

```go
// 获取系统统计
stats := memSys.GetStats()
// {
//   "core_tokens": 500,
//   "session_tokens": 45000,
//   "session_usage_ratio": 0.72,
//   "short_term_count": 23,
//   "long_term_count": 89,
//   "indexed_count": 115,
//   "vector_count": 112
// }
```

### 性能指标

所有操作的耗时自动记录到 `metrics.db`：

```sql
CREATE TABLE metrics (
    id TEXT PRIMARY KEY,
    operation TEXT,        -- retrieve, store, context_build, trim
    layer TEXT,           -- core, session, short_term, long_term
    duration_ms REAL,
    breakdown JSON,       -- 耗时分解
    timestamp DATETIME,
    context JSON
);
```

### 诊断命令

```bash
# 查看统计
/memory stats

# 性能指标
/memory perf

# 系统诊断
/memory diagnose
# 检查：
# - 文件与索引一致性
# - 向量索引健康状态
# - 性能瓶颈分析

# 重建索引
/memory reindex
```

### 健康检查

```go
warnings := memSys.CheckWarnings()
// [
//   { level: "warning", message: "会话上下文已使用 72%" },
//   { level: "info", message: "短期记忆 23/100 条" }
// ]
```

---

## 开发指南

### 快速开始

```go
package main

import (
    "context"
    v2 "aimate/internal/memory/v2"
)

func main() {
    // 1. 初始化记忆系统
    memSys, _ := v2.NewMemorySystem()
    memSys.Initialize(apiKey)
    memSys.SetProject("/path/to/project")
    defer memSys.Close()

    // 2. 处理对话
    ctx := context.Background()

    // 自动识别并存储记忆
    memSys.ProcessUserInput(ctx, "我习惯用 Tab 缩进")

    // 添加对话到会话
    memSys.AddConversation("user", "帮我重构登录模块", 20)
    memSys.AddConversation("assistant", "好的，让我先看看现有代码...", 50)

    // 3. 构建上下文
    builtCtx, _ := memSys.BuildContext(ctx, "登录模块怎么实现的")
    // 自动加载相关记忆

    // 4. 检索记忆
    results, _ := memSys.Search(ctx, "React 最佳实践", 5)
}
```

### 扩展新的记忆类型

1. 在 `types.go` 添加新的 `MemoryCategory`
2. 在 `classifier.go` 添加识别规则
3. 在对应的 Manager 中添加处理逻辑

### 测试

```bash
# 单元测试
go test ./internal/memory/v2/...

# 集成测试
go test -tags=integration ./internal/memory/v2/...

# E2E 测试
go test -tags=e2e ./internal/memory/v2/...
```

### 配置

```yaml
# ~/.aimate/config.yaml
memory:
  storage:
    global_root: "~/.aimate/memory"
    project_dir_name: ".aimate"

  context:
    total_budget: 128000
    core_ratio: 0.1
    session_ratio: 0.5
    short_term_ratio: 0.2
    long_term_ratio: 0.2

  session:
    max_tokens: 128000
    warning_ratio: 0.7
    critical_ratio: 0.85
    trim_target_ratio: 0.5
    protected_rounds: 3

  short_term:
    default_ttl_days: 14
    promote_threshold: 5

  embedding:
    enabled: true
    provider: "deepseek"
    model: "text-embedding-3-small"
    dimension: 1536

  maintenance:
    enabled: true
    interval_minutes: 60
```

---

## 总结

AIMate v2 记忆系统通过**四层架构 + 混合存储 + 自治运行**的设计，实现了：

✅ **零负担**：用户无需手动管理记忆
✅ **智能召回**：语义检索 + 时间权重
✅ **可追溯**：Markdown 存储 + Git 友好
✅ **可扩展**：模块化设计 + 清晰职责
✅ **高性能**：索引加速 + 增量同步

这是一个为**长期使用**而设计的记忆系统，让 AI 助手真正具备"记忆"能力。
