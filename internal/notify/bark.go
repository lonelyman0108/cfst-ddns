package notify

import (
	"context"
	"fmt"
	"strings"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func init() {
	Register(schema.TypeMeta{
		Type:        "bark",
		Name:        "Bark",
		Description: "iOS 推送，App Store 搜索 Bark 获取设备密钥",
		DocsURL:     "https://bark.day.app/",
		Fields: []schema.Field{
			{Key: "server", Label: "服务器地址", Type: schema.Text, Required: true, Default: "https://api.day.app"},
			{Key: "deviceKey", Label: "设备密钥", Type: schema.Password, Required: true, Secret: true},
			{Key: "group", Label: "分组", Type: schema.Text, Default: "cfst-ddns"},
			{Key: "sound", Label: "铃声", Type: schema.Text, Placeholder: "如 minuet，留空为默认"},
			{Key: "level", Label: "中断级别", Type: schema.Select, Default: "active", Options: []schema.Option{
				{Label: "默认 active", Value: "active"}, {Label: "时效性 timeSensitive", Value: "timeSensitive"},
				{Label: "静默 passive", Value: "passive"}}},
			{Key: "icon", Label: "图标 URL", Type: schema.Text},
		},
	}, func(cfg schema.Config) (Notifier, error) { return &bark{cfg}, nil })
}

type bark struct{ cfg schema.Config }

func (b *bark) Send(ctx context.Context, msg Message) error {
	body := map[string]any{
		"device_key": b.cfg.Get("deviceKey"),
		"title":      msg.Title,
		"body":       msg.Content,
		"group":      b.cfg.Get("group"),
		"level":      b.cfg.Get("level"),
	}
	if s := b.cfg.Get("sound"); s != "" {
		body["sound"] = s
	}
	if s := b.cfg.Get("icon"); s != "" {
		body["icon"] = s
	}
	resp, err := httpx.Do(ctx, httpx.Request{Method: "POST",
		URL: strings.TrimRight(b.cfg.Get("server"), "/") + "/push", JSON: body})
	if err != nil {
		return err
	}
	var r struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := resp.Decode(&r); err != nil {
		return err
	}
	if r.Code != 200 {
		return fmt.Errorf("Bark: %s", r.Message)
	}
	return nil
}
