package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/cfst"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
	"github.com/lonelyman0108/cfst-ddns/internal/verify"
)

func TestVerifyFilterPicksFirstQualifiedAndAllFailedReturnsVerifyFailed(t *testing.T) {
	e, task, fp := healthEngine(t)
	installHealthFakeCFST(t, e)
	t.Setenv("FAKE_CFST_ROWS", "2")
	task.Verify = store.VerifyConfig{
		Enabled:        true,
		Mode:           store.VerifyModeWebSocket,
		Host:           "cdn.example.com",
		Path:           "/ws",
		TimeoutSeconds: 5,
		MaxCandidates:  15,
		Concurrency:    2,
	}
	e.Store.DB.Save(task)

	// Rank 1 (1.2.3.4) 失败，Rank 2 (1.2.0.2) 通过
	e.VerifyProbe = func(_ context.Context, ip string, _ store.VerifyConfig, _ int) verify.Result {
		if ip == "1.2.3.4" {
			return verify.Result{IP: ip, OK: false, Stage: "websocket", Status: 403, Error: "HTTP 403"}
		}
		return verify.Result{IP: ip, OK: true, Stage: "ok", Status: 101}
	}

	id, err := e.Enqueue(task.ID, "manual", false)
	if err != nil {
		t.Fatal(err)
	}
	e.executeJob(context.Background(), <-e.queue)
	var run store.Run
	e.Store.DB.First(&run, id)
	if run.Status != store.StatusSuccess || run.BestIPv4 != "1.2.0.2" || fp.records["1"].Value != "1.2.0.2" {
		t.Fatalf("run=%+v records=%+v", run, fp.records)
	}
	if !strings.Contains(run.Log, "✗ 验证失败: 1.2.3.4 [websocket] HTTP 403") || !strings.Contains(run.Log, "✓ 验证通过: 1.2.0.2") {
		t.Fatalf("log missing verify details:\n%s", run.Log)
	}

	// 全部候选未通过协议验证 -> errVerifyFailed，保留原 DNS，通知不输出最优 IP
	e.VerifyProbe = func(_ context.Context, ip string, _ store.VerifyConfig, _ int) verify.Result {
		return verify.Result{IP: ip, OK: false, Stage: "tls", Error: "handshake timeout"}
	}
	id, err = e.Enqueue(task.ID, "manual", false)
	if err != nil {
		t.Fatal(err)
	}
	e.executeJob(context.Background(), <-e.queue)
	run = store.Run{}
	e.Store.DB.First(&run, id)
	if run.Status != store.StatusFailed || run.MessageKey != "verifyFailed" || fp.records["1"].Value != "1.2.0.2" {
		t.Fatalf("all failed run=%+v records=%+v", run, fp.records)
	}
	msg := BuildMessage("CFST DDNS", task, &run)
	if strings.Contains(msg.Content, "最优 IP") {
		t.Fatalf("failed verify message should omit best IP:\n%s", msg.Content)
	}
}

func TestHealthCheckVerifyLinkageAndVerifyOnly(t *testing.T) {
	e, task, fp := healthEngine(t)
	installHealthFakeCFST(t, e)
	t.Setenv("FAKE_CFST_ROWS", "2")

	// 仅开启协议验证、健康检查不配置延迟与速度阈值
	task.HealthCheck = store.HealthCheckConfig{
		Enabled:             true,
		IntervalMinutes:     10,
		ConsecutiveFailures: 1,
		CooldownMinutes:     30,
	}
	task.Verify = store.VerifyConfig{
		Enabled:        true,
		Mode:           store.VerifyModeWebSocket,
		Host:           "cdn.example.com",
		Path:           "/ws",
		TimeoutSeconds: 5,
		MaxCandidates:  15,
		Concurrency:    2,
	}
	e.Store.DB.Save(task)

	// 验证 healthConfigFingerprint 感知 Verify 变更
	fpBefore := healthConfigFingerprint(task)
	task.Verify.Path = "/v2"
	if healthConfigFingerprint(task) == fpBefore {
		t.Fatal("healthConfigFingerprint did not change on Verify change")
	}
	task.Verify.Path = "/ws"
	e.Store.DB.Save(task)

	// cfst 测量显示 1.1.1.1 延迟 50ms 正常，但协议验证失败
	e.HealthProbe = func(_ context.Context, opt cfst.RunOptions, _ cfst.Output) ([]store.SpeedResult, error) {
		return []store.SpeedResult{{IP: opt.IPs[0], Latency: 50, Speed: 10}}, nil
	}
	e.VerifyProbe = func(_ context.Context, ip string, _ store.VerifyConfig, _ int) verify.Result {
		switch ip {
		case "1.1.1.1", "1.2.3.4":
			return verify.Result{IP: ip, OK: false, Stage: "websocket", Status: 403, Error: "HTTP 403"}
		default:
			return verify.Result{IP: ip, OK: true, Stage: "ok", Status: 101}
		}
	}

	res, err := e.HealthJob(context.Background(), task.ID)
	if err != nil || !res.TriggerFullRun {
		t.Fatalf("health res=%+v err=%v", res, err)
	}
	var check store.HealthCheck
	e.Store.DB.First(&check, res.CheckID)
	if check.Status != store.HealthStatusUnhealthy || check.FailureReason != "verifyFailed" ||
		len(check.Results) != 1 || check.Results[0].Reason != "verifyFailed" || check.Results[0].Detail != "[websocket] HTTP 403" {
		t.Fatalf("check=%+v", check)
	}

	id, err := e.enqueue(task.ID, "health", false, res.CheckID)
	if err != nil {
		t.Fatal(err)
	}
	e.executeJob(context.Background(), <-e.queue)
	var run store.Run
	e.Store.DB.First(&run, id)
	if run.Status != store.StatusSuccess || !run.Changed || fp.records["1"].Value != "1.2.0.2" {
		t.Fatalf("health-triggered run=%+v records=%+v", run, fp.records)
	}
}

func TestHealthSelectionConfigVerifyCandidateFloor(t *testing.T) {
	baseTask := func(verifyEnabled bool, maxCandidates int) *store.Task {
		return &store.Task{
			SpeedTest:   store.SpeedTestConfig{DownloadCount: 1},
			HealthCheck: store.HealthCheckConfig{Enabled: true, MinSpeed: 5},
			Verify:      store.VerifyConfig{Enabled: verifyEnabled, MaxCandidates: maxCandidates},
		}
	}

	if got := healthSelectionConfig(baseTask(false, 15), 1).DownloadCount; got != 3 {
		t.Fatalf("without verify DownloadCount = %d, want 3", got)
	}
	if got := healthSelectionConfig(baseTask(true, 15), 1).DownloadCount; got != 10 {
		t.Fatalf("with verify maxCandidates=15 DownloadCount = %d, want 10", got)
	}
	if got := healthSelectionConfig(baseTask(true, 5), 1).DownloadCount; got != 5 {
		t.Fatalf("with verify maxCandidates=5 DownloadCount = %d, want 5", got)
	}
	if got := healthSelectionConfig(baseTask(true, 15), 12).DownloadCount; got != 12 {
		t.Fatalf("with verify typeCount=12 DownloadCount = %d, want 12", got)
	}
}
