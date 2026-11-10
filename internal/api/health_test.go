package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/auth"
	"github.com/lonelyman0108/cfst-ddns/internal/cfst"
	"github.com/lonelyman0108/cfst-ddns/internal/engine"
	"github.com/lonelyman0108/cfst-ddns/internal/logbus"
	"github.com/lonelyman0108/cfst-ddns/internal/scheduler"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
	"github.com/lonelyman0108/cfst-ddns/internal/secret"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

func healthServer(t *testing.T) (*Server, func(string, string, any) *httptest.ResponseRecorder, *store.Task) {
	t.Helper()
	dir := t.TempDir()
	box, err := secret.New("api-health")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "db"), box)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := logbus.NewHub()
	mgr := &cfst.Manager{Dir: dir, Log: log}
	eng := engine.New(st, mgr, hub, log, dir)
	sch := scheduler.New(st, eng, log)
	if err := sch.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sch.Stop)
	authSvc, err := auth.New(st)
	if err != nil {
		t.Fatal(err)
	}
	token, err := authSvc.Setup("test", "test-password")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Store: st, Engine: eng, Scheduler: sch, CFST: mgr, Hub: hub, Auth: authSvc, Log: log}
	acc := store.Account{Name: "cf", Provider: "cloudflare", Config: schema.Config{"apiToken": "test"}}
	if err := st.SaveAccount(&acc); err != nil {
		t.Fatal(err)
	}
	task := DefaultTask()
	task.Name = "health"
	task.Targets = []store.Target{{AccountID: acc.ID, Domain: "example.com", RR: "cdn"}}
	if err := st.DB.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	handler := s.Handler()
	request := func(method, path string, body any) *httptest.ResponseRecorder {
		var data []byte
		if body != nil {
			data, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(data))
		req.Header.Set("Authorization", "Bearer "+token.Token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	return s, request, &task
}

func TestHealthTaskDefaultsAndValidation(t *testing.T) {
	s, request, task := healthServer(t)
	if got := DefaultTask().HealthCheck; got.Enabled || got.IntervalMinutes != 10 || got.ConsecutiveFailures != 3 || got.CooldownMinutes != 30 {
		t.Fatalf("defaults=%+v", got)
	}
	task.HealthCheck = store.HealthCheckConfig{}
	if err := s.normalizeTask(task); err != nil {
		t.Fatal(err)
	}
	if task.HealthCheck.IntervalMinutes != 10 {
		t.Fatalf("legacy=%+v", task.HealthCheck)
	}
	cfg := store.HealthCheckConfig{Enabled: true, IntervalMinutes: 10, MaxLatency: 200, ConsecutiveFailures: 3, CooldownMinutes: 30}
	for _, mutate := range []func(*store.HealthCheckConfig){func(c *store.HealthCheckConfig) { c.IntervalMinutes = 0 }, func(c *store.HealthCheckConfig) { c.IntervalMinutes = 1441 }, func(c *store.HealthCheckConfig) { c.MaxLatency = 0 }, func(c *store.HealthCheckConfig) { c.MinSpeed = -1 }, func(c *store.HealthCheckConfig) { c.ConsecutiveFailures = 21 }, func(c *store.HealthCheckConfig) { c.CooldownMinutes = 10081 }} {
		task.HealthCheck = cfg
		mutate(&task.HealthCheck)
		w := request(http.MethodPut, fmt.Sprintf("/api/tasks/%d", task.ID), task)
		if w.Code != 400 {
			t.Fatalf("config=%+v response=%d %s", task.HealthCheck, w.Code, w.Body)
		}
	}
	task.HealthCheck = cfg
	task.HealthCheck.MaxLatency = math.NaN()
	if err := s.normalizeTask(task); err == nil {
		t.Fatal("NaN accepted")
	}
	task.HealthCheck = cfg
	task.HealthCheck.MinSpeed = math.Inf(1)
	if err := s.normalizeTask(task); err == nil {
		t.Fatal("Inf accepted")
	}
	task.HealthCheck = cfg
	task.HealthCheck.CooldownMinutes = 0
	if w := request(http.MethodPut, fmt.Sprintf("/api/tasks/%d", task.ID), task); w.Code != 200 {
		t.Fatalf("valid=%d %s", w.Code, w.Body)
	}
}

func TestHealthAPIHistoryLimitDefaultsCloneAndBusyGuards(t *testing.T) {
	s, request, task := healthServer(t)
	path := fmt.Sprintf("/api/tasks/%d/health", task.ID)
	w := request(http.MethodGet, path, nil)
	if w.Code != 200 {
		t.Fatalf("health=%d %s", w.Code, w.Body)
	}
	var empty map[string]json.RawMessage
	json.Unmarshal(w.Body.Bytes(), &empty)
	if string(empty["items"]) != "[]" || len(empty["state"]) == 0 {
		t.Fatalf("empty=%s", w.Body)
	}
	for i := 0; i < 205; i++ {
		s.Store.CreateHealthCheck(&store.HealthCheck{TaskID: task.ID, CheckedAt: time.Now().Add(time.Duration(i) * time.Second), Status: "unknown"})
	}
	for _, tc := range []struct {
		query string
		want  int
	}{{"", 50}, {"?limit=500", 200}, {"?limit=2", 2}} {
		w = request(http.MethodGet, path+tc.query, nil)
		var response struct{ Items []store.HealthCheck }
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if len(response.Items) != tc.want || response.Items[0].CheckedAt.Before(response.Items[1].CheckedAt) || response.Items[0].Results == nil {
			t.Fatalf("history=%s", w.Body)
		}
	}
	s.Store.SaveHealthState(store.HealthState{TaskID: task.ID, ConsecutiveFailures: 3})
	w = request(http.MethodPost, fmt.Sprintf("/api/tasks/%d/clone", task.ID), nil)
	if w.Code != 200 {
		t.Fatalf("clone=%s", w.Body)
	}
	var clone taskView
	json.Unmarshal(w.Body.Bytes(), &clone)
	state, _ := s.Store.GetHealthState(clone.ID)
	if state.ConsecutiveFailures != 0 {
		t.Fatal("cloned runtime state")
	}
	task.HealthCheck = store.HealthCheckConfig{Enabled: true, IntervalMinutes: 10, MaxLatency: 100, ConsecutiveFailures: 3, CooldownMinutes: 30}
	s.Store.DB.Save(task)
	if err := s.Engine.EnqueueHealth(task.ID); err != nil {
		t.Fatal(err)
	}
	w = request(http.MethodDelete, fmt.Sprintf("/api/tasks/%d", task.ID), nil)
	if w.Code != 409 {
		t.Fatalf("delete active health=%d", w.Code)
	}
	backup := request(http.MethodGet, "/api/backup", nil)
	var b map[string]any
	json.Unmarshal(backup.Body.Bytes(), &b)
	w = request(http.MethodPost, "/api/backup/restore", b)
	if w.Code != 409 {
		t.Fatalf("restore active health=%d %s", w.Code, w.Body)
	}
}

func TestHealthBackupRoundtripAndLegacyCompatibility(t *testing.T) {
	s, request, task := healthServer(t)
	until := time.Now().Add(time.Hour)
	runID := uint(999)
	s.Store.SaveHealthState(store.HealthState{TaskID: task.ID, ConsecutiveFailures: 2, CooldownUntil: &until, LastTriggeredRunID: &runID})
	s.Store.CreateHealthCheck(&store.HealthCheck{TaskID: task.ID, CheckedAt: time.Now(), Status: "unhealthy", TriggeredRunID: &runID, Results: []store.HealthResult{{IP: "2606:4700::1", Status: "unhealthy", Reason: "latency"}}})
	w := request(http.MethodGet, "/api/backup", nil)
	var b map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if b["healthStates"] == nil || b["healthChecks"] == nil {
		t.Fatalf("missing health backup: %s", w.Body)
	}
	w = request(http.MethodPost, "/api/backup/restore", b)
	if w.Code != 200 {
		t.Fatalf("restore=%d %s", w.Code, w.Body)
	}
	state, _ := s.Store.GetHealthState(task.ID)
	items, _ := s.Store.ListHealthChecks(task.ID, 50)
	if state.ConsecutiveFailures != 2 || state.CooldownUntil == nil || state.LastTriggeredRunID != nil || len(items) != 1 || items[0].TriggeredRunID != nil {
		t.Fatalf("state=%+v checks=%+v", state, items)
	}
	// Invalid backup must roll back the complete restore, including health history.
	bad := map[string]any{}
	for k, v := range b {
		bad[k] = v
	}
	bad["healthStates"] = []map[string]any{{"taskId": 12345}}
	w = request(http.MethodPost, "/api/backup/restore", bad)
	if w.Code == 200 {
		t.Fatal("orphan health state accepted")
	}
	items, _ = s.Store.ListHealthChecks(task.ID, 50)
	if len(items) != 1 {
		t.Fatal("restore failed to roll back history")
	}
	delete(b, "healthStates")
	delete(b, "healthChecks")
	tasks := b["tasks"].([]any)
	delete(tasks[0].(map[string]any), "healthCheck")
	// Legacy import can retain incomplete tasks in a disabled state.
	tasks[0].(map[string]any)["enabled"] = false
	tasks[0].(map[string]any)["targets"] = []any{}
	w = request(http.MethodPost, "/api/backup/restore", b)
	if w.Code != 200 {
		t.Fatalf("legacy restore=%d %s", w.Code, w.Body)
	}
	restored, _ := s.Store.GetTask(task.ID)
	if restored.HealthCheck.Enabled || restored.HealthCheck.IntervalMinutes != 10 || restored.Update.RecordCount != 1 {
		t.Fatalf("legacy task=%+v", restored)
	}
	items, _ = s.Store.ListHealthChecks(task.ID, 50)
	if len(items) != 0 {
		t.Fatal("old health history survived legacy restore")
	}
}
