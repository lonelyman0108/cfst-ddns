package provider

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func init() {
	Register(schema.TypeMeta{
		Type:        "huaweicloud",
		Name:        "华为云 DNS",
		Description: "华为云云解析服务，使用访问密钥 AK/SK（需 DNS FullAccess 权限）",
		DocsURL:     "https://console.huaweicloud.com/iam/#/mine/accessKey",
		Fields: []schema.Field{
			{Key: "accessKey", Label: "Access Key (AK)", Type: schema.Text, Required: true,
				Help: "在「我的凭证 → 访问密钥」创建，建议使用仅授予 DNS 权限的 IAM 用户"},
			{Key: "secretKey", Label: "Secret Key (SK)", Type: schema.Password, Required: true, Secret: true},
		},
	}, newHuaweiCloud)
}

// huaweiAPI 为云解析服务全局终端节点，测试中可替换。
var huaweiAPI = "https://dns.myhuaweicloud.com"

const (
	hwDefaultLine = "default_view"
	hwPageSize    = 500
	hwDateFormat  = "20060102T150405Z"
)

type huaweiCloud struct {
	cfg      schema.Config
	endpoint string

	mu    sync.Mutex
	zones map[string]string // 域名 → zone id
}

func newHuaweiCloud(cfg schema.Config) (Provider, error) {
	return &huaweiCloud{cfg: cfg, endpoint: huaweiAPI, zones: map[string]string{}}, nil
}

// hwCanonicalURI 对路径逐段编码并确保以 "/" 结尾。
func hwCanonicalURI(path string) string {
	segs := strings.Split(path, "/")
	for i, s := range segs {
		segs[i] = httpx.PercentEncode(s)
	}
	u := strings.Join(segs, "/")
	if !strings.HasSuffix(u, "/") {
		u += "/"
	}
	return u
}

// hwCanonicalQuery 按参数名排序编码查询串。
func hwCanonicalQuery(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		vals := append([]string(nil), q[k]...)
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, httpx.PercentEncode(k)+"="+httpx.PercentEncode(v))
		}
	}
	return strings.Join(parts, "&")
}

// hwCanonicalRequest 构造规范请求，返回规范串与 SignedHeaders；headers 的键须为小写。
func hwCanonicalRequest(method, path string, q url.Values, headers map[string]string, body []byte) (canonical, signed string) {
	keys := httpx.SortedKeys(headers)
	var ch strings.Builder
	for _, k := range keys {
		ch.WriteString(k + ":" + strings.TrimSpace(headers[k]) + "\n")
	}
	signed = strings.Join(keys, ";")
	canonical = method + "\n" + hwCanonicalURI(path) + "\n" + hwCanonicalQuery(q) + "\n" +
		ch.String() + "\n" + signed + "\n" + httpx.SHA256Hex(body)
	return canonical, signed
}

// hwStringToSign 构造待签名串。
func hwStringToSign(sdkDate, canonical string) string {
	return "SDK-HMAC-SHA256\n" + sdkDate + "\n" + httpx.SHA256Hex([]byte(canonical))
}

// hwSign 计算签名（十六进制）。
func hwSign(secretKey, stringToSign string) string {
	return hex.EncodeToString(httpx.HMACSHA256([]byte(secretKey), []byte(stringToSign)))
}

func (h *huaweiCloud) call(ctx context.Context, method, path string, q url.Values, body any, out any) error {
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = b
	}
	u, err := url.Parse(h.endpoint)
	if err != nil {
		return err
	}
	sdkDate := now().UTC().Format(hwDateFormat)
	canonical, signed := hwCanonicalRequest(method, path, q, map[string]string{
		"content-type": "application/json",
		"host":         u.Host,
		"x-sdk-date":   sdkDate,
	}, payload)
	auth := "SDK-HMAC-SHA256 Access=" + h.cfg.Get("accessKey") + ", SignedHeaders=" + signed +
		", Signature=" + hwSign(h.cfg.Get("secretKey"), hwStringToSign(sdkDate, canonical))

	full := strings.TrimRight(h.endpoint, "/") + path
	if len(q) > 0 {
		full += "?" + hwCanonicalQuery(q)
	}
	resp, err := httpx.Do(ctx, httpx.Request{Method: method, URL: full, Body: payload, Header: map[string]string{
		"Content-Type":  "application/json",
		"X-Sdk-Date":    sdkDate,
		"Authorization": auth,
	}})
	if err != nil {
		return err
	}
	if !resp.OK() {
		var e struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			ErrCode string `json:"error_code"`
			ErrMsg  string `json:"error_msg"`
		}
		_ = json.Unmarshal(resp.Body, &e)
		if e.Code == "" && e.Message == "" {
			e.Code, e.Message = e.ErrCode, e.ErrMsg
		}
		if e.Code == "" && e.Message == "" {
			return fmt.Errorf("华为云: HTTP %d: %s", resp.Status, httpx.Snippet(resp.Body))
		}
		return fmt.Errorf("华为云: %s [%s]", e.Message, e.Code)
	}
	if out != nil {
		return resp.Decode(out)
	}
	return nil
}

