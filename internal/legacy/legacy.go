// Package legacy 解析 v1（Bash 脚本版）的配置，转换为 v2 的账号、通知渠道与任务。
package legacy

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/lonelyman0108/cfst-ddns/internal/cfst"
	"github.com/lonelyman0108/cfst-ddns/internal/scheduler"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

// ---------- 变量解析 ----------

var (
	keyRe = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	// defaultRe 匹配 ${VAR:-默认值} / ${VAR-默认值}（直接粘贴旧脚本时出现）
	defaultRe = regexp.MustCompile(`^\$\{[A-Za-z_][A-Za-z0-9_]*:?-(.*)\}$`)
	// refRe 匹配没有默认值的纯引用 ${VAR} / $VAR，其值在外部环境中，无法导入
	refRe = regexp.MustCompile(`^\$(?:\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))$`)
)

// Parse 解析 v1 配置内容并转换，解析阶段的警告排在前面。
func Parse(content string, base store.SpeedTestConfig) *Plan {
	vars, warns := parseVars(content)
	p := Convert(vars, base)
	p.Warnings = append(warns, p.Warnings...)
	return p
}

// ParseVars 宽松解析 config.sh / .env / docker-compose environment 片段。
// 支持 KEY=VALUE、export KEY=VALUE、- KEY=VALUE、KEY: VALUE、引号与注释；
// 只接受全大写变量名（跳过 compose 中的 image:、services: 等结构行）。后出现的同名变量覆盖前者。
func ParseVars(content string) map[string]string {
	vars, _ := parseVars(content)
	return vars
}

func parseVars(content string) (map[string]string, []string) {
	vars := map[string]string{}
	refs := map[string]string{}
	content = strings.TrimPrefix(content, "\ufeff")
	for _, raw := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
		// compose 列表项整体加引号：- "KEY=VALUE"
		if n := len(line); n >= 2 && (line[0] == '"' || line[0] == '\'') && line[n-1] == line[0] {
			line = line[1 : n-1]
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, value, ok := splitLine(line)
		if !ok {
			continue
		}
		v, ref := unquote(value)
		vars[key] = v
		if ref != "" {
			refs[key] = ref
		} else {
			delete(refs, key)
		}
	}
	var warns []string
	for _, k := range sortedKeys(refs) {
		warns = append(warns, fmt.Sprintf("变量 %s 引用了外部环境变量 ${%s}，请填写实际值", k, refs[k]))
	}
	return vars, warns
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// splitLine 取 = 与 : 中最早出现且左侧是合法变量名的分隔符。
func splitLine(line string) (string, string, bool) {
	best := -1
	for _, sep := range []string{"=", ":"} {
		i := strings.Index(line, sep)
		if i <= 0 || (best >= 0 && i > best) {
			continue
		}
		key := line[:i]
		if sep == ":" {
			key = strings.TrimSpace(key)
		}
		if keyRe.MatchString(key) {
			best = i
		}
	}
	if best < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:best]), strings.TrimSpace(line[best+1:]), true
}

// unquote 去除引号与行尾注释，展开 ${VAR:-默认值}；
// 值为纯引用 ${VAR} / $VAR（单引号内除外）时返回空值与被引用的变量名。
func unquote(v string) (string, string) {
	if v == "" {
		return "", ""
	}
	if v[0] == '\'' {
		end := strings.IndexByte(v[1:], '\'')
		if end < 0 {
			return v[1:], ""
		}
		return v[1 : end+1], ""
	}
	switch q := v[0]; q {
	case '"':
		var b strings.Builder
		for i := 1; i < len(v); i++ {
			ch := v[i]
			if ch == q {
				break
			}
			if q == '"' && ch == '\\' && i+1 < len(v) && strings.IndexByte(`"\$`+"`", v[i+1]) >= 0 {
				i++
				ch = v[i]
			}
			b.WriteByte(ch)
		}
		v = b.String()
	default:
		if i := strings.Index(v, " #"); i >= 0 {
			v = v[:i]
		}
		v = strings.TrimSpace(v)
	}
	if m := defaultRe.FindStringSubmatch(v); m != nil {
		v = m[1]
	}
	if m := refRe.FindStringSubmatch(v); m != nil {
		return "", m[1] + m[2]
	}
	return v, ""
}

