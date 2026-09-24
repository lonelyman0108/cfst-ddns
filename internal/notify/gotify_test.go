package notify

import (
	"strings"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func TestGotify(t *testing.T) {
	srv, c := newServer(t, 200, `{"id":1}`)
	if err := send(mustNew(t, "gotify", schema.Config{"server": srv.URL + "/", "appToken": "AppTok", "priority": "8"})); err != nil {
		t.Fatal(err)
	}
	m := c.JSON(t)
	if c.Method != "POST" || c.Path != "/message" || c.Header.Get("X-Gotify-Key") != "AppTok" {
		t.Fatalf("请求不符: %s %s %v", c.Method, c.Path, c.Header)
	}
	if m["title"] != testMsg.Title || m["message"] != testMsg.Content || m["priority"] != float64(8) {
		t.Errorf("body = %s", c.Body)
	}

	srv2, _ := newServer(t, 401, `{"error":"Unauthorized","errorCode":401,"errorDescription":"you need to provide a valid access token"}`)
	err := send(mustNew(t, "gotify", schema.Config{"server": srv2.URL, "appToken": "bad"}))
	if err == nil || !strings.Contains(err.Error(), "valid access token") {
		t.Errorf("err = %v", err)
	}
}
