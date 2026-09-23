# 激活码机制 + 多备选退避 + 日志账号隔离 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan ta***REMOVED***-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 去掉部署访问口令登录 gate，改为激活码机制；选课目标支持每发布多备选（满员依次退避）；日志按账号隔离；修复发布名称显示与保存后自动退出。

**Architecture:** 激活码/激活状态落在 SQLite 新表（activation_codes / activations），管理口令（XUANKE_ADMIN_TOKEN）仅用于生成激活码的管理面板，登录不再要求口令。targets 表加 priority 列承载备选顺序；调度器按「账号×发布」链式提交，仅满员错误（zhidao.ErrFull）切换下一备选。task_log 加 account 列按会话账号过滤。

**Tech Stack:** Go net/http + modernc.org/sqlite + React 19 + Vite + Tailwind

**Spec:** 用户需求（2026-09-12）：发布名修复 / 保存后退出 / 多备选退避 / 日志隔离 / 激活码机制

## Global Constraints

- 纯黑白极简 UI，无彩色 emoji，注释与沟通用简体中文
- 不兼容旧数据：schema 变化后旧库需删除重建（db.Open 对缺列旧表拒绝启动）
- 各账号物理隔离（每账号独立 zhidao.Client + 会话绑定账号）
- 每个小步骤后立即 commit

---

### Task 1: 发布名称修复 + 保存后自动退出（前端）

**Files:**
- Modify: `web/src/routes/Select.tsx`
- Test: `cd web && npm run build`

**Interfaces:**
- Consumes: `api<T>` (web/src/api/client.ts，session 认证)
- Produces: Select 页 onDone 回调在保存成功后触发

- [ ] **Step 1: 发布名直接显示完整 publish_name**

在 `web/src/routes/Select.tsx` 的 tabs useMemo 中：

```ts
const tabs = useMemo(
  () =>
    publishes.map((p) => ({
      ...p,
      label: p.publish_name || `发布 #${p.publish_id}`,
      open: p.in_date_range,
      tip: `可选 ${p.can_select} 门 · 已选 ${p.has_selected} 门 · 共 ${p.total_count} 门班次`,
    })),
  [publishes]
)
```

删除正则截断逻辑 `(p.publish_name.match(/高二年(.+)$/) || [])[1] || ...`。

- [ ] **Step 2: 保存成功后自动返回控制台**

在 `save()` 成功分支末尾加 `onDone()`：

```ts
setSaved(true)
toast({
  title: targets.length > 0 ? "目标保存成功" : "目标已清空",
  description: targets.length > 0 ? `已锁定 ${targets.length} 门预选课程` : "已清空所有预选目标",
  variant: "default",
})
onDone() // 保存后直接返回控制台
```

- [ ] **Step 3: 构建验证**

Run: `cd web && npm run build`
Expected: tsc 通过、产物更新

- [ ] **Step 4: Commit**

```bash
git add web/src/routes/Select.tsx
git commit -m "fix(ui): show full publish name and return to console after save"
```

---

### Task 2: 数据库 schema v3（priority / account / 激活码表）

**Files:**
- Modify: `backend/internal/db/schema.sql`
- Modify: `backend/internal/db/db.go`
- Test: `backend/internal/db/db_test.go`

**Interfaces:**
- Produces: targets.priority 列、task_log.account 列、activation_codes/activations 表

- [ ] **Step 1: schema.sql 增加列与新表**

```sql
-- 目标课程（按账号隔离 + 备选优先级，priority 越小越先提交）
CREATE TABLE IF NOT EXISTS targets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account TEXT NOT NULL,
  publish_id INTEGER NOT NULL,
  class_id INTEGER NOT NULL,
  course_name TEXT NOT NULL,
  priority INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 报名日志（按账号隔离）
CREATE TABLE IF NOT EXISTS task_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account TEXT NOT NULL DEFAULT '',
  class_id INTEGER NOT NULL,
  action TEXT NOT NULL,
  result TEXT,
  is_ok INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 激活码（可设总可用次数）
