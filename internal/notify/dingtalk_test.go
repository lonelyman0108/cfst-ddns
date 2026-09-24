package notify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func TestDingtalkSignVector(t *testing.T) {
	sts, sign := dingtalkSign("SECtest", 1700000000000)
	if sts != "1700000000000\nSECtest" {
		t.Errorf("stringToSign = %q", sts)
	}
	// 向量由 openssl 独立计算：
	// printf '1700000000000\nSECtest' | openssl dgst -sha256 -hmac SECtest -binary | base64
	if sign != "aZLLrriXgn05YbwaGR7knYsLeJADjr9NwLaNNKpxh4g=" {
		t.Errorf("sign = %s", sign)
	}
	m := hmac.New(sha256.New, []byte("SECtest"))
	m.Write([]byte(sts))
	if want := base64.StdEncoding.EncodeToString(m.Sum(nil)); sign != want {
		t.Errorf("sign = %s, 期望 %s", sign, want)
	}
}

func TestDingtalkSend(t *testing.T) {
	fixNow(t, time.UnixMilli(1700000000000))
	srv, c := newServer(t, 200, `{"errcode":0,"errmsg":"ok"}`)
	setVar(t, &dingtalkAPI, srv.URL+"/robot/send")
	if err := send(mustNew(t, "dingtalk", schema.Config{"accessToken": "tok", "secret": "SECtest"})); err != nil {
		t.Fatal(err)
	}
	if c.Method != "POST" || c.Path != "/robot/send" {
		t.Fatalf("请求不符: %s %s", c.Method, c.Path)
	}
	q := c.Query
	if q["access_token"][0] != "tok" || q["timestamp"][0] != "1700000000000" ||
		q["sign"][0] != "aZLLrriXgn05YbwaGR7knYsLeJADjr9NwLaNNKpxh4g=" {
		t.Errorf("查询参数不符: %v", q)
	}
	m := c.JSON(t)
	if m["msgtype"] != "markdown" || dig(m, "markdown", "title") != testMsg.Title {
		t.Errorf("body = %s", c.Body)
	}
	if text, _ := dig(m, "markdown", "text").(string); !strings.Contains(text, "**www.example.com**") {
		t.Errorf("text = %q", text)
	}
}

func TestDingtalkFullURLNoSecret(t *testing.T) {
	srv, c := newServer(t, 200, `{"errcode":310000,"errmsg":"keywords not in content"}`)
	err := send(mustNew(t, "dingtalk", schema.Config{"accessToken": srv.URL + "/robot/send?access_token=abc"}))
	if err == nil || !strings.Contains(err.Error(), "310000") {
		t.Fatalf("err = %v", err)
	}
	if c.Query["access_token"][0] != "abc" || c.Query["sign"] != nil {
		t.Errorf("查询参数不符: %v", c.Query)
	}
}