// ---------- 转换 ----------

// Account 为待创建的 DNS 账号。
type Account struct {
	Name     string
	Provider string
	Config   schema.Config
}

// Notifier 为待创建的通知渠道。
type Notifier struct {
	Name         string
	Type         string
	Config       schema.Config
	OnSuccess    bool
	OnFailure    bool
	OnlyOnChange bool
}

// Task 为待创建的任务；Records 为完整域名，需按账号的域名列表拆出主域名与主机记录。
type Task struct {
	Name      string
	Cron      string
	IPType    string
	SpeedTest store.SpeedTestConfig
	Records   []string
}

// Summary 返回任务的可读概述。
func (t Task) Summary() string {
	types := map[string]string{"v4": "A", "v6": "AAAA", "both": "A + AAAA"}[t.IPType]
	records := "无"
	if len(t.Records) > 0 {
		records = strings.Join(t.Records, "、")
	}
	args, _ := cfst.BuildArgs(t.SpeedTest, "", "")
	params := "全部默认"
	if len(args) > 6 {
		// 去掉末尾由程序控制的 -f/-o/-p
		params = strings.Join(args[:len(args)-6], " ")
	}
	cron := t.Cron
	if cron == "" {
		cron = "仅手动执行"
	}
	s := fmt.Sprintf("目标记录（%s）: %s；非默认参数: %s；执行周期: %s", types, records, params, cron)
	if t.SpeedTest.IPSource == "custom" {
		s += "；使用自定义 IP 段"
	}
	return s
}

// Plan 为转换结果。
type Plan struct {
	Account      *Account
	Notifiers    []Notifier
	Task         *Task
	GithubMirror string // 空表示未配置
	Warnings     []string
}

func (p *Plan) warn(format string, a ...any) {
	p.Warnings = append(p.Warnings, fmt.Sprintf(format, a...))
}

// DefaultCron 为旧版 Docker 镜像的默认执行周期。
const DefaultCron = "0 */6 * * *"

// 已知但在 v2 中无需迁移的变量（不产生警告）。
var ignored = map[string]bool{
	"CFST_BIN": true, "DATA_DIR": true, "RESULT_FILE": true, "AUTO_INSTALL_CFST": true, "LOG_MAX_SIZE": true,
	"TZ": true,
}

// 会被转换的变量。
var mapped = map[string]bool{
	"DNS_PROVIDER": true, "DNS_RECORD_NAMES": true, "CF_RECORD_NAMES": true, "CF_API_TOKEN": true, "CF_API_KEY": true,
	"CF_EMAIL": true, "CF_ZONE_ID": true, "DNSPOD_TOKEN": true, "CFST_PARAMS": true, "CFST_TEST_MODE": true,
	"SKIP_SPEED_TEST": true, "GITHUB_MIRROR": true, "CFST_VERSION": true, "ENABLE_BARK": true, "BARK_URL": true,
	"BARK_KEY": true, "ENABLE_TELEGRAM": true, "TG_BOT_TOKEN": true, "TG_CHAT_ID": true, "ENABLE_CRON": true,
	"CRON_SCHEDULE": true,
}

