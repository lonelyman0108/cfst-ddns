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
}

// UpdatePolicy 控制如何把测速结果写入 DNS。
type UpdatePolicy struct {
	RecordCount   int  `json:"recordCount"`
	SkipUnchanged bool `json:"skipUnchanged"`
}

// Target 为一条需要维护的 DNS 记录。
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
	ID          uint            `gorm:"primaryKey" json:"id"`
	Name        string          `gorm:"size:128" json:"name"`
	Enabled     bool            `json:"enabled"`
	Cron        string          `gorm:"size:128" json:"cron"`
	IPType      string          `gorm:"column:ip_type;size:8" json:"ipType"`
	SpeedTest   SpeedTestConfig `gorm:"serializer:json" json:"speedTest"`
	Update      UpdatePolicy    `gorm:"serializer:json" json:"update"`
	Targets     []Target        `gorm:"serializer:json" json:"targets"`
	NotifierIDs []uint          `gorm:"column:notifier_ids;serializer:json" json:"notifierIds"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// 执行状态。
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
}

// Run 为一次任务执行记录。
type Run struct {
	ID          uint          `gorm:"primaryKey" json:"id"`
	TaskID      uint          `gorm:"index" json:"taskId"`
	TaskName    string        `json:"taskName"`
	Trigger     string        `gorm:"size:16" json:"trigger"`
	DryRun      bool          `gorm:"not null;default:false" json:"dryRun"` // 试运行：只测速，不写 DNS、不通知
	Status      string        `gorm:"size:16;index" json:"status"`
	StartedAt   *time.Time    `json:"startedAt"`
	FinishedAt  *time.Time    `json:"finishedAt"`
	DurationMs  int64         `json:"durationMs"`
	BestIPv4    string        `gorm:"column:best_ipv4" json:"bestIPv4"`
	BestIPv6    string        `gorm:"column:best_ipv6" json:"bestIPv6"`
	BestLatency float64       `json:"bestLatency"`
	BestSpeed   float64       `json:"bestSpeed"`
	Changed     bool          `json:"changed"`
	Message     string        `json:"message"`
	Log         string        `json:"log,omitempty"`
	Results     []SpeedResult `gorm:"serializer:json" json:"results,omitempty"`
	Changes     []DNSChange   `gorm:"serializer:json" json:"changes,omitempty"`
	CreatedAt   time.Time     `gorm:"index" json:"createdAt"`
}

// SummaryColumns 为列表查询时需要的列（不含大字段）。
var SummaryColumns = []string{"id", "task_id", "task_name", "trigger", "dry_run", "status", "started_at", "finished_at",
	"duration_ms", "best_ipv4", "best_ipv6", "best_latency", "best_speed", "changed", "message", "created_at"}

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
