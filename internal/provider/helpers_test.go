package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

// captured 为测试服务器收到的一次请求。
type captured struct {
	Method string
	Path   string
	Query  url.Values
	Header http.Header
	Host   string
	Body   []byte
}

// Form 解析表单请求体。
func (c captured) Form() url.Values {
	v, _ := url.ParseQuery(string(c.Body))
	return v
}

type fakeAPI struct {
	*httptest.Server
	mu   sync.Mutex
	reqs []captured
}

func (f *fakeAPI) requests() []captured {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]captured(nil), f.reqs...)
}

// newFakeAPI 启动测试服务器，handler 返回状态码与响应体。
func newFakeAPI(t *testing.T, handler func(c captured) (int, string)) *fakeAPI {
	t.Helper()
	f := &fakeAPI{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		c := captured{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Header: r.Header.Clone(),
			Host: r.Host, Body: body}
		f.mu.Lock()
		f.reqs = append(f.reqs, c)
		f.mu.Unlock()
		status, resp := handler(c)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, resp)
	}))
	t.Cleanup(f.Close)
	return f
}

// fixClock 固定当前时间与随机串。
func fixClock(t *testing.T, ts int64) {
	t.Helper()
	oldNow, oldNonce := now, nonce
	now = func() time.Time { return time.Unix(ts, 0) }
	nonce = func() string { return "fixed-nonce" }
	t.Cleanup(func() { now, nonce = oldNow, oldNonce })
}

// setEndpoint 临时替换包级 endpoint 变量。
func setEndpoint(t *testing.T, p *string, v string) {
	t.Helper()
	old := *p
	*p = v
	t.Cleanup(func() { *p = old })
}

func mustNew(t *testing.T, typ string, cfg schema.Config) Provider {
	t.Helper()
	p, err := New(typ, cfg)
	if err != nil {
		t.Fatalf("New(%s): %v", typ, err)
	}
	return p
}

var bg = context.Background()
