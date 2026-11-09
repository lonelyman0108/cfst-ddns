package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/cfst"
	"github.com/lonelyman0108/cfst-ddns/internal/netcheck"
	"github.com/lonelyman0108/cfst-ddns/internal/provider"
	"github.com/lonelyman0108/cfst-ddns/internal/schema"
	"github.com/lonelyman0108/cfst-ddns/internal/store"
	"gorm.io/gorm"
)

// ProviderFactory and HealthProbe use the same provider/runner contracts as full runs.
type ProviderFactory func(string, schema.Config) (provider.Provider, error)
type HealthProbe func(context.Context, cfst.RunOptions, cfst.Output) ([]store.SpeedResult, error)

type HealthTargetIP struct {
	TargetKey string
	AccountID uint
	FQDN      string
	IPType    string
	IP        string
}

type HealthJobResult struct {
	CheckID        uint
	TriggerFullRun bool
}

func healthTargetKey(tg store.Target, typ string) string {
	line := tg.Line
	if defaultLines[line] {
		line = "default"
	}
	return fmt.Sprintf("%d|%s|%s|%s", tg.AccountID, strings.ToLower(provider.FQDN(tg.RR, tg.Domain)), typ, line)
}

func matchesTarget(r provider.Record, tg store.Target, typ string) bool {
	rr := strings.TrimSpace(tg.RR)
	if rr == "" {
		rr = "@"
	}
	name := r.Name
	if name == "" && r.FQDN != "" {
		name = provider.RR(r.FQDN, tg.Domain)
	}
	return strings.EqualFold(name, rr) && strings.EqualFold(r.Type, typ) && sameLine(r.Line, tg.Line)
}

func fingerprint(v any) string {
	b, _ := json.Marshal(v)
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}

func recordFingerprint(records []provider.Record) string {
	values := make([]string, 0, len(records))
	for _, r := range records {
		value := r.Value
		if ip := net.ParseIP(strings.TrimSpace(value)); ip != nil {
			value = ip.String()
		}
		values = append(values, fmt.Sprintf("%s|%s|%t", r.ID, value, r.Proxied))
	}
	sort.Strings(values)
	return fingerprint(values)
}

func healthConfigFingerprint(task *store.Task) string {
	return fingerprint(struct {
		IPType  string
		Targets []store.Target
		Speed   store.SpeedTestConfig
		Health  store.HealthCheckConfig
		Update  store.UpdatePolicy
	}{task.IPType, task.Targets, task.SpeedTest, task.HealthCheck, task.Update})
}

func (e *Engine) newProvider(kind string, cfg schema.Config) (provider.Provider, error) {
	if e.ProviderFactory != nil {
		return e.ProviderFactory(kind, cfg)
	}
	return provider.New(kind, cfg)
}

// CollectCurrentIPs reads authoritative provider records, never the local RecordState cache.
func (e *Engine) CollectCurrentIPs(ctx context.Context, task *store.Task) ([]HealthTargetIP, []store.HealthResult, error) {
	ips, results, _, err := e.collectCurrentIPs(ctx, task)
	return ips, results, err
}

