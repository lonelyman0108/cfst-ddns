package notify

import (
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func TestRegistry(t *testing.T) {
	want := []string{"bark", "telegram", "wecom", "dingtalk", "feishu", "serverchan", "pushplus", "gotify", "ntfy", "smtp", "webhook"}
	metas := Metas()
	if len(metas) != len(want) {
		t.Fatalf("注册渠道数 = %d, 期望 %d", len(metas), len(want))
	}
	for i, m := range metas {
		if m.Type != want[i] {
			t.Errorf("第 %d 个渠道 = %s, 期望 %s", i, m.Type, want[i])
		}
		for _, f := range m.Fields {
			if f.Type == schema.Select && f.Default != "" {
				ok := false
				for _, o := range f.Options {
					ok = ok || o.Value == f.Default
				}
				if !ok {
					t.Errorf("%s.%s 默认值 %q 不在选项中", m.Type, f.Key, f.Default)
				}
			}
			if f.Type == schema.Password && !f.Secret {
				t.Errorf("%s.%s 为密码字段但未标记 Secret", m.Type, f.Key)
			}
		}
	}

	minimal := map[string]schema.Config{
		"bark":       {"deviceKey": "k"},
		"telegram":   {"botToken": "t", "chatId": "1"},
		"wecom":      {"webhook": "k"},
		"dingtalk":   {"accessToken": "t"},
		"feishu":     {"webhook": "h"},
		"serverchan": {"sendKey": "SCT1"},
		"pushplus":   {"token": "t"},
		"gotify":     {"server": "https://g", "appToken": "t"},
		"ntfy":       {"topic": "x"},
		"smtp":       {"host": "smtp.example.com", "to": "a@example.com"},
		"webhook":    {"url": "https://example.com"},
	}
	for typ, cfg := range minimal {
		if _, err := New(typ, cfg); err != nil {
			t.Errorf("New(%s) 最小配置失败: %v", typ, err)
		}
	}
	if _, err := New("wecom", schema.Config{}); err == nil {
		t.Error("缺少必填项时应返回错误")
	}
}

func TestTruncateBytes(t *testing.T) {
	if got := truncateBytes("中文abc", 4); got != "中" {
		t.Errorf("truncateBytes = %q", got)
	}
	if got := truncateBytes("abc", 10); got != "abc" {
		t.Errorf("truncateBytes = %q", got)
	}
	if got := jsonEscape("a\"b\n<c>"); got != `a\"b\n<c>` {
		t.Errorf("jsonEscape = %q", got)
	}
}
