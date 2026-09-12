# 高并发秒杀级架构优化与四大缺陷深度修复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan ta***REMOVED***-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 彻底修复激活码超卖、Token 失效静默、开窗空列表卡死、删号内存残留四大缺陷，并全方位落地连接池预热（压制 1.9s TLS 握手延迟）、服务端时钟毫秒级对齐、开窗前 10 秒 250ms 高频冲刺、骑驴找马自动换课、失败分级智能退避、以及单 exe 双击即用的 ddddocr 本地验证码识别引擎（并发限流默认为 1 且自由可配）。

**Architecture:** 
1. **存储层（Store）**：将激活码消费从事务读改写升级为单条原子条件 `UPDATE ... WHERE used_uses < total_uses` + `RowsAffected()` 判定；升版 Schema v4 增加目标换课开关支持。
2. **调度引擎（Scheduler）**：`ProbeNow` 遇未登录立即触发关联账号异步重登；`tick` 引入开窗到点（本地时间校准后）时间兜底触发，规避平台瞬间下发空列表的冷场；黄金期（开窗前 10 秒）动态提速至 250ms 一轮提交；开窗前 5 分钟起启动静默探测保持长连接热态；增加失败分级退避机制（风控退避 30s，网络抖动快速重试）；实现“骑驴找马”智能换课（持保底，见更优空位退低抢高，失败立即回抢）。
3. **网络与时钟层（Zhidao Client）**：自定义 HTTP Transport，配置 `MaxIdleConnsPerHost: 32`、`IdleConnTimeout: 90s`、`ForceAttemptHTTP2: true`；通过轻量请求读取平台 `Date` 响应头完成时钟毫秒级采样校准。
4. **账号与验证码层（Accounts & Captcha）**：`Manager` 提供线程安全的 `Remove(acct)` 彻底清理内存会话，管理员删号同步调用；抽象 `CaptchaRecognizer` 接口，支持本地内嵌 ddddocr 与硅基流动 Vision 二选一热切换，内置信号量限流（默认并发 1）。
5. **单 exe 交付保障**：ddddocr 模型与 Windows/Linux 动态库通过构建嵌入与运行时自解压缓存技术，保证用户端依然是一个单独的 `xuanke.exe` 双击即可运行，免 Python 环境。

**Tech Stack:** Go 1.26（net/http, sync, database/sql）、React 18 + Vite + Tailwind CSS、SQLite（modernc.org/sqlite）、ONNX Runtime。

## Global Constraints

- 严禁破坏多账号物理隔离：账号 A 的请求绝不可携带账号 B 的任何状态与会话。
- 所有改动遵循“零无故报错、防平台限流”原则：风控类错误必须退避，严禁无脑高频空转轰炸平台。
- 数据库保持不兼容旧库策略：Schema v4 缺列或旧数据结构直接拒绝启动并提示重建。
- 前端严格保持纯黑白极简艺术设计规范（Monochrome Fine Art）：无彩色、无吵闹口号、单色徽章、tabular-nums。
- 交付产物必须保证单二进制（单 exe 双击即用），不能要求普通用户手动配置 Python 或手动放置 dll。
- 每一处非平凡逻辑变动必须有单测守护；测试命令：`cd backend && go test -race ./...`，前端命令：`cd web && npm run build`。

---

### Task 1: 激活码原子扣减防超卖（store.go）

**Files:**
- Modify: `backend/internal/store/store.go:222-246`
- Test: `backend/internal/store/store_test.go:140-174`

**Interfaces:**
- Consumes: `s.db.Begin()`, `tx.Exec()`, `tx.Commit()`
- Produces: `func (s *Store) ConsumeActivationCode(code, acct string) (bool, error)`

**问题根因:** 原实现通过 `SELECT total_uses, used_uses` 读取后在 Go 代码层判断 `used >= total` 再执行 `UPDATE`，在多连接并行事务场景下存在经典的“先查后改”竞态条件，容易超卖。

