package secure

import (
	"path/filepath"
	"testing"
)

// TestEncryptDecryptRoundTrip 加密→解密往返，结果必须一致。
func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	plain := "这是主人的密码!Abc@123"
	enc, err := Encrypt(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	if enc == plain {
		t.Fatal("密文不应等于明文")
	}
	dec, err := Decrypt(enc, key)
	if err != nil {
		t.Fatal(err)
	}
	if dec != plain {
		t.Fatalf("往返失败: %q != %q", dec, plain)
	}
}

// TestEncryptUniqueNonce 同一明文两次加密结果不同（随机 nonce）。
func TestEncryptUniqueNonce(t *testing.T) {
	key := make([]byte, 32)
	a, _ := Encrypt("same", key)
	b, _ := Encrypt("same", key)
	if a == b {
		t.Fatal("随机 nonce 应产生不同密文")
	}
}

// TestDecryptTamper 篡改密文必须报错（GCM 认证失败）。
func TestDecryptTamper(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := Encrypt("secret", key)
	tampered := enc
	// 翻转最后一个字符
	b := []byte(tampered)
	b[len(b)-1] ^= 1
	if _, err := Decrypt(string(b), key); err == nil {
		t.Fatal("篡改密文应解密失败")
	}
}

// TestLoadOrCreateKey 主密钥长度校验 + 自动生成/复用。
func TestLoadOrCreateKey(t *testing.T) {
	t.Setenv("XUANKE_MASTER_KEY", "")
	dbPath := filepath.Join(t.TempDir(), "data", "test.db")
	k1, err := LoadOrCreateKey(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(k1) != 32 {
		t.Fatalf("密钥应为 32 字节，实际 %d", len(k1))
	}
	// 再次调用应复用已生成密钥
	k2, err := LoadOrCreateKey(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for i := range k1 {
		if k1[i] != k2[i] {
			t.Fatal("密钥应持久化复用")
		}
	}
	// 非法环境变量
	t.Setenv("XUANKE_MASTER_KEY", "abc")
	if _, err := LoadOrCreateKey(dbPath); err == nil {
		t.Fatal("非法主密钥应报错")
	}
}
