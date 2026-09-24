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

func TestFeishuSignVector(t *testing.T) {
	sts, sign := feishuSign("SECtest", 1700000000)
	if sts != "1700000000\nSECtest" {
		t.Errorf("stringToSign = %q", sts)
	}
	// 向量由 openssl 独立计算（以待签名串为密钥，对空串签名）：
	// printf '' | openssl dgst -sha256 -hmac "$(printf '1700000000\nSECtest')" -binary | base64
	if sign != "G7XpBpG8NgG02fJOAhX6FRAObIljmFoxVReo8I62pEk=" {
		t.Errorf("sign = %s", sign)
	}
	m := hmac.New(sha256.New, []byte(sts))
	if want := base64.StdEncoding.EncodeToString(m.Sum(nil)); sign != want {
		t.Errorf("sign = %s, 期望 %s", sign, want)
	}
}

func TestFeishuPostSigned(t *testing.T) {
	fixNow(t, time.Unix(1700000000, 0))
	srv, c := newServer(t, 200, `{"code":0,"data":{},"msg":"success"}`)
	setVar(t, &feishuAPI, srv.URL+"/open-apis/bot/v2/hook/")
	if err := send(mustNew(t, "feishu", schema.Config{"webhook": "hook-id", "secret": "SECtest"})); err != nil {
		t.Fatal(err)
	}
	if c.Method != "POST" || c.Path != "/open-apis/bot/v2/hook/hook-id" {
		t.Fatalf("请求不符: %s %s", c.Method, c.Path)
	}
	m := c.JSON(t)
	if m["timestamp"] != "1700000000" || m["sign"] != "G7XpBpG8NgG02fJOAhX6FRAObIljmFoxVReo8I62pEk=" {
		t.Errorf("签名字段不符: %v %v", m["timestamp"], m["sign"])
	}
	if m["msg_type"] != "post" || dig(m, "content", "post", "zh_cn", "title") != testMsg.Title {
		t.Errorf("body = %s", c.Body)
	}
	lines, _ := dig(m, "content", "post", "zh_cn", "content").([]any)
	if len(lines) != 2 {
		t.Errorf("段落数 = %d", len(lines))
	}
}

func TestFeishuTextAndErrors(t *testing.T) {
	srv, c := newServer(t, 200, `{"StatusCode":0,"StatusMessage":"success"}`)
	if err := send(mustNew(t, "feishu", schema.Config{"webhook": srv.URL + "/x", "msgtype": "text"})); err != nil {
		t.Fatal(err)
	}
	m := c.JSON(t)
	if m["msg_type"] != "text" || m["sign"] != nil {
		t.Errorf("body = %s", c.Body)
	}
	if text, _ := dig(m, "content", "text").(string); !strings.HasPrefix(text, testMsg.Title) {
		t.Errorf("text = %q", text)
	}

	for _, body := range []string{`{"code":19021,"msg":"sign match fail or timestamp is not within one hour from current time"}`,
		`{"StatusCode":9499,"StatusMessage":"Bad Request"}`} {
		srv, _ := newServer(t, 200, body)
		if err := send(mustNew(t, "feishu", schema.Config{"webhook": srv.URL})); err == nil {
			t.Errorf("响应 %s 应返回错误", body)
		}
	}
}
