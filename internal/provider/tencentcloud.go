package provider

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func init() {
	Register(schema.TypeMeta{
		Type:        "tencentcloud",
		Name:        "腾讯云 DNSPod",
		Description: "腾讯云 API 3.0，使用访问密钥 SecretId / SecretKey（需 QcloudDNSPodFullAccess 权限）",
		DocsURL:     "https://console.cloud.tencent.com/cam/capi",
		Fields: []schema.Field{
			{Key: "secretId", Label: "SecretId", Type: schema.Text, Required: true,
				Help: "建议在访问管理中创建子用户并仅授予 DNSPod 权限"},
			{Key: "secretKey", Label: "SecretKey", Type: schema.Password, Required: true, Secret: true},
		},
	}, newTencentCloud)
}

// tencentAPI 为 DNSPod API 3.0 地址，测试中可替换。
var tencentAPI = "https://dnspod.tencentcloudapi.com"

const (
	tcVersion     = "2021-03-23"
	tcService     = "dnspod"
	tcContentType = "application/json; charset=utf-8"
	tcDefaultLine = "默认"
	tcPageSize    = 3000
)

type tencentCloud struct {
	cfg      schema.Config
	endpoint string
}

func newTencentCloud(cfg schema.Config) (Provider, error) {
	return &tencentCloud{cfg: cfg, endpoint: tencentAPI}, nil
}

// tc3CanonicalRequest 构造 TC3 规范请求串，签名头为 content-type;host;x-tc-action。
func tc3CanonicalRequest(host, action string, payload []byte) string {
	return "POST\n/\n\n" +
		"content-type:" + tcContentType + "\n" +
		"host:" + host + "\n" +
		"x-tc-action:" + strings.ToLower(action) + "\n\n" +
		"content-type;host;x-tc-action\n" +
		httpx.SHA256Hex(payload)
}

// tc3StringToSign 构造待签名串，date 为 UTC 日期。
func tc3StringToSign(ts int64, service, canonical string) string {
	date := time.Unix(ts, 0).UTC().Format("2006-01-02")
	return "TC3-HMAC-SHA256\n" + strconv.FormatInt(ts, 10) + "\n" +
		date + "/" + service + "/tc3_request\n" + httpx.SHA256Hex([]byte(canonical))
}

// tc3Sign 计算签名（十六进制）。
func tc3Sign(secretKey, service string, ts int64, stringToSign string) string {
	date := time.Unix(ts, 0).UTC().Format("2006-01-02")
	k := httpx.HMACSHA256([]byte("TC3"+secretKey), []byte(date))
	k = httpx.HMACSHA256(k, []byte(service))
	k = httpx.HMACSHA256(k, []byte("tc3_request"))
	return hex.EncodeToString(httpx.HMACSHA256(k, []byte(stringToSign)))
}

// tc3Authorization 返回 Authorization 头。
func tc3Authorization(secretID, secretKey, service, host, action string, ts int64, payload []byte) string {
	sts := tc3StringToSign(ts, service, tc3CanonicalRequest(host, action, payload))
	date := time.Unix(ts, 0).UTC().Format("2006-01-02")
	return "TC3-HMAC-SHA256 Credential=" + secretID + "/" + date + "/" + service + "/tc3_request, " +
		"SignedHeaders=content-type;host;x-tc-action, Signature=" + tc3Sign(secretKey, service, ts, sts)
}

type tcError struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

func (t *tencentCloud) call(ctx context.Context, action string, params map[string]any, out any) error {
	if params == nil {
		params = map[string]any{}
	}
	payload, err := json.Marshal(params)
	if err != nil {
		return err
	}
	u, err := url.Parse(t.endpoint)
	if err != nil {
		return err
	}
	ts := now().Unix()
	resp, err := httpx.Do(ctx, httpx.Request{Method: "POST", URL: t.endpoint, Body: payload, Header: map[string]string{
		"Authorization":  tc3Authorization(t.cfg.Get("secretId"), t.cfg.Get("secretKey"), tcService, u.Host, action, ts, payload),
		"Content-Type":   tcContentType,
		"X-TC-Action":    action,
		"X-TC-Version":   tcVersion,
		"X-TC-Timestamp": strconv.FormatInt(ts, 10),
	}})
	if err != nil {
		return err
	}
	var r struct {
		Response struct {
			Error *tcError `json:"Error"`
		} `json:"Response"`
	}
	if err := resp.Decode(&r); err != nil {
		return err
	}
	if e := r.Response.Error; e != nil {
		return &tcAPIError{*e}
	}
	if !resp.OK() {
		return fmt.Errorf("腾讯云: HTTP %d: %s", resp.Status, httpx.Snippet(resp.Body))
	}
	if out != nil {
		var w struct {
			Response json.RawMessage `json:"Response"`
		}
		if err := resp.Decode(&w); err != nil {
			return err
		}
		if err := json.Unmarshal(w.Response, out); err != nil {
			return fmt.Errorf("腾讯云: 解析响应失败: %w", err)
		}
	}
	return nil
}

