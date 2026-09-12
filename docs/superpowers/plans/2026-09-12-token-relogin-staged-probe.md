# 教务令牌自动重登 + 分阶段探测 + 窗口开启提交重试 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan ta***REMOVED***-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让系统在窗口开启时能第一时间抢课（探测分阶段收紧、提交重试脱离节流），并在教务 token 失效时自动重登、前端实时显示 token 有效性。

**Architecture:** 调度器 `tick` 分离「探测」与「提交」两条通道——探测按「平日 30s / 临门 5s / 已到点 5s」分阶段收紧，窗口确认开启后提交重试固定 1 秒一轮（不再被探测节流卡死）。探测命中 `ErrUnauthorized` 时按账号标记 token 失效并异步自动重登（防重入 + 30s 节流），成功后新 token 落库、立即补一次探测。`SchedulerState` 增加 `token_valid` 字段（按账号），前端 Dashboard 运行指标卡片改为实时显示教务令牌有效性。

**Tech Stack:** Go 1.26 标准库（net/http / sync / time）、React 18 + TanStack Query、SQLite（modernc.org/sqlite）。

**Spec:** 无独立 spec 文档；需求来自本轮对话确认：
- 每 30 秒探测时检测 token 可用性，失效自动重登；
- 前端只显示 token 有效性（有效 / 已失效），不显示次数与时间；
- 窗口临门阶段探测收紧（默认临门 5 秒、已到点 5 秒）；
- 窗口开启后提交重试固定 1 秒一轮；
- 探测失败（网络类）不得把已开启的窗口误判为关闭。

## Global Constraints

- 探测与提交必须遵循「失败计入节流」防抖原则（平台"访问过于频繁"1 分钟熔断）。
- 提交动作只允许在 `InDateRange`（平台亲口说开了）为真之后发生；本地时间到点只是触发"立刻去问"。
- 自动重登只在明确的 `ErrUnauthorized` 时触发，网络类失败绝不重登。
- 重登必须异步（不阻塞 300ms tick 主循环）、防重入（同账号并发只登一次）、30s 节流。
- 前端保持纯黑白极简设计规范：单色徽章、tabular-nums、无彩色、无 emoji。
- 代码注释用简体中文；不新增依赖。
- 测试命令：`cd backend && go test ./...`；前端验证：`cd web && npm run build`。

---

### Task 1: 调度器「提交通道」——窗口开启后提交重试脱离 30 秒探测节流

**Files:**
- Modify: `backend/internal/scheduler/scheduler.go:245-296`（tick）

**Interfaces:**
- Produces: 无新签名。`tick` 内部语义变化：窗口开启后，即使距上次探测不足 30 秒，也照常执行 `submitAll()`（若尚未开启则维持现状安静等待）。

**问题根因:** 现在 `tick` 开头 `probe` 闸门不过（距上次探测 < 30s）就 `return`，窗口开启后若上次探测刚过 30s，会白白等满 30 秒才提交下一轮。窗口开启的黄金 30 秒里只能提交 1 轮。

**改法:** 把「探测」与「提交」拆成两段。`tick` 先按需探测刷新快照与 `WindowOpened`；然后**无论本轮是否执行了探测**，只要 `WindowOpened` 为真就调 `submitAll()`。为控制平台压力，给 `submitAll` 加一个最小间隔（1 秒）闸门：距上次提交不足 1 秒则跳过本轮。

- [ ] **Step 1: 写失败测试（窗口开启后 30 秒内应能再次提交）**

