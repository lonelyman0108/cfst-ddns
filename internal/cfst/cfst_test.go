package cfst

import (
	"io"
	"reflect"
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
