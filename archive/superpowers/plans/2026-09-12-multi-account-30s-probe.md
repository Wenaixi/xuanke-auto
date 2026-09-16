# 多账号目标隔离 + 30 秒查询降频 实施计划

> **面向 Agent 执行者：** 必须使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 按任务逐项执行本计划。步骤使用复选框（`- [ ]`）语法进行跟踪。

**目标：** 将调度器课程探测频率从 300ms/2s 降到固定 **30 秒**一次，从根因上消除平台"访问过于频繁"1 分钟熔断导致的选课大厅数据拉空；同时让多账号的**目标课程与抢课状态按账号隔离**（各账号互不冲突、各自独立），并通过前端账号下拉切换查看各账号的目标与状态。

**架构：**
- **不引入新包、不新增表**（简洁优先）：保持单个共享 Scheduler + 单个共享 zhidao Client。
- 目标持久化按账号隔离：`targets` 表增加 `account TEXT NOT NULL DEFAULT ''` 列（旧单账号数据自动归到空账号，行为不破坏）。store 增加 `SetTargetsForAccount(acct)` / `LoadTargetsForAccount(acct)`。
- API 层：`PUT /api/targets` 请求体加 `account` 字段 → 按账号保存目标；`GET /api/state` 接受 `?account=` 参数 → 返回该账号的目标与状态（调度器保存各账号的目标快照）。`GET /api/electives` 课程数据本身全校共享（同一平台同一学期数据），不需要按账号路由。
- `Scheduler` 探测节流改为 **30 秒常量**（`probeInterval`）；窗口开启后仍立即提交该账号目标。
- 前端：登录后调 `/api/accounts` 得到已保存账号列表（新增 `GET /api/accounts`，返回 targets 表中的 account 去重列表）；顶栏账号下拉切换，`/state` 与 `/targets` 带 `account` 参数；`Login` 登录成功自动加入账号下拉。
- `Login` handler 保存账号到新 `accounts` 表（仅账号名，不存密码），用于账号下拉与重启恢复。

**技术栈：**
- Go 1.26、`net/http`（Go 1.22+ 增强路由）、`modernc.org/sqlite`（纯 Go 免 CGO）
- React 18、TypeScript 5、Vite 5、Radix Primitives、TanStack Query

**规格：** 本计划实现用户需求原文（"查询课程不用这么频繁啊，30s一次就行，各个账号不冲突，都独立"；"我希望逻辑改成自动维护多账号啊，就是只要登录就自动加账号了"）。全程保持纯黑白极简艺术 UI。

## 全局约束

- Go 版本 >= 1.22（用增强路由 mux），禁止引入 gin/echo 等重型框架。
- 数据库只用 `modernc.org/sqlite`（纯 Go，免 CGO）。
- 所有对外 API 返回统一 JSON 格式：`{"code":0,"data":...,"msg":""}`（code=0 成功，非 0 失败）。
- token/账号属于敏感数据，后端 SQLite 明文存储（本项目本地自用，API 响应不回传密码）。
- 前端构建产物必须嵌入 Go 二进制（go:embed），开发时 Vite 代理 /api 到后端。
- 项目根目录：`E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto`，`backend/` 与 `web/` 两个子目录。
- UI 保持纯黑白极简艺术风格（`#09090b` 底、`neutral-900` 发丝边框、`#fafafa` 白），绝对零圆角。

---

### 任务 1：数据库新增账号表（仅账号名）+ 按账号隔离目标

**文件：**
- Modify: `backend/internal/db/schema.sql`
- Modify: `backend/internal/store/store.go`

**接口：**
- 新增 `SaveAccountName(acct string) error`：往 `accounts` 表插入/更新账号名（幂等，账号名即主键）。
- 新增 `ListAccounts() ([]string, error)`：返回所有账号名（按 `account` 排序）。
- 改造目标存储按账号隔离：`targets` 表增加 `account TEXT NOT NULL DEFAULT ''` 列；新增 `SetTargetsForAccount(acct string, targets []scheduler.Target) error` 与 `LoadTargetsForAccount(acct string) ([]scheduler.Target, error)`；现有 `SetTargets` / `LoadTargets` 改为 `account=''` 过滤（单账号行为不变）。

- [ ] **Step 1：schema.sql 增加 accounts 表与 targets.account 列**

