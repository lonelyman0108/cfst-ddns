package cfst

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
)

// Mirror 为 GitHub 镜像预设。
type Mirror struct {
	Mirror string `json:"mirror"`
	Label  string `json:"label"`
}

// MirrorPresets 为内置的常用 GitHub 加速代理（{url} 为完整的 GitHub 地址）。
// 公共代理的可用性会变化，界面上应先测速再选用。
var MirrorPresets = []Mirror{
	{"", "直连 GitHub"},
	{"https://ghfast.top/{url}", "ghfast.top"},
	{"https://gh-proxy.com/{url}", "gh-proxy.com"},
	{"https://ghproxy.net/{url}", "ghproxy.net"},
}

// MirrorProbe 为一次镜像测速结果。
type MirrorProbe struct {
	Mirror    string `json:"mirror"`
	Label     string `json:"label"`
	OK        bool   `json:"ok"`
	LatencyMs int64  `json:"latencyMs"`
	Error     string `json:"error,omitempty"`
}

// probeAsset 为测速使用的发布资源（固定版本，确保存在）。
var probeAsset = fmt.Sprintf("https://github.com/%s/releases/download/%s/cfst_linux_amd64.tar.gz", repo, FallbackVersion)

// MirrorLabel 返回镜像的显示名称：预设使用预设名，否则取主机名。
func MirrorLabel(mirror string) string {
	for _, p := range MirrorPresets {
		if p.Mirror == mirror {
			return p.Label
		}
	}
	if u, err := url.Parse(strings.ReplaceAll(mirror, "{url}", "")); err == nil && u.Host != "" {
		return u.Host
	}
	return mirror
}

// ProbeMirrors 并发探测各镜像下载发布资源的首包耗时，按可用、耗时排序。
func ProbeMirrors(ctx context.Context, mirrors []string) []MirrorProbe {
	out := make([]MirrorProbe, len(mirrors))
	var wg sync.WaitGroup
	for i, m := range mirrors {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = probeMirror(ctx, m)
		}()
	}
	wg.Wait()
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].OK != out[j].OK {
			return out[i].OK
		}
		return out[i].OK && out[i].LatencyMs < out[j].LatencyMs
	})
	return out
}

func probeMirror(ctx context.Context, mirror string) MirrorProbe {
	p := MirrorProbe{Mirror: mirror, Label: MirrorLabel(mirror)}
	if mirror != "" && !strings.HasPrefix(mirror, "http://") && !strings.HasPrefix(mirror, "https://") {
		p.Error = "地址需以 http:// 或 https:// 开头"
		return p
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, MirrorURL(mirror, probeAsset), nil)
	if err != nil {
		p.Error = err.Error()
		return p
	}
	// 只取前 1KB，不支持 Range 的代理读到后即断开
	req.Header.Set("Range", "bytes=0-1023")
	req.Header.Set("User-Agent", httpx.UserAgent)
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			p.Error = "超时（8 秒）"
		} else {
			p.Error = err.Error()
		}
		return p
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		p.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return p
	}
	// 有的代理出错时返回 200 + HTML 页面
	if ct := resp.Header.Get("Content-Type"); strings.HasPrefix(ct, "text/html") {
		p.Error = "返回的不是文件（" + ct + "）"
		return p
	}
	head := make([]byte, 2)
	if _, err := io.ReadFull(resp.Body, head); err != nil {
		p.Error = "读取响应失败: " + err.Error()
		return p
	}
	if head[0] != 0x1f || head[1] != 0x8b {
		p.Error = "返回内容不是有效的发布文件"
		return p
	}
	p.OK, p.LatencyMs = true, time.Since(start).Milliseconds()
	return p
}
