package logbus

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Event 为推送给订阅者的执行事件。
type Event struct {
	Type string // log | progress | status | done
	Data any
}

// RunLog 保存一次执行的日志并广播给订阅者。
type RunLog struct {
	mu     sync.Mutex
	lines  []string
	subs   map[chan Event]struct{}
	closed bool
}

func newRunLog() *RunLog { return &RunLog{subs: map[chan Event]struct{}{}} }

// Printf 追加一行带时间戳的日志。
func (l *RunLog) Printf(format string, args ...any) {
	l.Line(fmt.Sprintf(format, args...))
}

// Line 追加日志行（可含换行，会被拆分）。
func (l *RunLog) Line(s string) {
	ts := time.Now().Format("15:04:05")
	for _, part := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		line := ts + " " + part
		l.mu.Lock()
		l.lines = append(l.lines, line)
		l.broadcast(Event{Type: "log", Data: line})
		l.mu.Unlock()
	}
}

// Progress 广播进度行（不保存）。
func (l *RunLog) Progress(s string) {
	l.mu.Lock()
	l.broadcast(Event{Type: "progress", Data: s})
	l.mu.Unlock()
}

// Emit 广播任意事件。done 事件后关闭所有订阅。
func (l *RunLog) Emit(typ string, data any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.broadcast(Event{Type: typ, Data: data})
	if typ == "done" {
		l.closed = true
		for ch := range l.subs {
			close(ch)
		}
		l.subs = map[chan Event]struct{}{}
	}
}

// broadcast 需在持锁时调用；订阅者消费过慢时丢弃事件而不是阻塞执行。
func (l *RunLog) broadcast(e Event) {
	for ch := range l.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Text 返回完整日志。
func (l *RunLog) Text() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.lines, "\n")
}

// Subscribe 返回已有日志与后续事件通道；执行已结束时通道为 nil。
func (l *RunLog) Subscribe() ([]string, chan Event, func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	backlog := append([]string(nil), l.lines...)
	if l.closed {
		return backlog, nil, func() {}
	}
	ch := make(chan Event, 256)
	l.subs[ch] = struct{}{}
	return backlog, ch, func() {
		l.mu.Lock()
		if _, ok := l.subs[ch]; ok {
			delete(l.subs, ch)
			close(ch)
		}
		l.mu.Unlock()
	}
}

// Hub 管理进行中执行的日志。
type Hub struct {
	mu   sync.Mutex
	runs map[uint]*RunLog
}

func NewHub() *Hub { return &Hub{runs: map[uint]*RunLog{}} }

// Open 为执行创建日志。
func (h *Hub) Open(runID uint) *RunLog {
	h.mu.Lock()
	defer h.mu.Unlock()
	l := newRunLog()
	h.runs[runID] = l
	return l
}

// Get 返回进行中执行的日志。
func (h *Hub) Get(runID uint) (*RunLog, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	l, ok := h.runs[runID]
	return l, ok
}

// Release 在执行结束一段时间后移除日志，给迟到的订阅者留出窗口。
func (h *Hub) Release(runID uint) {
	time.AfterFunc(time.Minute, func() {
		h.mu.Lock()
		delete(h.runs, runID)
		h.mu.Unlock()
	})
}
