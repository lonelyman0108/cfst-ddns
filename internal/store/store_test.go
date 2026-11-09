package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
	"github.com/lonelyman0108/cfst-ddns/internal/secret"
)

func open(t *testing.T) *Store {
	t.Helper()
	box, err := secret.New("test-key")
	if err != nil {
		t.Fatal(err)
	}
	st, err := Open(filepath.Join(t.TempDir(), "t.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestAccountConfigEncrypted(t *testing.T) {
	st := open(t)
	a := &Account{Name: "cf", Provider: "cloudflare", Config: schema.Config{"apiToken": "super-secret"}}
	if err := st.SaveAccount(a); err != nil {
		t.Fatal(err)
	}
	var raw Account
	st.DB.First(&raw, a.ID)
	if raw.ConfigEnc == "" || raw.ConfigEnc[:7] != "enc:v1:" {
		t.Fatalf("config not encrypted: %q", raw.ConfigEnc)
	}
	got, err := st.GetAccount(a.ID)
	if err != nil || got.Config["apiToken"] != "super-secret" {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestUpsertRecordState(t *testing.T) {
	st := open(t)
	for _, v := range []string{"1.1.1.1", "2.2.2.2"} {
		if err := st.UpsertRecordState(RecordState{TaskID: 1, AccountID: 1, FQDN: "a.example.com", Type: "A", Value: v}); err != nil {
			t.Fatal(err)
		}
	}
	var list []RecordState
	st.DB.Find(&list)
	if len(list) != 1 || list[0].Value != "2.2.2.2" {
		t.Fatalf("got %+v", list)
	}
}

func TestHealthStateUpsertAndDefault(t *testing.T) {
	st := open(t)
	state, err := st.GetHealthState(42)
	if err != nil {
		t.Fatal(err)
	}
	if state.TaskID != 42 || state.ConsecutiveFailures != 0 || state.LastCheckedAt != nil {
		t.Fatalf("default state = %+v", state)
	}
	state.ConsecutiveFailures = 2
	if err := st.SaveHealthState(*state); err != nil {
		t.Fatal(err)
	}
	state.ConsecutiveFailures = 3
	if err := st.SaveHealthState(*state); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := st.DB.Model(&HealthState{}).Where("task_id = ?", 42).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	got, err := st.GetHealthState(42)
	if err != nil || count != 1 || got.ConsecutiveFailures != 3 {
		t.Fatalf("state = %+v count=%d err=%v", got, count, err)
	}
}

func TestHealthCheckJSONAndPurge(t *testing.T) {
	st := open(t)
	old := time.Now().Add(-48 * time.Hour)
	if err := st.CreateHealthCheck(&HealthCheck{
		TaskID: 1, CheckedAt: old, CreatedAt: old, Status: HealthStatusUnhealthy,
		Results: []HealthResult{{TargetKey: "1|cdn.example.com|A|", IPType: "v4", IP: "1.1.1.1", Latency: 250, Speed: 2.5, Healthy: false, Reason: "latency"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateHealthCheck(&HealthCheck{TaskID: 1, CheckedAt: time.Now(), Status: HealthStatusHealthy}); err != nil {
		t.Fatal(err)
	}
	items, err := st.ListHealthChecks(1, 20)
	if err != nil || len(items) != 2 || len(items[1].Results) != 1 || items[1].Results[0].IP != "1.1.1.1" {
		t.Fatalf("items = %+v err=%v", items, err)
	}
	deleted, err := st.PurgeHealthChecks(1)
	if err != nil || deleted != 1 {
		t.Fatalf("deleted=%d err=%v", deleted, err)
	}
	items, err = st.ListHealthChecks(1, 20)
	if err != nil || len(items) != 1 || items[0].Status != HealthStatusHealthy {
		t.Fatalf("remaining = %+v err=%v", items, err)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	st := open(t)
	def := st.GetSettings()
	if def.HistoryRetentionDays != 30 || def.NotifyTitlePrefix == "" {
		t.Fatalf("defaults = %+v", def)
	}
	def.GithubMirror, def.HookEnabled, def.HookToken = "https://m.example", true, "tok"
	if err := st.SaveSettings(def); err != nil {
		t.Fatal(err)
	}
	if got := st.GetSettings(); got != def {
		t.Fatalf("got %+v", got)
	}
}

func TestTaskJSONColumns(t *testing.T) {
	st := open(t)
	task := &Task{Name: "t", IPType: "both", Targets: []Target{{AccountID: 1, Domain: "example.com", RR: "@"}}, NotifierIDs: []uint{3}}
	task.SpeedTest.Threads = 123
	if err := st.DB.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	got, err := st.GetTask(task.ID)
	if err != nil || got.SpeedTest.Threads != 123 || len(got.Targets) != 1 || got.NotifierIDs[0] != 3 {
		t.Fatalf("got %+v %v", got, err)
	}
}
