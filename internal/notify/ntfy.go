package notify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func init() {
	Register(schema.TypeMeta{
		Type:        "ntfy",
		Name:        "ntfy",
		Description: "开源推送服务，可使用公共服务器 ntfy.sh 或自建",
		DocsURL:     "https://docs.ntfy.sh/publish/",
		Fields: []schema.Field{
			{Key: "server", Label: "服务器地址", Type: schema.Text, Required: true, Default: "https://ntfy.sh"},
			{Key: "topic", Label: "主题 Topic", Type: schema.Text, Required: true,
				Help: "使用公共服务器时任何人知道主题名即可订阅，请使用不易猜测的名称"},
			{Key: "token", Label: "Access Token（可选）", Type: schema.Password, Secret: true, Placeholder: "tk_..."},
			{Key: "username", Label: "用户名（可选）", Type: schema.Text, Help: "未填写 Access Token 时使用用户名密码认证"},
			{Key: "password", Label: "密码（可选）", Type: schema.Password, Secret: true},
			{Key: "priority", Label: "优先级", Type: schema.Select, Default: "3", Options: []schema.Option{
				{Label: "1 最低", Value: "1"}, {Label: "2 低", Value: "2"}, {Label: "3 默认", Value: "3"},
				{Label: "4 高", Value: "4"}, {Label: "5 紧急", Value: "5"}}},
			{Key: "tags", Label: "标签（可选）", Type: schema.Text, Placeholder: "如 globe_with_meridians,cfst",
				Help: "逗号分隔，可使用 emoji 短代码；留空时按结果自动使用 ✅ 或 ⚠️"},
		},
	}, func(cfg schema.Config) (Notifier, error) { return &ntfy{cfg}, nil })
}

type ntfy struct{ cfg schema.Config }

func (n *ntfy) Send(ctx context.Context, msg Message) error {
	var tags []string
	for _, t := range strings.Split(n.cfg.Get("tags"), ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	if len(tags) == 0 {
		if msg.Success {
			tags = []string{"white_check_mark"}
		} else {
			tags = []string{"warning"}
		}
	}
	// 使用 JSON 发布接口（POST 到服务器根路径），避免非 ASCII 标题放在 Header 中的编码问题
	body := map[string]any{
		"topic":    n.cfg.Get("topic"),
		"title":    msg.Title,
		"message":  msg.Content,
		"priority": n.cfg.Int("priority", 3),
		"tags":     tags,
	}
	h := map[string]string{}
	if t := n.cfg.Get("token"); t != "" {
		h["Authorization"] = "Bearer " + t
	} else if u := n.cfg.Get("username"); u != "" {
		h["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(u+":"+n.cfg.Get("password")))
	}
	resp, err := httpx.Do(ctx, httpx.Request{Method: "POST",
		URL: strings.TrimRight(n.cfg.Get("server"), "/") + "/", Header: h, JSON: body})
	if err != nil {
		return err
	}
	if !resp.OK() {
		var r struct {
			Code  int    `json:"code"`
			Error string `json:"error"`
		}
		if json.Unmarshal(resp.Body, &r) == nil && r.Error != "" {
			return fmt.Errorf("ntfy: [%d] %s", r.Code, r.Error)
		}
		return httpError("ntfy", resp)
	}
	return nil
}