CREATE TABLE IF NOT EXISTS activation_codes (
  code TEXT PRIMARY KEY,
  total_uses INTEGER NOT NULL,
  used_uses INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 已激活账号（激活一次永久有效）
CREATE TABLE IF NOT EXISTS activations (
  account TEXT PRIMARY KEY,
  activated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

- [ ] **Step 2: db.go 拒绝缺列的旧 v2 库**

在 `refuseLegacy` 中增加两个 pragma 检查（targets.priority、task_log.account 缺失 → 拒绝）：

```go
// columnExists 检查表是否存在指定列。
func columnExists(d *sql.DB, table, column string) (bool, error) {
	rows, err := d.Query(fmt.Sprintf("SELECT 1 FROM pragma_table_info('%s') WHERE name='%s'", table, column))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	return rows.Next(), nil
}

// 旧 v2 库缺列（不兼容，提示删除重建）
for _, col := range [][2]string{{"targets", "priority"}, {"task_log", "account"}} {
	ok, err := columnExists(d, col[0], col[1])
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("检测到旧版数据库（%s 表缺 %s 列），本版本不兼容旧数据。请删除 data/xuanke.db 后重新启动", col[0], col[1])
	}
}
```

注意 `refuseLegacy` 中 account 表检查保持；`fmt` 需要 import。

- [ ] **Step 3: db_test 增加新表存在断言**

在 `TestOpenAndSchema` 后追加：`d.QueryRow("SELECT count(*) FROM activation_codes").Scan(&n)` 不应报错（表存在即可）。

- [ ] **Step 4: 测试**

Run: `cd backend && go test ./internal/db/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/db/
git commit -m "feat(db): schema v3 with target priority, log account, activation codes"
```

---

### Task 3: store 层（priority 读写 / 日志账号隔离 / 激活码存储）

**Files:**
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/store_test.go`

**Interfaces:**
- Produces（供 scheduler/api 使用）:
  - `SetTargetsForAccount(acct string, targets []scheduler.Target) error`（带 Priority 排序写入）
  - `LoadTargetsForAccount(acct string) ([]scheduler.Target, error)`（返回含 Priority）
  - `AppendLog(acct string, classID int, action, result string, isOK bool) error`
  - `LoadLogs(acct string, limit int) ([]LogEntry, error)`
  - `CreateActivationCode(code string, totalUses int) error`
  - `IsActivated(acct string) (bool, error)`
  - `ConsumeActivationCode(code, acct string) (bool, error)`
  - `ListActivationCodes() ([]ActivationCode, error)`
  - `DeleteActivationCode(code string) error`

- [ ] **Step 1: Target priority 持久化**

```go
// SetTargetsForAccount 按账号保存目标课程（先删后插；按 priority 排序保证备选顺序）。
func (s *Store) SetTargetsForAccount(acct string, targets []scheduler.Target) error {
	sort.SliceStable(targets, func(i, j int) bool { return targets[i].Priority < targets[j].Priority })
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
			"INSERT INTO targets (account, publish_id, class_id, course_name, priority) VALUES (?, ?, ?, ?, ?)",
			acct, t.PublishID, t.ClassID, t.CourseName, t.Priority); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LoadTargetsForAccount 读取指定账号的目标课程（按 priority 排序）。
func (s *Store) LoadTargetsForAccount(acct string) ([]scheduler.Target, error) {
	rows, err := s.db.Query("SELECT publish_id, class_id, course_name, priority FROM targets WHERE account = ? ORDER BY priority", acct)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []scheduler.Target
	for rows.Next() {
		var t scheduler.Target
		if err := rows.Scan(&t.PublishID, &t.ClassID, &t.CourseName, &t.Priority); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
```

需要 `sort` import。`scheduler.Target` 加 `Priority int \`json:"priority"\``（Task 4 定义，先保证编译通过需同步修改 scheduler.Target——见 Task 4 说明，两处一起改）。

- [ ] **Step 2: 日志按账号过滤**

```go
// AppendLog 追加报名日志（account 标识来源账号，日志按账号隔离）。
func (s *Store) AppendLog(acct string, classID int, action, result string, isOK bool) error {
	ok := 0
	if isOK {
		ok = 1
	}
	_, err := s.db.Exec("INSERT INTO task_log (account, class_id, action, result, is_ok) VALUES (?, ?, ?, ?, ?)",
		acct, classID, action, result, ok)
	return err
}

// LoadLogs 读取指定账号最近 limit 条日志。
func (s *Store) LoadLogs(acct string, limit int) ([]LogEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(
		"SELECT id, class_id, action, result, is_ok, created_at FROM task_log WHERE account = ? ORDER BY id DESC LIMIT ?", acct, limit)
	...
}
```

- [ ] **Step 3: 激活码存储方法**

```go
// ActivationCode 激活码记录。
type ActivationCode struct {
	Code      string `json:"code"`
	TotalUses int    `json:"total_uses"`
	UsedUses  int    `json:"used_uses"`
	CreatedAt string `json:"created_at"`
}

// CreateActivationCode 新建激活码。
func (s *Store) CreateActivationCode(code string, totalUses int) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO activation_codes (code, total_uses) VALUES (?, ?)", code, totalUses)
	return err
}

// IsActivated 查询账号是否已激活。
func (s *Store) IsActivated(acct string) (bool, error) {
	var n int
	err := s.db.QueryRow("SELECT count(*) FROM activations WHERE account = ?", acct).Scan(&n)
	return n > 0, err
}

// ConsumeActivationCode 激活账号：事务内扣减激活码次数 + 记录激活。返回是否成功。
func (s *Store) ConsumeActivationCode(code, acct string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var total, used int
	err = tx.QueryRow("SELECT total_uses, used_uses FROM activation_codes WHERE code = ?", code).Scan(&total, &used)
	if err == sql.ErrNoRows {
		return false, nil // 激活码不存在
	}
	if err != nil {
		return false, err
	}
	if used >= total {
		return false, nil // 次数用尽
	}
	if _, err := tx.Exec("UPDATE activation_codes SET used_uses = used_uses + 1 WHERE code = ?", code); err != nil {
		return false, err
	}
	if _, err := tx.Exec("INSERT OR IGNORE INTO activations (account) VALUES (?)", acct); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// ListActivationCodes 列出全部激活码。
func (s *Store) ListActivationCodes() ([]ActivationCode, error) {
	rows, err := s.db.Query("SELECT code, total_uses, used_uses, created_at FROM activation_codes ORDER BY created_at DESC")
	...
}

// DeleteActivationCode 删除激活码。
func (s *Store) DeleteActivationCode(code string) error {
	_, err := s.db.Exec("DELETE FROM activation_codes WHERE code = ?", code)
	return err
}
```

注意：`ConsumeActivationCode` 中激活与扣次数需在同一事务保证原子性；激活码不存在或次数用尽返回 (false, nil)。

- [ ] **Step 4: store_test 更新与新测试**

更新 TestLogs（传账号）、TestTargetsByAccount（断言 priority）。新增 TestActivationCodes（创建/消耗/次数用尽/列表/删除）、TestLogsByAccount（账号隔离）。

- [ ] **Step 5: 测试**

Run: `cd backend && go build ./... && go test ./internal/store/`
Expected: PASS（scheduler.Target 已加 Priority 后）

- [ ] **Step 6: Commit**

```bash
git add backend/internal/store/
git commit -m "feat(store): target priority, per-account logs, activation codes"
```

---

### Task 4: scheduler 多备选退避（人数对比判定满员）+ zhidao 实时人数

**Files:**
- Modify: `backend/internal/zhidao/client.go`
- Modify: `backend/internal/scheduler/scheduler.go`
- Modify: `backend/internal/scheduler/scheduler_test.go`

**Interfaces:**
- Consumes: `store.AppendLog(acct, ...)`、`store.SaveSuccess(acct, id)`、`scheduler.Target.Priority`
- Produces: 按人数确认满员 → 依次退避下一备选；`StudentCounts` 实时复核

**满员判定（用户指定：不解析平台"满"字错误，直接对比人数）：**
- 提交前：课程快照中 `selected_count >= max_count`（且 max_count > 0）→ 确认满员
- 提交失败时：调用 `StudentCounts([classID])` 实时复核，`selectedCount >= maxCount` 才切下一备选
- 已确认满员的课程记入 `full map[string]map[int]bool`（每 tick 不再重复处理/刷日志）；下次快照刷新显示该课已不满（有人退课）时解除标记、状态回 pending 重新尝试

- [ ] **Step 1: zhidao.Client 暴露满员判定辅助**

StudentCounts 已有实现。新增 `IsClassFull(classID int) (bool, error)` 便捷封装：

```go
// IsClassFull 实时查询该课程是否已满（selectedCount >= maxCount）。
func (c *Client) IsClassFull(classID int) (bool, error) {
	counts, err := c.StudentCounts([]int{classID})
	if err != nil {
		return false, err
	}
	for _, c := range counts {
		if c.ID == classID {
			return c.MaxCount > 0 && c.SelectedCount >= c.MaxCount, nil
		}
	}
	return false, fmt.Errorf("课程 %d 无人数数据", classID)
}
```

`CountEntry` 增加 `MaxCount int \`json:"maxCount"\`` 字段（findElectivesStudentCount 返回含 maxCount）。

- [ ] **Step 2: scheduler 人数退避链式提交**

`Target` 加 `Priority int \`json:"priority"\``。Scheduler 增加字段：

```go
full map[string]map[int]bool // [账号][classID] 已确认满员（每 tick 去重，快照刷新后解除）
```

核心链式提交逻辑（替换旧 `submit`）：

```go
// submitAll 并发提交所有账号所有发布的目标链（每链独立 goroutine，链内按人数确认满员依次退避）。
func (s *Scheduler) submitAll() {
	s.mu.Lock()
	type chain struct {
		acct string
		ts   []Target
	}
	var chains []chain
	for acct, ts := range s.acctTargets {
		byPub := map[int][]Target{}
		for _, t := range ts {
			byPub[t.PublishID] = append(byPub[t.PublishID], t)
		}
		for _, list := range byPub {
			sort.SliceStable(list, func(i, j int) bool { return list[i].Priority < list[j].Priority })
			chains = append(chains, chain{acct, list})
		}
	}
	s.mu.Unlock()
	for _, c := range chains {
		s.spawnChain(c.acct, c.ts)
	}
}

// spawnChain 逐备选提交：确认满员（快照或实时人数）才切下一备选。
func (s *Scheduler) spawnChain(acct string, ts []Target) {
	key := acct + "\x00" + strconv.Itoa(ts[0].PublishID)
	s.chainMu.Lock()
	if s.chains[key] {
		s.chainMu.Unlock()
		return
	}
	s.chains[key] = true
	s.chainMu.Unlock()
	go func() {
		defer func() {
			s.chainMu.Lock()
			delete(s.chains, key)
			s.chainMu.Unlock()
		}()
		client, ok := s.clients.ClientFor(acct)
		for _, t := range ts {
			s.mu.Lock()
			// 已成功 / 已确认满员去重（快照显示不满时解除）
			if s.doneHas(acct, t.ClassID) {
				s.mu.Unlock()
				return
			}
			s.releaseFullIfFreedLocked(acct, t.ClassID)
			if s.fullHas(acct, t.ClassID) {
				s.mu.Unlock()
				continue
			}
			// 快照人数确认满员 → 记入 full，跳过
			if s.classFullInSnapshot(t.ClassID) {
				s.markFullLocked(acct, t)
				s.mu.Unlock()
				continue
			}
			if s.inflight[acct] == nil {
				s.inflight[acct] = map[int]bool{}
			}
			s.inflight[acct][t.ClassID] = true
			idx := s.statusIndexLocked(acct, t.ClassID)
			if idx >= 0 {
				s.state.Courses[idx].Status = "submitted"
			}
			s.mu.Unlock()

			var msg string
			var err error
			if !ok {
				err = errors.New("账号会话未建立，等待重新登录")
			} else {
				msg, err = client.SelectClass(t.ClassID)
			}

			s.mu.Lock()
			delete(s.inflight[acct], t.ClassID)
			if err == nil {
				if s.done[acct] == nil {
					s.done[acct] = map[int]bool{}
				}
				s.done[acct][t.ClassID] = true
				s.setStateLocked(s.statusIndexLocked(acct, t.ClassID), "success", msg)
				if s.store != nil {
					s.store.AppendLog(acct, t.ClassID, "select", msg, true)
					_ = s.store.SaveSuccess(acct, t.ClassID)
				}
				log.Printf("[scheduler] 账号 %s 课程 %d（%s）报名成功: %s", acct, t.ClassID, t.CourseName, msg)
				s.mu.Unlock()
				return
			}
			// 提交失败：实时人数复核，确实满员才切下一备选
			full, cErr := s.classFullRealtime(acct, t.ClassID)
			if cErr == nil && full {
				s.markFullLocked(acct, t)
				s.mu.Unlock()
				continue
			}
			// 未确认满员（网络错误/复核失败）：保留失败状态，终止本链，下个 tick 重试
			s.setStateLocked(s.statusIndexLocked(acct, t.ClassID), "failed", err.Error())
			if s.store != nil {
				s.store.AppendLog(acct, t.ClassID, "select", "账号 "+acct+": "+err.Error(), false)
			}
			s.mu.Unlock()
			return
		}
	}()
}

// classFullInSnapshot 快照人数确认满员（需持锁）。
func (s *Scheduler) classFullInSnapshot(classID int) bool {
	if s.lastData == nil {
		return false
	}
	for _, p := range s.lastData.Publishes {
		for _, c := range p.Classes {
			if c.ID == classID {
				return c.MaxCount > 0 && c.SelectedCount >= c.MaxCount
			}
		}
	}
	return false
}

// classFullRealtime 实时人数复核（锁外调用，持锁时不能调网络）。
func (s *Scheduler) classFullRealtime(acct string, classID int) (bool, error) {
	client, ok := s.clients.ClientFor(acct)
	if !ok {
		return false, errors.New("无客户端")
	}
	return client.IsClassFull(classID)
}

// markFullLocked 确认满员：记入 full 集合 + 状态置 failed + 记日志（每课只记一次）。
func (s *Scheduler) markFullLocked(acct string, t Target) {
	if s.full[acct] == nil {
		s.full[acct] = map[int]bool{}
	}
	if s.full[acct][t.ClassID] {
		return
	}
	s.full[acct][t.ClassID] = true
	s.setStateLocked(s.statusIndexLocked(acct, t.ClassID), "failed", "该课程已满员，退避至下一备选")
	if s.store != nil {
		s.store.AppendLog(acct, t.ClassID, "select", "账号 "+acct+": 课程 "+t.CourseName+" 已满员，切换备选", false)
	}
}

// releaseFullIfFreedLocked 快照显示该课已不满员时解除 full 标记并回 pending（需持锁）。
func (s *Scheduler) releaseFullIfFreedLocked(acct string, classID int) {
	if !s.fullHas(acct, classID) {
		return
	}
	if s.classFullInSnapshot(classID) {
		return
	}
	// 快照刚刷新且显示不满 → 解除
	if time.Since(s.lastDataAt) > snapshotTTL {
		return // 快照已过期，等下一次有效快照再判断
	}
	delete(s.full[acct], classID)
	idx := s.statusIndexLocked(acct, classID)
	if idx >= 0 {
		s.state.Courses[idx].Status = "pending"
		s.state.Courses[idx].Result = ""
	}
}
```

`fullHas` 辅助函数同 doneHas 模式。`CourseStatus` 加 `Priority int \`json:"priority"\``；`SetTargetsForAccount` 状态构建填 Priority。scheduler.Client 接口加 `IsClassFull(classID int) (bool, error)`，fakeClient 实现它。

需要 import：`sort`、`strconv`。

- [ ] **Step 3: scheduler_test 满员退避测试**

fakeClient 增加 `counts map[int]zhidao.CountEntry` 与 `IsClassFull` 实现；`newFakeClient` 默认人数不满（SelectedCount=0, MaxCount=36）。更新 `targets()` 加 Priority。新增：

```go
func TestBackupFallbackOnFull(t *testing.T) {
	fc := newFakeClient(false)
	// 第一备选快照显示已满：健美操 36/36
	fc.data.Publishes[0].Classes = []zhidao.Class{
		{ID: 61115, CourseName: "健美操", SelectedCount: 36, MaxCount: 36},
		{ID: 61205, CourseName: "篮球", SelectedCount: 0, MaxCount: 36},
	}
	s := New(&fakeAccts{fc}, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{
		{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0},
		{PublishID: 1, ClassID: 61205, CourseName: "篮球", Priority: 1},
	})
	s.Start()
	defer s.Stop()

	setAllOpened(fc)
	s.resetProbe()
	waitStatusAcct(t, s, "acct1", 61115, "failed", 3*time.Second)
	waitStatusAcct(t, s, "acct1", 61205, "success", 3*time.Second)
}

func TestBackupNotAdvancedOnNetworkError(t *testing.T) {
	fc := newFakeClient(false)
	fc.selectErr[61115] = errors.New("connection reset") // 非满员错误
	fc.data.Publishes[0].Classes = []zhidao.Class{
		{ID: 61115, CourseName: "健美操", SelectedCount: 0, MaxCount: 36},
		{ID: 61205, CourseName: "篮球", SelectedCount: 0, MaxCount: 36},
	}
	s := New(&fakeAccts{fc}, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{
		{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0},
		{PublishID: 1, ClassID: 61205, CourseName: "篮球", Priority: 1},
	})
	s.Start()
	defer s.Stop()

	setAllOpened(fc)
	s.resetProbe()
	waitStatusAcct(t, s, "acct1", 61115, "failed", 3*time.Second)
	// 备选不应被提交（未确认满员）
	time.Sleep(100 * time.Millisecond)
	fc.mu.Lock()
	calls := fc.selectCalls[61205]
	fc.mu.Unlock()
	if calls != 0 {
		t.Fatalf("未确认满员时不应切备选，备选被提交 %d 次", calls)
	}
}
```

`setAllOpened` 需能作用于含 Classes 的数据（现有实现改 InDateRange 不影响）。fakeStore.AppendLog 签名改为 `AppendLog(acct string, classID int, action, result string, isOK bool)`。

- [ ] **Step 4: 测试**

Run: `cd backend && go test ./internal/scheduler/ -run 'Backup|State|Isolation|Parallel|Restore|Snapshot|OpenTime' -race`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/zhidao/ backend/internal/scheduler/
git commit -m "feat(scheduler): backup fallback by seat-count comparison"
```

---

### Task 5: API 层（激活码流程 / 日志隔离 / 移除登录口令 gate）

**Files:**
- Modify: `backend/internal/api/handler.go`
- Modify: `backend/internal/api/router.go`
- Modify: `backend/internal/api/handler_test.go`
- Modify: `backend/main.go`

**Interfaces:**
- Consumes: store 激活码方法、session.Store、accounts.Manager
- Produces:
  - `POST /api/login {account,password}` → 激活则签发会话 `{token,account}`；未激活返回 code=1001
  - `POST /api/activate {account,code}` → 成功签发会话
  - `GET/POST/DELETE /api/admin/codes`（管理口令 X-Admin-Token）
  - `GET /api/logs` 按会话账号过滤

- [ ] **Step 1: handler 登录流程改造**

```go
// LoginRequest 登录请求体（教务账密，无部署口令）。
type LoginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

// handleLogin 登录：教务登录 -> 检查激活 -> 已激活签发会话，未激活提示激活。
func (d *Deps) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, 1, nil, "请求体解析失败: "+err.Error())
		return
	}
	if req.Account == "" || req.Password == "" {
		writeJSON(w, 1, nil, "账号与密码不能为空")
		return
	}
	if _, err := d.Accounts.LoginByPassword(req.Account, req.Password, d.Encrypt); err != nil {
		writeJSON(w, 1, nil, "登录失败: "+err.Error())
		return
	}
	activated, err := d.Store.IsActivated(req.Account)
	if err != nil {
		writeJSON(w, 1, nil, "查询激活状态失败: "+err.Error())
		return
	}
	if !activated {
		// 未激活：前端据此弹出激活码模态框
		writeJSON(w, 1001, map[string]string{"account": req.Account}, "该账号尚未激活，请输入激活码")
		return
	}
	d.issueSession(w, req.Account)
}

// issueSession 记录账号名 + 签发会话 + 记日志。
func (d *Deps) issueSession(w http.ResponseWriter, acct string) {
	if err := d.Store.SaveAccountName(acct); err != nil {
		log.Printf("[api] 保存账号名失败: %v", err)
	}
	sess := d.Sessions.Create(acct)
	d.Store.AppendLog(acct, 0, "login", "账号 "+acct+" 登录成功", true)
	writeJSON(w, 0, map[string]string{"token": sess, "account": acct}, "登录成功")
}

// ActivateRequest 激活请求体。
type ActivateRequest struct {
	Account string `json:"account"`
	Code    string `json:"code"`
}

// handleActivate 激活账号：消耗激活码并签发会话。
func (d *Deps) handleActivate(w http.ResponseWriter, r *http.Request) {
	var req ActivateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, 1, nil, "请求体解析失败: "+err.Error())
		return
	}
	if req.Account == "" || req.Code == "" {
		writeJSON(w, 1, nil, "账号与激活码不能为空")
		return
	}
	ok, err := d.Store.ConsumeActivationCode(strings.TrimSpace(req.Code), req.Account)
	if err != nil {
		writeJSON(w, 1, nil, "激活失败: "+err.Error())
		return
	}
	if !ok {
		writeJSON(w, 1, nil, "激活码无效或已用尽")
		return
	}
	d.issueSession(w, req.Account)
}
```

需要 `strings` import（或直接 TrimSpace 前导尾随）。

- [ ] **Step 2: 管理接口（生成/列出/删除激活码）**

```go
// requireAdmin 管理口令校验（生成激活码专用，非登录 gate）。
func requireAdmin(d *Deps, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := r.Header.Get("X-Admin-Token")
		if d.AdminToken == "" || subtle.ConstantTimeCompare([]byte(tok), []byte(d.AdminToken)) != 1 {
			writeJSON(w, 403, nil, "管理口令错误")
			return
		}
		next(w, r)
	}
}

