// Package cfst 管理 CloudflareSpeedTest 二进制（下载/版本/IP 段文件）并以子进程运行测速。
package cfst

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
)

const (
	repo = "XIU2/CloudflareSpeedTest"
	// FallbackVersion 在无法访问 GitHub API 时使用。
	FallbackVersion = "v2.3.5"
)

// Manager 管理 cfst 安装目录。
type Manager struct {
	Dir    string // 安装目录，如 data/cfst
	Mirror func() string
	Log    *slog.Logger

	mu         sync.Mutex
	installing bool
}

// Status 为安装状态。
type Status struct {
	Installed  bool   `json:"installed"`
	Version    string `json:"version"`
	Path       string `json:"path"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	Asset      string `json:"asset"`
	Installing bool   `json:"installing"`
}

// Release 为 GitHub Release 摘要。
type Release struct {
	Tag            string    `json:"tag"`
	Name           string    `json:"name"`
	PublishedAt    time.Time `json:"publishedAt"`
	AssetAvailable bool      `json:"assetAvailable"`
}

func binName() string {
	if runtime.GOOS == "windows" {
		return "cfst.exe"
	}
	return "cfst"
}

// BinPath 返回二进制路径。
func (m *Manager) BinPath() string { return filepath.Join(m.Dir, binName()) }

// IPFile 返回 IP 段文件路径，kind 为 v4 或 v6。
func (m *Manager) IPFile(kind string) string {
	if kind == "v6" {
		return filepath.Join(m.Dir, "ipv6.txt")
	}
	return filepath.Join(m.Dir, "ip.txt")
}

func (m *Manager) defaultIPFile(kind string) string {
	return strings.TrimSuffix(m.IPFile(kind), ".txt") + ".default.txt"
}

// Installed 判断二进制是否存在。
func (m *Manager) Installed() bool {
	st, err := os.Stat(m.BinPath())
	return err == nil && !st.IsDir()
}

// Version 返回已安装版本。
func (m *Manager) Version() string {
	b, err := os.ReadFile(filepath.Join(m.Dir, "VERSION"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// Status 返回当前状态。
func (m *Manager) Status() Status {
	m.mu.Lock()
	installing := m.installing
	m.mu.Unlock()
	asset, _ := AssetName(runtime.GOOS, runtime.GOARCH, goarm())
	return Status{Installed: m.Installed(), Version: m.Version(), Path: m.BinPath(),
		OS: runtime.GOOS, Arch: runtime.GOARCH, Asset: asset, Installing: installing}
}

// goarm 读取编译时的 GOARM（仅 arm 架构有意义）。
func goarm() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "GOARM" {
				return strings.SplitN(s.Value, ",", 2)[0]
			}
		}
	}
	return "7"
}

// AssetName 返回对应平台的发布包文件名（与旧版 install.sh 规则一致）。
func AssetName(goos, goarch, arm string) (string, error) {
	var arch string
	switch goarch {
	case "amd64", "arm64", "386", "mips", "mipsle", "mips64", "mips64le":
		arch = goarch
	case "arm":
		if arm == "" {
			arm = "7"
		}
		arch = "armv" + arm
	default:
		return "", fmt.Errorf("不支持的 CPU 架构: %s", goarch)
	}
	switch goos {
	case "linux":
		return "cfst_linux_" + arch + ".tar.gz", nil
	case "windows", "darwin":
		return "cfst_" + goos + "_" + arch + ".zip", nil
	default:
		return "", fmt.Errorf("不支持的操作系统: %s", goos)
	}
}

// MirrorURL 按镜像规则改写 GitHub 下载地址：
// 含 {url} 时替换为完整地址（ghproxy 风格），否则替换 https://github.com 前缀。
func MirrorURL(mirror, rawURL string) string {
	mirror = strings.TrimSpace(mirror)
	if mirror == "" {
		return rawURL
	}
	if strings.Contains(mirror, "{url}") {
		return strings.ReplaceAll(mirror, "{url}", rawURL)
	}
	return strings.TrimRight(mirror, "/") + strings.TrimPrefix(rawURL, "https://github.com")
}

// Releases 查询 GitHub Releases。
func (m *Manager) Releases(ctx context.Context) ([]Release, error) {
	resp, err := httpx.Do(ctx, httpx.Request{URL: "https://api.github.com/repos/" + repo + "/releases?per_page=20",
		Header: map[string]string{"Accept": "application/vnd.github+json"}})
	if err != nil {
		return nil, fmt.Errorf("访问 GitHub API 失败: %w", err)
	}
	if !resp.OK() {
		return nil, fmt.Errorf("GitHub API 返回 HTTP %d: %s", resp.Status, httpx.Snippet(resp.Body))
	}
	var raw []struct {
		TagName     string    `json:"tag_name"`
		Name        string    `json:"name"`
		PublishedAt time.Time `json:"published_at"`
		Draft       bool      `json:"draft"`
		Prerelease  bool      `json:"prerelease"`
		Assets      []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(resp.Body, &raw); err != nil {
		return nil, err
	}
	asset, _ := AssetName(runtime.GOOS, runtime.GOARCH, goarm())
	var out []Release
	for _, r := range raw {
		if r.Draft {
			continue
		}
		ok := false
		for _, a := range r.Assets {
			if a.Name == asset {
				ok = true
			}
		}
		out = append(out, Release{Tag: r.TagName, Name: r.Name, PublishedAt: r.PublishedAt, AssetAvailable: ok})
	}
	return out, nil
}

// latestTag 获取最新版本号，失败时依次尝试 releases/latest 重定向与内置版本。
func (m *Manager) latestTag(ctx context.Context) string {
	resp, err := httpx.Do(ctx, httpx.Request{URL: "https://api.github.com/repos/" + repo + "/releases/latest", Retries: 1})
	if err == nil && resp.OK() {
		var r struct {
			TagName string `json:"tag_name"`
		}
		if json.Unmarshal(resp.Body, &r) == nil && r.TagName != "" {
			return r.TagName
		}
	}
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, "https://github.com/"+repo+"/releases/latest", nil)
	if r, err := client.Do(req); err == nil {
		r.Body.Close()
		if loc := r.Header.Get("Location"); strings.Contains(loc, "/tag/") {
			return loc[strings.LastIndex(loc, "/")+1:]
		}
	}
	m.Log.Warn("无法获取 cfst 最新版本，使用内置版本", "version", FallbackVersion)
	return FallbackVersion
}

// Install 下载并安装指定版本（"latest" 或空表示最新）。
func (m *Manager) Install(ctx context.Context, version string) (string, error) {
	m.mu.Lock()
	if m.installing {
		m.mu.Unlock()
		return "", errors.New("正在安装中，请稍候")
	}
	m.installing = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.installing = false
		m.mu.Unlock()
	}()

	if version == "" || version == "latest" {
		version = m.latestTag(ctx)
	}
	asset, err := AssetName(runtime.GOOS, runtime.GOARCH, goarm())
	if err != nil {
		return "", err
	}
	raw := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, version, asset)
	url := MirrorURL(m.Mirror(), raw)
	m.Log.Info("下载 cfst", "version", version, "url", url)

	dctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	resp, err := httpx.Do(dctx, httpx.Request{URL: url, Retries: 2})
	if err != nil {
		return "", fmt.Errorf("下载失败（国内网络建议在设置中配置 GitHub 镜像）: %w", err)
	}
	if !resp.OK() {
		return "", fmt.Errorf("下载失败: HTTP %d %s", resp.Status, url)
	}
	files, err := extract(asset, resp.Body)
	if err != nil {
		return "", fmt.Errorf("解压失败: %w", err)
	}
	if err := m.installFiles(files, version); err != nil {
		return "", err
	}
	m.Log.Info("cfst 安装完成", "version", version)
	return version, nil
}

// extract 从压缩包中取出所需文件（忽略目录层级）。
func extract(asset string, data []byte) (map[string][]byte, error) {
	want := map[string]string{"cfst": "bin", "cfst.exe": "bin", "CloudflareST": "bin", "CloudflareST.exe": "bin",
		"ip.txt": "ip.txt", "ipv6.txt": "ipv6.txt"}
	out := map[string][]byte{}
	take := func(name string, r io.Reader) error {
		key, ok := want[filepath.Base(name)]
		if !ok {
			return nil
		}
		b, err := io.ReadAll(io.LimitReader(r, 64<<20))
		if err != nil {
			return err
		}
		out[key] = b
		return nil
	}
	if strings.HasSuffix(asset, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			err = take(f.Name, rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
		}
	} else {
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		tr := tar.NewReader(gz)
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if h.Typeflag == tar.TypeReg {
				if err := take(h.Name, tr); err != nil {
					return nil, err
				}
			}
		}
	}
	if out["bin"] == nil {
		return nil, errors.New("压缩包中未找到 cfst 可执行文件")
	}
	return out, nil
}

func (m *Manager) installFiles(files map[string][]byte, version string) error {
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		return err
	}
	// 先写临时文件再重命名，避免运行中的测速读到半个文件
	tmp := m.BinPath() + ".new"
	if err := os.WriteFile(tmp, files["bin"], 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmp, m.BinPath()); err != nil {
		return fmt.Errorf("替换可执行文件失败（测速是否正在运行？）: %w", err)
	}
	for _, kind := range []string{"v4", "v6"} {
		name := "ip.txt"
		if kind == "v6" {
			name = "ipv6.txt"
		}
		data, ok := files[name]
		if !ok {
			continue
		}
		oldDefault, _ := os.ReadFile(m.defaultIPFile(kind))
		cur, err := os.ReadFile(m.IPFile(kind))
		// 仅在用户未修改过 IP 段文件时覆盖
		if err != nil || bytes.Equal(cur, oldDefault) {
			if err := os.WriteFile(m.IPFile(kind), data, 0o644); err != nil {
				return err
			}
		}
		if err := os.WriteFile(m.defaultIPFile(kind), data, 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(m.Dir, "VERSION"), []byte(version+"\n"), 0o644)
}

// InstallFromDir 从预置目录（bundled 镜像）复制安装。
func (m *Manager) InstallFromDir(dir string) error {
	files := map[string][]byte{}
	for key, name := range map[string]string{"bin": "cfst", "ip.txt": "ip.txt", "ipv6.txt": "ipv6.txt"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			if key == "bin" {
				return err
			}
			continue
		}
		files[key] = b
	}
	version := "bundled"
	if b, err := os.ReadFile(filepath.Join(dir, "VERSION")); err == nil {
		version = strings.TrimSpace(string(b))
	}
	return m.installFiles(files, version)
}

// ReadIPFile 读取 IP 段文件。
func (m *Manager) ReadIPFile(kind string) (string, error) {
	b, err := os.ReadFile(m.IPFile(kind))
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(b), err
}

// WriteIPFile 保存 IP 段文件。
func (m *Manager) WriteIPFile(kind, content string) error {
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		return err
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(m.IPFile(kind), []byte(content), 0o644)
}

// ResetIPFile 恢复为随版本分发的默认内容。
func (m *Manager) ResetIPFile(kind string) (string, error) {
	b, err := os.ReadFile(m.defaultIPFile(kind))
	if err != nil {
		return "", errors.New("没有可恢复的默认文件，请先安装 cfst")
	}
	return string(b), os.WriteFile(m.IPFile(kind), b, 0o644)
}
