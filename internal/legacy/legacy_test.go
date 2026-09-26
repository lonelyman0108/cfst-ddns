package legacy

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

var base = store.SpeedTestConfig{Threads: 200, PingTimes: 4, DownloadCount: 10, DownloadTime: 10,
	Port: 443, MaxLatency: 9999, MaxLossRate: 1, IPSource: "default"}

func TestParseVars(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want map[string]string
	}{
		{"shell", `DNS_PROVIDER="cloudflare"
CF_API_TOKEN='abc"def'
# CF_EMAIL="x@y.z"
CFST_PARAMS="-n 200 -t 4"   # 行尾注释
export TG_CHAT_ID=12345
BARK_URL=https://api.day.app # 注释`,
			map[string]string{"DNS_PROVIDER": "cloudflare", "CF_API_TOKEN": `abc"def`, "CFST_PARAMS": "-n 200 -t 4",
				"TG_CHAT_ID": "12345", "BARK_URL": "https://api.day.app"}},
		{"env 与 CRLF", "\ufeffA_B=1\r\nC=x=y\r\n\r\nEMPTY=\r\n", map[string]string{"A_B": "1", "C": "x=y", "EMPTY": ""}},
		{"compose 列表", `services:
  cfst-ddns:
    image: lonelyman0108/cfst-ddns:latest
    environment:
      - TZ=Asia/Shanghai
      - CRON_SCHEDULE=0 */6 * * *
      - "DNS_RECORD_NAMES=a.example.com b.example.com"
      # - ENABLE_BARK=true`,
			map[string]string{"TZ": "Asia/Shanghai", "CRON_SCHEDULE": "0 */6 * * *", "DNS_RECORD_NAMES": "a.example.com b.example.com"}},
		{"compose 映射", `    environment:
      CF_API_TOKEN: "tok:en"
      BARK_URL: https://api.day.app/KEY
      ENABLE_CRON: 'true'`,
			map[string]string{"CF_API_TOKEN": "tok:en", "BARK_URL": "https://api.day.app/KEY", "ENABLE_CRON": "true"}},
		{"旧脚本默认值与转义", `DNS_PROVIDER="${DNS_PROVIDER:-dnspod}"
X="a \"b\" \$c"
X2="unterminated`,
			map[string]string{"DNS_PROVIDER": "dnspod", "X": `a "b" $c`, "X2": "unterminated"}},
		{"后者覆盖", "A=1\nA=2", map[string]string{"A": "2"}},
		{"外部变量引用", "A=${TOKEN}\nB=\"$TOKEN\"\nC='${TOKEN}'\nD=${X:-${Y}}\nE=pre${X}",
			map[string]string{"A": "", "B": "", "C": "${TOKEN}", "D": "", "E": "pre${X}"}},
		{"忽略小写与无效行", "lower=1\nnot a var\n=x\n- \n", map[string]string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ParseVars(c.in); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %#v\nwant %#v", got, c.want)
			}
		})
	}
}

