package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lonelyman0108/cfst-ddns/internal/engine"
	"github.com/lonelyman0108/cfst-ddns/internal/scheduler"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

type taskView struct {
	store.Task
	Running   bool       `json:"running"`
	NextRunAt *time.Time `json:"nextRunAt"`
	LastRun   *store.Run `json:"lastRun"`
}

// DefaultTask 为新建任务的默认值。
func DefaultTask() store.Task {
	return store.Task{
		Enabled: true,
		Cron:    "0 */6 * * *",
		IPType:  "v4",
		SpeedTest: store.SpeedTestConfig{Threads: 200, PingTimes: 4, DownloadCount: 10, DownloadTime: 10,
			Port: 443, MaxLatency: 9999, MaxLossRate: 1, IPSource: "default"},
		Update:      store.UpdatePolicy{RecordCount: 1, SkipUnchanged: true},
		Targets:     []store.Target{},
		NotifierIDs: []uint{},
	}
}

func (s *Server) lastRuns() map[uint]*store.Run {
	var runs []store.Run
	// 每个任务最近一次执行
	s.Store.DB.Select(store.SummaryColumns).
		Where("id IN (?)", s.Store.DB.Model(&store.Run{}).Select("MAX(id)").Group("task_id")).
		Find(&runs)
	out := map[uint]*store.Run{}
	for i := range runs {
		out[runs[i].TaskID] = &runs[i]
	}
	return out
}

func (s *Server) view(t store.Task, last map[uint]*store.Run) taskView {
	_, running := s.Engine.Running(t.ID)
	if t.Targets == nil {
		t.Targets = []store.Target{}
	}
	if t.NotifierIDs == nil {
		t.NotifierIDs = []uint{}
	}
	return taskView{Task: t, Running: running, NextRunAt: s.Scheduler.Next(t.ID), LastRun: last[t.ID]}
}

func (s *Server) listTasks(c *gin.Context) {
	list, err := s.Store.ListTasks()
	if err != nil {
		dbFail(c, err)
		return
	}
	last := s.lastRuns()
	out := make([]taskView, 0, len(list))
	for _, t := range list {
		out = append(out, s.view(t, last))
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) taskDefaults(c *gin.Context) {
	c.JSON(http.StatusOK, s.view(DefaultTask(), nil))
}

func (s *Server) getTask(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	t, err := s.Store.GetTask(id)
	if err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, s.view(*t, s.lastRuns()))
}

// normalizeTask 校验并规范化任务字段。
func (s *Server) normalizeTask(t *store.Task) error {
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" {
		return errors.New("任务名称不能为空")
	}
	t.Cron = strings.TrimSpace(t.Cron)
	if err := scheduler.Validate(t.Cron); err != nil {
		return err
	}
	switch t.IPType {
	case "v4", "v6", "both":
	default:
		return errors.New("IP 类型必须为 v4、v6 或 both")
	}
	st := &t.SpeedTest
	if st.Threads < 0 || st.Threads > 1000 {
		return errors.New("延迟测速线程需在 1-1000 之间")
	}
	if st.MaxLossRate < 0 || st.MaxLossRate > 1 {
		return errors.New("丢包率上限需在 0-1 之间")
	}
	if st.IPSource != "custom" {
		st.IPSource = "default"
	} else {
		if (t.IPType != "v6" && strings.TrimSpace(st.IPv4Ranges) == "") ||
			(t.IPType != "v4" && strings.TrimSpace(st.IPv6Ranges) == "") {
			return errors.New("自定义 IP 段不能为空")
		}
	}
	if t.Update.RecordCount < 1 {
		t.Update.RecordCount = 1
	}
	if t.Update.RecordCount > 10 {
		return errors.New("每种类型最多写入 10 条记录")
	}
	if len(t.Targets) == 0 {
		return errors.New("至少需要一条目标记录")
	}
	seen := map[string]bool{}
	for i := range t.Targets {
		tg := &t.Targets[i]
		tg.Domain = strings.ToLower(strings.Trim(strings.TrimSpace(tg.Domain), "."))
		tg.RR = strings.TrimSpace(tg.RR)
		if tg.RR == "" {
			tg.RR = "@"
		}
		if tg.Domain == "" {
			return fmt.Errorf("第 %d 条目标记录的主域名不能为空", i+1)
		}
		if _, err := s.Store.GetAccount(tg.AccountID); err != nil {
			return fmt.Errorf("第 %d 条目标记录的 DNS 账号不存在", i+1)
		}
		key := fmt.Sprintf("%d|%s|%s|%s", tg.AccountID, tg.RR, tg.Domain, tg.Line)
		if seen[key] {
			return fmt.Errorf("目标记录重复: %s.%s", tg.RR, tg.Domain)
		}
		seen[key] = true
	}
	if t.NotifierIDs == nil {
		t.NotifierIDs = []uint{}
	}
	return nil
}

