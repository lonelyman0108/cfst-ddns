package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func init() {
	Register(schema.TypeMeta{
		Type:        "gotify",
		Name:        "Gotify",
		Description: "自建推送服务，在 Gotify 后台 Apps 中创建应用获取 Token",
		DocsURL:     "https://gotify.net/docs/pushmsg",
		Fields: []schema.Field{
			{Key: "server", Label: "服务器地址", Type: schema.Text, Required: true, Placeholder: "https://gotify.example.com"},
			{Key: "appToken", Label: "应用 Token", Type: schema.Password, Required: true, Secret: true},
			{Key: "priority", Label: "优先级", Type: schema.Number, Default: "5", Help: "0–10，数值越大越重要"},
		},
	}, func(cfg schema.Config) (Notifier, error) { return &gotify{cfg}, nil })
}

type gotify struct{ cfg schema.Config }

func (g *gotify) Send(ctx context.Context, msg Message) error {
	body := map[string]any{
		"title":    msg.Title,
		"message":  msg.Content,
		"priority": g.cfg.Int("priority", 5),
	}
	resp, err := httpx.Do(ctx, httpx.Request{Method: "POST",
		URL:    strings.TrimRight(g.cfg.Get("server"), "/") + "/message",
		Header: map[string]string{"X-Gotify-Key": g.cfg.Get("appToken")},
		JSON:   body})
	if err != nil {
		return err
	}
	if !resp.OK() {
		var r struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"errorDescription"`
		}
		if json.Unmarshal(resp.Body, &r) == nil && r.Error != "" {
			return fmt.Errorf("Gotify: HTTP %d %s: %s", resp.Status, r.Error, r.ErrorDescription)
		}
		return httpError("Gotify", resp)
	}
	return nil
}
