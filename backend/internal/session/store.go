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
	// sweeperStop / sweeperDone：周期清扫协程控制（M-7 会话过期后台清扫）。
	sweeperStop  chan struct{}
	sweeperDone  chan struct{}
	sweeperClose sync.Once
}

// ticket 一次激活票据：绑定账号，只能使用一次。
type ticket struct {
	account string
	expires time.Time
	used    bool
}

// New 创建会话存储，ttl 为会话有效期，并启动周期清扫协程（M-7：后台定期
// 删除过期会话与过期激活票据，杜绝令牌表无限膨胀）。ttl<=0 时不启动清扫
// （零 TTL 测试场景——会话立即过期，清扫无意义）。
func New(ttl time.Duration) *Store {
	s := &Store{sessions: make(map[string]*Session), tickets: make(map[string]*ticket), ttl: ttl}
	if ttl > 0 {
		s.sweeperStop = make(chan struct{})
		s.sweeperDone = make(chan struct{})
		go s.sweepLoop()
	}
	return s
}

// sweepLoop 周期清扫：每 5 分钟删除过期会话与过期票据（防令牌表无限膨胀）。
func (s *Store) sweepLoop() {
	defer close(s.sweeperDone)
	tk := time.NewTicker(5 * time.Minute)
	defer tk.Stop()
	for {
		select {
		case <-tk.C:
			s.sweepExpired()
		case <-s.sweeperStop:
			return
		}
	}
}

// sweepExpired 删除全部过期会话与过期票据（锁内线性扫描，表小可接受）。
func (s *Store) sweepExpired() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for tok, sess := range s.sessions {
		if now.After(sess.Expires) {
			delete(s.sessions, tok)
		}
	}
	for tok, t := range s.tickets {
		if now.After(t.expires) {
			delete(s.tickets, tok)
		}
	}
}

// Close 停止清扫协程并等待退出（进程退出时调用，测试 Cleanup 防泄漏）。
func (s *Store) Close() {
	s.sweeperClose.Do(func() {
		if s.sweeperStop == nil {
			return
		}
		close(s.sweeperStop)
		<-s.sweeperDone
	})
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

// ConsumeTicket 校验并校验性消费激活票据：存在、未用尽、未过期，且绑定账号一致。
// 注意：本方法只做"校验并占用"，不落任何持久化副作用——调用方（handleActivate）在
// 校验成功后紧接着调 ConsumeActivationCode 真正消耗激活码并签发会话。
// 消费成功即作废该票据（一次登录一次激活，重复使用返回错误）。
// F13-m1：票据在激活码校验失败时（无效/用尽/已激活）已被此步占用销毁——
// 调用方随后返回错误文案且不签发会话，用户必须重新登录拿新票据再试。这是"票据单次、
// 防重放"的刻意设计决策：宁可输错激活码重登一次，也不让同一票据反复探测不同激活码
// （票据 5 分钟 TTL 内可被重放穷举）。该契约已在 docs/review-round13.md 与 CLAUDE.md 落盘。
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

// CreateAdmin 签发管理员会话（账号绑定配置的管理员账号名，Admin=true）。
// 默认 admin；改名后（XUANKE_ADMIN_NAME）仍以配置名绑定，杜绝"会话账号=字面量 admin"
// 与"IsAdminAccountName=配置名"两套真相错位（M-3）。
func (s *Store) CreateAdmin(name string) string {
	return s.create(name, true)
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