func (e *Engine) collectCurrentIPs(ctx context.Context, task *store.Task) ([]HealthTargetIP, []store.HealthResult, map[string]string, error) {
	ips := []HealthTargetIP{}
	results := []store.HealthResult{}
	snapshots := map[string]string{}
	seen := map[string]bool{}
	for _, tg := range task.Targets {
		acc, err := e.Store.GetAccount(tg.AccountID)
		var p provider.Provider
		if err == nil {
			p, err = e.newProvider(acc.Provider, acc.Config)
		}
		targetIPs := []HealthTargetIP{}
		targetResults := []store.HealthResult{}
		targetSnapshots := map[string]string{}
		proxied := tg.Proxied
		for _, ipType := range ipTypes(task.IPType) {
			typ := recordType(ipType)
			key := healthTargetKey(tg, typ)
			result := store.HealthResult{TargetKey: key, IPType: ipType}
			if tg.Proxied {
				result.Status, result.Reason = store.HealthStatusSkipped, "proxied"
				targetResults = append(targetResults, result)
				continue
			}
			queryErr := err
			var records []provider.Record
			if queryErr == nil {
				qctx, cancel := context.WithTimeout(ctx, 30*time.Second)
				rr := tg.RR
				if rr == "" {
					rr = "@"
				}
				all, readErr := p.ListRecords(qctx, tg.Domain, rr, typ)
				queryErr = readErr
				if queryErr == nil {
					queryErr = qctx.Err()
				}
				cancel()
				for _, r := range all {
					if matchesTarget(r, tg, typ) {
						records = append(records, r)
					}
				}
			}
			if queryErr != nil {
				result.Status, result.Reason = store.HealthStatusUnknown, "provider"
				result.Detail = queryErr.Error()
				targetResults = append(targetResults, result)
				continue
			}
			targetSnapshots[key] = recordFingerprint(records)
			if len(records) == 0 {
				result.Status, result.Reason = store.HealthStatusSkipped, "noRecords"
				targetResults = append(targetResults, result)
			}
			for _, r := range records {
				if r.Proxied {
					proxied = true
				}
				ip := net.ParseIP(strings.TrimSpace(r.Value))
				if ip == nil || (ip.To4() != nil) != (ipType == "v4") {
					result.Status, result.Reason = store.HealthStatusUnknown, "invalidIP"
					result.IP = r.Value
					targetResults = append(targetResults, result)
					continue
				}
				value := ip.String()
				if !seen[key+"|"+value] {
					seen[key+"|"+value] = true
					targetIPs = append(targetIPs, HealthTargetIP{key, tg.AccountID, provider.FQDN(tg.RR, tg.Domain), ipType, value})
				}
			}
		}
		if proxied {
			// Any proxied record makes the whole target unsafe for automatic replacement.
			for _, ipType := range ipTypes(task.IPType) {
				results = append(results, store.HealthResult{TargetKey: healthTargetKey(tg, recordType(ipType)), IPType: ipType, Status: store.HealthStatusSkipped, Reason: "proxied"})
			}
			// A query error must still make the batch unknown, even for a subsequently skipped target.
			for _, r := range targetResults {
				if r.Status == store.HealthStatusUnknown {
					results = append(results, r)
				}
			}
			continue
		}
		ips = append(ips, targetIPs...)
		results = append(results, targetResults...)
		for k, v := range targetSnapshots {
			snapshots[k] = v
		}
	}
	sort.Slice(ips, func(i, j int) bool { return ips[i].TargetKey+ips[i].IP < ips[j].TargetKey+ips[j].IP })
	return ips, results, snapshots, nil
}

// EvaluateHealth supports legacy results with only Healthy/Reason, while unknown and skipped
// results remain explicitly distinct from successful measurements.
func EvaluateHealth(results []store.HealthResult, cfg store.HealthCheckConfig) (bool, string) {
	seen := false
	reason := ""
	for _, r := range results {
		if r.Status == store.HealthStatusUnknown || r.Reason == "provider" || r.Reason == "proxyDetected" || r.Reason == "probeError" || r.Reason == "invalidIP" {
			return false, "unknown"
		}
		if r.Status == store.HealthStatusSkipped || r.Reason == "proxied" || r.Reason == "noRecords" {
			continue
		}
		seen = true
		switch {
		case r.Reason != "":
			if reason == "" {
				reason = r.Reason
			}
		case r.Status == store.HealthStatusUnhealthy:
			if reason == "" {
				reason = "unreachable"
			}
		case math.IsNaN(r.Latency) || math.IsInf(r.Latency, 0) || math.IsNaN(r.Speed) || math.IsInf(r.Speed, 0):
			return false, "unknown"
		case r.LossRate >= 1 || r.Latency <= 0:
			if reason == "" {
				reason = "unreachable"
			}
		case cfg.MaxLatency > 0 && r.Latency > cfg.MaxLatency:
			if reason == "" {
				reason = "latency"
			}
		case cfg.MinSpeed > 0 && r.Speed < cfg.MinSpeed:
			if reason == "" {
				reason = "speed"
			}
		}
	}
	if !seen {
		return false, "skipped"
	}
	return reason == "", reason
}

var errHealthSnapshot = errors.New("健康检查目标已变化或不可安全更新，DNS 记录未修改")