// Convert 把 v1 变量转换为 v2 配置；base 为新建任务的默认测速参数。
func Convert(vars map[string]string, base store.SpeedTestConfig) *Plan {
	p := &Plan{}
	get := func(k string) string { return strings.TrimSpace(vars[k]) }

	var unknown []string
	for k := range vars {
		if !ignored[k] && !mapped[k] && !strings.HasPrefix(k, "CFST_DDNS_") && !strings.HasPrefix(k, "ADMIN_") {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	for _, k := range unknown {
		p.warn("未识别的变量 %s，已忽略", k)
	}

	p.Account = convertAccount(p, vars)
	p.convertNotifiers(vars)

	if m := strings.TrimRight(get("GITHUB_MIRROR"), "/"); m != "" {
		if strings.HasPrefix(m, "http://") || strings.HasPrefix(m, "https://") {
			p.GithubMirror = m
		} else {
			p.warn("GITHUB_MIRROR 不是 http(s) 地址，已忽略: %s", m)
		}
	}
	if v := get("CFST_VERSION"); v != "" {
		p.warn("CFST_VERSION=%s 需在「cfst 管理」页面手动安装对应版本", v)
	}
	if strings.EqualFold(get("SKIP_SPEED_TEST"), "true") {
		p.warn("SKIP_SPEED_TEST 在 v2 中已移除，每次执行都会重新测速")
	}

	if p.Account == nil {
		p.warn("未导入 DNS 账号，因此不会创建任务")
		return p
	}
	p.Task = convertTask(p, vars, base)
	return p
}

func convertAccount(p *Plan, vars map[string]string) *Account {
	get := func(k string) string { return strings.TrimSpace(vars[k]) }
	prov := strings.ToLower(get("DNS_PROVIDER"))
	hasCF := get("CF_API_TOKEN") != "" || get("CF_API_KEY") != ""
	hasDP := get("DNSPOD_TOKEN") != ""
	if prov == "" {
		// v1 默认使用 Cloudflare；只配置了 DNSPod 凭据时按 DNSPod 处理
		prov = "cloudflare"
		if !hasCF && hasDP {
			prov = "dnspod"
		}
	}
	switch prov {
	case "cloudflare":
		if hasDP {
			p.warn("DNS_PROVIDER 为 cloudflare，未使用的 DNSPOD_TOKEN 已忽略")
		}
		cfg := schema.Config{}
		switch {
		case get("CF_API_TOKEN") != "":
			cfg["authType"], cfg["apiToken"] = "token", get("CF_API_TOKEN")
		case get("CF_API_KEY") != "" && get("CF_EMAIL") != "":
			cfg["authType"], cfg["email"], cfg["apiKey"] = "key", get("CF_EMAIL"), get("CF_API_KEY")
		default:
			p.warn("未找到 Cloudflare 凭据（CF_API_TOKEN，或 CF_EMAIL + CF_API_KEY），未创建账号")
			return nil
		}
		// 示例配置中的占位值不导入
		if z := get("CF_ZONE_ID"); z != "" && z != "your_zone_id_here" {
			cfg["zoneId"] = z
		}
		return &Account{Name: "Cloudflare（v1 导入）", Provider: "cloudflare", Config: cfg}
	case "dnspod":
		if hasCF {
			p.warn("DNS_PROVIDER 为 dnspod，未使用的 Cloudflare 凭据已忽略")
		}
		id, token, ok := strings.Cut(get("DNSPOD_TOKEN"), ",")
		id, token = strings.TrimSpace(id), strings.TrimSpace(token)
		if !ok || id == "" || token == "" {
			p.warn("DNSPOD_TOKEN 格式应为 ID,Token，未创建账号")
			return nil
		}
		return &Account{Name: "DNSPod（v1 导入）", Provider: "dnspod", Config: schema.Config{"tokenId": id, "token": token}}
	default:
		p.warn("不支持的 DNS_PROVIDER: %s，未创建账号", prov)
		return nil
	}
}

func (p *Plan) convertNotifiers(vars map[string]string) {
	get := func(k string) string { return strings.TrimSpace(vars[k]) }
	enabled := func(k string) bool { return strings.EqualFold(get(k), "true") }
	// v1 每次执行结束都会发送通知
	add := func(name, typ string, cfg schema.Config) {
		p.Notifiers = append(p.Notifiers, Notifier{Name: name, Type: typ, Config: cfg, OnSuccess: true, OnFailure: true})
	}

	if enabled("ENABLE_BARK") {
		server, key := splitBark(get("BARK_URL"), get("BARK_KEY"))
		if key == "" {
			p.warn("Bark 缺少设备密钥（BARK_KEY，或 BARK_URL 末尾的密钥），未导入")
		} else {
			add("Bark（v1 导入）", "bark", schema.Config{"server": server, "deviceKey": key})
		}
	} else if get("BARK_KEY") != "" {
		p.warn("Bark 未启用（ENABLE_BARK 不为 true），未导入")
	}

	if enabled("ENABLE_TELEGRAM") {
		if get("TG_BOT_TOKEN") == "" || get("TG_CHAT_ID") == "" {
			p.warn("Telegram 缺少 TG_BOT_TOKEN 或 TG_CHAT_ID，未导入")
		} else {
			add("Telegram（v1 导入）", "telegram", schema.Config{"botToken": get("TG_BOT_TOKEN"), "chatId": get("TG_CHAT_ID")})
		}
	} else if get("TG_BOT_TOKEN") != "" {
		p.warn("Telegram 未启用（ENABLE_TELEGRAM 不为 true），未导入")
	}
}

// splitBark 兼容 v1 的两种写法：BARK_URL + BARK_KEY，或把密钥写在 BARK_URL 末尾。
func splitBark(rawURL, key string) (string, string) {
	server := strings.TrimRight(rawURL, "/")
	if server == "" {
		server = "https://api.day.app"
	}
	if key != "" {
		return server, key
	}
	u, err := url.Parse(server)
	if err != nil || u.Host == "" {
		return server, ""
	}
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return server, ""
	}
	i := strings.LastIndex(path, "/")
	key = path[i+1:]
	u.Path = ""
	if i > 0 {
		u.Path = "/" + path[:i]
	}
	return u.String(), key
}

func convertTask(p *Plan, vars map[string]string, base store.SpeedTestConfig) *Task {
	get := func(k string) string { return strings.TrimSpace(vars[k]) }
	t := &Task{Name: "v1 导入任务", IPType: "v4", SpeedTest: base}

	switch mode := strings.ToLower(get("CFST_TEST_MODE")); mode {
	case "", "v4":
	case "v6", "both":
		t.IPType = mode
	default:
		p.warn("CFST_TEST_MODE=%s 无效，已使用 v4", mode)
	}

	cron := get("CRON_SCHEDULE")
	switch {
	case strings.EqualFold(get("ENABLE_CRON"), "false"):
		// 旧版单次执行模式：改为仅手动 / Webhook 触发
		t.Cron = ""
	case cron != "":
		if err := scheduler.Validate(cron); err != nil {
			p.warn("CRON_SCHEDULE=%s 无效（%v），已使用默认 %s", cron, err, DefaultCron)
			cron = DefaultCron
		}
		t.Cron = cron
	case strings.EqualFold(get("ENABLE_CRON"), "true"):
		t.Cron = DefaultCron
	default:
		t.Cron = DefaultCron
		p.warn("未找到 CRON_SCHEDULE，任务执行周期暂设为 %s，请按需修改", DefaultCron)
	}

	names := get("DNS_RECORD_NAMES")
	if names == "" {
		names = get("CF_RECORD_NAMES")
	}
	seen := map[string]bool{}
	for _, n := range strings.FieldsFunc(names, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	}) {
		n = strings.ToLower(strings.Trim(strings.TrimSpace(n), "."))
		if n != "" && !seen[n] {
			seen[n] = true
			t.Records = append(t.Records, n)
		}
	}
	if len(t.Records) == 0 {
		p.warn("未找到 DNS_RECORD_NAMES，任务没有目标记录，需手动补充")
	}

	st, warns := ParseParams(get("CFST_PARAMS"), base)
	t.SpeedTest = st
	p.Warnings = append(p.Warnings, warns...)
	// 自定义 IP 段必须覆盖任务的每种 IP 类型
	if st.IPSource == "custom" && ((t.IPType != "v6" && st.IPv4Ranges == "") || (t.IPType != "v4" && st.IPv6Ranges == "")) {
		p.warn("-ip 参数未覆盖测速模式 %s 所需的 IP 类型，已改用默认 IP 段", t.IPType)
		t.SpeedTest.IPSource, t.SpeedTest.IPv4Ranges, t.SpeedTest.IPv6Ranges = "default", "", ""
	}
	return t
}

