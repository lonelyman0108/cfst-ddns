package provider

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

// newTestGoDaddy 模拟 www 的 A 记录集合。
func newTestGoDaddy(t *testing.T, values ...string) (Provider, *fakeAPI, *[]gdRecord) {
	set := []gdRecord{}
	for _, v := range values {
		set = append(set, gdRecord{Data: v, Name: "www", TTL: 600, Type: "A"})
	}
	api := newFakeAPI(t, func(c captured) (int, string) {
		if c.Header.Get("Authorization") != "sso-key KEY:SECRET" {
			return 401, `{"code":"UNABLE_TO_AUTHENTICATE","message":"Unauthorized"}`
		}
		switch {
		case c.Path == "/v1/domains":
			return 200, `[{"domain":"a.com","status":"ACTIVE"},{"domain":"b.com","status":"ACTIVE"}]`
		case c.Path == "/v1/domains/example.com/records/A/www":
			switch c.Method {
			case "GET":
				b, _ := json.Marshal(set)
				return 200, string(b)
			case "PUT":
				var body []gdRecord
				_ = json.Unmarshal(c.Body, &body)
				set = body
				return 200, ``
			case "DELETE":
				set = nil
				return 204, ``
			}
		case c.Path == "/v1/domains/example.com/records":
			return 200, `[{"data":"1.1.1.1","name":"www","ttl":600,"type":"A"},{"data":"ns1","name":"@","ttl":3600,"type":"NS"}]`
		}
		return 404, `{"code":"NOT_FOUND","message":"not found"}`
	})
	setEndpoint(t, &godaddyAPI, api.URL)
	return mustNew(t, "godaddy", schema.Config{"apiKey": "KEY", "apiSecret": "SECRET"}), api, &set
}

func TestGoDaddyList(t *testing.T) {
	p, api, _ := newTestGoDaddy(t, "1.1.1.1", "2.2.2.2")
	ds, err := p.ListDomains(bg)
	if err != nil || strings.Join(ds, ",") != "a.com,b.com" {
		t.Fatalf("domains %v %v", ds, err)
	}
	if q := api.requests()[0].Query; q.Get("statuses") != "ACTIVE" {
		t.Fatalf("bad query %v", q)
	}
	recs, err := p.ListRecords(bg, "example.com", "www", "A")
	if err != nil || len(recs) != 2 || recs[1].ID != "A|www|2.2.2.2" || recs[1].FQDN != "www.example.com" {
		t.Fatalf("records %+v %v", recs, err)
	}
	// 仅按名称过滤：拉取全部后本地过滤
	recs, err = p.ListRecords(bg, "example.com", "www", "")
	if err != nil || len(recs) != 1 || recs[0].Type != "A" {
		t.Fatalf("records %+v %v", recs, err)
	}
}

func TestGoDaddyWrite(t *testing.T) {
	p, api, set := newTestGoDaddy(t, "1.1.1.1")
	if err := p.CreateRecord(bg, "example.com", Record{Name: "www", Type: "A", Value: "2.2.2.2", TTL: 60}); err != nil {
		t.Fatal(err)
	}
	var put captured
	for _, c := range api.requests() {
		if c.Method == "PUT" {
			put = c
		}
	}
	var body []map[string]any
	_ = json.Unmarshal(put.Body, &body)
	if len(body) != 2 || body[1]["data"] != "2.2.2.2" || body[1]["ttl"] != 600.0 || put.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("bad PUT body %s", put.Body)
	}
	if err := p.UpdateRecord(bg, "example.com", Record{ID: "A|www|1.1.1.1", Name: "www", Type: "A", Value: "3.3.3.3"}); err != nil {
		t.Fatal(err)
	}
	var vals []string
	for _, r := range *set {
		vals = append(vals, r.Data)
	}
	if strings.Join(vals, ",") != "2.2.2.2,3.3.3.3" {
		t.Fatalf("after update %v", vals)
	}
	for _, v := range []string{"2.2.2.2", "3.3.3.3"} {
		if err := p.DeleteRecord(bg, "example.com", Record{ID: "A|www|" + v}); err != nil {
			t.Fatal(err)
		}
	}
	reqs := api.requests()
	if last := reqs[len(reqs)-1]; last.Method != "DELETE" || last.Path != "/v1/domains/example.com/records/A/www" {
		t.Fatalf("want DELETE of empty set, got %s %s", last.Method, last.Path)
	}
	if err := p.DeleteRecord(bg, "example.com", Record{ID: "bad"}); err == nil {
		t.Fatal("want invalid id error")
	}
}

func TestGoDaddyError(t *testing.T) {
	p, _, _ := newTestGoDaddy(t)
	g := p.(*goDaddy)
	g.cfg = schema.Config{"apiKey": "KEY", "apiSecret": "WRONG"}
	err := p.Test(bg)
	if err == nil || !strings.Contains(err.Error(), "UNABLE_TO_AUTHENTICATE") || !strings.Contains(err.Error(), "GoDaddy") {
		t.Fatalf("got %v", err)
	}
}