```go
// TestWindowOpenRetriesWithoutWaitingProbe: 窗口开启后，即使探测被 30s 节流挡住，也应每 1 秒重试提交（网络失败 → 1 秒后重试成功）
func TestWindowOpenRetriesWithoutWaitingProbe(t *testing.T) {
	fc := newFakeClient(false)
	fc.selectErr[61115] = errors.New("connection reset") // 第一次提交失败（网络类）
	s := New(&fakeAccts{fc}, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()

	setAllOpened(fc)
	s.resetProbe()
	waitStatusAcct(t, s, "acct1", 61115, "failed", 3*time.Second)
	// 清除错误：下一次 1 秒重试应成功（不需要等待 30s 探测闸门）
	fc.mu.Lock()
	delete(fc.selectErr, 61115)
	fc.mu.Unlock()
	waitStatusAcct(t, s, "acct1", 61115, "success", 3*time.Second)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/scheduler/ -run TestWindowOpenRetriesWithoutWaitingProbe -v`
Expected: FAIL（当前实现 30 秒内只提交 1 轮，第二次提交被探测节流挡住）

- [ ] **Step 3: 实现——重构 tick 为「探测 + 提交」两段**

在 `scheduler.go` 顶部加常量：

```go
// submitInterval 窗口开启后提交重试最小间隔：1 秒（黄金期高频但不打爆平台）。
const submitInterval = time.Second
```

在 `Scheduler` struct 增加字段：

```go
lastSubmit time.Time // 上次提交时间（submitAll 节流）
```

重构 `tick` 与抽出的 `probe()`：

```go
// tick 单次轮询：先按需探测刷新窗口状态与课程快照，窗口开启后按 1 秒间隔持续提交。
func (s *Scheduler) tick() {
	now := time.Now()
	s.mu.Lock()
	last := s.lastProbe
	s.mu.Unlock()

	// 探测闸门：距上次探测不足当前阶段间隔且非首次则跳过；窗口到点后首 tick 立即探测
	probe := last.IsZero() || now.Sub(last) >= s.probeIntervalFor(now)
	// 超高性能：窗口到点后的首次 tick 立即探测（不等待节流闸门放过）
	if !probe && now.After(s.openTime) && last.Before(s.openTime.Add(-time.Second)) {
		probe = true
	}
	if probe {
		s.probe()
	}

	s.mu.Lock()
	opened := s.state.WindowOpened
	s.mu.Unlock()
	if !opened {
		return
	}
	// 提交重试闸门：距上次提交不足 1 秒则跳过本轮（探测节流不影响提交）
	s.mu.Lock()
	lastSubmit := s.lastSubmit
	s.mu.Unlock()
	if !lastSubmit.IsZero() && now.Sub(lastSubmit) < submitInterval {
		return
	}
	s.submitAll()
}

// probe 执行一次课程探测并刷新快照与窗口状态。
func (s *Scheduler) probe() {
	now := time.Now()
	client, ok := s.clients.AnyClient()
	if !ok {
		return // 尚无账号登录，安静等待
	}
	data, err := client.FindElectives()
	if err != nil {
		s.mu.Lock()
		s.lastProbe = now // 失败同样计入节流闸门，网络故障时不会疯狂重试
		s.state.WindowOpened = false
		s.mu.Unlock()
		if !errors.Is(err, zhidao.ErrUnauthorized) {
			log.Printf("[scheduler] 查询课程失败: %v", err)
		}
		return
	}
	s.mu.Lock()
	s.lastProbe = now
	s.lastData = data
	s.lastDataAt = now
	opened := false
	for _, p := range data.Publishes {
		if p.InDateRange {
			opened = true
			break
		}
	}
	s.state.WindowOpened = opened
	s.mu.Unlock()
}
```

（注意：原 `tick` 里"失败计入节流 + `WindowOpened` 置 false"的行为原样保留在 `probe()` 中——网络类失败不会把已开启窗口误判为关闭。当前实现失败分支无条件 `WindowOpened = false`，若担心网络抖动误关窗口，可在失败分支**不清除**已开启状态，只置 `lastProbe`。下面 Step 4 的测试会覆盖此点。）

- [ ] **Step 4: 运行全部调度器测试**

Run: `cd backend && go test ./internal/scheduler/ -v`
Expected: PASS（含现有 TestStateMachine / TestBackupFallbackOnFull 等，以及新测试）