```sql
-- 多账号表：仅存账号名（密码不入库），登录成功即 upsert，account 唯一
CREATE TABLE IF NOT EXISTS accounts (
  account TEXT PRIMARY KEY,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

```sql
-- targets 表迁移：新增 account 列（旧数据自动归到 '' 空账号，行为不变）
ALTER TABLE targets ADD COLUMN account TEXT NOT NULL DEFAULT '';
```

> 注意：新建库时 `ALTER TABLE` 会因列已存在而报错吗？——不会。`ALTER TABLE ... ADD COLUMN` 在列已存在时返回 `duplicate column` 错误。**为兼容老库与新库**，不加裸 ALTER 语句，而是在 Open 时用 `PRAGMA table_info(targets)` 探测列是否存在，不存在才执行 ALTER（见 Step 2）。

- [ ] **Step 2：db 增加最小化迁移逻辑**

在 `backend/internal/db/db.go`（或新建 `migrate.go`）：

```go
// ensureTargetsAccountColumn 老库 targets 无 account 列时补列。
func ensureTargetsAccountColumn(d *sql.DB) error {
	rows, err := d.Query("PRAGMA table_info(targets)")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == "account" {
			return nil // 已有列，无需迁移
		}
	}
	_, err = d.Exec("ALTER TABLE targets ADD COLUMN account TEXT NOT NULL DEFAULT ''")
	return err
}
```

在 `db.Open()` 里 `schemaSQL` 之后调用 `ensureTargetsAccountColumn(d)`。

- [ ] **Step 3：store 新增账号与按账号目标方法**

```go
// SaveAccountName 记录账号名（登录成功调用；幂等）。
func (s *Store) SaveAccountName(acct string) error {
	if acct == "" {
		return nil
	}
	_, err := s.db.Exec("INSERT OR IGNORE INTO accounts (account) VALUES (?)", acct)
	return err
}