**改法:** 替换为单条原子条件更新：`UPDATE activation_codes SET used_uses = used_uses + 1 WHERE code = ? AND used_uses < total_uses`，通过 `RowsAffected()` 结果是否为 1 判定是否成功抢占配额，随后插入 `activations` 表。

- [ ] **Step 1: 运行现有多连接并发激活测试，确认现有表现**

运行命令：
```bash
cd backend && go test -v -run TestActivationCodeConcurrentConsume ./internal/store
```

- [ ] **Step 2: 编写原子更新逻辑并替换旧代码**

在 `backend/internal/store/store.go` 中修改 `ConsumeActivationCode`：
```go
func (s *Store) ConsumeActivationCode(code, acct string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	res, err := tx.Exec("UPDATE activation_codes SET used_uses = used_uses + 1 WHERE code = ? AND used_uses < total_uses", code)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}

	if _, err := tx.Exec("INSERT OR IGNORE INTO activations (account) VALUES (?)", acct); err != nil {
		return false, err
	}
	return true, tx.Commit()
}
```

- [ ] **Step 3: 运行测试验证**

运行命令：
```bash
cd backend && go test -v -run TestActivation ./internal/store
```
预期：`TestActivationCodeConcurrentConsume` 与 `TestActivationCodes` 均 PASS。

- [ ] **Step 4: Commit**

```bash
git add backend/internal/store/store.go
git commit -m "fix(store): 激活码消费改为原子条件更新防止并发超卖"
```

---

### Task 2: ProbeNow 遇 token 失效触发自动重登（scheduler.go）

**Files:**
- Modify: `backend/internal/scheduler/scheduler.go:297-313`
- Test: `backend/internal/scheduler/scheduler_test.go`

**Interfaces:**
- Consumes: `s.clients.AnyClientWithAccount()`, `s.maybeRelogin(acct)`, `zhidao.ErrUnauthorized`
- Produces: `ProbeNow() (*zhidao.ElectivesData, error)`

**问题根因:** 用户手动进入选课大厅触发 `ProbeNow` 时，若关联客户端 Token 失效返回 `ErrUnauthorized`，直接返回错误，未触发 `maybeRelogin`，导致用户必须干等调度器下一次常规探测周期。

**改法:** 在 `ProbeNow()` 中捕获 `ErrUnauthorized`，若匹配则通过 `AnyClientWithAccount()` 定位失活账号并调用 `maybeRelogin(acct)` 启动异步恢复流程。

- [ ] **Step 1: 编写失败测试**

在 `backend/internal/scheduler/scheduler_test.go` 中新增 `TestProbeNowTriggersReloginOnUnauthorized`：
```go
func TestProbeNowTriggersReloginOnUnauthorized(t *testing.T) {
	fc := newFakeClient(false)
	fc.err = zhidao.ErrUnauthorized
	relogCh := make(chan bool, 1)
	fa := &fakeAccts{c: fc, relog: func() {
		relogCh <- true
	}}
	s := New(fa, &fakeStore{}, time.Now().Add(time.Hour), time.Hour)
	_, err := s.ProbeNow()
	if err == nil || !errors.Is(err, zhidao.ErrUnauthorized) {
		t.Fatalf("期望返回 ErrUnauthorized, 实际: %v", err)
	}
	select {
	case <-relogCh:
	case <-time.After(time.Second):
		t.Fatal("ProbeNow 遇到 token 失效时未能触发自动重登")
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd backend && go test -v -run TestProbeNowTriggersReloginOnUnauthorized ./internal/scheduler
```

- [ ] **Step 3: 完善 ProbeNow 实现**

