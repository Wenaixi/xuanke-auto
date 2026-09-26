package accounts

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"xuanke-auto/backend/internal/zhidao"
)

// readyProbe 夹具就绪探测：向 mock 服务器发一条健康请求，把 Windows 回环冷启动窗口
// 前移到夹具构造期（与 api/zhidao 包同款根治——accounts 是唯一
// 无就绪前移的包，两测试 mock /login 首请求 connectex 正是夹具缺口）。
// 连接层失败轮询重试（200ms×10 + 显式 2s 超时，总窗口 ~2s，与 api/zhidao 宽栅栏
// 一次性配平——accounts 曾是收敛后残余面最低收敛点，当前被
// 包序天然保护（store 高耗时后接 zhidao 而非 accounts），包序变化即暴露），全部
// 失败才上抛由调用方 Fatal。
// 三处夹具未有 socketPreheat 双保险（zhidao/api 有 socket 预创建），
// 8 轮全绿实证无残余；若未来 accounts 再出冷启动 flake 第一候选即补 socketPreheat。
func readyProbe(t *testing.T, baseURL string) {
	t.Helper()
	const (
		probeRetries = 10
		probeDelay   = 200 * time.Millisecond
	)
	client := &http.Client{Timeout: 2 * time.Second}
	var lastErr error
	for i := 0; i <= probeRetries; i++ {
		req, err := http.NewRequest(http.MethodGet, baseURL+"/login", nil)
		if err != nil {
			t.Fatalf("就绪探测请求构造失败: %v", err)
		}
		resp, err := client.Do(req)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return
		}
		lastErr = err
		if i < probeRetries {
			time.Sleep(probeDelay)
		}
	}
	t.Fatalf("mock 服务器就绪探测失败: %v", lastErr)
}

// fakeStore 内存假凭据持久化（记录 SaveCredential 调用，供断言"有效登录才落库"）。
type fakeStore struct {
	saved bool
}

func (f *fakeStore) SaveCredential(acct, passwordEnc, idToken string) error {
	f.saved = true
	return nil
}

func (f *fakeStore) LoadCredentials() ([]Credential, error) { return nil, nil }

// visionSrvManager 构造带显式 Vision 识别引擎的 Manager（New 不再静默建引擎，
// 夹具显式注入——模拟生产 initCaptchaAtStartup→applyCaptchaRecognizerFor→SetRecognizer 通道）。
func visionSrvManager(_ *testing.T, baseURL string, st Store) *Manager {
	m := New(baseURL, zhidao.VisionConfig{BaseURL: baseURL, APIKey: "k", Model: "m"}, st)
	m.SetRecognizer(zhidao.NewVisionRecognizer(zhidao.VisionConfig{BaseURL: baseURL, APIKey: "k", Model: "m"}))
	return m
}

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
			// 与真实平台 text/html 差异（识别链路不消费响应体，无害），
			// 补 CT 保持夹具语义对齐
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte("ok"))
		}
	}))
	t.Cleanup(srv.Close)
	readyProbe(t, srv.URL)
	return srv
}