// ListAccounts 返回所有账号名。
func (s *Store) ListAccounts() ([]string, error) {
	rows, err := s.db.Query("SELECT account FROM accounts ORDER BY account")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SetTargetsForAccount 按账号保存目标（先删后插）。
func (s *Store) SetTargetsForAccount(acct string, targets []scheduler.Target) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM targets WHERE account = ?", acct); err != nil {
		return err
	}
	for _, t := range targets {
		if _, err := tx.Exec(
			"INSERT INTO targets (account, publish_id, class_id, course_name) VALUES (?, ?, ?, ?)",
			acct, t.PublishID, t.ClassID, t.CourseName); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LoadTargetsForAccount 读取账号目标。
func (s *Store) LoadTargetsForAccount(acct string) ([]scheduler.Target, error) {
	rows, err := s.db.Query("SELECT publish_id, class_id, course_name FROM targets WHERE account = ? ORDER BY id", acct)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []scheduler.Target
	for rows.Next() {
		var t scheduler.Target
		if err := rows.Scan(&t.PublishID, &t.ClassID, &t.CourseName); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
```

**现有 `SetTargets` / `LoadTargets` 改写为 `account = ''` 版本**（保持单账号行为不变）：

```go
func (s *Store) SetTargets(targets []scheduler.Target) error {
	return s.SetTargetsForAccount("", targets)
}

func (s *Store) LoadTargets() ([]scheduler.Target, error) {
	return s.LoadTargetsForAccount("")
}
```

- [ ] **Step 4：写测试验证**

在 `store_test.go` 追加：

```go
func TestTargetsByAccount(t *testing.T) {
	s := openTestStore(t)
	if err := s.SaveAccountName("acct1"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveAccountName("acct2"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveAccountName("acct1"); err != nil { // 幂等
		t.Fatal(err)
	}
	got, err := s.ListAccounts()
	if err != nil || len(got) != 2 || got[0] != "acct1" || got[1] != "acct2" {
		t.Fatalf("账号列表异常: %v %v", got, err)
	}

	// 账号隔离目标
	t1 := []scheduler.Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操"}}
	t2 := []scheduler.Target{{PublishID: 2, ClassID: 61205, CourseName: "篮球"}}
	if err := s.SetTargetsForAccount("acct1", t1); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTargetsForAccount("acct2", t2); err != nil {
		t.Fatal(err)
	}
	l1, _ := s.LoadTargetsForAccount("acct1")
	l2, _ := s.LoadTargetsForAccount("acct2")
	if len(l1) != 1 || l1[0].ClassID != 61115 || len(l2) != 1 || l2[0].ClassID != 61205 {
		t.Fatalf("账号目标隔离失败: %+v %+v", l1, l2)
	}
	// 旧单账号接口为空账号
	if _, err := s.SetTargets(t1); err != nil { // 写空账号
		t.Fatal(err)
	}
	legacy, _ := s.LoadTargets()
	if len(legacy) != 1 || legacy[0].ClassID != 61115 {
		t.Fatalf("旧单账号接口破坏: %+v", legacy)
	}
}
```

- [ ] **Step 5：运行测试确认通过**

Run: `cd backend && go test ./internal/store -run 'TestTargetsByAccount|TestTargetsRoundTrip' -v`
Expected: PASS

- [ ] **Step 6：提交**

```bash
git add backend/internal/db/schema.sql backend/internal/db/db.go backend/internal/store/store.go backend/internal/store/store_test.go
git commit -m "feat(store): per-account targets isolation with accounts table"
```

---

### 任务 2：Scheduler 探测节流改为 30 秒常量 + 账号状态聚合

**文件：**
- Modify: `backend/internal/scheduler/scheduler.go`

**接口：**
- `probeInterval = 30 * time.Second` 常量（当前是 2 秒判断）。
- 新增 `SetTargetsForAccount(acct string, targets []Target)`：为指定账号保存目标并重建该账号状态。
- 新增 `StateForAccount(acct string) SchedulerState`：返回该账号的目标状态与全局窗口标志。
- 新增 `Accounts() []string`：返回所有已知账号名。
- 现有 `SetTargets` / `State` 保持为空账号（`""`）行为，向后兼容。

- [ ] **Step 1：探测节流 30 秒常量**

```go
// probeInterval 课程探测最小间隔：30 秒，避免触发平台"访问过于频繁"熔断。
const probeInterval = 30 * time.Second
```

```go
func (s *Scheduler) tick() {
	// 距上次成功探测不足 30 秒：静默跳过（限速保护）
	if time.Since(s.lastSuccessProbe) < probeInterval {
		return
	}
	// ...其余逻辑不变
}
```

- [ ] **Step 2：Scheduler 结构新增账号目标映射**

```go
type Scheduler struct {
	// ...
	acctTargets map[string][]Target // 按账号隔离的目标课程（key=账号名，""=默认）
}
```

`New` 初始化 `acctTargets: make(map[string][]Target)`。

`SetTargets` 改为转发到空账号：

```go
func (s *Scheduler) SetTargets(targets []Target) { s.SetTargetsForAccount("", targets) }

// SetTargetsForAccount 为指定账号替换目标并重建状态。
func (s *Scheduler) SetTargetsForAccount(acct string, targets []Target) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acctTargets[acct] = targets
	// 重建状态（CourseStatus 带 account 字段，见 Step 3）
}
```

- [ ] **Step 3：CourseStatus 增加 account 字段，状态含账号信息**

```go
type CourseStatus struct {
	Account    string `json:"account"`
	PublishID  int    `json:"publish_id"`
	// ...
}
```

`StateForAccount`:

```go
// StateForAccount 返回指定账号的状态快照（Courses 仅含该账号目标；WindowOpened 全校共享）。
func (s *Scheduler) StateForAccount(acct string) SchedulerState {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	st.Courses = nil
	for _, c := range s.state.Courses {
		if c.Account == acct {
			st.Courses = append(st.Courses, c)
		}
	}
	return st
}
```

`Accounts()`:

```go
// Accounts 返回所有已知账号名（含空账号）。
func (s *Scheduler) Accounts() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.acctTargets))
	for a := range s.acctTargets {
		out = append(out, a)
	}
	return out
}
```

`submitAll` 改为遍历**当前账号目标**（不再提交全校目标）：

```go
func (s *Scheduler) submitAll() {
	s.mu.Lock()
	var targets []Target
	for _, ts := range s.acctTargets {
		targets = append(targets, ts...)
	}
	s.mu.Unlock()
	for _, t := range targets {
		s.submit(t)
	}
}
```

- [ ] **Step 4：运行测试确认通过**

Run: `cd backend && go test ./internal/scheduler -v`
Expected: 现有 `TestStateMachine` / `TestSubmitFailureRetries` / `TestRestoreDoneSkipsResubmit` 全部 PASS（这些测试用 `SetTargets`（空账号），节流 30 秒不影响打开后立即提交路径；等待 `success` 的超时 3s 内，首次探测在 Start 后立即发生，窗口打开后下个 tick 探测到后提交）。

> 注意：`TestStateMachine` 中 `time.Sleep(50 * time.Millisecond)` 断言"窗口未开状态 pending"——tick 首次运行即探测（Start 后 300ms），窗口未开时状态保持 pending，OK；`setOpen(true)` 后最迟 300ms+30s？——**不会等 30s**：`lastSuccessProbe` 只在成功探测后记录，窗口打开前每 30s 才探测一次，但 `setOpen(true)` 后下一次 tick 若距上次成功探测 <30s 会跳过，等待超时 3s 内可能错过。**修复：`tick()` 改为先探测再节流判断**（见 Step 5）。

- [ ] **Step 5：修正节流逻辑（先探测、节流保护的是重复探测）**

```go
func (s *Scheduler) tick() {
	s.mu.Lock()
	// 距上次成功探测不足 30 秒且非首次：静默跳过（限速保护）
	last := s.lastSuccessProbe
	s.mu.Unlock()
	if !last.IsZero() && time.Since(last) < probeInterval {
		return
	}
	data, err := s.client.FindElectives()
	if err != nil {
		// ...（同现有）
	}
	s.mu.Lock()
	s.lastSuccessProbe = time.Now()
	s.mu.Unlock()
	// ...
}
```

> 这样窗口打开瞬间能立即探测（不因上次成功探测被跳过），且此后 30 秒内不重复探测。`TestStateMachine` 的 3s 超时内窗口打开后的首次 tick 必然探测，测试稳定通过。

- [ ] **Step 6：提交**

```bash
git add backend/internal/scheduler/scheduler.go
git commit -m "feat(scheduler): 30s probe throttle with per-account targets"
```

---

### 任务 3：API 层接入账号参数

**文件：**
- Modify: `backend/internal/api/handler.go`
- Modify: `backend/internal/api/router.go`

**接口：**
- `PUT /api/targets` 请求体加 `account` 字段 → `sched.SetTargetsForAccount(acct, targets)` + `st.SetTargetsForAccount`（双写）。
- `GET /api/state` 接受 `?account=` 参数 → `sched.StateForAccount(acct)`。
- 新增 `GET /api/accounts` → `st.ListAccounts()` + `sched.Accounts()` 去重合并。
- `POST /api/login` → 成功后 `st.SaveAccountName(req.Account)`（记录账号名，密码不入库）。
- `GET /api/electives` 不变（课程数据全校共享）。

- [ ] **Step 1：handler 改写**

`Deps` 保持 `Sched *scheduler.Scheduler` 不变。`handleLogin` 末尾追加账号名记录：

```go
token, err := d.Client.Login(req.Account, req.Password)
if err != nil {
	writeJSON(w, 1, nil, "登录失败: "+err.Error())
	return
}
d.Store.SaveAccount(req.Account, req.Password, token) // 兼容旧单账号存储
d.Store.SaveAccountName(req.Account)                   // 多账号列表
d.Client.SetCredentials(req.Account, req.Password, token)
// ...
```

`handleSetTargets`:

```go
type TargetsRequest struct {
	Account string             `json:"account"` // 目标所属账号；空 = 默认账号
	Targets []scheduler.Target `json:"targets"`
}

func (d *Deps) handleSetTargets(w http.ResponseWriter, r *http.Request) {
	var req TargetsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, 1, nil, "请求体解析失败: "+err.Error())
		return
	}
	if req.Targets == nil {
		req.Targets = []scheduler.Target{}
	}
	for _, t := range req.Targets {
		if t.ClassID <= 0 {
			writeJSON(w, 1, nil, "class_id 无效")
			return
		}
	}
	acct := req.Account
	if err := d.Store.SetTargetsForAccount(acct, req.Targets); err != nil {
		writeJSON(w, 1, nil, "保存目标失败: "+err.Error())
		return
	}
	d.Sched.SetTargetsForAccount(acct, req.Targets)
	d.Store.AppendLog(0, "set_targets", fmt.Sprintf("账号 %s：%d 门目标课程", displayAcct(acct), len(req.Targets)), true)
	writeJSON(w, 0, req.Targets, "目标已保存")
}

