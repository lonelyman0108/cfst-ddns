// Package verify 提供写入 DNS 前及健康检查中的通用协议级验证（TLS / HTTP / WebSocket）。
package verify

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/i18n"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

// Result 表示单个候选 IP 的协议级验证结果。
type Result struct {
	IP       string
	OK       bool
	Stage    string // tcp | tls | http | websocket | ok
	Status   int
	Duration time.Duration
	Error    string
}

// ParseExpectStatus 解析期望的状态码列表；留空时按模式返回默认值。
func ParseExpectStatus(mode, raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		switch mode {
		case store.VerifyModeHTTP:
			return []int{http.StatusOK}, nil
		case store.VerifyModeWebSocket:
			return []int{http.StatusSwitchingProtocols}, nil
		default:
			return nil, nil
		}
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '，' || r == ' ' || r == '\t'
	})
	if len(parts) == 0 {
		return nil, i18n.Errorf("期望状态码无效: %s", raw)
	}
	seen := make(map[int]bool, len(parts))
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		code, err := strconv.Atoi(p)
		if err != nil || code < 100 || code > 599 {
			return nil, i18n.Errorf("期望状态码无效: %s", p)
		}
		if !seen[code] {
			seen[code] = true
			out = append(out, code)
		}
	}
	return out, nil
}

// NormalizePath 规范化请求路径（空串转为 "/"，缺前导 "/" 时自动补齐）。
func NormalizePath(raw string) string {
	p := strings.TrimSpace(raw)
	p = strings.NewReplacer("\r", "", "\n", "").Replace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

// Probe 对指定 IP 按配置的协议模式执行直连探测（不读取系统代理环境变量）。
func Probe(ctx context.Context, ip string, cfg store.VerifyConfig, defaultPort int) Result {
	start := time.Now()
	cfg = cfg.WithDefaults()
	mode := cfg.Mode
	port := cfg.Port
	if port <= 0 {
		port = defaultPort
	}
	if port <= 0 {
		port = 443
	}
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 5
	}
	host := strings.NewReplacer("\r", "", "\n", "").Replace(strings.TrimSpace(cfg.Host))

	pctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	var dialer net.Dialer
	rawConn, err := dialer.DialContext(pctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return Result{IP: ip, Stage: "tcp", Duration: time.Since(start), Error: err.Error()}
	}
	stop := context.AfterFunc(pctx, func() { _ = rawConn.Close() })
	defer stop()
	defer rawConn.Close()
	if dl, ok := pctx.Deadline(); ok {
		_ = rawConn.SetDeadline(dl)
	}

	var rwConn net.Conn = rawConn
	useTLS := mode == store.VerifyModeTLS || port != 80
	if useTLS {
		// 强制 ALPN http/1.1，防止协商 h2 后无法进行 HTTP/1.1 WebSocket Upgrade
		tlsConn := tls.Client(rawConn, &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: cfg.Insecure,
			NextProtos:         []string{"http/1.1"},
		})
		if err := tlsConn.HandshakeContext(pctx); err != nil {
			return Result{IP: ip, Stage: "tls", Duration: time.Since(start), Error: err.Error()}
		}
		if mode == store.VerifyModeTLS {
			return Result{IP: ip, OK: true, Stage: "ok", Duration: time.Since(start)}
		}
		rwConn = tlsConn
	}

	expected, err := ParseExpectStatus(mode, cfg.ExpectStatus)
	if err != nil {
		return Result{IP: ip, Stage: mode, Duration: time.Since(start), Error: err.Error()}
	}

	path := NormalizePath(cfg.Path)
	var req strings.Builder
	req.WriteString("GET " + path + " HTTP/1.1\r\n")
	req.WriteString("Host: " + host + "\r\n")
	req.WriteString("User-Agent: " + httpx.UserAgent + "\r\n")
	if mode == store.VerifyModeWebSocket {
		var keyBytes [16]byte
		if _, err := rand.Read(keyBytes[:]); err != nil {
			return Result{IP: ip, Stage: mode, Duration: time.Since(start), Error: err.Error()}
		}
		req.WriteString("Upgrade: websocket\r\n")
		req.WriteString("Connection: Upgrade\r\n")
		req.WriteString("Sec-WebSocket-Key: " + base64.StdEncoding.EncodeToString(keyBytes[:]) + "\r\n")
		req.WriteString("Sec-WebSocket-Version: 13\r\n\r\n")
	} else {
		req.WriteString("Connection: close\r\n\r\n")
	}

	if _, err := io.WriteString(rwConn, req.String()); err != nil {
		return Result{IP: ip, Stage: mode, Duration: time.Since(start), Error: err.Error()}
	}
	resp, err := http.ReadResponse(bufio.NewReader(rwConn), &http.Request{Method: http.MethodGet})
	if err != nil {
		return Result{IP: ip, Stage: mode, Duration: time.Since(start), Error: err.Error()}
	}
	_ = resp.Body.Close()

	if !containsStatus(expected, resp.StatusCode) {
		return Result{IP: ip, Stage: mode, Status: resp.StatusCode, Duration: time.Since(start),
			Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	if mode == store.VerifyModeWebSocket && resp.StatusCode == http.StatusSwitchingProtocols {
		if !strings.Contains(strings.ToLower(resp.Header.Get("Upgrade")), "websocket") {
			return Result{IP: ip, Stage: mode, Status: resp.StatusCode, Duration: time.Since(start),
				Error: "missing Upgrade: websocket header"}
		}
	}
	return Result{IP: ip, OK: true, Stage: "ok", Status: resp.StatusCode, Duration: time.Since(start)}
}

func containsStatus(list []int, target int) bool {
	for _, code := range list {
		if code == target {
			return true
		}
	}
	return false
}

// Filter 按原始测速排名分批并发验证候选 IP，凑够 need 个即停止并保持原始排序。
func Filter(
	ctx context.Context,
	candidates []store.SpeedResult,
	cfg store.VerifyConfig,
	defaultPort, need int,
	probeFn func(context.Context, string, store.VerifyConfig, int) Result,
	onResult func(Result),
) []store.SpeedResult {
	cfg = cfg.WithDefaults()
	if probeFn == nil {
		probeFn = Probe
	}
	if need < 1 {
		need = 1
	}
	limit := cfg.MaxCandidates
	if limit < 1 {
		limit = 15
	}
	conc := cfg.Concurrency
	if conc < 1 {
		conc = 5
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	var passed []store.SpeedResult
	for start := 0; start < len(candidates) && len(passed) < need && ctx.Err() == nil; start += conc {
		end := min(len(candidates), start+conc)
		batch := candidates[start:end]
		results := make([]Result, len(batch))
		var wg sync.WaitGroup
		for i, cand := range batch {
			wg.Add(1)
			go func(idx int, ip string) {
				defer wg.Done()
				results[idx] = probeFn(ctx, ip, cfg, defaultPort)
			}(i, cand.IP)
		}
		wg.Wait()

		for i, res := range results {
			if onResult != nil {
				onResult(res)
			}
			if res.OK {
				passed = append(passed, batch[i])
			}
		}
	}
	return passed
}
