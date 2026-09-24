package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func init() {
	Register(schema.TypeMeta{
		Type:        "smtp",
		Name:        "邮件 SMTP",
		Description: "通过 SMTP 发送邮件，QQ/163 等邮箱需在设置中开启 SMTP 并使用授权码作为密码",
		Fields: []schema.Field{
			{Key: "host", Label: "SMTP 服务器", Type: schema.Text, Required: true, Placeholder: "smtp.qq.com"},
			{Key: "port", Label: "端口", Type: schema.Number, Required: true, Default: "465",
				Help: "SSL 通常为 465，STARTTLS 通常为 587，不加密通常为 25"},
			{Key: "security", Label: "加密方式", Type: schema.Select, Default: "ssl", Options: []schema.Option{
				{Label: "SSL/TLS", Value: "ssl"}, {Label: "STARTTLS", Value: "starttls"}, {Label: "不加密", Value: "none"}}},
			{Key: "username", Label: "用户名", Type: schema.Text, Placeholder: "通常为完整邮箱地址",
				Help: "留空则不进行登录认证"},
			{Key: "password", Label: "密码 / 授权码", Type: schema.Password, Secret: true},
			{Key: "from", Label: "发件人", Type: schema.Text, Placeholder: "cfst-ddns <me@example.com>",
				Help: "留空则使用用户名"},
			{Key: "to", Label: "收件人", Type: schema.Text, Required: true, Help: "多个地址用逗号分隔"},
		},
	}, func(cfg schema.Config) (Notifier, error) { return &smtpNotifier{cfg}, nil })
}

type smtpNotifier struct{ cfg schema.Config }

// smtpDial 建立 TCP 连接，测试中可替换。
var smtpDial = func(ctx context.Context, addr string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "tcp", addr)
}

func (s *smtpNotifier) addresses() (*mail.Address, []*mail.Address, error) {
	fromStr := s.cfg.Get("from")
	if fromStr == "" {
		fromStr = s.cfg.Get("username")
	}
	if fromStr == "" {
		return nil, nil, errors.New("邮件: 发件人与用户名不能同时为空")
	}
	from, err := mail.ParseAddress(fromStr)
	if err != nil {
		return nil, nil, fmt.Errorf("邮件: 发件人地址无效: %s", fromStr)
	}
	var to []*mail.Address
	for _, p := range strings.FieldsFunc(s.cfg.Get("to"), func(r rune) bool { return r == ',' || r == ';' || r == '，' }) {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		a, err := mail.ParseAddress(p)
		if err != nil {
			return nil, nil, fmt.Errorf("邮件: 收件人地址无效: %s", p)
		}
		to = append(to, a)
	}
	if len(to) == 0 {
		return nil, nil, errors.New("邮件: 收件人不能为空")
	}
	return from, to, nil
}

// buildMail 构造 UTF-8 纯文本邮件，主题按 RFC 2047 编码，正文使用 base64 传输编码。
func buildMail(from *mail.Address, to []*mail.Address, msg Message, host string) []byte {
	var b bytes.Buffer
	tos := make([]string, len(to))
	for i, a := range to {
		tos[i] = a.String()
	}
	t := now()
	b.WriteString("From: " + from.String() + "\r\n")
	b.WriteString("To: " + strings.Join(tos, ", ") + "\r\n")
	b.WriteString("Subject: " + mime.BEncoding.Encode("UTF-8", msg.Title) + "\r\n")
	b.WriteString("Date: " + t.Format(time.RFC1123Z) + "\r\n")
	b.WriteString("Message-ID: <" + strconv.FormatInt(t.UnixNano(), 36) + ".cfst-ddns@" + host + ">\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	enc := base64.StdEncoding.EncodeToString([]byte(msg.Content))
	for len(enc) > 76 {
		b.WriteString(enc[:76] + "\r\n")
		enc = enc[76:]
	}
	b.WriteString(enc + "\r\n")
	return b.Bytes()
}

func (s *smtpNotifier) Send(ctx context.Context, msg Message) error {
	from, to, err := s.addresses()
	if err != nil {
		return err
	}
	host := s.cfg.Get("host")
	security := s.cfg.Get("security")
	addr := net.JoinHostPort(host, strconv.Itoa(s.cfg.Int("port", 465)))

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	conn, err := smtpDial(ctx, addr)
	if err != nil {
		return fmt.Errorf("邮件: 连接 %s 失败: %w", addr, err)
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if dl, ok := ctx.Deadline(); ok {
		conn.SetDeadline(dl)
	}
	tlsCfg := &tls.Config{ServerName: host}
	if security == "ssl" {
		conn = tls.Client(conn, tlsCfg)
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("邮件: 连接服务器失败: %w", err)
	}
	defer c.Close()

	if security == "starttls" {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("邮件: 服务器不支持 STARTTLS")
		}
		if err := c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("邮件: STARTTLS 失败: %w", err)
		}
	}
	if user := s.cfg.Get("username"); user != "" {
		ok, mechs := c.Extension("AUTH")
		if !ok {
			return errors.New("邮件: 服务器不支持登录认证")
		}
		if err := c.Auth(pickAuth(mechs, user, s.cfg.Get("password"))); err != nil {
			return fmt.Errorf("邮件: 登录失败: %w", err)
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return fmt.Errorf("邮件: 发件人被拒绝: %w", err)
	}
	for _, a := range to {
		if err := c.Rcpt(a.Address); err != nil {
			return fmt.Errorf("邮件: 收件人 %s 被拒绝: %w", a.Address, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("邮件: 发送失败: %w", err)
	}
	if _, err := w.Write(buildMail(from, to, msg, host)); err != nil {
		return fmt.Errorf("邮件: 发送失败: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("邮件: 发送失败: %w", err)
	}
	_ = c.Quit()
	return nil
}

// pickAuth 按服务器声明的机制选择 PLAIN 或 LOGIN。
// 未使用 smtp.PlainAuth：它在非 TLS 连接上会拒绝认证，而「不加密」是用户的显式选择。
func pickAuth(mechs, user, pass string) smtp.Auth {
	list := strings.Fields(strings.ToUpper(mechs))
	has := func(m string) bool {
		for _, x := range list {
			if x == m {
				return true
			}
		}
		return false
	}
	if !has("PLAIN") && has("LOGIN") {
		return loginAuth{user, pass}
	}
	return plainAuth{user, pass}
}

type plainAuth struct{ user, pass string }

func (a plainAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "PLAIN", []byte("\x00" + a.user + "\x00" + a.pass), nil
}

func (a plainAuth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, errors.New("PLAIN 认证收到意外的质询")
	}
	return nil, nil
}

type loginAuth struct{ user, pass string }

func (a loginAuth) Start(*smtp.ServerInfo) (string, []byte, error) { return "LOGIN", nil, nil }

func (a loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	p := strings.ToLower(string(fromServer))
	switch {
	case strings.Contains(p, "user"):
		return []byte(a.user), nil
	case strings.Contains(p, "pass"):
		return []byte(a.pass), nil
	}
	return nil, fmt.Errorf("LOGIN 认证收到未知质询: %s", fromServer)
}
