package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func init() {
	Register(schema.TypeMeta{
		Type:        "dnspod",
		Name:        "DNSPod",
		Description: "DNSPod 旧版 API（dnsapi.cn），使用「ID + Token」认证",
		DocsURL:     "https://console.dnspod.cn/account/token/token",
		Fields: []schema.Field{
			{Key: "tokenId", Label: "Token ID", Type: schema.Text, Required: true,
				Help: "在 DNSPod 控制台「API 密钥 → DNSPod Token」创建，ID 为数字"},
			{Key: "token", Label: "Token", Type: schema.Password, Required: true, Secret: true},
		},
	}, newDNSPod)
}

// dnspodAPI 为旧版 API 地址，测试中可替换。
var dnspodAPI = "https://dnsapi.cn"

const (
	dnspodDefaultLine = "默认"
	dnspodPageSize    = 3000
)

type dnspod struct {
	cfg      schema.Config
	endpoint string
}

func newDNSPod(cfg schema.Config) (Provider, error) {
	return &dnspod{cfg: cfg, endpoint: dnspodAPI}, nil
}

type dnspodStatus struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// call 调用 API；allowEmpty 为 true 时把 code "10"（列表为空）视为成功并返回 false。
func (d *dnspod) call(ctx context.Context, action string, params map[string]string, out any, allowEmpty bool) (bool, error) {
	form := map[string]string{
		"login_token":    d.cfg.Get("tokenId") + "," + d.cfg.Get("token"),
		"format":         "json",
		"lang":           "cn",
		"error_on_empty": "no",
	}
	for k, v := range params {
		form[k] = v
	}
	resp, err := httpx.Do(ctx, httpx.Request{Method: "POST", URL: strings.TrimRight(d.endpoint, "/") + "/" + action, Form: form})
	if err != nil {
		return false, err
	}
	var st struct {
		Status dnspodStatus `json:"status"`
	}
	if err := resp.Decode(&st); err != nil {
		return false, err
	}
	switch st.Status.Code {
	case "1":
	case "10":
		if allowEmpty {
			return false, nil
		}
		fallthrough
	default:
		return false, fmt.Errorf("DNSPod: %s [%s]", st.Status.Message, st.Status.Code)
	}
	if out != nil {
		if err := resp.Decode(out); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (d *dnspod) Test(ctx context.Context) error {
	_, err := d.call(ctx, "Domain.List", map[string]string{"type": "all", "offset": "0", "length": "1"}, nil, true)
	return err
}

func (d *dnspod) ListDomains(ctx context.Context) ([]string, error) {
	var names []string
	for offset := 0; ; offset += dnspodPageSize {
		var r struct {
			Domains []struct {
				Name string `json:"name"`
			} `json:"domains"`
		}
		ok, err := d.call(ctx, "Domain.List", map[string]string{
			"type": "all", "offset": strconv.Itoa(offset), "length": strconv.Itoa(dnspodPageSize)}, &r, true)
		if err != nil {
			return nil, err
		}
		for _, x := range r.Domains {
			names = append(names, x.Name)
		}
		if !ok || len(r.Domains) < dnspodPageSize {
			break
		}
	}
	return names, nil
}

type dnspodRecord struct {
	ID    flexString `json:"id"`
	Name  string     `json:"name"`
	Line  string     `json:"line"`
	Type  string     `json:"type"`
	TTL   flexString `json:"ttl"`
	Value string     `json:"value"`
}

func (d *dnspod) ListRecords(ctx context.Context, domain, rr, recordType string) ([]Record, error) {
	params := map[string]string{"domain": domain, "length": strconv.Itoa(dnspodPageSize)}
	if rr != "" {
		params["sub_domain"] = rr
	}
	if recordType != "" {
		params["record_type"] = recordType
	}
	var out []Record
	for offset := 0; ; offset += dnspodPageSize {
		params["offset"] = strconv.Itoa(offset)
		var r struct {
			Records []dnspodRecord `json:"records"`
		}
		ok, err := d.call(ctx, "Record.List", params, &r, true)
		if err != nil {
			return nil, err
		}
		for _, x := range r.Records {
			if rr != "" && !strings.EqualFold(x.Name, rr) {
				continue
			}
			if recordType != "" && !strings.EqualFold(x.Type, recordType) {
				continue
			}
			out = append(out, Record{ID: string(x.ID), Name: x.Name, FQDN: FQDN(x.Name, domain), Type: x.Type,
				Value: x.Value, TTL: x.TTL.Int(), Line: x.Line})
		}
		if !ok || len(r.Records) < dnspodPageSize {
			break
		}
	}
	return out, nil
}

func (d *dnspod) params(domain string, r Record) map[string]string {
	line := r.Line
	if line == "" {
		line = dnspodDefaultLine
	}
	return map[string]string{
		"domain":      domain,
		"sub_domain":  rrOf(r.Name),
		"record_type": r.Type,
		"record_line": line,
		"value":       r.Value,
		"ttl":         strconv.Itoa(clampTTL(r.TTL, 1, 600)),
	}
}

func (d *dnspod) CreateRecord(ctx context.Context, domain string, r Record) error {
	_, err := d.call(ctx, "Record.Create", d.params(domain, r), nil, false)
	return err
}

func (d *dnspod) UpdateRecord(ctx context.Context, domain string, r Record) error {
	p := d.params(domain, r)
	p["record_id"] = r.ID
	_, err := d.call(ctx, "Record.Modify", p, nil, false)
	return err
}

func (d *dnspod) DeleteRecord(ctx context.Context, domain string, r Record) error {
	_, err := d.call(ctx, "Record.Remove", map[string]string{"domain": domain, "record_id": r.ID}, nil, false)
	return err
}
