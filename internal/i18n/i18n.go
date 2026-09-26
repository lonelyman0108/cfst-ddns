// Package i18n 提供接口返回文案（表单元数据、错误信息）的多语言翻译。
//
// 简体中文为源语言：代码中直接书写中文原文，原文本身即翻译目录的键，
// 其他语言在 en.go、zh_tw.go、ja.go 中按原文查表，缺失时回退为原文。
// 带参数的文案以格式串为键（如 "文件为 %s/%s，本机为 %s/%s"），先翻译格式串再填参数。
//
// 运行日志、通知正文、服务端日志与服务商返回的原始错误不做翻译。
package i18n

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Lang 为支持的界面语言。
type Lang string

const (
	ZhCN Lang = "zh-CN" // 源语言
	ZhTW Lang = "zh-TW"
	En   Lang = "en"
	Ja   Lang = "ja"
)

// Default 为未指定或无法识别时使用的语言。
const Default = ZhCN

// Langs 为全部支持的语言。
var Langs = []Lang{ZhCN, ZhTW, En, Ja}

// catalogs 为各语言的翻译目录，键为简体中文原文。
var catalogs = map[Lang]map[string]string{
	ZhTW: zhTW,
	En:   en,
	Ja:   ja,
}

// Parse 解析 Accept-Language（或单个语言标签），返回最匹配的支持语言。
// zh-TW / zh-HK / zh-MO / zh-Hant 归为繁体中文，其余 zh 归为简体中文。
func Parse(header string) Lang {
	type cand struct {
		lang Lang
		q    float64
		i    int
	}
	var list []cand
	for i, part := range strings.Split(header, ",") {
		tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		q := 1.0
		for _, p := range strings.Split(params, ";") {
			if k, v, ok := strings.Cut(strings.TrimSpace(p), "="); ok && strings.TrimSpace(k) == "q" {
				if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
					q = f
				}
			}
		}
		if l, ok := match(tag); ok && q > 0 {
			list = append(list, cand{l, q, i})
		}
	}
	if len(list) == 0 {
		return Default
	}
	sort.SliceStable(list, func(a, b int) bool { return list[a].q > list[b].q })
	return list[0].lang
}

func match(tag string) (Lang, bool) {
	t := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tag), "_", "-"))
	primary, rest, _ := strings.Cut(t, "-")
	switch primary {
	case "zh":
		for _, sub := range strings.Split(rest, "-") {
			switch sub {
			case "tw", "hk", "mo", "hant":
				return ZhTW, true
			case "cn", "sg", "hans":
				return ZhCN, true
			}
		}
		return ZhCN, true
	case "en":
		return En, true
	case "ja":
		return Ja, true
	}
	return "", false
}

// T 翻译一条原文，目录中没有时返回原文。
func T(l Lang, src string) string {
	if s, ok := catalogs[l][src]; ok && s != "" {
		return s
	}
	return src
}

// Has 判断指定语言的目录中是否有该原文的翻译。
func Has(l Lang, src string) bool {
	_, ok := catalogs[l][src]
	return ok
}

// Sprintf 翻译格式串后填入参数；参数中的 Text 与 Localizer 同样按语言翻译。
func Sprintf(l Lang, format string, args ...any) string {
	return sprintf(T(l, format), localizeArgs(l, args))
}

// Text 标记需要翻译的字符串参数（如字段名），在 Sprintf / Msg 中按语言翻译。
type Text string

// Localizer 是能按语言输出自身文案的值，通常是错误。
type Localizer interface {
	Localize(l Lang) string
}

// Msg 是可翻译的文案，同时实现 error：Error() 返回简体中文，
// 因此在日志、运行记录中表现与普通错误一致，接口层再按请求语言调用 Localize。
// 格式串中可以用 %w 包装错误，errors.Is / errors.As 可穿透。
type Msg struct {
	Format string
	Args   []any
}

// M 创建一条可翻译文案。
func M(format string, args ...any) *Msg { return &Msg{Format: format, Args: args} }

// New 创建一个可翻译的错误（可用作哨兵错误）。
func New(text string) error { return &Msg{Format: text} }

// Errorf 类似 fmt.Errorf，返回可翻译的错误。
func Errorf(format string, args ...any) error { return &Msg{Format: format, Args: args} }

func (m *Msg) Error() string { return m.Localize(ZhCN) }

// Localize 按语言输出文案。
func (m *Msg) Localize(l Lang) string {
	return sprintf(T(l, m.Format), localizeArgs(l, m.Args))
}

// Unwrap 返回参数中的错误，便于 errors.Is / errors.As。
func (m *Msg) Unwrap() []error {
	var out []error
	for _, a := range m.Args {
		if e, ok := a.(error); ok {
			out = append(out, e)
		}
	}
	return out
}

// Localize 输出错误在指定语言下的文案；不可翻译的错误（如服务商原始错误）原样返回。
func Localize(l Lang, err error) string {
	if err == nil {
		return ""
	}
	if lz, ok := err.(Localizer); ok {
		return lz.Localize(l)
	}
	return err.Error()
}

func localizeArgs(l Lang, args []any) []any {
	if len(args) == 0 {
		return args
	}
	out := make([]any, len(args))
	for i, a := range args {
		switch v := a.(type) {
		case Text:
			out[i] = T(l, string(v))
		case Localizer:
			out[i] = v.Localize(l)
		case error:
			out[i] = v.Error()
		default:
			out[i] = a
		}
	}
	return out
}

// sprintf 支持 %w（按 %v 输出）。
func sprintf(format string, args []any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(strings.ReplaceAll(format, "%w", "%v"), args...)
}
