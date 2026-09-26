package cfst

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

// Output 接收运行输出。
type Output interface {
	Line(s string)
	Progress(s string)
}

// RunOptions 为一次测速（单一 IP 类型）的参数。
type RunOptions struct {
	IPType  string // v4 / v6
	Config  store.SpeedTestConfig
	WorkDir string // 临时文件目录
	Tag     string // 临时文件名前缀
}

// BuildArgs 生成 cfst 参数。ipFile/outFile 由调用方提供。
// 附加参数放在前面，必须由程序控制的参数（-f/-o/-p）放在最后以覆盖同名参数。
func BuildArgs(c store.SpeedTestConfig, ipFile, outFile string) ([]string, error) {
	var args []string
	extra, err := SplitArgs(c.ExtraArgs)
	if err != nil {
		return nil, err
	}
	args = append(args, extra...)
	intArg := func(flag string, v, def int) {
		if v > 0 && v != def {
			args = append(args, flag, strconv.Itoa(v))
		}
	}
	intArg("-n", c.Threads, 200)
	intArg("-t", c.PingTimes, 4)
	intArg("-dn", c.DownloadCount, 10)
	intArg("-dt", c.DownloadTime, 10)
	intArg("-tp", c.Port, 443)
	if u := strings.TrimSpace(c.URL); u != "" {
		args = append(args, "-url", u)
	}
	if c.Httping {
		args = append(args, "-httping")
		if c.HttpingCode > 0 {
			args = append(args, "-httping-code", strconv.Itoa(c.HttpingCode))
		}
		if colo := strings.ReplaceAll(strings.TrimSpace(c.CFColo), " ", ""); colo != "" {
			args = append(args, "-cfcolo", strings.ToUpper(colo))
		}
	}
	intArg("-tl", c.MaxLatency, 9999)
	if c.MinLatency > 0 {
		args = append(args, "-tll", strconv.Itoa(c.MinLatency))
	}
	if c.MaxLossRate >= 0 && c.MaxLossRate < 1 {
		args = append(args, "-tlr", strconv.FormatFloat(c.MaxLossRate, 'f', -1, 64))
	}
	if c.MinSpeed > 0 {
		args = append(args, "-sl", strconv.FormatFloat(c.MinSpeed, 'f', -1, 64))
	}
	if c.DisableDownload {
		args = append(args, "-dd")
	}
	if c.AllIP {
		args = append(args, "-allip")
	}
	args = append(args, "-f", ipFile, "-o", outFile, "-p", "0")
	return args, nil
}

// SplitArgs 按空白拆分参数，支持单/双引号。
func SplitArgs(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	var quote rune
	has := false
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, has = r, true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if has {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	if quote != 0 {
		return nil, errors.New("附加参数中的引号未闭合")
	}
	if has {
		out = append(out, cur.String())
	}
	return out, nil
}

// Run 执行一次测速并返回结果（按 cfst 排序）。
func (m *Manager) Run(ctx context.Context, opt RunOptions, out Output) ([]store.SpeedResult, error) {
	if m.isInstalling() {
		return nil, errors.New("cfst 正在安装，请稍后")
	}
	if !m.Installed() {
		return nil, errors.New("cfst 未安装，请先在「cfst 管理」中安装")
	}
	if err := os.MkdirAll(opt.WorkDir, 0o755); err != nil {
		return nil, err
	}
	ipFile := m.IPFile(opt.IPType)
	if opt.Config.IPSource == "custom" {
		ranges := opt.Config.IPv4Ranges
		if opt.IPType == "v6" {
			ranges = opt.Config.IPv6Ranges
		}
		ranges = strings.TrimSpace(strings.NewReplacer(",", "\n", "，", "\n").Replace(ranges))
		if ranges == "" {
			return nil, fmt.Errorf("自定义 IP 段（%s）为空", opt.IPType)
		}
		ipFile = filepath.Join(opt.WorkDir, fmt.Sprintf("%s-%s-ip.txt", opt.Tag, opt.IPType))
		if err := os.WriteFile(ipFile, []byte(ranges+"\n"), 0o644); err != nil {
			return nil, err
		}
		defer os.Remove(ipFile)
	} else if _, err := os.Stat(ipFile); err != nil {
		return nil, fmt.Errorf("IP 段文件不存在: %s", ipFile)
	}
	outFile := filepath.Join(opt.WorkDir, fmt.Sprintf("%s-%s.csv", opt.Tag, opt.IPType))
	_ = os.Remove(outFile)
	defer os.Remove(outFile)

	args, err := BuildArgs(opt.Config, ipFile, outFile)
	if err != nil {
		return nil, err
	}
	out.Line("$ cfst " + strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, m.BinPath(), args...)
	cmd.Dir = m.Dir
	cmd.WaitDelay = 5 * time.Second
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 cfst 失败: %w", err)
	}
	done := make(chan struct{})
	go func() {
		pump(pr, out)
		close(done)
	}()
	err = cmd.Wait()
	pw.Close()
	<-done
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("cfst 异常退出: %w", err)
	}
	f, err := os.ReadFile(outFile)
	if err != nil {
		// 没有任何 IP 满足条件时 cfst 不会生成文件
		return nil, nil
	}
	return ParseCSV(f, opt.IPType)
}

