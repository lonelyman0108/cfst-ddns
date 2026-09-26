// Package app 负责读取启动参数并装配各组件。
package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/api"
	"github.com/lonelyman0108/cfst-ddns/internal/auth"
	"github.com/lonelyman0108/cfst-ddns/internal/cfst"
	"github.com/lonelyman0108/cfst-ddns/internal/engine"
	"github.com/lonelyman0108/cfst-ddns/internal/httpx"
	"github.com/lonelyman0108/cfst-ddns/internal/logbus"
	"github.com/lonelyman0108/cfst-ddns/internal/scheduler"
	"github.com/lonelyman0108/cfst-ddns/internal/secret"
	"github.com/lonelyman0108/cfst-ddns/internal/store"

	_ "github.com/lonelyman0108/cfst-ddns/internal/notify"
	_ "github.com/lonelyman0108/cfst-ddns/internal/provider"
)

// Config 为启动参数，命令行优先于环境变量。
type Config struct {
	Listen      string
	DataDir     string
	Secret      string
	LogLevel    string
	BundleDir   string
	AutoInstall bool
	AdminUser   string
	AdminPass   string
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// ParseFlags 解析 serve 子命令参数。
func ParseFlags(args []string) (Config, error) {
	var c Config
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.StringVar(&c.Listen, "listen", env("CFST_DDNS_LISTEN", ":8080"), "监听地址 (CFST_DDNS_LISTEN)")
	fs.StringVar(&c.DataDir, "data", env("CFST_DDNS_DATA", "./data"), "数据目录 (CFST_DDNS_DATA)")
	fs.StringVar(&c.LogLevel, "log-level", env("CFST_DDNS_LOG_LEVEL", "info"), "日志级别 debug/info/warn/error (CFST_DDNS_LOG_LEVEL)")
	fs.StringVar(&c.BundleDir, "bundle-dir", env("CFST_DDNS_BUNDLE_DIR", ""), "预置 cfst 目录，首次启动时复制 (CFST_DDNS_BUNDLE_DIR)")
	fs.BoolVar(&c.AutoInstall, "auto-install", env("CFST_DDNS_AUTO_INSTALL", "true") == "true", "未安装 cfst 时自动下载 (CFST_DDNS_AUTO_INSTALL)")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	c.Secret = os.Getenv("CFST_DDNS_SECRET")
	c.AdminUser = os.Getenv("ADMIN_USERNAME")
	c.AdminPass = os.Getenv("ADMIN_PASSWORD")
	abs, err := filepath.Abs(c.DataDir)
	if err != nil {
		return c, err
	}
	c.DataDir = abs
	return c, nil
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	}
	return slog.LevelInfo
}

// OpenStore 打开数据目录下的数据库（serve 与 reset-password 共用）。
func OpenStore(c Config) (*store.Store, error) {
	if err := os.MkdirAll(c.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	box, err := secret.LoadOrCreate(c.Secret, filepath.Join(c.DataDir, "secret.key"))
	if err != nil {
		return nil, err
	}
	return store.Open(filepath.Join(c.DataDir, "cfst-ddns.db"), box)
}

// Serve 启动服务直到收到退出信号。
func Serve(c Config, build api.BuildInfo, static fs.FS) error {
	ring := logbus.NewRing(2000)
	log := logbus.NewLogger(os.Stdout, ring, parseLevel(c.LogLevel))
	slog.SetDefault(log)
	httpx.UserAgent = "cfst-ddns/" + build.Version

	st, err := OpenStore(c)
	if err != nil {
		return err
	}
	defer st.Close()

	authSvc, err := auth.New(st)
	if err != nil {
		return err
	}
	if !st.Initialized() && c.AdminUser != "" && c.AdminPass != "" {
		if _, err := authSvc.Setup(c.AdminUser, c.AdminPass); err != nil {
			return fmt.Errorf("通过环境变量创建管理员失败: %w", err)
		}
		log.Info("已通过环境变量创建管理员", "username", c.AdminUser)
	}

	mgr := &cfst.Manager{Dir: filepath.Join(c.DataDir, "cfst"), BundleDir: c.BundleDir, Log: log,
		Mirror: func() string { return st.GetSettings().GithubMirror },
		Busy: func() bool {
			var n int64
			st.DB.Model(&store.Run{}).Where("status IN ?", []string{store.StatusQueued, store.StatusRunning}).Count(&n)
			return n > 0
		}}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	prepareCFST(ctx, c, mgr, log)

	hub := logbus.NewHub()
	eng := engine.New(st, mgr, hub, log, filepath.Join(c.DataDir, "tmp"))
	eng.Start(ctx)
	sch := scheduler.New(st, eng, log)
	if err := sch.Start(); err != nil {
		return err
	}
	defer sch.Stop()

	srv := &api.Server{Store: st, Auth: authSvc, Engine: eng, Scheduler: sch, CFST: mgr, Hub: hub, AppLog: ring,
		Log: log, Static: static, DataDir: c.DataDir, Build: build, StartedAt: time.Now()}
	hs := &http.Server{Addr: c.Listen, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}

	errCh := make(chan error, 1)
	go func() { errCh <- hs.ListenAndServe() }()
	log.Info("cfst-ddns 已启动", "version", build.Version, "listen", c.Listen, "data", c.DataDir)
	if static == nil {
		log.Warn("前端资源未嵌入，仅提供 API（请在 web/ 执行 pnpm build 后重新编译）")
	}
	if !st.Initialized() {
		log.Info("尚未创建管理员，请在浏览器中打开页面完成初始化")
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP 服务异常: %w", err)
		}
	case <-ctx.Done():
		log.Info("正在停止...")
	}
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return hs.Shutdown(sctx)
}

// prepareCFST 在首次启动时从预置目录复制或后台自动下载 cfst。
func prepareCFST(ctx context.Context, c Config, mgr *cfst.Manager, log *slog.Logger) {
	if mgr.Installed() {
		return
	}
	if c.BundleDir != "" {
		if err := mgr.InstallFromDir(c.BundleDir); err != nil {
			log.Warn("从预置目录安装 cfst 失败", "dir", c.BundleDir, "err", err)
		} else {
			log.Info("已从预置目录安装 cfst", "version", mgr.Version())
			return
		}
	}
	if !c.AutoInstall {
		log.Warn("cfst 未安装，请在「cfst 管理」页面安装")
		return
	}
	go func() {
		ictx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		if _, err := mgr.Install(ictx, "latest"); err != nil {
			log.Warn("自动安装 cfst 失败，可在「cfst 管理」页面重试（国内网络请先配置 GitHub 镜像）", "err", err)
		}
	}()
}
