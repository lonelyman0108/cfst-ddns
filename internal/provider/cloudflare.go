package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func init() {
	Register(schema.TypeMeta{
		Type:        "cloudflare",
		Name:        "Cloudflare",
		Description: "推荐使用 API Token（权限：Zone.Zone:Read + Zone.DNS:Edit）",
		DocsURL:     "https://dash.cloudflare.com/profile/api-tokens",
		Fields: []schema.Field{
			{Key: "authType", Label: "认证方式", Type: schema.Select, Required: true, Default: "token",
				Options: []schema.Option{{Label: "API Token（推荐）", Value: "token"}, {Label: "Global API Key", Value: "key"}}},
			{Key: "apiToken", Label: "API Token", Type: schema.Password, Required: true, Secret: true,
				ShowIf: &schema.Condition{Key: "authType", Value: "token"}},
			{Key: "email", Label: "账号邮箱", Type: schema.Text, Required: true,
				ShowIf: &schema.Condition{Key: "authType", Value: "key"}},
			{Key: "apiKey", Label: "Global API Key", Type: schema.Password, Required: true, Secret: true,
				ShowIf: &schema.Condition{Key: "authType", Value: "key"}},
			{Key: "zoneId", Label: "Zone ID（可选）", Type: schema.Text,
				Help: "Token 没有 Zone:Read 权限时填写，此时账号只能管理该 Zone 对应的域名"},
		},
	}, newCloudflare)
}

const cfAPI = "https://api.cloudflare.com/client/v4"

type cloudflare struct {
	cfg    schema.Config
	zoneID string

	mu    sync.Mutex
	zones map[string]string // 域名 → zone id
}

func newCloudflare(cfg schema.Config) (Provider, error) {
	return &cloudflare{cfg: cfg, zoneID: cfg.Get("zoneId"), zones: map[string]string{}}, nil
}

type cfResp struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Result     any `json:"result"`
	ResultInfo struct {
		Page       int `json:"page"`
		TotalPages int `json:"total_pages"`
	} `json:"result_info"`
}

func (c *cloudflare) call(ctx context.Context, method, path string, body any, result any) (*cfResp, error) {
	h := map[string]string{}
	if c.cfg.Get("authType") == "key" {
		h["X-Auth-Email"] = c.cfg.Get("email")
		h["X-Auth-Key"] = c.cfg.Get("apiKey")
	} else {
		h["Authorization"] = "Bearer " + c.cfg.Get("apiToken")
	}
	resp, err := httpx.Do(ctx, httpx.Request{Method: method, URL: cfAPI + path, Header: h, JSON: body})
	if err != nil {
		return nil, err
	}
	out := &cfResp{Result: result}
	if err := resp.Decode(out); err != nil {
		return nil, err
	}
	if !out.Success {
		msgs := make([]string, 0, len(out.Errors))
		for _, e := range out.Errors {
			msgs = append(msgs, fmt.Sprintf("[%d] %s", e.Code, e.Message))
		}
		return nil, fmt.Errorf("Cloudflare: %s", strings.Join(msgs, "; "))
	}
	return out, nil
}

func (c *cloudflare) Test(ctx context.Context) error {
	if c.cfg.Get("authType") != "key" {
		if _, err := c.call(ctx, "GET", "/user/tokens/verify", nil, nil); err != nil {
			return err
		}
	}
	if c.zoneID != "" {
		_, err := c.call(ctx, "GET", "/zones/"+c.zoneID+"/dns_records?per_page=1", nil, nil)
		return err
	}
	_, err := c.ListDomains(ctx)
	return err
}

type cfZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *cloudflare) ListDomains(ctx context.Context) ([]string, error) {
	if c.zoneID != "" {
		var z cfZone
		if _, err := c.call(ctx, "GET", "/zones/"+c.zoneID, nil, &z); err != nil {
			return nil, err
		}
		c.cache(z)
		return []string{z.Name}, nil
	}
	var names []string
	for page := 1; ; page++ {
		var zones []cfZone
		r, err := c.call(ctx, "GET", fmt.Sprintf("/zones?per_page=50&page=%d", page), nil, &zones)
		if err != nil {
			return nil, err
		}
		for _, z := range zones {
			c.cache(z)
			names = append(names, z.Name)
		}
		if page >= r.ResultInfo.TotalPages {
			break
		}
	}
	return names, nil
}

func (c *cloudflare) cache(z cfZone) {
	c.mu.Lock()
	c.zones[strings.ToLower(z.Name)] = z.ID
	c.mu.Unlock()
}

func (c *cloudflare) zone(ctx context.Context, domain string) (string, error) {
	domain = strings.ToLower(domain)
	c.mu.Lock()
	id, ok := c.zones[domain]
	c.mu.Unlock()
	if ok {
		return id, nil
	}
	if c.zoneID != "" {
		return c.zoneID, nil
	}
	var zones []cfZone
	if _, err := c.call(ctx, "GET", "/zones?name="+url.QueryEscape(domain), nil, &zones); err != nil {
		return "", err
	}
	if len(zones) == 0 {
		return "", fmt.Errorf("Cloudflare: 账号下未找到域名 %s", domain)
	}
	c.cache(zones[0])
	return zones[0].ID, nil
}

type cfRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

func (c *cloudflare) ListRecords(ctx context.Context, domain, rr, recordType string) ([]Record, error) {
	zid, err := c.zone(ctx, domain)
	if err != nil {
		return nil, err
	}
	q := url.Values{"per_page": {"100"}}
	if rr != "" {
		q.Set("name", FQDN(rr, domain))
	}
	if recordType != "" {
		q.Set("type", recordType)
	}
	var out []Record
	for page := 1; ; page++ {
		q.Set("page", fmt.Sprint(page))
		var recs []cfRecord
		r, err := c.call(ctx, "GET", "/zones/"+zid+"/dns_records?"+q.Encode(), nil, &recs)
		if err != nil {
			return nil, err
		}
		for _, x := range recs {
			out = append(out, Record{ID: x.ID, Name: RR(x.Name, domain), FQDN: x.Name, Type: x.Type,
				Value: x.Content, TTL: x.TTL, Proxied: x.Proxied})
		}
		if page >= r.ResultInfo.TotalPages {
			break
		}
	}
	return out, nil
}

func (c *cloudflare) body(domain string, r Record) cfRecord {
	ttl := r.TTL
	if ttl <= 0 {
		ttl = 1 // 自动
	}
	return cfRecord{Type: r.Type, Name: FQDN(r.Name, domain), Content: r.Value, TTL: ttl, Proxied: r.Proxied}
}

func (c *cloudflare) CreateRecord(ctx context.Context, domain string, r Record) error {
	zid, err := c.zone(ctx, domain)
	if err != nil {
		return err
	}
	_, err = c.call(ctx, "POST", "/zones/"+zid+"/dns_records", c.body(domain, r), nil)
	return err
}

func (c *cloudflare) UpdateRecord(ctx context.Context, domain string, r Record) error {
	zid, err := c.zone(ctx, domain)
	if err != nil {
		return err
	}
	_, err = c.call(ctx, "PUT", "/zones/"+zid+"/dns_records/"+r.ID, c.body(domain, r), nil)
	return err
}

func (c *cloudflare) DeleteRecord(ctx context.Context, domain string, r Record) error {
	zid, err := c.zone(ctx, domain)
	if err != nil {
		return err
	}
	_, err = c.call(ctx, "DELETE", "/zones/"+zid+"/dns_records/"+r.ID, nil, nil)
	return err
}
