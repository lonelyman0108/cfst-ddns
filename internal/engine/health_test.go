package engine

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/cfst"
	"github.com/lonelyman0108/cfst-ddns/internal/logbus"
	"github.com/lonelyman0108/cfst-ddns/internal/provider"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
	"github.com/lonelyman0108/cfst-ddns/internal/secret"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

func TestEvaluateHealthUsesConfiguredThresholds(t *testing.T) {
	cfg := store.HealthCheckConfig{MaxLatency: 200, MinSpeed: 5}
	good, reason := EvaluateHealth([]store.HealthResult{{IP: "1.1.1.1", Latency: 120, Speed: 8}}, cfg)
	if !good || reason != "" {
		t.Fatalf("good = %v reason=%q", good, reason)
	}
	bad, reason := EvaluateHealth([]store.HealthResult{{IP: "1.1.1.1", Latency: 120, Speed: 4.9}}, cfg)
	if bad || reason != "speed" {
		t.Fatalf("speed bad = %v reason=%q", bad, reason)
	}
	bad, reason = EvaluateHealth([]store.HealthResult{{IP: "1.1.1.1", Latency: 201, Speed: 8}}, cfg)
	if bad || reason != "latency" {
		t.Fatalf("latency bad = %v reason=%q", bad, reason)
	}
}

func TestEvaluateHealthUnknownAndSkippedDoNotCountAsHealthy(t *testing.T) {
	cfg := store.HealthCheckConfig{MaxLatency: 200}
	if healthy, reason := EvaluateHealth([]store.HealthResult{{IP: "1.1.1.1", Reason: "provider", Healthy: false}}, cfg); healthy || reason != "unknown" {
		t.Fatalf("unknown = %v reason=%q", healthy, reason)
	}
	if healthy, reason := EvaluateHealth([]store.HealthResult{{IP: "1.1.1.1", Reason: "proxied", Healthy: false}}, cfg); healthy || reason != "skipped" {
		t.Fatalf("skipped = %v reason=%q", healthy, reason)
	}
}

func TestShouldTriggerHealthAfterConsecutiveFailuresAndCooldown(t *testing.T) {
	cfg := store.HealthCheckConfig{ConsecutiveFailures: 3, CooldownMinutes: 30}
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	if ShouldTriggerHealth(store.HealthState{ConsecutiveFailures: 1}, now, cfg, true) {
		t.Fatal("should not trigger before threshold")
	}
	if !ShouldTriggerHealth(store.HealthState{ConsecutiveFailures: 2}, now, cfg, true) {
		t.Fatal("should trigger at threshold")
	}
	cooldown := now.Add(10 * time.Minute)
	if ShouldTriggerHealth(store.HealthState{ConsecutiveFailures: 3, CooldownUntil: &cooldown}, now, cfg, true) {
		t.Fatal("should not trigger during cooldown")
	}
	if ShouldTriggerHealth(store.HealthState{ConsecutiveFailures: 3}, now, cfg, false) {
		t.Fatal("healthy result should not trigger")
	}
}

func healthEngine(t *testing.T) (*Engine, *store.Task, *fakeProvider) {
	t.Helper()
	dir := t.TempDir()
	box, err := secret.New("health-test")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "db"), box)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := New(st, &cfst.Manager{Dir: filepath.Join(dir, "cfst"), Log: log}, logbus.NewHub(), log, dir)
	fp := &fakeProvider{records: map[string]provider.Record{"1": {ID: "1", Name: "cdn", Type: "A", Value: "1.1.1.1"}}}
	e.ProviderFactory = func(string, schema.Config) (provider.Provider, error) { return fp, nil }
	e.Probe = func(context.Context, []string, bool) []store.NetworkProbe { return nil }
	e.HealthProbe = func(_ context.Context, opt cfst.RunOptions, _ cfst.Output) ([]store.SpeedResult, error) {
		out := []store.SpeedResult{}
		for _, ip := range opt.IPs {
			out = append(out, store.SpeedResult{IP: ip, Latency: 250, Speed: 2})
		}
		return out, nil
	}
	acc := store.Account{Name: "test", Provider: "cloudflare", Config: schema.Config{"apiToken": "test"}}
	if err := st.SaveAccount(&acc); err != nil {
		t.Fatal(err)
	}
	task := &store.Task{Name: "health", Enabled: true, IPType: "v4", SpeedTest: store.SpeedTestConfig{MaxLossRate: 1}, Update: store.UpdatePolicy{RecordCount: 1, SkipUnchanged: true}, HealthCheck: store.HealthCheckConfig{Enabled: true, IntervalMinutes: 10, MaxLatency: 200, ConsecutiveFailures: 3, CooldownMinutes: 30}, Targets: []store.Target{{AccountID: acc.ID, Domain: "example.com", RR: "cdn"}}}
	if err := st.DB.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	return e, task, fp
}

