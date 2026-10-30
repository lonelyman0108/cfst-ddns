package netcheck

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

func TestLatencyAnomalous(t *testing.T) {
	mk := func(lat ...float64) []store.SpeedResult {
		out := make([]store.SpeedResult, len(lat))
		for i, l := range lat {
			out[i].Latency = l
		}
		return out
	}
	cases := []struct {
		name string
		res  []store.SpeedResult
		want bool
	}{
		{"本地秒回", mk(0.5, 0.7, 0.8, 0.9, 1.1, 1.3, 1.6), true},
		{"正常直连", mk(38, 42, 45, 51, 60, 75, 120), false},
		{"同机房但分散", mk(1, 2, 3, 4, 6, 9, 15), false},
		{"样本太少", mk(0.5, 0.6, 0.7), false},
	}
	for _, c := range cases {
		if got := LatencyAnomalous(c.res); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestParseTrace(t *testing.T) {
	kv := ParseTrace("fl=1\nip=2403:71c0::a\nloc=JP\ncolo=NRT\n")
	if kv["ip"] != "2403:71c0::a" || kv["loc"] != "JP" || kv["colo"] != "NRT" {
		t.Fatalf("kv = %v", kv)
	}
}

// swap 临时替换包级变量，测试结束后恢复。
func swap[T any](t *testing.T, p *T, v T) {
	old := *p
	*p = v
	t.Cleanup(func() { *p = old })
}

func TestProbe(t *testing.T) {
	trace := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "h=%s\nip=203.0.113.9\nloc=JP\ncolo=NRT\n", r.Host)
	}))
	defer trace.Close()
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "198.51.100.1\n")
	}))
	defer echo.Close()

	host, port, _ := net.SplitHostPort(trace.Listener.Addr().String())
	swap(t, &traceScheme, "http")
	swap(t, &tracePort, port)
	swap(t, &traceTargets, map[string][]traceTarget{"v4": {{host, "speed.cloudflare.com:" + port}}})
	// 正常网络下保留地址连不上：用一个已关闭的端口模拟
	closed, _ := net.Listen("tcp4", "127.0.0.1:0")
	closedAddr := closed.Addr().String()
	closed.Close()
	swap(t, &echoURLs, map[string][]string{"v4": {"http://" + closedAddr, echo.URL}})
	swap(t, &canaryAddr, map[string]string{"v4": closedAddr})
	swap(t, &canaryTimeout, 500*time.Millisecond)

	p := Probe(context.Background(), []string{"v4"}, true)[0]
	if p.Intercepted || p.EgressIP != "203.0.113.9" || p.Loc != "JP" || p.Colo != "NRT" || p.LocalIP != "198.51.100.1" || p.ConnectMs <= 0 {
		t.Fatalf("probe = %+v", p)
	}
	if !p.EgressMismatch() {
		t.Fatal("出口不同应判定为不一致")
	}

	// 被拦截时保留地址也能握手成功
	swap(t, &canaryAddr, map[string]string{"v4": trace.Listener.Addr().String()})
	if p := Probe(context.Background(), []string{"v4"}, true)[0]; !p.Intercepted {
		t.Fatalf("probe = %+v", p)
	}
	// 经代理模式不探测保留地址
	if p := Probe(context.Background(), []string{"v4"}, false)[0]; p.Intercepted {
		t.Fatalf("probe = %+v", p)
	}
}
