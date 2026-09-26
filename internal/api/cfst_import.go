package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lonelyman0108/cfst-ddns/internal/cfst"
)

// importFail 按错误类型返回状态码。
func (s *Server) importFail(c *gin.Context, err error) {
	var bad *cfst.BadFileError
	switch {
	case errors.As(err, &bad):
		fail(c, http.StatusBadRequest, err)
	case errors.Is(err, cfst.ErrInstalling), errors.Is(err, cfst.ErrTaskActive):
		fail(c, http.StatusConflict, err)
	default:
		s.Log.Error("导入 cfst 失败", "err", err)
		fail(c, http.StatusInternalServerError, err)
	}
}

func (s *Server) cfstUpload(c *gin.Context) {
	// 预留 1MB 给 multipart 边界与其他字段
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfst.MaxUploadSize+1<<20)
	fh, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			failMsg(c, http.StatusRequestEntityTooLarge, "文件超过 64MB")
			return
		}
		failMsg(c, http.StatusBadRequest, "请选择要上传的文件")
		return
	}
	if fh.Size > cfst.MaxUploadSize {
		failMsg(c, http.StatusRequestEntityTooLarge, "文件超过 64MB")
		return
	}
	f, err := fh.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, cfst.MaxUploadSize))
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Minute)
	defer cancel()
	res, err := s.CFST.Import(ctx, fh.Filename, data, c.PostForm("version"))
	if err != nil {
		s.importFail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (s *Server) cfstScan(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	c.JSON(http.StatusOK, s.CFST.Scan(ctx))
}

func (s *Server) cfstAdopt(c *gin.Context) {
	var req struct {
		Path    string `json:"path"`
		Version string `json:"version"`
	}
	if !bind(c, &req) {
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		failMsg(c, http.StatusBadRequest, "path 不能为空")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Minute)
	defer cancel()
	res, err := s.CFST.Adopt(ctx, strings.TrimSpace(req.Path), req.Version)
	if err != nil {
		s.importFail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// ---------- GitHub 镜像 ----------

func (s *Server) cfstMirrors(c *gin.Context) { c.JSON(http.StatusOK, cfst.MirrorPresets) }

func (s *Server) cfstMirrorTest(c *gin.Context) {
	var req struct {
		Mirrors []string `json:"mirrors"`
	}
	if !bindOptional(c, &req) {
		return
	}
	list := req.Mirrors
	if list == nil {
		for _, p := range cfst.MirrorPresets {
			list = append(list, p.Mirror)
		}
		list = append(list, s.Store.GetSettings().GithubMirror)
	}
	var mirrors []string
	seen := map[string]bool{}
	for _, m := range list {
		m = strings.TrimRight(strings.TrimSpace(m), "/")
		if !seen[m] {
			seen[m] = true
			mirrors = append(mirrors, m)
		}
	}
	if len(mirrors) > 20 {
		failMsg(c, http.StatusBadRequest, "一次最多测试 20 个镜像")
		return
	}
	c.JSON(http.StatusOK, cfst.ProbeMirrors(c.Request.Context(), mirrors))
}