func TestParseParams(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		check func(st store.SpeedTestConfig) bool
		extra string
		warns int
	}{
		{"常用参数", "-n 500 -t 8 -dn 5 -dt 15 -tp 80 -tl 200 -tll 40 -tlr 0.2 -sl 5.5", func(st store.SpeedTestConfig) bool {
			return st.Threads == 500 && st.PingTimes == 8 && st.DownloadCount == 5 && st.DownloadTime == 15 && st.Port == 80 &&
				st.MaxLatency == 200 && st.MinLatency == 40 && st.MaxLossRate == 0.2 && st.MinSpeed == 5.5
		}, "", 0},
		{"开关与字符串", `-httping -httping-code 204 -cfcolo HKG,NRT -dd -allip -url "https://x.example/a b"`, func(st store.SpeedTestConfig) bool {
			return st.Httping && st.HttpingCode == 204 && st.CFColo == "HKG,NRT" && st.DisableDownload && st.AllIP &&
				st.URL == "https://x.example/a b"
		}, "", 0},
		{"等号写法", "-n=300 -dd=false", func(st store.SpeedTestConfig) bool { return st.Threads == 300 && !st.DisableDownload }, "", 0},
		{"自定义 IP", "-ip 1.1.1.1,2.2.2.0/24,2606:4700::/32", func(st store.SpeedTestConfig) bool {
			return st.IPSource == "custom" && st.IPv4Ranges == "1.1.1.1\n2.2.2.0/24" && st.IPv6Ranges == "2606:4700::/32"
		}, "", 0},
		{"程序控制与未知参数", "-o result.csv -p 10 -f ip.txt -debug -foo bar -n 100", func(st store.SpeedTestConfig) bool {
			return st.Threads == 100
		}, "-debug -foo bar", 1},
		{"无效值", "-n abc -sl", func(st store.SpeedTestConfig) bool { return st.Threads == 200 && st.MinSpeed == 0 }, "", 2},
		{"引号未闭合", `-url "x`, func(st store.SpeedTestConfig) bool { return st == base }, "", 1},
		{"空", "", func(st store.SpeedTestConfig) bool { return st == base }, "", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st, warns := ParseParams(c.in, base)
			if !c.check(st) || st.ExtraArgs != c.extra || len(warns) != c.warns {
				t.Fatalf("st = %+v\nwarns = %v", st, warns)
			}
		})
	}
}

func TestSplitBark(t *testing.T) {
	cases := [][4]string{
		{"https://api.day.app", "KEY", "https://api.day.app", "KEY"},
		{"https://api.day.app/KEY/", "", "https://api.day.app", "KEY"},
		{"https://bark.example.com/push/KEY", "", "https://bark.example.com/push", "KEY"},
		{"", "", "https://api.day.app", ""},
		{"https://api.day.app", "", "https://api.day.app", ""},
	}
	for _, c := range cases {
		s, k := splitBark(c[0], c[1])
		if s != c[2] || k != c[3] {
			t.Errorf("splitBark(%q, %q) = %q, %q", c[0], c[1], s, k)
		}
	}
}

func TestSplitRecord(t *testing.T) {
	domains := []string{"example.com", "sub.example.com", "example.co.uk"}
	cases := []struct{ fqdn, domain, rr string }{
		{"a.example.com", "example.com", "a"},
		{"example.com", "example.com", "@"},
		{"x.y.sub.example.com", "sub.example.com", "x.y"},
		{"cdn.example.co.uk.", "example.co.uk", "cdn"},
		{"A.Example.COM", "example.com", "a"},
		{"notexample.com", "", ""},
		{"other.org", "", ""},
	}
	for _, c := range cases {
		d, rr, ok := SplitRecord(c.fqdn, domains)
		if d != c.domain || rr != c.rr || ok != (c.domain != "") {
			t.Errorf("SplitRecord(%q) = %q %q %v", c.fqdn, d, rr, ok)
		}
	}
}

func hasWarn(p *Plan, sub string) bool {
	for _, w := range p.Warnings {
		if strings.Contains(w.Error(), sub) {
			return true
		}
	}
	return false
}

