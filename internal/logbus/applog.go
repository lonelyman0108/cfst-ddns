// Package logbus 提供应用日志环形缓冲与执行日志的发布订阅。
package logbus

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Line 为一条应用日志。
type Line struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// Ring 为固定容量的日志缓冲。
type Ring struct {
	mu    sync.Mutex
	lines []Line
	next  int
	full  bool
}

func NewRing(n int) *Ring { return &Ring{lines: make([]Line, n)} }

func (r *Ring) add(l Line) {
	r.mu.Lock()
	r.lines[r.next] = l
	r.next = (r.next + 1) % len(r.lines)
	if r.next == 0 {
		r.full = true
	}
	r.mu.Unlock()
}

// Tail 返回最近 n 条日志（按时间正序）。
func (r *Ring) Tail(n int) []Line {
	r.mu.Lock()
	defer r.mu.Unlock()
	var all []Line
	if r.full {
		all = append(all, r.lines[r.next:]...)
	}
	all = append(all, r.lines[:r.next]...)
	if n > 0 && len(all) > n {
		all = all[len(all)-n:]
	}
	return all
}

// handler 同时写入输出流与环形缓冲。
type handler struct {
	out   io.Writer
	ring  *Ring
	level slog.Level
	attrs []slog.Attr
	mu    *sync.Mutex
}

// NewLogger 创建应用日志器。
func NewLogger(out io.Writer, ring *Ring, level slog.Level) *slog.Logger {
	return slog.New(&handler{out: out, ring: ring, level: level, mu: &sync.Mutex{}})
}

func (h *handler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }

func (h *handler) Handle(_ context.Context, r slog.Record) error {
	var sb strings.Builder
	sb.WriteString(r.Message)
	write := func(a slog.Attr) bool {
		if a.Key == "" {
			return true
		}
		fmt.Fprintf(&sb, " %s=%v", a.Key, a.Value.Any())
		return true
	}
	for _, a := range h.attrs {
		write(a)
	}
	r.Attrs(write)
	msg := sb.String()
	h.ring.add(Line{Time: r.Time, Level: r.Level.String(), Message: msg})
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := fmt.Fprintf(h.out, "%s %-5s %s\n", r.Time.Format("2006-01-02 15:04:05"), r.Level.String(), msg)
	return err
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	n := *h
	n.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &n
}

func (h *handler) WithGroup(string) slog.Handler { return h }