type tcAPIError struct{ tcError }

func (e *tcAPIError) Error() string { return fmt.Sprintf("腾讯云: %s [%s]", e.Message, e.Code) }

func (t *tencentCloud) Test(ctx context.Context) error {
	return t.call(ctx, "DescribeDomainList", map[string]any{"Offset": 0, "Limit": 1}, nil)
}

func (t *tencentCloud) ListDomains(ctx context.Context) ([]string, error) {
	var names []string
	for offset := 0; ; offset += tcPageSize {
		var r struct {
			DomainList []struct {
				Name string `json:"Name"`
			} `json:"DomainList"`
		}
		err := t.call(ctx, "DescribeDomainList", map[string]any{"Type": "ALL", "Offset": offset, "Limit": tcPageSize}, &r)
		if err != nil {
			// 账号下没有域名时返回 ResourceNotFound.NoDataOfDomain
			if e, ok := err.(*tcAPIError); ok && strings.HasPrefix(e.Code, "ResourceNotFound.NoData") {
				break
			}
			return nil, err
		}
		for _, d := range r.DomainList {
			names = append(names, d.Name)
		}
		if len(r.DomainList) < tcPageSize {
			break
		}
	}
	return names, nil
}

type tcRecord struct {
	RecordID uint64 `json:"RecordId"`
	Name     string `json:"Name"`
	Type     string `json:"Type"`
	Value    string `json:"Value"`
	Line     string `json:"Line"`
	TTL      int    `json:"TTL"`
}

func (t *tencentCloud) ListRecords(ctx context.Context, domain, rr, recordType string) ([]Record, error) {
	params := map[string]any{"Domain": domain, "Limit": tcPageSize}
	if rr != "" {
		params["Subdomain"] = rr
	}
	if recordType != "" {
		params["RecordType"] = recordType
	}
	var out []Record
	for offset := 0; ; offset += tcPageSize {
		params["Offset"] = offset
		var r struct {
			RecordList []tcRecord `json:"RecordList"`
		}
		if err := t.call(ctx, "DescribeRecordList", params, &r); err != nil {
			if e, ok := err.(*tcAPIError); ok && e.Code == "ResourceNotFound.NoDataOfRecord" {
				break
			}
			return nil, err
		}
		for _, x := range r.RecordList {
			if rr != "" && !strings.EqualFold(x.Name, rr) {
				continue
			}
			if recordType != "" && !strings.EqualFold(x.Type, recordType) {
				continue
			}
			out = append(out, Record{ID: strconv.FormatUint(x.RecordID, 10), Name: x.Name, FQDN: FQDN(x.Name, domain),
				Type: x.Type, Value: x.Value, TTL: x.TTL, Line: x.Line})
		}
		if len(r.RecordList) < tcPageSize {
			break
		}
	}
	return out, nil
}

func (t *tencentCloud) params(domain string, r Record) map[string]any {
	line := r.Line
	if line == "" {
		line = tcDefaultLine
	}
	return map[string]any{
		"Domain":     domain,
		"SubDomain":  rrOf(r.Name),
		"RecordType": r.Type,
		"RecordLine": line,
		"Value":      r.Value,
		"TTL":        clampTTL(r.TTL, 1, 600),
	}
}

func (t *tencentCloud) recordID(r Record) (uint64, error) {
	id, err := strconv.ParseUint(r.ID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("腾讯云: 无效的记录 ID %q", r.ID)
	}
	return id, nil
}

func (t *tencentCloud) CreateRecord(ctx context.Context, domain string, r Record) error {
	return t.call(ctx, "CreateRecord", t.params(domain, r), nil)
}

func (t *tencentCloud) UpdateRecord(ctx context.Context, domain string, r Record) error {
	id, err := t.recordID(r)
	if err != nil {
		return err
	}
	p := t.params(domain, r)
	p["RecordId"] = id
	return t.call(ctx, "ModifyRecord", p, nil)
}

func (t *tencentCloud) DeleteRecord(ctx context.Context, domain string, r Record) error {
	id, err := t.recordID(r)
	if err != nil {
		return err
	}
	return t.call(ctx, "DeleteRecord", map[string]any{"Domain": domain, "RecordId": id}, nil)
}
