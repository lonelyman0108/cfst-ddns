package cfst

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// 测试二进制在设置 FAKE_CFST 时扮演 cfst：1 为输出版本号后正常退出，fail 为报错退出，hang 为输出版本后卡住。
func TestMain(m *testing.M) {
	switch os.Getenv("FAKE_CFST") {
	case "fail":
		fmt.Println("boom: missing library")
		os.Exit(2)
	case "hang":
		fmt.Println("CloudflareSpeedTest v2.3.1")
		time.Sleep(time.Minute)
		os.Exit(0)
	case "1":
		fmt.Println("CloudflareSpeedTest v2.3.4")
		fmt.Println("检查版本更新中...")
		fmt.Println("*** 发现新版本 [v9.9.9]！请前往 [https://github.com/XIU2/CloudflareSpeedTest] 更新！ ***")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func elfHeader(machine elfMachine, class, data byte) []byte {
	b := make([]byte, 64)
	copy(b, "\x7fELF")
	b[4], b[5], b[6] = class, data, 1
	var order binary.ByteOrder = binary.LittleEndian
	if data == 2 {
		order = binary.BigEndian
	}
	order.PutUint16(b[16:], 2) // ET_EXEC
	order.PutUint16(b[18:], uint16(machine))
	order.PutUint32(b[20:], 1)
	if class == 1 {
		order.PutUint16(b[40:], 52)
		return b[:52]
	}
	order.PutUint16(b[52:], 64)
	return b
}

type elfMachine uint16

func peHeader(machine uint16) []byte {
	b := make([]byte, 0x80+24)
	copy(b, "MZ")
	binary.LittleEndian.PutUint32(b[0x3c:], 0x80)
	copy(b[0x80:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(b[0x84:], machine)
	return b
}

func machoHeader(cpu uint32) []byte {
	b := make([]byte, 32)
	binary.LittleEndian.PutUint32(b, 0xfeedfacf)
	binary.LittleEndian.PutUint32(b[4:], cpu)
	binary.LittleEndian.PutUint32(b[12:], 2) // MH_EXECUTE
	return b
}

func TestDetectPlatform(t *testing.T) {
	cases := []struct {
		name     string
		data     []byte
		os, arch string
	}{
		{"elf amd64", elfHeader(62, 2, 1), "linux", "amd64"},
		{"elf arm64", elfHeader(183, 2, 1), "linux", "arm64"},
		{"elf armv7", elfHeader(40, 1, 1), "linux", "arm"},
		{"elf 386", elfHeader(3, 1, 1), "linux", "386"},
		{"elf mipsle", elfHeader(8, 1, 1), "linux", "mipsle"},
		{"elf mips", elfHeader(8, 1, 2), "linux", "mips"},
		{"elf mips64le", elfHeader(8, 2, 1), "linux", "mips64le"},
		{"pe amd64", peHeader(0x8664), "windows", "amd64"},
		{"pe arm64", peHeader(0xaa64), "windows", "arm64"},
		{"pe 386", peHeader(0x14c), "windows", "386"},
		{"macho arm64", machoHeader(0x0100000c), "darwin", "arm64"},
		{"macho amd64", machoHeader(0x01000007), "darwin", "amd64"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goos, goarch, err := DetectPlatform(bytes.NewReader(c.data))
			if err != nil || goos != c.os || goarch != c.arch {
				t.Fatalf("got %s/%s %v, want %s/%s", goos, goarch, err, c.os, c.arch)
			}
		})
	}
	for _, bad := range [][]byte{nil, []byte("#!/bin/sh\necho hi\n"), []byte("PK\x03\x04garbage")} {
		var be *BadFileError
		if _, _, err := DetectPlatform(bytes.NewReader(bad)); !errors.As(err, &be) {
			t.Errorf("%q: 应返回 BadFileError, got %v", bad, err)
		}
	}
}

// 本机编译出的测试二进制应被识别为本机平台。
func TestDetectPlatformSelf(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	f, err := os.Open(exe)
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	goos, goarch, err := DetectPlatform(f)
	if err != nil || goos != runtime.GOOS || goarch != runtime.GOARCH {
		t.Fatalf("got %s/%s %v", goos, goarch, err)
	}
}

func TestCheckPlatform(t *testing.T) {
	cases := []struct {
		os, arch, hostOS, hostArch string
		ok                         bool
	}{
		{"linux", "amd64", "linux", "amd64", true},
		{"linux", "386", "linux", "amd64", true},
		{"linux", "arm", "linux", "arm", true},
		{"linux", "arm", "linux", "arm64", true},
		{"darwin", "amd64", "darwin", "arm64", true},
		{"linux", "arm64", "linux", "amd64", false},
		{"linux", "amd64", "darwin", "amd64", false},
		{"windows", "amd64", "linux", "amd64", false},
		{"linux", "mipsle", "linux", "mips", false},
	}
	for _, c := range cases {
		err := checkPlatform(c.os, c.arch, c.hostOS, c.hostArch)
		if (err == nil) != c.ok {
			t.Errorf("%+v: err = %v", c, err)
		}
	}
	err := checkPlatform("linux", "amd64", "darwin", "arm64")
	if err == nil || err.Error() != "文件为 linux/amd64，本机为 darwin/arm64" {
		t.Fatalf("message = %v", err)
	}
}

func TestParseVersion(t *testing.T) {
	cases := map[string]string{
		"CloudflareSpeedTest v2.3.4\n检查版本更新中...\n发现新版本 [v9.9.9]": "v2.3.4",
		"cfst_v2.2.5_linux_amd64.tar.gz": "v2.2.5",
		"cfst_linux_amd64.tar.gz":        "",
		"V2.0.3":                         "v2.0.3",
		"version 2.3":                    "",
	}
	for in, want := range cases {
		if got := ParseVersion(in); got != want {
			t.Errorf("ParseVersion(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"2.3.5": "v2.3.5", " v2.3.5 ": "v2.3.5", "a b": "", "": "", "../x": ""} {
		if got := normalizeVersion(in); got != want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	data := t.TempDir()
	return &Manager{Dir: filepath.Join(data, "cfst"), Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Mirror: func() string { return "" }}
}

func selfBinary(t *testing.T) []byte {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Skip(err)
	}
	t.Setenv("FAKE_CFST", "1")
	return b
}

func zipOf(t *testing.T, files map[string][]byte) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, b := range files {
		w, _ := zw.Create(name)
		w.Write(b)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestImport(t *testing.T) {
	bin := selfBinary(t)
	ctx := context.Background()

	// 裸二进制：版本来自 -v 输出（取第一个，不取更新提示中的新版本），缺失的 IP 段文件写入默认值
	m := newTestManager(t)
	res, err := m.Import(ctx, "cfst", bin, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Version != "v2.3.4" || res.OS != runtime.GOOS || res.Arch != runtime.GOARCH {
		t.Fatalf("res = %+v", res)
	}
	if !m.Installed() || m.Version() != "v2.3.4" {
		t.Fatalf("installed=%v version=%q", m.Installed(), m.Version())
	}
	if ip, _ := m.ReadIPFile("v4"); !strings.Contains(ip, "104.16.0.0/13") {
		t.Fatalf("默认 IP 段未写入: %q", ip)
	}

	// 压缩包：版本来自文件名，ip.txt 来自压缩包
	m = newTestManager(t)
	archive := zipOf(t, map[string][]byte{"cfst_darwin/cfst": bin, "cfst_darwin/ip.txt": []byte("1.1.1.0/24\n")})
	res, err = m.Import(ctx, "cfst_v2.3.5_darwin.zip", archive, "")
	if err != nil || res.Version != "v2.3.5" {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if ip, _ := m.ReadIPFile("v4"); ip != "1.1.1.0/24\n" {
		t.Fatalf("ip.txt = %q", ip)
	}

	// 显式版本优先
	res, err = m.Import(ctx, "cfst_v2.3.5.zip", archive, "2.0.0")
	if err != nil || res.Version != "v2.0.0" || m.Version() != "v2.0.0" {
		t.Fatalf("res = %+v, err = %v", res, err)
	}

	// 平台不匹配
	other := elfHeader(62, 2, 1)
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		other = elfHeader(183, 2, 1)
	}
	var be *BadFileError
	if _, err := m.Import(ctx, "cfst", other, ""); !errors.As(err, &be) || !strings.Contains(err.Error(), "本机为") {
		t.Fatalf("err = %v", err)
	}
	if m.Version() != "v2.0.0" {
		t.Fatal("失败的导入不应改动已安装版本")
	}

	// 安装中拒绝并发导入，也拒绝测速
	done, _ := m.begin()
	if _, err := m.Import(ctx, "cfst", bin, ""); !errors.Is(err, ErrInstalling) {
		t.Fatalf("err = %v", err)
	}
	if _, err := m.Run(ctx, RunOptions{IPType: "v4", WorkDir: t.TempDir()}, nil); err == nil || !strings.Contains(err.Error(), "正在安装") {
		t.Fatalf("Run err = %v", err)
	}
	done()

	// 有任务排队或运行时拒绝，并释放安装标记
	m.Busy = func() bool { return true }
	if _, err := m.Import(ctx, "cfst", bin, ""); !errors.Is(err, ErrTaskActive) {
		t.Fatalf("err = %v", err)
	}
	if m.Status().Installing {
		t.Fatal("安装标记未释放")
	}
}

func TestScanAndAdopt(t *testing.T) {
	bin := selfBinary(t)
	ctx := context.Background()
	m := newTestManager(t)
	exe := func(n string) string {
		if runtime.GOOS == "windows" {
			return n + ".exe"
		}
		return n
	}
	root := filepath.Dir(m.Dir)
	stray := filepath.Join(root, exe("CloudflareST"))
	if err := os.WriteFile(stray, bin, 0o755); err != nil {
		t.Fatal(err)
	}
	bundle := t.TempDir()
	m.BundleDir = bundle
	os.WriteFile(filepath.Join(bundle, "cfst"), elfHeader(8, 1, 2), 0o755) // linux/mips，本机不兼容
	os.WriteFile(filepath.Join(bundle, "VERSION"), []byte("v2.2.2\n"), 0o644)

	list := m.Scan(ctx)
	var ds, bd *Candidate
	for i, c := range list {
		switch c.Source {
		case "datadir":
			ds = &list[i]
		case "bundled":
			bd = &list[i]
		}
	}
	if ds == nil || !ds.Compatible || ds.Version != "v2.3.4" || !strings.HasSuffix(ds.Path, exe("CloudflareST")) {
		t.Fatalf("datadir candidate = %+v (list %+v)", ds, list)
	}
	if bd == nil || bd.Compatible || bd.Message == "" || bd.Version != "" {
		t.Fatalf("bundled candidate = %+v", bd)
	}

	if _, err := m.Adopt(ctx, filepath.Join(root, "nope"), ""); err == nil {
		t.Fatal("非候选路径应拒绝")
	}
	res, err := m.Adopt(ctx, ds.Path, "")
	if err != nil || res.Version != "v2.3.4" || m.Version() != "v2.3.4" {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	// 已登记且内容相同的文件不再列出
	for _, c := range m.Scan(ctx) {
		if c.Source == "datadir" {
			t.Fatalf("不应再列出: %+v", c)
		}
	}
}

func TestProbeMirrorsInvalid(t *testing.T) {
	res := ProbeMirrors(context.Background(), []string{"ftp://x"})
	if len(res) != 1 || res[0].OK || res[0].Error == "" || res[0].Label != "x" {
		t.Fatalf("res = %+v", res)
	}
	if MirrorLabel("") != "直连 GitHub" || MirrorLabel("https://mirror.example/{url}") != "mirror.example" {
		t.Fatal("label 错误")
	}
}

func TestProbe(t *testing.T) {
	selfBinary(t)
	exe, _ := os.Executable()
	old := probeTimeout
	probeTimeout = time.Second
	defer func() { probeTimeout = old }()
	dir := t.TempDir()

	// 非零退出且没有版本号：视为无法运行
	t.Setenv("FAKE_CFST", "fail")
	var be *BadFileError
	if _, err := probe(context.Background(), exe, dir); !errors.As(err, &be) ||
		!strings.Contains(err.Error(), "试运行失败") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
	// 输出版本后超时被结束：仍然成功
	t.Setenv("FAKE_CFST", "hang")
	if v, err := probe(context.Background(), exe, dir); err != nil || v != "v2.3.1" {
		t.Fatalf("v = %q, err = %v", v, err)
	}
	// 正常退出
	t.Setenv("FAKE_CFST", "1")
	if v, err := probe(context.Background(), exe, dir); err != nil || v != "v2.3.4" {
		t.Fatalf("v = %q, err = %v", v, err)
	}
}
