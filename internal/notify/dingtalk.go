package notify

import (
	"context"
	"encoding/base64"
	"net/url"
	"strconv"
	"strings"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func init() {
	Register(schema.TypeMeta{
		Type:        "dingtalk",
		Name:        "钉钉群机器人",
		Description: "群设置 → 机器人 → 添加自定义机器人；安全设置推荐选择「加签」，若使用「自定义关键词」请确保通知标题包含该关键词",
		DocsURL:     "https://open.dingtalk.com/document/robots/custom-robot-access",
		Fields: []schema.Field{
			{Key: "accessToken", Label: "Access Token 或 Webhook 地址", Type: schema.Password, Required: true, Secret: true,
				Placeholder: "https://oapi.dingtalk.com/robot/send?access_token=...",
				Help:        "可填写完整 Webhook 地址，或只填写 access_token= 后面的部分"},
			{Key: "secret", Label: "加签密钥（可选）", Type: schema.Password, Secret: true,
				Placeholder: "SEC 开头", Help: "安全设置选择「加签」时填写"},
		},
	}, func(cfg schema.Config) (Notifier, error) { return &dingtalk{cfg}, nil })
}

var dingtalkAPI = "https://oapi.dingtalk.com/robot/send"

type dingtalk struct{ cfg schema.Config }

// dingtalkSign 计算加签：base64(HmacSHA256(secret, timestamp+"\n"+secret))，timestamp 为毫秒。
// 返回待签名串与未经 URL 编码的签名。
func dingtalkSign(secret string, ts int64) (stringToSign, sign string) {
	stringToSign = strconv.FormatInt(ts, 10) + "\n" + secret
	sign = base64.StdEncoding.EncodeToString(httpx.HMACSHA256([]byte(secret), []byte(stringToSign)))
	return
}

func (d *dingtalk) url() string {
	t := d.cfg.Get("accessToken")
	u := t
	if !strings.HasPrefix(t, "http://") && !strings.HasPrefix(t, "https://") {
		u = dingtalkAPI + "?access_token=" + url.QueryEscape(t)
	}
	if secret := d.cfg.Get("secret"); secret != "" {
		ts := now().UnixMilli()
		_, sign := dingtalkSign(secret, ts)
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u += sep + "timestamp=" + strconv.FormatInt(ts, 10) + "&sign=" + url.QueryEscape(sign)
	}
	return u
}

func (d *dingtalk) Send(ctx context.Context, msg Message) error {
	body := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]any{
			"title": msg.Title,
			"text":  "## " + msg.Title + "\n\n" + markdownOf(msg),
		},
	}
	resp, err := httpx.Do(ctx, httpx.Request{Method: "POST", URL: d.url(), JSON: body})
	if err != nil {
		return err
	}
	return checkErrcode("钉钉", resp)
}
