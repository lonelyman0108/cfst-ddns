package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func init() {
	Register(schema.TypeMeta{
		Type:        "godaddy",
		Name:        "GoDaddy",
		Description: "GoDaddy 生产环境 API Key。注意：GoDaddy 仅对拥有 10 个以上域名或特定套餐的账号开放 DNS API，否则返回 403",
		DocsURL:     "https://developer.godaddy.com/keys",
		Fields: []schema.Field{
			{Key: "apiKey", Label: "API Key", Type: schema.Text, Required: true,
				Help: "创建时环境选择 Production（生产环境），OTE 测试环境的 Key 无法管理真实域名"},
			{Key: "apiSecret", Label: "API Secret", Type: schema.Password, Required: true, Secret: true},
		},
	}, newGoDaddy)
}

// godaddyAPI 为 API 地址，测试中可替换。
var godaddyAPI = "https://api.godaddy.com"

const (
	gdMinTTL   = 600
	gdPageSize = 500
	gdMaxPages = 100
)

type goDaddy struct {
	cfg      schema.Config
	endpoint string
}

func newGoDaddy(cfg schema.Config) (Provider, error) {
	return &goDaddy{cfg: cfg, endpoint: godaddyAPI}, nil
}

func (g *goDaddy) call(ctx context.Context, method, path string, body any, out any) (int, error) {
	req := httpx.Request{Method: method, URL: strings.TrimRight(g.endpoint, "/") + path, Header: map[string]string{
		"Authorization": "sso-key " + g.cfg.Get("apiKey") + ":" + g.cfg.Get("apiSecret"),
		"Accept":        "application/json",
	}}
	if body != nil {
		req.JSON = body
	}
	resp, err := httpx.Do(ctx, req)
	if err != nil {
		return 0, err
	}
	if !resp.OK() {
		var e struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(resp.Body, &e) != nil || (e.Code == "" && e.Message == "") {
			return resp.Status, fmt.Errorf("GoDaddy: HTTP %d: %s", resp.Status, httpx.Snippet(resp.Body))
		}
		return resp.Status, fmt.Errorf("GoDaddy: %s [%s]", e.Message, e.Code)
	}
	if out != nil && len(resp.Body) > 0 {
		return resp.Status, resp.Decode(out)
	}
	return resp.Status, nil
}

func (g *goDaddy) Test(ctx context.Context) error {
	_, err := g.call(ctx, "GET", "/v1/domains?limit=1", nil, nil)
	return err
}

func (g *goDaddy) ListDomains(ctx context.Context) ([]string, error) {
	const limit = 1000
	var names []string
	marker := ""
	for {
		q := url.Values{"statuses": {"ACTIVE"}, "limit": {strconv.Itoa(limit)}}
		if marker != "" {
			q.Set("marker", marker)
		}
		var ds []struct {
			Domain string `json:"domain"`
		}
		if _, err := g.call(ctx, "GET", "/v1/domains?"+q.Encode(), nil, &ds); err != nil {
			return nil, err
		}
		for _, d := range ds {
			names = append(names, d.Domain)
		}
		if len(ds) < limit {
			break
		}
		marker = ds[len(ds)-1].Domain
	}
	return names, nil
}

type gdRecord struct {
	Data string `json:"data"`
	Name string `json:"name,omitempty"`
	TTL  int    `json:"ttl,omitempty"`
	Type string `json:"type,omitempty"`
}

func gdPath(domain, recordType, name string) string {
	p := "/v1/domains/" + url.PathEscape(domain) + "/records"
	if recordType != "" {
		p += "/" + url.PathEscape(strings.ToUpper(recordType))
		if name != "" {
			p += "/" + url.PathEscape(name)
		}
	}
	return p
}

func gdID(recordType, name, value string) string {
	return strings.ToUpper(recordType) + "|" + name + "|" + value
}

func gdSplitID(id string) (recordType, name, value string, err error) {
	parts := strings.SplitN(id, "|", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("GoDaddy: 无效的记录 ID %q", id)
	}
	return parts[0], parts[1], parts[2], nil
}