// ParseParams 把 CFST_PARAMS 映射为结构化测速参数，无法映射的参数保留到附加参数。
func ParseParams(params string, base store.SpeedTestConfig) (store.SpeedTestConfig, []string) {
	st := base
	var warns []string
	args, err := cfst.SplitArgs(params)
	if err != nil {
		return st, []string{"CFST_PARAMS 解析失败（" + err.Error() + "），已忽略"}
	}
	var extra []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, val, hasVal := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if !strings.HasPrefix(arg, "-") {
			extra = append(extra, arg)
			continue
		}
		// value 取 -k=v 或下一个参数
		value := func() (string, bool) {
			if hasVal {
				return val, true
			}
			if i+1 < len(args) {
				i++
				return args[i], true
			}
			return "", false
		}
		setInt := func(dst *int) {
			v, ok := value()
			n, err := strconv.Atoi(v)
			if !ok || err != nil {
				warns = append(warns, fmt.Sprintf("CFST_PARAMS 中 %s 的值无效: %q，已忽略", arg, v))
				return
			}
			*dst = n
		}
		setFloat := func(dst *float64) {
			v, ok := value()
			n, err := strconv.ParseFloat(v, 64)
			if !ok || err != nil {
				warns = append(warns, fmt.Sprintf("CFST_PARAMS 中 %s 的值无效: %q，已忽略", arg, v))
				return
			}
			*dst = n
		}
		setBool := func(dst *bool) {
			*dst = !hasVal || val != "false"
		}
		switch name {
		case "n":
			setInt(&st.Threads)
		case "t":
			setInt(&st.PingTimes)
		case "dn":
			setInt(&st.DownloadCount)
		case "dt":
			setInt(&st.DownloadTime)
		case "tp":
			setInt(&st.Port)
		case "tl":
			setInt(&st.MaxLatency)
		case "tll":
			setInt(&st.MinLatency)
		case "httping-code":
			setInt(&st.HttpingCode)
		case "tlr":
			setFloat(&st.MaxLossRate)
		case "sl":
			setFloat(&st.MinSpeed)
		case "httping":
			setBool(&st.Httping)
		case "dd":
			setBool(&st.DisableDownload)
		case "allip":
			setBool(&st.AllIP)
		case "url":
			st.URL, _ = value()
		case "cfcolo":
			st.CFColo, _ = value()
		case "ip":
			v, _ := value()
			var v4, v6 []string
			for _, r := range strings.Split(v, ",") {
				if r = strings.TrimSpace(r); r == "" {
					continue
				}
				if strings.Contains(r, ":") {
					v6 = append(v6, r)
				} else {
					v4 = append(v4, r)
				}
			}
			st.IPSource, st.IPv4Ranges, st.IPv6Ranges = "custom", strings.Join(v4, "\n"), strings.Join(v6, "\n")
		case "f":
			v, _ := value()
			warns = append(warns, fmt.Sprintf("CFST_PARAMS 中的 -f %s 未迁移：IP 段文件请在「cfst 管理」中编辑，或在任务中使用自定义 IP 段", v))
		case "o", "p":
			// 由程序控制，忽略
			value()
		case "v", "h":
		default:
			// 未知参数原样保留；后面紧跟的非 - 开头参数视为其取值
			extra = append(extra, arg)
			if !hasVal && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				extra = append(extra, args[i])
			}
		}
	}
	if len(extra) > 0 {
		quoted := make([]string, len(extra))
		for i, a := range extra {
			if strings.ContainsAny(a, " \t") {
				a = `"` + a + `"`
			}
			quoted[i] = a
		}
		st.ExtraArgs = strings.TrimSpace(strings.TrimSpace(st.ExtraArgs + " " + strings.Join(quoted, " ")))
	}
	return st, warns
}

// SplitRecord 按账号的域名列表把完整域名拆为主域名与主机记录（取最长匹配）。
func SplitRecord(fqdn string, domains []string) (domain, rr string, ok bool) {
	fqdn = strings.ToLower(strings.Trim(fqdn, "."))
	for _, d := range domains {
		d = strings.ToLower(strings.Trim(d, "."))
		if d == "" || len(d) <= len(domain) {
			continue
		}
		if fqdn == d || strings.HasSuffix(fqdn, "."+d) {
			domain = d
		}
	}
	if domain == "" {
		return "", "", false
	}
	if fqdn == domain {
		return domain, "@", true
	}
	return domain, strings.TrimSuffix(fqdn, "."+domain), true
}
