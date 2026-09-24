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
		Type:        "pushplus",
		Name:        "PushPlus 推送加",
		Description: "微信公众号推送，在官网登录后获取 Token",
		DocsURL:     "https://www.pushplus.plus/doc/",
		Fields: []schema.Field{
			{Key: "token", Label: "Token", Type: schema.Password, Required: true, Secret: true},
			{Key: "topic", Label: "群组编码（可选）", Type: schema.Text, Help: "填写后推送给该群组的所有订阅者"},
			{Key: "template", Label: "消息模板", Type: schema.Select, Default: "markdown", Options: []schema.Option{
				{Label: "Markdown", Value: "markdown"}, {Label: "纯文本 txt", Value: "txt"}, {Label: "HTML", Value: "html"}}},
		},
	}, func(cfg schema.Config) (Notifier, error) { return &pushplus{cfg}, nil })
}

var pushplusAPI = "https://www.pushplus.plus/send"

type pushplus struct{ cfg schema.Config }

func (p *pushplus) Send(ctx context.Context, msg Message) error {
	tpl := p.cfg.Get("template")
	var content string
	switch tpl {
	case "txt":
		content = msg.Content
	case "html":
		content = strings.ReplaceAll(html.EscapeString(msg.Content), "\n", "<br>")
	default:
		tpl = "markdown"
		content = markdownOf(msg)
	}
	body := map[string]any{
		"token":    p.cfg.Get("token"),
		"title":    msg.Title,
		"content":  content,
		"template": tpl,
	}
	if t := p.cfg.Get("topic"); t != "" {
		body["topic"] = t
	}
	resp, err := httpx.Do(ctx, httpx.Request{Method: "POST", URL: pushplusAPI, JSON: body})
	if err != nil {
		return err
	}
	var r struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := resp.Decode(&r); err != nil {
		return fmt.Errorf("PushPlus: %w", err)
	}
	if r.Code != 200 {
		return fmt.Errorf("PushPlus: [%d] %s", r.Code, r.Msg)
	}
	return nil
}