var (
	ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)
	// barStart 匹配 cfst 进度条开头的计数器，如 "12 / 64 ["
	barStart = regexp.MustCompile(`\d+ / \d+ \[`)
)

// lastBar 在多个进度条首尾相接时只保留最后一个。
// cfst 使用的 pb/v3 在输出不是终端时不加 \r，每 200ms 直接追加一整条进度条。
func lastBar(s string) string {
	locs := barStart.FindAllStringIndex(s, -1)
	if len(locs) > 1 {
		return s[locs[len(locs)-1][0]:]
	}
	return s
}

func clean(s string) string {
	return strings.TrimSpace(lastBar(ansi.ReplaceAllString(s, "")))
}

// pump 把输出拆分为日志行与进度行：以 \n 结尾的是日志；以 \r 结尾，
// 或暂时没有换行的尾部（非终端模式下的进度条）作为进度行，限流推送。
func pump(r io.Reader, out Output) {
	var buf []byte
	var last time.Time
	progress := func(s string) {
		if s = clean(s); s != "" && time.Since(last) >= 300*time.Millisecond {
			last = time.Now()
			out.Progress(s)
		}
	}
	chunk := make([]byte, 32*1024)
	for {
		n, err := r.Read(chunk)
		buf = append(buf, chunk[:n]...)
		for {
			i := bytes.IndexAny(buf, "\r\n")
			if i < 0 {
				break
			}
			seg := string(buf[:i])
			if buf[i] == '\n' {
				if line := clean(seg); line != "" {
					out.Line(line)
				}
			} else {
				progress(seg)
			}
			buf = buf[i+1:]
		}
		if err != nil {
			if line := clean(string(buf)); line != "" {
				out.Line(line)
			}
			return
		}
		if len(buf) > 0 && barStart.Match(buf) {
			progress(string(buf))
			// 无换行的进度条会不断累积，只保留最后一条
			if len(buf) > 16*1024 {
				buf = []byte(lastBar(string(buf)))
			}
		}
	}
}

// ParseCSV 解析 cfst 结果文件（兼容 6 列与带地区码的 7 列格式）。
func ParseCSV(data []byte, ipType string) ([]store.SpeedResult, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("解析测速结果失败: %w", err)
	}
	var out []store.SpeedResult
	for i, row := range rows {
		if i == 0 || len(row) < 6 {
			continue
		}
		f := func(s string) float64 {
			v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
			return v
		}
		res := store.SpeedResult{IPType: ipType, Rank: len(out) + 1, IP: strings.TrimSpace(row[0]),
			Sent: int(f(row[1])), Received: int(f(row[2])), LossRate: f(row[3]), Latency: f(row[4]), Speed: f(row[5])}
		if len(row) > 6 {
			res.Colo = strings.TrimSpace(row[6])
		}
		if res.IP != "" {
			out = append(out, res)
		}
	}
	return out, nil
}
