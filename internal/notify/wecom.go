package notify

import (
	"context"
	"net/url"
	"strings"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func init() {
	Register(schema.TypeMeta{
		Type:        "wecom",
		Name:        "企业微信群机器人",
		Description: "在企业微信群聊中添加群机器人，复制其 Webhook 地址",
		DocsURL:     "https://developer.work.weixin.qq.com/document/path/91770",
		Fields: []schema.Field{
			{Key: "webhook", Label: "Webhook 地址或 Key", Type: schema.Password, Required: true, Secret: true,
				Placeholder: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=...",
				Help:        "可填写完整 Webhook 地址，或只填写 key= 后面的部分"},
			{Key: "msgtype", Label: "消息类型", Type: schema.Select, Default: "markdown", Options: []schema.Option{
				{Label: "Markdown", Value: "markdown"}, {Label: "纯文本", Value: "text"}},
				Help: "微信插件端不支持 Markdown 消息时可改为纯文本"},
		},
	}, func(cfg schema.Config) (Notifier, error) { return &wecom{cfg}, nil })
}

var wecomAPI = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send"

type wecom struct{ cfg schema.Config }

func (w *wecom) url() string {
	h := w.cfg.Get("webhook")
	if strings.HasPrefix(h, "http://") || strings.HasPrefix(h, "https://") {
		return h
	}
	return wecomAPI + "?key=" + url.QueryEscape(h)
}

func (w *wecom) Send(ctx context.Context, msg Message) error {
	var body map[string]any
	if w.cfg.Get("msgtype") == "text" {
		// 文本内容最长 2048 字节
		text := truncateBytes(msg.Title+"\n\n"+msg.Content, 2048)
		body = map[string]any{"msgtype": "text", "text": map[string]any{"content": text}}
	} else {
		// Markdown 内容最长 4096 字节
		text := truncateBytes("## "+msg.Title+"\n\n"+markdownOf(msg), 4096)
		body = map[string]any{"msgtype": "markdown", "markdown": map[string]any{"content": text}}
	}
	resp, err := httpx.Do(ctx, httpx.Request{Method: "POST", URL: w.url(), JSON: body})
	if err != nil {
		return err
	}
	return checkErrcode("企业微信", resp)
}
