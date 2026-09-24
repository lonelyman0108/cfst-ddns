package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/logbus"
	"github.com/lonelyman0108/cfst-ddns/internal/notify"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

var statusText = map[string]string{
	store.StatusSuccess: "成功",
	store.StatusPartial: "部分失败",
	store.StatusFailed:  "失败",
}

var actionText = map[string]string{
	"create": "新建", "update": "更新", "delete": "删除", "skip": "未变化", "error": "失败",
}

// BuildMessage 生成执行结果通知。
func BuildMessage(prefix string, task *store.Task, run *store.Run) notify.Message {
	title := fmt.Sprintf("%s %s - %s", strings.TrimSpace(prefix), statusText[run.Status], task.Name)
	title = strings.TrimSpace(title)

	var txt, md strings.Builder
	best := func(ipType, label string) {
		for _, r := range run.Results {
			if r.IPType == ipType && r.Rank == 1 {
				line := fmt.Sprintf("%s: %s（%.0f ms / %.2f MB/s / %s）", label, r.IP, r.Latency, r.Speed, orNA(r.Colo))
				txt.WriteString(line + "\n")
				md.WriteString("- **" + label + "**: `" + r.IP + "` " + fmt.Sprintf("%.0f ms / %.2f MB/s / %s", r.Latency, r.Speed, orNA(r.Colo)) + "\n")
				return
			}
		}
	}
	best("v4", "最优 IPv4")
	best("v6", "最优 IPv6")
	if run.Message != "" {
		txt.WriteString("结果: " + run.Message + "\n")
		md.WriteString("- **结果**: " + run.Message + "\n")
	}
	if len(run.Changes) > 0 {
		txt.WriteString("\n记录变更:\n")
		md.WriteString("\n**记录变更**\n\n")
		for _, c := range run.Changes {
			var detail string
			switch c.Action {
			case "update":
				detail = c.OldValue + " → " + c.NewValue
			case "create":
				detail = c.NewValue
			case "delete":
				detail = c.OldValue
			case "skip":
				detail = c.NewValue
			case "error":
				detail = c.Message
			}
			line := fmt.Sprintf("[%s] %s %s %s", actionText[c.Action], c.FQDN, c.Type, detail)
			txt.WriteString(line + "\n")
			md.WriteString("- " + line + "\n")
		}
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	if run.FinishedAt != nil {
		ts = run.FinishedAt.Format("2006-01-02 15:04:05")
	}
	dur := (time.Duration(run.DurationMs) * time.Millisecond).Round(time.Second)
	txt.WriteString(fmt.Sprintf("\n耗时: %s\n时间: %s", dur, ts))
	md.WriteString(fmt.Sprintf("\n耗时: %s  \n时间: %s", dur, ts))
	return notify.Message{Title: title, Content: txt.String(), Markdown: md.String(), Success: run.Status == store.StatusSuccess}
}

func orNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

// shouldNotify 判断某个渠道是否需要接收此次结果。
func shouldNotify(n *store.Notifier, run *store.Run) bool {
	if !n.Enabled {
		return false
	}
	if run.Status == store.StatusSuccess {
		return n.OnSuccess && (!n.OnlyOnChange || run.Changed)
	}
	return n.OnFailure
}

func (e *Engine) notify(task *store.Task, run *store.Run, rl *logbus.RunLog) {
	if len(task.NotifierIDs) == 0 {
		return
	}
	msg := BuildMessage(e.Store.GetSettings().NotifyTitlePrefix, task, run)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, id := range task.NotifierIDs {
		n, err := e.Store.GetNotifier(id)
		if err != nil {
			rl.Printf("! 通知渠道 #%d 不存在，已跳过", id)
			continue
		}
		if !shouldNotify(n, run) {
			continue
		}
		wg.Add(1)
		go func(n *store.Notifier) {
			defer wg.Done()
			err := Send(n, msg)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				rl.Printf("✗ 通知「%s」发送失败: %v", n.Name, err)
			} else {
				rl.Printf("✓ 通知「%s」已发送", n.Name)
			}
		}(n)
	}
	wg.Wait()
}

// Send 通过一个渠道发送消息。
func Send(n *store.Notifier, msg notify.Message) error {
	nt, err := notify.New(n.Type, n.Config)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return nt.Send(ctx, msg)
}
