package accounts

import (
	"log"
	"sync"

	"xuanke-auto/backend/internal/scheduler"
	"xuanke-auto/backend/internal/zhidao"
)

// Store 凭据持久化最小接口（store.Store 实现）。
type Store interface {
	SaveCredential(acct, passwordEnc, idToken string) error
	LoadCredentials() ([]Credential, error)
}

// Credential 账号凭据（与 store.Credential 同构，避免包循环）。
type Credential struct {
	Account     string
	PasswordEnc string
	IDToken     string
}

// Manager 多账号客户端注册表：每个账号一个独立 zhidao.Client（独立 token/cookie 会话）。
// 账号 A 的请求绝不携带账号 B 的会话——物理隔离的核心。
type Manager struct {
	baseURL string
	vision  zhidao.VisionConfig
	st      Store

	mu      sync.Mutex
	clients map[string]*zhidao.Client // 账号名 -> 独立客户端
	order   []string                  // 登录顺序
}

// New 创建多账号客户端注册表。
func New(baseURL string, vision zhidao.VisionConfig, st Store) *Manager {
	return &Manager{
		baseURL: baseURL,
		vision:  vision,
		st:      st,
		clients: make(map[string]*zhidao.Client),
	}
}

// ensure 返回账号对应的独立客户端（不存在则创建空壳）。
func (m *Manager) ensure(acct string) *zhidao.Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.clients[acct]; ok {
		return c
	}
	c := zhidao.New(m.baseURL, m.vision)
	m.clients[acct] = c
	m.order = append(m.order, acct)
	return c
}

// ClientFor 返回指定账号的独立客户端。
func (m *Manager) ClientFor(acct string) (scheduler.Client, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.clients[acct]
	return c, ok
}

// AnyClient 返回任一已登录账号客户端（课程数据全校共享，任一账号可探测）。
func (m *Manager) AnyClient() (scheduler.Client, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.order) == 0 {
		return nil, false
	}
	return m.clients[m.order[0]], true
}

// Registered 返回已注册账号（按登录顺序）。
func (m *Manager) Registered() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.order))
	copy(out, m.order)
	return out
}

// LoginByPassword 用账密登录该账号独立客户端；成功后加密密码与 token 落库。
func (m *Manager) LoginByPassword(acct, password string, encrypt func(string) (string, error)) (string, error) {
	c := m.ensure(acct)
	token, err := c.Login(acct, password)
	if err != nil {
		return "", err
	}
	if m.st != nil && encrypt != nil {
		if enc, err := encrypt(password); err == nil {
			if err := m.st.SaveCredential(acct, enc, token); err != nil {
				log.Printf("[accounts] 持久化凭据失败: %v", err)
			}
		}
	}
	return token, nil
}

// Restore 重启时用持久化凭据恢复各账号客户端（密码解密后在内存中，仅用于自动重登）。
func (m *Manager) Restore(creds []Credential, decrypt func(string) (string, error)) {
	for _, cd := range creds {
		c := m.ensure(cd.Account)
		pwd := ""
		if decrypt != nil {
			if p, err := decrypt(cd.PasswordEnc); err == nil {
				pwd = p
			}
		}
		c.SetCredentials(cd.Account, pwd, cd.IDToken)
		c.SetCookies(map[string]string{
			"access_limit_cookie": "***REMOVED***",
			"zd_edu_cookie":       cd.IDToken,
		})
		log.Printf("[accounts] 恢复账号 %s 的会话（token %s）", cd.Account, tokenShort(cd.IDToken))
	}
}

func tokenShort(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8] + "..."
}
