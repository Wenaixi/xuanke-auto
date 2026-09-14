package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// LoadOrCreateKey 读取 XUANKE_MASTER_KEY（64 位 hex）或自动生成 32 字节密钥保存到 DB 旁。
func LoadOrCreateKey(dbPath string) ([]byte, error) {
	if env := os.Getenv("XUANKE_MASTER_KEY"); env != "" {
		key, err := hex.DecodeString(env)
		if err != nil || len(key) != 32 {
			return nil, errors.New("XUANKE_MASTER_KEY 必须是 64 位十六进制字符串（32 字节）")
		}
		return key, nil
	}
	keyFile := filepath.Join(filepath.Dir(dbPath), ".master_key")
	if b, err := os.ReadFile(keyFile); err == nil {
		// F17-04（第 17 轮）：预生成的密钥文件同样校验 32 字节——此前只校验环境变量。
		// 损坏/截断/空 .master_key 会让 AES-256-GCM 初始化失败，且 store 层无对账，
		// 加密凭据静默无法读取（严重时新部署 WriteFile 用随机 32 字节覆盖损坏文件、
		// 数据库变成"不可解密"永久损坏）。一律显式报错，拒绝带伤启动。
		if len(b) != 32 {
			return nil, errors.New(".master_key 文件必须是 32 字节（损坏或版本不匹配）")
		}
		return b, nil
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(keyFile), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyFile, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

// Encrypt AES-256-GCM 加密：随机 nonce 前置，输出 hex。
func Encrypt(plain string, key []byte) (string, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(blk)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return hex.EncodeToString(out), nil
}

// Decrypt 解密 Encrypt 的输出。
func Decrypt(encHex string, key []byte) (string, error) {
	raw, err := hex.DecodeString(encHex)
	if err != nil {
		return "", err
	}
	blk, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(blk)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("密文长度非法")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
