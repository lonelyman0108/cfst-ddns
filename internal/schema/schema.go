// Package schema 描述由后端下发、前端动态渲染的配置表单。
package schema

import (
	"fmt"
	"strconv"
	"strings"
)

// Config 为服务商/通知渠道配置，所有值统一以字符串保存。
type Config map[string]string

// Get 返回去除首尾空白后的值。
func (c Config) Get(key string) string { return strings.TrimSpace(c[key]) }

// Bool 解析布尔值。
func (c Config) Bool(key string) bool {
	v, _ := strconv.ParseBool(c.Get(key))
	return v
}

// Int 解析整数，失败返回 def。
func (c Config) Int(key string, def int) int {
	v, err := strconv.Atoi(c.Get(key))
	if err != nil {
		return def
	}
	return v
}

type FieldType string

const (
	Text     FieldType = "text"
	Password FieldType = "password"
	Number   FieldType = "number"
	Switch   FieldType = "switch"
	Select   FieldType = "select"
	Textarea FieldType = "textarea"
)

type Option struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type Condition struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Field struct {
	Key         string     `json:"key"`
	Label       string     `json:"label"`
	Type        FieldType  `json:"type"`
	Required    bool       `json:"required,omitempty"`
	Secret      bool       `json:"secret,omitempty"`
	Placeholder string     `json:"placeholder,omitempty"`
	Help        string     `json:"help,omitempty"`
	Default     string     `json:"default,omitempty"`
	Options     []Option   `json:"options,omitempty"`
	ShowIf      *Condition `json:"showIf,omitempty"`
}

type TypeMeta struct {
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	DocsURL     string  `json:"docsUrl,omitempty"`
	Fields      []Field `json:"fields"`
}

// Mask 是密钥字段对外展示的占位值。
const Mask = "******"

// visible 判断字段在当前配置下是否生效。
func (f Field) visible(cfg Config) bool {
	return f.ShowIf == nil || cfg.Get(f.ShowIf.Key) == f.ShowIf.Value
}

// ApplyDefaults 为空字段填充默认值，返回新 map。
func (m TypeMeta) ApplyDefaults(cfg Config) Config {
	out := Config{}
	for k, v := range cfg {
		out[k] = v
	}
	for _, f := range m.Fields {
		if out.Get(f.Key) == "" && f.Default != "" {
			out[f.Key] = f.Default
		}
	}
	return out
}

// Validate 校验必填项与数字格式。
func (m TypeMeta) Validate(cfg Config) error {
	for _, f := range m.Fields {
		if !f.visible(cfg) {
			continue
		}
		v := cfg.Get(f.Key)
		if f.Required && v == "" {
			return fmt.Errorf("%s 不能为空", f.Label)
		}
		if f.Type == Number && v != "" {
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				return fmt.Errorf("%s 必须是数字", f.Label)
			}
		}
		if f.Type == Select && v != "" && len(f.Options) > 0 {
			ok := false
			for _, o := range f.Options {
				if o.Value == v {
					ok = true
					break
				}
			}
			if !ok {
				return fmt.Errorf("%s 取值无效: %s", f.Label, v)
			}
		}
	}
	return nil
}

// Clean 只保留 Schema 中声明的字段。
func (m TypeMeta) Clean(cfg Config) Config {
	out := Config{}
	for _, f := range m.Fields {
		if v, ok := cfg[f.Key]; ok {
			out[f.Key] = strings.TrimSpace(v)
		}
	}
	return out
}

// Masked 返回把密钥字段替换为占位符的副本。
func (m TypeMeta) Masked(cfg Config) Config {
	out := Config{}
	for k, v := range cfg {
		out[k] = v
	}
	for _, f := range m.Fields {
		if f.Secret && out[f.Key] != "" {
			out[f.Key] = Mask
		}
	}
	return out
}

// Merge 用旧配置补齐新配置中未修改（占位符或缺失）的密钥字段。
func (m TypeMeta) Merge(old, incoming Config) Config {
	out := m.Clean(incoming)
	for _, f := range m.Fields {
		if !f.Secret {
			continue
		}
		v, ok := incoming[f.Key]
		if !ok || v == Mask {
			out[f.Key] = old[f.Key]
		}
	}
	return out
}
