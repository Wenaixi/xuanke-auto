package accounts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xuanke-auto/backend/internal/zhidao"
)

// fakeStore 内存假凭据持久化（记录 SaveCredential 调用，供断言"有效登录才落库"）。
type fakeStore struct {
	saved bool
}

func (f *fakeStore) SaveCredential(acct, passwordEnc, idToken string) error {
	f.saved = true
	return nil
}

func (f *fakeStore) LoadCredentials() ([]Credential, error) { return nil, nil }

// loginRejectSrv 假教务平台：验证码识别恒成功、doLogin 恒拒绝——
// 用于构造"登录失败"路径（密码错/平台瞬时拒绝），失败前先按真实客户端注册。
func loginRejectSrv(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{"message": map[string]any{"content": "abcd"}}},
			})
		case strings.HasSuffix(r.URL.Path, "/login/doLogin"):
			// 恒拒绝：构造登录失败
			json.NewEncoder(w).Encode(map[string]any{"code": 1, "isOk": false, "msg": "账号或密码错误"})
		default: // /login 与 /login/captcha
			w.Write([]byte("ok"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// seedValidClient 直接把"已持有有效 token 的已注册客户端"注入注册表（等价于
// Restore/SetCredentials 后的工作客户端），模拟该账号此前已正常工作的现场。
func seedValidClient(t *testing.T, m *Manager, srv *httptest.Server, acct string) {
	t.Helper()
	c := zhidao.New(srv.URL, zhidao.VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	c.SetCredentials(acct, "pwd", "tok-valid")
	m.mu.Lock()
	m.clients[acct] = c
	m.order = append(m.order, acct)
	m.mu.Unlock()
}

// TestLoginFailKeepsExistingValidClient B24-01：已持有效 token 的已注册客户端，
// 本次账密登录失败时不得被 B23-02 清理摘除——否则自动抢课静默停摆 + 该账号
// 选课大厅持续报错，直到手动重新登录成功（黄金期手滑输错密码即全程失联）。
// 修复前（无 token 判别直接摘除）：红——已注册客户端被删除，ClientFor 报不存在。
// 修复后（Token()!="" 保留）：绿——客户端仍存在，下个 tick 自动链继续工作。
func TestLoginFailKeepsExistingValidClient(t *testing.T) {
	srv := loginRejectSrv(t)
	m := New(srv.URL, zhidao.VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"}, &fakeStore{})
	seedValidClient(t, m, srv, "acct1")

	if _, err := m.LoginByPassword("acct1", "wrong-pwd", func(s string) (string, error) { return "ENC:" + s, nil }); err == nil {
		t.Fatal("doLogin 恒拒绝：期望登录失败")
	}

	if _, ok := m.ClientFor("acct1"); !ok {
		t.Fatal("已持有效 token 的已注册客户端在本次登录失败后必须保留（B24-01：B23-02 清理误删）")
	}
}

// TestLoginFailRemovesFreshShell B23-02 回归守卫：纯新建的空壳客户端（从未有
// token），登录失败必须被摘除——绝不让空 token 客户端抢占 order[0] 成为全校
// 探测载体（AnyClient 恒返回 ErrUnauthorized，probe 主体每 tick 空转重登、
// lastData 永不刷新）。B24-01 的"token 判别"绝不放行此路径。
func TestLoginFailRemovesFreshShell(t *testing.T) {
	srv := loginRejectSrv(t)
	m := New(srv.URL, zhidao.VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"}, &fakeStore{})

	if _, err := m.LoginByPassword("newbie", "wrong-pwd", func(s string) (string, error) { return "ENC:" + s, nil }); err == nil {
		t.Fatal("doLogin 恒拒绝：期望登录失败")
	}

	if _, ok := m.ClientFor("newbie"); ok {
		t.Fatal("登录失败的空壳客户端必须被摘除（B23-02 契约不可侵犯）")
	}
	if _, ok := m.AnyClient(); ok {
		t.Fatal("登录失败的空壳客户端不得作为 AnyClient 探测载体")
	}
}
