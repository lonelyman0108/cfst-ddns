package engine

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lonelyman0108/cfst-ddns/internal/notify"
	"github.com/lonelyman0108/cfst-ddns/internal/update"
)

// keyUpdateNotified 记录已推送过的最新版本，同一版本只提醒一次。
const keyUpdateNotified = "update_notified_version"

const maxNotesRunes = 500

var (
	mdLinkRe    = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	blankLineRe = regexp.MustCompile(`\n{3,}`)
)

// cleanNotes 整理 Release 正文：去掉开头的版本标题与末尾的 Docker 用法，
// plain 为 true 时把 Markdown 链接还原为文字，便于纯文本渠道阅读。
func cleanNotes(s string, plain bool) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if i := strings.Index(s, "## Docker"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "## ") {
		_, s, _ = strings.Cut(s, "\n")
	}
	if plain {
		s = mdLinkRe.ReplaceAllString(s, "$1")
	}
	s = blankLineRe.ReplaceAllString(strings.TrimSpace(s), "\n\n")
	return truncateRunes(s, maxNotesRunes)
}

// BuildUpdateMessage 生成新版本提醒。
func BuildUpdateMessage(prefix string, info update.Info) notify.Message {
	title := strings.TrimSpace(fmt.Sprintf("%s 发现新版本 %s", strings.TrimSpace(prefix), info.Latest))
	var txt, md strings.Builder
	txt.WriteString("当前版本: " + info.Current + "\n最新版本: " + info.Latest + "\n")
	md.WriteString("- **当前版本**: `" + info.Current + "`\n- **最新版本**: `" + info.Latest + "`\n")
	if info.PublishedAt != nil {
		ts := info.PublishedAt.Local().Format("2006-01-02 15:04:05")
		txt.WriteString("发布时间: " + ts + "\n")
		md.WriteString("- **发布时间**: " + ts + "\n")
	}
	if notes := cleanNotes(info.Notes, true); notes != "" {
		txt.WriteString("\n" + notes + "\n")
	}
	if notes := cleanNotes(info.Notes, false); notes != "" {
		md.WriteString("\n" + notes + "\n")
	}
	if info.URL != "" {
		txt.WriteString("\n" + info.URL)
		md.WriteString("\n[查看发布说明](" + info.URL + ")")
	}
	return notify.Message{Title: title, Content: txt.String(), Markdown: md.String(), Success: true}
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}

// NotifyUpdate 向开启了新版本提醒的渠道推送一次。没有可用渠道或全部发送失败时
// 不记录，下次检查会重试。
func (e *Engine) NotifyUpdate(info update.Info) {
	if !info.Available || info.Latest == "" || e.Store.Value(keyUpdateNotified) == info.Latest {
		return
	}
	list, err := e.Store.ListNotifiers()
	if err != nil {
		e.Log.Warn("读取通知渠道失败", "err", err)
		return
	}
	msg := BuildUpdateMessage(e.Store.GetSettings().NotifyTitlePrefix, info)
	sent := false
	for i := range list {
		n := &list[i]
		if !n.Enabled || !n.OnUpdate {
			continue
		}
		if err := Send(n, msg); err != nil {
			e.Log.Warn("新版本提醒发送失败", "notifier", n.Name, "err", err)
			continue
		}
		sent = true
	}
	if !sent {
		return
	}
	e.Log.Info("已推送新版本提醒", "version", info.Latest)
	if err := e.Store.SetValue(keyUpdateNotified, info.Latest); err != nil {
		e.Log.Warn("记录已提醒版本失败", "err", err)
	}
}
