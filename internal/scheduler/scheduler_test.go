package scheduler

import (
	"io"
	"log/slog"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

func TestSchedulerHealthIndependentOfCron(t *testing.T) {
	s := New(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.cron.Start()
	defer s.Stop()
	task := store.Task{ID: 1, Enabled: true, Cron: "@daily", HealthCheck: store.HealthCheckConfig{Enabled: true, IntervalMinutes: 10}}
	s.Sync(&task)
	if s.Next(1) == nil || s.HealthNext(1) == nil {
		t.Fatal("both schedules required")
	}
	original := s.healthEntries[1]
	task.HealthCheck.IntervalMinutes = 20
	s.Sync(&task)
	if s.healthEntries[1] == original || s.Next(1) == nil {
		t.Fatal("unchanged cron must not prevent health reschedule")
	}
	task.Cron = ""
	s.Sync(&task)
	if s.Next(1) != nil || s.HealthNext(1) == nil {
		t.Fatal("health must work without cron")
	}
	task.Enabled = false
	s.Sync(&task)
	if s.Next(1) != nil || s.HealthNext(1) != nil {
		t.Fatal("disabled task scheduled")
	}
	task.Enabled = true
	s.Sync(&task)
	s.Remove(1)
	if s.HealthNext(1) != nil || len(s.healthEntries) != 0 {
		t.Fatal("removed task scheduled")
	}
}