func qualifiedReplacement(r store.SpeedResult, task *store.Task) bool {
	ip := net.ParseIP(r.IP)
	if ip == nil || (ip.To4() != nil) != (r.IPType == "v4") {
		return false
	}
	healthy, _ := EvaluateHealth([]store.HealthResult{{IP: r.IP, Latency: r.Latency, Speed: r.Speed, LossRate: r.LossRate}}, task.HealthCheck)
	s := task.SpeedTest
	return healthy && (s.MaxLatency <= 0 || r.Latency <= float64(s.MaxLatency)) && r.Latency >= float64(s.MinLatency) && (s.MaxLossRate < 0 || s.MaxLossRate >= 1 || r.LossRate <= s.MaxLossRate) && (s.MinSpeed <= 0 || r.Speed >= s.MinSpeed)
}

// state is the state before this check; the next unhealthy result reaches the threshold.
func ShouldTriggerHealth(state store.HealthState, now time.Time, cfg store.HealthCheckConfig, unhealthy bool) bool {
	return unhealthy && state.ConsecutiveFailures+1 >= max(1, cfg.ConsecutiveFailures) && (state.CooldownUntil == nil || !now.Before(*state.CooldownUntil))
}

func healthProbeConfig(task *store.Task, count int) store.SpeedTestConfig {
	cfg := task.SpeedTest
	cfg.MaxLatency, cfg.MinLatency, cfg.MaxLossRate, cfg.MinSpeed = 999999, 0, 1, 0
	cfg.ExtraArgs = ""
	cfg.DisableDownload = task.HealthCheck.MinSpeed <= 0
	cfg.DownloadCount = count
	return cfg
}

type discardHealthOutput struct{}

func (discardHealthOutput) Line(string)     {}
func (discardHealthOutput) Progress(string) {}

