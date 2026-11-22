package notify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func TestProxyFieldMetadataAndValidation(t *testing.T) {
	hasProxy := func(m schema.TypeMeta) bool {
		for _, f := range m.Fields {
			if f.Key == "proxy" {
				return true
			}
		}
		return false
	}

	for _, m := range Metas() {
		if m.Type == "smtp" {
			if hasProxy(m) {
				t.Errorf("smtp should not expose HTTP proxy field")
			}
		} else if !hasProxy(m) {
			t.Errorf("%s should expose proxy field", m.Type)
		}
	}

	// 非法代理协议/格式在 Validate 与 New 阶段即拒绝
	for _, bad := range []string{"ftp://127.0.0.1:21", "127.0.0.1:7890", "http://"} {
		if err := Validate("telegram", schema.Config{"botToken": "t", "chatId": "1", "proxy": bad}); err == nil {
			t.Errorf("Validate accepted bad proxy %q", bad)
		}
		if _, err := New("webhook", schema.Config{"url": "https://example.com", "proxy": bad}); err == nil {
			t.Errorf("New accepted bad proxy %q", bad)
		}
	}

	for _, good := range []string{"", "http://127.0.0.1:7890", "https://proxy.example:443", "socks5://127.0.0.1:1080", "socks5h://user:pass@127.0.0.1:1080"} {
		if err := Validate("telegram", schema.Config{"botToken": "t", "chatId": "1", "proxy": good}); err != nil {
			t.Errorf("Validate rejected valid proxy %q: %v", good, err)
		}
	}
}

func TestTelegramAndWebhookSendViaProxy(t *testing.T) {
	var seenTargets []string
	var seenBodies []string
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenTargets = append(seenTargets, r.URL.String())
		b, _ := io.ReadAll(r.Body)
		seenBodies = append(seenBodies, string(b))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer proxySrv.Close()

	tg := mustNew(t, "telegram", schema.Config{
		"botToken": "123:abc",
		"chatId":   "42",
		"apiBase":  "http://telegram.example.invalid",
		"proxy":    proxySrv.URL,
	})
	if err := tg.Send(context.Background(), testMsg); err != nil {
		t.Fatalf("telegram via proxy failed: %v", err)
	}

	wh := mustNew(t, "webhook", schema.Config{
		"url":    "http://webhook.example.invalid/hook",
		"method": "POST",
		"proxy":  proxySrv.URL,
	})
	if err := wh.Send(context.Background(), testMsg); err != nil {
		t.Fatalf("webhook via proxy failed: %v", err)
	}

	if len(seenTargets) != 2 || !strings.Contains(seenTargets[0], "telegram.example.invalid") || !strings.Contains(seenTargets[1], "webhook.example.invalid") {
		t.Fatalf("unexpected proxy targets: %v", seenTargets)
	}
	if len(seenBodies) != 2 || !strings.Contains(seenBodies[0], testMsg.Title) || !strings.Contains(seenBodies[1], testMsg.Title) {
		t.Fatalf("unexpected proxy bodies: %v", seenBodies)
	}
}
