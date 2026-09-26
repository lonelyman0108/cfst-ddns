package i18n

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"testing"
)

func TestParse(t *testing.T) {
	cases := map[string]Lang{
		"":                                    ZhCN,
		"zh-CN":                               ZhCN,
		"zh":                                  ZhCN,
		"zh-Hans-CN":                          ZhCN,
		"zh-SG":                               ZhCN,
		"zh-TW":                               ZhTW,
		"zh-tw":                               ZhTW,
		"zh_TW":                               ZhTW,
		"zh-HK":                               ZhTW,
		"zh-MO":                               ZhTW,
		"zh-Hant":                             ZhTW,
		"zh-Hant-HK":                          ZhTW,
		"en":                                  En,
		"en-US":                               En,
		"EN-gb":                               En,
		"ja":                                  Ja,
		"ja-JP":                               Ja,
		"fr-FR":                               ZhCN,
		"*":                                   ZhCN,
		"fr-FR, en;q=0.8, ja;q=0.9":           Ja,
		"de, en-US;q=0.7, zh-TW;q=0.7":        En,
		"en;q=0, ja":                          Ja,
		"ko-KR,ko;q=0.9,zh-HK;q=0.8,en;q=0.5": ZhTW,
	}
	for in, want := range cases {
		if got := Parse(in); got != want {
			t.Errorf("Parse(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMsg(t *testing.T) {
	err := Errorf("文件为 %s/%s，本机为 %s/%s", "linux", "amd64", "darwin", "arm64")
	if err.Error() != "文件为 linux/amd64，本机为 darwin/arm64" {
		t.Fatalf("Error() = %q", err.Error())
	}
	want := map[Lang]string{
		ZhCN: "文件为 linux/amd64，本机为 darwin/arm64",
		ZhTW: "檔案為 linux/amd64，本機為 darwin/arm64",
		En:   "File is built for linux/amd64, but this host is darwin/arm64",
		Ja:   "ファイルは linux/amd64 用ですが、このホストは darwin/arm64 です",
	}
	for l, w := range want {
		if got := Localize(l, err); got != w {
			t.Errorf("%s: %q, want %q", l, got, w)
		}
	}

	// 参数中的 Text 与嵌套的 Msg 同样翻译，%w 包装的错误可被 errors.Is 找到
	inner := Errorf("%s 不能为空", Text("端口"))
	outer := Errorf("导入失败，数据未改动: %w", inner)
	if !errors.Is(outer, inner) {
		t.Fatal("errors.Is 应穿透 Msg")
	}
	if got := Localize(En, outer); got != "Import failed; no data was changed: Port is required" {
		t.Errorf("en: %q", got)
	}
	if got := outer.Error(); got != "导入失败，数据未改动: 端口 不能为空" {
		t.Errorf("zh-CN: %q", got)
	}

	// 普通错误与缺失的翻译原样输出
	if got := Localize(Ja, errors.New("upstream: boom")); got != "upstream: boom" {
		t.Errorf("plain: %q", got)
	}
	if got := Localize(En, New("没有翻译的原文")); got != "没有翻译的原文" {
		t.Errorf("fallback: %q", got)
	}
	// 译文可用 %[n] 调整参数顺序
	if got := Sprintf(En, "账号「%s」下找不到 %s 所属的主域名，未加入任务", "CF", "a.example.com"); got != `No domain for a.example.com found under account "CF"; not added to the task` {
		t.Errorf("indexed: %q", got)
	}
}

// verbRe 匹配格式化动词，可带 %[n] 参数序号。
var verbRe = regexp.MustCompile(`%(?:\[(\d+)\])?[-+# 0]*\d*(?:\.\d+)?([a-zA-Z%])`)

// verbs 返回 参数序号 → 动词，%w 视同 %v。
func verbs(s string) map[int]byte {
	out := map[int]byte{}
	n := 0
	for _, m := range verbRe.FindAllStringSubmatch(s, -1) {
		v := m[2][0]
		if v == '%' {
			continue
		}
		if v == 'w' {
			v = 'v'
		}
		if m[1] != "" {
			n, _ = strconv.Atoi(m[1])
		} else {
			n++
		}
		out[n] = v
	}
	return out
}

// TestCatalogs 检查各语言目录的键一致，且译文与原文的格式化参数一致。
func TestCatalogs(t *testing.T) {
	for _, l := range []Lang{ZhTW, Ja} {
		for k := range en {
			if _, ok := catalogs[l][k]; !ok {
				t.Errorf("%s 缺少 %q（en 中有）", l, k)
			}
		}
		for k := range catalogs[l] {
			if _, ok := en[k]; !ok {
				t.Errorf("en 缺少 %q（%s 中有）", k, l)
			}
		}
	}
	for l, cat := range catalogs {
		for k, v := range cat {
			if v == "" {
				t.Errorf("%s: %q 的译文为空", l, k)
			}
			if a, b := fmt.Sprint(verbs(k)), fmt.Sprint(verbs(v)); a != b {
				t.Errorf("%s: 格式化参数不一致\n  %q %s\n  %q %s", l, k, a, v, b)
			}
		}
	}
}