func TestConvert(t *testing.T) {
	t.Run("cloudflare 完整配置", func(t *testing.T) {
		p := Convert(ParseVars(`DNS_PROVIDER="cloudflare"
DNS_RECORD_NAMES="a.example.com, b.example.com a.example.com"
CF_API_TOKEN="tok"
CF_ZONE_ID="your_zone_id_here"
CFST_PARAMS="-n 300 -sl 5 -debug"
CFST_TEST_MODE="both"
GITHUB_MIRROR="https://mirror.example/"
ENABLE_BARK="true"
BARK_URL="https://api.day.app"
BARK_KEY="bk"
ENABLE_TELEGRAM="true"
TG_BOT_TOKEN="123:abc"
TG_CHAT_ID="-100"
ENABLE_CRON=true
CRON_SCHEDULE="*/30 * * * *"
SKIP_SPEED_TEST="false"
FOO=bar`), base)
		want := &Account{Name: "Cloudflare（v1 导入）", Provider: "cloudflare", Config: schema.Config{"authType": "token", "apiToken": "tok"}}
		if !reflect.DeepEqual(p.Account, want) {
			t.Fatalf("account = %+v", p.Account)
		}
		if len(p.Notifiers) != 2 || p.Notifiers[0].Type != "bark" || p.Notifiers[0].Config["deviceKey"] != "bk" ||
			p.Notifiers[1].Type != "telegram" || p.Notifiers[1].Config["chatId"] != "-100" || !p.Notifiers[1].OnSuccess || !p.Notifiers[1].OnFailure {
			t.Fatalf("notifiers = %+v", p.Notifiers)
		}
		tk := p.Task
		if tk == nil || tk.Cron != "*/30 * * * *" || tk.IPType != "both" || tk.SpeedTest.Threads != 300 ||
			tk.SpeedTest.MinSpeed != 5 || tk.SpeedTest.ExtraArgs != "-debug" ||
			!reflect.DeepEqual(tk.Records, []string{"a.example.com", "b.example.com"}) {
			t.Fatalf("task = %+v", tk)
		}
		if p.GithubMirror != "https://mirror.example" {
			t.Fatalf("mirror = %q", p.GithubMirror)
		}
		if len(p.Warnings) != 1 || !hasWarn(p, "FOO") {
			t.Fatalf("warnings = %v", p.Warnings)
		}
		if s := tk.Summary().Error(); !strings.Contains(s, "A + AAAA") || !strings.Contains(s, "-debug -n 300 -sl 5") || !strings.Contains(s, "*/30") {
			t.Fatalf("summary = %s", s)
		}
	})

	t.Run("cloudflare global key", func(t *testing.T) {
		p := Convert(ParseVars("CF_EMAIL=a@b.c\nCF_API_KEY=k\nCF_ZONE_ID=z1\nDNS_RECORD_NAMES=x.example.com"), base)
		if p.Account == nil || p.Account.Config["authType"] != "key" || p.Account.Config["email"] != "a@b.c" ||
			p.Account.Config["zoneId"] != "z1" {
			t.Fatalf("account = %+v", p.Account)
		}
		if p.Task.Cron != DefaultCron || !hasWarn(p, "CRON_SCHEDULE") {
			t.Fatalf("未配置周期时应使用默认值并提示: %q %v", p.Task.Cron, p.Warnings)
		}
	})

	t.Run("dnspod 推断与单次模式", func(t *testing.T) {
		p := Convert(ParseVars("DNSPOD_TOKEN=12345,abcdef\nENABLE_CRON=false\nCRON_SCHEDULE=0 */6 * * *\nDNS_RECORD_NAMES=example.com"), base)
		if p.Account == nil || p.Account.Provider != "dnspod" || p.Account.Config["tokenId"] != "12345" || p.Account.Config["token"] != "abcdef" {
			t.Fatalf("account = %+v", p.Account)
		}
		if p.Task.Cron != "" || len(p.Warnings) != 0 {
			t.Fatalf("cron = %q warnings = %v", p.Task.Cron, p.Warnings)
		}
	})

	t.Run("dnspod 格式错误", func(t *testing.T) {
		p := Convert(ParseVars("DNS_PROVIDER=dnspod\nDNSPOD_TOKEN=onlytoken"), base)
		if p.Account != nil || p.Task != nil || !hasWarn(p, "ID,Token") || !hasWarn(p, "不会创建任务") {
			t.Fatalf("plan = %+v", p)
		}
	})

	t.Run("缺少凭据与无效值", func(t *testing.T) {
		p := Convert(ParseVars("DNS_PROVIDER=aliyun\nGITHUB_MIRROR=mirror.example\nCFST_VERSION=v2.2.5\nSKIP_SPEED_TEST=true"), base)
		for _, w := range []string{"aliyun", "GITHUB_MIRROR", "CFST_VERSION", "SKIP_SPEED_TEST"} {
			if !hasWarn(p, w) {
				t.Errorf("missing warning %q: %v", w, p.Warnings)
			}
		}
		if p.Account != nil || p.GithubMirror != "" {
			t.Fatalf("plan = %+v", p)
		}
	})

	t.Run("通知未启用或不完整", func(t *testing.T) {
		p := Convert(ParseVars("CF_API_TOKEN=t\nBARK_KEY=k\nENABLE_TELEGRAM=true\nTG_BOT_TOKEN=x\nCRON_SCHEDULE=bad cron"), base)
		if len(p.Notifiers) != 0 || !hasWarn(p, "ENABLE_BARK") || !hasWarn(p, "TG_CHAT_ID") {
			t.Fatalf("notifiers = %+v warnings = %v", p.Notifiers, p.Warnings)
		}
		if p.Task.Cron != DefaultCron || !hasWarn(p, "无效") || !hasWarn(p, "DNS_RECORD_NAMES") {
			t.Fatalf("task = %+v warnings = %v", p.Task, p.Warnings)
		}
	})

	t.Run("Bark 密钥写在地址中", func(t *testing.T) {
		p := Convert(ParseVars("CF_API_TOKEN=t\nENABLE_BARK=true\nBARK_URL=https://api.day.app/KEY123"), base)
		if len(p.Notifiers) != 1 || p.Notifiers[0].Config["server"] != "https://api.day.app" || p.Notifiers[0].Config["deviceKey"] != "KEY123" {
			t.Fatalf("notifiers = %+v", p.Notifiers)
		}
	})

	t.Run("自定义 IP 未覆盖测速模式", func(t *testing.T) {
		p := Convert(ParseVars(`CF_API_TOKEN=t
CFST_TEST_MODE=both
CFST_PARAMS="-ip 1.1.1.0/24"`), base)
		if p.Task.SpeedTest.IPSource != "default" || !hasWarn(p, "-ip") {
			t.Fatalf("speedTest = %+v warnings = %v", p.Task.SpeedTest, p.Warnings)
		}
		p = Convert(ParseVars(`CF_API_TOKEN=t
CFST_PARAMS="-ip 1.1.1.0/24"`), base)
		if p.Task.SpeedTest.IPSource != "custom" || p.Task.SpeedTest.IPv4Ranges != "1.1.1.0/24" {
			t.Fatalf("speedTest = %+v", p.Task.SpeedTest)
		}
	})

	t.Run("示例配置文件", func(t *testing.T) {
		// legacy/config.example.sh 的主要内容：CF_ZONE_ID 为占位值，凭据为空
		p := Convert(ParseVars(`DNS_PROVIDER="cloudflare"
DNS_RECORD_NAMES="test.example.com"
CF_API_TOKEN=""
CF_API_KEY=""
CF_EMAIL=""
CF_ZONE_ID="your_zone_id_here"
DNSPOD_TOKEN=""
CFST_PARAMS="-n 200 -t 4"
CFST_TEST_MODE="v4"
SKIP_SPEED_TEST="false"
GITHUB_MIRROR=""
ENABLE_BARK="false"                          # 是否启用 Bark 通知
BARK_URL="https://api.day.app"               # Bark 服务器地址
BARK_KEY=""                                  # Bark 设备密钥
ENABLE_TELEGRAM="false"
TG_BOT_TOKEN=""
TG_CHAT_ID=""`), base)
		if p.Account != nil || len(p.Notifiers) != 0 || !hasWarn(p, "未找到 Cloudflare 凭据") {
			t.Fatalf("plan = %+v", p)
		}
	})
}

func TestParseRefs(t *testing.T) {
	p := Parse("CF_API_TOKEN=${CF_TOKEN}\nDNS_RECORD_NAMES=a.example.com\nTG_BOT_TOKEN=$BOT\nTG_BOT_TOKEN=real", base)
	if len(p.Warnings) < 2 || p.Warnings[0].Error() != "变量 CF_API_TOKEN 引用了外部环境变量 ${CF_TOKEN}，请填写实际值" {
		t.Fatalf("warnings = %v", p.Warnings)
	}
	// 后面重新赋了实际值的变量不再提示
	for _, w := range p.Warnings {
		if strings.Contains(w.Error(), "TG_BOT_TOKEN 引用") {
			t.Fatalf("unexpected warning: %s", w)
		}
	}
	// 引用被视为空值，因此没有凭据
	if p.Account != nil || !hasWarn(p, "未找到 Cloudflare 凭据") {
		t.Fatalf("account = %+v", p.Account)
	}
}
