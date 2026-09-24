package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lonelyman0108/cfst-ddns/internal/engine"
	"github.com/lonelyman0108/cfst-ddns/internal/notify"
	"github.com/lonelyman0108/cfst-ddns/internal/provider"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

type accountView struct {
	store.Account
	TaskCount int `json:"taskCount"`
}

// accountUsage 统计每个账号被多少任务引用。
func (s *Server) accountUsage() map[uint]int {
	out := map[uint]int{}
	tasks, _ := s.Store.ListTasks()
	for _, t := range tasks {
		seen := map[uint]bool{}
		for _, tg := range t.Targets {
			if !seen[tg.AccountID] {
				seen[tg.AccountID] = true
				out[tg.AccountID]++
			}
		}
	}
	return out
}

func maskAccount(a store.Account, usage int) accountView {
	if meta, ok := provider.Meta(a.Provider); ok {
		a.Config = meta.Masked(a.Config)
	}
	return accountView{Account: a, TaskCount: usage}
}

func (s *Server) listAccounts(c *gin.Context) {
	list, err := s.Store.ListAccounts()
	if err != nil {
		dbFail(c, err)
		return
	}
	usage := s.accountUsage()
	out := make([]accountView, 0, len(list))
	for _, a := range list {
		out = append(out, maskAccount(a, usage[a.ID]))
	}
	c.JSON(http.StatusOK, out)
}

type accountReq struct {
	ID       uint          `json:"id"` // 仅测试接口使用：编辑已保存账号时用于补齐被掩码的密钥
	Name     string        `json:"name"`
	Provider string        `json:"provider"`
	Config   schema.Config `json:"config"`
	Remark   string        `json:"remark"`
}

func (s *Server) createAccount(c *gin.Context) {
	var req accountReq
	if !bind(c, &req) {
		return
	}
	meta, ok := provider.Meta(req.Provider)
	if !ok {
		failMsg(c, http.StatusBadRequest, "不支持的 DNS 服务商")
		return
	}
	a := store.Account{Name: strings.TrimSpace(req.Name), Provider: req.Provider,
		Config: meta.ApplyDefaults(meta.Clean(req.Config)), Remark: req.Remark}
	if a.Name == "" {
		a.Name = meta.Name
	}
	if err := meta.Validate(a.Config); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if err := s.Store.SaveAccount(&a); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, maskAccount(a, 0))
}

func (s *Server) updateAccount(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req accountReq
	if !bind(c, &req) {
		return
	}
	a, err := s.Store.GetAccount(id)
	if err != nil {
		dbFail(c, err)
		return
	}
	meta, _ := provider.Meta(a.Provider)
	a.Config = meta.ApplyDefaults(meta.Merge(a.Config, req.Config))
	if err := meta.Validate(a.Config); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if n := strings.TrimSpace(req.Name); n != "" {
		a.Name = n
	}
	a.Remark = req.Remark
	if err := s.Store.SaveAccount(a); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, maskAccount(*a, s.accountUsage()[a.ID]))
}

func (s *Server) deleteAccount(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if n := s.accountUsage()[id]; n > 0 {
		failMsg(c, http.StatusConflict, "该账号正被任务使用，请先修改相关任务")
		return
	}
	if err := s.Store.DB.Delete(&store.Account{}, id).Error; err != nil {
		dbFail(c, err)
		return
	}
	okJSON(c)
}

func testResult(c *gin.Context, err error, success string) {
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": success})
}

func testProvider(p provider.Provider) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := p.Test(ctx); err != nil {
		return "", err
	}
	domains, err := p.ListDomains(ctx)
	if err != nil {
		return "凭据有效（无法列出域名: " + err.Error() + "）", nil
	}
	if len(domains) == 0 {
		return "凭据有效，但账号下没有域名", nil
	}
	shown := domains
	if len(shown) > 5 {
		shown = shown[:5]
	}
	msg := "连接成功，共 " + strconv.Itoa(len(domains)) + " 个域名：" + strings.Join(shown, ", ")
	if len(domains) > 5 {
		msg += " …"
	}
	return msg, nil
}

func (s *Server) testAccountConfig(c *gin.Context) {
	var req accountReq
	if !bind(c, &req) {
		return
	}
	cfg := req.Config
	if req.ID > 0 {
		a, err := s.Store.GetAccount(req.ID)
		if err != nil {
			dbFail(c, err)
			return
		}
		req.Provider = a.Provider
		meta, _ := provider.Meta(a.Provider)
		cfg = meta.Merge(a.Config, req.Config)
	}
	p, err := provider.New(req.Provider, cfg)
	if err != nil {
		testResult(c, err, "")
		return
	}
	msg, err := testProvider(p)
	testResult(c, err, msg)
}

func (s *Server) accountProvider(c *gin.Context) (provider.Provider, bool) {
	id, ok := idParam(c)
	if !ok {
		return nil, false
	}
	a, err := s.Store.GetAccount(id)
	if err != nil {
		dbFail(c, err)
		return nil, false
	}
	p, err := provider.New(a.Provider, a.Config)
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return nil, false
	}
	return p, true
}

func (s *Server) testAccount(c *gin.Context) {
	p, ok := s.accountProvider(c)
	if !ok {
		return
	}
	msg, err := testProvider(p)
	testResult(c, err, msg)
}

