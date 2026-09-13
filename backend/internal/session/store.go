package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// randReader 可注入的随机源（测试注入失败场景用；生产恒为 crypto/rand）。
var randReader = rand.Read

// Session 一次服务端会话（绑定唯一账号；Admin 标记管理员身份）。
type Session struct {
	Account string
	Admin   bool // 管理员会话（admin 账号 + 管理口令登录签发）
	Expires time.Time
}

// ticketTTL 激活票据有效期：登录后 5 分钟内未激活即失效，防票据泄露长期有效。
const ticketTTL = 5 * time.Minute

// Store 内存会话注册表：随机令牌 -> 账号绑定，过期自动失效。
// ticket 激活票据：登录成功但未激活的账号凭它完成激活（短期单次，C-2）。
type Store struct {
	mu       sync.Mutex
	sessions map[string]*Session
	tickets  map[string]*ticket
	ttl      time.Duration
}

// ticket 一次激活票据：绑定账号，只能使用一次。
type ticket struct {
	account string
	expires time.Time
	used    bool
}

// New 创建会话存储，ttl 为会话有效期。
func New(ttl time.Duration) *Store {
	return &Store{sessions: make(map[string]*Session), tickets: make(map[string]*ticket), ttl: ttl}
}

// CreateTicket 为"刚通过教务登录但尚未激活"的账号签发短期单次激活票据。
// 票据绑定该账号；激活接口校验票据与激活账号一致后才消耗激活码（C-2）。
func (s *Store) CreateTicket(account string) string {
	tok := randToken()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tickets[tok] = &ticket{account: account, expires: time.Now().Add(ticketTTL)}
	return tok
}

// ConsumeTicket 校验并单次消费激活票据：存在、未用尽、未过期，且绑定账号一致。
// 消费成功即作废该票据（一次登录一次激活，重复使用返回错误）。
func (s *Store) ConsumeTicket(token, account string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tickets[token]
	if !ok {
		return errors.New("票据不存在")
	}
	if t.used {
		return errors.New("票据已使用")
	}
	if time.Now().After(t.expires) {
		delete(s.tickets, token)
		return errors.New("票据已过期")
	}
	if t.account != account {
		return errors.New("票据与账号不匹配")
	}
	t.used = true
	delete(s.tickets, token)
	return nil
}

// Create 为账号签发新会话令牌（32 字节 hex，普通用户）。
func (s *Store) Create(account string) string {
	return s.create(account, false)
}

// CreateAdmin 签发管理员会话（账号固定 admin，Admin=true）。
func (s *Store) CreateAdmin() string {
	return s.create("admin", true)
}

func (s *Store) create(account string, admin bool) string {
	token := randToken()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = &Session{Account: account, Admin: admin, Expires: time.Now().Add(s.ttl)}
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

// IsAdmin 校验令牌是否为管理员会话。
func (s *Store) IsAdmin(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(sess.Expires) {
		delete(s.sessions, token)
		return false
	}
	return sess.Admin
}

// IsAdminToken 校验令牌是否有效且为管理员会话（requireAdminSession 用）。
func (s *Store) IsAdminToken(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(sess.Expires) {
		delete(s.sessions, token)
		return false
	}
	return sess.Admin
}

// IsAdminAccount 校验"会话绑定的账号名"（用于穿透判定；与身份无关）。
func (s *Store) IsAdminAccount(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(sess.Expires) {
		delete(s.sessions, token)
		return false
	}
	return sess.Account == "admin"
}

// RevokeAccount 吊销指定账号签发的全部会话（管理员删除账号时调用）。
// 锁内遍历删除，使被删账号既有的浏览器令牌立即失效，等不到 12h TTL。
func (s *Store) RevokeAccount(account string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for tok, sess := range s.sessions {
		if sess.Account == account {
			delete(s.sessions, tok)
		}
	}
}

// Delete 注销会话（退出登录）。
func (s *Store) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

func randToken() string {
	b := make([]byte, 32)
	if _, err := randReader(b); err != nil {
		// 与 config.randomAdminToken 同策略：熵源故障拒绝签发可预测令牌。
		panic("crypto/rand 不可用，无法签发安全会话令牌")
	}
	return hex.EncodeToString(b)
}
