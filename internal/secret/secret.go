// Package secret 使用 AES-256-GCM 加密数据库中的敏感配置。
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const prefix = "enc:v1:"

// Box 负责加解密。
type Box struct{ aead cipher.AEAD }

// New 由任意长度的口令派生 32 字节密钥。
func New(passphrase string) (*Box, error) {
	if passphrase == "" {
		return nil, errors.New("加密密钥为空")
	}
	key := sha256.Sum256([]byte(passphrase))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// LoadOrCreate 优先使用传入的口令；为空时读取或生成 keyFile。
func LoadOrCreate(passphrase, keyFile string) (*Box, error) {
	if passphrase != "" {
		return New(passphrase)
	}
	b, err := os.ReadFile(keyFile)
	if err == nil {
		return New(strings.TrimSpace(string(b)))
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	k := hex.EncodeToString(raw)
	if err := os.MkdirAll(filepath.Dir(keyFile), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyFile, []byte(k+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("写入密钥文件失败: %w", err)
	}
	return New(k)
}

// Encrypt 返回带前缀的 base64 密文。
func (b *Box) Encrypt(plain []byte) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := b.aead.Seal(nonce, nonce, plain, nil)
	return prefix + base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt 解密 Encrypt 的输出；无前缀的值视为明文原样返回。
func (b *Box) Decrypt(s string) ([]byte, error) {
	if !strings.HasPrefix(s, prefix) {
		return []byte(s), nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, prefix))
	if err != nil {
		return nil, err
	}
	ns := b.aead.NonceSize()
	if len(raw) < ns {
		return nil, errors.New("密文格式错误")
	}
	plain, err := b.aead.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return nil, errors.New("解密失败：加密密钥与数据不匹配（是否更换了 secret.key / CFST_DDNS_SECRET？）")
	}
	return plain, nil
}
