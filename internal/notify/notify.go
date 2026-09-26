// Package notify 定义通知渠道抽象与注册表。
//
// 新增渠道：在本目录新建一个文件，实现 Notifier 接口，
// 并在 init() 中调用 Register 注册元数据与构造函数。
package notify

import (
	"context"
	"sort"
	"sync"

	"github.com/lonelyman0108/cfst-ddns/internal/i18n"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

// Message 是一条通知。Content 为纯文本，多行用 "\n" 分隔。
// Markdown 为同一内容的 Markdown 版本，支持 Markdown 的渠道优先使用。
type Message struct {
	Title    string
	Content  string
	Markdown string
	Success  bool
}

// Notifier 发送通知。
type Notifier interface {
	Send(ctx context.Context, msg Message) error
}

type Factory func(cfg schema.Config) (Notifier, error)

type entry struct {
	meta    schema.TypeMeta
	factory Factory
}

var (
	mu       sync.RWMutex
	registry = map[string]entry{}
	order    []string
)

// Register 注册一个通知渠道，通常在 init() 中调用。
func Register(meta schema.TypeMeta, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[meta.Type]; ok {
		panic("notify: duplicate type " + meta.Type)
	}
	registry[meta.Type] = entry{meta: meta, factory: f}
	order = append(order, meta.Type)
}

var preferred = []string{"bark", "telegram", "wecom", "dingtalk", "feishu", "serverchan", "pushplus", "gotify", "ntfy", "smtp", "webhook"}

func rank(t string) int {
	for i, p := range preferred {
		if p == t {
			return i
		}
	}
	return len(preferred)
}

// Metas 返回全部渠道元数据。
func Metas() []schema.TypeMeta {
	mu.RLock()
	defer mu.RUnlock()
	types := append([]string(nil), order...)
	sort.SliceStable(types, func(i, j int) bool { return rank(types[i]) < rank(types[j]) })
	out := make([]schema.TypeMeta, 0, len(types))
	for _, t := range types {
		out = append(out, registry[t].meta)
	}
	return out
}

// Meta 返回指定类型的元数据。
func Meta(t string) (schema.TypeMeta, bool) {
	mu.RLock()
	defer mu.RUnlock()
	e, ok := registry[t]
	return e.meta, ok
}

// New 校验配置并构造 Notifier。
func New(t string, cfg schema.Config) (Notifier, error) {
	mu.RLock()
	e, ok := registry[t]
	mu.RUnlock()
	if !ok {
		return nil, i18n.Errorf("不支持的通知渠道: %s", t)
	}
	cfg = e.meta.ApplyDefaults(cfg)
	if err := e.meta.Validate(cfg); err != nil {
		return nil, err
	}
	return e.factory(cfg)
}
