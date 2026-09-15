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

// minimalRecognizer 最小 CaptchaRecognizer 替身（区分具体引擎实例）。
type minimalRecognizer struct {
	name string
}

func (r minimalRecognizer) Recognize(img []byte) (string, error) { return "abcd", nil }

// TestNewClientAfterSetRecognizerGetsEngine B29-01：SetRecognizer 必须把引擎同时写进
// m.vision 模板——否则 SetRecognizer 只注入当前已有客户端，此后 ensure 新建客户端经
// zhidao.New(m.baseURL, m.vision) 时 recognizer 恒为 nil：默认兜底仅认 APIKey（SF_API_KEY
// 留空的 ddddocr 典型部署），新账号登录识别直接报"未配置验证码识别引擎"（识别 3 次全败、
// 登录失败），系统从第一个新账号起无法登录任何新账号。既有客户端因 SetRecognizer 注入
// 过引擎不受影响，掩盖了该问题在重启前不被发现。
// 修复前（SetRecognizer 不写模板）：红——新建客户端识别器为 nil。
// 修复后（模板同步 + SetVision 保留引擎）：绿——ddddocr 引擎透传到新客户端。
func TestNewClientAfterSetRecognizerGetsEngine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"isOk":true,"token":"tok-ok"}`))
	}))
	t.Cleanup(srv.Close)
	m := New(srv.URL, zhidao.VisionConfig{BaseURL: srv.URL, APIKey: "", Model: ""}, &fakeStore{})

	// 模拟 initCaptchaAtStartup：配置 ddddocr 引擎（SF_API_KEY 留空的典型部署）
	m.SetRecognizer(minimalRecognizer{name: "ddddocr"})

	// 新账号登录：ensure 走 zhidao.New(m.baseURL, m.vision)，新客户端识别器必须生效
	if _, err := m.LoginByPassword("newbie", "pwd", func(s string) (string, error) { return "ENC:" + s, nil }); err != nil {
		t.Fatalf("新账号登录应成功（识别器必须从模板透传），实际失败: %v", err)
	}
	c, ok := m.ClientFor("newbie")
	if !ok {
		t.Fatal("登录后客户端应已注册")
	}
	cc, _ := c.(*zhidao.Client)
	if r := cc.CurrentRecognizer(); r == nil {
		t.Fatal("SetRecognizer 后新建的客户端必须拿到识别引擎（B29-01：模板 recognizer 恒 nil 致新账号登录全败）")
	} else if l, ok := r.(minimalRecognizer); !ok || l.name != "ddddocr" {
		t.Fatalf("新客户端引擎必须是 SetRecognizer 注入的 ddddocr 实例，实际 %T %#v", r, r)
	}

	// 对偶守卫：热更新 Vision 配置后（SetVision→SetRecognizer 之间），模板引擎仍保留——
	// 若 SetVision 直接覆盖 m.vision，此后再 ensure 的新客户端会短暂拿到 nil 引擎
	m.SetVision(zhidao.VisionConfig{BaseURL: "http://new", APIKey: "", Model: ""})
	m.SetRecognizer(minimalRecognizer{name: "ddddocr"})
	if _, err := m.LoginByPassword("newbie2", "pwd", func(s string) (string, error) { return "ENC:" + s, nil }); err != nil {
		t.Fatalf("SetVision+SetRecognizer 后新账号登录应成功: %v", err)
	}
	c2, ok := m.ClientFor("newbie2")
	if !ok {
		t.Fatal("第二个新账号客户端应已注册")
	}
	cc2, _ := c2.(*zhidao.Client)
	if r := cc2.CurrentRecognizer(); r == nil {
		t.Fatal("SetVision 不得清掉模板引擎（B29-01：新客户端必须继续拿到 ddddocr）")
	}
}