在 `backend/internal/scheduler/scheduler.go` 中调整 `ProbeNow`：
```go
func (s *Scheduler) ProbeNow() (*zhidao.ElectivesData, error) {
	if s.clients == nil {
		return nil, errors.New("没有任何已登录账号")
	}
	client, ok := s.clients.AnyClient()
	if !ok || client == nil {
		return nil, errors.New("没有任何已登录账号")
	}
	data, err := client.FindElectives()
	if err != nil {
		if errors.Is(err, zhidao.ErrUnauthorized) {
			if acct, _, ok := s.clients.AnyClientWithAccount(); ok {
				s.maybeRelogin(acct)
			}
		}
		return nil, err
	}
	s.mu.Lock()
	s.lastProbe = time.Now()
	s.lastData = data
	s.lastDataAt = time.Now()
	s.mu.Unlock()
	return data, nil
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd backend && go test -v -run TestProbeNowTriggersReloginOnUnauthorized ./internal/scheduler
```

- [ ] **Step 5: Commit**

```bash
git add backend/internal/scheduler/scheduler.go backend/internal/scheduler/scheduler_test.go
git commit -m "fix(scheduler): ProbeNow 遭遇 Token 失效立即触发自动重登"
```

---

### Task 3: 窗口到点时间兜底启动提交循环（scheduler.go）

**Files:**
- Modify: `backend/internal/scheduler/scheduler.go:334-351`
- Test: `backend/internal/scheduler/scheduler_test.go`

**Interfaces:**
- Consumes: `s.state.WindowOpened`, `s.openTimeNow()`, `time.Now()`
- Produces: `tick()` 提交门禁判定逻辑

**问题根因:** 以前开窗提交唯一的触发条件是 `s.state.WindowOpened == true`（必须依赖探测返回含有 `inDateRange=true` 的发布列表）。如果开放瞬间平台服务器被大量挤爆返回了空数据列表，`WindowOpened` 保持 false，系统会白白错过最黄金的前几秒钟。

**改法:** 提交循环的启动门禁改为复合判定：只要 `s.state.WindowOpened` 为真，或者当前系统时间已经到达或超过开窗时间（`time.Now().After(open)` 或相等），就立即允许进入提交逻辑，不让空数据阻断冲刺。

- [ ] **Step 1: 编写失败测试**

在 `backend/internal/scheduler/scheduler_test.go` 中添加测试：
```go
func TestTickSubmitsWhenTimeReachedEvenIfPublishesEmpty(t *testing.T) {
	fc := newFakeClient(false)
	// 模拟平台到点拉空列表：返回空 Publishes
	fc.mu.Lock()
	fc.data.Publishes = nil
	fc.mu.Unlock()

	openTime := time.Now().Add(-time.Second) // 已经到点
	s := New(&fakeAccts{c: fc}, &fakeStore{}, openTime, 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()

	// 即使快照为空，到点后也应该触发提交尝试
	waitStatusAcct(t, s, "acct1", 61115, "success", 3*time.Second)
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd backend && go test -v -run TestTickSubmitsWhenTimeReachedEvenIfPublishesEmpty ./internal/scheduler
```

- [ ] **Step 3: 修改 tick 门禁**

在 `backend/internal/scheduler/scheduler.go` 的 `tick()` 方法中：
```go
	s.mu.Lock()
	opened := s.state.WindowOpened
	open := s.openTimeNow()
	s.mu.Unlock()

	// 判定窗口是否已开：探测已确认开启 OR 本地时间已到达开放时间点（双保险兜底）
	timeReached := !open.IsZero() && !now.Before(open)
	if !opened && !timeReached {
		return
	}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd backend && go test -v -run TestTickSubmitsWhenTimeReachedEvenIfPublishesEmpty ./internal/scheduler
```

- [ ] **Step 5: Commit**

```bash
git add backend/internal/scheduler/scheduler.go backend/internal/scheduler/scheduler_test.go
git commit -m "fix(scheduler): 窗口到达指定时间后即使数据暂时为空也兜底触发提交"
```

---

### Task 4: 管理员删除账号后调度器与注册表内存清理（accounts + scheduler + api）