// handleAdminCodes 激活码管理：POST 生成 / GET 列表 / DELETE 删除。
func (d *Deps) handleAdminCodes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		codes, err := d.Store.ListActivationCodes()
		if err != nil {
			writeJSON(w, 1, nil, "读取激活码失败: "+err.Error())
			return
		}
		writeJSON(w, 0, codes, "")
	case http.MethodPost:
		var req struct {
			Count   int `json:"count"`
			Uses    int `json:"uses"` // 每个激活码可用次数
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeJSON(w, 1, nil, "请求体解析失败: "+err.Error())
			return
		}
		if req.Count < 1 || req.Count > 100 {
			writeJSON(w, 1, nil, "生成数量需在 1-100 之间")
			return
		}
		if req.Uses < 1 {
			writeJSON(w, 1, nil, "每个激活码使用次数至少为 1")
			return
		}
		codes := make([]string, 0, req.Count)
		for i := 0; i < req.Count; i++ {
			code := newActivationCode()
			if err := d.Store.CreateActivationCode(code, req.Uses); err != nil {
				writeJSON(w, 1, nil, "生成激活码失败: "+err.Error())
				return
			}
			codes = append(codes, code)
		}
		writeJSON(w, 0, codes, "生成成功")
	case http.MethodDelete:
		var req struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeJSON(w, 1, nil, "请求体解析失败: "+err.Error())
			return
		}
		if err := d.Store.DeleteActivationCode(strings.TrimSpace(req.Code)); err != nil {
			writeJSON(w, 1, nil, "删除失败: "+err.Error())
			return
		}
		writeJSON(w, 0, nil, "已删除")
	default:
		writeJSON(w, 405, nil, "方法不允许")
	}
}

