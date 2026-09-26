package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lonelyman0108/cfst-ddns/internal/i18n"
	"github.com/lonelyman0108/cfst-ddns/internal/legacy"
	"github.com/lonelyman0108/cfst-ddns/internal/notify"
	"github.com/lonelyman0108/cfst-ddns/internal/provider"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

type legacyAccount struct {
	Name     string        `json:"name"`
	Provider string        `json:"provider"`
	Config   schema.Config `json:"config"`
}

type legacyNotifier struct {
	Name         string        `json:"name"`
	Type         string        `json:"type"`
	Config       schema.Config `json:"config"`
	OnSuccess    bool          `json:"onSuccess"`
	OnFailure    bool          `json:"onFailure"`
	OnChangeOnly bool          `json:"onChangeOnly"`
}

type legacyTask struct {
	Name    string `json:"name"`
	Cron    string `json:"cron"`
	IPType  string `json:"ipType"`
	Summary string `json:"summary"`
}

type legacyPreview struct {
	Accounts  []legacyAccount  `json:"accounts"`
	Notifiers []legacyNotifier `json:"notifiers"`
	Tasks     []legacyTask     `json:"tasks"`
	Settings  struct {
		GithubMirror string `json:"githubMirror,omitempty"`
	} `json:"settings"`
	Warnings []string `json:"warnings"`
}

// legacyPlan 解析 v1 配置并用 Schema 规范化、校验账号与渠道配置；导入后的默认名称按请求语言翻译。
func legacyPlan(content string, l i18n.Lang) *legacy.Plan {
	p := legacy.Parse(content, DefaultTask().SpeedTest)
	if a := p.Account; a != nil {
		a.Name = i18n.T(l, a.Name)
		meta, _ := provider.Meta(a.Provider)
		a.Config = meta.ApplyDefaults(meta.Clean(a.Config))
		if err := meta.Validate(a.Config); err != nil {
			p.Warnings = append(p.Warnings, i18n.M("DNS 账号配置无效（%v），未导入账号与任务", err))
			p.Account, p.Task = nil, nil
		}
	}
	if t := p.Task; t != nil {
		t.Name = i18n.T(l, t.Name)
	}
	valid := p.Notifiers[:0]
	for _, n := range p.Notifiers {
		n.Name = i18n.T(l, n.Name)
		meta, _ := notify.Meta(n.Type)
		n.Config = meta.ApplyDefaults(meta.Clean(n.Config))
		if err := meta.Validate(n.Config); err != nil {
			p.Warnings = append(p.Warnings, i18n.M("%s 配置无效（%v），未导入", n.Name, err))
			continue
		}
		valid = append(valid, n)
	}
	p.Notifiers = valid
	return p
}

// localizeWarnings 按语言输出警告列表，始终返回非 nil 切片。
func localizeWarnings(l i18n.Lang, warns []*i18n.Msg) []string {
	out := make([]string, 0, len(warns))
	for _, w := range warns {
		out = append(out, w.Localize(l))
	}
	return out
}

func previewOf(p *legacy.Plan, l i18n.Lang) legacyPreview {
	out := legacyPreview{Accounts: []legacyAccount{}, Notifiers: []legacyNotifier{}, Tasks: []legacyTask{}, Warnings: localizeWarnings(l, p.Warnings)}
	if a := p.Account; a != nil {
		meta, _ := provider.Meta(a.Provider)
		out.Accounts = append(out.Accounts, legacyAccount{a.Name, a.Provider, meta.Masked(a.Config)})
	}
	for _, n := range p.Notifiers {
		meta, _ := notify.Meta(n.Type)
		out.Notifiers = append(out.Notifiers, legacyNotifier{n.Name, n.Type, meta.Masked(n.Config), n.OnSuccess, n.OnFailure, n.OnlyOnChange})
	}
	if t := p.Task; t != nil {
		out.Tasks = append(out.Tasks, legacyTask{t.Name, t.Cron, t.IPType, t.Summary().Localize(l)})
	}
	out.Settings.GithubMirror = p.GithubMirror
	return out
}

