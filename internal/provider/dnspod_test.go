package provider

import (
	"strings"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func newTestDNSPod(t *testing.T, handler func(c captured) (int, string)) (Provider, *fakeAPI) {
	api := newFakeAPI(t, handler)
	setEndpoint(t, &dnspodAPI, api.URL)
	return mustNew(t, "dnspod", schema.Config{"tokenId": "12345", "token": "abcdef"}), api
}

func TestDNSPodListRecords(t *testing.T) {
	p, api := newTestDNSPod(t, func(c captured) (int, string) {
		return 200, `{"status":{"code":"1","message":"Action completed successful"},"info":{"record_total":"2"},
			"records":[{"id":"1001","name":"www","line":"默认","type":"A","ttl":"600","value":"1.1.1.1"},
			{"id":1002,"name":"www2","line":"默认","type":"A","ttl":"600","value":"2.2.2.2"}]}`
	})
	recs, err := p.ListRecords(bg, "example.com", "www", "A")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("want 1 record, got %+v", recs)
	}
	r := recs[0]
	if r.ID != "1001" || r.FQDN != "www.example.com" || r.TTL != 600 || r.Line != "默认" || r.Value != "1.1.1.1" {
		t.Fatalf("bad record %+v", r)
	}
	req := api.requests()[0]
	f := req.Form()
	if req.Method != "POST" || req.Path != "/Record.List" {
		t.Fatalf("bad request %s %s", req.Method, req.Path)
	}
	if f.Get("login_token") != "12345,abcdef" || f.Get("format") != "json" || f.Get("domain") != "example.com" ||
		f.Get("sub_domain") != "www" || f.Get("record_type") != "A" || f.Get("offset") != "0" {
		t.Fatalf("bad form %v", f)
	}
}

func TestDNSPodListRecordsEmpty(t *testing.T) {
	p, _ := newTestDNSPod(t, func(c captured) (int, string) {
		return 200, `{"status":{"code":"10","message":"记录列表为空"}}`
	})
	recs, err := p.ListRecords(bg, "example.com", "www", "A")
	if err != nil || len(recs) != 0 {
		t.Fatalf("want empty, got %v %v", recs, err)
	}
}

func TestDNSPodCreateModifyRemove(t *testing.T) {
	p, api := newTestDNSPod(t, func(c captured) (int, string) {
		return 200, `{"status":{"code":"1","message":"ok"}}`
	})
	if err := p.CreateRecord(bg, "example.com", Record{Name: "", Type: "A", Value: "1.2.3.4"}); err != nil {
		t.Fatal(err)
	}
	if err := p.UpdateRecord(bg, "example.com", Record{ID: "99", Name: "www", Type: "AAAA", Value: "::1", TTL: 120, Line: "电信"}); err != nil {
		t.Fatal(err)
	}
	if err := p.DeleteRecord(bg, "example.com", Record{ID: "99"}); err != nil {
		t.Fatal(err)
	}
	reqs := api.requests()
	c := reqs[0].Form()
	if reqs[0].Path != "/Record.Create" || c.Get("sub_domain") != "@" || c.Get("record_line") != "默认" ||
		c.Get("ttl") != "600" || c.Get("value") != "1.2.3.4" {
		t.Fatalf("bad create %s %v", reqs[0].Path, c)
	}
	m := reqs[1].Form()
	if reqs[1].Path != "/Record.Modify" || m.Get("record_id") != "99" || m.Get("record_line") != "电信" ||
		m.Get("ttl") != "120" || m.Get("record_type") != "AAAA" {
		t.Fatalf("bad modify %s %v", reqs[1].Path, m)
	}
	d := reqs[2].Form()
	if reqs[2].Path != "/Record.Remove" || d.Get("record_id") != "99" || d.Get("domain") != "example.com" {
		t.Fatalf("bad remove %s %v", reqs[2].Path, d)
	}
}

func TestDNSPodError(t *testing.T) {
	p, _ := newTestDNSPod(t, func(c captured) (int, string) {
		return 200, `{"status":{"code":"-1","message":"登录失败"}}`
	})
	err := p.Test(bg)
	if err == nil || !strings.Contains(err.Error(), "登录失败") || !strings.Contains(err.Error(), "DNSPod") {
		t.Fatalf("want error, got %v", err)
	}
	// 非列表接口的 code 10 仍视为错误
	p2, _ := newTestDNSPod(t, func(c captured) (int, string) {
		return 200, `{"status":{"code":"10","message":"x"}}`
	})
	if err := p2.CreateRecord(bg, "example.com", Record{Name: "a", Type: "A", Value: "1.1.1.1"}); err == nil {
		t.Fatal("want error")
	}
}

func TestDNSPodListDomains(t *testing.T) {
	p, api := newTestDNSPod(t, func(c captured) (int, string) {
		return 200, `{"status":{"code":"1"},"info":{"domain_total":2},"domains":[{"id":1,"name":"a.com"},{"id":2,"name":"b.cn"}]}`
	})
	ds, err := p.ListDomains(bg)
	if err != nil || strings.Join(ds, ",") != "a.com,b.cn" {
		t.Fatalf("got %v %v", ds, err)
	}
	if api.requests()[0].Path != "/Domain.List" {
		t.Fatal("bad path")
	}
}