**Files:**
- Modify: `backend/internal/accounts/manager.go`
- Modify: `backend/internal/scheduler/scheduler.go`
- Modify: `backend/internal/api/handler.go:452-471`
- Test: `backend/internal/accounts/manager_test.go` (新建或添加), `backend/internal/scheduler/scheduler_test.go`

**Interfaces:**
- Consumes: `accounts.Manager.Remove(acct string)`
- Produces: `Scheduler.RemoveAccount(acct string)`

**问题根因:** 原来在后台删除账号时，仅清理了 SQLite 数据库中的记录，内存中的 `accounts.Manager.clients` 和 `scheduler.acctTargets` 依然驻留，正在执行或者下一个心跳依然会为该账号发起真实提交。

**改法:**
1. `accounts.Manager` 增加 `Remove(acct string)` 方法，安全清理映射表与登录顺序切片；
2. `Scheduler` 增加 `RemoveAccount(acct string)` 方法，清空该账号的目标与状态；并在 `submitAll` 发起前二次核验客户端是否依然存在；
3. `handleAdminDeleteAccount` 在调 DB 删除后，同步调用 `d.Accounts.Remove(req.Account)` 与 `d.Sched.RemoveAccount(req.Account)`。

- [ ] **Step 1: 编写 Manager.Remove 失败测试**

在 `backend/internal/accounts/manager_test.go` 中编写：
```go
func TestManagerRemove(t *testing.T) {
	m := New("https://example.com", zhidao.VisionConfig{}, nil)
	m.ensure("acct1")
	if _, ok := m.ClientFor("acct1"); !ok {
		t.Fatal("acct1 应该存在")
	}
	m.Remove("acct1")
	if _, ok := m.ClientFor("acct1"); ok {
		t.Fatal("acct1 移除后不应存在")
	}
	for _, a := range m.Registered() {
		if a == "acct1" {
			t.Fatal("Registered 中不应再包含 acct1")
		}
	}
}
```

- [ ] **Step 2: 实现 Manager.Remove**

在 `backend/internal/accounts/manager.go` 中添加：
```go
// Remove 移除指定账号的客户端映射与注册顺序。
func (m *Manager) Remove(acct string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, acct)
	newOrder := make([]string, 0, len(m.order))
	for _, a := range m.order {
		if a != acct {
			newOrder = append(newOrder, a)
		}
	}
	m.order = newOrder
}
```

- [ ] **Step 3: 实现 Scheduler.RemoveAccount**

在 `backend/internal/scheduler/scheduler.go` 中添加：
```go
// RemoveAccount 从调度器中彻底注销指定账号的目标与状态。
func (s *Scheduler) RemoveAccount(acct string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.acctTargets, acct)
	delete(s.inflight, acct)
	delete(s.done, acct)
	delete(s.full, acct)
	delete(s.tokenValid, acct)
	delete(s.relogging, acct)

	keep := s.state.Courses[:0]
	for _, c := range s.state.Courses {
		if c.Account != acct {
			keep = append(keep, c)
		}
	}
	s.state.Courses = keep
}
```
并在 `submitAll()` 遍历 `s.acctTargets` 时检查：
```go
		for acct, ts := range s.acctTargets {
			if _, ok := s.clients.ClientFor(acct); !ok {
				continue
			}
			// ... 组装 chains
		}
```

- [ ] **Step 4: 修改 handleAdminDeleteAccount 并串联**

在 `backend/internal/api/handler.go` 中：
```go
		if err := d.Store.DeleteAccount(req.Account); err != nil {
			writeJSON(w, 1, nil, "删除失败: "+err.Error())
			return
		}
		if d.Accounts != nil {
			d.Accounts.Remove(req.Account)
		}
		if d.Sched != nil {
			d.Sched.RemoveAccount(req.Account)
		}
```

- [ ] **Step 5: 验证测试并通过**

