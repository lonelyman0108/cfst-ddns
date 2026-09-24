package notify

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func init() {
	Register(schema.TypeMeta{
		Type:        "feishu",
		Name:        "飞书 / Lark 群机器人",
		Description: "群设置 → 群机器人 → 添加自定义机器人，复制 Webhook 地址；Lark 国际版请填写完整地址",
		DocsURL:     "https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot",
		Fields: []schema.Field{
			{Key: "webhook", Label: "Webhook 地址", Type: schema.Password, Required: true, Secret: true,
				Placeholder: "https://open.feishu.cn/open-apis/bot/v2/hook/...",
				Help:        "可填写完整 Webhook 地址，或只填写 hook/ 后面的部分（默认飞书国内版）"},
			{Key: "secret", Label: "签名密钥（可选）", Type: schema.Password, Secret: true,
				Help: "安全设置中启用「签名校验」时填写"},
			{Key: "msgtype", Label: "消息类型", Type: schema.Select, Default: "post", Options: []schema.Option{
				{Label: "富文本 post", Value: "post"}, {Label: "纯文本 text", Value: "text"}}},
		},
	}, func(cfg schema.Config) (Notifier, error) { return &feishu{cfg}, nil })
}

var feishuAPI = "https://open.feishu.cn/open-apis/bot/v2/hook/"

type feishu struct{ cfg schema.Config }

// feishuSign 计算签名：以 timestamp+"\n"+secret 为密钥对空串做 HmacSHA256 再 base64，timestamp 为秒。
func feishuSign(secret string, ts int64) (stringToSign, sign string) {
	stringToSign = strconv.FormatInt(ts, 10) + "\n" + secret
	sign = base64.StdEncoding.EncodeToString(httpx.HMACSHA256([]byte(stringToSign), nil))
	return
}

func (f *feishu) url() string {
	h := f.cfg.Get("webhook")
	if strings.HasPrefix(h, "http://") || strings.HasPrefix(h, "https://") {
		return h
	}
	return feishuAPI + h
}

func (f *feishu) Send(ctx context.Context, msg Message) error {
	var body map[string]any
	if f.cfg.Get("msgtype") == "text" {
		body = map[string]any{"msg_type": "text", "content": map[string]any{"text": msg.Title + "\n\n" + msg.Content}}
	} else {
		var lines [][]map[string]any
		for _, l := range strings.Split(msg.Content, "\n") {
			lines = append(lines, []map[string]any{{"tag": "text", "text": l}})
		}
		body = map[string]any{"msg_type": "post", "content": map[string]any{
			"post": map[string]any{"zh_cn": map[string]any{"title": msg.Title, "content": lines}},
		}}
	}
	if secret := f.cfg.Get("secret"); secret != "" {
		ts := now().Unix()
		_, sign := feishuSign(secret, ts)
		body["timestamp"] = strconv.FormatInt(ts, 10)
		body["sign"] = sign
	}
	resp, err := httpx.Do(ctx, httpx.Request{Method: "POST", URL: f.url(), JSON: body})
	if err != nil {
		return err
	}
	// 新版返回 {"code":0,"msg":"success"}，旧版返回 {"StatusCode":0,"StatusMessage":"success"}
	var r struct {
		Code          *int   `json:"code"`
		Msg           string `json:"msg"`
		StatusCode    *int   `json:"StatusCode"`
		StatusMessage string `json:"StatusMessage"`
	}
	if err := resp.Decode(&r); err != nil {
		return fmt.Errorf("飞书: %w", err)
	}
	switch {
	case r.Code != nil && *r.Code != 0:
		return fmt.Errorf("飞书: [%d] %s", *r.Code, r.Msg)
	case r.StatusCode != nil && *r.StatusCode != 0:
		return fmt.Errorf("飞书: [%d] %s", *r.StatusCode, r.StatusMessage)
	case r.Code == nil && r.StatusCode == nil && !resp.OK():
		return httpError("飞书", resp)
	}
	return nil
}
