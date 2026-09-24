package notify

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func init() {
	Register(schema.TypeMeta{
		Type:        "telegram",
		Name:        "Telegram",
		Description: "通过 @BotFather 创建机器人获取 Token，向机器人发消息后访问 getUpdates 获取 Chat ID",
		DocsURL:     "https://core.telegram.org/bots#how-do-i-create-a-bot",
		Fields: []schema.Field{
			{Key: "botToken", Label: "Bot Token", Type: schema.Password, Required: true, Secret: true},
			{Key: "chatId", Label: "Chat ID", Type: schema.Text, Required: true},
			{Key: "apiBase", Label: "API 地址", Type: schema.Text, Default: "https://api.telegram.org",
				Help: "国内网络可填写反代地址"},
			{Key: "threadId", Label: "话题 ID（可选）", Type: schema.Text},
			{Key: "silent", Label: "静默发送", Type: schema.Switch, Default: "false"},
		},
	}, func(cfg schema.Config) (Notifier, error) { return &telegram{cfg}, nil })
}

type telegram struct{ cfg schema.Config }

func (t *telegram) Send(ctx context.Context, msg Message) error {
	text := "<b>" + html.EscapeString(msg.Title) + "</b>\n\n" + html.EscapeString(msg.Content)
	body := map[string]any{
		"chat_id":              t.cfg.Get("chatId"),
		"text":                 text,
		"parse_mode":           "HTML",
		"disable_notification": t.cfg.Bool("silent"),
	}
	if id := t.cfg.Int("threadId", 0); id > 0 {
		body["message_thread_id"] = id
	}
	url := strings.TrimRight(t.cfg.Get("apiBase"), "/") + "/bot" + t.cfg.Get("botToken") + "/sendMessage"
	resp, err := httpx.Do(ctx, httpx.Request{Method: "POST", URL: url, JSON: body})
	if err != nil {
		return err
	}
	var r struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := resp.Decode(&r); err != nil {
		return err
	}
	if !r.OK {
		return fmt.Errorf("Telegram: %s", r.Description)
	}
	return nil
}
