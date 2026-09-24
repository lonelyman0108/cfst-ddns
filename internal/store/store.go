// Package store 封装 SQLite 持久化。
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
	"github.com/lonelyman0108/cfst-ddns/internal/secret"
)

// Store 为数据访问入口。
type Store struct {
	DB  *gorm.DB
	box *secret.Box
}

// ErrNotFound 表示记录不存在。
var ErrNotFound = gorm.ErrRecordNotFound

// Open 打开数据库并执行迁移。
func Open(path string, box *secret.Box) (*Store, error) {
	dsn := path + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1) // SQLite 单写者，避免 SQLITE_BUSY
	if err := db.AutoMigrate(&User{}, &KV{}, &Account{}, &Notifier{}, &Task{}, &Run{}, &RecordState{}); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}
	return &Store{DB: db, box: box}, nil
}

// WithDB 返回使用指定连接（如事务）的副本。
func (s *Store) WithDB(db *gorm.DB) *Store { return &Store{DB: db, box: s.box} }

// Close 关闭数据库。
func (s *Store) Close() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// ---------- 配置加解密 ----------

func (s *Store) sealConfig(cfg schema.Config) (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return s.box.Encrypt(b)
}

func (s *Store) openConfig(enc string) (schema.Config, error) {
	cfg := schema.Config{}
	if enc == "" {
		return cfg, nil
	}
	b, err := s.box.Decrypt(enc)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ---------- 账号 ----------

func (s *Store) ListAccounts() ([]Account, error) {
	var list []Account
	if err := s.DB.Order("id").Find(&list).Error; err != nil {
		return nil, err
	}
	for i := range list {
		cfg, err := s.openConfig(list[i].ConfigEnc)
		if err != nil {
			return nil, err
		}
		list[i].Config = cfg
	}
	return list, nil
}

func (s *Store) GetAccount(id uint) (*Account, error) {
	var a Account
	if err := s.DB.First(&a, id).Error; err != nil {
		return nil, err
	}
	cfg, err := s.openConfig(a.ConfigEnc)
	if err != nil {
		return nil, err
	}
	a.Config = cfg
	return &a, nil
}

func (s *Store) SaveAccount(a *Account) error {
	enc, err := s.sealConfig(a.Config)
	if err != nil {
		return err
	}
	a.ConfigEnc = enc
	return s.DB.Save(a).Error
}

// ---------- 通知 ----------

func (s *Store) ListNotifiers() ([]Notifier, error) {
	var list []Notifier
	if err := s.DB.Order("id").Find(&list).Error; err != nil {
		return nil, err
	}
	for i := range list {
		cfg, err := s.openConfig(list[i].ConfigEnc)
		if err != nil {
			return nil, err
		}
		list[i].Config = cfg
	}
	return list, nil
}

func (s *Store) GetNotifier(id uint) (*Notifier, error) {
	var n Notifier
	if err := s.DB.First(&n, id).Error; err != nil {
		return nil, err
	}
	cfg, err := s.openConfig(n.ConfigEnc)
	if err != nil {
		return nil, err
	}
	n.Config = cfg
	return &n, nil
}

func (s *Store) SaveNotifier(n *Notifier) error {
	enc, err := s.sealConfig(n.Config)
	if err != nil {
		return err
	}
	n.ConfigEnc = enc
	return s.DB.Save(n).Error
}

// ---------- 任务 ----------

func (s *Store) ListTasks() ([]Task, error) {
	var list []Task
	return list, s.DB.Order("id").Find(&list).Error
}

func (s *Store) GetTask(id uint) (*Task, error) {
	var t Task
	if err := s.DB.First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// ---------- 设置 ----------

// Settings 为可在界面修改的系统设置。
type Settings struct {
	GithubMirror         string `json:"githubMirror"`
	HistoryRetentionDays int    `json:"historyRetentionDays"`
	HookEnabled          bool   `json:"hookEnabled"`
	HookToken            string `json:"hookToken"`
	NotifyTitlePrefix    string `json:"notifyTitlePrefix"`
}

const (
	keyMirror    = "github_mirror"
	keyRetention = "history_retention_days"
	keyHookOn    = "hook_enabled"
	keyHookToken = "hook_token"
	keyPrefix    = "notify_title_prefix"
	keyJWTSecret = "jwt_secret"
)

func (s *Store) get(key string) (string, bool) {
	var kv KV
	if err := s.DB.First(&kv, "key = ?", key).Error; err != nil {
		return "", false
	}
	return kv.Value, true
}

func (s *Store) set(tx *gorm.DB, key, value string) error {
	return tx.Save(&KV{Key: key, Value: value}).Error
}

// GetSettings 读取设置，缺省值在此定义。
func (s *Store) GetSettings() Settings {
	st := Settings{HistoryRetentionDays: 30, NotifyTitlePrefix: "CFST DDNS"}
	if v, ok := s.get(keyMirror); ok {
		st.GithubMirror = v
	}
	if v, ok := s.get(keyRetention); ok {
		if n, err := strconv.Atoi(v); err == nil {
			st.HistoryRetentionDays = n
		}
	}
	if v, ok := s.get(keyHookOn); ok {
		st.HookEnabled = v == "true"
	}
	if v, ok := s.get(keyHookToken); ok {
		st.HookToken = v
	}
	if v, ok := s.get(keyPrefix); ok {
		st.NotifyTitlePrefix = v
	}
	return st
}

// SaveSettings 保存设置。
func (s *Store) SaveSettings(st Settings) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		for k, v := range map[string]string{
			keyMirror:    st.GithubMirror,
			keyRetention: strconv.Itoa(st.HistoryRetentionDays),
			keyHookOn:    strconv.FormatBool(st.HookEnabled),
			keyHookToken: st.HookToken,
			keyPrefix:    st.NotifyTitlePrefix,
		} {
			if err := s.set(tx, k, v); err != nil {
				return err
			}
		}
		return nil
	})
}

// JWTSecret 返回签名密钥，不存在时生成。
func (s *Store) JWTSecret() ([]byte, error) {
	if v, ok := s.get(keyJWTSecret); ok && v != "" {
		return hex.DecodeString(v)
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	if err := s.set(s.DB, keyJWTSecret, hex.EncodeToString(b)); err != nil {
		return nil, err
	}
	return b, nil
}

// RandomToken 生成十六进制随机串。
func RandomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------- 用户 ----------

func (s *Store) GetUser() (*User, error) {
	var u User
	if err := s.DB.Order("id").First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) Initialized() bool {
	_, err := s.GetUser()
	return err == nil
}

// ---------- 记录状态 ----------

// UpsertRecordState 保存目标记录最近一次写入的值。
func (s *Store) UpsertRecordState(st RecordState) error {
	st.ID = 0
	st.UpdatedAt = time.Now()
	return s.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_id"}, {Name: "account_id"}, {Name: "fqdn"}, {Name: "type"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&st).Error
}

// ---------- 执行记录 ----------

// PurgeRuns 删除早于指定天数的执行记录。
func (s *Store) PurgeRuns(days int) (int64, error) {
	if days <= 0 {
		return 0, errors.New("天数必须大于 0")
	}
	before := time.Now().AddDate(0, 0, -days)
	res := s.DB.Where("created_at < ? AND status NOT IN ?", before, []string{StatusQueued, StatusRunning}).Delete(&Run{})
	return res.RowsAffected, res.Error
}
