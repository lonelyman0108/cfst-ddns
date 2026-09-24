package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lonelyman0108/cfst-ddns/internal/notify"
	"github.com/lonelyman0108/cfst-ddns/internal/provider"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

// ---------- 仪表盘 ----------

func (s *Server) dashboard(c *gin.Context) {
	db := s.Store.DB
	var taskCount, enabledCount, accountCount, notifierCount, runs24h, success24h, failed24h int64
	since := time.Now().Add(-24 * time.Hour)
	db.Model(&store.Task{}).Count(&taskCount)
	db.Model(&store.Task{}).Where("enabled = ?", true).Count(&enabledCount)
	db.Model(&store.Account{}).Count(&accountCount)
	db.Model(&store.Notifier{}).Count(&notifierCount)
	db.Model(&store.Run{}).Where("created_at >= ?", since).Count(&runs24h)
	db.Model(&store.Run{}).Where("created_at >= ? AND status = ?", since, store.StatusSuccess).Count(&success24h)
	db.Model(&store.Run{}).Where("created_at >= ? AND status IN ?", since, []string{store.StatusFailed, store.StatusPartial}).Count(&failed24h)

	active := []store.Run{}
	db.Select(store.SummaryColumns).Where("status IN ?", []string{store.StatusQueued, store.StatusRunning}).Order("id").Find(&active)
	last := []store.Run{}
	db.Select(store.SummaryColumns).Order("id DESC").Limit(10).Find(&last)

	tasks, _ := s.Store.ListTasks()
	names := map[uint]string{}
	type upcoming struct {
		TaskID    uint      `json:"taskId"`
		TaskName  string    `json:"taskName"`
		NextRunAt time.Time `json:"nextRunAt"`
	}
	up := []upcoming{}
	for _, t := range tasks {
		names[t.ID] = t.Name
		if n := s.Scheduler.Next(t.ID); n != nil {
			up = append(up, upcoming{t.ID, t.Name, *n})
		}
	}
	sort.Slice(up, func(i, j int) bool { return up[i].NextRunAt.Before(up[j].NextRunAt) })

	type record struct {
		TaskID    uint      `json:"taskId"`
		TaskName  string    `json:"taskName"`
		FQDN      string    `json:"fqdn"`
		Type      string    `json:"type"`
		Value     string    `json:"value"`
		UpdatedAt time.Time `json:"updatedAt"`
	}
	var states []store.RecordState
	db.Order("fqdn, type").Find(&states)
	records := []record{}
	for _, st := range states {
		records = append(records, record{st.TaskID, names[st.TaskID], st.FQDN, st.Type, st.Value, st.UpdatedAt})
	}

	type point struct {
		Time     time.Time `json:"time"`
		TaskID   uint      `json:"taskId"`
		TaskName string    `json:"taskName"`
		IPType   string    `json:"ipType"`
		Latency  float64   `json:"latency"`
		Speed    float64   `json:"speed"`
	}
	var trendRuns []store.Run
	db.Select("id, task_id, task_name, best_ipv4, best_ipv6, best_latency, best_speed, finished_at, created_at").
		Where("status IN ? AND best_latency > 0", []string{store.StatusSuccess, store.StatusPartial}).
		Order("id DESC").Limit(50).Find(&trendRuns)
	trend := []point{}
	for i := len(trendRuns) - 1; i >= 0; i-- {
		r := trendRuns[i]
		t := r.CreatedAt
		if r.FinishedAt != nil {
			t = *r.FinishedAt
		}
		ipType := "v4"
		if r.BestIPv4 == "" {
			ipType = "v6"
		}
		trend = append(trend, point{t, r.TaskID, r.TaskName, ipType, r.BestLatency, r.BestSpeed})
	}

	st := s.CFST.Status()
	c.JSON(http.StatusOK, gin.H{
		"stats": gin.H{"taskCount": taskCount, "enabledTaskCount": enabledCount, "accountCount": accountCount,
			"notifierCount": notifierCount, "runs24h": runs24h, "success24h": success24h, "failed24h": failed24h},
		"cfst":     gin.H{"installed": st.Installed, "version": st.Version},
		"active":   active,
		"lastRuns": last,
		"records":  records,
		"trend":    trend,
		"upcoming": up,
	})
}

