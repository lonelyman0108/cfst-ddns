// Package httpx 提供服务商与通知渠道共用的 HTTP 工具。
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/i18n"
)

// Client 为全局共享客户端，单次请求超时 30 秒。
var Client = &http.Client{Timeout: 30 * time.Second}

// UserAgent 在所有请求中携带。
var UserAgent = "cfst-ddns/dev"

// Request 描述一次 HTTP 调用。
type Request struct {
	Method   string
	URL      string
	Header   map[string]string
	Body     []byte // 原始请求体
	JSON     any    // 若非 nil 则序列化为 JSON 请求体，并设置 Content-Type
	Form     map[string]string
	Retries  int          // 网络错误或 5xx 时的重试次数，默认 2
	ProxyURL string       // 可选代理地址（http/https/socks5/socks5h），为空时使用全局 Client
	Client   *http.Client // 可选自定义客户端，非 nil 时优先使用
}

// ValidateProxyURL 校验通知渠道使用的代理地址；空字符串返回 nil, nil。
func ValidateProxyURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, i18n.Errorf("代理地址无效: %v", err)
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "http", "https", "socks5", "socks5h":
		u.Scheme = scheme
	default:
		return nil, i18n.Errorf("不支持的代理协议: %s", u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, i18n.New("代理地址缺少主机名")
	}
	if p := u.Port(); p != "" {
		port, err := strconv.Atoi(p)
		if err != nil || port < 1 || port > 65535 {
			return nil, i18n.New("代理端口需在 1-65535 之间")
		}
	} else if scheme == "socks5" || scheme == "socks5h" {
		return nil, i18n.New("SOCKS5 代理必须指定端口")
	}
	return u, nil
}

// Response 为已读取完整响应体的结果。
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Decode 把响应体解析为 JSON。
func (r *Response) Decode(v any) error {
	if err := json.Unmarshal(r.Body, v); err != nil {
		return fmt.Errorf("解析响应失败 (HTTP %d): %s", r.Status, Snippet(r.Body))
	}
	return nil
}

// OK 判断是否为 2xx。
func (r *Response) OK() bool { return r.Status >= 200 && r.Status < 300 }

// Do 发送请求，对网络错误和 5xx 自动重试（指数退避）。
func Do(ctx context.Context, req Request) (*Response, error) {
	body := req.Body
	header := map[string]string{}
	for k, v := range req.Header {
		header[k] = v
	}
	if req.JSON != nil {
		b, err := json.Marshal(req.JSON)
		if err != nil {
			return nil, err
		}
		body = b
		if _, ok := header["Content-Type"]; !ok {
			header["Content-Type"] = "application/json"
		}
	} else if req.Form != nil {
		body = []byte(EncodeForm(req.Form))
		if _, ok := header["Content-Type"]; !ok {
			header["Content-Type"] = "application/x-www-form-urlencoded"
		}
	}
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	retries := req.Retries
	if retries == 0 {
		retries = 2
	}

	client := req.Client
	var proxyURL *url.URL
	if client == nil {
		if strings.TrimSpace(req.ProxyURL) != "" {
			var err error
			proxyURL, err = ValidateProxyURL(req.ProxyURL)
			if err != nil {
				return nil, err
			}
			var tr *http.Transport
			if base, ok := http.DefaultTransport.(*http.Transport); ok {
				tr = base.Clone()
			} else {
				tr = &http.Transport{}
			}
			tr.Proxy = http.ProxyURL(proxyURL)
			tr.DisableKeepAlives = true
			defer tr.CloseIdleConnections()
			client = &http.Client{Transport: tr, Timeout: Client.Timeout}
		} else {
			client = Client
		}
	}

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(1<<(attempt-1)) * time.Second):
			}
		}
		var rd io.Reader
		if body != nil {
			rd = bytes.NewReader(body)
		}
		hr, err := http.NewRequestWithContext(ctx, method, req.URL, rd)
		if err != nil {
			return nil, err
		}
		hr.Header.Set("User-Agent", UserAgent)
		for k, v := range header {
			hr.Header.Set(k, v)
		}
		resp, err := client.Do(hr)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		r := &Response{Status: resp.StatusCode, Header: resp.Header, Body: data}
		if resp.StatusCode >= 500 && attempt < retries {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, Snippet(data))
			continue
		}
		return r, nil
	}
	if ue, ok := lastErr.(*url.Error); ok {
		ue.URL = redact(ue.URL)
	}
	if proxyURL != nil && proxyURL.User != nil {
		return nil, fmt.Errorf("请求 %s 失败: %s", redact(req.URL), sanitizeProxyErr(lastErr, proxyURL))
	}
	return nil, fmt.Errorf("请求 %s 失败: %w", redact(req.URL), lastErr)
}

func sanitizeProxyErr(err error, proxyURL *url.URL) string {
	s := err.Error()
	s = strings.ReplaceAll(s, proxyURL.String(), proxyURL.Redacted())
	if pass, ok := proxyURL.User.Password(); ok && pass != "" {
		s = strings.ReplaceAll(s, proxyURL.User.String()+"@", proxyURL.User.Username()+":xxxxx@")
		s = strings.ReplaceAll(s, ":"+pass+"@", ":xxxxx@")
	}
	return s
}

// EncodeForm 以稳定顺序编码表单。
func EncodeForm(m map[string]string) string {
	keys := SortedKeys(m)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(QueryEscape(k))
		sb.WriteByte('=')
		sb.WriteString(QueryEscape(m[k]))
	}
	return sb.String()
}

// Snippet 截取响应体用于错误信息。
func Snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len([]rune(s)) > 300 {
		s = string([]rune(s)[:300]) + "..."
	}
	return s
}

// redact 去掉 URL 中的查询参数，避免日志泄露令牌。
func redact(u string) string {
	if i := strings.IndexByte(u, '?'); i >= 0 {
		return u[:i]
	}
	return u
}
