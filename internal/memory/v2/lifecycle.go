// Package v2 提供记忆生命周期管理功能
package v2

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// LifecycleManager 生命周期管理器
// 负责记忆的自动过期清理、归档、提升等生命周期管理
type LifecycleManager struct {
	shortTermMgr *ShortTermMemoryManager
	longTermMgr  *LongTermMemoryManager
	syncer       *IndexSyncer
	config       *MemoryConfig

	// 运行状态
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
	mu      sync.Mutex

	// 上次维护时间
	lastMaintenance time.Time
}

// NewLifecycleManager 创建生命周期管理器
func NewLifecycleManager(
	shortTermMgr *ShortTermMemoryManager,
	longTermMgr *LongTermMemoryManager,
	syncer *IndexSyncer,
	config *MemoryConfig,
) *LifecycleManager {
	return &LifecycleManager{
		shortTermMgr: shortTermMgr,
		longTermMgr:  longTermMgr,
		syncer:       syncer,
		config:       config,
		stopCh:       make(chan struct{}),
	}
}

// Start 启动后台维护任务
func (m *LifecycleManager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopCh = make(chan struct{})
	m.mu.Unlock()

	m.wg.Add(1)
	go m.maintenanceLoop()
}

// Stop 停止后台维护任务
func (m *LifecycleManager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	close(m.stopCh)
	m.mu.Unlock()

	m.wg.Wait()
}

// maintenanceLoop 维护循环
func (m *LifecycleManager) maintenanceLoop() {
	defer m.wg.Done()

	// 启动时执行一次维护
	m.RunMaintenance(context.Background())

	ticker := time.NewTicker(time.Duration(m.config.Maintenance.IntervalMinutes) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.RunMaintenance(context.Background())
		}
	}
}

// RunMaintenance 执行维护任务
func (m *LifecycleManager) RunMaintenance(ctx context.Context) *MaintenanceResult {
	result := &MaintenanceResult{
		StartTime: time.Now(),
	}

	if !m.config.Maintenance.Enabled {
		result.EndTime = time.Now()
		return result
	}

	// 1. 清理过期短期记忆
	if m.config.Maintenance.CleanupExpired && m.shortTermMgr != nil {
		cleaned, err := m.shortTermMgr.CleanExpired()
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("清理过期记忆失败: %v", err))
		}
		result.ExpiredCleaned = cleaned
	}

	// 2. 归档长期未访问的记忆
	if m.longTermMgr != nil {
		archived, err := m.longTermMgr.ArchiveInactive()
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("归档长期记忆失败: %v", err))
		}
		result.InactiveArchived = archived
	}

	// 3. 自动提升高频访问的短期记忆
	if m.shortTermMgr != nil && m.longTermMgr != nil {
		promoted, err := m.promoteHighAccessMemories()
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("提升记忆失败: %v", err))
		}
		result.Promoted = promoted
	}

	// 4. 同步索引
	if m.config.Maintenance.SyncIndex && m.syncer != nil {
		syncResult, err := m.syncer.SyncAll()
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("同步索引失败: %v", err))
		} else {
			result.IndexSynced = syncResult.Created + syncResult.Updated
			result.OrphanedCleaned = syncResult.Deleted
		}
	}

	m.lastMaintenance = time.Now()
	result.EndTime = time.Now()
	result.DurationMs = result.EndTime.Sub(result.StartTime).Milliseconds()

	return result
}

// promoteHighAccessMemories 提升高频访问的短期记忆为长期记忆
func (m *LifecycleManager) promoteHighAccessMemories() (int, error) {
	highAccess, err := m.shortTermMgr.GetHighAccessMemories()
	if err != nil {
		return 0, err
	}

	promoted := 0
	for _, mem := range highAccess {
		// 提升为长期记忆
		_, err := m.longTermMgr.PromoteFromShortTerm(mem)
		if err != nil {
			continue
		}

		// 删除原短期记忆
		if err := m.shortTermMgr.Delete(mem.ID); err != nil {
			continue
		}

		promoted++
	}

	return promoted, nil
}