```bash
cd backend && go test -v ./internal/accounts ./internal/scheduler
```

- [ ] **Step 6: Commit**

```bash
git add backend/internal/accounts backend/internal/scheduler backend/internal/api/handler.go
git commit -m "fix(admin): 管理员删除账号时同步清理内存客户端与调度目标"
```

---

### Task 5: HTTP/TLS 连接池扩容与开窗前静默预热（client.go + scheduler.go）

**Files:**
- Modify: `backend/internal/zhidao/client.go:37-47`
- Modify: `backend/internal/scheduler/scheduler.go`
- Test: `backend/internal/zhidao/client_test.go`, `backend/internal/scheduler/scheduler_test.go`

**Interfaces:**
- Consumes: `http.Transport{MaxIdleConnsPerHost: 32, IdleConnTimeout: 90s}`, `Client.Prewarm()`
- Produces: 零握手复用长连接

**问题根因:** 实测至道服务器单个 HTTPS 请求握手耗时高达 1.89 秒。Go 原生默认 Client 仅维持每个主机 2 个空闲连接，且开窗前常规 30 秒长轮询极易导致 TCP/TLS 连接被防火墙断开。开窗瞬间几十个请求并发打过去，绝大部分都要重新经历 1.9 秒的 TLS 握手！

**改法:**
1. 为 `zhidao.Client` 统一定制高性能 `http.Transport`，调大 `MaxIdleConnsPerHost: 64`，启用 HTTP/2 和 TCP KeepAlive；
2. 调度器增加开窗前静默预热机制：在距开放时间 ≤2 分钟时，每 15 秒主动发起一次轻量查询，保持出口连接池滚烫；到点瞬间连接立即可用，省下 1.9 秒网络空转。

- [ ] **Step 1: 定制客户端 Transport**

在 `backend/internal/zhidao/client.go` 中改造：
```go
var sharedTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	MaxIdleConnsPerHost:   64,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

func New(baseURL string, visionCfg VisionConfig) *Client {
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout:   15 * time.Second,
			Transport: sharedTransport,
		},
		cookies:   make(map[string]string),
		visionCfg: visionCfg,
	}
}
```

- [ ] **Step 2: 调度器接入临门预热逻辑**

在 `backend/internal/scheduler/scheduler.go` 的 `tick()` 探测逻辑中引入预热保护，确保临门状态持续复用该 Transport。编写预热单测。

- [ ] **Step 3: 运行验证**

```bash
cd backend && go test -v ./internal/zhidao ./internal/scheduler
```

- [ ] **Step 4: Commit**

```bash
git add backend/internal/zhidao/client.go backend/internal/scheduler/scheduler.go
git commit -m "perf(network): 优化 HTTP 连接池并引入临门预热压制 TLS 握手延迟"
```

---

### Task 6: 服务端时钟毫秒级对齐（client.go + scheduler.go）

**Files:**
- Modify: `backend/internal/zhidao/client.go`
- Modify: `backend/internal/scheduler/scheduler.go`
- Test: `backend/internal/scheduler/scheduler_test.go`

**Interfaces:**
- Consumes: HTTP 响应头 `Date`
- Produces: `s.serverClockOffset time.Duration`，在计算倒计时与开窗判定时自动补偿本地误差

**问题根因:** 用户本地电脑时钟可能比教务服务器慢或快数百毫秒甚至数秒。如果本地慢了 500ms，程序就会比实际开窗时间晚半秒才发起抢购。

**改法:**
1. 每次网络请求成功返回时，解析其 `Date` 响应头，结合本地请求时间计算服务器时钟偏移量（`offset = serverTime - localTime`）；
2. 调度器维护平滑更新的 `serverClockOffset`，使得所有时钟判定（`now.Add(offset)`）均对齐至教务服务端真实时刻。

- [ ] **Step 1: 编写时钟校准单测**

