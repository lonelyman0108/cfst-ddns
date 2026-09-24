package engine

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/provider"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

func recs(values ...string) []provider.Record {
	var out []provider.Record
	for i, v := range values {
		out = append(out, provider.Record{ID: fmt.Sprint(i + 1), Value: v})
	}
	return out
}

func TestMakePlan(t *testing.T) {
	cases := []struct {
		name     string
		existing []provider.Record
		desired  []string
		keep     int
		updates  int
		creates  []string
		deletes  int
	}{
		{"新建", nil, []string{"1.1.1.1"}, 0, 0, []string{"1.1.1.1"}, 0},
		{"未变化", recs("1.1.1.1"), []string{"1.1.1.1"}, 1, 0, nil, 0},
		{"改写", recs("9.9.9.9"), []string{"1.1.1.1"}, 0, 1, nil, 0},
		{"部分命中", recs("1.1.1.1", "9.9.9.9"), []string{"2.2.2.2", "1.1.1.1"}, 1, 1, nil, 0},
		{"增加数量", recs("1.1.1.1"), []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}, 1, 0, []string{"2.2.2.2", "3.3.3.3"}, 0},
		{"减少数量", recs("1.1.1.1", "2.2.2.2", "3.3.3.3"), []string{"3.3.3.3"}, 1, 0, nil, 2},
		{"重复旧记录", recs("1.1.1.1", "1.1.1.1"), []string{"1.1.1.1"}, 1, 0, nil, 1},
		{"期望去重", nil, []string{"1.1.1.1", "1.1.1.1", ""}, 0, 0, []string{"1.1.1.1"}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := MakePlan(c.existing, c.desired)
			if len(p.Keep) != c.keep || len(p.Updates) != c.updates || len(p.Deletes) != c.deletes ||
				!reflect.DeepEqual(p.Creates, c.creates) {
				t.Fatalf("got keep=%d updates=%d creates=%v deletes=%d", len(p.Keep), len(p.Updates), p.Creates, len(p.Deletes))
			}
			if (c.updates+len(c.creates)+c.deletes > 0) != p.Changed() {
				t.Fatalf("Changed() = %v", p.Changed())
			}
		})
	}
}

func TestSameLine(t *testing.T) {
	if !sameLine("默认", "") || !sameLine("", "default") || !sameLine("default_view", "") {
		t.Fatal("默认线路应互相匹配")
	}
	if sameLine("电信", "") || !sameLine("电信", "电信") || sameLine("", "电信") {
		t.Fatal("非默认线路匹配错误")
	}
}

// fakeProvider 在内存中模拟 DNS 记录。
type fakeProvider struct {
	records map[string]provider.Record
	next    int
	failOn  string
}

