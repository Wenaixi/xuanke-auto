package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Session 一次服务端会话（绑定唯一账号）。
type Session struct {
	Account string
	Expires time.Time
}

// Store 内存会话注册表：随机令牌 -> 账号绑定，过期自动失效。
type Store struct {
	mu       sync.Mutex
	sessions map[string]*Session
	ttl      time.Duration
}

// New 创建会话存储，ttl 为会话有效期。
func New(ttl time.Duration) *Store {
	return &Store{sessions: make(map[string]*Session), ttl: ttl}
}

// Create 为账号签发新会话令牌（32 字节 hex）。
func (s *Store) Create(account string) string {
	token := randToken()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = &Session{Account: account, Expires: time.Now().Add(s.ttl)}
	return token
}

// Account 校验令牌，返回绑定的账号。
func (s *Store) Account(token string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[token]
	if !ok {
		return "", false
	}
	if time.Now().After(sess.Expires) {
		delete(s.sessions, token)
		return "", false
	}
	return sess.Account, true
}

// Delete 注销会话（退出登录）。
func (s *Store) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

func randToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
