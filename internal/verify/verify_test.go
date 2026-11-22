package verify

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

func selfSignedTLSConfig(t *testing.T, dnsName string, nextProtos []string) *tls.Config {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: dnsName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{dnsName},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	return &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: nextProtos}
}

func startTLSServer(t *testing.T, tlsCfg *tls.Config, handler func(net.Conn, tls.ConnectionState)) (string, int) {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", tlsCfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				tc := c.(*tls.Conn)
				if err := tc.Handshake(); err != nil {
					return
				}
				handler(tc, tc.ConnectionState())
			}(c)
		}
	}()
	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return host, port
}

func TestParseExpectStatusAndNormalizePath(t *testing.T) {
	if got, err := ParseExpectStatus(store.VerifyModeTLS, ""); err != nil || got != nil {
		t.Fatalf("tls default=%v %v", got, err)
	}
	if got, err := ParseExpectStatus(store.VerifyModeHTTP, ""); err != nil || len(got) != 1 || got[0] != 200 {
		t.Fatalf("http default=%v %v", got, err)
	}
	if got, err := ParseExpectStatus(store.VerifyModeWebSocket, ""); err != nil || len(got) != 1 || got[0] != 101 {
		t.Fatalf("ws default=%v %v", got, err)
	}
	if got, err := ParseExpectStatus(store.VerifyModeWebSocket, "101, 400，101"); err != nil || len(got) != 2 || got[0] != 101 || got[1] != 400 {
		t.Fatalf("custom=%v %v", got, err)
	}
	for _, bad := range []string{"abc", "99", "600", ","} {
		if _, err := ParseExpectStatus(store.VerifyModeHTTP, bad); err == nil {
			t.Errorf("ParseExpectStatus(%q) expected error", bad)
		}
	}
	if got := NormalizePath(""); got != "/" {
		t.Fatalf("NormalizePath empty=%q", got)
	}
	if got := NormalizePath("ws"); got != "/ws" {
		t.Fatalf("NormalizePath relative=%q", got)
	}
}