func (s *Server) accountDomains(c *gin.Context) {
	p, ok := s.accountProvider(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	list, err := p.ListDomains(ctx)
	if err != nil {
		fail(c, http.StatusBadGateway, err)
		return
	}
	if list == nil {
		list = []string{}
	}
	c.JSON(http.StatusOK, list)
}

func (s *Server) accountRecords(c *gin.Context) {
	p, ok := s.accountProvider(c)
	if !ok {
		return
	}
	domain := strings.TrimSpace(c.Query("domain"))
	if domain == "" {
		failMsg(c, http.StatusBadRequest, "domain 不能为空")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	list, err := p.ListRecords(ctx, domain, strings.TrimSpace(c.Query("rr")), strings.ToUpper(c.Query("type")))
	if err != nil {
		fail(c, http.StatusBadGateway, err)
		return
	}
	if list == nil {
		list = []provider.Record{}
	}
	c.JSON(http.StatusOK, list)
}

// ---------- 通知渠道 ----------

func maskNotifier(n store.Notifier) store.Notifier {
	if meta, ok := notify.Meta(n.Type); ok {
		n.Config = meta.Masked(n.Config)
	}
	return n
}

func (s *Server) listNotifiers(c *gin.Context) {
	list, err := s.Store.ListNotifiers()
	if err != nil {
		dbFail(c, err)
		return
	}
	for i := range list {
		list[i] = maskNotifier(list[i])
	}
	c.JSON(http.StatusOK, list)
}

type notifierReq struct {
	ID           uint          `json:"id"` // 仅测试接口使用：编辑已保存渠道时用于补齐被掩码的密钥
	Name         string        `json:"name"`
	Type         string        `json:"type"`
	Enabled      bool          `json:"enabled"`
	Config       schema.Config `json:"config"`
	OnSuccess    bool          `json:"onSuccess"`
	OnFailure    bool          `json:"onFailure"`
	OnlyOnChange bool          `json:"onlyOnChange"`
}

func (s *Server) createNotifier(c *gin.Context) {
	var req notifierReq
	if !bind(c, &req) {
		return
	}
	meta, ok := notify.Meta(req.Type)
	if !ok {
		failMsg(c, http.StatusBadRequest, "不支持的通知渠道")
		return
	}
	n := store.Notifier{Name: strings.TrimSpace(req.Name), Type: req.Type, Enabled: req.Enabled,
		Config: meta.ApplyDefaults(meta.Clean(req.Config)), OnSuccess: req.OnSuccess, OnFailure: req.OnFailure, OnlyOnChange: req.OnlyOnChange}
	if n.Name == "" {
		n.Name = meta.Name
	}
	if err := meta.Validate(n.Config); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if err := s.Store.SaveNotifier(&n); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, maskNotifier(n))
}

func (s *Server) updateNotifier(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req notifierReq
	if !bind(c, &req) {
		return
	}
	n, err := s.Store.GetNotifier(id)
	if err != nil {
		dbFail(c, err)
		return
	}
	meta, _ := notify.Meta(n.Type)
	n.Config = meta.ApplyDefaults(meta.Merge(n.Config, req.Config))
	if err := meta.Validate(n.Config); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		n.Name = name
	}
	n.Enabled, n.OnSuccess, n.OnFailure, n.OnlyOnChange = req.Enabled, req.OnSuccess, req.OnFailure, req.OnlyOnChange
	if err := s.Store.SaveNotifier(n); err != nil {
		dbFail(c, err)
		return
	}
	c.JSON(http.StatusOK, maskNotifier(*n))
}

func (s *Server) deleteNotifier(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.Store.DB.Delete(&store.Notifier{}, id).Error; err != nil {
		dbFail(c, err)
		return
	}
	// 从任务中移除引用
	tasks, _ := s.Store.ListTasks()
	for _, t := range tasks {
		var ids []uint
		for _, nid := range t.NotifierIDs {
			if nid != id {
				ids = append(ids, nid)
			}
		}
		if len(ids) != len(t.NotifierIDs) {
			t.NotifierIDs = ids
			s.Store.DB.Save(&t)
		}
	}
	okJSON(c)
}

func testMessage(prefix string) notify.Message {
	now := time.Now().Format("2006-01-02 15:04:05")
	content := "这是一条来自 cfst-ddns 的测试通知。\n如果你看到这条消息，说明通知渠道配置正确。\n\n时间: " + now
	return notify.Message{Title: strings.TrimSpace(prefix + " 测试通知"), Content: content,
		Markdown: "这是一条来自 **cfst-ddns** 的测试通知。\n\n如果你看到这条消息，说明通知渠道配置正确。\n\n时间: " + now, Success: true}
}

func (s *Server) testNotifierConfig(c *gin.Context) {
	var req notifierReq
	if !bind(c, &req) {
		return
	}
	n := &store.Notifier{Type: req.Type, Config: req.Config}
	if req.ID > 0 {
		saved, err := s.Store.GetNotifier(req.ID)
		if err != nil {
			dbFail(c, err)
			return
		}
		meta, _ := notify.Meta(saved.Type)
		n = &store.Notifier{Type: saved.Type, Config: meta.Merge(saved.Config, req.Config)}
	}
	err := engine.Send(n, testMessage(s.Store.GetSettings().NotifyTitlePrefix))
	testResult(c, err, "发送成功")
}

func (s *Server) testNotifier(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	n, err := s.Store.GetNotifier(id)
	if err != nil {
		dbFail(c, err)
		return
	}
	err = engine.Send(n, testMessage(s.Store.GetSettings().NotifyTitlePrefix))
	testResult(c, err, "发送成功")
}
