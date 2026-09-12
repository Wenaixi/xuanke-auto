# 管理员后台 + 运行时热配置 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan ta***REMOVED***-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用账号 `admin` + 管理口令（= XUANKE_ADMIN_TOKEN 的值）登录进入独立管理员后台界面，集中管理系统运行状态、全部账号、激活码与开关、验证码配置，并支持运行时热重载（不重启进程生效）。

**Architecture:** 复用现有会话认证体系：`handleLogin` 识别 `admin` 账号并比对口令（ConstantTimeCompare）签发特殊管理员会话（`*admin` 标记），`requireAdmin` 改为会话级判定（不依赖 X-Admin-Token）。配置从进程内可变 `runtime.Config` 原子快照读取：启动加载 `data/.env` 注入默认值；`PUT /api/admin/config` 写库（`settings` 表）+ 热更新内存，各组件（调度器、账号管理器、Vision 配置）改为运行时读取而非启动期固化。前端在 App.tsx 分流：`account==='admin'` 渲染独立 Admin 页面（含激活码、运行状态、账号、日志、系统配置五大区）。

**Tech Stack:** Go 1.22+ net/http / modernc.org/sqlite；React 19 + Vite + TypeScript + Tailwind + Radix UI（现有栈，不加依赖）。

**Spec:** [用户需求：admin 账号 + 管理口令登录进管理员界面；管理员界面含激活码管理与开关、系统运行状态、账号管理、日志总览、验证码 openai 格式 url/key/model 设置；配置热重载]

## Global Constraints

- 纯黑白极简 UI（无彩色、无 emoji）；中文文案无 emoji、无 AI 味
- 管理口令唯一来源：`XUANKE_ADMIN_TOKEN`（data/.env 或真实环境变量），缺失拒绝启动（沿用现状）
- 学生账号行为完全不受影响（管理员入口对普通用户无感）；多账号物理隔离不变
- 热重载：所有可配置项（Vision url/key/model、激活码开关、窗口时间）改动后立即生效，无需重启
- 敏感值（Vision APIKey）返回给前端时脱敏（只回显后 4 位 + 掩码）
- schema v4：新增 `settings`（k/v 配置）+ `admins` 冗余防呆（可选）表；不兼容旧库则拒绝启动提示重建
- 密码 AES-GCM 加密不变；不新增第三方依赖
- 每任务结束提交一次；不推送远程

---

### Task 1: runtime 配置中心（内存原子快照 + 热更新）

**Files:**
- Create: `backend/internal/runtime/config.go`
- Modify: `backend/internal/config/config.go`（保留 .env 静态加载，补充 runtime 源）
- Test: `backend/internal/runtime/config_test.go`

**Interfaces:**
- Consumes: `config.Config`（.env 静态默认值）
- Produces: `runtime.Config` 结构与 `runtime.Store`（`Get() Config` 原子快照 / `Update(func(*Config))` / `EnableActivation(bool)` / `SetVision(baseURL, key, model)` / `SetOpenTime(string)`）

- [ ] **Step 1: 定义 Config 与 Store**

```go
// backend/internal/runtime/config.go
package runtime

import (
    "sync"
    "time"
)

type Config struct {
    ActivationEnabled bool   // 激活码机制开关
    VisionBaseURL     string // 验证码识别 OpenAI 兼容地址
    VisionAPIKey      string // 验证码识别密钥
    VisionModel       string // 验证码识别模型
    OpenTime          string // 选课开放时间（本地时区字符串）
    OpenTimeParsed    time.Time
}

type Store struct {
    mu sync.RWMutex
    c  Config
}

func New(initial Config) *Store { ... }          // 拷贝初始值
func (s *Store) Get() Config { ... }             // RLock + 返回拷贝
func (s *Store) Update(f func(*Config)) { ... }  // Lock + 应用 f + 解析 OpenTime
```

- [ ] **Step 2: 写测试（读/改/并发安全）**

```go
// backend/internal/runtime/config_test.go
func TestStoreGetUpdate(t *testing.T) { /* 初始值 -> Update 修改 -> Get 读到新值 */ }
func TestUpdateOpenTimeParse(t *testing.T) { /* OpenTime 合法时 OpenTimeParsed 非零 */ }
```

- [ ] **Step 3: main.go 组装 runtime.Store**

启动时 `cfg := config.Load()` 后构造 `rt := runtime.New(runtime.Config{...从 cfg 拷贝...})`；`config.Load()` 增加 `XUANKE_OPEN_TIME` 读取（默认 `2026-09-13 09:00:00`）。

- [ ] **Step 4: 测试 + 提交**

```bash
cd backend && go test ./internal/runtime/ ./internal/config/
git add backend/internal/runtime backend/internal/config && git commit -m "feat(runtime): in-memory atomic config store with hot reload"
```

---

### Task 2: settings 持久化（schema v4）

