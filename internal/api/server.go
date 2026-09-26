// Package api 提供 REST API、SSE 与前端静态资源。
package api

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lonelyman0108/cfst-ddns/internal/auth"
	"github.com/lonelyman0108/cfst-ddns/internal/cfst"
	"github.com/lonelyman0108/cfst-ddns/internal/engine"
	"github.com/lonelyman0108/cfst-ddns/internal/logbus"
	"github.com/lonelyman0108/cfst-ddns/internal/scheduler"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

// BuildInfo 为版本信息。
type BuildInfo struct {
	Version   string
	Commit    string
	BuildTime string
}

// Server 聚合 API 依赖。
type Server struct {
	Store     *store.Store
	Auth      *auth.Service
	Engine    *engine.Engine
	Scheduler *scheduler.Scheduler
	CFST      *cfst.Manager
	Hub       *logbus.Hub
	AppLog    *logbus.Ring
	Log       *slog.Logger
	Static    fs.FS // 前端构建产物，可为 nil
	DataDir   string
	Build     BuildInfo
	StartedAt time.Time
}

// Handler 构建 HTTP 处理器。
func (s *Server) Handler() http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), s.accessLog())

	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	pub := r.Group("/api")
	pub.GET("/auth/status", s.authStatus)
	pub.POST("/auth/setup", s.authSetup)
	pub.POST("/auth/login", s.authLogin)
	pub.Any("/hooks/tasks/:id/run", s.hookRun)
	pub.GET("/runs/:id/stream", s.requireAuth(true), s.runStream)

	a := r.Group("/api", s.requireAuth(false))
	a.GET("/auth/me", s.authMe)
	a.POST("/auth/password", s.authPassword)

	a.GET("/meta/providers", s.metaProviders)
	a.GET("/meta/notifiers", s.metaNotifiers)
	a.GET("/system/info", s.systemInfo)
	a.GET("/system/logs", s.systemLogs)

	a.GET("/accounts", s.listAccounts)
	a.POST("/accounts", s.createAccount)
	a.POST("/accounts/test", s.testAccountConfig)
	a.PUT("/accounts/:id", s.updateAccount)
	a.DELETE("/accounts/:id", s.deleteAccount)
	a.POST("/accounts/:id/test", s.testAccount)
	a.GET("/accounts/:id/domains", s.accountDomains)
	a.GET("/accounts/:id/records", s.accountRecords)

	a.GET("/notifiers", s.listNotifiers)
	a.POST("/notifiers", s.createNotifier)
	a.POST("/notifiers/test", s.testNotifierConfig)
	a.PUT("/notifiers/:id", s.updateNotifier)
	a.DELETE("/notifiers/:id", s.deleteNotifier)
	a.POST("/notifiers/:id/test", s.testNotifier)

	a.GET("/tasks", s.listTasks)
	a.GET("/tasks/defaults", s.taskDefaults)
	a.GET("/tasks/:id", s.getTask)
	a.POST("/tasks", s.createTask)
	a.PUT("/tasks/:id", s.updateTask)
	a.PATCH("/tasks/:id/enabled", s.setTaskEnabled)
	a.DELETE("/tasks/:id", s.deleteTask)
	a.POST("/tasks/:id/clone", s.cloneTask)
	a.POST("/tasks/:id/run", s.runTask)
	a.GET("/cron/preview", s.cronPreview)

	a.GET("/runs", s.listRuns)
	a.GET("/runs/active", s.activeRuns)
	a.GET("/runs/:id", s.getRun)
	a.POST("/runs/:id/cancel", s.cancelRun)
	a.DELETE("/runs/:id", s.deleteRun)
	a.DELETE("/runs", s.purgeRuns)

	a.GET("/dashboard", s.dashboard)

	a.GET("/cfst", s.cfstStatus)
	a.GET("/cfst/releases", s.cfstReleases)
	a.POST("/cfst/install", s.cfstInstall)
	a.POST("/cfst/upload", s.cfstUpload)
	a.GET("/cfst/scan", s.cfstScan)
	a.POST("/cfst/adopt", s.cfstAdopt)
	a.GET("/cfst/mirrors", s.cfstMirrors)
	a.POST("/cfst/mirrors/test", s.cfstMirrorTest)
	a.GET("/cfst/ipfile/:kind", s.getIPFile)
	a.PUT("/cfst/ipfile/:kind", s.putIPFile)
	a.POST("/cfst/ipfile/:kind/reset", s.resetIPFile)

	a.GET("/settings", s.getSettings)
	a.PUT("/settings", s.putSettings)
	a.POST("/settings/hook-token", s.regenHookToken)
	a.GET("/backup", s.backup)
	a.POST("/backup/restore", s.restore)
	a.POST("/import/legacy", s.importLegacy)

	r.NoRoute(s.static)
	return r
}

func (s *Server) accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		p := c.Request.URL.Path
		if !strings.HasPrefix(p, "/api/") || strings.HasSuffix(p, "/stream") {
			return
		}
		st := c.Writer.Status()
		if st >= 400 || c.Request.Method != http.MethodGet {
			s.Log.Debug("HTTP", "method", c.Request.Method, "path", p, "status", st, "cost", time.Since(start).Round(time.Millisecond))
		}
	}
}

// requireAuth 校验 Bearer 令牌；allowQuery 允许通过 ?token= 传递（SSE）。
func (s *Server) requireAuth(allowQuery bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		tok := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if tok == "" && allowQuery {
			tok = c.Query("token")
		}
		if tok == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			return
		}
		user, err := s.Auth.Verify(tok)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.Set("user", user)
		c.Next()
	}
}

// static 提供前端资源，未知路径回退 index.html（SPA history 模式）。
func (s *Server) static(c *gin.Context) {
	p := c.Request.URL.Path
	if strings.HasPrefix(p, "/api/") || c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
		return
	}
	if s.Static == nil {
		c.String(http.StatusNotFound, "前端未构建")
		return
	}
	name := strings.TrimPrefix(path.Clean(p), "/")
	if name != "" {
		if st, err := fs.Stat(s.Static, name); err == nil && !st.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			}
			c.FileFromFS(name, http.FS(s.Static))
			return
		}
	}
	b, err := fs.ReadFile(s.Static, "index.html")
	if err != nil {
		c.String(http.StatusNotFound, "前端未构建：请先在 web/ 目录执行 pnpm build 后重新编译")
		return
	}
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", b)
}

// ---------- 通用工具 ----------

func fail(c *gin.Context, code int, err error) {
	c.JSON(code, gin.H{"error": err.Error()})
}

func failMsg(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"error": msg})
}

func okJSON(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) }

// dbFail 把数据库错误转为合适的状态码。
func dbFail(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		failMsg(c, http.StatusNotFound, "记录不存在")
		return
	}
	fail(c, http.StatusInternalServerError, err)
}

func idParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		failMsg(c, http.StatusBadRequest, "无效的 ID")
		return 0, false
	}
	return uint(id), true
}

// bindOptional 解析可省略的 JSON 请求体，空请求体视为全部默认值。
func bindOptional(c *gin.Context, v any) bool {
	if err := json.NewDecoder(c.Request.Body).Decode(v); err != nil && !errors.Is(err, io.EOF) {
		failMsg(c, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return false
	}
	return true
}

func bind(c *gin.Context, v any) bool {
	if err := c.ShouldBindJSON(v); err != nil {
		failMsg(c, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return false
	}
	return true
}