- [ ] **Step 5: 提交**

```bash
cd backend && git add internal/scheduler/scheduler.go internal/scheduler/scheduler_test.go && git commit -m "feat(scheduler): 窗口开启后提交重试脱离 30s 探测节流，固定 1 秒一轮"
```

---

### Task 2: 调度器「分阶段探测」——临门与已到点阶段收紧探测频率

**Files:**
- Modify: `backend/internal/scheduler/scheduler.go`（`probeIntervalFor` 方法 + `tick` 闸门）

**Interfaces:**
- Consumes: Task 1 的 `probe()` / `tick` 结构。
- Produces: 新常量 `probeIntervalFar`（30s）与 `probeIntervalNear`（5s）；新方法 `probeIntervalFor(now time.Time) time.Duration`。

**需求:** 平日维持 30 秒探测；距开放时间 ≤ 5 分钟时收紧到 5 秒；已到点但平台未开时持续 5 秒盯守（直到 `InDateRange` 为真）。

- [ ] **Step 1: 实现——抽出分阶段间隔方法**

在 `scheduler.go` 常量区改造：

```go
// 探测分阶段间隔：平日 30 秒；临门（距开放 ≤5 分钟）与已到点未开 5 秒盯守。
const (
	probeIntervalFar  = 30 * time.Second
	probeIntervalNear = 5 * time.Second
	nearWindow        = 5 * time.Minute // 临门窗口：开放前 5 分钟起收紧
)

// probeIntervalFor 按当前时刻与开放时间的距离选择探测间隔。
func (s *Scheduler) probeIntervalFor(now time.Time) time.Duration {
	if now.After(s.openTime.Add(-nearWindow)) {
		return probeIntervalNear // 临门或已到点：5 秒
	}
	return probeIntervalFar // 平日：30 秒
}
```

改造 `tick` 的探测闸门（Task 1 里已引用 `s.probeIntervalFor(now)`，这里落定）：

```go
probe := last.IsZero() || now.Sub(last) >= s.probeIntervalFor(now)
```

（原 `probeInterval` 常量删除或改为 `probeIntervalFar`。搜索确认无其他引用后删除。）

- [ ] **Step 2: 写阶段间隔测试**

```go
func TestProbeIntervalFor(t *testing.T) {
	open := time.Date(2026, 9, 13, 9, 0, 0, 0, time.Local)
	s := New(&fakeAccts{&fakeClient{}}, &fakeStore{}, open, time.Second)

	// 平日：距开放 >5 分钟 → 30 秒
	far := open.Add(-6 * time.Minute)
	if got := s.probeIntervalFor(far); got != probeIntervalFar {
		t.Fatalf("平日应 30s，实际 %v", got)
	}
	// 临门：距开放 4 分钟 → 5 秒
	near := open.Add(-4 * time.Minute)
	if got := s.probeIntervalFor(near); got != probeIntervalNear {
		t.Fatalf("临门应 5s，实际 %v", got)
	}
	// 已到点：开放后 1 分钟 → 5 秒盯守
	passed := open.Add(time.Minute)
	if got := s.probeIntervalFor(passed); got != probeIntervalNear {
		t.Fatalf("已到点应 5s，实际 %v", got)
	}
}
```

- [ ] **Step 3: 运行调度器测试确认通过**

Run: `cd backend && go test ./internal/scheduler/ -run 'TestProbeIntervalFor|TestStateMachine|TestWindowOpenRetriesWithoutWaitingProbe' -v`
Expected: PASS

- [ ] **Step 4: 提交**

```bash
cd backend && git add internal/scheduler/scheduler.go internal/scheduler/scheduler_test.go && git commit -m "feat(scheduler): 分阶段探测——平日 30s、临门与已到点 5s 收紧"
```

---

### Task 3: 教务 token 失效检测 + 自动重登（后端核心）