- [ ] **Step 2: 在 client.go 中提取 Date 响应头并输出时差**

- [ ] **Step 3: 在 scheduler.go 中引入时差动态补偿**

- [ ] **Step 4: 运行验证通过并 Commit**

```bash
git commit -m "feat(scheduler): 增加教务服务端时钟采样校准与动态时间补偿"
```

---

### Task 7: 窗口开窗黄金期 250ms 冲刺节流（scheduler.go）

**Files:**
- Modify: `backend/internal/scheduler/scheduler.go:58-60, 340-352`
- Test: `backend/internal/scheduler/scheduler_test.go`

**Interfaces:**
- Produces: 动态 `submitInterval`（黄金期 10 秒内 250ms，平时 1s）

**问题根因:** 目前开窗后的 `submitInterval` 固定为 1 秒。在开窗后的前 10 秒“生死黄金期”，每秒只打 1 轮太保守，名额可能在第 300 毫秒就被别人占满。

**改法:**
定义动态提交间隔函数：在窗口确认开启（或时间到达）后的前 10 秒内，提交间隔降低到 250ms（黄金期冲刺），10 秒过后自动平滑恢复为 1 秒，兼顾抢课胜率与防平台风控限流。

- [ ] **Step 1: 编写黄金期提交间隔单测**
- [ ] **Step 2: 实现动态计算逻辑**
- [ ] **Step 3: 验证单测并 Commit**

```bash
git commit -m "perf(scheduler): 开窗前 10 秒启用 250ms 黄金期高频冲刺"
```

---

### Task 8: 提交失败分级智能退避（scheduler.go）

**Files:**
- Modify: `backend/internal/scheduler/scheduler.go:590-637`
- Test: `backend/internal/scheduler/scheduler_test.go`

**Interfaces:**
- Produces: 分级处理策略：Token 失效 -> 自动重登；确认满员 -> 切下一备选；触发风控文案 -> 单课退避 30s；网络异常 -> 快速重试

**问题根因:** 之前所有失败（除了满员和 token 过期）都一视同仁直接报错等下个 tick，若平台返回“访问过于频繁”，继续高频重试会直接导致账号被封禁 30 分钟。

**改法:**
解析 `SelectClass` 返回的错误内容，如果检测到“频繁”或 429 相关错误，为该账号该课程打上 `backoffUntil = now + 30s` 标签，退避期间跳过该课程避免雪崩；对于纯网络超时，则允许黄金期紧凑重试。

- [ ] **Step 1: 编写分级重试测试用例**
- [ ] **Step 2: 实现失败分级与风控退避**
- [ ] **Step 3: 运行验证并通过所有单测**
- [ ] **Step 4: Commit**

```bash
git commit -m "feat(scheduler): 提交失败分级处理与平台风控智能退避"
```

---

### Task 9: 数据库 Schema v4 支持“骑驴找马换课”（db + store）

**Files:**
- Modify: `backend/internal/db/schema.sql`
- Modify: `backend/internal/db/db.go`
- Modify: `backend/internal/store/store.go`
- Test: `backend/internal/store/store_test.go`

**Interfaces:**
- Produces: `targets.allow_swap` 列，`scheduler.Target{AllowSwap bool}`

**改法:**
1. `schema.sql` 中的 `targets` 表添加 `allow_swap INTEGER NOT NULL DEFAULT 0`；
2. `db.go` 的 `refuseLegacy` 中增加对 `targets.allow_swap` 列的检查，升级到 v4 校验；
3. `store.go` 的 `SetTargetsForAccount` / `LoadTargetsForAccount` 读写 `allow_swap` 字段。

- [ ] **Step 1: 修改 schema.sql 和 db.go 校验**
- [ ] **Step 2: 更新 store.go 的读写逻辑**
- [ ] **Step 3: 更新 store_test.go 测试用例并验证**
- [ ] **Step 4: Commit**

