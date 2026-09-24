package notify

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

var fixedTime = time.Date(2026, 9, 24, 8, 30, 0, 0, time.Local)

func TestWebhookJSONDefault(t *testing.T) {
	fixNow(t, fixedTime)
	srv, c := newServer(t, 200, "ok")
	msg := testMsg
	msg.Content = "含 \"引号\" 与\n换行 {{title}}"
	cfg := schema.Config{"url": srv.URL + "/hook", "headers": "Authorization: Bearer x\nX-Test:  1 "}
	if err := mustNew(t, "webhook", cfg).Send(t.Context(), msg); err != nil {
		t.Fatal(err)
	}
	if c.Method != "POST" || c.Path != "/hook" || c.Header.Get("Authorization") != "Bearer x" || c.Header.Get("X-Test") != "1" ||
		!strings.HasPrefix(c.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("请求不符: %s %s %v", c.Method, c.Path, c.Header)
	}
	m := c.JSON(t)
	if m["content"] != msg.Content || m["status"] != "success" || m["time"] != "2026-09-24 08:30:00" || m["title"] != msg.Title {
		t.Errorf("body = %s", c.Body)
	}
}

func TestWebhookForm(t *testing.T) {
	srv, c := newServer(t, 200, "ok")
	cfg := schema.Config{"url": srv.URL, "method": "PUT", "contentType": "form", "bodyTemplate": "t={{title}}&s={{status}}"}
	msg := testMsg
	msg.Title = "a&b=c 中"
	msg.Success = false
	if err := mustNew(t, "webhook", cfg).Send(t.Context(), msg); err != nil {
		t.Fatal(err)
	}
	form, _ := url.ParseQuery(string(c.Body))
	if c.Method != "PUT" || form.Get("t") != msg.Title || form.Get("s") != "failure" ||
		c.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
		t.Errorf("请求不符: %s %s %v", c.Method, c.Body, c.Header)
	}
}

func TestWebhookGET(t *testing.T) {
	srv, c := newServer(t, 200, "ok")
	cfg := schema.Config{"url": srv.URL + "/push?title={{title}}&body={{content}}", "method": "GET"}
	if err := send(mustNew(t, "webhook", cfg)); err != nil {
		t.Fatal(err)
	}
	if c.Method != "GET" || c.Query["title"][0] != testMsg.Title || c.Query["body"][0] != testMsg.Content || len(c.Body) != 0 {
		t.Errorf("请求不符: %s %v %q", c.Method, c.Query, c.Body)
	}
}

func TestWebhookTextAndErrors(t *testing.T) {
	srv, c := newServer(t, 200, "ok")
	cfg := schema.Config{"url": srv.URL, "contentType": "text", "headers": "Content-Type: text/markdown"}
	if err := send(mustNew(t, "webhook", cfg)); err != nil {
		t.Fatal(err)
	}
	if string(c.Body) != testMsg.Title+"\n\n"+testMsg.Content || c.Header.Get("Content-Type") != "text/markdown" {
		t.Errorf("请求不符: %q %v", c.Body, c.Header)
	}

	bad := schema.Config{"url": srv.URL, "bodyTemplate": `{"title":{{title}}}`}
	if err := send(mustNew(t, "webhook", bad)); err == nil || !strings.Contains(err.Error(), "JSON") {
		t.Errorf("非法 JSON 模板应报错: %v", err)
	}
	if err := send(mustNew(t, "webhook", schema.Config{"url": srv.URL, "headers": "no-colon"})); err == nil {
		t.Error("非法请求头应报错")
	}
	srv2, _ := newServer(t, 404, "not found")
	if err := send(mustNew(t, "webhook", schema.Config{"url": srv2.URL})); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("err = %v", err)
	}
}
