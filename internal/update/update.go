// Package update 检查 cfst-ddns 自身的新版本。
//
// 渠道由当前版本号决定：
//   - release（vX.Y.Z）：比较 GitHub releases/latest 的版本号
//   - nightly（vX.Y.Z-nightly.N）：比较 nightly 预发布标题中的构建号 N
//   - dev（其他）：不检查
package update

import (
	"context"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/i18n"
)

const (
	Repo    = "lonelyman0108/cfst-ddns"
	RepoURL = "https://github.com/" + Repo

	ChannelRelease = "release"
	ChannelNightly = "nightly"
	ChannelDev     = "dev"

	cacheTTL = 6 * time.Hour
	errorTTL = 30 * time.Minute // 失败结果缓存较短，网络恢复后能尽快重试
)

var (
	releaseRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)
	nightlyRe = regexp.MustCompile(`-nightly\.(\d+)$`)
	// nightly 预发布标题形如 "Nightly #57 (abc1234)"，由 CI 生成
	titleRe = regexp.MustCompile(`#(\d+)`)
	// nightly 预发布正文中的版本行，形如 "- 版本：`v2.2.0-nightly.57`"
	bodyVersionRe = regexp.MustCompile("`(v[^`\\s]+-nightly\\.\\d+)`")
)

// Channel 根据版本号判断构建渠道。
func Channel(version string) string {
	switch {
	case releaseRe.MatchString(version):
		return ChannelRelease
	case nightlyRe.MatchString(version):
		return ChannelNightly
	default:
		return ChannelDev
	}
}

// Info 为一次检查的结果。
type Info struct {
	Channel     string     `json:"channel"`
	Current     string     `json:"current"`
	Latest      string     `json:"latest,omitempty"`
	Available   bool       `json:"available"`
	URL         string     `json:"url,omitempty"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
	CheckedAt   *time.Time `json:"checkedAt,omitempty"`
	Error       string     `json:"error,omitempty"` // 由接口按请求语言填写
	Err         error      `json:"-"`
	Notes       string     `json:"-"` // Release 正文，仅用于通知
}

// Checker 查询并缓存最新版本。
type Checker struct {
	Version string
	Log     *slog.Logger
	// Fetch 获取 Release，测试时替换；为 nil 时请求 GitHub API
	Fetch func(ctx context.Context, path string) (*Release, error)

	mu     sync.Mutex
	cached *Info
	expiry time.Time
}

// Release 为 GitHub Release 中用到的字段。
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
}

// Cached 返回缓存的结果，没有缓存时返回只含当前版本的结果。
func (c *Checker) Cached() Info {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cached != nil {
		return *c.cached
	}
	return Info{Channel: Channel(c.Version), Current: c.Version}
}

// Check 返回最新版本信息；缓存未过期且 force 为 false 时直接使用缓存。
func (c *Checker) Check(ctx context.Context, force bool) Info {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.cached != nil && time.Now().Before(c.expiry) {
		return *c.cached
	}
	info := c.check(ctx)
	now := time.Now()
	info.CheckedAt = &now
	c.cached = &info
	if info.Err != nil {
		c.expiry = now.Add(errorTTL)
	} else {
		c.expiry = now.Add(cacheTTL)
	}
	return info
}

func (c *Checker) check(ctx context.Context) Info {
	info := Info{Channel: Channel(c.Version), Current: c.Version}
	var path string
	switch info.Channel {
	case ChannelRelease:
		path = "releases/latest"
	case ChannelNightly:
		path = "releases/tags/nightly"
	default:
		return info
	}
	r, err := c.fetch(ctx, path)
	if err != nil {
		info.Err = err
		if c.Log != nil {
			c.Log.Debug("检查新版本失败", "err", err)
		}
		return info
	}
	info.URL, info.Notes = r.HTMLURL, r.Body
	if !r.PublishedAt.IsZero() {
		t := r.PublishedAt
		info.PublishedAt = &t
	}
	if info.Channel == ChannelRelease {
		info.Latest = r.TagName
		info.Available = newerRelease(r.TagName, c.Version)
		return info
	}
	latest := buildNumber(titleRe, r.Name)
	cur := buildNumber(nightlyRe, c.Version)
	if m := bodyVersionRe.FindStringSubmatch(r.Body); m != nil {
		info.Latest = m[1]
	} else {
		info.Latest = r.Name
	}
	info.Available = latest > 0 && latest > cur
	return info
}

func (c *Checker) fetch(ctx context.Context, path string) (*Release, error) {
	if c.Fetch != nil {
		return c.Fetch(ctx, path)
	}
	resp, err := httpx.Do(ctx, httpx.Request{URL: "https://api.github.com/repos/" + Repo + "/" + path, Retries: 1,
		Header: map[string]string{"Accept": "application/vnd.github+json"}})
	if err != nil {
		return nil, i18n.Errorf("访问 GitHub API 失败: %w", err)
	}
	if !resp.OK() {
		return nil, i18n.Errorf("GitHub API 返回 HTTP %d: %s", resp.Status, httpx.Snippet(resp.Body))
	}
	var r Release
	if err := resp.Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// newerRelease 判断 latest 是否比 current 新，两者都须为 vX.Y.Z。
func newerRelease(latest, current string) bool {
	a, b := parseRelease(latest), parseRelease(current)
	if a == nil || b == nil {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func parseRelease(v string) []int {
	m := releaseRe.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return nil
	}
	out := make([]int, 3)
	for i := range out {
		out[i], _ = strconv.Atoi(m[i+1])
	}
	return out
}

func buildNumber(re *regexp.Regexp, s string) int {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// Run 在后台定期检查新版本，直到 ctx 结束。enabled 返回 false 时跳过本轮；
// 发现新版本时调用 onAvailable。dev 构建不检查。
func (c *Checker) Run(ctx context.Context, delay, interval time.Duration, enabled func() bool, onAvailable func(Info)) {
	if Channel(c.Version) == ChannelDev {
		return
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if enabled() {
			if info := c.Check(ctx, true); info.Available {
				onAvailable(info)
			}
		}
		t.Reset(interval)
	}
}
