package notify

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func TestNtfyJSONPublish(t *testing.T) {
	srv, c := newServer(t, 200, `{"id":"x"}`)
	cfg := schema.Config{"server": srv.URL, "topic": "cfst", "token": "tk_abc", "priority": "4", "tags": "a, b"}
	if err := send(mustNew(t, "ntfy", cfg)); err != nil {
		t.Fatal(err)
	}
	if c.Method != "POST" || c.Path != "/" || c.Header.Get("Authorization") != "Bearer tk_abc" {
		t.Fatalf("请求不符: %s %s %v", c.Method, c.Path, c.Header)
	}
	m := c.JSON(t)
	tags, _ := m["tags"].([]any)
	if m["topic"] != "cfst" || m["title"] != testMsg.Title || m["message"] != testMsg.Content ||
		m["priority"] != float64(4) || len(tags) != 2 || tags[1] != "b" {
		t.Errorf("body = %s", c.Body)
	}
	if c.Header.Get("Title") != "" {
		t.Error("不应使用 Title 请求头")
	}
}

func TestNtfyBasicAuthAndError(t *testing.T) {
	srv, c := newServer(t, 403, `{"code":40301,"http":403,"error":"forbidden"}`)
	msg := testMsg
	msg.Success = false
	err := mustNew(t, "ntfy", schema.Config{"server": srv.URL, "topic": "t", "username": "u", "password": "p"}).Send(t.Context(), msg)
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("err = %v", err)
	}
	if c.Header.Get("Authorization") != "Basic "+base64.StdEncoding.EncodeToString([]byte("u:p")) {
		t.Errorf("Authorization = %s", c.Header.Get("Authorization"))
	}
	if tags, _ := c.JSON(t)["tags"].([]any); len(tags) != 1 || tags[0] != "warning" {
		t.Errorf("失败时默认标签不符: %v", tags)
	}
}