func TestHealthCollectAuthoritativeRecordsAndSkipWholeProxiedTarget(t *testing.T) {
	e, task, fp := healthEngine(t)
	task.IPType = "both"
	task.Targets = append(task.Targets, store.Target{AccountID: task.Targets[0].AccountID, Domain: "example.com", RR: "other"})
	fp.records["2"] = provider.Record{ID: "2", Name: "cdn", Type: "A", Value: "2.2.2.2", Line: "other"}
	fp.records["3"] = provider.Record{ID: "3", Name: "cdn", Type: "AAAA", Value: "2606:4700::1", Proxied: true}
	fp.records["4"] = provider.Record{ID: "4", Name: "other", Type: "A", Value: "1.1.1.1"}
	e.Store.UpsertRecordState(store.RecordState{TaskID: task.ID, AccountID: task.Targets[0].AccountID, FQDN: "other.example.com", Type: "A", Value: "9.9.9.9"})
	ips, results, err := e.CollectCurrentIPs(context.Background(), task)
	if err != nil || len(ips) != 1 || ips[0].IP != "1.1.1.1" || !strings.Contains(ips[0].TargetKey, "other.example.com") {
		t.Fatalf("ips=%+v results=%+v err=%v", ips, results, err)
	}
	skipped := 0
	for _, r := range results {
		if r.Reason == "proxied" {
			skipped++
		}
	}
	if skipped != 2 {
		t.Fatalf("results=%+v", results)
	}
}

