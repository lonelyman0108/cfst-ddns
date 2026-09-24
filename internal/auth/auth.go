// Package auth 实现单用户认证（bcrypt + JWT）。
package auth

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/lonelyman0108/cfst-ddns/internal/store"
)

// TokenTTL 为令牌有效期。
const TokenTTL = 7 * 24 * time.Hour

// Service 负责账号与令牌。
type Service struct {
	store  *store.Store
	secret []byte

	mu       sync.Mutex
	failures map[string][]time.Time // 按客户端 IP 记录登录失败，简单限流
}

func New(st *store.Store) (*Service, error) {
	sec, err := st.JWTSecret()
	if err != nil {
		return nil, err
	}
	return &Service{store: st, secret: sec, failures: map[string][]time.Time{}}, nil
}

type claims struct {
	Ver int `json:"ver"`
	jwt.RegisteredClaims
}

// Token 为登录结果。
type Token struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	Username  string    `json:"username"`
}

// ValidateCredentials 校验用户名与密码格式。
func ValidateCredentials(username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" || utf8.RuneCountInString(username) > 32 {
		return errors.New("用户名长度需为 1-32 个字符")
	}
	if utf8.RuneCountInString(password) < 6 {
		return errors.New("密码至少 6 位")
	}
	return nil
}

// Setup 创建管理员（仅未初始化时）。
func (s *Service) Setup(username, password string) (*Token, error) {
	if s.store.Initialized() {
		return nil, errors.New("系统已初始化")
	}
	if err := ValidateCredentials(username, password); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &store.User{Username: strings.TrimSpace(username), PasswordHash: string(hash)}
	if err := s.store.DB.Create(u).Error; err != nil {
		return nil, err
	}
	return s.issue(u)
}

// Login 校验密码并签发令牌；同一 IP 15 分钟内失败 10 次后暂时锁定。
func (s *Service) Login(ip, username, password string) (*Token, error) {
	if s.locked(ip) {
		return nil, errors.New("登录失败次数过多，请 15 分钟后再试")
	}
	u, err := s.store.GetUser()
	if err != nil {
		return nil, errors.New("系统未初始化")
	}
	if u.Username != strings.TrimSpace(username) ||
		bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		s.fail(ip)
		return nil, errors.New("用户名或密码错误")
	}
	s.mu.Lock()
	delete(s.failures, ip)
	s.mu.Unlock()
	return s.issue(u)
}

func (s *Service) locked(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	cut := time.Now().Add(-15 * time.Minute)
	var recent []time.Time
	for _, t := range s.failures[ip] {
		if t.After(cut) {
			recent = append(recent, t)
		}
	}
	s.failures[ip] = recent
	return len(recent) >= 10
}

func (s *Service) fail(ip string) {
	s.mu.Lock()
	s.failures[ip] = append(s.failures[ip], time.Now())
	s.mu.Unlock()
}

// ChangePassword 修改密码并使旧令牌失效，返回新令牌。
func (s *Service) ChangePassword(oldPwd, newPwd string) (*Token, error) {
	u, err := s.store.GetUser()
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPwd)) != nil {
		return nil, errors.New("原密码错误")
	}
	return s.SetPassword(u.Username, newPwd)
}

// SetPassword 直接重置用户名与密码（CLI reset-password 使用），不存在则创建。
func (s *Service) SetPassword(username, password string) (*Token, error) {
	if err := ValidateCredentials(username, password); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u, err := s.store.GetUser()
	if err != nil {
		u = &store.User{}
	}
	u.Username = strings.TrimSpace(username)
	u.PasswordHash = string(hash)
	u.TokenVersion++
	if err := s.store.DB.Save(u).Error; err != nil {
		return nil, err
	}
	return s.issue(u)
}

func (s *Service) issue(u *store.User) (*Token, error) {
	exp := time.Now().Add(TokenTTL)
	c := claims{Ver: u.TokenVersion, RegisteredClaims: jwt.RegisteredClaims{
		Subject: u.Username, ExpiresAt: jwt.NewNumericDate(exp), IssuedAt: jwt.NewNumericDate(time.Now())}}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(s.secret)
	if err != nil {
		return nil, err
	}
	return &Token{Token: tok, ExpiresAt: exp, Username: u.Username}, nil
}

// Verify 校验令牌并返回用户名。
func (s *Service) Verify(token string) (string, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected alg %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return "", errors.New("登录已失效，请重新登录")
	}
	u, err := s.store.GetUser()
	if err != nil || u.Username != c.Subject || u.TokenVersion != c.Ver {
		return "", errors.New("登录已失效，请重新登录")
	}
	return u.Username, nil
}