// newActivationCode 生成 XK-XXXX-XXXX-XXXX 格式激活码。
func newActivationCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("XK-%X-%X-%X", b[0:2], b[2:4], b[4:6]) // 6 字节 → 3 段
}
```

`newActivationCode` 实际使用 6 字节生成 3 段；调整实现：

```go
func newActivationCode() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	s := strings.ToUpper(hex.EncodeToString(b))
	return fmt.Sprintf("XK-%s-%s-%s", s[0:4], s[4:8], s[8:12])
}
```

需要 `crypto/rand`、`encoding/hex` import。`AdminToken` 字段保留在 Deps。

- [ ] **Step 3: 日志按会话账号过滤**

```go
// handleLogs 报名日志（仅返回当前会话账号自己的日志）。
func (d *Deps) handleLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := d.Store.LoadLogs(sessionAccount(r), 100)
	if err != nil {
		writeJSON(w, 1, nil, "读取日志失败: "+err.Error())
		return
	}
	writeJSON(w, 0, logs, "")
}
```

- [ ] **Step 4: router 注册新路由 + 移除登录口令**

```go
mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
	if !limiter.allow(clientIP(r)) {
		writeJSON(w, 429, nil, "登录尝试过于频繁，请稍后再试")
		return
	}
	d.handleLogin(w, r)
})
// 激活接口（未认证，因登录后未激活才需要激活）
mux.HandleFunc("POST /api/activate", func(w http.ResponseWriter, r *http.Request) {
	if !limiter.allow(clientIP(r)) {
		writeJSON(w, 429, nil, "激活尝试过于频繁，请稍后再试")
		return
	}
	d.handleActivate(w, r)
})
// 管理接口：生成/列出/删除激活码（管理口令）
mux.HandleFunc("GET /api/admin/codes", func(w http.ResponseWriter, r *http.Request) {
	requireAdmin(d, d.handleAdminCodes)(w, r)
})
mux.HandleFunc("POST /api/admin/codes", func(w http.ResponseWriter, r *http.Request) {
	requireAdmin(d, d.handleAdminCodes)(w, r)
})
mux.HandleFunc("DELETE /api/admin/codes", func(w http.ResponseWriter, r *http.Request) {
	requireAdmin(d, d.handleAdminCodes)(w, r)
})
```

删除 LoginRequest 的 AdminToken 字段相关逻辑。

- [ ] **Step 5: handler_test 更新与新增**

更新 `loginAndGetToken`（去掉 admin_token）——但登录前需先激活账号。新增 helper：

```go
// activateAccount 生成激活码并激活账号，返回会话令牌。
func activateAccount(t *testing.T, d *testDeps, acct string) string {
	t.Helper()
	// 生成激活码
	code := "XK-ABCD-EF12-3456"
	if err := d.store.CreateActivationCode(code, 5); err != nil {
		t.Fatal(err)
	}
	// 登录（未激活 → 1001）
	code1, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"`+acct+`","password":"pwd"}`)
	if code1 != 200 || j["code"].(float64) != 1001 {
		t.Fatalf("未激活账号应返回 1001: %d %v", code1, j)
	}
	// 激活
	code2, j2 := doJSON(t, d.api, "POST", "/api/activate", `{"account":"`+acct+`","code":"`+code+`"}`)
	if code2 != 200 || j2["code"].(float64) != 0 {
		t.Fatalf("激活失败: %d %v", code2, j2)
	}
	return j2["data"].(map[string]any)["token"].(string)
}
```