**Files:**
- Modify: `backend/internal/db/schema.sql`（新增 settings 表）
- Modify: `backend/internal/db/db.go`（refuseLegacy 增加 settings 表存在检查）
- Modify: `backend/internal/store/store.go`（SaveSettings / LoadSettings map 读写）
- Test: `backend/internal/store/store_test.go`

**Interfaces:**
- Produces: `store.SaveSettings(map[string]string) error`（REPLACE 全量）；`store.LoadSettings() (map[string]string, error)`

- [ ] **Step 1: schema.sql 增加 settings 表**

```sql
-- 系统配置（管理员热重载，k/v 存储）
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

- [ ] **Step 2: store 实现 SaveSettings / LoadSettings（事务全量替换）**

- [ ] **Step 3: 测试 SettingsRoundTrip**

- [ ] **Step 4: 提交**

```bash
git add backend/internal/db backend/internal/store && git commit -m "feat(db): schema v4 settings table for runtime config persistence"
```

---

### Task 3: 会话级管理员鉴权（admin 账号 + 管理口令）

**Files:**
- Modify: `backend/internal/session/store.go`（Session 增加 `Admin bool`）
- Modify: `backend/internal/api/handler.go`（handleLogin 识别 admin + ConstantTimeCompare；issueSessionAdmin）
- Modify: `backend/internal/api/router.go`（`/api/admin/*` 全部改 requireAdminSession）
- Modify: `backend/internal/api/handler_test.go`（TestAdminLogin / 测试更新）

**Interfaces:**
- Consumes: `session.Store` 扩展 `CreateAdmin() string`
- Produces: `sessionAccountRole(r)`（`("admin", true)` 或 `(acct, false)`）；`requireAdminSession(d, next)` 中间件（会话不存在或非 admin → 403）

- [ ] **Step 1: session.Store 增加 Admin 标记**

```go
// store.go
type Session struct {
    Account string
    Admin   bool // 管理员会话（admin 账号登录签发）
    Expires time.Time
}
func (s *Store) Create(account string) string        // 普通会话（Admin:false）
func (s *Store) CreateAdmin() string                 // 管理员会话（Account:"admin", Admin:true）
func (s *Store) Account(token string) (string, bool) // 语义不变
func (s *Store) IsAdmin(token string) bool           // 校验令牌且 Admin
```

- [ ] **Step 2: handleLogin 前置 admin 分支**

```go
if req.Account == "admin" {
    if subtle.ConstantTimeCompare([]byte(req.Password), []byte(d.AdminToken)) != 1 {
        writeJSON(w, 1, nil, "管理口令错误")
        return
    }
    sess := d.Sessions.CreateAdmin()
    d.Store.AppendLog("admin", 0, "login", "管理员登录成功", true)
    writeJSON(w, 0, map[string]string{"token": sess, "account": "admin"}, "管理员登录成功")
    return
}
```

- [ ] **Step 3: requireAdminSession 替换 requireAdmin（X-Admin-Token 从路由彻底移除）**

```go
func requireAdminSession(d *Deps, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if !d.Sessions.IsAdmin(bearerToken(r)) {
            writeJSON(w, 403, nil, "需要管理员权限")
            return
        }
        next(w, r)
    }
}
```

- [ ] **Step 4: 测试 TestAdminLoginOK / TestAdminLoginBadPassword / TestAdminRequiresSession**

- [ ] **Step 5: 提交**

```bash
git add backend/internal/session backend/internal/api && git commit -m "feat(auth): admin account with admin-token password, session-scoped admin middleware"
```

---

### Task 4: 管理员 API（激活码 + 配置 + 状态 + 账号 + 日志总览）

**Files:**
- Modify: `backend/internal/api/handler.go`（新增 handleAdmin* 系列）
- Modify: `backend/internal/api/router.go`（注册新路由）
- Test: `backend/internal/api/handler_test.go`

**Interfaces:**
- Consumes: `runtime.Store`、`store.Store`（新 settings 方法）、`scheduler.Scheduler`、`accounts.Manager`、`session.Store`
- Produces:
  - `GET /api/admin/config` → `{activation_enabled, vision:{base_url,api_key_masked,model}, open_time}`
  - `PUT /api/admin/config` body `{activation_enabled?, vision?, open_time?}` → 热更新 runtime + SaveSettings 持久化，返回新配置
  - `GET /api/admin/stats` → `{window_opened, open_time, accounts, targets, success, log_count}`
  - `GET /api/admin/accounts` → `[{account, targets:[{class_id,course_name,priority}], success:[class_id]}]`
  - `DELETE /api/admin/accounts` body `{account}` → 移除账号（清 credentials/accounts/targets/success/activations）
  - `GET /api/admin/logs?limit=100` → 全量日志（不按账号过滤）
  - 既有 `GET/POST/DELETE /api/admin/codes` 改为会话级 admin

- [ ] **Step 1: Deps 增加 `Runtime *runtime.Store`**

- [ ] **Step 2: handleAdminConfig GET（Vision key 脱敏：`****<后4位>`）/ PUT（热更新 runtime + 落库）**

- [ ] **Step 3: handleAdminStats / handleAdminAccounts / handleAdminDeleteAccount / handleAdminLogs**

- [ ] **Step 4: router.go 全部 admin 路由换 requireAdminSession 并注册新路由**

- [ ] **Step 5: 测试 TestAdminConfigHotReload / TestAdminStats / TestAdminAccounts / TestAdminDeleteAccount / TestAdminLogsAll**

- [ ] **Step 6: 提交**

```bash
git add backend/internal/api && git commit -m "feat(api): admin dashboard APIs with runtime config hot reload"
```

---

### Task 5: 配置热重载打通组件（调度器 + 账号管理器 + Vision）

**Files:**
- Modify: `backend/internal/scheduler/scheduler.go`（openTime 改 runtime 读取）
- Modify: `backend/internal/accounts/manager.go`（Vision 配置动态化）
- Modify: `backend/internal/zhidao/client.go`（recognizeCaptcha 读取调用侧传入 cfg；Client 增加 `SetVision(VisionConfig)`）
- Modify: `backend/main.go`（rt 注入 scheduler / accounts / api）

**Interfaces:**
- Produces: `accounts.Manager.SetVision(VisionConfig)`（遍历各 client 更新 visionCfg）；`zhidao.Client.SetVision(VisionConfig)`
- Consumes: `runtime.Store`

- [ ] **Step 1: zhidao.Client 加 SetVision + 锁保护 visionCfg**

- [ ] **Step 2: accounts.Manager 加 SetVision（遍历所有 client）**

- [ ] **Step 3: scheduler 打开时间改 runtime（tick/StateForAccount 读 `rt.Get().OpenTimeParsed`）**

- [ ] **Step 4: 测试 scheduler 打开时间用 runtime 快照后仍通过**

- [ ] **Step 5: 提交**

```bash
git add backend/internal/scheduler backend/internal/accounts backend/internal/zhidao backend/main.go && git commit -m "feat(hot-reload): scheduler open time and vision config read runtime store"
```

---

### Task 6: 前端路由分流（admin 独立页面）

**Files:**
- Modify: `web/src/App.tsx`（account==='admin' → Admin 页）
- Create: `web/src/routes/Admin.tsx`（五大区布局 + 顶栏退出）
- Modify: `web/src/routes/Login.tsx`（删除激活码管理入口的文案无关项；无改动或微调说明文案）
- Modify: `web/src/types.ts`（新增 AdminConfig / AdminStats / AdminAccount / AdminLog 类型）

**Interfaces:**
- Consumes: 后端 `/api/admin/*` 全部接口（会话 token 直接 Bearer，无需 X-Admin-Token）
- Produces: Admin 页面五个 Tab 区块

- [ ] **Step 1: types.ts 新增类型**

- [ ] **Step 2: Admin.tsx 骨架：顶栏（返回学生端/退出）+ 五个 Tab**

- [ ] **Step 3: Tab1 激活码管理（复用现有逻辑，去掉 X-Admin-Token，改会话 token）**

- [ ] **Step 4: Tab2 系统配置（激活码开关 switch、Vision url/key/model 输入 + 保存；保存后 toast + refetch config）**

- [ ] **Step 5: Tab3 运行状态（窗口时间/开关、账号数、目标数、成功数、日志数）**

- [ ] **Step 6: Tab4 账号管理（列表 + 目标详情 + 删除按钮）**

- [ ] **Step 7: Tab5 日志总览（全量日志流，不按账号过滤）**

- [ ] **Step 8: App.tsx 分流 + Login 说明文案（admin 专属入口提示）**

- [ ] **Step 9: 构建验证 + 提交**

```bash
cd web && npm run build
git add web/src && git commit -m "feat(ui): admin dashboard page with five management sections"
```

---

### Task 7: 全链路验证 + 冷启动端到端

**Files:**
- Modify: `backend/internal/api/handler_test.go`（补充 admin 全流程测试）
- Modify: `CLAUDE.md`（记录 admin 后台与热重载决策）

- [ ] **Step 1: 全量测试** `cd backend && go test -race ./...`

- [ ] **Step 2: 冷启动验证（删除 data/xuanke.db + data/.env 模板重生成后手动填值）**
  - admin/管理口令登录 → 返回管理员会话 token
  - GET /api/admin/config 读到 .env 默认值（activation on + Vision）
  - PUT /api/admin/config 改 Vision key/model + 激活码 off → 立即生效（无需重启）
  - 普通账号登录仍走激活码流程；激活码 off 时直接进系统
  - /api/admin/accounts 列账号、删除账号、日志总览可见全部
- [ ] **Step 3: 前端 dev 验证（vite 5173 + go run 3091 代理）**

- [ ] **Step 4: 更新 CLAUDE.md（管理员后台 + 热配置决策 + 部署文档 admin 入口）**

- [ ] **Step 5: 最终提交**

```bash
git add CLAUDE.md backend web && git commit -m "docs: admin dashboard, runtime hot config, deployment with admin account"
```
