package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

// 阿里云官方文档「RPC 签名机制」示例（ECS DescribeRegions）。
func TestAliSignOfficialVector(t *testing.T) {
	p := map[string]string{
		"Timestamp": "2016-02-23T12:46:24Z", "Format": "XML", "AccessKeyId": "testid", "Action": "DescribeRegions",
		"SignatureMethod": "HMAC-SHA1", "SignatureNonce": "3ee8c1b8-83d3-44af-a94f-4e0ad82fd6cf",
		"Version": "2014-05-26", "SignatureVersion": "1.0",
	}
	want := "GET&%2F&AccessKeyId%3Dtestid%26Action%3DDescribeRegions%26Format%3DXML%26SignatureMethod%3DHMAC-SHA1" +
		"%26SignatureNonce%3D3ee8c1b8-83d3-44af-a94f-4e0ad82fd6cf%26SignatureVersion%3D1.0" +
		"%26Timestamp%3D2016-02-23T12%253A46%253A24Z%26Version%3D2014-05-26"
	if got := aliStringToSign("GET", p); got != want {
		t.Fatalf("string to sign\n got %s\nwant %s", got, want)
	}
	if got := aliSign("GET", p, "testsecret"); got != "OLeaidS1JvxuMvnyHOwuJ+uX5qY=" {
		t.Fatalf("signature %s", got)
	}
}

func newTestAli(t *testing.T, handler func(c captured) (int, string)) (Provider, *fakeAPI) {
	fixClock(t, 1458837714) // 2016-03-24T16:41:54Z
	api := newFakeAPI(t, handler)
	setEndpoint(t, &aliAPI, api.URL)
	return mustNew(t, "alidns", schema.Config{"accessKeyId": "testid", "accessKeySecret": "testsecret"}), api
}

// checkAli 校验公共参数与签名，返回业务参数。
func checkAli(t *testing.T, c captured, action string) map[string]string {
	t.Helper()
	if c.Method != "GET" || c.Path != "/" {
		t.Fatalf("bad request %s %s", c.Method, c.Path)
	}
	p := map[string]string{}
	for k, v := range c.Query {
		p[k] = v[0]
	}
	sig := p["Signature"]
	delete(p, "Signature")
	if p["Action"] != action || p["Version"] != "2015-01-09" || p["Format"] != "JSON" || p["AccessKeyId"] != "testid" ||
		p["SignatureMethod"] != "HMAC-SHA1" || p["SignatureVersion"] != "1.0" || p["SignatureNonce"] != "fixed-nonce" ||
		p["Timestamp"] != "2016-03-24T16:41:54Z" {
		t.Fatalf("bad common params %v", p)
	}
	if sig == "" || sig != aliSign("GET", p, "testsecret") {
		t.Fatalf("bad signature %q", sig)
	}
	return p
}

func TestAliListRecords(t *testing.T) {
	p, api := newTestAli(t, func(c captured) (int, string) {
		return 200, `{"TotalCount":2,"DomainRecords":{"Record":[
			{"RecordId":"9999985","RR":"www","Type":"A","Value":"1.1.1.1","TTL":600,"Line":"default"},
			{"RecordId":"9999986","RR":"www.api","Type":"A","Value":"2.2.2.2","TTL":600,"Line":"default"}]}}`
	})
	recs, err := p.ListRecords(bg, "example.com", "www", "A")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].ID != "9999985" || recs[0].FQDN != "www.example.com" || recs[0].Line != "default" {
		t.Fatalf("bad records %+v", recs)
	}
	q := checkAli(t, api.requests()[0], "DescribeDomainRecords")
	if q["DomainName"] != "example.com" || q["RRKeyWord"] != "www" || q["Type"] != "A" || q["PageNumber"] != "1" {
		t.Fatalf("bad params %v", q)
	}
}

func TestAliListDomainsPaging(t *testing.T) {
	p, api := newTestAli(t, func(c captured) (int, string) {
		n := aliDomainPage
		if c.Query.Get("PageNumber") == "2" {
			n = 1
		}
		var items []string
		for i := 0; i < n; i++ {
			items = append(items, fmt.Sprintf(`{"DomainName":"d%s-%d.com"}`, c.Query.Get("PageNumber"), i))
		}
		return 200, `{"Domains":{"Domain":[` + strings.Join(items, ",") + `]}}`
	})
	ds, err := p.ListDomains(bg)
	if err != nil || len(ds) != aliDomainPage+1 || ds[aliDomainPage] != "d2-0.com" {
		t.Fatalf("got %d domains, err %v", len(ds), err)
	}
	if len(api.requests()) != 2 {
		t.Fatalf("want 2 requests, got %d", len(api.requests()))
	}
}

func TestAliWrite(t *testing.T) {
	p, api := newTestAli(t, func(c captured) (int, string) {
		if c.Query.Get("Action") == "UpdateDomainRecord" {
			return 400, `{"Code":"DomainRecordDuplicate","Message":"The DNS record already exists."}`
		}
		return 200, `{"RecordId":"1","RequestId":"x"}`
	})
	if err := p.CreateRecord(bg, "example.com", Record{Name: "@", Type: "A", Value: "1.2.3.4", TTL: 60}); err != nil {
		t.Fatal(err)
	}
	if err := p.UpdateRecord(bg, "example.com", Record{ID: "123", Name: "www", Type: "A", Value: "1.2.3.4"}); err != nil {
		t.Fatalf("duplicate should be ignored: %v", err)
	}
	if err := p.DeleteRecord(bg, "example.com", Record{ID: "123"}); err != nil {
		t.Fatal(err)
	}
	reqs := api.requests()
	q := checkAli(t, reqs[0], "AddDomainRecord")
	if q["DomainName"] != "example.com" || q["RR"] != "@" || q["TTL"] != "600" || q["Line"] != "default" || q["Value"] != "1.2.3.4" {
		t.Fatalf("bad add %v", q)
	}
	q = checkAli(t, reqs[1], "UpdateDomainRecord")
	if q["RecordId"] != "123" || q["RR"] != "www" {
		t.Fatalf("bad update %v", q)
	}
	q = checkAli(t, reqs[2], "DeleteDomainRecord")
	if q["RecordId"] != "123" {
		t.Fatalf("bad delete %v", q)
	}
}

func TestAliError(t *testing.T) {
	p, _ := newTestAli(t, func(c captured) (int, string) {
		return 404, `{"Code":"InvalidAccessKeyId.NotFound","Message":"Specified access key is not found."}`
	})
	err := p.Test(bg)
	if err == nil || !strings.Contains(err.Error(), "阿里云") || !strings.Contains(err.Error(), "InvalidAccessKeyId.NotFound") {
		t.Fatalf("got %v", err)
	}
}
