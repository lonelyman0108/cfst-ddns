package engine

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
	"github.com/lonelyman0108/cfst-ddns/internal/secret"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
	"github.com/lonelyman0108/cfst-ddns/internal/update"
)

func TestNotifyUpdate(t *testing.T) {
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

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }))
	defer srv.Close()
	cfg := schema.Config{"url": srv.URL, "method": "POST", "contentType": "json"}
	on := &store.Notifier{Name: "on", Type: "webhook", Enabled: true, OnUpdate: true, Config: cfg}
	off := &store.Notifier{Name: "off", Type: "webhook", Enabled: true, OnSuccess: true, Config: cfg}
	for _, n := range []*store.Notifier{on, off} {
		if err := st.SaveNotifier(n); err != nil {
			t.Fatal(err)
		}
	}

	e := New(st, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), dir)
	info := update.Info{Available: true, Current: "v2.2.0", Latest: "v2.3.0", URL: "https://example.com"}
	e.NotifyUpdate(info)
	if hits.Load() != 1 {
		t.Fatalf("应只推送给开启提醒的渠道一次，实际 %d", hits.Load())
	}
	e.NotifyUpdate(info)
	if hits.Load() != 1 {
		t.Fatal("同一版本不应重复推送")
	}
	info.Latest = "v2.4.0"
	e.NotifyUpdate(info)
	if hits.Load() != 2 {
		t.Fatal("更新的版本应再次推送")
	}
}

func TestBuildUpdateMessage(t *testing.T) {
	notes := strings.Repeat("变", 600)
	m := BuildUpdateMessage("CFST DDNS", update.Info{Current: "v2.2.0", Latest: "v2.3.0", Notes: notes, URL: "https://x"})
	if m.Title != "CFST DDNS 发现新版本 v2.3.0" {
		t.Fatalf("title = %q", m.Title)
	}
	if strings.Contains(m.Content, notes) || !strings.Contains(m.Content, "…") || !strings.HasSuffix(m.Content, "https://x") {
		t.Fatalf("正文未截断或缺少链接: %q", m.Content[len(m.Content)-40:])
	}
}

func TestCleanNotes(t *testing.T) {
	body := "## [2.2.0](https://x/compare) (2026-10-30)\n\n\n### 新功能\n\n* 测速前检测代理 ([#12](https://x/12)) ([cb0e2fd](https://x/c))\n\n## Docker\n\n```bash\ndocker pull x\n```"
	got := cleanNotes(body, true)
	if got != "### 新功能\n\n* 测速前检测代理 (#12) (cb0e2fd)" {
		t.Fatalf("plain = %q", got)
	}
	if md := cleanNotes(body, false); !strings.Contains(md, "[#12](https://x/12)") || strings.Contains(md, "Docker") || strings.Contains(md, "2.2.0") {
		t.Fatalf("markdown = %q", md)
	}
}
