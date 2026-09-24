package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func init() {
	Register(schema.TypeMeta{
		Type:        "alidns",
		Name:        "阿里云 DNS",
		Description: "阿里云云解析 DNS，使用 AccessKey（需 AliyunDNSFullAccess 权限）",
		DocsURL:     "https://ram.console.aliyun.com/manage/ak",
		Fields: []schema.Field{
			{Key: "accessKeyId", Label: "AccessKey ID", Type: schema.Text, Required: true,
				Help: "建议创建 RAM 子用户并仅授予云解析权限"},
			{Key: "accessKeySecret", Label: "AccessKey Secret", Type: schema.Password, Required: true, Secret: true},
		},
	}, newAliDNS)
}

// aliAPI 为云解析 API 地址，测试中可替换。
var aliAPI = "https://alidns.aliyuncs.com"

const (
	aliVersion     = "2015-01-09"
	aliDefaultLine = "default"
	aliMinTTL      = 600
	aliDomainPage  = 100
	aliRecordPage  = 500
)

type aliDNS struct {
	cfg      schema.Config
	endpoint string
}

func newAliDNS(cfg schema.Config) (Provider, error) {
	return &aliDNS{cfg: cfg, endpoint: aliAPI}, nil
}

// aliCanonicalQuery 按参数名排序并以 RFC 3986 编码拼接。
func aliCanonicalQuery(params map[string]string) string {
	keys := httpx.SortedKeys(params)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, httpx.PercentEncode(k)+"="+httpx.PercentEncode(params[k]))
	}
	return strings.Join(parts, "&")
}

// aliStringToSign 构造签名 v1 待签名串。
func aliStringToSign(method string, params map[string]string) string {
	return method + "&" + httpx.PercentEncode("/") + "&" + httpx.PercentEncode(aliCanonicalQuery(params))
}

// aliSign 计算 HMAC-SHA1 签名（base64）。
func aliSign(method string, params map[string]string, secret string) string {
	return base64.StdEncoding.EncodeToString(httpx.HMACSHA1([]byte(secret+"&"), []byte(aliStringToSign(method, params))))
}

func (a *aliDNS) call(ctx context.Context, action string, params map[string]string, out any) error {
	p := map[string]string{
		"Format":           "JSON",
		"Version":          aliVersion,
		"AccessKeyId":      a.cfg.Get("accessKeyId"),
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureVersion": "1.0",
		"SignatureNonce":   nonce(),
		"Timestamp":        now().UTC().Format("2006-01-02T15:04:05Z"),
		"Action":           action,
	}
	for k, v := range params {
		p[k] = v
	}
	p["Signature"] = aliSign("GET", p, a.cfg.Get("accessKeySecret"))
	resp, err := httpx.Do(ctx, httpx.Request{Method: "GET",
		URL: strings.TrimRight(a.endpoint, "/") + "/?" + aliCanonicalQuery(p)})
	if err != nil {
		return err
	}
	if !resp.OK() {
		var e struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		}
		if err := resp.Decode(&e); err != nil || e.Code == "" {
			return fmt.Errorf("阿里云: HTTP %d: %s", resp.Status, httpx.Snippet(resp.Body))
		}
		return &aliAPIError{Code: e.Code, Message: e.Message}
	}
	if out != nil {
		return resp.Decode(out)
	}
	return nil
}

type aliAPIError struct{ Code, Message string }

func (e *aliAPIError) Error() string { return fmt.Sprintf("阿里云: %s [%s]", e.Message, e.Code) }

func (a *aliDNS) Test(ctx context.Context) error {
	return a.call(ctx, "DescribeDomains", map[string]string{"PageNumber": "1", "PageSize": "1"}, nil)
}

func (a *aliDNS) ListDomains(ctx context.Context) ([]string, error) {
	var names []string
	for page := 1; ; page++ {
		var r struct {
			Domains struct {
				Domain []struct {
					DomainName string `json:"DomainName"`
				} `json:"Domain"`
			} `json:"Domains"`
		}
		err := a.call(ctx, "DescribeDomains", map[string]string{
			"PageNumber": strconv.Itoa(page), "PageSize": strconv.Itoa(aliDomainPage)}, &r)
		if err != nil {
			return nil, err
		}
		for _, d := range r.Domains.Domain {
			names = append(names, d.DomainName)
		}
		if len(r.Domains.Domain) < aliDomainPage {
			break
		}
	}
	return names, nil
}

type aliRecord struct {
	RecordID string `json:"RecordId"`
	RR       string `json:"RR"`
	Type     string `json:"Type"`
	Value    string `json:"Value"`
	TTL      int    `json:"TTL"`
	Line     string `json:"Line"`
}

func (a *aliDNS) ListRecords(ctx context.Context, domain, rr, recordType string) ([]Record, error) {
	params := map[string]string{"DomainName": domain, "PageSize": strconv.Itoa(aliRecordPage)}
	if rr != "" {
		params["RRKeyWord"] = rr
	}
	if recordType != "" {
		params["Type"] = recordType
	}
	var out []Record
	for page := 1; ; page++ {
		params["PageNumber"] = strconv.Itoa(page)
		var r struct {
			DomainRecords struct {
				Record []aliRecord `json:"Record"`
			} `json:"DomainRecords"`
		}
		if err := a.call(ctx, "DescribeDomainRecords", params, &r); err != nil {
			return nil, err
		}
		for _, x := range r.DomainRecords.Record {
			// RRKeyWord 为模糊匹配，需再精确比对
			if rr != "" && !strings.EqualFold(x.RR, rr) {
				continue
			}
			if recordType != "" && !strings.EqualFold(x.Type, recordType) {
				continue
			}
			out = append(out, Record{ID: x.RecordID, Name: x.RR, FQDN: FQDN(x.RR, domain), Type: x.Type,
				Value: x.Value, TTL: x.TTL, Line: x.Line})
		}
		if len(r.DomainRecords.Record) < aliRecordPage {
			break
		}
	}
	return out, nil
}

func (a *aliDNS) params(r Record) map[string]string {
	line := r.Line
	if line == "" {
		line = aliDefaultLine
	}
	return map[string]string{
		"RR":    rrOf(r.Name),
		"Type":  r.Type,
		"Value": r.Value,
		"TTL":   strconv.Itoa(clampTTL(r.TTL, aliMinTTL, aliMinTTL)),
		"Line":  line,
	}
}

func (a *aliDNS) CreateRecord(ctx context.Context, domain string, r Record) error {
	p := a.params(r)
	p["DomainName"] = domain
	return a.call(ctx, "AddDomainRecord", p, nil)
}

func (a *aliDNS) UpdateRecord(ctx context.Context, domain string, r Record) error {
	p := a.params(r)
	p["RecordId"] = r.ID
	err := a.call(ctx, "UpdateDomainRecord", p, nil)
	// 记录内容未变化时阿里云返回 DomainRecordDuplicate，视为成功
	if e, ok := err.(*aliAPIError); ok && e.Code == "DomainRecordDuplicate" {
		return nil
	}
	return err
}

func (a *aliDNS) DeleteRecord(ctx context.Context, domain string, r Record) error {
	return a.call(ctx, "DeleteDomainRecord", map[string]string{"RecordId": r.ID}, nil)
}
