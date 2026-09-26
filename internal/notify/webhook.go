package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/i18n"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func init() {
	Register(schema.TypeMeta{
		Type:        "webhook",
		Name:        "自定义 Webhook",
		Description: "向任意 HTTP 地址发送通知，支持占位符 {{title}} {{content}} {{status}} {{time}}",
		Fields: []schema.Field{
			{Key: "url", Label: "请求地址", Type: schema.Text, Required: true,
				Placeholder: "https://example.com/hook?msg={{title}}", Help: "地址中的占位符会被替换并做 URL 编码"},
			{Key: "method", Label: "请求方法", Type: schema.Select, Default: "POST", Options: []schema.Option{
				{Label: "POST", Value: "POST"}, {Label: "PUT", Value: "PUT"}, {Label: "GET", Value: "GET"}}},
			{Key: "contentType", Label: "请求体格式", Type: schema.Select, Default: "json", Options: []schema.Option{
				{Label: "JSON", Value: "json"}, {Label: "表单 x-www-form-urlencoded", Value: "form"}, {Label: "纯文本", Value: "text"}},
				Help: "GET 请求不发送请求体"},
			{Key: "headers", Label: "自定义请求头", Type: schema.Textarea, Placeholder: "Authorization: Bearer xxx",
				Help: "每行一个，格式为 Key: Value"},
			{Key: "bodyTemplate", Label: "请求体模板", Type: schema.Textarea,
				Placeholder: `{"title":"{{title}}","content":"{{content}}","status":"{{status}}","time":"{{time}}"}`,
				Help:        "留空使用默认模板；JSON 模式下占位符的值会自动转义，表单模式下会做 URL 编码。{{status}} 为 success 或 failure"},
		},
	}, func(cfg schema.Config) (Notifier, error) { return &webhook{cfg}, nil })
}

var webhookDefaultTemplates = map[string]string{
	"json": `{"title":"{{title}}","content":"{{content}}","status":"{{status}}","time":"{{time}}"}`,
	"form": "title={{title}}&content={{content}}&status={{status}}&time={{time}}",
	"text": "{{title}}\n\n{{content}}",
}

var webhookContentTypes = map[string]string{
	"json": "application/json; charset=utf-8",
	"form": "application/x-www-form-urlencoded",
	"text": "text/plain; charset=utf-8",
}

type webhook struct{ cfg schema.Config }

// webhookRender 用 escape 处理后的值一次性替换全部占位符（值中的占位符不会被二次替换）。
func webhookRender(tpl string, msg Message, escape func(string) string) string {
	status := "failure"
	if msg.Success {
		status = "success"
	}
	return strings.NewReplacer(
		"{{title}}", escape(msg.Title),
		"{{content}}", escape(msg.Content),
		"{{status}}", escape(status),
		"{{time}}", escape(now().Format("2006-01-02 15:04:05")),
	).Replace(tpl)
}

// parseHeaders 解析每行 "Key: Value" 格式的请求头。
func parseHeaders(s string) (map[string]string, error) {
	h := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, i18n.Errorf("Webhook: 请求头格式错误: %s", line)
		}
		h[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return h, nil
}

func (w *webhook) Send(ctx context.Context, msg Message) error {
	method := strings.ToUpper(w.cfg.Get("method"))
	if method == "" {
		method = "POST"
	}
	ct := w.cfg.Get("contentType")
	if _, ok := webhookContentTypes[ct]; !ok {
		ct = "json"
	}
	header, err := parseHeaders(w.cfg.Get("headers"))
	if err != nil {
		return err
	}
	req := httpx.Request{Method: method, URL: webhookRender(w.cfg.Get("url"), msg, url.QueryEscape)}

	if method != "GET" {
		tpl := w.cfg.Get("bodyTemplate")
		if tpl == "" {
			tpl = webhookDefaultTemplates[ct]
		}
		var body string
		switch ct {
		case "json":
			body = webhookRender(tpl, msg, jsonEscape)
			if !json.Valid([]byte(body)) {
				return fmt.Errorf("Webhook: 请求体模板替换后不是合法的 JSON")
			}
		case "form":
			body = webhookRender(tpl, msg, url.QueryEscape)
		default:
			body = webhookRender(tpl, msg, func(s string) string { return s })
		}
		req.Body = []byte(body)
		if !hasHeader(header, "Content-Type") {
			header["Content-Type"] = webhookContentTypes[ct]
		}
	}
	req.Header = header

	resp, err := httpx.Do(ctx, req)
	if err != nil {
		return err
	}
	if !resp.OK() {
		return httpError("Webhook", resp)
	}
	return nil
}

func hasHeader(h map[string]string, key string) bool {
	for k := range h {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}