func (h *huaweiCloud) Test(ctx context.Context) error {
	return h.call(ctx, "GET", "/v2/zones", url.Values{"type": {"public"}, "limit": {"1"}}, nil, nil)
}

type hwZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (h *huaweiCloud) listZones(ctx context.Context, name string) ([]hwZone, error) {
	q := url.Values{"type": {"public"}, "limit": {strconv.Itoa(hwPageSize)}}
	if name != "" {
		q.Set("name", name)
	}
	var all []hwZone
	for offset := 0; ; offset += hwPageSize {
		q.Set("offset", strconv.Itoa(offset))
		var r struct {
			Zones []hwZone `json:"zones"`
		}
		if err := h.call(ctx, "GET", "/v2/zones", q, nil, &r); err != nil {
			return nil, err
		}
		for _, z := range r.Zones {
			z.Name = strings.TrimSuffix(z.Name, ".")
			h.mu.Lock()
			h.zones[strings.ToLower(z.Name)] = z.ID
			h.mu.Unlock()
			all = append(all, z)
		}
		if len(r.Zones) < hwPageSize {
			break
		}
	}
	return all, nil
}

func (h *huaweiCloud) ListDomains(ctx context.Context) ([]string, error) {
	zones, err := h.listZones(ctx, "")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(zones))
	for _, z := range zones {
		names = append(names, z.Name)
	}
	return names, nil
}

func (h *huaweiCloud) zone(ctx context.Context, domain string) (string, error) {
	key := strings.ToLower(strings.TrimSuffix(domain, "."))
	h.mu.Lock()
	id, ok := h.zones[key]
	h.mu.Unlock()
	if ok {
		return id, nil
	}
	// name 为模糊匹配，结果写入缓存后按精确名称查找
	if _, err := h.listZones(ctx, key); err != nil {
		return "", err
	}
	h.mu.Lock()
	id, ok = h.zones[key]
	h.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("华为云: 账号下未找到公网域名 %s", domain)
	}
	return id, nil
}

type hwRecordset struct {
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	TTL     int      `json:"ttl"`
	Records []string `json:"records"`
	Line    string   `json:"line,omitempty"`
}

func (rs hwRecordset) line() string {
	if rs.Line == "" {
		return hwDefaultLine
	}
	return rs.Line
}

// listRecordsets 列出记录集；rr、recordType 为空表示不过滤。
func (h *huaweiCloud) listRecordsets(ctx context.Context, zid, domain, rr, recordType string) ([]hwRecordset, error) {
	q := url.Values{"limit": {strconv.Itoa(hwPageSize)}}
	fqdn := ""
	if rr != "" {
		fqdn = FQDN(rr, domain) + "."
		q.Set("name", fqdn)
		q.Set("search_mode", "equal")
	}
	if recordType != "" {
		q.Set("type", strings.ToUpper(recordType))
	}
	var out []hwRecordset
	for offset := 0; ; offset += hwPageSize {
		q.Set("offset", strconv.Itoa(offset))
		var r struct {
			Recordsets []hwRecordset `json:"recordsets"`
		}
		if err := h.call(ctx, "GET", "/v2.1/zones/"+zid+"/recordsets", q, nil, &r); err != nil {
			return nil, err
		}
		for _, rs := range r.Recordsets {
			if fqdn != "" && !strings.EqualFold(rs.Name, fqdn) {
				continue
			}
			if recordType != "" && !strings.EqualFold(rs.Type, recordType) {
				continue
			}
			out = append(out, rs)
		}
		if len(r.Recordsets) < hwPageSize {
			break
		}
	}
	return out, nil
}

func (h *huaweiCloud) ListRecords(ctx context.Context, domain, rr, recordType string) ([]Record, error) {
	zid, err := h.zone(ctx, domain)
	if err != nil {
		return nil, err
	}
	sets, err := h.listRecordsets(ctx, zid, domain, rr, recordType)
	if err != nil {
		return nil, err
	}
	var out []Record
	for _, rs := range sets {
		name := strings.TrimSuffix(rs.Name, ".")
		for _, v := range rs.Records {
			out = append(out, Record{ID: rs.ID + "|" + v, Name: RR(name, domain), FQDN: name, Type: rs.Type,
				Value: v, TTL: rs.TTL, Line: rs.line()})
		}
	}
	return out, nil
}

