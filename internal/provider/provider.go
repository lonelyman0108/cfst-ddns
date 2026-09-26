// Package provider 定义 DNS 服务商抽象与注册表。
//
// 新增服务商：在本目录新建一个文件，实现 Provider 接口，
// 并在 init() 中调用 Register 注册元数据与构造函数。
package provider

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/lonelyman0108/cfst-ddns/internal/i18n"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

// Record 是一条 DNS 记录。Name 为主机记录（rr），如 "www"、"@"。
type Record struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	FQDN    string `json:"fqdn"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied,omitempty"`
	Line    string `json:"line,omitempty"`
}

// Provider 是 DNS 服务商需要实现的最小能力集合。
// domain 均为主域名（如 example.com），rr 为主机记录（"@" 表示根域名）。
type Provider interface {
	// Test 校验凭据是否可用。
	Test(ctx context.Context) error
	// ListDomains 列出账号下可管理的主域名。
	ListDomains(ctx context.Context) ([]string, error)
	// ListRecords 列出记录；rr、recordType 为空表示不过滤。
	ListRecords(ctx context.Context, domain, rr, recordType string) ([]Record, error)
	// CreateRecord 新建记录，使用 r 的 Name/Type/Value/TTL/Proxied/Line。
	CreateRecord(ctx context.Context, domain string, r Record) error
	// UpdateRecord 按 r.ID 更新记录。
	UpdateRecord(ctx context.Context, domain string, r Record) error
	// DeleteRecord 按 r.ID 删除记录。
	DeleteRecord(ctx context.Context, domain string, r Record) error
}

// Factory 根据配置构造 Provider。
type Factory func(cfg schema.Config) (Provider, error)

type entry struct {
	meta    schema.TypeMeta
	factory Factory
}

var (
	mu       sync.RWMutex
	registry = map[string]entry{}
	order    []string
)

// Register 注册一个服务商，通常在 init() 中调用。
func Register(meta schema.TypeMeta, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[meta.Type]; ok {
		panic("provider: duplicate type " + meta.Type)
	}
	registry[meta.Type] = entry{meta: meta, factory: f}
	order = append(order, meta.Type)
}

// Metas 返回全部服务商元数据（按注册顺序，Cloudflare 优先）。
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

// preferred 控制界面中的展示顺序，未列出的排在后面。
var preferred = []string{"cloudflare", "dnspod", "tencentcloud", "alidns", "huaweicloud", "godaddy"}

func rank(t string) int {
	for i, p := range preferred {
		if p == t {
			return i
		}
	}
	return len(preferred)
}

// Meta 返回指定类型的元数据。
func Meta(t string) (schema.TypeMeta, bool) {
	mu.RLock()
	defer mu.RUnlock()
	e, ok := registry[t]
	return e.meta, ok
}

// New 校验配置并构造 Provider。
func New(t string, cfg schema.Config) (Provider, error) {
	mu.RLock()
	e, ok := registry[t]
	mu.RUnlock()
	if !ok {
		return nil, i18n.Errorf("不支持的 DNS 服务商: %s", t)
	}
	cfg = e.meta.ApplyDefaults(cfg)
	if err := e.meta.Validate(cfg); err != nil {
		return nil, err
	}
	return e.factory(cfg)
}

// ErrNotFound 表示资源不存在。
var ErrNotFound = errors.New("not found")

// FQDN 由主机记录与主域名拼出完整域名。
func FQDN(rr, domain string) string {
	rr = strings.TrimSpace(rr)
	if rr == "" || rr == "@" {
		return domain
	}
	return rr + "." + domain
}

// RR 由完整域名与主域名反推主机记录。
func RR(fqdn, domain string) string {
	fqdn = strings.TrimSuffix(strings.ToLower(fqdn), ".")
	domain = strings.ToLower(domain)
	if fqdn == domain {
		return "@"
	}
	return strings.TrimSuffix(fqdn, "."+domain)
}