func TestHealthMeasurementsDeduplicateAndDisableFilters(t *testing.T) {
	e, task, fp := healthEngine(t)
	task.Targets = append(task.Targets, store.Target{AccountID: task.Targets[0].AccountID, Domain: "example.com", RR: "other"})
	task.HealthCheck.MinSpeed = 5
	task.SpeedTest.DisableDownload = true
	task.SpeedTest.ExtraArgs = "-dd -f bad -tl 1 -dn 1"
	task.SpeedTest.MinLatency = 40
	e.Store.DB.Save(task)
	fp.records["2"] = provider.Record{ID: "2", Name: "other", Type: "A", Value: "1.1.1.1"}
	calls := 0
	e.HealthProbe = func(_ context.Context, opt cfst.RunOptions, _ cfst.Output) ([]store.SpeedResult, error) {
		calls++
		if !reflect.DeepEqual(opt.IPs, []string{"1.1.1.1"}) || opt.Config.DisableDownload || opt.Config.DownloadCount != 1 || opt.Config.ExtraArgs != "" || opt.Config.MinLatency != 0 || opt.Config.MinSpeed != 0 || opt.Config.MaxLossRate != 1 {
			t.Fatalf("opt=%+v", opt)
		}
		return []store.SpeedResult{{IP: "1.1.1.1", Latency: 30, Speed: 6}}, nil
	}
	result, err := e.HealthJob(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	var check store.HealthCheck
	e.Store.DB.First(&check, result.CheckID)
	if calls != 1 || check.Status != "healthy" || len(check.Results) != 2 {
		t.Fatalf("calls=%d check=%+v", calls, check)
	}
}

func TestHealthFailureCooldownAndSnapshotReset(t *testing.T) {
	e, task, fp := healthEngine(t)
	for i := 0; i < 3; i++ {
		r, err := e.HealthJob(context.Background(), task.ID)
		if err != nil || r.TriggerFullRun != (i == 2) {
			t.Fatalf("check %d=%+v err=%v", i, r, err)
		}
	}
	state, _ := e.Store.GetHealthState(task.ID)
	if state.CooldownUntil != nil {
		t.Fatal("cooldown persisted before enqueue")
	}
	checks, _ := e.Store.ListHealthChecks(task.ID, 1)
	id, err := e.enqueue(task.ID, "health", false, checks[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	state, _ = e.Store.GetHealthState(task.ID)
	if state.CooldownUntil == nil || state.LastTriggeredRunID == nil || *state.LastTriggeredRunID != id {
		t.Fatalf("state=%+v", state)
	}
	var run store.Run
	e.Store.DB.First(&run, id)
	if run.HealthCheckID == nil || run.HealthReason != "latency" {
		t.Fatalf("run=%+v", run)
	}
	<-e.queue
	e.finishPending(&run)
	r, err := e.HealthJob(context.Background(), task.ID)
	if err != nil || r.TriggerFullRun {
		t.Fatalf("cooldown check=%+v err=%v", r, err)
	}
	fp.records["1"] = provider.Record{ID: "1", Name: "cdn", Type: "A", Value: "2.2.2.2"}
	e.HealthJob(context.Background(), task.ID)
	state, _ = e.Store.GetHealthState(task.ID)
	if state.ConsecutiveFailures != 1 || state.CooldownUntil == nil {
		t.Fatalf("snapshot state=%+v", state)
	}
	task.HealthCheck.MaxLatency = 100
	e.Store.DB.Save(task)
	e.HealthJob(context.Background(), task.ID)
	state, _ = e.Store.GetHealthState(task.ID)
	if state.ConsecutiveFailures != 1 {
		t.Fatalf("config state=%+v", state)
	}
}

func TestHealthUnknownAndMissingRecords(t *testing.T) {
	e, task, fp := healthEngine(t)
	e.HealthJob(context.Background(), task.ID)
	e.ProviderFactory = func(string, schema.Config) (provider.Provider, error) { return nil, errors.New("provider unavailable") }
	r, err := e.HealthJob(context.Background(), task.ID)
	if err != nil || r.TriggerFullRun {
		t.Fatalf("unknown=%+v err=%v", r, err)
	}
	state, _ := e.Store.GetHealthState(task.ID)
	if state.ConsecutiveFailures != 1 || state.LastHealthy != nil || state.Status != "unknown" {
		t.Fatalf("state=%+v", state)
	}
	e.ProviderFactory = func(string, schema.Config) (provider.Provider, error) { return fp, nil }
	e.HealthProbe = func(context.Context, cfst.RunOptions, cfst.Output) ([]store.SpeedResult, error) {
		return nil, context.DeadlineExceeded
	}
	e.HealthJob(context.Background(), task.ID)
	state, _ = e.Store.GetHealthState(task.ID)
	if state.ConsecutiveFailures != 2 || state.Status != "unhealthy" {
		t.Fatalf("timeout state=%+v", state)
	}
	fp.records = map[string]provider.Record{}
	e.HealthJob(context.Background(), task.ID)
	state, _ = e.Store.GetHealthState(task.ID)
	if state.ConsecutiveFailures != 1 || state.Status != "unhealthy" {
		t.Fatalf("missing state=%+v", state)
	}
}

func TestHealthDeadlineRemainsUnhealthy(t *testing.T) {
	e, task, _ := healthEngine(t)
	e.HealthProbe = func(ctx context.Context, _ cfst.RunOptions, _ cfst.Output) ([]store.SpeedResult, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	result, err := e.HealthJob(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	var check store.HealthCheck
	if err := e.Store.DB.First(&check, result.CheckID).Error; err != nil {
		t.Fatal(err)
	}
	if check.Status != store.HealthStatusUnhealthy || check.FailureReason != "timeout" {
		t.Fatalf("check=%+v", check)
	}
	state, _ := e.Store.GetHealthState(task.ID)
	if state.ConsecutiveFailures != 1 {
		t.Fatalf("state=%+v", state)
	}
}
func TestHealthUnsupportedLineIsUnknown(t *testing.T) {
	e, task, _ := healthEngine(t)
	task.Targets[0].Line = "telecom"
	ips, results, err := e.CollectCurrentIPs(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 0 || len(results) == 0 || results[0].Status != store.HealthStatusUnknown || results[0].Reason != "unsupportedLine" {
		t.Fatalf("ips=%+v results=%+v", ips, results)
	}
}

func TestHealthRequiredCountsArePerTarget(t *testing.T) {
	counts := healthRequiredCounts([]store.HealthResult{
		{TargetKey: "a", IP: "1.1.1.1", Status: store.HealthStatusUnhealthy},
		{TargetKey: "a", IP: "2.2.2.2", Status: store.HealthStatusUnhealthy},
		{TargetKey: "a", IP: "3.3.3.3", Status: store.HealthStatusHealthy},
		{TargetKey: "b", IP: "4.4.4.4", Status: store.HealthStatusHealthy},
		{TargetKey: "b", IP: "", Status: store.HealthStatusSkipped},
	}, 1)
	if counts["a"] != 3 || counts["b"] != 1 {
		t.Fatalf("counts=%v", counts)
	}
}

func TestHealthQueueDedupBusyAndFullQueueDoesNotConsumeCooldown(t *testing.T) {
	e, task, _ := healthEngine(t)
	if err := e.EnqueueHealth(task.ID); err != nil {
		t.Fatal(err)
	}
	if !e.Busy() {
		t.Fatal("health invisible to Busy")
	}
	if _, running := e.Running(task.ID); !running {
		t.Fatal("health invisible to Running")
	}
	if err := e.EnqueueHealth(task.ID); !errors.Is(err, ErrBusy) {
		t.Fatalf("duplicate=%v", err)
	}
	if _, err := e.Enqueue(task.ID, "manual", false); err != nil {
		t.Fatal(err)
	}
	e.executeJob(context.Background(), <-e.queue)
	checks, _ := e.Store.ListHealthChecks(task.ID, 1)
	if len(checks) != 1 || checks[0].Status != "skipped" || checks[0].FailureReason != "busy" {
		t.Fatalf("checks=%+v", checks)
	}
	j := <-e.queue
	e.finishPending(&store.Run{TaskID: task.ID, ID: j.runID})
	task.HealthCheck.ConsecutiveFailures = 1
	e.Store.DB.Save(task)
	r, err := e.HealthJob(context.Background(), task.ID)
	if err != nil || !r.TriggerFullRun {
		t.Fatalf("r=%+v err=%v", r, err)
	}
	for i := 0; i < cap(e.queue); i++ {
		e.queue <- job{kind: "full"}
	}
	if _, err := e.enqueue(task.ID, "health", false, r.CheckID); err == nil {
		t.Fatal("full queue accepted")
	}
	state, _ := e.Store.GetHealthState(task.ID)
	if state.CooldownUntil != nil || state.LastTriggeredRunID != nil {
		t.Fatalf("state=%+v", state)
	}
}

func installHealthFakeCFST(t *testing.T, e *Engine) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(e.CFST.Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.CFST.BinPath(), bin, 0755); err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{"v4", "v6"} {
		if err := os.WriteFile(e.CFST.IPFile(typ), []byte("1.2.3.0/24"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("FAKE_CFST", "1")
}

func TestHealthFullRunThresholdsAndProxiedTargets(t *testing.T) {
	for _, tc := range []struct {
		name        string
		latency     float64
		normalSpeed float64
		count       int
		wantChanged bool
	}{
		{"health threshold", 40, 0, 1, false}, {"normal threshold", 200, 20, 1, false}, {"top N", 200, 0, 2, false}, {"qualified", 200, 0, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, task, fp := healthEngine(t)
			installHealthFakeCFST(t, e)
			task.HealthCheck.MaxLatency = tc.latency
			task.HealthCheck.ConsecutiveFailures = 1
			task.SpeedTest.MinSpeed = tc.normalSpeed
			task.SpeedTest.DisableDownload = true
			task.Update.RecordCount = tc.count
			task.Targets = append(task.Targets, store.Target{AccountID: task.Targets[0].AccountID, Domain: "example.com", RR: "proxy"})
			fp.records["p"] = provider.Record{ID: "p", Name: "proxy", Type: "A", Value: "9.9.9.9", Proxied: true}
			if err := e.Store.DB.Save(task).Error; err != nil {
				t.Fatal(err)
			}
			check, err := e.HealthJob(context.Background(), task.ID)
			if err != nil || !check.TriggerFullRun {
				t.Fatalf("check=%+v err=%v", check, err)
			}
			id, err := e.enqueue(task.ID, "health", false, check.CheckID)
			if err != nil {
				t.Fatal(err)
			}
			e.executeJob(context.Background(), <-e.queue)
			var run store.Run
			e.Store.DB.First(&run, id)
			if run.Changed != tc.wantChanged || fp.records["p"].Value != "9.9.9.9" || !fp.records["p"].Proxied {
				t.Fatalf("run=%+v records=%+v", run, fp.records)
			}
			if !tc.wantChanged && fp.records["1"].Value != "1.1.1.1" {
				t.Fatalf("unqualified update: %+v", fp.records)
			}
			state, _ := e.Store.GetHealthState(task.ID)
			if state.CooldownUntil == nil || (tc.wantChanged && state.ConsecutiveFailures != 0) || (!tc.wantChanged && state.ConsecutiveFailures == 0) {
				t.Fatalf("state=%+v", state)
			}
		})
	}
}

func TestHealthFullRunRejectsChangedSnapshot(t *testing.T) {
	e, task, fp := healthEngine(t)
	installHealthFakeCFST(t, e)
	task.HealthCheck.ConsecutiveFailures = 1
	e.Store.DB.Save(task)
	check, err := e.HealthJob(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	id, err := e.enqueue(task.ID, "health", false, check.CheckID)
	if err != nil {
		t.Fatal(err)
	}
	fp.records["1"] = provider.Record{ID: "1", Name: "cdn", Type: "A", Value: "8.8.8.8"}
	e.executeJob(context.Background(), <-e.queue)
	var run store.Run
	e.Store.DB.First(&run, id)
	if run.Changed || fp.records["1"].Value != "8.8.8.8" {
		t.Fatalf("run=%+v records=%+v", run, fp.records)
	}
}

func TestHealthPartialMissingTargetStaysSkipped(t *testing.T) {
	e, task, fp := healthEngine(t)
	installHealthFakeCFST(t, e)
	task.HealthCheck.ConsecutiveFailures = 1
	task.Targets = append(task.Targets, store.Target{AccountID: task.Targets[0].AccountID, Domain: "example.com", RR: "missing"})
	e.Store.DB.Save(task)
	check, err := e.HealthJob(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.enqueue(task.ID, "health", false, check.CheckID); err != nil {
		t.Fatal(err)
	}
	e.executeJob(context.Background(), <-e.queue)
	for _, r := range fp.records {
		if r.Name == "missing" {
			t.Fatalf("skipped missing target was modified: %+v", fp.records)
		}
	}
}

func TestHealthUnknownPreservesProviderErrorDetailsAndMappings(t *testing.T) {
	e, task, _ := healthEngine(t)
	task.Targets = append(task.Targets, store.Target{AccountID: 9999, Domain: "example.com", RR: "missing"})
	e.Store.DB.Save(task)
	r, err := e.HealthJob(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	var check store.HealthCheck
	e.Store.DB.First(&check, r.CheckID)
	if check.Status != "unknown" || len(check.Results) != 2 {
		t.Fatalf("check=%+v", check)
	}
	foundDetail := false
	for _, r := range check.Results {
		if r.Detail != "" {
			foundDetail = true
		}
	}
	if !foundDetail {
		t.Fatal("provider error details lost")
	}
}

func TestHealthWorkerSerializesAndTransitionsToFullRun(t *testing.T) {
	e, task, _ := healthEngine(t)
	installHealthFakeCFST(t, e)
	task.HealthCheck.ConsecutiveFailures = 1
	e.Store.DB.Save(task)
	other := *task
	other.ID = 0
	other.Name = "manual"
	e.Store.DB.Create(&other)
	entered, release := make(chan struct{}), make(chan struct{})
	e.HealthProbe = func(ctx context.Context, opt cfst.RunOptions, out cfst.Output) ([]store.SpeedResult, error) {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return []store.SpeedResult{{IP: "1.1.1.1", Latency: 250}}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.Start(ctx)
	if err := e.EnqueueHealth(task.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("health did not start")
	}
	id, err := e.Enqueue(other.ID, "manual", true)
	if err != nil {
		t.Fatal(err)
	}
	var queued store.Run
	e.Store.DB.First(&queued, id)
	if queued.Status != store.StatusQueued {
		t.Fatalf("concurrent full run=%s", queued.Status)
	}
	if _, err := e.BeginMaintenance(); !errors.Is(err, ErrBusy) {
		t.Fatalf("health maintenance=%v", err)
	}
	close(release)
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("worker did not finish health and full runs")
		case <-ticker.C:
			if e.Busy() {
				continue
			}
			var runs []store.Run
			e.Store.DB.Order("id").Find(&runs)
			if len(runs) != 2 || runs[0].Status != store.StatusSuccess || runs[1].Trigger != "health" || runs[1].Status != store.StatusSuccess || runs[1].HealthCheckID == nil {
				t.Fatalf("runs=%+v", runs)
			}
			checks, _ := e.Store.ListHealthChecks(task.ID, 50)
			if len(checks) != 1 || checks[0].TriggeredRunID == nil || *checks[0].TriggeredRunID != runs[1].ID {
				t.Fatalf("checks=%+v", checks)
			}
			return
		}
	}
}

func TestHealthProxyAndMissingIndividualResult(t *testing.T) {
	e, task, _ := healthEngine(t)
	e.Probe = func(context.Context, []string, bool) []store.NetworkProbe {
		return []store.NetworkProbe{{IPType: "v4", Intercepted: true}}
	}
	calls := 0
	e.HealthProbe = func(context.Context, cfst.RunOptions, cfst.Output) ([]store.SpeedResult, error) {
		calls++
		return nil, nil
	}
	result, err := e.HealthJob(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	var check store.HealthCheck
	e.Store.DB.First(&check, result.CheckID)
	if calls != 0 || check.Status != "unknown" || check.Results[0].Reason != "proxyDetected" {
		t.Fatalf("proxy=%+v", check)
	}
	task.SpeedTest.NetworkMode = store.NetworkProxy
	e.Store.DB.Save(task)
	result, err = e.HealthJob(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	check = store.HealthCheck{}
	e.Store.DB.First(&check, result.CheckID)
	if calls != 1 || check.Status != "unhealthy" || check.Results[0].Reason != "missingResult" {
		t.Fatalf("missing=%+v", check)
	}
}

func TestHealthFailuresNotResetByForcedUnchangedDNS(t *testing.T) {
	e, task, fp := healthEngine(t)
	installHealthFakeCFST(t, e)
	task.Update.SkipUnchanged = false
	e.Store.DB.Save(task)
	fp.records["1"] = provider.Record{ID: "1", Name: "cdn", Type: "A", Value: "1.2.3.4"}
	e.Store.SaveHealthState(store.HealthState{TaskID: task.ID, ConsecutiveFailures: 2})
	id, err := e.Enqueue(task.ID, "manual", false)
	if err != nil {
		t.Fatal(err)
	}
	e.executeJob(context.Background(), <-e.queue)
	var run store.Run
	e.Store.DB.First(&run, id)
	state, _ := e.Store.GetHealthState(task.ID)
	if run.Status != store.StatusSuccess || run.Changed || state.ConsecutiveFailures != 2 {
		t.Fatalf("run=%+v state=%+v", run, state)
	}
}

func TestHealthNotificationsOnlyFromTriggeredFullRun(t *testing.T) {
	e, task, _ := healthEngine(t)
	installHealthFakeCFST(t, e)
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	notifier := store.Notifier{Name: "health-hook", Type: "webhook", Enabled: true, OnSuccess: true, OnFailure: true, OnlyOnChange: true, Config: schema.Config{"url": server.URL, "method": "POST", "contentType": "json"}}
	if err := e.Store.SaveNotifier(&notifier); err != nil {
		t.Fatal(err)
	}
	task.NotifierIDs = []uint{notifier.ID}
	task.HealthCheck.ConsecutiveFailures = 1
	task.HealthCheck.MinSpeed = 20
	e.Store.DB.Save(task)
	e.HealthProbe = func(context.Context, cfst.RunOptions, cfst.Output) ([]store.SpeedResult, error) {
		return []store.SpeedResult{{IP: "1.1.1.1", Latency: 50, Speed: 25}}, nil
	}
	result, err := e.HealthJob(context.Background(), task.ID)
	if err != nil || result.TriggerFullRun || hits.Load() != 0 {
		t.Fatalf("healthy=%+v err=%v hits=%d", result, err, hits.Load())
	}
	e.HealthProbe = func(context.Context, cfst.RunOptions, cfst.Output) ([]store.SpeedResult, error) {
		return []store.SpeedResult{{IP: "1.1.1.1", Latency: 50, Speed: 1}}, nil
	}
	result, err = e.HealthJob(context.Background(), task.ID)
	if err != nil || !result.TriggerFullRun || hits.Load() != 0 {
		t.Fatalf("unhealthy=%+v err=%v hits=%d", result, err, hits.Load())
	}
	id, err := e.enqueue(task.ID, "health", false, result.CheckID)
	if err != nil {
		t.Fatal(err)
	}
	e.executeJob(context.Background(), <-e.queue)
	var run store.Run
	e.Store.DB.First(&run, id)
	if run.Status != store.StatusFailed || hits.Load() != 1 {
		t.Fatalf("run=%+v hits=%d", run, hits.Load())
	}
	msg := BuildMessage("test", task, &run)
	if !strings.Contains(msg.Content, "健康检查") || !strings.Contains(msg.Content, "speed") {
		t.Fatalf("missing health context: %s", msg.Content)
	}
}