// importLegacy 预览或导入 v1 配置。
func (s *Server) importLegacy(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
		Apply   bool   `json:"apply"`
	}
	if !bind(c, &req) {
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		failMsg(c, http.StatusBadRequest, "请粘贴 v1 的配置内容")
		return
	}
	l := lang(c)
	p := legacyPlan(req.Content, l)
	if !req.Apply {
		c.JSON(http.StatusOK, previewOf(p, l))
		return
	}

	type counts struct {
		Accounts  int `json:"accounts"`
		Notifiers int `json:"notifiers"`
		Tasks     int `json:"tasks"`
	}
	var created counts
	warnings := append([]*i18n.Msg{}, p.Warnings...)

	// 先完成网络请求（拆分目标记录），再在一个事务中写入全部数据
	var targets []store.Target
	if p.Account != nil && p.Task != nil {
		targets = legacyTargets(c.Request.Context(), p.Account, p.Task.Records, &warnings)
	}
	var task *store.Task
	err := s.Store.DB.Transaction(func(tx *gorm.DB) error {
		txs := s.Store.WithDB(tx)
		var account *store.Account
		if a := p.Account; a != nil {
			account = &store.Account{Name: a.Name, Provider: a.Provider, Config: a.Config, Remark: i18n.T(l, "从 v1 配置导入")}
			if err := txs.SaveAccount(account); err != nil {
				return err
			}
			created.Accounts++
		}
		notifierIDs := []uint{}
		for _, n := range p.Notifiers {
			m := store.Notifier{Name: n.Name, Type: n.Type, Enabled: true, Config: n.Config,
				OnSuccess: n.OnSuccess, OnFailure: n.OnFailure, OnlyOnChange: n.OnlyOnChange}
			if err := txs.SaveNotifier(&m); err != nil {
				return err
			}
			notifierIDs = append(notifierIDs, m.ID)
			created.Notifiers++
		}
		if p.GithubMirror != "" {
			st := txs.GetSettings()
			st.GithubMirror = p.GithubMirror
			if err := txs.SaveSettings(st); err != nil {
				return err
			}
		}
		if t := p.Task; t != nil && account != nil {
			nt := DefaultTask()
			nt.Name, nt.Cron, nt.IPType, nt.SpeedTest, nt.NotifierIDs = t.Name, t.Cron, t.IPType, t.SpeedTest, notifierIDs
			for _, tg := range targets {
				tg.AccountID = account.ID
				nt.Targets = append(nt.Targets, tg)
			}
			if len(nt.Targets) > 0 {
				if err := normalizeTaskIn(txs, &nt); err != nil {
					warnings = append(warnings, i18n.M("任务校验失败（%v），已创建为停用状态，请编辑后启用", err))
					nt.Enabled = false
				}
			} else {
				// 没有目标记录的任务无法执行，先停用，等用户补充后再启用
				nt.Enabled = false
				warnings = append(warnings, i18n.M("任务已创建但未启用：没有可用的目标记录，请编辑任务补充后启用"))
			}
			if err := tx.Create(&nt).Error; err != nil {
				return err
			}
			task = &nt
			created.Tasks++
		}
		return nil
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, i18n.Errorf("导入失败，数据未改动: %w", err))
		return
	}
	if task != nil {
		s.Scheduler.Sync(task)
	}
	s.Log.Info("已导入 v1 配置", "accounts", created.Accounts, "notifiers", created.Notifiers, "tasks", created.Tasks)
	c.JSON(http.StatusOK, gin.H{"created": created, "warnings": localizeWarnings(l, warnings)})
}

// legacyTargets 用账号的域名列表拆分完整域名；拆不出的记录进入 warnings，不猜测主域名。
// 返回的目标记录尚未填写 AccountID。
func legacyTargets(ctx context.Context, a *legacy.Account, records []string, warnings *[]*i18n.Msg) []store.Target {
	targets := []store.Target{}
	if len(records) == 0 {
		return targets
	}
	ttl := 600
	if a.Provider == "cloudflare" {
		ttl = 1 // 自动，与 v1 一致
	}
	p, err := provider.New(a.Provider, a.Config)
	var domains []string
	if err == nil {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		domains, err = p.ListDomains(cctx)
		cancel()
	}
	if err != nil {
		*warnings = append(*warnings, i18n.M("获取「%s」的域名列表失败（%v），以下记录未加入任务: %s", a.Name, err, strings.Join(records, "、")))
		return targets
	}
	for _, r := range records {
		domain, rr, ok := legacy.SplitRecord(r, domains)
		if !ok {
			*warnings = append(*warnings, i18n.M("账号「%s」下找不到 %s 所属的主域名，未加入任务", a.Name, r))
			continue
		}
		targets = append(targets, store.Target{Domain: domain, RR: rr, TTL: ttl})
	}
	return targets
}
