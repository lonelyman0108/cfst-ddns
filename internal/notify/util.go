package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
)

// now 返回当前时间，测试中可替换为固定值。
var now = time.Now

// markdownOf 返回消息的 Markdown 版本，缺省时退回纯文本。
func markdownOf(m Message) string {
	if m.Markdown != "" {
		return m.Markdown
	}
	return m.Content
}

// truncateBytes 按 UTF-8 字节数截断，不会切断多字节字符。
func truncateBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

// truncateRunes 按字符数截断。
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// jsonEscape 返回可直接嵌入 JSON 字符串字面量的转义结果（不含两侧引号）。
func jsonEscape(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	out := strings.TrimSuffix(b.String(), "\n")
	return out[1 : len(out)-1]
}

// checkErrcode 校验 {"errcode":0,"errmsg":"ok"} 风格的响应（企业微信、钉钉）。
func checkErrcode(name string, resp *httpx.Response) error {
	var r struct {
		ErrCode *int   `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := resp.Decode(&r); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if r.ErrCode == nil {
		return fmt.Errorf("%s: 响应异常 (HTTP %d): %s", name, resp.Status, httpx.Snippet(resp.Body))
	}
	if *r.ErrCode != 0 {
		return fmt.Errorf("%s: [%d] %s", name, *r.ErrCode, r.ErrMsg)
	}
	return nil
}

// httpError 生成非 2xx 响应的错误信息。
func httpError(name string, resp *httpx.Response) error {
	return fmt.Errorf("%s: HTTP %d: %s", name, resp.Status, httpx.Snippet(resp.Body))
}