其余测试中所有 `loginAndGetToken` 调用改为 `activateAccount`。新增：
- TestLoginUnactivatedNeedsCode：登录返回 1001
- TestActivateIssuesSession：激活后签发会话
- TestActivateBadCode：无效激活码失败
- TestAdminCodes：管理口令生成/列表；错误口令 403
- TestLogsByAccount：acct1 会话看不到 acct2 的日志（AppendLog 各账号写入）

`newTestDeps` 中 `Register` 的 adminToken 参数保留（现为管理口令）。TestLoginBadAdmin 改为 TestAdminBadToken（管理接口错误口令 403）。TestLoginRateLimit 中 body 去掉 admin_token。

- [ ] **Step 6: main.go 调整注释**

AdminToken 现在语义为「管理口令（生成激活码）」。`api.Register` 签名不变。启动要求保留：

```go
if cfg.AdminToken == "" {
	log.Fatal("未设置管理口令：请设置环境变量 XUANKE_ADMIN_TOKEN 后启动（用于生成激活码）")
}
```

- [ ] **Step 7: 测试**

Run: `cd backend && go build ./... && go test ./...`
Expected: 全 PASS

- [ ] **Step 8: Commit**

```bash
git add backend/internal/api/ backend/main.go
git commit -m "feat(api): activation code auth replacing login admin gate, per-account logs"
```

