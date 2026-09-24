package provider

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func TestHuaweiCanonical(t *testing.T) {
	if got := hwCanonicalURI("/v2/zones"); got != "/v2/zones/" {
		t.Fatalf("uri %s", got)
	}
	if got := hwCanonicalURI("/v2.1/zones/abc/recordsets/"); got != "/v2.1/zones/abc/recordsets/" {
		t.Fatalf("uri %s", got)
	}
	q := url.Values{"type": {"public"}, "name": {"www.example.com."}, "limit": {"500"}}
	if got := hwCanonicalQuery(q); got != "limit=500&name=www.example.com.&type=public" {
		t.Fatalf("query %s", got)
	}
	cr, signed := hwCanonicalRequest("GET", "/v2/zones", url.Values{"type": {"public"}}, map[string]string{
		"content-type": "application/json",
		"host":         "dns.myhuaweicloud.com",
		"x-sdk-date":   "20191115T033655Z",
	}, nil)
	want := "GET\n/v2/zones/\ntype=public\n" +
		"content-type:application/json\nhost:dns.myhuaweicloud.com\nx-sdk-date:20191115T033655Z\n\n" +
		"content-type;host;x-sdk-date\n" +
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // sha256("")
	if cr != want || signed != "content-type;host;x-sdk-date" {
		t.Fatalf("canonical request\n%q\n%q", cr, want)
	}
	sts := hwStringToSign("20191115T033655Z", cr)
	if sts != "SDK-HMAC-SHA256\n20191115T033655Z\n"+httpx.SHA256Hex([]byte(cr)) {
		t.Fatalf("string to sign %q", sts)
	}
	m := hmac.New(sha256.New, []byte("sk"))
	m.Write([]byte(sts))
	if got := hwSign("sk", sts); got != hex.EncodeToString(m.Sum(nil)) {
		t.Fatalf("sign %s", got)
	}
}

// hwFake 模拟一个 zone 下的记录集存储。
type hwFake struct {
	sets map[string]*hwRecordset
}

func newTestHuawei(t *testing.T, sets ...hwRecordset) (Provider, *fakeAPI, *hwFake) {
	fixClock(t, 1573788615) // 2019-11-15T03:30:15Z
	st := &hwFake{sets: map[string]*hwRecordset{}}
	for i := range sets {
		st.sets[sets[i].ID] = &sets[i]
	}
	api := newFakeAPI(t, func(c captured) (int, string) {
		checkHuaweiAuth(t, c)
		const base = "/v2.1/zones/zone1/recordsets"
		switch {
		case c.Method == "GET" && c.Path == "/v2/zones":
			return 200, `{"zones":[{"id":"zone0","name":"sub.example.com."},{"id":"zone1","name":"example.com."}],"metadata":{"total_count":2}}`
		case c.Method == "GET" && c.Path == base:
			var list []hwRecordset
			for _, rs := range st.sets {
				if (c.Query.Get("name") == "" || rs.Name == c.Query.Get("name")) &&
					(c.Query.Get("type") == "" || rs.Type == c.Query.Get("type")) {
					list = append(list, *rs)
				}
			}
			b, _ := json.Marshal(map[string]any{"recordsets": list})
			return 200, string(b)
		case c.Method == "POST" && c.Path == base:
			var rs hwRecordset
			_ = json.Unmarshal(c.Body, &rs)
			rs.ID = "new"
			st.sets[rs.ID] = &rs
			return 202, `{"id":"new"}`
		case strings.HasPrefix(c.Path, base+"/"):
			id := strings.TrimPrefix(c.Path, base+"/")
			rs, ok := st.sets[id]
			if !ok {
				return 404, `{"code":"DNS.0312","message":"Record set does not exist."}`
			}
			switch c.Method {
			case "GET":
				b, _ := json.Marshal(rs)
				return 200, string(b)
			case "PUT":
				var body hwRecordset
				_ = json.Unmarshal(c.Body, &body)
				rs.Records, rs.TTL = body.Records, body.TTL
				return 202, `{}`
			case "DELETE":
				delete(st.sets, id)
				return 202, `{}`
			}
		}
		return 400, `{"code":"DNS.0001","message":"unexpected"}`
	})
	setEndpoint(t, &huaweiAPI, api.URL)
	return mustNew(t, "huaweicloud", schema.Config{"accessKey": "AK", "secretKey": "SK"}), api, st
}