// HealthJob is bounded and serialized with full jobs, including callers outside the worker.
func (e *Engine) HealthJob(parent context.Context, taskID uint) (HealthJobResult, error) {
	e.workMu.Lock()
	defer e.workMu.Unlock()
	e.mu.Lock()
	if e.maintenance {
		e.mu.Unlock()
		return HealthJobResult{}, ErrBusy
	}
	e.healthActive[taskID] = true
	e.mu.Unlock()
	defer func() { e.mu.Lock(); delete(e.healthActive, taskID); e.mu.Unlock() }()
	ctx, cancel := context.WithTimeout(parent, 15*time.Minute)
	defer cancel()
	task, err := e.Store.GetTask(taskID)
	if err != nil {
		return HealthJobResult{}, err
	}
	check := store.HealthCheck{TaskID: taskID, CheckedAt: time.Now(), Results: []store.HealthResult{}}
	e.mu.Lock()
	_, fullPending := e.pending[taskID]
	e.mu.Unlock()
	snapshots := map[string]string{}
	if !task.Enabled || !task.HealthCheck.Enabled || fullPending {
		check.Status, check.FailureReason = store.HealthStatusSkipped, "disabled"
		if fullPending {
			check.FailureReason = "busy"
		}
	} else {
		mappings, results, current, collectErr := e.collectCurrentIPs(ctx, task)
		if collectErr != nil {
			return HealthJobResult{}, collectErr
		}
		snapshots = current
		check.Results = results
		hasUnknown := false
		for _, r := range results {
			if r.Status == store.HealthStatusUnknown {
				hasUnknown = true
			}
		}
		if hasUnknown {
			for _, m := range mappings {
				check.Results = append(check.Results, store.HealthResult{TargetKey: m.TargetKey, IPType: m.IPType, IP: m.IP, Status: store.HealthStatusUnknown, Reason: "provider"})
			}
		}
		if !hasUnknown && len(mappings) > 0 {
			if e.Probe != nil {
				for _, p := range e.Probe(ctx, ipTypes(task.IPType), !task.SpeedTest.IsProxy()) {
					if !task.SpeedTest.IsProxy() && p.Intercepted {
						hasUnknown = true
					}
				}
			}
			for _, ipType := range ipTypes(task.IPType) {
				ips := []string{}
				seen := map[string]bool{}
				for _, m := range mappings {
					if m.IPType == ipType && !seen[m.IP] {
						seen[m.IP] = true
						ips = append(ips, m.IP)
					}
				}
				if len(ips) == 0 {
					continue
				}
				byIP := map[string]store.SpeedResult{}
				probeReason, probeDetail := "", ""
				if hasUnknown {
					probeReason = "proxyDetected"
				} else {
					probe := e.HealthProbe
					if probe == nil {
						probe = e.CFST.Run
					}
					measured, probeErr := probe(ctx, cfst.RunOptions{IPType: ipType, IPs: ips, Config: healthProbeConfig(task, len(ips)), WorkDir: e.TempDir, Tag: fmt.Sprintf("health%d", task.ID)}, discardHealthOutput{})
					if probeErr != nil || ctx.Err() != nil {
						probeReason = "probeError"
						if probeErr != nil {
							probeDetail = probeErr.Error()
						} else {
							probeDetail = ctx.Err().Error()
						}
					} else if !task.SpeedTest.IsProxy() && !task.SpeedTest.Httping && netcheck.LatencyAnomalous(measured) {
						probeReason = "proxyDetected"
					}
					for _, r := range measured {
						if ip := net.ParseIP(r.IP); ip != nil {
							byIP[ip.String()] = r
						}
					}
				}
				for _, m := range mappings {
					if m.IPType != ipType {
						continue
					}
					r := store.HealthResult{TargetKey: m.TargetKey, IPType: m.IPType, IP: m.IP}
					if probeReason != "" {
						r.Status, r.Reason, r.Detail = store.HealthStatusUnknown, probeReason, probeDetail
					} else if v, ok := byIP[m.IP]; ok {
						r.Latency, r.Speed, r.LossRate = v.Latency, v.Speed, v.LossRate
						r.Healthy, r.Reason = EvaluateHealth([]store.HealthResult{r}, task.HealthCheck)
						r.Status = store.HealthStatusUnhealthy
						if r.Healthy {
							r.Status = store.HealthStatusHealthy
						}
					} else {
						r.Status, r.Reason = store.HealthStatusUnhealthy, "missingResult"
					}
					check.Results = append(check.Results, r)
				}
			}
		}
		healthy, reason := EvaluateHealth(check.Results, task.HealthCheck)
		check.Status, check.FailureReason = store.HealthStatusUnhealthy, reason
		if healthy {
			check.Status = store.HealthStatusHealthy
		} else if reason == "unknown" {
			check.Status = store.HealthStatusUnknown
		} else if reason == "skipped" {
			check.Status = store.HealthStatusSkipped
			// Only a task with no records in any applicable nonproxied target is recoverably missing.
			if len(snapshots) > 0 && len(mappings) == 0 {
				missing := true
				for _, r := range check.Results {
					if r.Status != store.HealthStatusSkipped {
						missing = false
					}
				}
				if missing {
					check.Status, check.FailureReason = store.HealthStatusUnhealthy, "noRecords"
					for i := range check.Results {
						if check.Results[i].Reason == "noRecords" {
							check.Results[i].Status = store.HealthStatusUnhealthy
						}
					}
				}
			}
		}
		if ctx.Err() != nil {
			check.Status, check.FailureReason = store.HealthStatusUnknown, "probeError"
		}
	}
	check.TargetSnapshots = snapshots
	check.ConfigFingerprint = healthConfigFingerprint(task)
	snapshot := fingerprint(struct {
		Config  string
		Targets map[string]string
	}{check.ConfigFingerprint, snapshots})
	result := HealthJobResult{}
	err = e.Store.DB.Transaction(func(tx *gorm.DB) error {
		st := e.Store.WithDB(tx)
		state, err := st.GetHealthState(taskID)
		if err != nil {
			return err
		}
		if check.Status == store.HealthStatusHealthy || check.Status == store.HealthStatusUnhealthy {
			if state.SnapshotFingerprint != snapshot {
				state.ConsecutiveFailures = 0
			}
			state.SnapshotFingerprint = snapshot
		}
		result.TriggerFullRun = ShouldTriggerHealth(*state, check.CheckedAt, task.HealthCheck, check.Status == store.HealthStatusUnhealthy)
		state.LastCheckedAt = &check.CheckedAt
		state.Status = check.Status
		state.LastHealthy = nil
		switch check.Status {
		case store.HealthStatusHealthy:
			state.ConsecutiveFailures = 0
			healthy := true
			state.LastHealthy = &healthy
		case store.HealthStatusUnhealthy:
			state.ConsecutiveFailures++
			healthy := false
			state.LastHealthy = &healthy
		}
		if err := st.CreateHealthCheck(&check); err != nil {
			return err
		}
		return st.SaveHealthState(*state)
	})
	result.CheckID = check.ID
	return result, err
}
