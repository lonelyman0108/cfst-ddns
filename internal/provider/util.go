package provider

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// now 返回当前时间，测试中可替换以得到确定性签名。
var now = time.Now

// nonce 生成随机串，测试中可替换。
var nonce = func() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// clampTTL 在 ttl<=0 时返回 def，小于 min 时返回 min。
func clampTTL(ttl, min, def int) int {
	if ttl <= 0 {
		return def
	}
	if ttl < min {
		return min
	}
	return ttl
}

// rrOf 规范化主机记录，空串视为 "@"。
func rrOf(rr string) string {
	rr = strings.TrimSpace(rr)
	if rr == "" {
		return "@"
	}
	return rr
}

// flexString 兼容 JSON 中以字符串或数字表示的字段。
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	if string(b) == "null" {
		*f = ""
		return nil
	}
	*f = flexString(b)
	return nil
}

func (f flexString) Int() int {
	v, _ := strconv.Atoi(string(f))
	return v
}
