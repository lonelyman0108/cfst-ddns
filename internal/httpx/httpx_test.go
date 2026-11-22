package httpx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateProxyURL(t *testing.T) {
	for _, raw := range []string{"", "  ", "http://127.0.0.1:7890", "HTTPS://proxy.example:443", "socks5://127.0.0.1:1080", "socks5h://user:pass@[::1]:1080"} {
		u, err := ValidateProxyURL(raw)
		if err != nil {
			t.Errorf("ValidateProxyURL(%q) unexpected err: %v", raw, err)
		}
		if strings.TrimSpace(raw) == "" && u != nil {
			t.Errorf("ValidateProxyURL(%q) = %v, want nil", raw, u)
		}
	}
	for _, raw := range []string{
		"ftp://127.0.0.1:21",
		"127.0.0.1:7890",
		"http://",
		"http://:7890",
		"http://[::1",
		"http://127.0.0.1:0",
		"http://127.0.0.1:70000",
		"socks5://127.0.0.1",
		"socks5h://proxy.local",
	} {
		if _, err := ValidateProxyURL(raw); err == nil {
			t.Errorf("ValidateProxyURL(%q) expected error", raw)
		}
	}
}

func TestDoRoutesThroughProxyAndRedactsCredentials(t *testing.T) {
	proxied := false
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied = true
		if r.URL.Host != "notify.example.invalid" {
			t.Fatalf("unexpected target host: %s", r.URL.String())
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer proxySrv.Close()

	resp, err := Do(context.Background(), Request{
		Method:   http.MethodPost,
		URL:      "http://notify.example.invalid/send",
		JSON:     map[string]string{"hi": "1"},
		ProxyURL: proxySrv.URL,
		Retries:  1,
	})
	if err != nil {
		t.Fatalf("Do via proxy failed: %v", err)
	}
	if !proxied || !resp.OK() {
		t.Fatalf("proxied=%v status=%d", proxied, resp.Status)
	}

	_, err = Do(context.Background(), Request{
		Method:   http.MethodGet,
		URL:      "http://notify.example.invalid/fail?token=abc",
		ProxyURL: "http://alice:supersecretpass@127.0.0.1:1",
		Retries:  1,
	})
	if err == nil {
		t.Fatal("expected failure from unreachable proxy")
	}
	if strings.Contains(err.Error(), "supersecretpass") || strings.Contains(err.Error(), "token=abc") {
		t.Fatalf("secret leaked in error: %v", err)
	}
}