**Files:**
- Modify: `backend/internal/scheduler/scheduler.go`（探测分支 + `TokenValidFor` + `maybeRelogin` + Store 接口）
- Modify: `backend/internal/scheduler/scheduler_test.go`（fakeAccts 扩展 + 新测试）
- Modify: `backend/internal/accounts/manager.go`（`AnyClientWithAccount` + `Relogin`）
- Test: `backend/internal/scheduler/scheduler_test.go`

**Interfaces:**
- Consumes:
  - `zhidao.ErrUnauthorized`（`backend/internal/zhidao/client.go:196`）
  - `zhidao.Client.ReloginIfNeeded() (bool, error)`（`client.go:252`）
  - `zhidao.Client.Token() string`（`client.go:69`）
  - `store.Store.UpdateIDToken(acct, idToken string) error`（`store.go:56`，已存在）
- Produces:
  - `accounts.Manager` 新方法 `AnyClientWithAccount() (string, scheduler.Client, bool)` 与 `Relogin(acct string) (bool, error)`
  - `scheduler.AccountClients` 接口新增这两个方法
  - `scheduler.Store` 接口新增 `UpdateIDToken(acct, idToken string) error`（store.Store 已实现）
  - `Scheduler.TokenValidFor(acct string) bool`
  - `SchedulerState.TokenValid bool`（JSON 字段 `token_valid`，按账号，`StateForAccount` 填充）

**需求:** 探测命中 `ErrUnauthorized` → 标记该账号 token 失效 → 异步自动重登（防重入 + 30s 节流）→ 成功后新 token 落库 + 立即补一次探测。网络类失败绝不重登。前端经 `/state` 读到 `token_valid` 显示有效性。

**注意（AnyClient 定位账号问题）:** 现有 `AnyClient()` 只返回客户端不返回账号名。失效时无法知道是哪个账号的 token 失效。最小改法：`Manager` 增加 `AnyClientWithAccount()` 返回 `(acct, client, ok)`——`tick` 用它对失效信号定位账号并只对该账号重登。`Manager.order[0]` 即首个登录账号。

- [ ] **Step 1: 扩展 fake 与写失败测试**

先给 `scheduler_test.go` 的 `fakeAccts` 增加重登能力：

```go
// fakeAccts 伪账号注册表：所有账号共享一个 fakeClient（测试用）。
type fakeAccts struct {
	c        *fakeClient
	relogErr error      // 重登错误（可编程）
	relog    func()     // 重登钩子（可编程，记录是否被调用）
}

func (f *fakeAccts) ClientFor(acct string) (Client, bool) { return f.c, true }
func (f *fakeAccts) AnyClient() (Client, bool)            { return f.c, true }
func (f *fakeAccts) AnyClientWithAccount() (string, Client, bool) { return "acct1", f.c, true }
func (f *fakeAccts) Relogin(acct string) (bool, error) {
	if f.relogErr != nil {
		return false, f.relogErr
	}
	if f.relog != nil {
		f.relog()
	}
	return true, nil
}
```

新测试：

```go
// TestTokenInvalidTriggersRelogin: 探测命中 ErrUnauthorized → 标记失效 → 自动重登 → 恢复有效。
func TestTokenInvalidTriggersRelogin(t *testing.T) {
	fc := newFakeClient(false)
	fc.err = zhidao.ErrUnauthorized // 探测返回失效
	relogged := make(chan bool, 1)
	fa := &fakeAccts{c: fc, relog: func() { relogged <- true }}
	s := New(fa, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()

	// 等探测触发重登
	select {
	case <-relogged:
	case <-time.After(2 * time.Second):
		t.Fatal("探测命中失效后应触发自动重登")
	}
	// 重登前/重登中 token 应显示无效
	if s.TokenValidFor("acct1") {
		t.Fatal("重登完成前 token 应显示无效")
	}
	// 重登成功后（清掉失效错误）再探测恢复有效
	fc.mu.Lock()
	fc.err = nil
	fc.mu.Unlock()
	s.resetProbe()
	time.Sleep(200 * time.Millisecond)
	if !s.TokenValidFor("acct1") {
		t.Fatal("重登成功后 token 应显示有效")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/scheduler/ -run TestTokenInvalidTriggersRelogin -v`
