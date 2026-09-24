package notify

import (
	"net/url"
	"strings"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func TestServerchanURL(t *testing.T) {
	cases := map[string]string{
		"SCT123abc":    "https://sctapi.ftqq.com/SCT123abc.send",
		"sctp1234tXYZ": "https://1234.push.ft07.com/send/sctp1234tXYZ.send",
	}
	for key, want := range cases {
		if got := serverchanURL(key); got != want {
			t.Errorf("serverchanURL(%s) = %s, 期望 %s", key, got, want)
		}
	}
}

func TestServerchanSend(t *testing.T) {
	srv, c := newServer(t, 200, `{"code":0,"message":"","data":{"pushid":"1"}}`)
	old := serverchanURL
	serverchanURL = func(key string) string { return srv.URL + "/" + key + ".send" }
	t.Cleanup(func() { serverchanURL = old })

	msg := testMsg
	msg.Title = strings.Repeat("长", 40)
	if err := mustNew(t, "serverchan", schema.Config{"sendKey": "SCTkey"}).Send(t.Context(), msg); err != nil {
		t.Fatal(err)
	}
	if c.Method != "POST" || c.Path != "/SCTkey.send" {
		t.Fatalf("请求不符: %s %s", c.Method, c.Path)
	}
	form, _ := url.ParseQuery(string(c.Body))
	if len([]rune(form.Get("title"))) != 32 || form.Get("desp") != testMsg.Markdown {
		t.Errorf("表单不符: %v", form)
	}

	srv2, _ := newServer(t, 200, `{"code":40001,"message":"bad pushkey"}`)
	serverchanURL = func(string) string { return srv2.URL }
	if err := send(mustNew(t, "serverchan", schema.Config{"sendKey": "x"})); err == nil || !strings.Contains(err.Error(), "bad pushkey") {
		t.Errorf("err = %v", err)
	}
}