func (f *fakeProvider) Test(context.Context) error { return nil }
func (f *fakeProvider) ListDomains(context.Context) ([]string, error) {
	return []string{"example.com"}, nil
}
func (f *fakeProvider) ListRecords(_ context.Context, _, rr, typ string) ([]provider.Record, error) {
	var out []provider.Record
	for _, r := range f.records {
		if r.Name == rr && r.Type == typ {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (f *fakeProvider) CreateRecord(_ context.Context, _ string, r provider.Record) error {
	if f.failOn == "create" {
		return fmt.Errorf("boom")
	}
	f.next++
	r.ID = fmt.Sprintf("r%02d", f.next)
	f.records[r.ID] = r
	return nil
}
func (f *fakeProvider) UpdateRecord(_ context.Context, _ string, r provider.Record) error {
	f.records[r.ID] = r
	return nil
}
func (f *fakeProvider) DeleteRecord(_ context.Context, _ string, r provider.Record) error {
	delete(f.records, r.ID)
	return nil
}

func (f *fakeProvider) values() []string {
	var v []string
	for _, r := range f.records {
		v = append(v, r.Value)
	}
	sort.Strings(v)
	return v
}

func TestSyncTarget(t *testing.T) {
	e := &Engine{}
	fp := &fakeProvider{records: map[string]provider.Record{
		"r90": {ID: "r90", Name: "cdn", Type: "A", Value: "9.9.9.9"},
		"r91": {ID: "r91", Name: "cdn", Type: "A", Value: "8.8.8.8", Line: "电信"}, // 其他线路不应被改动
		"r92": {ID: "r92", Name: "www", Type: "A", Value: "7.7.7.7"},
	}}
	tg := store.Target{Domain: "example.com", RR: "cdn", TTL: 600}

	ch, err := e.syncTarget(context.Background(), fp, tg, "A", []string{"1.1.1.1", "2.2.2.2"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := fp.values(); !reflect.DeepEqual(got, []string{"1.1.1.1", "2.2.2.2", "7.7.7.7", "8.8.8.8"}) {
		t.Fatalf("records = %v", got)
	}
	if len(ch) != 2 || ch[0].Action != "update" || ch[1].Action != "create" {
		t.Fatalf("changes = %+v", ch)
	}

	// 再次同步相同 IP：全部跳过
	ch, _ = e.syncTarget(context.Background(), fp, tg, "A", []string{"2.2.2.2", "1.1.1.1"}, false)
	for _, c := range ch {
		if c.Action != "skip" {
			t.Fatalf("expected skip, got %+v", c)
		}
	}

	// 数量减少：删除多余
	ch, _ = e.syncTarget(context.Background(), fp, tg, "A", []string{"2.2.2.2"}, false)
	if got := fp.values(); !reflect.DeepEqual(got, []string{"2.2.2.2", "7.7.7.7", "8.8.8.8"}) {
		t.Fatalf("records = %v, changes = %+v", got, ch)
	}

	// 失败时返回错误并记录 error 变更
	fp.failOn = "create"
	ch, err = e.syncTarget(context.Background(), fp, store.Target{Domain: "example.com", RR: "new"}, "A", []string{"3.3.3.3"}, false)
	if err == nil || ch[0].Action != "error" {
		t.Fatalf("expected error, got %v %+v", err, ch)
	}
}

func TestBuildMessage(t *testing.T) {
	run := &store.Run{Status: store.StatusSuccess, Message: "DNS 记录已更新", DurationMs: 61000,
		Results: []store.SpeedResult{{IPType: "v4", Rank: 1, IP: "1.1.1.1", Latency: 50, Speed: 12.3, Colo: "HKG"}},
		Changes: []store.DNSChange{{FQDN: "cdn.example.com", Type: "A", Action: "update", OldValue: "9.9.9.9", NewValue: "1.1.1.1"}}}
	msg := BuildMessage("CFST DDNS", &store.Task{Name: "主任务"}, run)
	if msg.Title != "CFST DDNS 成功 - 主任务" || !msg.Success {
		t.Fatalf("title = %q", msg.Title)
	}
	for _, want := range []string{"最优 IPv4: 1.1.1.1", "HKG", "[更新] cdn.example.com A 9.9.9.9 → 1.1.1.1", "耗时: 1m1s"} {
		if !contains(msg.Content, want) {
			t.Fatalf("content missing %q:\n%s", want, msg.Content)
		}
	}
}

func TestShouldNotify(t *testing.T) {
	n := &store.Notifier{Enabled: true, OnSuccess: true, OnFailure: true, OnlyOnChange: true}
	if shouldNotify(n, &store.Run{Status: store.StatusSuccess}) {
		t.Fatal("未变化时不应通知")
	}
	if !shouldNotify(n, &store.Run{Status: store.StatusSuccess, Changed: true}) {
		t.Fatal("变化时应通知")
	}
	if !shouldNotify(n, &store.Run{Status: store.StatusFailed}) {
		t.Fatal("失败时应通知")
	}
	n.Enabled = false
	if shouldNotify(n, &store.Run{Status: store.StatusFailed}) {
		t.Fatal("禁用时不应通知")
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
