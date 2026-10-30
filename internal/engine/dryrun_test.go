package engine

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/cfst"
	"github.com/lonelyman0108/cfst-ddns/internal/logbus"
	"github.com/lonelyman0108/cfst-ddns/internal/provider"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
	"github.com/lonelyman0108/cfst-ddns/internal/secret"
	"github.com/lonelyman0108/cfst-ddns/internal/store"

	_ "github.com/lonelyman0108/cfst-ddns/internal/notify"
)

// 测试二进制在 FAKE_CFST=1 时扮演 cfst：把一条结果写入 -o 指定的文件。
func TestMain(m *testing.M) {
	if os.Getenv("FAKE_CFST") == "1" {
		for i, a := range os.Args {
			if a == "-o" && i+1 < len(os.Args) {
				csv := "IP 地址,已发送,已接收,丢包率,平均延迟,下载速度(MB/s),地区码\n1.2.3.4,4,4,0.00,50.00,12.30,HKG\n"
				// FAKE_CFST_ROWS 追加更多结果行，用于验证结果保存上限
				n, _ := strconv.Atoi(os.Getenv("FAKE_CFST_ROWS"))
				// FAKE_CFST_LOCAL=1 模拟本地代理秒回握手：全部延迟约 1 ms
				local := os.Getenv("FAKE_CFST_LOCAL") == "1"
				if local {
					csv = "IP 地址,已发送,已接收,丢包率,平均延迟,下载速度(MB/s),地区码\n1.2.3.4,4,4,0.00,0.80,0.00,N/A\n"
				}
				for j := 1; j < n; j++ {
					lat := float64(50 + j)
					if local {
						lat = 0.8 + float64(j%5)*0.1
					}
					csv += fmt.Sprintf("1.2.%d.%d,4,4,0.00,%.2f,1.00,HKG\n", j/250, j%250+1, lat)
				}
				_ = os.WriteFile(os.Args[i+1], []byte(csv), 0o644)
			}
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

var dryFake = &fakeProvider{records: map[string]provider.Record{}}

func init() {
	provider.Register(schema.TypeMeta{Type: "enginefake", Name: "fake"},
		func(schema.Config) (provider.Provider, error) { return dryFake, nil })
}

func TestDryRun(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	bin, err := os.ReadFile(exe)
	if err != nil {
		t.Skip(err)
	}
	t.Setenv("FAKE_CFST", "1")

	dir := t.TempDir()
	box, err := secret.LoadOrCreate("", filepath.Join(dir, "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "test.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := &cfst.Manager{Dir: filepath.Join(dir, "cfst"), Log: log, Mirror: func() string { return "" }}
	os.MkdirAll(mgr.Dir, 0o755)
	os.WriteFile(mgr.BinPath(), bin, 0o755)
	os.WriteFile(mgr.IPFile("v4"), []byte("1.2.3.0/24\n"), 0o644)

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }))
	defer srv.Close()
	n := &store.Notifier{Name: "hook", Type: "webhook", Enabled: true, OnSuccess: true, OnFailure: true,
		Config: schema.Config{"url": srv.URL, "method": "POST", "contentType": "json"}}
	acc := &store.Account{Name: "fake", Provider: "enginefake", Config: schema.Config{}}
	if err := st.SaveNotifier(n); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveAccount(acc); err != nil {
		t.Fatal(err)
	}
	task := &store.Task{Name: "t", IPType: "v4", Update: store.UpdatePolicy{RecordCount: 1, SkipUnchanged: true},
		SpeedTest:   store.SpeedTestConfig{IPSource: "default", MaxLossRate: 1},
		Targets:     []store.Target{{AccountID: acc.ID, Domain: "example.com", RR: "cdn"}},
		NotifierIDs: []uint{n.ID}}
	if err := st.DB.Create(task).Error; err != nil {
		t.Fatal(err)
	}

	e := New(st, mgr, logbus.NewHub(), log, filepath.Join(dir, "tmp"))
	// 不访问真实网络：默认自检结果为直连
	probe := store.NetworkProbe{IPType: "v4", ConnectMs: 30, EgressIP: "203.0.113.1", LocalIP: "203.0.113.1"}
	var gotCanary bool
	e.Probe = func(_ context.Context, types []string, canary bool) []store.NetworkProbe {
		gotCanary = canary
		return []store.NetworkProbe{probe}
	}
	run := func(dry bool) store.Run {
		id, err := e.Enqueue(task.ID, "manual", dry)
		if err != nil {
			t.Fatal(err)
		}
		e.execute(context.Background(), <-e.queue)
		var r store.Run
		st.DB.First(&r, id)
		return r
	}

	r := run(true)
	if r.Status != store.StatusSuccess || !r.DryRun || r.BestIPv4 != "1.2.3.4" || len(r.Changes) != 0 {
		t.Fatalf("dry run = %+v", r)
	}
	if r.MessageKey != "dryRunDone" || r.Message != "试运行完成，未修改 DNS" {
		t.Fatalf("dry run message = %q / %q", r.MessageKey, r.Message)
	}
	if !strings.Contains(r.Log, "试运行：跳过 DNS 同步与通知") {
		t.Fatalf("log missing dry-run line:\n%s", r.Log)
	}
	var states int64
	st.DB.Model(&store.RecordState{}).Count(&states)
	if len(dryFake.records) != 0 || hits.Load() != 0 || states != 0 {
		t.Fatalf("试运行不应改 DNS / 发通知 / 写记录状态: records=%d hits=%d states=%d", len(dryFake.records), hits.Load(), states)
	}

	r = run(false)
	st.DB.Model(&store.RecordState{}).Count(&states)
	if r.Status != store.StatusSuccess || r.DryRun || !r.Changed || len(dryFake.records) != 1 || hits.Load() != 1 || states != 1 {
		t.Fatalf("normal run = %+v records=%d hits=%d states=%d", r, len(dryFake.records), hits.Load(), states)
	}
	if r.MessageKey != "dnsUpdated" {
		t.Fatalf("normal run messageKey = %q", r.MessageKey)
	}

	// 结果超过上限时只保存前 MaxStoredResults 个，并记录实际总数
	t.Setenv("FAKE_CFST_ROWS", "150")
	r = run(true)
	if len(r.Results) != MaxStoredResults || r.ResultTotals["v4"] != 150 || r.Results[0].IP != "1.2.3.4" {
		t.Fatalf("results=%d totals=%v first=%v", len(r.Results), r.ResultTotals, r.Results[0].IP)
	}
	if r.Network == nil || r.Network.Mode != store.NetworkDirect || r.Network.ProxySuspected || !gotCanary {
		t.Fatalf("network = %+v canary=%v", r.Network, gotCanary)
	}

	// 直连模式：保留地址握手成功 → 不测速、不改 DNS，但发失败通知
	before := hits.Load()
	dryFake.records = map[string]provider.Record{}
	probe.Intercepted = true
	r = run(false)
	if r.Status != store.StatusFailed || r.MessageKey != "proxyDetected" || r.Network.Reason != "intercepted" ||
		len(r.Results) != 0 || len(dryFake.records) != 0 || hits.Load() != before+1 {
		t.Fatalf("intercepted run = %+v network=%+v records=%d", r, r.Network, len(dryFake.records))
	}

	// 直连模式：保留地址被放行，但 TCPing 延迟全部约 1 ms → 保存结果，不改 DNS
	probe.Intercepted = false
	t.Setenv("FAKE_CFST_LOCAL", "1")
	t.Setenv("FAKE_CFST_ROWS", "20")
	r = run(false)
	if r.Status != store.StatusFailed || r.MessageKey != "proxyDetected" || r.Network.Reason != "latency" ||
		len(r.Results) != 20 || r.BestIPv4 != "" || len(dryFake.records) != 0 {
		t.Fatalf("latency run = %+v network=%+v records=%d", r, r.Network, len(dryFake.records))
	}

	// 经代理模式：不做保留地址探测与延迟判定，正常更新；出口变化写入日志
	task.SpeedTest.NetworkMode = store.NetworkProxy
	if err := st.DB.Save(task).Error; err != nil {
		t.Fatal(err)
	}
	probe.EgressIP = "198.51.100.7"
	r = run(false)
	if r.Status != store.StatusSuccess || gotCanary || r.Network.Mode != store.NetworkProxy || len(dryFake.records) != 1 {
		t.Fatalf("proxy run = %+v network=%+v canary=%v", r, r.Network, gotCanary)
	}
	probe.EgressIP = "198.51.100.8"
	r = run(false)
	if !strings.Contains(r.Log, "代理出口与上次不同（198.51.100.7 → 198.51.100.8）") {
		t.Fatalf("log missing egress change:\n%s", r.Log)
	}
}
