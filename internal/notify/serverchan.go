package notify

import (
	"context"
	"fmt"
	"regexp"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func init() {
	Register(schema.TypeMeta{
		Type:        "serverchan",
		Name:        "Server 酱",
		Description: "支持 Server 酱 Turbo（SCT 开头）与 Server 酱³（sctp 开头）的 SendKey",
		DocsURL:     "https://sct.ftqq.com/sendkey",
		Fields: []schema.Field{
			{Key: "sendKey", Label: "SendKey", Type: schema.Password, Required: true, Secret: true},
		},
	}, func(cfg schema.Config) (Notifier, error) { return &serverchan{cfg}, nil })
}

var sctpKey = regexp.MustCompile(`^sctp(\d+)t`)

// serverchanURL 根据 SendKey 返回推送地址，测试中可替换。
var serverchanURL = func(key string) string {
	if m := sctpKey.FindStringSubmatch(key); m != nil {
		return fmt.Sprintf("https://%s.push.ft07.com/send/%s.send", m[1], key)
	}
	return fmt.Sprintf("https://sctapi.ftqq.com/%s.send", key)
}

type serverchan struct{ cfg schema.Config }

func (s *serverchan) Send(ctx context.Context, msg Message) error {
	resp, err := httpx.Do(ctx, httpx.Request{Method: "POST", URL: serverchanURL(s.cfg.Get("sendKey")),
		Form: map[string]string{
			"title": truncateRunes(msg.Title, 32), // 标题最长 32 字符
			"desp":  markdownOf(msg),
		}})
	if err != nil {
		return err
	}
	var r struct {
		Code    *int   `json:"code"`
		Message string `json:"message"`
		Info    string `json:"info"`
	}
	if err := resp.Decode(&r); err != nil {
		return fmt.Errorf("Server 酱: %w", err)
	}
	if r.Code == nil {
		return httpError("Server 酱", resp)
	}
	if *r.Code != 0 {
		m := r.Message
		if m == "" {
			m = r.Info
		}
		return fmt.Errorf("Server 酱: [%d] %s", *r.Code, m)
	}
	return nil
}