func displayAcct(acct string) string {
	if acct == "" {
		return "默认"
	}
	return acct
}
```

`handleState`:

```go
func (d *Deps) handleState(w http.ResponseWriter, r *http.Request) {
	acct := r.URL.Query().Get("account")
	st := d.Sched.StateForAccount(acct)
	writeJSON(w, 0, st, "")
}
```

新增 `handleAccounts`:

```go
func (d *Deps) handleAccounts(w http.ResponseWriter, r *http.Request) {
	names, err := d.Store.ListAccounts()
	if err != nil {
		writeJSON(w, 1, nil, "读取账号列表失败: "+err.Error())
		return
	}
	seen := map[string]bool{}
	out := []string{}
	for _, a := range names {
		if a == "" || seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
	}
	for _, a := range d.Sched.Accounts() {
		if a == "" || seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
	}
	writeJSON(w, 0, out, "")
}
```

- [ ] **Step 2：router 注册**

```go
mux.HandleFunc("GET /api/accounts", d.handleAccounts)
```

- [ ] **Step 3：api 测试补充**

在 `handler_test.go`：

- `newTestDeps` 用 `client.SetCredentials("acct", "pwd", "tok")` 后先 `st.SaveAccountName("acct")`，保证 `/api/accounts` 非空。
- 新增 `TestAccounts`：`GET /api/accounts` 返回含 `acct` 的数组。
- 更新 `TestSetTargetsAndState`：`PUT /api/targets` body 加 `account:"acct1"`，然后 `GET /api/state?account=acct1` 应返回该账号课程。
- 运行全部测试通过。

- [ ] **Step 4：提交**

```bash
git add backend/internal/api/handler.go backend/internal/api/router.go backend/internal/api/handler_test.go
git commit -m "feat(api): account-scoped targets/state with accounts listing"
```

---

### 任务 4：main.go 启动时预加载账号目标（重启恢复）

**文件：**
- Modify: `backend/main.go`

- [ ] **Step 1：启动时加载所有账号目标**

`LoadTargets` 现仅读空账号目标；`LoadTargetsForAccount` 逐账号读取。`manager` 无需新包，`main.go` 直接用 `st.ListAccounts()` 恢复各账号目标到 Scheduler：

```go
// 恢复各账号目标（账号名来自 accounts 表 + 目标表）
accts, err := st.ListAccounts()
if err != nil {
	log.Printf("[main] 读取账号列表失败: %v", err)
}
allTargets := []scheduler.Target{}
for _, a := range accts {
	ts, err := st.LoadTargetsForAccount(a)
	if err != nil {
		log.Printf("[main] 读取账号 %s 目标失败: %v", a, err)
		continue
	}
	if len(ts) > 0 {
		sched.SetTargetsForAccount(a, ts)
		allTargets = append(allTargets, ts...)
	}
}
// 旧单账号目标（空账号）同样恢复
if legacy, err := st.LoadTargets(); err == nil && len(legacy) > 0 {
	sched.SetTargetsForAccount("", legacy)
	allTargets = append(allTargets, legacy...)
}
// 兼容旧逻辑：sched.SetTargets(allTargets) 保留（内部即空账号目标）
sched.SetTargets(allTargets)
```

> 注：`SetTargets` 走空账号，重复设置空账号目标无害（同值覆盖）。

- [ ] **Step 2：构建运行验证**

Run: `cd backend && go build ./... && go test ./...`
Expected: 全部 PASS；`go vet ./...` 无告警。

- [ ] **Step 3：提交**

```bash
git add backend/main.go
git commit -m "feat(main): restore per-account targets on boot"
```

---

### 任务 5：前端多账号切换 + 登录自动加账号

**文件：**
- Modify: `web/src/types.ts`
- Modify: `web/src/api/client.ts`
- Modify: `web/src/App.tsx`
- Modify: `web/src/routes/Login.tsx`
- Modify: `web/src/routes/Dashboard.tsx`
- Modify: `web/src/routes/Select.tsx`

**接口：**
- 新增 `GET /api/accounts` 返回 `string[]`（账号名列表）。
- `api` 增加可选 `account` 参数（仅对需要账号的接口追加 query）。
- `App` 状态 `accounts: string[]`、`current: string`；登录成功后刷新列表并切到新账号；账号下拉切换 `current`。
- `Dashboard`/`Select` 的 `/state` 查询带 `account`；`Select` 的 `save` 请求体加 `account`；`/electives` 不带（全校共享）。
- `Login` 登录成功回调携带新账号名。

- [ ] **Step 1：types.ts 增加 Account 类型**

```ts
export type Account = string
```

- [ ] **Step 2：api/client.ts 支持 account 参数**

```ts
export async function api<T>(path: string, opts?: RequestInit & { account?: string }): Promise<T> {
  const { account, ...rest } = opts ?? {}
  const q = account ? `?account=${encodeURIComponent(account)}` : ""
  const r = await fetch(BASE + path + q, { headers: { "Content-Type": "application/json" }, ...rest })
  // ...其余不变
}
```

- [ ] **Step 3：App.tsx 管理账号状态**

- 初始化 `useQuery(["accounts"], () => api<Account[]>("/accounts"))`（失败为空数组）。
- `login(token, account)` 回调：存 token、`setCurrent(account)`、`queryClient.invalidateQueries(["accounts"])` 刷新列表。
- 渲染 `Dashboard`/`Select` 时传入 `account={current}` 与 `accounts={accounts}`。
- 页面切换时 `onGoSelect`/`onDone` 不变。

- [ ] **Step 4：Login 登录成功回调携带 account**

```ts
const data = await api<{ token: string; account: string }>("/login", { method: "POST", body: JSON.stringify({...}) })
onLogin(data.token, data.account)
```

- [ ] **Step 5：Dashboard 与 Select 查询/保存带 account**

`Dashboard.tsx`：
- `Props` 增加 `account: string` 与 `accounts: Account[]`；
- `useQuery` 的 `queryKey: ["state", account]`，`queryFn: () => api<SchedulerState>("/state", { account })`；
- 顶栏加账号下拉（Radix Select 或原生 `<select>`，纯黑白极简样式），onChange 触发 `onSwitchAccount`；
- 倒计时区域文案 `300ms 周期监听` → `30 秒查询节流`；`心跳轮询周期 300 毫秒` → `30 秒`。

`Select.tsx`：
- `Props` 增加 `account: string`；
- `useQuery` 的 `/state` 带 `account`（回显该账号目标）；
- `save` 请求体 `{ account, targets }`；
- 顶栏 `SELECTED` 计数与保存逻辑不变。

- [ ] **Step 6：构建验证**

Run: `cd web && npm run build`
Expected: tsc 通过，产物嵌入 Go 二进制后刷新页面正常。

- [ ] **Step 7：提交**

```bash
git add web/src
git commit -m "feat(ui): multi-account switcher and auto-add on login"
```

---

### 任务 6：全链路验证 + 收尾

**文件：**
- Modify: `web/src/routes/Dashboard.tsx`（"300ms 周期监听" → "30 秒查询节流"、"心跳轮询周期 300 毫秒" → "30 秒"）
- Modify: `backend/internal/scheduler/scheduler.go`（启动日志 `轮询间隔 %v` → `轮询间隔 30s（节流保护）`）

- [ ] **Step 1：替换高频轮询文案**

- Dashboard：`300ms 周期监听` → `30 秒查询节流`；`心跳轮询周期 300 毫秒` → `30 秒`。
- Scheduler `Start()` 日志改为 `"[scheduler] 已启动，轮询间隔 %v（探测节流 30 秒）"`。

- [ ] **Step 2：全量测试**

Run: `cd backend && go test ./... -race` 且 `cd web && npm run build`
Expected: 全部 PASS，构建成功。

- [ ] **Step 3：重建并实测**

```bash
cd backend && go build -o xuanke.exe . && Copy-Item xuanke.exe ..\xuanke.exe -Force
```

启动 `xuanke.exe`，浏览器打开 `http://localhost:3091`：
- 前端正常加载，账号下拉列出已登录账号；
- `/api/accounts` 返回已保存账号列表；
- 登录新账号 → 列表即时新增该账号并自动切换；
- `/api/state?account=<acct>` 返回该账号目标状态，日志不再 300ms 刷屏；
- 调度器日志显示 `轮询间隔 30s`。

- [ ] **Step 4：更新 CLAUDE.md**

在根目录 CLAUDE.md 补记：多账号目标隔离（targets.account 列 + accounts 账号名表）、30 秒探测节流决策（`probeInterval` 常量 + 先探测后节流）、`/api/accounts` 与 `?account=` 参数约定。

- [ ] **Step 5：提交**

```bash
git add -A
git commit -m "feat: multi-account targets with 30s probe throttle, clean UI copy"
```