---

### Task 6: 前端激活码机制（登录流程 / 激活码模态框 / 管理面板）

**Files:**
- Modify: `web/src/types.ts`
- Modify: `web/src/routes/Login.tsx`
- Modify: `web/src/App.tsx`
- Test: `cd web && npm run build`

**Interfaces:**
- Consumes: `api<T>`；`ApiError.code`（1001 = 未激活）
- Produces: 登录 → 未激活弹模态框 → 激活 → onLogin(token, account)

- [ ] **Step 1: types.ts 增加激活码类型**

```ts
export interface ActivationCode {
  code: string
  total_uses: number
  used_uses: number
  created_at: string
}
```

- [ ] **Step 2: Login.tsx 重写**

移除部署口令输入框与 adminToken state。登录请求 `{account, password}`。捕获 `ApiError` 且 `code === 1001` 时：记录待激活账号，弹出激活码模态框（复用现有 Dialog/Sheet 组件），输入激活码 → `POST /api/activate {account, code}` → 成功回调 `onLogin(token, account)`。

底部新增「管理」入口：点击展开输入管理口令（存 sessionStorage 或组件 state），校验通过后显示激活码面板——生成数量/每码次数 + 生成按钮、激活码列表（复制/删除）。管理接口请求头带 `X-Admin-Token`（api client 增加一个 headers 参数）。