Expected: FAIL（当前无 `TokenValidFor`、无自动重登）

- [ ] **Step 3: 实现——Manager 加重登能力**

`accounts/manager.go` 修改：

```go
// AnyClientWithAccount 返回任一已登录账号的客户端与账号名（课程数据全校共享，任一账号可探测）。
func (m *Manager) AnyClientWithAccount() (string, scheduler.Client, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.order) == 0 {
		return "", nil, false
	}
	acct := m.order[0]
	return acct, m.clients[acct], true
}

// Relogin 对指定账号客户端执行自动重登（返回是否已重登与错误）。
func (m *Manager) Relogin(acct string) (bool, error) {
	m.mu.Lock()
	c, ok := m.clients[acct]
	m.mu.Unlock()
	if !ok {
		return false, fmt.Errorf("账号 %s 未注册", acct)
	}
	return c.ReloginIfNeeded()
}
```

`scheduler.go` 的接口扩展：

```go
// AccountClients 多账号客户端注册表（真实实现 accounts.Manager）。
type AccountClients interface {
	ClientFor(acct string) (Client, bool)
	AnyClient() (Client, bool)
	// AnyClientWithAccount 返回任一已登录账号的客户端与账号名（失效时定位账号用）。
	AnyClientWithAccount() (string, Client, bool)
	// Relogin 对指定账号自动重登（返回是否已重登与错误）。
	Relogin(acct string) (bool, error)
}

// Store 调度器依赖的最小持久化接口（由 store 包实现）。
type Store interface {
	AppendLog(acct string, classID int, action, result string, isOK bool) error
	SaveSuccess(acct string, classID int) error
	UpdateIDToken(acct, idToken string) error // 自动重登后落库新 token
}
```

`scheduler.go` 的 Scheduler 新增字段与初始化：

```go
reloginMu sync.Mutex                 // 重登防重入（全局一把，账号并发低）
reloginAt map[string]time.Time       // [账号] 上次重登时间（30s 节流）
tokenValid map[string]bool           // [账号] token 有效性（未失效 == 有效）
```

`New` 里：

```go
reloginAt:  make(map[string]time.Time),
tokenValid: make(map[string]bool),
```

新增方法与常量：

```go
// TokenValidFor 查询指定账号教务 token 有效性（未记录失效即视为有效）。
func (s *Scheduler) TokenValidFor(acct string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.tokenValid[acct]
}

// maybeRelogin 对指定账号异步自动重登：防重入 + 30s 节流，成功后落库新 token 并补一次探测。
func (s *Scheduler) maybeRelogin(acct string) {
	s.reloginMu.Lock()
	defer s.reloginMu.Unlock()
	if s.tokenValid[acct] {
		return // 已在失效/重登中
	}
	if t, ok := s.reloginAt[acct]; ok && time.Since(t) < reloginInterval {
		return // 30s 节流
	}
	s.reloginAt[acct] = time.Now()
	s.mu.Lock()
	s.tokenValid[acct] = true // 标记失效
	s.mu.Unlock()

	go func() {
		relogged, err := s.clients.Relogin(acct)
		if err != nil {
			log.Printf("[scheduler] 账号 %s 自动重登失败: %v", acct, err)
			return
		}
		if !relogged {
			return
		}
		// 新 token 落库
		if client, ok := s.clients.ClientFor(acct); ok {
			if tok := client.Token(); tok != "" {
				if err := s.store.UpdateIDToken(acct, tok); err != nil {
					log.Printf("[scheduler] 账号 %s 新 token 落库失败: %v", acct, err)
				}
			}
		}
		s.mu.Lock()
		s.tokenValid[acct] = false // 恢复有效
		s.mu.Unlock()
		log.Printf("[scheduler] 账号 %s 教务 token 已自动重登恢复", acct)
		// 重登成功后立即补一次探测（换新 token 后窗口可能已开）
		s.mu.Lock()
		s.lastProbe = time.Time{}
		s.mu.Unlock()
	}()
}

// reloginInterval 重登节流：30 秒内最多重登一次（与探测节流同频，避免频繁登录触发平台限流）。
const reloginInterval = 30 * time.Second
```