// hwSplitID 把 "recordsetId|值" 拆开。
func hwSplitID(id string) (string, string, error) {
	i := strings.IndexByte(id, '|')
	if i <= 0 {
		return "", "", fmt.Errorf("华为云: 无效的记录 ID %q", id)
	}
	return id[:i], id[i+1:], nil
}

func (h *huaweiCloud) getRecordset(ctx context.Context, zid, rsid string) (*hwRecordset, error) {
	var rs hwRecordset
	if err := h.call(ctx, "GET", "/v2.1/zones/"+zid+"/recordsets/"+rsid, nil, nil, &rs); err != nil {
		return nil, err
	}
	return &rs, nil
}

// putRecordset 更新记录集的值；值为空时删除整个记录集。
func (h *huaweiCloud) putRecordset(ctx context.Context, zid string, rs *hwRecordset) error {
	if len(rs.Records) == 0 {
		return h.call(ctx, "DELETE", "/v2.1/zones/"+zid+"/recordsets/"+rs.ID, nil, nil, nil)
	}
	body := map[string]any{"name": rs.Name, "type": rs.Type, "ttl": rs.TTL, "records": rs.Records}
	return h.call(ctx, "PUT", "/v2.1/zones/"+zid+"/recordsets/"+rs.ID, nil, body, nil)
}

func without(values []string, v string) []string {
	out := make([]string, 0, len(values))
	for _, x := range values {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func contains(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}

func (h *huaweiCloud) CreateRecord(ctx context.Context, domain string, r Record) error {
	zid, err := h.zone(ctx, domain)
	if err != nil {
		return err
	}
	line := r.Line
	if line == "" {
		line = hwDefaultLine
	}
	ttl := clampTTL(r.TTL, 1, 300)
	sets, err := h.listRecordsets(ctx, zid, domain, rrOf(r.Name), r.Type)
	if err != nil {
		return err
	}
	for i := range sets {
		rs := &sets[i]
		if rs.line() != line {
			continue
		}
		// 同名同类型同线路的记录集已存在：追加记录值
		rs.Records = append(without(rs.Records, r.Value), r.Value)
		if r.TTL > 0 {
			rs.TTL = r.TTL
		}
		return h.putRecordset(ctx, zid, rs)
	}
	body := hwRecordset{Name: FQDN(rrOf(r.Name), domain) + ".", Type: strings.ToUpper(r.Type), TTL: ttl,
		Records: []string{r.Value}, Line: line}
	return h.call(ctx, "POST", "/v2.1/zones/"+zid+"/recordsets", nil, body, nil)
}

func (h *huaweiCloud) UpdateRecord(ctx context.Context, domain string, r Record) error {
	zid, err := h.zone(ctx, domain)
	if err != nil {
		return err
	}
	rsid, old, err := hwSplitID(r.ID)
	if err != nil {
		return err
	}
	rs, err := h.getRecordset(ctx, zid, rsid)
	if err != nil {
		return err
	}
	line := r.Line
	if line == "" {
		line = hwDefaultLine
	}
	sameSet := strings.EqualFold(strings.TrimSuffix(rs.Name, "."), FQDN(rrOf(r.Name), domain)) &&
		strings.EqualFold(rs.Type, r.Type) && rs.line() == line
	if !sameSet {
		// 主机记录/类型/线路发生变化：从原记录集移除后按新属性创建
		rs.Records = without(rs.Records, old)
		if err := h.putRecordset(ctx, zid, rs); err != nil {
			return err
		}
		return h.CreateRecord(ctx, domain, r)
	}
	var vals []string
	for _, v := range rs.Records {
		if v == old {
			v = r.Value
		}
		if !contains(vals, v) {
			vals = append(vals, v)
		}
	}
	if !contains(vals, r.Value) {
		vals = append(vals, r.Value)
	}
	rs.Records = vals
	if r.TTL > 0 {
		rs.TTL = r.TTL
	}
	return h.putRecordset(ctx, zid, rs)
}

func (h *huaweiCloud) DeleteRecord(ctx context.Context, domain string, r Record) error {
	zid, err := h.zone(ctx, domain)
	if err != nil {
		return err
	}
	rsid, old, err := hwSplitID(r.ID)
	if err != nil {
		return err
	}
	rs, err := h.getRecordset(ctx, zid, rsid)
	if err != nil {
		return err
	}
	rs.Records = without(rs.Records, old)
	return h.putRecordset(ctx, zid, rs)
}
