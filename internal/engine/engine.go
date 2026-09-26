// Package engine 负责任务执行：测速 → 选 IP → 同步 DNS → 通知 → 写历史。
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/cfst"
	"github.com/lonelyman0108/cfst-ddns/internal/logbus"
	"github.com/lonelyman0108/cfst-ddns/internal/provider"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

// ErrBusy 表示任务已在队列或运行中。
var ErrBusy = errors.New("该任务已在执行队列中")

// Engine 串行执行任务（多个测速同时运行会互相抢占带宽，结果失真）。
type Engine struct {
	Store   *store.Store
	CFST    *cfst.Manager
	Hub     *logbus.Hub
	Log     *slog.Logger
	TempDir string

	queue chan uint // run id

	mu      sync.Mutex
	pending map[uint]uint // task id → run id（排队或运行中）
	cancels map[uint]context.CancelFunc
	cancelQ map[uint]bool // 排队中被取消的 run
}

// New 创建执行引擎。
func New(st *store.Store, m *cfst.Manager, hub *logbus.Hub, log *slog.Logger, tempDir string) *Engine {
	return &Engine{Store: st, CFST: m, Hub: hub, Log: log, TempDir: tempDir,
		queue: make(chan uint, 256), pending: map[uint]uint{}, cancels: map[uint]context.CancelFunc{}, cancelQ: map[uint]bool{}}
}

// Start 启动工作协程，并把上次异常退出遗留的执行标记为失败。
func (e *Engine) Start(ctx context.Context) {
	now := time.Now()
	e.Store.DB.Model(&store.Run{}).Where("status IN ?", []string{store.StatusQueued, store.StatusRunning}).
		Updates(map[string]any{"status": store.StatusFailed, "message": "服务重启，执行被中断", "finished_at": now})
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case id := <-e.queue:
				e.execute(ctx, id)
			}
		}
	}()
}

// Enqueue 为任务创建一次执行并排队；dryRun 为试运行（只测速，不写 DNS、不通知）。
func (e *Engine) Enqueue(taskID uint, trigger string, dryRun bool) (uint, error) {
	task, err := e.Store.GetTask(taskID)
	if err != nil {
		return 0, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, busy := e.pending[taskID]; busy {
		return 0, ErrBusy
	}
	run := &store.Run{TaskID: task.ID, TaskName: task.Name, Trigger: trigger, DryRun: dryRun, Status: store.StatusQueued}
	if err := e.Store.DB.Create(run).Error; err != nil {
		return 0, err
	}
	e.pending[taskID] = run.ID
	mode := ""
	if dryRun {
		mode = "，试运行"
	}
	e.Hub.Open(run.ID).Printf("任务「%s」已加入队列（触发方式: %s%s）", task.Name, trigger, mode)
	select {
	case e.queue <- run.ID:
	default:
		delete(e.pending, taskID)
		e.Store.DB.Model(run).Updates(map[string]any{"status": store.StatusFailed, "message": "执行队列已满"})
		return 0, errors.New("执行队列已满")
	}
	return run.ID, nil
}

// Running 返回任务当前排队或运行中的 run id。
func (e *Engine) Running(taskID uint) (uint, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	id, ok := e.pending[taskID]
	return id, ok
}

// Cancel 取消排队或运行中的执行。
func (e *Engine) Cancel(runID uint) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if c, ok := e.cancels[runID]; ok {
		c()
		return nil
	}
	for _, id := range e.pending {
		if id == runID {
			e.cancelQ[runID] = true
			return nil
		}
	}
	return errors.New("执行不在运行中")
}

func (e *Engine) finishPending(run *store.Run) {
	e.mu.Lock()
	delete(e.pending, run.TaskID)
	delete(e.cancels, run.ID)
	delete(e.cancelQ, run.ID)
	e.mu.Unlock()
}

// execute 执行一次任务，所有错误都记录在 run 中。
func (e *Engine) execute(parent context.Context, runID uint) {
	var run store.Run
	if err := e.Store.DB.First(&run, runID).Error; err != nil {
		e.Log.Error("读取执行记录失败", "run", runID, "err", err)
		return
	}
	rl, ok := e.Hub.Get(runID)
	if !ok {
		rl = e.Hub.Open(runID)
	}
	defer e.Hub.Release(runID)
	defer e.finishPending(&run)

	ctx, cancel := context.WithTimeout(parent, 2*time.Hour)
	defer cancel()
	e.mu.Lock()
	canceled := e.cancelQ[runID]
	e.cancels[runID] = cancel
	e.mu.Unlock()

	start := time.Now()
	run.StartedAt = &start
	run.Status = store.StatusRunning
	e.Store.DB.Model(&run).Updates(map[string]any{"status": run.Status, "started_at": start})
	rl.Emit("status", run)

	var (
		task *store.Task
		err  error
	)
	if canceled {
		err = context.Canceled
	} else {
		task, err = e.Store.GetTask(run.TaskID)
	}
	if err == nil {
		err = e.runTask(ctx, task, &run, rl)
	}

	end := time.Now()
	run.FinishedAt = &end
	run.DurationMs = end.Sub(start).Milliseconds()
	switch {
	case errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled):
		run.Status = store.StatusCanceled
		run.Message = "已取消"
		rl.Printf("执行已取消")
	case err != nil:
		run.Status = store.StatusFailed
		run.Message = err.Error()
		rl.Printf("✗ 执行失败: %v", err)
	}
	rl.Printf("结束，耗时 %s", time.Duration(run.DurationMs)*time.Millisecond)

	if task != nil && run.Status != store.StatusCanceled && !run.DryRun {
		e.notify(task, &run, rl)
	}
	run.Log = rl.Text()
	if err := e.Store.DB.Save(&run).Error; err != nil {
		e.Log.Error("保存执行记录失败", "run", run.ID, "err", err)
	}
	e.Log.Info("任务执行结束", "task", run.TaskName, "run", run.ID, "status", run.Status, "message", run.Message)
	summary := run
	summary.Log, summary.Results, summary.Changes = "", nil, nil
	rl.Emit("done", summary)
}

