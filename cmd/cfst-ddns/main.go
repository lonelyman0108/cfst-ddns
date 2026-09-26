// cfst-ddns：CloudflareSpeedTest 自动测速并更新 DNS 的 Web 服务。
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/api"
	"github.com/lonelyman0108/cfst-ddns/internal/app"
	"github.com/lonelyman0108/cfst-ddns/internal/auth"
	"github.com/lonelyman0108/cfst-ddns/web"
)

// 由 -ldflags "-X main.version=..." 注入。
var (
	version   = "dev"
	commit    = "none"
	buildTime = "unknown"
)

// fillFromVCS 在未通过 -ldflags 注入时，用 go build 自动嵌入的 Git 信息补充提交号与时间。
// go run 不嵌入 VCS 信息，此时保持默认值。
func fillFromVCS() {
	info, ok := debug.ReadBuildInfo()
	if !ok || commit != "none" {
		return
	}
	var dirty bool
	for _, st := range info.Settings {
		switch st.Key {
		case "vcs.revision":
			commit = st.Value
			if len(commit) > 7 {
				commit = commit[:7]
			}
		case "vcs.time":
			if buildTime == "unknown" {
				buildTime = st.Value
			}
		case "vcs.modified":
			dirty = st.Value == "true"
		}
	}
	if dirty && commit != "none" {
		commit += "-dirty"
	}
}

const usage = `cfst-ddns %s

用法:
  cfst-ddns [serve] [参数]           启动服务（默认）
  cfst-ddns reset-password [参数]    重置管理员用户名/密码
  cfst-ddns healthcheck              检查本机服务是否存活（容器 HEALTHCHECK 使用）
  cfst-ddns version                  显示版本

serve 参数:
  -listen      监听地址，默认 :8080          (CFST_DDNS_LISTEN)
  -data        数据目录，默认 ./data         (CFST_DDNS_DATA)
  -log-level   debug/info/warn/error         (CFST_DDNS_LOG_LEVEL)
  -bundle-dir  预置 cfst 目录                (CFST_DDNS_BUNDLE_DIR)
  -auto-install 未安装 cfst 时自动下载       (CFST_DDNS_AUTO_INSTALL)

其他环境变量:
  CFST_DDNS_SECRET   配置加密密钥（不设置则自动生成 data/secret.key）
  ADMIN_USERNAME / ADMIN_PASSWORD   首次启动时自动创建管理员
`

func main() {
	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		cmd, args = args[0], args[1:]
	}
	fillFromVCS()
	build := api.BuildInfo{Version: version, Commit: commit, BuildTime: buildTime}

	var err error
	switch cmd {
	case "serve":
		var c app.Config
		if c, err = app.ParseFlags(args); err == nil {
			err = app.Serve(c, build, web.Dist())
		}
	case "reset-password":
		err = resetPassword(args)
	case "healthcheck":
		err = healthcheck()
	case "version":
		fmt.Printf("cfst-ddns %s (commit %s, built %s, %s %s/%s)\n", version, commit, buildTime, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	case "help", "-h", "--help":
		fmt.Printf(usage, version)
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", cmd)
		fmt.Printf(usage, version)
		os.Exit(2)
	}
	if err != nil && err != flag.ErrHelp {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

func resetPassword(args []string) error {
	fs := flag.NewFlagSet("reset-password", flag.ContinueOnError)
	user := fs.String("username", "admin", "新用户名")
	pass := fs.String("password", "", "新密码（至少 6 位）")
	data := fs.String("data", "", "数据目录，默认同 serve")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pass == "" {
		return fmt.Errorf("请通过 -password 指定新密码")
	}
	serveArgs := []string{}
	if *data != "" {
		serveArgs = append(serveArgs, "-data", *data)
	}
	c, err := app.ParseFlags(serveArgs)
	if err != nil {
		return err
	}
	st, err := app.OpenStore(c)
	if err != nil {
		return err
	}
	defer st.Close()
	svc, err := auth.New(st)
	if err != nil {
		return err
	}
	if _, err := svc.SetPassword(*user, *pass); err != nil {
		return err
	}
	fmt.Printf("已重置管理员: %s（所有已登录会话已失效）\n", *user)
	return nil
}

// healthcheck 请求本机 /healthz，端口取自 CFST_DDNS_LISTEN。
func healthcheck() error {
	listen := os.Getenv("CFST_DDNS_LISTEN")
	if listen == "" {
		listen = ":8080"
	}
	_, port, err := net.SplitHostPort(listen)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
