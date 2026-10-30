package update

import (
	"context"
	"errors"
	"testing"
)

func TestChannel(t *testing.T) {
	cases := map[string]string{
		"v2.2.0":            ChannelRelease,
		"2.2.0":             ChannelRelease,
		"v2.2.0-nightly.57": ChannelNightly,
		"dev":               ChannelDev,
		"dev-abc1234":       ChannelDev,
		"v2.2.0-rc.1":       ChannelDev,
	}
	for v, want := range cases {
		if got := Channel(v); got != want {
			t.Errorf("Channel(%q) = %q, want %q", v, got, want)
		}
	}
}

func TestNewerRelease(t *testing.T) {
	cases := []struct {
		latest, cur string
		want        bool
	}{
		{"v2.3.0", "v2.2.0", true},
		{"v2.10.0", "v2.9.9", true},
		{"v3.0.0", "v2.99.99", true},
		{"v2.2.0", "v2.2.0", false},
		{"v2.1.9", "v2.2.0", false},
		{"nightly", "v2.2.0", false},
	}
	for _, c := range cases {
		if got := newerRelease(c.latest, c.cur); got != c.want {
			t.Errorf("newerRelease(%q, %q) = %v", c.latest, c.cur, got)
		}
	}
}

func fakeFetch(r *Release, err error, calls *int, paths *[]string) func(context.Context, string) (*Release, error) {
	return func(_ context.Context, p string) (*Release, error) {
		*calls++
		*paths = append(*paths, p)
		return r, err
	}
}

func TestCheckRelease(t *testing.T) {
	var calls int
	var paths []string
	c := &Checker{Version: "v2.2.0", Fetch: fakeFetch(&Release{TagName: "v2.3.0", HTMLURL: "u"}, nil, &calls, &paths)}
	info := c.Check(context.Background(), false)
	if !info.Available || info.Latest != "v2.3.0" || info.Channel != ChannelRelease || paths[0] != "releases/latest" {
		t.Fatalf("unexpected %+v %v", info, paths)
	}
	c.Check(context.Background(), false)
	if calls != 1 {
		t.Fatalf("缓存未生效，请求了 %d 次", calls)
	}
	c.Check(context.Background(), true)
	if calls != 2 {
		t.Fatalf("force 未绕过缓存，请求了 %d 次", calls)
	}
}

func TestCheckNightly(t *testing.T) {
	var calls int
	var paths []string
	r := &Release{TagName: "nightly", Name: "Nightly #60 (abc1234)", Body: "- 版本：`v2.2.0-nightly.60`\n"}
	c := &Checker{Version: "v2.2.0-nightly.57", Fetch: fakeFetch(r, nil, &calls, &paths)}
	info := c.Check(context.Background(), false)
	if !info.Available || info.Latest != "v2.2.0-nightly.60" || paths[0] != "releases/tags/nightly" {
		t.Fatalf("unexpected %+v %v", info, paths)
	}
	c = &Checker{Version: "v2.2.0-nightly.60", Fetch: fakeFetch(r, nil, &calls, &paths)}
	if c.Check(context.Background(), false).Available {
		t.Fatal("同一构建不应提示更新")
	}
}

func TestCheckDevAndError(t *testing.T) {
	var calls int
	var paths []string
	c := &Checker{Version: "dev", Fetch: fakeFetch(nil, nil, &calls, &paths)}
	if info := c.Check(context.Background(), true); info.Available || calls != 0 {
		t.Fatalf("dev 不应检查: %+v", info)
	}
	c = &Checker{Version: "v2.2.0", Fetch: fakeFetch(nil, errors.New("boom"), &calls, &paths)}
	if info := c.Check(context.Background(), false); info.Err == nil || info.Available {
		t.Fatalf("应返回错误: %+v", info)
	}
}
