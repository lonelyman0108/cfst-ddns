// Package netcheck 在测速前检查网络环境，识别测速流量被透明代理 / TUN 拦截的情况。
//
// Clash TUN、Surge 增强模式、OpenClash 等会在本地直接完成 TCP 握手，
// cfst 的 TCPing 延迟因此全部接近 0，排序失去意义，写入 DNS 的 IP 等于随机挑选。
package netcheck

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

type traceTarget struct{ IP, Host string }

var errNoTrace = errors.New("无法获取 Cloudflare trace")

// 以下地址可在测试中替换。
var (
	// canaryAddr 为文档保留地址（RFC 5737 / RFC 3849），公网不可路由；能握手成功只可能是被本地拦截
	canaryAddr = map[string]string{"v4": "192.0.2.1:443", "v6": "[2001:db8::1]:443"}
	// traceTargets 为 Cloudflare 自有站点及其边缘 IP，依次尝试直到取得 trace
	traceTargets = map[string][]traceTarget{
		"v4": {{"104.16.132.229", "cloudflare.com"}, {"162.159.140.220", "speed.cloudflare.com"}, {"172.64.155.89", "speed.cloudflare.com"}},
		"v6": {{"2606:4700::6810:84e5", "cloudflare.com"}, {"2606:4700::6810:85e5", "cloudflare.com"}},
	}
	// echoURLs 为国内回显出口 IP 的服务（规则分流时通常直连），依次尝试
	echoURLs = map[string][]string{
		"v4": {"https://ipv4.ddnspod.com", "https://ip.3322.net"},
		"v6": {"https://ipv6.ddnspod.com", "https://v6.ip.zxinc.org/getip"},
	}
	traceScheme   = "https"
	tracePort     = "443"
	canaryTimeout = 1500 * time.Millisecond
	dialTimeout   = 3 * time.Second
	httpTimeout   = 5 * time.Second
)

func network(ipType string) string {
	if ipType == "v6" {
		return "tcp6"
	}
	return "tcp4"
}

// Probe 对每种 IP 类型做自检；canary 为 false 时跳过保留地址探测（经代理模式不需要判定）。
// 各项探测并发进行，单个类型最多耗时约 httpTimeout。
func Probe(ctx context.Context, ipTypes []string, canary bool) []store.NetworkProbe {
	out := make([]store.NetworkProbe, len(ipTypes))
	var wg sync.WaitGroup
	for i, t := range ipTypes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = probeOne(ctx, t, canary)
		}()
	}
	wg.Wait()
	return out
}

func probeOne(ctx context.Context, ipType string, canary bool) store.NetworkProbe {
	p := store.NetworkProbe{IPType: ipType}
	var wg sync.WaitGroup
	run := func(f func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f()
		}()
	}
	if canary {
		run(func() { p.Intercepted = canDial(ctx, ipType, canaryAddr[ipType], canaryTimeout) })
	}
	var traceErr error
	run(func() {
		targets := traceTargets[ipType]
		if len(targets) > 0 {
			p.ConnectMs = connectMs(ctx, ipType, targets[0].IP)
		}
		var kv map[string]string
		if kv, traceErr = trace(ctx, ipType, targets); traceErr == nil {
			p.EgressIP, p.Loc, p.Colo = kv["ip"], kv["loc"], kv["colo"]
		}
	})
	run(func() { p.LocalIP = echo(ctx, ipType) })
	wg.Wait()
	if traceErr != nil && !p.Intercepted {
		p.Error = traceErr.Error()
	}
	return p
}

func canDial(ctx context.Context, ipType, addr string, timeout time.Duration) bool {
	d := net.Dialer{Timeout: timeout}
	c, err := d.DialContext(ctx, network(ipType), addr)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// connectMs 返回 3 次 TCP 握手中的最小耗时（毫秒），全部失败为 0。
func connectMs(ctx context.Context, ipType, ip string) float64 {
	best := time.Duration(0)
	for range 3 {
		d := net.Dialer{Timeout: dialTimeout}
		start := time.Now()
		c, err := d.DialContext(ctx, network(ipType), net.JoinHostPort(ip, tracePort))
		if err != nil {
			continue
		}
		el := time.Since(start)
		c.Close()
		if best == 0 || el < best {
			best = el
		}
	}
	return float64(best.Microseconds()) / 1000
}

// directClient 返回不读取代理环境变量、只走指定协议栈的客户端；pin 非空时固定连到该 IP。
func directClient(ipType, pin, serverName string) *http.Client {
	tr := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			if pin != "" {
				_, port, _ := net.SplitHostPort(addr)
				addr = net.JoinHostPort(pin, port)
			}
			return (&net.Dialer{Timeout: dialTimeout}).DialContext(ctx, network(ipType), addr)
		},
		TLSClientConfig:   &tls.Config{ServerName: serverName},
		DisableKeepAlives: true,
	}
	return &http.Client{Transport: tr, Timeout: httpTimeout}
}

func get(ctx context.Context, c *http.Client, url, host string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if host != "" {
		req.Host = host
	}
	req.Header.Set("User-Agent", httpx.UserAgent)
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return string(b), nil
}

// trace 依次请求 /cdn-cgi/trace，返回第一个含 ip 字段的结果。
func trace(ctx context.Context, ipType string, targets []traceTarget) (map[string]string, error) {
	var lastErr error = errNoTrace
	for _, t := range targets {
		body, err := get(ctx, directClient(ipType, t.IP, t.Host), traceScheme+"://"+t.Host+"/cdn-cgi/trace", t.Host)
		if err != nil {
			lastErr = err
			continue
		}
		if kv := ParseTrace(body); kv["ip"] != "" {
			return kv, nil
		}
	}
	return nil, lastErr
}

// ParseTrace 解析 /cdn-cgi/trace 的 key=value 文本。
func ParseTrace(body string) map[string]string {
	kv := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(body))
	for sc.Scan() {
		if k, v, ok := strings.Cut(sc.Text(), "="); ok {
			kv[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return kv
}

// echo 返回回显服务看到的出口 IP，全部失败为空。
func echo(ctx context.Context, ipType string) string {
	for _, u := range echoURLs[ipType] {
		body, err := get(ctx, directClient(ipType, "", ""), u, "")
		if err != nil {
			continue
		}
		if ip := net.ParseIP(strings.TrimSpace(body)); ip != nil {
			return ip.String()
		}
	}
	return ""
}

// 测速结果延迟异常的判定阈值：中位数低于 anomalyMedianMs，且 P10–P90 跨度小于 anomalySpreadMs。
const (
	anomalyMinResults = 5
	anomalyMedianMs   = 5.0
	anomalySpreadMs   = 2.0
)

// LatencyAnomalous 判断 TCPing 结果是否呈现「全部极低且几乎一致」的本地握手特征。
// 正常直连到 Cloudflare 的延迟会随 IP 段明显分散；只适用于 TCPing，HTTPing 的延迟包含代理链路。
func LatencyAnomalous(res []store.SpeedResult) bool {
	if len(res) < anomalyMinResults {
		return false
	}
	lat := make([]float64, len(res))
	for i, r := range res {
		lat[i] = r.Latency
	}
	slices.Sort(lat)
	at := func(q float64) float64 { return lat[int(q*float64(len(lat)-1))] }
	return at(0.5) < anomalyMedianMs && at(0.9)-at(0.1) < anomalySpreadMs
}
