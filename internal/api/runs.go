package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

func (s *Server) listRuns(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	q := s.Store.DB.Model(&store.Run{})
	if v := c.Query("taskId"); v != "" {
		q = q.Where("task_id = ?", v)
	}
	if v := c.Query("status"); v != "" {
		q = q.Where("status = ?", v)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		dbFail(c, err)
		return
	}
	items := []store.Run{}
	if err := q.Select(store.SummaryColumns).Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (s *Server) activeRuns(c *gin.Context) {
	items := []store.Run{}
	s.Store.DB.Select(store.SummaryColumns).Where("status IN ?", []string{store.StatusQueued, store.StatusRunning}).
		Order("id").Find(&items)
	c.JSON(http.StatusOK, items)
}

func (s *Server) getRun(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var r store.Run
	if err := s.Store.DB.First(&r, id).Error; err != nil {
		dbFail(c, err)
		return
	}
	// 运行中的日志尚未落库，从内存读取
	if l, ok := s.Hub.Get(id); ok && r.Log == "" {
		r.Log = l.Text()
	}
	if r.Results == nil {
		r.Results = []store.SpeedResult{}
	}
	if r.Changes == nil {
		r.Changes = []store.DNSChange{}
	}
	// 显式输出大字段（omitempty 会吞掉空值）
	c.JSON(http.StatusOK, struct {
		store.Run
		Log     string              `json:"log"`
		Results []store.SpeedResult `json:"results"`
		Changes []store.DNSChange   `json:"changes"`
	}{r, r.Log, r.Results, r.Changes})
}

func (s *Server) cancelRun(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.Engine.Cancel(id); err != nil {
		fail(c, http.StatusConflict, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) deleteRun(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	res := s.Store.DB.Where("status NOT IN ?", []string{store.StatusQueued, store.StatusRunning}).Delete(&store.Run{}, id)
	if res.Error != nil {
		dbFail(c, res.Error)
		return
	}
	if res.RowsAffected == 0 {
		failMsg(c, http.StatusConflict, "记录不存在或正在执行")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) purgeRuns(c *gin.Context) {
	days, _ := strconv.Atoi(c.Query("beforeDays"))
	n, err := s.Store.PurgeRuns(days)
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": n})
}

// runStream 以 SSE 推送执行日志；执行已结束时只推送完整日志与 done。
func (s *Server) runStream(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var r store.Run
	if err := s.Store.DB.Select(store.SummaryColumns).First(&r, id).Error; err != nil {
		dbFail(c, err)
		return
	}
	w := c.Writer
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	send := func(event string, data any) {
		var payload string
		if str, ok := data.(string); ok {
			payload = str
		} else {
			b, _ := json.Marshal(data)
			payload = string(b)
		}
		writeSSE(w, event, payload)
		w.Flush()
	}

	l, live := s.Hub.Get(id)
	if !live {
		var full store.Run
		s.Store.DB.Select("log").First(&full, id)
		for _, line := range splitLines(full.Log) {
			send("log", line)
		}
		send("done", r)
		return
	}
	backlog, ch, unsubscribe := l.Subscribe()
	defer unsubscribe()
	for _, line := range backlog {
		send("log", line)
	}
	send("status", r)
	if ch == nil {
		s.Store.DB.Select(store.SummaryColumns).First(&r, id)
		send("done", r)
		return
	}
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ping.C:
			_, _ = io.WriteString(w, ": ping\n\n")
			w.Flush()
		case e, open := <-ch:
			if !open {
				return
			}
			send(e.Type, e.Data)
			if e.Type == "done" {
				return
			}
		}
	}
}

// writeSSE 写入一个事件，多行数据拆为多条 data 字段。
func writeSSE(w io.Writer, event, data string) {
	fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range splitLines(data) {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	if data == "" {
		fmt.Fprint(w, "data: \n")
	}
	fmt.Fprint(w, "\n")
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
