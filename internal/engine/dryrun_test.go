package engine

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
}