func (g *goDaddy) ListRecords(ctx context.Context, domain, rr, recordType string) ([]Record, error) {
	// 路径式接口要求同时指定类型才能按名称过滤，否则拉取后本地过滤
	name := ""
	if recordType != "" {
		name = rr
	}
	path := gdPath(domain, recordType, name)
	var out []Record
	var prevFirst string
	for page := 0; page < gdMaxPages; page++ {
		var recs []gdRecord
		q := url.Values{"limit": {strconv.Itoa(gdPageSize)}, "offset": {strconv.Itoa(page * gdPageSize)}}
		if _, err := g.call(ctx, "GET", path+"?"+q.Encode(), nil, &recs); err != nil {
			return nil, err
		}
		if len(recs) == 0 {
			break
		}
		// 防止服务端忽略 offset 时重复返回同一页
		first := recs[0].Type + "|" + recs[0].Name + "|" + recs[0].Data
		if page > 0 && first == prevFirst {
			break
		}
		prevFirst = first
		for _, x := range recs {
			if x.Name == "" {
				x.Name = rr
			}
			if x.Type == "" {
				x.Type = recordType
			}
			if rr != "" && !strings.EqualFold(x.Name, rr) {
				continue
			}
			if recordType != "" && !strings.EqualFold(x.Type, recordType) {
				continue
			}
			out = append(out, Record{ID: gdID(x.Type, x.Name, x.Data), Name: x.Name, FQDN: FQDN(x.Name, domain),
				Type: x.Type, Value: x.Data, TTL: x.TTL})
		}
		if len(recs) < gdPageSize {
			break
		}
	}
	return out, nil
}

// getSet 读取同名同类型的全部记录值。
func (g *goDaddy) getSet(ctx context.Context, domain, recordType, name string) ([]gdRecord, error) {
	var recs []gdRecord
	status, err := g.call(ctx, "GET", gdPath(domain, recordType, name), nil, &recs)
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return recs, nil
}

// putSet 整组替换同名同类型的记录；为空时删除。
func (g *goDaddy) putSet(ctx context.Context, domain, recordType, name string, recs []gdRecord) error {
	path := gdPath(domain, recordType, name)
	if len(recs) == 0 {
		_, err := g.call(ctx, "DELETE", path, nil, nil)
		return err
	}
	body := make([]gdRecord, 0, len(recs))
	for _, r := range recs {
		body = append(body, gdRecord{Data: r.Data, TTL: clampTTL(r.TTL, gdMinTTL, gdMinTTL)})
	}
	_, err := g.call(ctx, "PUT", path, body, nil)
	return err
}

func gdWithout(recs []gdRecord, value string) []gdRecord {
	out := make([]gdRecord, 0, len(recs))
	for _, r := range recs {
		if r.Data != value {
			out = append(out, r)
		}
	}
	return out
}

func (g *goDaddy) CreateRecord(ctx context.Context, domain string, r Record) error {
	name := rrOf(r.Name)
	recs, err := g.getSet(ctx, domain, r.Type, name)
	if err != nil {
		return err
	}
	recs = append(gdWithout(recs, r.Value), gdRecord{Data: r.Value, TTL: clampTTL(r.TTL, gdMinTTL, gdMinTTL)})
	return g.putSet(ctx, domain, r.Type, name, recs)
}

func (g *goDaddy) UpdateRecord(ctx context.Context, domain string, r Record) error {
	oldType, oldName, oldValue, err := gdSplitID(r.ID)
	if err != nil {
		return err
	}
	name := rrOf(r.Name)
	if !strings.EqualFold(oldType, r.Type) || !strings.EqualFold(oldName, name) {
		if err := g.DeleteRecord(ctx, domain, r); err != nil {
			return err
		}
		return g.CreateRecord(ctx, domain, r)
	}
	recs, err := g.getSet(ctx, domain, oldType, oldName)
	if err != nil {
		return err
	}
	out := make([]gdRecord, 0, len(recs)+1)
	for _, x := range recs {
		if x.Data == oldValue || x.Data == r.Value {
			continue
		}
		out = append(out, x)
	}
	out = append(out, gdRecord{Data: r.Value, TTL: clampTTL(r.TTL, gdMinTTL, gdMinTTL)})
	return g.putSet(ctx, domain, oldType, oldName, out)
}

func (g *goDaddy) DeleteRecord(ctx context.Context, domain string, r Record) error {
	recordType, name, value, err := gdSplitID(r.ID)
	if err != nil {
		return err
	}
	recs, err := g.getSet(ctx, domain, recordType, name)
	if err != nil {
		return err
	}
	rest := gdWithout(recs, value)
	if len(rest) == len(recs) {
		return nil // 记录已不存在
	}
	return g.putSet(ctx, domain, recordType, name, rest)
}