（注意 `maybeRelogin` 里 `s.tokenValid[acct]` 的语义：`tokenValid[acct]=false` 表示有效，`=true` 表示失效。因为 map 缺失默认 false==有效，天然合理。上面 Step 1 测试里 `TokenValidFor` 用的是 `!s.tokenValid[acct]`，一致。）

`tick`/`probe` 探测分支改造（`probe()` 里）：

```go
data, err := client.FindElectives()
if err != nil {
	s.mu.Lock()
	s.lastProbe = now
	s.mu.Unlock()
	if errors.Is(err, zhidao.ErrUnauthorized) {
		// 探测账号的 token 失效 → 自动重登
		if acct, _, ok := s.clients.AnyClientWithAccount(); ok {
			s.maybeRelogin(acct)
		}
		return
	}
	log.Printf("[scheduler] 查询课程失败: %v", err)
	return
}
```

（原 `probe()` 失败分支里的 `s.state.WindowOpened = false` 移除——网络类失败/失效不应把已开启窗口误判为关闭。若担心窗口关闭信号丢失，`probe` 成功时才据数据更新 `WindowOpened`。当前已按此改。）

`StateForAccount` 填充 `TokenValid`：

```go
st.TokenValid = !s.tokenValid[acct]
```

`SchedulerState` 增加字段：

```go
TokenValid bool `json:"token_valid"` // 当前账号教务 token 有效性（有效=true）
```

（`tokenValid` map 语义为"失效标记"，`TokenValid` 输出为"有效性布尔"——两者取反。保持内部 map 缺失==有效，外部字段缺失==false==未显示失效，安全。）

- [ ] **Step 4: 运行全部后端测试**

Run: `cd backend && go test ./...`
Expected: PASS（含新测试与全部现有测试）

- [ ] **Step 5: 提交**

```bash
cd backend && git add internal/scheduler/scheduler.go internal/scheduler/scheduler_test.go internal/accounts/manager.go && git commit -m "feat(scheduler): 教务 token 失效自动重登 + token 有效性状态"
```

---

### Task 4: 前端 Dashboard 显示教务令牌有效性

**Files:**
- Modify: `web/src/routes/Dashboard.tsx`（运行指标卡片）
- Modify: `web/src/types.ts`（`SchedulerState` 加 `token_valid`）

**Interfaces:**
- Consumes: Task 3 的 `SchedulerState.TokenValid`（JSON 字段 `token_valid`）。
- Produces: 无。

**需求:** 前端只显示 token 有效性。运行指标卡片把写死的「认证通信链路 / 双通道会话」换成活的徽章：有效 → 白点 + "教务令牌 · 有效"；失效 → 警示 + "教务令牌 · 已失效 · 自动恢复中"。纯黑白极简风格。

- [ ] **Step 1: 改 types.ts**

```ts
export interface SchedulerState {
  open_time: string
  window_opened: boolean
  token_valid: boolean
  courses: CourseStatus[]
}
```

- [ ] **Step 2: 改 Dashboard.tsx 运行指标卡片**

在「运行指标」卡片里，把「认证通信链路」行替换为 token 有效性徽章（保留「双通道会话」行不变）：

