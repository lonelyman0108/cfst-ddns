package provider

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

// 官方文档「签名方法 v3」示例（https://cloud.tencent.com/document/api/1427/56189）。
// 文档中的密钥已打码，因此只校验与密钥无关的中间串。
func TestTC3OfficialVector(t *testing.T) {
	u := string(rune(92)) + "u" // 文档载荷中的中文为字面量 Unicode 转义（反斜杠 + u + 4 位十六进制）
	payload := []byte(`{"Limit": 1, "Filters": [{"Values": ["` + u + `672a` + u + `547d` + u + `540d"], "Name": "instance-name"}]}`)
	if got := httpx.SHA256Hex(payload); got != "35e9c5b0e3ae67532d3c9f17ead6c90222632e5b1ff7f6e89887f1398934f064" {
		t.Fatalf("payload hash %s", got)
	}
	cr := tc3CanonicalRequest("cvm.tencentcloudapi.com", "DescribeInstances", payload)
	wantCR := "POST\n/\n\ncontent-type:application/json; charset=utf-8\nhost:cvm.tencentcloudapi.com\n" +
		"x-tc-action:describeinstances\n\ncontent-type;host;x-tc-action\n" +
		"35e9c5b0e3ae67532d3c9f17ead6c90222632e5b1ff7f6e89887f1398934f064"
	if cr != wantCR {
		t.Fatalf("canonical request\n%q\n%q", cr, wantCR)
	}
	sts := tc3StringToSign(1551113065, "cvm", cr)
	wantSTS := "TC3-HMAC-SHA256\n1551113065\n2019-02-25/cvm/tc3_request\n" +
		"7019a55be8395899b900fb5564e4200d984910f34794a27cb3fb7d10ff6a1e84"
	if sts != wantSTS {
		t.Fatalf("string to sign\n%q\n%q", sts, wantSTS)
	}
	// 独立实现的派生密钥链
	mac := func(k []byte, s string) []byte { m := hmac.New(sha256.New, k); m.Write([]byte(s)); return m.Sum(nil) }
	k := mac(mac(mac([]byte("TC3sk"), "2019-02-25"), "cvm"), "tc3_request")
	if got, want := tc3Sign("sk", "cvm", 1551113065, sts), hex.EncodeToString(mac(k, sts)); got != want {
		t.Fatalf("sign %s != %s", got, want)
	}
	auth := tc3Authorization("AKIDx", "sk", "cvm", "cvm.tencentcloudapi.com", "DescribeInstances", 1551113065, payload)
	if !strings.HasPrefix(auth, "TC3-HMAC-SHA256 Credential=AKIDx/2019-02-25/cvm/tc3_request, SignedHeaders=content-type;host;x-tc-action, Signature=") {
		t.Fatalf("auth %s", auth)
	}
}

func newTestTencent(t *testing.T, handler func(c captured) (int, string)) (Provider, *fakeAPI) {
	fixClock(t, 1551113065)
	api := newFakeAPI(t, handler)
	setEndpoint(t, &tencentAPI, api.URL)
	return mustNew(t, "tencentcloud", schema.Config{"secretId": "AKIDtest", "secretKey": "sk"}), api
}

func checkTC3(t *testing.T, c captured, action string) map[string]any {
	t.Helper()
	if c.Method != "POST" || c.Path != "/" {
		t.Fatalf("bad request %s %s", c.Method, c.Path)
	}
	if c.Header.Get("X-TC-Action") != action || c.Header.Get("X-TC-Version") != "2021-03-23" ||
		c.Header.Get("X-TC-Timestamp") != "1551113065" || c.Header.Get("Content-Type") != tcContentType {
		t.Fatalf("bad headers %v", c.Header)
	}
	want := tc3Authorization("AKIDtest", "sk", "dnspod", c.Host, action, 1551113065, c.Body)
	if c.Header.Get("Authorization") != want {
		t.Fatalf("auth\n%s\n%s", c.Header.Get("Authorization"), want)
	}
	var body map[string]any
	if err := json.Unmarshal(c.Body, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestTencentListRecords(t *testing.T) {
	p, api := newTestTencent(t, func(c captured) (int, string) {
		return 200, `{"Response":{"RecordCountInfo":{"ListCount":2},"RecordList":[
			{"RecordId":556507778,"Name":"www","Type":"A","Value":"1.1.1.1","Line":"默认","TTL":600},
			{"RecordId":556507779,"Name":"www2","Type":"A","Value":"2.2.2.2","Line":"默认","TTL":600}],"RequestId":"x"}}`
	})
	recs, err := p.ListRecords(bg, "example.com", "www", "A")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].ID != "556507778" || recs[0].FQDN != "www.example.com" || recs[0].Line != "默认" {
		t.Fatalf("bad records %+v", recs)
	}
	body := checkTC3(t, api.requests()[0], "DescribeRecordList")
	if body["Domain"] != "example.com" || body["Subdomain"] != "www" || body["RecordType"] != "A" || body["Offset"] != 0.0 {
		t.Fatalf("bad body %v", body)
	}
}

func TestTencentNoDataOfRecord(t *testing.T) {
	p, _ := newTestTencent(t, func(c captured) (int, string) {
		return 200, `{"Response":{"Error":{"Code":"ResourceNotFound.NoDataOfRecord","Message":"记录列表为空。"},"RequestId":"x"}}`
	})
	recs, err := p.ListRecords(bg, "example.com", "www", "A")
	if err != nil || len(recs) != 0 {
		t.Fatalf("want empty, got %v %v", recs, err)
	}
}

func TestTencentWrite(t *testing.T) {
	p, api := newTestTencent(t, func(c captured) (int, string) {
		return 200, `{"Response":{"RecordId":1,"RequestId":"x"}}`
	})
	if err := p.CreateRecord(bg, "example.com", Record{Name: "@", Type: "A", Value: "1.2.3.4"}); err != nil {
		t.Fatal(err)
	}
	if err := p.UpdateRecord(bg, "example.com", Record{ID: "556507778", Name: "www", Type: "A", Value: "5.6.7.8", TTL: 300}); err != nil {
		t.Fatal(err)
	}
	if err := p.DeleteRecord(bg, "example.com", Record{ID: "556507778"}); err != nil {
		t.Fatal(err)
	}
	reqs := api.requests()
	b := checkTC3(t, reqs[0], "CreateRecord")
	if b["SubDomain"] != "@" || b["RecordLine"] != "默认" || b["TTL"] != 600.0 || b["Value"] != "1.2.3.4" {
		t.Fatalf("bad create %v", b)
	}
	b = checkTC3(t, reqs[1], "ModifyRecord")
	if b["RecordId"] != 556507778.0 || b["TTL"] != 300.0 {
		t.Fatalf("bad modify %v", b)
	}
	b = checkTC3(t, reqs[2], "DeleteRecord")
	if b["RecordId"] != 556507778.0 || b["Domain"] != "example.com" {
		t.Fatalf("bad delete %v", b)
	}
	if err := p.DeleteRecord(bg, "example.com", Record{ID: "abc"}); err == nil {
		t.Fatal("want invalid id error")
	}
}

func TestTencentError(t *testing.T) {
	p, _ := newTestTencent(t, func(c captured) (int, string) {
		return 200, `{"Response":{"Error":{"Code":"AuthFailure.SignatureFailure","Message":"签名错误"},"RequestId":"x"}}`
	})
	err := p.Test(bg)
	if err == nil || !strings.Contains(err.Error(), "AuthFailure.SignatureFailure") || !strings.Contains(err.Error(), "腾讯云") {
		t.Fatalf("got %v", err)
	}
}