api client 当前 `api<T>(path, opts)` 支持任意 RequestInit，`headers` 会与默认 content-type 合并：

```ts
const r = await fetch(BASE + path, {
  headers: {
    "Content-Type": "application/json",
    ...(rest.headers || {}),
  },
  ...rest,
})
```

目前实现是 `const { session, ...rest } = opts ?? {}` 然后 `const headers = { "Content-Type": ..., ...(rest.headers || {}) }`——需确认 client.ts 是否已展开 rest.headers。当前代码：

```ts
export async function api<T>(path: string, opts?: RequestInit & { session?: string }): Promise<T> {
  const { session, ...rest } = opts ?? {}
  const headers: Record<string, string> = { "Content-Type": "application/json" }
  if (session) headers.Authorization = `Bearer ${session}`
  const r = await fetch(BASE + path, { headers, ...rest })
```

`{ headers, ...rest }` 中 rest 若含 headers 会覆盖——修改为显式合并：

```ts
const r = await fetch(BASE + path, { ...rest, headers: { "Content-Type": "application/json", ...(session ? { Authorization: `Bearer ${session}` } : {}), ...(rest.headers as Record<string, string> | undefined) } })
```

- [ ] **Step 3: App.tsx 无改动**

Login 的 onLogin 回调已足够承载激活成功后的会话。仅确认 Login.tsx 的 `onLogin` prop 签名不变。

