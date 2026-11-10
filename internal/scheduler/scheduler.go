// Package scheduler 按 cron 表达式触发任务。
package scheduler

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/lonelyman0108/cfst-ddns/internal/engine"
	"github.com/lonelyman0108/cfst-ddns/internal/i18n"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

// Parser 支持标准 5 段表达式与 @every/@daily 等描述符。
var Parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// Validate 校验表达式（空表示仅手动）。
func Validate(expr string) error {
	if strings.TrimSpace(expr) == "" {
		return nil
	}
	if _, err := Parser.Parse(expr); err != nil {
		return i18n.Errorf("cron 表达式无效: %v", err)
	}
	return nil
}

// Preview 返回接下来 n 次执行时间。
func Preview(expr string, n int) ([]time.Time, error) {
	s, err := Parser.Parse(expr)
	if err != nil {
		return nil, i18n.Errorf("cron 表达式无效: %v", err)
	}
	out := make([]time.Time, 0, n)
	t := time.Now()
	for i := 0; i < n; i++ {
		t = s.Next(t)
		if t.IsZero() {
			break
		}
		out = append(out, t)
	}
	return out, nil
}

// Scheduler 维护任务与 cron 条目的映射。
type Scheduler struct {
	cron   *cron.Cron
	engine *engine.Engine
	store  *store.Store
	log    *slog.Logger

	mu            sync.Mutex
	entries       map[uint]cron.EntryID
	exprs         map[uint]string
	healthEntries map[uint]cron.EntryID
	healthExprs   map[uint]string
}

func New(st *store.Store, e *engine.Engine, log *slog.Logger) *Scheduler {
	return &Scheduler{cron: cron.New(cron.WithParser(Parser)), engine: e, store: st, log: log,
		entries: map[uint]cron.EntryID{}, exprs: map[uint]string{}, healthEntries: map[uint]cron.EntryID{}, healthExprs: map[uint]string{}}
}

// Start 加载全部任务并启动；另外每天 04:10 清理过期历史。
func (s *Scheduler) Start() error {
	tasks, err := s.store.ListTasks()
	if err != nil {
		return err
	}
	for i := range tasks {
		s.Sync(&tasks[i])
	}
	_, _ = s.cron.AddFunc("10 4 * * *", s.purge)
	s.cron.Start()
	return nil
}

// Stop 停止调度。
func (s *Scheduler) Stop() { s.cron.Stop() }

func (s *Scheduler) purge() {
	days := s.store.GetSettings().HistoryRetentionDays
	if days <= 0 {
		return
	}
	n, err := s.store.PurgeRuns(days)
	if err != nil {
		s.log.Error("清理历史失败", "err", err)
	} else if n > 0 {
		s.log.Info("已清理过期执行记录", "count", n, "days", days)
	}
}

// Sync 在任务增改后更新调度。
func (s *Scheduler) Sync(t *store.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	expr, healthExpr := strings.TrimSpace(t.Cron), ""
	if !t.Enabled {
		expr = ""
	} else if t.HealthCheck.Enabled {
		interval := t.HealthCheck.IntervalMinutes
		if interval <= 0 {
			interval = 10
		}
		healthExpr = fmt.Sprintf("@every %dm", interval)
	}
	s.syncEntry(t.ID, expr, s.entries, s.exprs, func() {
		if _, err := s.engine.Enqueue(t.ID, "cron", false); err != nil {
			s.log.Warn("定时触发任务失败", "task", t.ID, "err", err)
		}
	})
	s.syncEntry(t.ID, healthExpr, s.healthEntries, s.healthExprs, func() {
		if err := s.engine.EnqueueHealth(t.ID); err != nil {
			s.log.Warn("定时健康检查失败", "task", t.ID, "err", err)
		}
	})
}

func (s *Scheduler) syncEntry(taskID uint, expr string, entries map[uint]cron.EntryID, exprs map[uint]string, run func()) {
	if old, ok := entries[taskID]; ok {
		if exprs[taskID] == expr {
			return
		}
		s.cron.Remove(old)
		delete(entries, taskID)
		delete(exprs, taskID)
	}
	if expr == "" {
		return
	}
	entry, err := s.cron.AddFunc(expr, run)
	if err != nil {
		s.log.Error("注册定时任务失败", "task", taskID, "cron", expr, "err", err)
		return
	}
	entries[taskID], exprs[taskID] = entry, expr
}

// Remove 在任务删除后移除调度。
func (s *Scheduler) Remove(taskID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.healthExprs, taskID)
	if id, ok := s.healthEntries[taskID]; ok {
		s.cron.Remove(id)
		delete(s.healthEntries, taskID)
	}
	if e, ok := s.entries[taskID]; ok {
		s.cron.Remove(e)
		delete(s.entries, taskID)
		delete(s.exprs, taskID)
	}
}

// HealthNext returns the next check, independently of the full-run cron.
func (s *Scheduler) HealthNext(taskID uint) *time.Time {
	s.mu.Lock()
	id, ok := s.healthEntries[taskID]
	s.mu.Unlock()
	if !ok {
		return nil
	}
	next := s.cron.Entry(id).Next
	if next.IsZero() {
		return nil
	}
	return &next
}

// Next 返回任务下次执行时间。
func (s *Scheduler) Next(taskID uint) *time.Time {
	s.mu.Lock()
	id, ok := s.entries[taskID]
	s.mu.Unlock()
	if !ok {
		return nil
	}
	n := s.cron.Entry(id).Next
	if n.IsZero() {
		return nil
	}
	return &n
}