func checkHuaweiAuth(t *testing.T, c captured) {
	t.Helper()
	if c.Header.Get("X-Sdk-Date") != "20191115T033015Z" {
		t.Errorf("bad X-Sdk-Date %q", c.Header.Get("X-Sdk-Date"))
	}
	q := url.Values{}
	for k, v := range c.Query {
		q[k] = v
	}
	var body []byte
	if len(c.Body) > 0 {
		body = c.Body
	}
	cr, signed := hwCanonicalRequest(c.Method, c.Path, q, map[string]string{
		"content-type": c.Header.Get("Content-Type"), "host": c.Host, "x-sdk-date": c.Header.Get("X-Sdk-Date")}, body)
	want := "SDK-HMAC-SHA256 Access=AK, SignedHeaders=" + signed + ", Signature=" +
		hwSign("SK", hwStringToSign("20191115T033015Z", cr))
	if c.Header.Get("Authorization") != want {
		t.Errorf("auth\n%s\n%s", c.Header.Get("Authorization"), want)
	}
}

func TestHuaweiListAndSplit(t *testing.T) {
	p, api, _ := newTestHuawei(t,
		hwRecordset{ID: "rs1", Name: "www.example.com.", Type: "A", TTL: 300, Records: []string{"1.1.1.1", "2.2.2.2"}, Line: "default_view"},
		hwRecordset{ID: "rs2", Name: "api.example.com.", Type: "A", TTL: 300, Records: []string{"3.3.3.3"}})
	ds, err := p.ListDomains(bg)
	if err != nil || strings.Join(ds, ",") != "sub.example.com,example.com" {
		t.Fatalf("domains %v %v", ds, err)
	}
	recs, err := p.ListRecords(bg, "example.com", "www", "A")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 || recs[0].ID != "rs1|1.1.1.1" || recs[1].ID != "rs1|2.2.2.2" ||
		recs[0].Name != "www" || recs[0].FQDN != "www.example.com" || recs[0].Line != "default_view" {
		t.Fatalf("bad records %+v", recs)
	}
	last := api.requests()[len(api.requests())-1]
	if last.Query.Get("name") != "www.example.com." || last.Query.Get("type") != "A" || last.Query.Get("search_mode") != "equal" {
		t.Fatalf("bad query %v", last.Query)
	}
}

func TestHuaweiWrite(t *testing.T) {
	p, api, st := newTestHuawei(t,
		hwRecordset{ID: "rs1", Name: "www.example.com.", Type: "A", TTL: 300, Records: []string{"1.1.1.1"}})
	// 已存在记录集：追加值
	if err := p.CreateRecord(bg, "example.com", Record{Name: "www", Type: "A", Value: "2.2.2.2"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(st.sets["rs1"].Records, ","); got != "1.1.1.1,2.2.2.2" {
		t.Fatalf("after create %s", got)
	}
	// 更新单个值
	if err := p.UpdateRecord(bg, "example.com", Record{ID: "rs1|1.1.1.1", Name: "www", Type: "A", Value: "9.9.9.9", TTL: 60}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(st.sets["rs1"].Records, ","); got != "9.9.9.9,2.2.2.2" || st.sets["rs1"].TTL != 60 {
		t.Fatalf("after update %s ttl %d", got, st.sets["rs1"].TTL)
	}
	// 删除值，最后一个值删除时删除整个记录集
	if err := p.DeleteRecord(bg, "example.com", Record{ID: "rs1|9.9.9.9"}); err != nil {
		t.Fatal(err)
	}
	if err := p.DeleteRecord(bg, "example.com", Record{ID: "rs1|2.2.2.2"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := st.sets["rs1"]; ok {
		t.Fatal("recordset should be deleted")
	}
	// 不存在记录集：新建
	if err := p.CreateRecord(bg, "example.com", Record{Name: "@", Type: "AAAA", Value: "::1"}); err != nil {
		t.Fatal(err)
	}
	var post *captured
	for _, c := range api.requests() {
		if c.Method == "POST" {
			c := c
			post = &c
		}
	}
	if post == nil {
		t.Fatal("no POST")
	}
	var body map[string]any
	_ = json.Unmarshal(post.Body, &body)
	if body["name"] != "example.com." || body["type"] != "AAAA" || body["ttl"] != 300.0 || body["line"] != "default_view" {
		t.Fatalf("bad create body %v", body)
	}
	if err := p.DeleteRecord(bg, "example.com", Record{ID: "missing|1.1.1.1"}); err == nil ||
		!strings.Contains(err.Error(), "DNS.0312") || !strings.Contains(err.Error(), "华为云") {
		t.Fatalf("want not found error, got %v", err)
	}
	if err := p.DeleteRecord(bg, "example.com", Record{ID: "bad"}); err == nil {
		t.Fatal("want invalid id error")
	}
}

func TestHuaweiZoneNotFound(t *testing.T) {
	p, _, _ := newTestHuawei(t)
	if _, err := p.ListRecords(bg, "other.com", "", ""); err == nil || !strings.Contains(err.Error(), "未找到") {
		t.Fatalf("got %v", err)
	}
}