// ---------- cfst ----------

func (s *Server) cfstStatus(c *gin.Context) { c.JSON(http.StatusOK, s.CFST.Status()) }

func (s *Server) cfstReleases(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	list, err := s.CFST.Releases(ctx)
	if err != nil {
		fail(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func (s *Server) cfstInstall(c *gin.Context) {
	var req struct {
		Version string `json:"version"`
	}
	if !bind(c, &req) {
		return
	}
	var active int64
	s.Store.DB.Model(&store.Run{}).Where("status = ?", store.StatusRunning).Count(&active)
	if active > 0 {
		failMsg(c, http.StatusConflict, "有任务正在测速，请稍后再安装")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	v, err := s.CFST.Install(ctx, strings.TrimSpace(req.Version))
	if err != nil {
		s.Log.Error("安装 cfst 失败", "err", err)
		fail(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": v})
}

func ipKind(c *gin.Context) (string, bool) {
	k := c.Param("kind")
	if k != "v4" && k != "v6" {
		failMsg(c, http.StatusBadRequest, "kind 必须为 v4 或 v6")
		return "", false
	}
	return k, true
}

func countLines(s string) int {
	n := 0
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" && !strings.HasPrefix(t, "#") {
			n++
		}
	}
	return n
}

func (s *Server) getIPFile(c *gin.Context) {
	k, ok := ipKind(c)
	if !ok {
		return
	}
	content, err := s.CFST.ReadIPFile(k)
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": content, "lines": countLines(content)})
}

func (s *Server) putIPFile(c *gin.Context) {
	k, ok := ipKind(c)
	if !ok {
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if !bind(c, &req) {
		return
	}
	if countLines(req.Content) == 0 {
		failMsg(c, http.StatusBadRequest, "IP 段不能为空")
		return
	}
	if err := s.CFST.WriteIPFile(k, req.Content); err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) resetIPFile(c *gin.Context) {
	k, ok := ipKind(c)
	if !ok {
		return
	}
	content, err := s.CFST.ResetIPFile(k)
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": content, "lines": countLines(content)})
}

// ---------- 设置 ----------

func (s *Server) getSettings(c *gin.Context) { c.JSON(http.StatusOK, s.Store.GetSettings()) }

func (s *Server) putSettings(c *gin.Context) {
	cur := s.Store.GetSettings()
	var req map[string]json.RawMessage
	if !bind(c, &req) {
		return
	}
	next := cur
	b, _ := json.Marshal(req)
	if err := json.Unmarshal(b, &next); err != nil {
		failMsg(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	next.HookToken = cur.HookToken
	next.GithubMirror = strings.TrimRight(strings.TrimSpace(next.GithubMirror), "/")
	if next.GithubMirror != "" && !strings.HasPrefix(next.GithubMirror, "http://") && !strings.HasPrefix(next.GithubMirror, "https://") {
		failMsg(c, http.StatusBadRequest, "GitHub 镜像地址需以 http:// 或 https:// 开头")
		return
	}
	if next.HistoryRetentionDays < 0 {
		failMsg(c, http.StatusBadRequest, "历史保留天数不能为负数")
		return
	}
	if next.HookEnabled && next.HookToken == "" {
		next.HookToken = store.RandomToken(24)
	}
	if err := s.Store.SaveSettings(next); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, next)
}

func (s *Server) regenHookToken(c *gin.Context) {
	st := s.Store.GetSettings()
	st.HookToken = store.RandomToken(24)
	if err := s.Store.SaveSettings(st); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}

// hookRun 供外部系统通过令牌触发任务。
func (s *Server) hookRun(c *gin.Context) {
	st := s.Store.GetSettings()
	tok := c.Query("token")
	if tok == "" {
		tok = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	}
	if !st.HookEnabled || st.HookToken == "" || subtle.ConstantTimeCompare([]byte(tok), []byte(st.HookToken)) != 1 {
		failMsg(c, http.StatusForbidden, "Webhook 未启用或令牌错误")
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	s.enqueue(c, id, "hook")
}

// ---------- 备份与恢复 ----------

type backupFile struct {
	Format     string           `json:"format"`
	Version    int              `json:"version"`
	ExportedAt time.Time        `json:"exportedAt"`
	Settings   store.Settings   `json:"settings"`
	Accounts   []store.Account  `json:"accounts"`
	Notifiers  []store.Notifier `json:"notifiers"`
	Tasks      []store.Task     `json:"tasks"`
}

const backupFormat = "cfst-ddns-backup"

func (s *Server) backup(c *gin.Context) {
	accounts, err := s.Store.ListAccounts()
	if err != nil {
		dbFail(c, err)
		return
	}
	notifiers, err := s.Store.ListNotifiers()
	if err != nil {
		dbFail(c, err)
		return
	}
	tasks, err := s.Store.ListTasks()
	if err != nil {
		dbFail(c, err)
		return
	}
	b := backupFile{Format: backupFormat, Version: 1, ExportedAt: time.Now(), Settings: s.Store.GetSettings(),
		Accounts: accounts, Notifiers: notifiers, Tasks: tasks}
	name := fmt.Sprintf("cfst-ddns-backup-%s.json", time.Now().Format("20060102-150405"))
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.IndentedJSON(http.StatusOK, b)
}

func (s *Server) restore(c *gin.Context) {
	var b backupFile
	if !bind(c, &b) {
		return
	}
	if b.Format != backupFormat || b.Version != 1 {
		failMsg(c, http.StatusBadRequest, "不是有效的 cfst-ddns 备份文件")
		return
	}
	var active int64
	s.Store.DB.Model(&store.Run{}).Where("status IN ?", []string{store.StatusQueued, store.StatusRunning}).Count(&active)
	if active > 0 {
		failMsg(c, http.StatusConflict, "有任务正在执行，请稍后再恢复")
		return
	}
	for _, a := range b.Accounts {
		if _, ok := provider.Meta(a.Provider); !ok {
			failMsg(c, http.StatusBadRequest, "备份中包含不支持的 DNS 服务商: "+a.Provider)
			return
		}
	}
	for _, n := range b.Notifiers {
		if _, ok := notify.Meta(n.Type); !ok {
			failMsg(c, http.StatusBadRequest, "备份中包含不支持的通知渠道: "+n.Type)
			return
		}
	}
	old, _ := s.Store.ListTasks()
	err := s.Store.DB.Transaction(func(tx *gorm.DB) error {
		for _, m := range []any{&store.Task{}, &store.Account{}, &store.Notifier{}, &store.RecordState{}} {
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(m).Error; err != nil {
				return err
			}
		}
		txs := s.Store.WithDB(tx)
		for i := range b.Accounts {
			if err := txs.SaveAccount(&b.Accounts[i]); err != nil {
				return err
			}
		}
		for i := range b.Notifiers {
			if err := txs.SaveNotifier(&b.Notifiers[i]); err != nil {
				return err
			}
		}
		for i := range b.Tasks {
			if err := tx.Create(&b.Tasks[i]).Error; err != nil {
				return err
			}
		}
		b.Settings.HookToken = strings.TrimSpace(b.Settings.HookToken)
		return txs.SaveSettings(b.Settings)
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, fmt.Errorf("恢复失败，数据未改动: %w", err))
		return
	}
	for _, t := range old {
		s.Scheduler.Remove(t.ID)
	}
	tasks, _ := s.Store.ListTasks()
	for i := range tasks {
		s.Scheduler.Sync(&tasks[i])
	}
	s.Log.Info("已从备份恢复配置", "accounts", len(b.Accounts), "notifiers", len(b.Notifiers), "tasks", len(b.Tasks))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