```bash
git commit -m "feat(db): 升级数据库 Schema v4 支持课程允许换课配置"
```

---

### Task 10: 调度器“骑驴找马”自动换课引擎（scheduler.go）

**Files:**
- Modify: `backend/internal/scheduler/scheduler.go`
- Test: `backend/internal/scheduler/scheduler_test.go`

**Interfaces:**
- Produces: 退选低优先级课程并秒级冲刺更高优先级目标（失败时自动回抢）

**设计规则:**
- 只有开启了 `AllowSwap` 的目标，才会在已经有选中课程（保底课）的情况下，持续观察快照中更高优先级课程是否有退课空位。
- 一旦发现高优先级课程有名额：原子执行 `ExitClass(保底课)` -> `SelectClass(心仪课)`；若心仪课抢报失败，立即尝试 `SelectClass(保底课)` 回抢保底，全程严格记入日志。

- [ ] **Step 1: 编写换课流程单测（包括成功换课与失败回抢）**
- [ ] **Step 2: 在 spawnChain 中实现换课检测与执行链路**
- [ ] **Step 3: 运行验证通过**
- [ ] **Step 4: Commit**

```bash
git commit -m "feat(scheduler): 实现骑驴找马自动换课与失败回抢防护引擎"
```

---

### Task 11: 前端目标卡片支持“允许换课”配置（Select.tsx + types.ts）

**Files:**
- Modify: `web/src/types.ts`
- Modify: `web/src/routes/Select.tsx`
- Modify: `backend/internal/api/handler.go`

**Interfaces:**
- Produces: UI 目标选中态展示“换课守护”开关选项

- [ ] **Step 1: types.ts 扩展 Target 接口（增加 allow_swap?: boolean）**
- [ ] **Step 2: Select.tsx 增加允许换课开关**
- [ ] **Step 3: 运行 `npm run build` 确保前端类型与构建 100% 通过**
- [ ] **Step 4: Commit**

```bash
git commit -m "feat(web): 选课大厅支持单目标开启骑驴找马换课开关"
```

---

### Task 12: 验证码识别引擎抽象与并发限流信号量（captcha.go）

**Files:**
- Modify: `backend/internal/zhidao/captcha.go`
- Modify: `backend/internal/zhidao/client.go`
- Test: `backend/internal/zhidao/captcha_test.go`

**Interfaces:**
- Produces: 
  ```go
  type CaptchaRecognizer interface {
      Recognize(img []byte) (string, error)
  }
  ```
- 内置全局 `captchaSemaphore = make(chan struct{}, concurrency)` 控制识别并发（默认 1，可配置）。

- [ ] **Step 1: 抽象 CaptchaRecognizer 接口并将原硅基流动识别器重构为 VisionRecognizer**
- [ ] **Step 2: 引入并发限流信号量控制**
- [ ] **Step 3: 单测验证并发限流行为**
- [ ] **Step 4: Commit**

```bash
git commit -m "refactor(captcha): 抽象验证码识别接口并引入并发限流信号量"
```

---

### Task 13: 封装 ddddocr 本地引擎并保证单 exe 运行（local_ocr.go）

**Files:**
- Create: `backend/internal/zhidao/local_ocr.go`
- Create: `backend/internal/zhidao/assets/` (嵌入 onnx 模型和 dll 自解压逻辑)
- Test: `backend/internal/zhidao/local_ocr_test.go`

**Interfaces:**
- Produces: `LocalDdddOcrRecognizer` 结构体，实现 `CaptchaRecognizer` 接口。

**单 exe 保证方案:**
通过 Go 标准库 `//go:embed` 将轻量 OCR 模型内嵌到二进制中。在运行时，如果系统检测到缺少本地推理支持，自动释放并建立缓存运行，完全不需要用户电脑预装 Python 环境，真正保证双击单 exe 直接运行。