// MaxStoredResults 为每种 IP 类型保存到执行记录的测速结果上限。
const MaxStoredResults = 100

func ipTypes(t string) []string {
	switch t {
	case "v6":
		return []string{"v6"}
	case "both":
		return []string{"v4", "v6"}
	default:
		return []string{"v4"}
	}
}

func recordType(ipType string) string {
	if ipType == "v6" {
		return "AAAA"
	}
	return "A"
}

// runTask 为执行主体；返回的 error 表示整体失败，部分失败通过 run.Status 表达。
func (e *Engine) runTask(ctx context.Context, task *store.Task, run *store.Run, rl *logbus.RunLog) error {
	rl.Printf("开始执行任务「%s」，测速类型: %s，目标记录: %d 条", task.Name, task.IPType, len(task.Targets))
	if run.DryRun {
		rl.Printf("本次为试运行：只测速，不修改 DNS、不发送通知")
	} else if len(task.Targets) == 0 {
		return errors.New("任务未配置目标记录")
	}
	count := task.Update.RecordCount
	if count <= 0 {
		count = 1
	}

	// 1. 测速
	best := map[string][]string{} // ipType → 选中的 IP
	for _, t := range ipTypes(task.IPType) {
		rl.Printf("━━ 开始 IPv%s 测速 ━━", strings.TrimPrefix(t, "v"))
		res, err := e.CFST.Run(ctx, cfst.RunOptions{IPType: t, Config: task.SpeedTest,
			WorkDir: e.TempDir, Tag: fmt.Sprintf("run%d", run.ID)}, rl)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			rl.Printf("✗ IPv%s 测速失败: %v", strings.TrimPrefix(t, "v"), err)
			continue
		}
		// 默认 IP 段可测出数千个结果，只保存排名靠前的部分，避免执行记录膨胀
		if run.ResultTotals == nil {
			run.ResultTotals = map[string]int{}
		}
		run.ResultTotals[t] = len(res)
		run.Results = append(run.Results, res[:min(len(res), MaxStoredResults)]...)
		if len(res) == 0 {
			rl.Printf("! IPv%s 没有满足条件的 IP，将保留现有记录", strings.TrimPrefix(t, "v"))
			continue
		}
		for i := 0; i < len(res) && i < count; i++ {
			best[t] = append(best[t], res[i].IP)
		}
		top := res[0]
		rl.Printf("✓ IPv%s 最优: %s  延迟 %.2f ms  速度 %.2f MB/s  地区 %s", strings.TrimPrefix(t, "v"), top.IP, top.Latency, top.Speed, top.Colo)
		if t == "v4" {
			run.BestIPv4 = top.IP
		} else {
			run.BestIPv6 = top.IP
		}
		if run.BestLatency == 0 {
			run.BestLatency, run.BestSpeed = top.Latency, top.Speed
		}
	}
	e.Store.DB.Model(run).Updates(map[string]any{"best_ipv4": run.BestIPv4, "best_ipv6": run.BestIPv6,
		"best_latency": run.BestLatency, "best_speed": run.BestSpeed})
	rl.Emit("status", *run)
	if len(best) == 0 {
		return errors.New("没有获得任何可用 IP，DNS 记录未修改")
	}
	if run.DryRun {
		rl.Printf("试运行：跳过 DNS 同步与通知")
		run.Status = store.StatusSuccess
		run.Message = "试运行完成，未修改 DNS"
		return nil
	}

	// 2. 同步 DNS
	rl.Printf("━━ 更新 DNS 记录 ━━")
	providers := map[uint]provider.Provider{}
	names := map[uint]string{}
	errCount, total := 0, 0
	for _, tg := range task.Targets {
		acc, err := e.Store.GetAccount(tg.AccountID)
		var p provider.Provider
		if err == nil {
			names[acc.ID] = acc.Name
			if p = providers[acc.ID]; p == nil {
				p, err = provider.New(acc.Provider, acc.Config)
				providers[acc.ID] = p
			}
		}
		for _, t := range ipTypes(task.IPType) {
			fqdn := provider.FQDN(tg.RR, tg.Domain)
			ch := store.DNSChange{AccountID: tg.AccountID, AccountName: names[tg.AccountID], FQDN: fqdn, Type: recordType(t)}
			if err != nil {
				ch.Action, ch.Message = "error", "DNS 账号不可用: "+err.Error()
				run.Changes = append(run.Changes, ch)
				rl.Printf("✗ %s %s: %s", fqdn, ch.Type, ch.Message)
				errCount++
				total++
				continue
			}
			ips, ok := best[t]
			if !ok {
				ch.Action, ch.Message = "skip", "无可用 IP，保留原记录"
				run.Changes = append(run.Changes, ch)
				continue
			}
			total++
			changes, serr := e.syncTarget(ctx, p, tg, recordType(t), ips, !task.Update.SkipUnchanged)
			for i := range changes {
				changes[i].AccountID, changes[i].AccountName = tg.AccountID, names[tg.AccountID]
				c := changes[i]
				switch c.Action {
				case "create":
					rl.Printf("✓ 新建 %s %s → %s", c.FQDN, c.Type, c.NewValue)
				case "update":
					rl.Printf("✓ 更新 %s %s: %s → %s", c.FQDN, c.Type, c.OldValue, c.NewValue)
				case "delete":
					rl.Printf("✓ 删除多余记录 %s %s %s", c.FQDN, c.Type, c.OldValue)
				case "skip":
					rl.Printf("= %s %s 未变化 (%s)", c.FQDN, c.Type, c.NewValue)
				case "error":
					rl.Printf("✗ %s %s: %s", c.FQDN, c.Type, c.Message)
				}
				if c.Action != "skip" && c.Action != "error" {
					run.Changed = true
				}
			}
			run.Changes = append(run.Changes, changes...)
			if serr != nil {
				errCount++
				continue
			}
			if err := e.Store.UpsertRecordState(store.RecordState{TaskID: task.ID, AccountID: tg.AccountID, FQDN: fqdn,
				Type: recordType(t), Value: strings.Join(ips, ",")}); err != nil {
				e.Log.Warn("保存记录状态失败", "err", err)
			}
		}
	}

	switch {
	case total > 0 && errCount == total:
		return errors.New("全部 DNS 记录更新失败")
	case errCount > 0:
		run.Status = store.StatusPartial
		run.Message = fmt.Sprintf("%d/%d 条记录更新失败", errCount, total)
	default:
		run.Status = store.StatusSuccess
		if run.Changed {
			run.Message = "DNS 记录已更新"
		} else {
			run.Message = "IP 未变化"
		}
	}
	return nil
}

