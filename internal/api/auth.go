package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lonelyman0108/cfst-ddns/internal/i18n"
	"github.com/lonelyman0108/cfst-ddns/internal/notify"
	"github.com/lonelyman0108/cfst-ddns/internal/provider"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
	"github.com/lonelyman0108/cfst-ddns/internal/update"
)

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) authStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"initialized": s.Store.Initialized()})
}

func (s *Server) authSetup(c *gin.Context) {
	var req credentials
	if !bind(c, &req) {
		return
	}
	tok, err := s.Auth.Setup(req.Username, req.Password)
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	s.Log.Info("管理员账号已创建", "username", tok.Username)
	c.JSON(http.StatusOK, tok)
}

func (s *Server) authLogin(c *gin.Context) {
	var req credentials
	if !bind(c, &req) {
		return
	}
	tok, err := s.Auth.Login(c.ClientIP(), req.Username, req.Password)
	if err != nil {
		s.Log.Warn("登录失败", "ip", c.ClientIP(), "username", req.Username)
		fail(c, http.StatusUnauthorized, err)
		return
	}
	c.JSON(http.StatusOK, tok)
}

func (s *Server) authMe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"username": c.GetString("user")})
}

// authPassword 修改密码；旧令牌全部失效，响应中返回新令牌。
func (s *Server) authPassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if !bind(c, &req) {
		return
	}
	tok, err := s.Auth.ChangePassword(req.OldPassword, req.NewPassword)
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	s.Log.Info("管理员密码已修改")
	c.JSON(http.StatusOK, gin.H{"ok": true, "token": tok.Token, "expiresAt": tok.ExpiresAt, "username": tok.Username})
}

func (s *Server) metaProviders(c *gin.Context) {
	c.JSON(http.StatusOK, localizeMetas(lang(c), provider.Metas()))
}
func (s *Server) metaNotifiers(c *gin.Context) {
	c.JSON(http.StatusOK, localizeMetas(lang(c), notify.Metas()))
}

// localizeMetas 返回按语言翻译后的元数据副本，不修改注册表。
func localizeMetas(l i18n.Lang, metas []schema.TypeMeta) []schema.TypeMeta {
	out := make([]schema.TypeMeta, len(metas))
	for i, m := range metas {
		out[i] = m.Localize(l)
	}
	return out
}

// timezoneName 返回可读的时区，如 "Asia/Shanghai (UTC+08:00)"。
// time.Local 的名字恒为 "Local"，需从 TZ 或 /etc/localtime 的链接目标推断真实名称。
func timezoneName() string {
	name := os.Getenv("TZ")
	if name == "" {
		name = time.Local.String()
	}
	if name == "Local" {
		name = ""
		if p, err := filepath.EvalSymlinks("/etc/localtime"); err == nil {
			if i := strings.Index(p, "zoneinfo/"); i >= 0 {
				name = p[i+len("zoneinfo/"):]
			}
		}
	}
	abbr, off := time.Now().Zone()
	if name == "" {
		name = abbr
	}
	sign := '+'
	if off < 0 {
		sign, off = '-', -off
	}
	return fmt.Sprintf("%s (UTC%c%02d:%02d)", name, sign, off/3600, off%3600/60)
}

func (s *Server) systemInfo(c *gin.Context) {
	tz := timezoneName()
	c.JSON(http.StatusOK, gin.H{
		"version": s.Build.Version, "commit": s.Build.Commit, "buildTime": s.Build.BuildTime,
		"buildId": s.Build.BuildID, "runId": s.Build.RunID, "repoUrl": update.RepoURL,
		"goVersion": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH,
		"dataDir": s.DataDir, "startedAt": s.StartedAt, "timezone": tz,
	})
}

// systemUpdate 返回新版本信息。force=1 时立即检查；关闭自动检查后只返回已有结果。
func (s *Server) systemUpdate(c *gin.Context) {
	force := c.Query("force") == "1"
	var info update.Info
	if !force && !s.Store.GetSettings().UpdateCheck {
		info = s.Update.Cached()
	} else {
		// 不跟随请求取消，避免把"客户端断开"缓存成检查失败
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		info = s.Update.Check(ctx, force)
	}
	if info.Err != nil {
		info.Error = i18n.Localize(lang(c), info.Err)
	}
	c.JSON(http.StatusOK, info)
}

func (s *Server) systemLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "500"))
	c.JSON(http.StatusOK, gin.H{"lines": s.AppLog.Tail(limit)})
}