func (s *Server) createTask(c *gin.Context) {
	t := DefaultTask()
	if !bind(c, &t) {
		return
	}
	t.ID = 0
	if err := s.normalizeTask(&t); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if err := s.Store.DB.Create(&t).Error; err != nil {
		dbFail(c, err)
		return
	}
	s.Scheduler.Sync(&t)
	c.JSON(http.StatusOK, s.view(t, nil))
}

func (s *Server) updateTask(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	old, err := s.Store.GetTask(id)
	if err != nil {
		dbFail(c, err)
		return
	}
	t := *old
	if !bind(c, &t) {
		return
	}
	t.ID, t.CreatedAt = old.ID, old.CreatedAt
	if err := s.normalizeTask(&t); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if err := s.Store.DB.Save(&t).Error; err != nil {
		dbFail(c, err)
		return
	}
	s.Store.DB.Model(&store.Run{}).Where("task_id = ?", t.ID).Update("task_name", t.Name)
	s.Scheduler.Sync(&t)
	c.JSON(http.StatusOK, s.view(t, s.lastRuns()))
}

func (s *Server) setTaskEnabled(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if !bind(c, &req) {
		return
	}
	t, err := s.Store.GetTask(id)
	if err != nil {
		dbFail(c, err)
		return
	}
	t.Enabled = req.Enabled
	if err := s.Store.DB.Model(t).Update("enabled", req.Enabled).Error; err != nil {
		dbFail(c, err)
		return
	}
	s.Scheduler.Sync(t)
	c.JSON(http.StatusOK, s.view(*t, s.lastRuns()))
}

func (s *Server) deleteTask(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if _, running := s.Engine.Running(id); running {
		failMsg(c, http.StatusConflict, "任务正在执行，请先取消")
		return
	}
	if err := s.Store.DB.Delete(&store.Task{}, id).Error; err != nil {
		dbFail(c, err)
		return
	}
	s.Store.DB.Where("task_id = ?", id).Delete(&store.RecordState{})
	s.Scheduler.Remove(id)
	okJSON(c)
}

func (s *Server) cloneTask(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	t, err := s.Store.GetTask(id)
	if err != nil {
		dbFail(c, err)
		return
	}
	n := *t
	n.ID = 0
	n.Name = t.Name + " (副本)"
	n.Enabled = false
	n.CreatedAt, n.UpdatedAt = time.Time{}, time.Time{}
	if err := s.Store.DB.Create(&n).Error; err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, s.view(n, nil))
}

func (s *Server) enqueue(c *gin.Context, id uint, trigger string) {
	runID, err := s.Engine.Enqueue(id, trigger)
	if errors.Is(err, engine.ErrBusy) {
		fail(c, http.StatusConflict, err)
		return
	}
	if err != nil {
		dbFail(c, err)
		return
	}
	s.Log.Info("任务已触发", "task", id, "run", runID, "trigger", trigger)
	c.JSON(http.StatusOK, gin.H{"runId": runID})
}

func (s *Server) runTask(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	s.enqueue(c, id, "manual")
}

func (s *Server) cronPreview(c *gin.Context) {
	times, err := scheduler.Preview(c.Query("expr"), 5)
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	out := make([]string, 0, len(times))
	for _, t := range times {
		out = append(out, t.Format(time.RFC3339))
	}
	c.JSON(http.StatusOK, gin.H{"next": out})
}