- [ ] **Step 1: 编写 local_ocr.go 框架与模型自解压加载器**
- [ ] **Step 2: 实现文字识别核心方法**
- [ ] **Step 3: 编写单测验证模型读取与识别能力**
- [ ] **Step 4: Commit**

```bash
git commit -m "feat(ocr): 实现基于内嵌资产的单 exe 免环境 ddddocr 本地识别引擎"
```

---

### Task 14: 管理员后台引擎热切换与并发数配置（Admin.tsx + runtime）

**Files:**
- Modify: `backend/internal/runtime/config.go`
- Modify: `backend/internal/api/handler.go`
- Modify: `web/src/routes/Admin.tsx`
- Modify: `web/src/types.ts`

**Interfaces:**
- Produces: 管理员可在“系统配置”中二选一选择验证码识别引擎（`ddddocr` 本地 / `vision` 硅基流动），并自由设置识别并发数（默认 1）。

- [ ] **Step 1: runtime.Config 扩展 `CaptchaEngine` 与 `CaptchaConcurrency`**
- [ ] **Step 2: API 接口支持 GET/PUT 读取与修改，热更新至所有客户端**
- [ ] **Step 3: 前端 Admin.tsx 配置选项卡增加引擎下拉选择框与并发输入框**
- [ ] **Step 4: 运行 `npm run build` 与后端单测验证通过**
- [ ] **Step 5: Commit**

```bash
git commit -m "feat(admin): 管理后台支持验证码识别引擎热切换与并发数自定义配置"
```

---

### Task 15: 统一全量回归验证、构建发布与更新项目核心记忆（CLAUDE.md）

**Files:**
- Modify: `CLAUDE.md`
- Build: `backend/xuanke.exe`
- Test: 全库单元测试与代码竞态检查

- [ ] **Step 1: 运行后端全量测试与竞态检测**
```bash
cd backend && go test -race ./...
```
确保全绿无任何竞态与报错。

- [ ] **Step 2: 运行前端全量构建**
```bash
cd web && npm run build
```
确保 Vite 打包与 TypeScript 类型检查 0 报错。

- [ ] **Step 3: 重新编译单二进制嵌入文件**
```bash
cd backend && go build -o xuanke.exe .
```

- [ ] **Step 4: 更新 CLAUDE.md 项目核心记忆**
记录四大缺陷修复、P0 级预热校时冲刺、骑驴找马换课规范、单 exe 运行策略与配置项。

- [ ] **Step 5: 最终归档 Commit**
```bash
git commit -m "chore(release): 完成高并发秒杀级架构优化与四大核心缺陷修复全量交付"
```

---

## 规划自检与覆盖性复核

1. **4 大前置缺陷覆盖**：
   - 激活码超卖 -> Task 1（原子条件更新 `RowsAffected`）
   - ProbeNow 失效不重登 -> Task 2（捕获 ErrUnauthorized 异步重登）
   - 开放瞬间空列表卡死 -> Task 3（到点时间复合兜底启动）
   - 删除账号残留内存提交 -> Task 4（Manager/Scheduler 内存同步移除）
2. **秒杀级性能优化覆盖**：
   - 连接池扩容与预热（压制 1.9s TLS） -> Task 5
   - 服务端时钟对齐 -> Task 6
   - 黄金期 250ms 冲刺 -> Task 7
   - 失败分级与风控退避 -> Task 8
3. **骑驴找马自动换课覆盖**：
   - 数据表 Schema v4 -> Task 9
   - 换课与失败回抢引擎 -> Task 10
   - 前端开关设置 -> Task 11
4. **验证码与单 exe 体验覆盖**：
   - 识别引擎抽象与并发限流 -> Task 12
   - ddddocr 免 Python 单 exe 支持 -> Task 13
   - 管理员二选一热切换与配置 -> Task 14
5. **严谨落地与验证**：
   - 遵循 TDD 模式，每个任务自闭环，先测试后实现，随时可验证。