// seedValidClient 直接把"已持有有效 token 的已注册客户端"注入注册表（等价于
// Restore/SetCredentials 后的工作客户端），模拟该账号此前已正常工作的现场。
func seedValidClient(t *testing.T, m *Manager, srv *httptest.Server, acct string) {
	t.Helper()
	c := zhidao.New(srv.URL, zhidao.VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	// zhidao.New 不再静默自建 Vision 引擎——种子客户端登录/重登路径需要识别器，
	// 显式注入（模拟生产 applyCaptchaRecognizerFor 的 SetRecognizer 通道）
	c.SetRecognizer(zhidao.NewVisionRecognizer(zhidao.VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"}))
	c.SetCredentials(acct, "pwd", "tok-valid")
	m.mu.Lock()
	m.clients[acct] = c
	m.order = append(m.order, acct)
	m.mu.Unlock()
}

// TestLoginFailKeepsExistingValidClient：已持有效 token 的已注册客户端，
// 本次账密登录失败时不得被清理摘除——否则自动抢课静默停摆 + 该账号
// 选课大厅持续报错，直到手动重新登录成功（黄金期手滑输错密码即全程失联）。
// 修复前（无 token 判别直接摘除）：红——已注册客户端被删除，ClientFor 报不存在。
// 修复后（Token()!="" 保留）：绿——客户端仍存在，下个 tick 自动链继续工作。
func TestLoginFailKeepsExistingValidClient(t *testing.T) {
	srv := loginRejectSrv(t)
	m := visionSrvManager(t, srv.URL, &fakeStore{})
	seedValidClient(t, m, srv, "acct1")

	if _, err := m.LoginByPassword("acct1", "wrong-pwd", func(s string) (string, error) { return "ENC:" + s, nil }); err == nil {
		t.Fatal("doLogin 恒拒绝：期望登录失败")
	}

	if _, ok := m.ClientFor("acct1"); !ok {
		t.Fatal("已持有效 token 的已注册客户端在本次登录失败后必须保留（清理误删）")
	}
}

// TestLoginFailRemovesFreshShell 回归守卫：纯新建的空壳客户端（从未有
// token），登录失败必须被摘除——绝不让空 token 客户端抢占 order[0] 成为全校
// 探测载体（AnyClient 恒返回 ErrUnauthorized，probe 主体每 tick 空转重登、
// lastData 永不刷新）。"token 判别"绝不放行此路径。
func TestLoginFailRemovesFreshShell(t *testing.T) {
	srv := loginRejectSrv(t)
	m := visionSrvManager(t, srv.URL, &fakeStore{})

	if _, err := m.LoginByPassword("newbie", "wrong-pwd", func(s string) (string, error) { return "ENC:" + s, nil }); err == nil {
		t.Fatal("doLogin 恒拒绝：期望登录失败")
	}

	if _, ok := m.ClientFor("newbie"); ok {
		t.Fatal("登录失败的空壳客户端必须被摘除（契约不可侵犯）")
	}
	if _, ok := m.AnyClient(); ok {
		t.Fatal("登录失败的空壳客户端不得作为 AnyClient 探测载体")
	}
}

// gateSrv 假教务平台：识别恒成功、doLogin 恒放行，并统计 doLogin 调用次数——
// 构造"手动登录（LoginByPassword）是否真实触达平台 doLogin"的断言依据。
func gateSrv(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{"message": map[string]any{"content": "abcd"}}},
			})
		case strings.HasSuffix(r.URL.Path, "/login/doLogin"):
			atomic.AddInt32(&calls, 1)
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "isOk": true, "token": "tok-new"})
		default: // /login 与 /login/captcha
			// 与真实平台 text/html 差异（识别链路不消费响应体，无害）
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte("ok"))
		}
	}))
	t.Cleanup(srv.Close)
	readyProbe(t, srv.URL)
	return srv, &calls
}

// TestLoginByPasswordRejectsWhenGateBudgetExhausted：学生手动登录（LoginByPassword）
// 必须先经全局 doLogin 频率闸门做非阻塞准入——窗口内 quota 已满（gateUsed == gateLoginPerMin）
// 时立即返回明确错误，绝不触达平台 doLogin，杜绝与排队重登并发打爆出口 IP（平台按 IP
// 计"登录失败次数过多"）。管理员换绑同走此收口（低频操作被拦一次重试即可，统一收敛更安全）。
// 修复前（无准入直发）：红——quota 已满仍触达 doLogin。
// 修复后（gateTryAcquire 非阻塞准入）：绿——拒绝且 doLogin 0 次。
func TestLoginByPasswordRejectsWhenGateBudgetExhausted(t *testing.T) {
	srv, calls := gateSrv(t)
	m := visionSrvManager(t, srv.URL, &fakeStore{})

	// 构造窗口 quota 已满现场：gateMu 下把窗口起点拨到当前、gateUsed 置满
	m.gateMu.Lock()
	m.gateWindow = time.Now()
	m.gateUsed = gateLoginPerMin
	m.gateMu.Unlock()

	if _, err := m.LoginByPassword("acct1", "pwd", func(s string) (string, error) { return "ENC:" + s, nil }); err == nil {
		t.Fatal("quota 已满时手动登录必须被拒绝")
	}
	if n := atomic.LoadInt32(calls); n != 0 {
		t.Fatalf("被闸门拒绝时绝不得触达平台 doLogin，实际 %d 次", n)
	}
	// 被拒后不得残留空壳客户端占位（与失败清理同语义）
	if _, ok := m.ClientFor("acct1"); ok {
		t.Fatal("被闸门拒绝不得注册空壳客户端")
	}
}

