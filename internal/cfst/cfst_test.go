package cfst

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

func TestAssetName(t *testing.T) {
	cases := map[[3]string]string{
		{"linux", "amd64", ""}:   "cfst_linux_amd64.tar.gz",
		{"linux", "arm", "7"}:    "cfst_linux_armv7.tar.gz",
		{"linux", "arm", "6"}:    "cfst_linux_armv6.tar.gz",
		{"linux", "mipsle", ""}:  "cfst_linux_mipsle.tar.gz",
		{"windows", "amd64", ""}: "cfst_windows_amd64.zip",
		{"darwin", "arm64", ""}:  "cfst_darwin_arm64.zip",
	}
	for in, want := range cases {
		got, err := AssetName(in[0], in[1], in[2])
		if err != nil || got != want {
			t.Errorf("%v: got %q %v, want %q", in, got, err, want)
		}
	}
	if _, err := AssetName("linux", "riscv64", ""); err == nil {
		t.Error("riscv64 应不支持")
	}
}

func TestMirrorURL(t *testing.T) {
	raw := "https://github.com/XIU2/CloudflareSpeedTest/releases/download/v2.3.5/cfst_linux_amd64.tar.gz"
	cases := map[string]string{
		"":                         raw,
		"https://mirror.example/":  "https://mirror.example/XIU2/CloudflareSpeedTest/releases/download/v2.3.5/cfst_linux_amd64.tar.gz",
		"https://gh.example/{url}": "https://gh.example/" + raw,
	}
	for mirror, want := range cases {
		if got := MirrorURL(mirror, raw); got != want {
			t.Errorf("MirrorURL(%q) = %q, want %q", mirror, got, want)
		}
	}
}

func TestSplitArgs(t *testing.T) {
	got, err := SplitArgs(`-url "https://a b/c" -cfcolo 'HKG,NRT'  -debug`)
	want := []string{"-url", "https://a b/c", "-cfcolo", "HKG,NRT", "-debug"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := SplitArgs(`-url "abc`); err == nil {
		t.Fatal("未闭合引号应报错")
	}
}

func TestBuildArgs(t *testing.T) {
	c := store.SpeedTestConfig{Threads: 200, PingTimes: 4, DownloadCount: 5, DownloadTime: 10, Port: 443,
		Httping: true, HttpingCode: 200, CFColo: "hkg, nrt", MaxLatency: 250, MaxLossRate: 0.2, MinSpeed: 5,
		ExtraArgs: "-debug"}
	got, err := BuildArgs(c, "ip.txt", "out.csv")
	if err != nil {
		t.Fatal(err)
	}
	s := strings.Join(got, " ")
	want := "-debug -dn 5 -httping -httping-code 200 -cfcolo HKG,NRT -tl 250 -tlr 0.2 -sl 5 -f ip.txt -o out.csv -p 0"
	if s != want {
		t.Fatalf("got  %s\nwant %s", s, want)
	}
	// 默认值不输出，丢包率 1 不输出
	got, _ = BuildArgs(store.SpeedTestConfig{MaxLossRate: 1, MaxLatency: 9999}, "a", "b")
	if strings.Join(got, " ") != "-f a -o b -p 0" {
		t.Fatalf("got %v", got)
	}
	// 经代理测速强制 HTTPing
	got, _ = BuildArgs(store.SpeedTestConfig{MaxLossRate: 1, MaxLatency: 9999, NetworkMode: store.NetworkProxy}, "a", "b")
	if strings.Join(got, " ") != "-httping -f a -o b -p 0" {
		t.Fatalf("proxy mode got %v", got)
	}
}

func TestBuildArgsForIPs(t *testing.T) {
	c := store.SpeedTestConfig{Threads: 200, PingTimes: 4, Port: 443, MaxLatency: 9999, MaxLossRate: 1}
	got, err := BuildArgsForIPs(c, []string{" 1.1.1.1 ", "2606:4700::1", "1.1.1.1"}, "out.csv")
	if err != nil {
		t.Fatal(err)
	}
	want := "-ip 1.1.1.1,2606:4700::1 -o out.csv -p 0"
	if strings.Join(got, " ") != want {
		t.Fatalf("got %q, want %q", strings.Join(got, " "), want)
	}
	if strings.Contains(strings.Join(got, " "), "-f ") {
		t.Fatal("精确 IP 模式不应传入 -f")
	}

	c = store.SpeedTestConfig{Httping: true, HttpingCode: 200, URL: "https://example.com/url", MaxLatency: 9999, MaxLossRate: 1}
	got, err = BuildArgsForIPs(c, []string{"203.0.113.10"}, "out.csv")
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(got, " ")
	for _, want := range []string{"-httping", "-httping-code 200", "-url https://example.com/url", "-ip 203.0.113.10", "-o out.csv", "-p 0"} {
		if !strings.Contains(args, want) {
			t.Fatalf("args %q missing %q", args, want)
		}
	}
}

func TestExactIPsCanonicalAndSafeArguments(t *testing.T) {
	args, err := BuildArgsForIPs(store.SpeedTestConfig{MaxLossRate: 1}, []string{"2606:4700:0000:0000:0000:0000:0000:0001", "2606:4700::1", "::ffff:1.1.1.1", "1.1.1.1"}, "out")
	if err != nil || !strings.Contains(strings.Join(args, " "), "-ip 2606:4700::1,1.1.1.1 ") {
		t.Fatalf("args=%v err=%v", args, err)
	}
	for _, extra := range []string{"-ip 9.9.9.9", "-ip=9.9.9.9", "--f=bad", "-f bad", "-o bad", "-p 10", "positional", "-debug positional", "-- -f bad"} {
		if _, err := BuildArgsForIPs(store.SpeedTestConfig{ExtraArgs: extra}, []string{"1.1.1.1"}, "out"); err == nil {
			t.Errorf("unsafe extra args accepted: %q", extra)
		}
	}
}

func TestRunExactIPsEmptyNeverUsesDefaultFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture")
	}
	dir := t.TempDir()
	m := &Manager{Dir: dir}
	if err := os.WriteFile(m.BinPath(), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(m.IPFile("v4"), []byte("1.0.0.0/8"), 0644)
	_, err := m.Run(context.Background(), RunOptions{IPType: "v4", IPs: []string{}, WorkDir: filepath.Join(dir, "work")}, &captured{})
	if err == nil || !strings.Contains(err.Error(), "精确 IP 列表为空") {
		t.Fatalf("empty exact list: %v", err)
	}
}

func TestRunExactIPsUsesNoRangeFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture")
	}
	dir := t.TempDir()
	m := &Manager{Dir: dir}
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > args.txt\nwhile [ \"$#\" -gt 0 ]; do\n if [ \"$1\" = -o ]; then shift; out=\"$1\"; fi\n shift\ndone\nprintf 'IP,sent,received,loss,latency,speed\\n2606:4700::1,4,4,0,80,7\\n' > \"$out\"\n"
	if err := os.WriteFile(m.BinPath(), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	result, err := m.Run(context.Background(), RunOptions{IPType: "v6", IPs: []string{"2606:4700::1"}, Config: store.SpeedTestConfig{NetworkMode: store.NetworkProxy, MaxLossRate: 1, DisableDownload: true}, WorkDir: dir, Tag: "exact"}, &captured{})
	if err != nil || len(result) != 1 || result[0].IP != "2606:4700::1" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	args, err := os.ReadFile(filepath.Join(dir, "args.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"-ip\n2606:4700::1", "-httping", "-dd"} {
		if !strings.Contains(string(args), want) {
			t.Fatalf("args=%s", args)
		}
	}
	if strings.Contains(string(args), "-f\n") {
		t.Fatalf("default range file used: %s", args)
	}
}

func TestParseCSVRejectsMalformedMetrics(t *testing.T) {
	for _, csv := range []string{"", "IP,sent,received,loss,latency,speed\n1.1.1.1,4,4,0,NaN,2\n", "IP,sent,received,loss,latency,speed\n1.1.1.1,4,4,0,broken,2\n", "IP,sent,received,loss,latency,speed\n1.1.1.1,4\n"} {
		if _, err := ParseCSV([]byte(csv), "v4"); err == nil {
			t.Errorf("malformed CSV accepted: %q", csv)
		}
	}
}

func TestWithoutProxyEnv(t *testing.T) {
	got := withoutProxyEnv([]string{"PATH=/bin", "http_proxy=x", "HTTPS_PROXY=y", "all_proxy=z", "NO_PROXY=a", "HOME=/h"})
	if strings.Join(got, " ") != "PATH=/bin HOME=/h" {
		t.Fatalf("got %v", got)
	}
}

func TestParseCSV(t *testing.T) {
	data := "\xef\xbb\xbfIP 地址,已发送,已接收,丢包率,平均延迟,下载速度(MB/s),地区码\n" +
		"104.16.0.1,4,4,0.00,52.31,18.20,HKG\n" +
		"104.16.0.2,4,3,0.25,60.00,0.00,N/A\n"
	got, err := ParseCSV([]byte(data), "v4")
	if err != nil || len(got) != 2 {
		t.Fatalf("got %v %v", got, err)
	}
	if got[0] != (store.SpeedResult{IPType: "v4", Rank: 1, IP: "104.16.0.1", Sent: 4, Received: 4, Latency: 52.31, Speed: 18.2, Colo: "HKG"}) {
		t.Fatalf("row0 = %+v", got[0])
	}
	if got[1].Rank != 2 || got[1].LossRate != 0.25 {
		t.Fatalf("row1 = %+v", got[1])
	}
	// 旧版 6 列格式
	old := "IP 地址,已发送,已接收,丢包率,平均延迟,下载速度 (MB/s)\n1.0.0.1,4,4,0.00,10.5,3.2\n"
	got, _ = ParseCSV([]byte(old), "v4")
	if len(got) != 1 || got[0].Colo != "" || got[0].Speed != 3.2 {
		t.Fatalf("old = %+v", got)
	}
}

type captured struct{ lines, progress []string }

func (c *captured) Line(s string)     { c.lines = append(c.lines, s) }
func (c *captured) Progress(s string) { c.progress = append(c.progress, s) }

func TestPump(t *testing.T) {
	var c captured
	pump(strings.NewReader("hello\n\x1b[32m1 / 3\x1b[0m\r2 / 3\r3 / 3\ndone"), &c)
	if !reflect.DeepEqual(c.lines, []string{"hello", "3 / 3", "done"}) {
		t.Fatalf("lines = %q", c.lines)
	}
	if len(c.progress) != 1 || c.progress[0] != "1 / 3" { // 其余进度被限流
		t.Fatalf("progress = %q", c.progress)
	}
}

// 非终端模式：pb/v3 不输出 \r，进度条直接首尾相接，结束后才出现换行。
func TestPumpConcatenatedBars(t *testing.T) {
	pr, pw := io.Pipe()
	var c captured
	done := make(chan struct{})
	go func() { pump(pr, &c); close(done) }()
	pw.Write([]byte("# XIU2/CloudflareSpeedTest v2.3.5\n\n开始延迟测速\n"))
	pw.Write([]byte("0 / 64 [____] 可用: 0 "))
	time.Sleep(350 * time.Millisecond)
	pw.Write([]byte("10 / 64 [-->__] 可用: 3 20 / 64 [---->_] 可用: 7 "))
	time.Sleep(350 * time.Millisecond)
	pw.Write([]byte("64 / 64 [------] 可用: 30 \n完成\n"))
	pw.Close()
	<-done
	if !reflect.DeepEqual(c.lines, []string{"# XIU2/CloudflareSpeedTest v2.3.5", "开始延迟测速", "64 / 64 [------] 可用: 30", "完成"}) {
		t.Fatalf("lines = %q", c.lines)
	}
	if !reflect.DeepEqual(c.progress, []string{"0 / 64 [____] 可用: 0", "20 / 64 [---->_] 可用: 7"}) {
		t.Fatalf("progress = %q", c.progress)
	}
}

func TestSplitBarLine(t *testing.T) {
	cases := map[string][]string{
		"5955 / 5955 [------] 可用: 5142  开始下载测速（下限：5.00 MB/s, 数量：10, 队列：168）": {"5955 / 5955 [------] 可用: 5142", "开始下载测速（下限：5.00 MB/s, 数量：10, 队列：168）"},
		"64 / 64 [------] 可用: 30":   {"64 / 64 [------] 可用: 30"},
		"10 / 10 [------]        3": {"10 / 10 [------]        3"},
		"开始延迟测速（模式：TCP, 端口：443）":    {"开始延迟测速（模式：TCP, 端口：443）"},
	}
	for in, want := range cases {
		if got := splitBarLine(in); !reflect.DeepEqual(got, want) {
			t.Errorf("splitBarLine(%q) = %q, want %q", in, got, want)
		}
	}
}