// MaintenanceResult 维护结果
type MaintenanceResult struct {
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	DurationMs       int64     `json:"duration_ms"`
	ExpiredCleaned   int       `json:"expired_cleaned"`
	InactiveArchived int       `json:"inactive_archived"`
	Promoted         int       `json:"promoted"`
	IndexSynced      int       `json:"index_synced"`
	OrphanedCleaned  int       `json:"orphaned_cleaned"`
	Errors           []string  `json:"errors,omitempty"`
}

// GetLastMaintenanceTime 获取上次维护时间
func (m *LifecycleManager) GetLastMaintenanceTime() time.Time {
	return m.lastMaintenance
}

// IsRunning 检查是否正在运行
func (m *LifecycleManager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// ForceCleanup 强制清理
func (m *LifecycleManager) ForceCleanup(ctx context.Context) *MaintenanceResult {
	return m.RunMaintenance(ctx)
}

// CleanOldSessions 清理旧会话
func (m *LifecycleManager) CleanOldSessions(retentionDays int) (int, error) {
	// 此方法需要会话管理器支持，暂时返回 0
	return 0, nil
}

// GetMaintenanceStats 获取维护统计
func (m *LifecycleManager) GetMaintenanceStats() *MaintenanceStats {
	stats := &MaintenanceStats{
		LastMaintenance: m.lastMaintenance,
		IsRunning:       m.IsRunning(),
	}

	if m.shortTermMgr != nil {
		shortTermStats, err := m.shortTermMgr.GetStats()
		if err == nil {
			stats.ShortTermTotal = shortTermStats.Total
			stats.ShortTermExpired = shortTermStats.ExpiredCount
		}
	}

	if m.longTermMgr != nil {
		longTermStats, err := m.longTermMgr.GetStats()
		if err == nil {
			stats.LongTermTotal = longTermStats.Total
			stats.LongTermArchived = longTermStats.ArchivedCount
		}
	}

	return stats
}

// MaintenanceStats 维护统计
type MaintenanceStats struct {
	LastMaintenance  time.Time `json:"last_maintenance"`
	IsRunning        bool      `json:"is_running"`
	ShortTermTotal   int       `json:"short_term_total"`
	ShortTermExpired int       `json:"short_term_expired"`
	LongTermTotal    int       `json:"long_term_total"`
	LongTermArchived int       `json:"long_term_archived"`
}

// ScheduleTask 调度任务
type ScheduleTask struct {
	Name     string
	Interval time.Duration
	LastRun  time.Time
	NextRun  time.Time
	Enabled  bool
	RunFunc  func(ctx context.Context) error
}

// TaskScheduler 任务调度器
type TaskScheduler struct {
	tasks   []*ScheduleTask
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
	mu      sync.Mutex
}

// NewTaskScheduler 创建任务调度器
func NewTaskScheduler() *TaskScheduler {
	return &TaskScheduler{
		tasks:  []*ScheduleTask{},
		stopCh: make(chan struct{}),
	}
}

// AddTask 添加任务
func (s *TaskScheduler) AddTask(name string, interval time.Duration, runFunc func(ctx context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks = append(s.tasks, &ScheduleTask{
		Name:     name,
		Interval: interval,
		Enabled:  true,
		RunFunc:  runFunc,
		NextRun:  time.Now().Add(interval),
	})
}

// Start 启动调度器
func (s *TaskScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runLoop()
}

// Stop 停止调度器
func (s *TaskScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()

	s.wg.Wait()
}

// runLoop 运行循环
func (s *TaskScheduler) runLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkAndRunTasks()
		}
	}
}

// checkAndRunTasks 检查并运行任务
func (s *TaskScheduler) checkAndRunTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	ctx := context.Background()

	for _, task := range s.tasks {
		if !task.Enabled {
			continue
		}

		if now.After(task.NextRun) {
			// 运行任务
			go func(t *ScheduleTask) {
				if err := t.RunFunc(ctx); err != nil {
					// 记录错误
					fmt.Printf("任务 %s 执行失败: %v\n", t.Name, err)
				}
			}(task)

			task.LastRun = now
			task.NextRun = now.Add(task.Interval)
		}
	}
}

// GetTasks 获取所有任务
func (s *TaskScheduler) GetTasks() []*ScheduleTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tasks
}
