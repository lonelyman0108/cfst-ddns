package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

type captured struct {
	Method string
	Path   string
	Query  map[string][]string
	Header http.Header
	Body   []byte
}

// JSON 把请求体解析为 map。
func (c *captured) JSON(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(c.Body, &m); err != nil {
		t.Fatalf("请求体不是 JSON: %v\n%s", err, c.Body)
	}
	return m
}

// newServer 启动记录请求的测试服务器，固定返回 status 与 body。
func newServer(t *testing.T, status int, body string) (*httptest.Server, *captured) {
	t.Helper()
	c := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Method = r.Method
		c.Path = r.URL.Path
		c.Query = r.URL.Query()
		c.Header = r.Header.Clone()
		c.Body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, c
}

func fixNow(t *testing.T, tm time.Time) {
	t.Helper()
	old := now
	now = func() time.Time { return tm }
	t.Cleanup(func() { now = old })
}

func setVar(t *testing.T, p *string, v string) {
	t.Helper()
	old := *p
	*p = v
	t.Cleanup(func() { *p = old })
}

func mustNew(t *testing.T, typ string, cfg schema.Config) Notifier {
	t.Helper()
	n, err := New(typ, cfg)
	if err != nil {
		t.Fatalf("New(%s): %v", typ, err)
	}
	return n
}

var testMsg = Message{
	Title:    "CFST 更新成功",
	Content:  "www.example.com → 1.1.1.1\nlatency 50ms",
	Markdown: "**www.example.com** → `1.1.1.1`",
	Success:  true,
}

func send(n Notifier) error { return n.Send(context.Background(), testMsg) }

// dig 按路径取嵌套 map 的值。
func dig(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}
