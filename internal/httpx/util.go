package httpx

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
)

// SortedKeys 返回按字典序排序的键。
func SortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// QueryEscape 为 application/x-www-form-urlencoded 编码（空格为 +）。
func QueryEscape(s string) string { return url.QueryEscape(s) }

// PercentEncode 按 RFC 3986 编码（空格为 %20，保留 -_.~），用于云厂商签名。
func PercentEncode(s string) string {
	e := url.QueryEscape(s)
	e = strings.ReplaceAll(e, "+", "%20")
	e = strings.ReplaceAll(e, "*", "%2A")
	e = strings.ReplaceAll(e, "%7E", "~")
	return e
}

func SHA256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func HMACSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

func HMACSHA1(key, data []byte) []byte {
	m := hmac.New(sha1.New, key)
	m.Write(data)
	return m.Sum(nil)
}