```tsx
<div className="py-2.5 flex items-center justify-between">
  <span className="text-neutral-400">教务令牌</span>
  <span className="flex items-center gap-1.5">
    {state?.token_valid === false ? (
      <>
        <span className="inline-block w-1.5 h-1.5 rounded-full bg-white/40 animate-pulse" />
        <span className="text-white/60 font-mono">已失效 · 自动恢复中</span>
      </>
    ) : (
      <>
        <span className="inline-block w-1.5 h-1.5 rounded-full bg-white" />
        <span className="text-white font-mono">有效</span>
      </>
    )}
  </span>
</div>
```

（将原「认证通信链路 / 双通道会话」行替换为「教务令牌」行 + 保留「双通道会话」行。纯黑白、tabular-nums、无彩色、无 emoji。）

- [ ] **Step 3: 前端类型检查 + 构建验证**

Run: `cd web && npm run build`
Expected: tsc 无错误，构建成功

- [ ] **Step 4: 提交**

```bash
cd web && git add src/routes/Dashboard.tsx src/types.ts && git commit -m "feat(web): 控制台实时显示教务令牌有效性（纯黑白徽章）"
```

---

### Task 5: 全量验证与收尾

**Files:**
- 无代码改动；只跑验证。

- [ ] **Step 1: 后端全量测试（含 -race）**

Run: `cd backend && go test ./... -race`
Expected: PASS（scheduler 有并发 goroutine，必须 -race）

- [ ] **Step 2: 前端构建**

Run: `cd web && npm run build`
Expected: 成功，无类型错误

- [ ] **Step 3: 极端情况自查（逐项核对）**

1. **窗口未开 + token 失效**：探测返回 `ErrUnauthorized` → `maybeRelogin` 自动重登 → 重登成功补探测 → 窗口若已开立即提交。✓
2. **窗口已开 + 网络抖动探测失败**：`probe()` 失败分支不再置 `WindowOpened=false` → 下一 tick（300ms）仍提交。✓
3. **重登防重入**：`maybeRelogin` 里 `reloginMu` + `tokenValid[acct]` 双保险——两个 tick 并发不会同时重登。✓
4. **重登 30s 节流**：`reloginAt` 记录上次重登时间，30 秒内不重复重登（Vision 持续失败不轰炸登录接口）。✓
5. **重登成功立即补探测**：`lastProbe = time.Time{}` 让下一 tick 立即探测（换新 token 后窗口可能已开）。✓
6. **token_valid 默认安全**：`TokenValidFor` 缺失即有效（`!s.tokenValid[acct]`），前端 `state?.token_valid === false` 才显示失效——未探测/初始态不误报失效。✓
7. **分阶段探测**：平日 30s、临门 5s、已到点未开 5s，失败计入节流（防 300ms 疯狂重试）。✓
8. **提交 1s 闸门**：窗口开启后每 1 秒至多一轮提交，`done`/`inflight` 去重保证不重复提交已成功课程。✓

- [ ] **Step 4: 更新 CLAUDE.md 项目记忆库**

在「关键决策与系统化调试排错记录」段追加三条：

```markdown
- **教务 token 失效自动重登（取代禁止自动重登）**：探测命中 `ErrUnauthorized` → 按账号标记失效 → 异步自动重登（防重入 + 30 秒节流，Vision 持续失败不轰炸登录接口）→ 新 token 落库（UpdateIDToken）→ 立即补一次探测。网络类失败绝不重登。前端只显示 token 有效性（`token_valid`），不显示次数与时间。
- **窗口开启后提交重试脱离探测节流**：探测（平日 30s / 临门 5 分钟起 5s / 已到点未开 5s 盯守）与提交（窗口开启后固定 1 秒一轮）解耦。提交只认平台 `InDateRange`，本地时间只触发"立刻去问"。
- **探测失败不误判窗口关闭**：`probe()` 网络类失败/失效只计入 `lastProbe` 节流，不再把 `WindowOpened` 置 false——已开启窗口不因一次网络抖动丢失提交机会。
```

- [ ] **Step 5: 提交**

```bash
git add CLAUDE.md && git commit -m "docs: 记录 token 自动重登与分阶段探测设计决策"
```

---
