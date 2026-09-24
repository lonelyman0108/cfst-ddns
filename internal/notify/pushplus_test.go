package notify

import (
	"strings"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func TestPushplus(t *testing.T) {
	srv, c := newServer(t, 200, `{"code":200,"msg":"请求成功","data":"abc"}`)
	setVar(t, &pushplusAPI, srv.URL+"/send")

	if err := send(mustNew(t, "pushplus", schema.Config{"token": "tk", "topic": "grp"})); err != nil {
		t.Fatal(err)
	}
	m := c.JSON(t)
	if c.Method != "POST" || c.Path != "/send" || m["token"] != "tk" || m["topic"] != "grp" ||
		m["template"] != "markdown" || m["content"] != testMsg.Markdown || m["title"] != testMsg.Title {
		t.Fatalf("请求不符: %s %s %s", c.Method, c.Path, c.Body)
	}

	if err := send(mustNew(t, "pushplus", schema.Config{"token": "tk", "template": "html"})); err != nil {
		t.Fatal(err)
	}
	m = c.JSON(t)
	if m["template"] != "html" || !strings.Contains(m["content"].(string), "<br>") || m["topic"] != nil {
		t.Errorf("html 模板不符: %s", c.Body)
	}

	srv2, _ := newServer(t, 200, `{"code":903,"msg":"无效的用户token"}`)
	setVar(t, &pushplusAPI, srv2.URL)
	if err := send(mustNew(t, "pushplus", schema.Config{"token": "bad"})); err == nil || !strings.Contains(err.Error(), "903") {
		t.Errorf("err = %v", err)
	}
}