- [ ] **Step 4: 构建验证**

Run: `cd web && npm run build`
Expected: tsc 通过

- [ ] **Step 5: Commit**

```bash
git add web/src/
git commit -m "feat(ui): activation code modal and admin code panel, remove login gate"
```

---

### Task 7: 前端多备选 UI

**Files:**
- Modify: `web/src/routes/Select.tsx`
- Modify: `web/src/routes/Dashboard.tsx`
- Modify: `web/src/types.ts`
- Test: `cd web && npm run build`

**Interfaces:**
- Consumes: `scheduler.Target`（含 priority）；`SchedulerState.courses`（含 priority）
- Produces: 每发布多备选，保存 `{targets:[{publish_id,class_id,course_name,priority}]}`

- [ ] **Step 1: types.ts Target/CourseStatus 加 priority**

```ts
export interface CourseStatus {
  publish_id: number
  class_id: number
  course_name: string
  priority: number
  status: string
  result: string
}

export interface Target {
  publish_id: number
  class_id: number
  course_name: string
  priority: number
}
```

- [ ] **Step 2: Select.tsx 多备选选择逻辑**

`selected` 从 `Record<number, number>` 改为 `Record<number, ClassItem[]>`（有序备选队列）：

```ts
const [selected, setSelected] = useState<Record<number, ClassItem[]>>({})
```

`pick` 追加/移除：

```ts
const pick = (cls: ClassItem) => {
  setSelected((prev) => {
    const list = prev[cls.publish_id] ?? []
    if (list.some((c) => c.id === cls.id)) {
      const next = { ...prev }
      next[cls.publish_id] = list.filter((c) => c.id !== cls.id)
      if (next[cls.publish_id].length === 0) delete next[cls.publish_id]
      toast({ title: "已移除备选", description: `已移出【${cls.course_name}】`, variant: "default" })
      return next
    }
    toast({ title: "已加入备选", description: `备选 ${list.length + 1}：【${cls.course_name}】，满员时自动退避`, variant: "default" })
    return { ...prev, [cls.publish_id]: [...list, cls] }
  })
  setSaved(false)
}
```

`save` 按队列顺序生成 priority：

```ts
const targets: Target[] = []
for (const p of publishes) {
  const list = selected[p.publish_id] ?? []
  list.forEach((cls, i) => {
    targets.push({ publish_id: p.publish_id, class_id: cls.id, course_name: cls.course_name, priority: i })
  })
}
```

回显：stateData.courses 已是按 priority 排序的列表，直接分组回填。

`selectedCount` 改为各发布备选数总和。卡片按钮文案：已选 →「备选 N」；`isSelected` 改为 `selected[publish_id]?.some(c => c.id === c.id)`。`selectedCount` 显示 `X 门 / 共 Y 发布`。tabs 上「已锁定」徽章显示备选数量。

- [ ] **Step 3: Dashboard 展示优先级**

课程卡片加备选序号：

```tsx
{c.priority >= 0 && <Badge variant="outline" className="text-[10px] font-mono">备选 {c.priority + 1}</Badge>}
```

顶部「预选目标课程」区统计改为 `courses.length`。

- [ ] **Step 4: 构建验证**

Run: `cd web && npm run build`
Expected: tsc 通过

- [ ] **Step 5: Commit**

```bash
git add web/src/routes/Select.tsx web/src/routes/Dashboard.tsx web/src/types.ts
git commit -m "feat(ui): multi-backup target queue per publish with priority"
```

---

### Task 8: 端到端验证 + 收尾

**Files:**
- None（仅运行验证）

- [ ] **Step 1: 全量测试**

Run: `cd backend && go test ./... && cd ../web && npm run build`
Expected: 全 PASS

- [ ] **Step 2: 冷启动端到端验证**

```bash
cd backend
# 删旧库（不兼容旧数据）
Remove-Item data/xuanke.db, data/.master_key -Force -ErrorAction SilentlyContinue
$env:XUANKE_ADMIN_TOKEN = "<测试占位口令>"
go run . &
```

- 登录未激活账号 → code 1001
- 管理接口生成激活码（X-Admin-Token）→ 复制 code
- 激活 → 签发会话
- 再次登录 → 直接签发会话（不需要激活码）
- 带会话 GET /api/logs → 仅自己账号日志
- PUT /api/targets 带 priority 多备选 → state 显示
- GET /api/electives 正常

- [ ] **Step 3: 前端开发服务器验证**

Vite dev 代理可用；Login 页无部署口令框；激活码模态框流程可用；管理面板可生成激活码；Select 页发布名显示完整、多备选可选、保存后返回控制台。

- [ ] **Step 4: 更新 CLAUDE.md**

记录：激活码机制（登录流程 1001 / 管理面板 X-Admin-Token）、多备选退避（ErrFull 满员切换）、日志账号隔离、targets.priority / task_log.account / activation 表。

- [ ] **Step 5: 最终提交**

```bash
git add CLAUDE.md
git commit -m "docs: activation codes, backup targets, per-account logs"
```