func TestProbeTLSHTTPAndWebSocket(t *testing.T) {
	// 同时通告 h2 与 http/1.1，验证 Probe 会强制 ALPN http/1.1
	var mu sync.Mutex
	var gotSNI, gotALPN, gotUpgrade, gotHost, gotPath string
	tlsCfg := selfSignedTLSConfig(t, "cdn.example.com", []string{"h2", "http/1.1"})
	tlsCfg.GetConfigForClient = func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		mu.Lock()
		gotSNI = hello.ServerName
		mu.Unlock()
		return nil, nil
	}
	ip, port := startTLSServer(t, tlsCfg, func(c net.Conn, st tls.ConnectionState) {
		req, err := http.ReadRequest(bufio.NewReader(c))
		if err != nil {
			return
		}
		mu.Lock()
		gotALPN = st.NegotiatedProtocol
		gotHost = req.Host
		gotPath = req.URL.Path
		gotUpgrade = req.Header.Get("Upgrade")
		mu.Unlock()
		switch req.URL.Path {
		case "/ws":
			_, _ = io.WriteString(c, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		case "/ws-bad-header":
			_, _ = io.WriteString(c, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\n\r\n")
		case "/blocked":
			_, _ = io.WriteString(c, "HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\n\r\n")
		case "/ok":
			_, _ = io.WriteString(c, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")
		default:
			_, _ = io.WriteString(c, "HTTP/1.1 400 Bad Request\r\nContent-Length: 0\r\n\r\n")
		}
	})

	// 1. 证书校验：不跳过 Insecure 时自签名证书应在 tls 阶段失败；跳过则通过
	res := Probe(context.Background(), ip, store.VerifyConfig{Enabled: true, Mode: store.VerifyModeTLS, Host: "cdn.example.com", Port: port}, 443)
	if res.OK || res.Stage != "tls" {
		t.Fatalf("expected tls verify failure without Insecure, got %+v", res)
	}
	res = Probe(context.Background(), ip, store.VerifyConfig{Enabled: true, Mode: store.VerifyModeTLS, Host: "cdn.example.com", Port: port, Insecure: true}, 443)
	mu.Lock()
	sni := gotSNI
	mu.Unlock()
	if !res.OK || res.Stage != "ok" || sni != "cdn.example.com" {
		t.Fatalf("tls pass=%+v sni=%q", res, sni)
	}

	// 2. WebSocket 101 成功，且确认 ALPN 协商为 http/1.1
	res = Probe(context.Background(), ip, store.VerifyConfig{Enabled: true, Mode: store.VerifyModeWebSocket, Host: "cdn.example.com", Port: port, Path: "ws", Insecure: true}, 443)
	mu.Lock()
	alpn, upgrade, host, path := gotALPN, gotUpgrade, gotHost, gotPath
	mu.Unlock()
	if !res.OK || res.Status != 101 || alpn != "http/1.1" || upgrade != "websocket" || host != "cdn.example.com" || path != "/ws" {
		t.Fatalf("ws pass=%+v alpn=%q upgrade=%q host=%q path=%q", res, alpn, upgrade, host, path)
	}

	// 3. WebSocket 缺少 Upgrade 头或返回 403 失败
	res = Probe(context.Background(), ip, store.VerifyConfig{Enabled: true, Mode: store.VerifyModeWebSocket, Host: "cdn.example.com", Port: port, Path: "/ws-bad-header", Insecure: true}, 443)
	if res.OK || res.Stage != "websocket" {
		t.Fatalf("ws missing upgrade should fail: %+v", res)
	}
	res = Probe(context.Background(), ip, store.VerifyConfig{Enabled: true, Mode: store.VerifyModeWebSocket, Host: "cdn.example.com", Port: port, Path: "/blocked", Insecure: true}, 443)
	if res.OK || res.Stage != "websocket" || res.Status != 403 {
		t.Fatalf("ws 403=%+v", res)
	}

	// 4. 自定义状态码 101,400 允许 400 通过
	res = Probe(context.Background(), ip, store.VerifyConfig{Enabled: true, Mode: store.VerifyModeWebSocket, Host: "cdn.example.com", Port: port, Path: "/other", ExpectStatus: "101,400", Insecure: true}, 443)
	if !res.OK || res.Status != 400 {
		t.Fatalf("ws custom expectStatus=%+v", res)
	}

	// 5. HTTP 模式 200 通过，403 失败
	res = Probe(context.Background(), ip, store.VerifyConfig{Enabled: true, Mode: store.VerifyModeHTTP, Host: "cdn.example.com", Port: port, Path: "/ok", Insecure: true}, 443)
	if !res.OK || res.Status != 200 {
		t.Fatalf("http pass=%+v", res)
	}
	res = Probe(context.Background(), ip, store.VerifyConfig{Enabled: true, Mode: store.VerifyModeHTTP, Host: "cdn.example.com", Port: port, Path: "/blocked", Insecure: true}, 443)
	if res.OK || res.Status != 403 || res.Stage != "http" {
		t.Fatalf("http fail=%+v", res)
	}

	// 6. TCP 连接失败标记为 tcp 阶段
	closedLn, _ := net.Listen("tcp", "127.0.0.1:0")
	_, closedPortStr, _ := net.SplitHostPort(closedLn.Addr().String())
	closedPort, _ := strconv.Atoi(closedPortStr)
	_ = closedLn.Close()
	res = Probe(context.Background(), "127.0.0.1", store.VerifyConfig{Enabled: true, Mode: store.VerifyModeWebSocket, Host: "cdn.example.com", Port: closedPort}, 443)
	if res.OK || res.Stage != "tcp" {
		t.Fatalf("closed port should fail at tcp: %+v", res)
	}
}

func TestFilterPreservesOrderAndStopsEarly(t *testing.T) {
	candidates := []store.SpeedResult{
		{IP: "1.1.1.1", Rank: 1},
		{IP: "1.1.1.2", Rank: 2},
		{IP: "1.1.1.3", Rank: 3},
		{IP: "1.1.1.4", Rank: 4},
		{IP: "1.1.1.5", Rank: 5},
	}
	var calls atomic.Int32
	var logged []string
	passed := Filter(context.Background(), candidates, store.VerifyConfig{
		Enabled:       true,
		MaxCandidates: 4,
		Concurrency:   3,
	}, 443, 1, func(_ context.Context, ip string, _ store.VerifyConfig, _ int) Result {
		calls.Add(1)
		// Rank 2 比 Rank 3 慢，但最终结果必须保持 Rank 2 在前
		if ip == "1.1.1.2" {
			time.Sleep(25 * time.Millisecond)
		}
		if ip == "1.1.1.1" {
			return Result{IP: ip, OK: false, Stage: "websocket", Status: 403, Error: "HTTP 403"}
		}
		return Result{IP: ip, OK: true, Stage: "ok", Status: 101}
	}, func(r Result) {
		logged = append(logged, fmt.Sprintf("%s:%t", r.IP, r.OK))
	})

	// 第一批(3个)凑够 need=1，不应继续测第 4 个
	if calls.Load() != 3 {
		t.Fatalf("calls=%d, want 3 (early stop after first batch)", calls.Load())
	}
	if len(passed) != 2 || passed[0].IP != "1.1.1.2" || passed[1].IP != "1.1.1.3" {
		t.Fatalf("passed=%+v (order not preserved)", passed)
	}
	if strings.Join(logged, ",") != "1.1.1.1:false,1.1.1.2:true,1.1.1.3:true" {
		t.Fatalf("logged=%v", logged)
	}
}
