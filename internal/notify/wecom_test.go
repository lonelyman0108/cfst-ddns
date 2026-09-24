package notify

import (
	"strings"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func TestWecomMarkdownWithKey(t *testing.T) {
	srv, c := newServer(t, 200, `{"errcode":0,"errmsg":"ok"}`)
	setVar(t, &wecomAPI, srv.URL+"/cgi-bin/webhook/send")
	if err := send(mustNew(t, "wecom", schema.Config{"webhook": "abc-123"})); err != nil {
		t.Fatal(err)
	}
	if c.Method != "POST" || c.Path != "/cgi-bin/webhook/send" || c.Query["key"][0] != "abc-123" {
		t.Fatalf("请求不符: %s %s %v", c.Method, c.Path, c.Query)
	}
	m := c.JSON(t)
	if m["msgtype"] != "markdown" {
		t.Errorf("msgtype = %v", m["msgtype"])
	}
	content, _ := dig(m, "markdown", "content").(string)
	if !strings.HasPrefix(content, "## CFST 更新成功") || !strings.Contains(content, "**www.example.com**") {
		t.Errorf("content = %q", content)
	}
}

func TestWecomTextFullURL(t *testing.T) {
	srv, c := newServer(t, 200, `{"errcode":0,"errmsg":"ok"}`)
	n := mustNew(t, "wecom", schema.Config{"webhook": srv.URL + "/hook?key=xyz", "msgtype": "text"})
	if err := send(n); err != nil {
		t.Fatal(err)
	}
	if c.Path != "/hook" || c.Query["key"][0] != "xyz" {
		t.Fatalf("请求不符: %s %v", c.Path, c.Query)
	}
	if got := dig(c.JSON(t), "text", "content"); got != testMsg.Title+"\n\n"+testMsg.Content {
		t.Errorf("content = %v", got)
	}
}

func TestWecomErrcode(t *testing.T) {
	srv, _ := newServer(t, 200, `{"errcode":93000,"errmsg":"invalid webhook url"}`)
	err := send(mustNew(t, "wecom", schema.Config{"webhook": srv.URL}))
	if err == nil || !strings.Contains(err.Error(), "93000") {
		t.Fatalf("err = %v", err)
	}
}