// syncTarget 同步一条目标记录的一种类型；返回变更明细，error 表示该目标失败。
func (e *Engine) syncTarget(ctx context.Context, p provider.Provider, tg store.Target, typ string, ips []string, force bool) ([]store.DNSChange, error) {
	fqdn := provider.FQDN(tg.RR, tg.Domain)
	rr := strings.TrimSpace(tg.RR)
	if rr == "" {
		rr = "@"
	}
	fail := func(err error) ([]store.DNSChange, error) {
		return []store.DNSChange{{FQDN: fqdn, Type: typ, Action: "error", NewValue: strings.Join(ips, ","), Message: err.Error()}}, err
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	all, err := p.ListRecords(cctx, tg.Domain, rr, typ)
	if err != nil {
		return fail(fmt.Errorf("查询现有记录失败: %w", err))
	}
	var existing []provider.Record
	for _, r := range all {
		if strings.EqualFold(r.Type, typ) && sameLine(r.Line, tg.Line) {
			existing = append(existing, r)
		}
	}
	plan := MakePlan(existing, ips)
	tmpl := provider.Record{Name: rr, Type: typ, TTL: tg.TTL, Proxied: tg.Proxied, Line: tg.Line}

	var out []store.DNSChange
	var firstErr error
	record := func(action string, old, nv string, err error) {
		c := store.DNSChange{FQDN: fqdn, Type: typ, Action: action, OldValue: old, NewValue: nv}
		if err != nil {
			c.Action, c.Message = "error", err.Error()
			if firstErr == nil {
				firstErr = err
			}
		}
		out = append(out, c)
	}
	for _, r := range plan.Keep {
		if force {
			nr := tmpl
			nr.ID, nr.Value = r.ID, r.Value
			record("update", r.Value, r.Value, p.UpdateRecord(cctx, tg.Domain, nr))
		} else {
			record("skip", r.Value, r.Value, nil)
		}
	}
	for _, u := range plan.Updates {
		nr := tmpl
		nr.ID, nr.Value = u.Record.ID, u.NewValue
		record("update", u.Record.Value, u.NewValue, p.UpdateRecord(cctx, tg.Domain, nr))
	}
	for _, v := range plan.Creates {
		nr := tmpl
		nr.Value = v
		record("create", "", v, p.CreateRecord(cctx, tg.Domain, nr))
	}
	for _, r := range plan.Deletes {
		record("delete", r.Value, "", p.DeleteRecord(cctx, tg.Domain, r))
	}
	return out, firstErr
}

// TempDirFor 返回数据目录下的临时目录。
func TempDirFor(dataDir string) string { return filepath.Join(dataDir, "tmp") }