// TestLoginByPasswordAllowedWhenGateBudgetAvailable 对偶守卫：窗口内 quota 充足时
// 手动登录必须正常放行（且消耗一次预算，与排队重登共享同一闸门计数）——绝不误伤正常登录。
func TestLoginByPasswordAllowedWhenGateBudgetAvailable(t *testing.T) {
	srv, calls := gateSrv(t)
	m := visionSrvManager(t, srv.URL, &fakeStore{})

	// 预置旧窗口（跨分钟）：确保 gateTryAcquire 内部按"窗口已过期重置"分支放行
	m.gateMu.Lock()
	m.gateWindow = time.Now().Add(-2 * time.Minute)
	m.gateUsed = 0
	m.gateMu.Unlock()

	if _, err := m.LoginByPassword("acct1", "pwd", func(s string) (string, error) { return "ENC:" + s, nil }); err != nil {
		t.Fatalf("quota 充足时手动登录应正常成功: %v", err)
	}
	if n := atomic.LoadInt32(calls); n != 1 {
		t.Fatalf("成功登录应恰好触达 1 次 doLogin，实际 %d", n)
	}
	// 消耗的预算必须计入闸门（与 gateWait 共享计数）
	m.gateMu.Lock()
	used := m.gateUsed
	m.gateMu.Unlock()
	if used != 1 {
		t.Fatalf("成功登录应消耗 1 次闸门预算，实际 gateUsed=%d", used)
	}
	if _, ok := m.ClientFor("acct1"); !ok {
		t.Fatal("登录成功后客户端应已注册")
	}
}

// minimalRecognizer 最小 CaptchaRecognizer 替身（区分具体引擎实例）。
type minimalRecognizer struct {
	name string
}

func (r minimalRecognizer) Recognize(img []byte) (string, error) { return "abcd", nil }

// TestNewClientAfterSetRecognizerGetsEngine：SetRecognizer 必须把引擎同时写进
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
	readyProbe(t, srv.URL)
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
		t.Fatal("SetRecognizer 后新建的客户端必须拿到识别引擎（模板 recognizer 恒 nil 致新账号登录全败）")
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
		t.Fatal("SetVision 不得清掉模板引擎（新客户端必须继续拿到 ddddocr）")
	}
}

// TestLoginByPasswordEncryptFailLogs 回归钉：加密函数失败时登录必须仍成功返回
// token（加密失败不阻断登录本身），且凭据绝不落库（fakeStore.saved 保持 false）——
// 自动重登将无保存账密，日志是唯一审计线索（决策锚 17 零吞错对称）。
// 修复前（无 else log）：退化为静默吞错，无测试覆盖。
func TestLoginByPasswordEncryptFailLogs(t *testing.T) {
	srv, calls := gateSrv(t)
	m := visionSrvManager(t, srv.URL, &fakeStore{})

	m.gateMu.Lock()
	m.gateWindow = time.Now().Add(-2 * time.Minute)
	m.gateUsed = 0
	m.gateMu.Unlock()

	tok, err := m.LoginByPassword("acct1", "pwd", func(s string) (string, error) {
		return "", fmt.Errorf("encrypt boom")
	})
	if err != nil {
		t.Fatalf("加密失败不应阻断登录成功: %v", err)
	}
	if n := atomic.LoadInt32(calls); n != 1 {
		t.Fatalf("应恰好触达 1 次 doLogin，实际 %d", n)
	}
	st := m.st.(*fakeStore)
	if st.saved {
		t.Fatal("加密失败时凭据不得落库")
	}
	if tok == "" {
		t.Fatal("登录应返回有效 token")
	}
}
