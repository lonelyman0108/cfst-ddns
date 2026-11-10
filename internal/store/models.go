package store

import (
	"time"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

// User 为唯一的管理员账号。TokenVersion 在改密后递增，使旧令牌失效。
type User struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"uniqueIndex;size:64"`
	PasswordHash string
	TokenVersion int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// KV 为设置项的键值存储。
type KV struct {
	Key   string `gorm:"primaryKey;size:64"`
	Value string
}

// Account 为 DNS 服务商账号，Config 加密后存于 ConfigEnc。
type Account struct {
	ID        uint          `gorm:"primaryKey" json:"id"`
	Name      string        `gorm:"size:128" json:"name"`
	Provider  string        `gorm:"size:32;index" json:"provider"`
	ConfigEnc string        `json:"-"`
	Config    schema.Config `gorm:"-" json:"config"`
	Remark    string        `json:"remark"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

// Notifier 为通知渠道，Config 加密后存于 ConfigEnc。
type Notifier struct {
	ID           uint          `gorm:"primaryKey" json:"id"`
	Name         string        `gorm:"size:128" json:"name"`
	Type         string        `gorm:"size:32" json:"type"`
	Enabled      bool          `json:"enabled"`
	ConfigEnc    string        `json:"-"`
	Config       schema.Config `gorm:"-" json:"config"`
	OnSuccess    bool          `json:"onSuccess"`
	OnFailure    bool          `json:"onFailure"`
	OnlyOnChange bool          `json:"onlyOnChange"`
	OnUpdate     bool          `json:"onUpdate"` // 接收新版本提醒，与任务绑定无关
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
}

// SpeedTestConfig 对应 cfst 命令行参数。
type SpeedTestConfig struct {
	Threads         int     `json:"threads"`
	PingTimes       int     `json:"pingTimes"`
	DownloadCount   int     `json:"downloadCount"`
	DownloadTime    int     `json:"downloadTime"`
	Port            int     `json:"port"`
	URL             string  `json:"url"`
	Httping         bool    `json:"httping"`
	HttpingCode     int     `json:"httpingCode"`
	CFColo          string  `json:"cfColo"`
	MaxLatency      int     `json:"maxLatency"`
	MinLatency      int     `json:"minLatency"`
	MaxLossRate     float64 `json:"maxLossRate"`
	MinSpeed        float64 `json:"minSpeed"`
	DisableDownload bool    `json:"disableDownload"`
	AllIP           bool    `json:"allIP"`
	IPSource        string  `json:"ipSource"`
	IPv4Ranges      string  `json:"ipv4Ranges"`
	IPv6Ranges      string  `json:"ipv6Ranges"`
	ExtraArgs       string  `json:"extraArgs"`
	// NetworkMode 为测速网络：direct（默认，空值同）要求直连，检测到代理时不更新 DNS；
	// proxy 表示有意经透明代理测速，强制使用 HTTPing（TCP 握手会被本地代理秒回，延迟无意义）
	NetworkMode string `json:"networkMode"`
}

// 测速网络模式。
const (
	NetworkDirect = "direct"
	NetworkProxy  = "proxy"
)

// IsProxy 判断是否为有意经代理测速。
func (c SpeedTestConfig) IsProxy() bool { return c.NetworkMode == NetworkProxy }

// UpdatePolicy 控制如何把测速结果写入 DNS。
type UpdatePolicy struct {
	RecordCount   int  `json:"recordCount"`
	SkipUnchanged bool `json:"skipUnchanged"`
}

// HealthCheckConfig 控制任务对当前 DNS IP 的定时健康检测。
type HealthCheckConfig struct {
	Enabled             bool    `json:"enabled"`
	IntervalMinutes     int     `json:"intervalMinutes"`
	MaxLatency          float64 `json:"maxLatency"`
	MinSpeed            float64 `json:"minSpeed"`
	ConsecutiveFailures int     `json:"consecutiveFailures"`
	CooldownMinutes     int     `json:"cooldownMinutes"`
}

func DefaultHealthCheckConfig() HealthCheckConfig {
	return HealthCheckConfig{IntervalMinutes: 10, ConsecutiveFailures: 3, CooldownMinutes: 30}
}

// WithDefaults fills legacy disabled configurations; enabled invalid values remain visible to validation.
func (c HealthCheckConfig) WithDefaults() HealthCheckConfig {
	if c == (HealthCheckConfig{}) {
		return DefaultHealthCheckConfig()
	}
	if !c.Enabled {
		if c.IntervalMinutes == 0 {
			c.IntervalMinutes = 10
		}
		if c.ConsecutiveFailures == 0 {
			c.ConsecutiveFailures = 3
		}
	}
	return c
}

type Target struct {
	AccountID uint   `json:"accountId"`
	Domain    string `json:"domain"`
	RR        string `json:"rr"`
	TTL       int    `json:"ttl"`
	Proxied   bool   `json:"proxied"`
	Line      string `json:"line"`
}

// Task 为一个测速 + 更新任务。
type Task struct {
	ID          uint              `gorm:"primaryKey" json:"id"`
	Name        string            `gorm:"size:128" json:"name"`
	Enabled     bool              `json:"enabled"`
	Cron        string            `gorm:"size:128" json:"cron"`
	IPType      string            `gorm:"column:ip_type;size:8" json:"ipType"`
	SpeedTest   SpeedTestConfig   `gorm:"serializer:json" json:"speedTest"`
	Update      UpdatePolicy      `gorm:"serializer:json" json:"update"`
	HealthCheck HealthCheckConfig `gorm:"serializer:json" json:"healthCheck"`
	Targets     []Target          `gorm:"serializer:json" json:"targets"`
	NotifierIDs []uint            `gorm:"column:notifier_ids;serializer:json" json:"notifierIds"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

const (
	HealthStatusHealthy   = "healthy"
	HealthStatusUnhealthy = "unhealthy"
	HealthStatusUnknown   = "unknown"
	HealthStatusSkipped   = "skipped"
)

// HealthState 保存每个任务的连续异常和冷却状态。
type HealthState struct {
	TaskID              uint       `gorm:"primaryKey" json:"taskId"`
	Status              string     `json:"status"`
	SnapshotFingerprint string     `json:"snapshotFingerprint,omitempty"`
	ConsecutiveFailures int        `json:"consecutiveFailures"`
	LastCheckedAt       *time.Time `json:"lastCheckedAt"`
	LastHealthy         *bool      `json:"lastHealthy"`
	CooldownUntil       *time.Time `json:"cooldownUntil"`
	LastTriggeredRunID  *uint      `json:"lastTriggeredRunId"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

// HealthResult 是一次健康检查中对单个当前 IP 的结果。
type HealthResult struct {
	TargetKey       string  `json:"targetKey"`
	IPType          string  `json:"ipType"`
	IP              string  `json:"ip"`
	Latency         float64 `json:"latency"`
	Speed           float64 `json:"speed"`
	LossRate        float64 `json:"lossRate"`
	Healthy         bool    `json:"healthy"`
	Status          string  `json:"status"` // healthy/unhealthy/unknown/skipped
	LatencyMeasured bool    `json:"latencyMeasured"`
	SpeedMeasured   bool    `json:"speedMeasured"`
	LossMeasured    bool    `json:"lossMeasured"`
	Detail          string  `json:"detail,omitempty"`
	Reason          string  `json:"reason,omitempty"`
}

// HealthCheck 是一次健康检查批次，不计入普通 Run 统计。
type HealthCheck struct {
	TargetSnapshots   map[string]string `gorm:"serializer:json" json:"targetSnapshots,omitempty"`
	ConfigFingerprint string            `json:"configFingerprint,omitempty"`
	ID                uint              `gorm:"primaryKey" json:"id"`
	TaskID            uint              `gorm:"index" json:"taskId"`
	CheckedAt         time.Time         `gorm:"index" json:"checkedAt"`
	Status            string            `gorm:"size:16" json:"status"`
	FailureReason     string            `json:"failureReason,omitempty"`
	Results           []HealthResult    `gorm:"serializer:json" json:"results"`
	TriggeredRunID    *uint             `json:"triggeredRunId,omitempty"`
	CreatedAt         time.Time         `json:"createdAt"`
}

const (
	StatusQueued   = "queued"
	StatusRunning  = "running"
	StatusSuccess  = "success"
	StatusPartial  = "partial"
	StatusFailed   = "failed"
	StatusCanceled = "canceled"
)

// SpeedResult 为一条测速结果。
type SpeedResult struct {
	IPType   string  `json:"ipType"`
	Rank     int     `json:"rank"`
	IP       string  `json:"ip"`
	Sent     int     `json:"sent"`
	Received int     `json:"received"`
	LossRate float64 `json:"lossRate"`
	Latency  float64 `json:"latency"`
	Speed    float64 `json:"speed"`
	Colo     string  `json:"colo"`
}

// NetworkProbe 为测速前对单一 IP 类型的网络自检结果。
type NetworkProbe struct {
	IPType string `json:"ipType"`
	// Intercepted 表示到保留地址（公网不可达）的 TCP 握手成功，说明连接被本地代理或网关拦截
	Intercepted bool `json:"intercepted"`
	// ConnectMs 为到 Cloudflare IP 的 TCP 握手耗时（多次取最小值），0 表示未连通
	ConnectMs float64 `json:"connectMs"`
	// EgressIP / Loc / Colo 来自 Cloudflare /cdn-cgi/trace，即 Cloudflare 看到的出口
	EgressIP string `json:"egressIP"`
	Loc      string `json:"loc"`
	Colo     string `json:"colo"`
	// LocalIP 为国内回显服务看到的出口；与 EgressIP 不同说明访问 Cloudflare 走了别的线路
	LocalIP string `json:"localIP"`
	Error   string `json:"error,omitempty"`
}

// EgressMismatch 判断两个出口是否都已获得且不一致。
func (p NetworkProbe) EgressMismatch() bool {
	return p.EgressIP != "" && p.LocalIP != "" && p.EgressIP != p.LocalIP
}

// NetworkCheck 为一次执行的网络自检记录。
type NetworkCheck struct {
	Mode   string         `json:"mode"`
	Probes []NetworkProbe `json:"probes"`
	// ProxySuspected 表示判定为误走代理（直连模式下据此跳过 DNS 更新）
	ProxySuspected bool `json:"proxySuspected"`
	// Reason 为判定依据：intercepted（保留地址握手成功）/ latency（测速延迟异常低且一致）
	Reason string `json:"reason,omitempty"`
}

// DNSChange 为一次记录变更（或跳过/失败）。
type DNSChange struct {
	AccountID   uint   `json:"accountId"`
	AccountName string `json:"accountName"`
	FQDN        string `json:"fqdn"`
	Type        string `json:"type"`
	Action      string `json:"action"`
	OldValue    string `json:"oldValue"`
	NewValue    string `json:"newValue"`
	Message     string `json:"message"`
	// MessageKey / MessageArgs 供前端按语言显示 Message；上游原始报错等无固定文案的情况为空
	MessageKey  string         `json:"messageKey,omitempty"`
	MessageArgs map[string]any `json:"messageArgs,omitempty"`
}

// Run 为一次任务执行记录。
type Run struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	TaskID        uint       `gorm:"index" json:"taskId"`
	TaskName      string     `json:"taskName"`
	Trigger       string     `gorm:"size:16" json:"trigger"`
	HealthCheckID *uint      `json:"healthCheckId,omitempty"`
	HealthReason  string     `json:"healthReason,omitempty"`
	DryRun        bool       `gorm:"not null;default:false" json:"dryRun"` // 试运行：只测速，不写 DNS、不通知
	Status        string     `gorm:"size:16;index" json:"status"`
	StartedAt     *time.Time `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
	DurationMs    int64      `json:"durationMs"`
	BestIPv4      string     `gorm:"column:best_ipv4" json:"bestIPv4"`
	BestIPv6      string     `gorm:"column:best_ipv6" json:"bestIPv6"`
	BestLatency   float64    `json:"bestLatency"`
	BestSpeed     float64    `json:"bestSpeed"`
	Changed       bool       `json:"changed"`
	Message       string     `json:"message"`
	// MessageKey / MessageArgs 供前端按语言显示 Message（中文原文仍用于通知与日志）；无固定文案的错误为空
	MessageKey  string         `json:"messageKey,omitempty"`
	MessageArgs map[string]any `gorm:"serializer:json" json:"messageArgs,omitempty"`
	Log         string         `json:"log,omitempty"`
	Results     []SpeedResult  `gorm:"serializer:json" json:"results,omitempty"`
	// ResultTotals 为各 IP 类型实际测得的结果数（Results 只保存前 MaxStoredResults 个）
	ResultTotals map[string]int `gorm:"serializer:json" json:"resultTotals,omitempty"`
	Changes      []DNSChange    `gorm:"serializer:json" json:"changes,omitempty"`
	Network      *NetworkCheck  `gorm:"serializer:json" json:"network,omitempty"`
	CreatedAt    time.Time      `gorm:"index" json:"createdAt"`
}

// SummaryColumns 为列表查询时需要的列（不含大字段）。
var SummaryColumns = []string{"id", "task_id", "task_name", "trigger", "dry_run", "status", "started_at", "finished_at",
	"duration_ms", "best_ipv4", "best_ipv6", "best_latency", "best_speed", "changed", "message", "message_key", "message_args",
	"created_at", "health_check_id", "health_reason"}

// RecordState 记录每个目标最近一次写入的值，用于仪表盘。
type RecordState struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	TaskID    uint      `gorm:"uniqueIndex:idx_record_state" json:"taskId"`
	AccountID uint      `gorm:"uniqueIndex:idx_record_state" json:"accountId"`
	FQDN      string    `gorm:"uniqueIndex:idx_record_state;size:255" json:"fqdn"`
	Type      string    `gorm:"uniqueIndex:idx_record_state;size:8" json:"type"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}
