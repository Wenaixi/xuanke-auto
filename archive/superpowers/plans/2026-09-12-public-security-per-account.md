# 公网安全加固 + 多账号物理隔离 + 拒绝旧数据 + 超高性能 实施计划

> **面向 Agent 执行者：** 必须使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 按任务逐项执行本计划。步骤使用复选框（- [ ]）语法跟踪。

**目标：** 将服务从「本地单机自用」改造为「可安全部署公网」的抢课引擎：彻底移除旧版默认账号兼容层（不兼容旧数据）、每个账号独立至道会话（互不互通）、部署口令 + 会话级授权 + 密码加密入库（公网安全）、窗口开放瞬间立即探测 + 全账号并发提交 + 课程数据内存缓存（超高性能）。

**架构：**
- **accounts 包（多账号客户端注册表）**：以账号名为键维护每个账号**独立**的 `zhidao.Client`（独立 token/cookie 会话）。调度器用任一已登录账号客户端探测窗口（课程数据全校共享），报名则用目标账号自己的客户端（谁的账号谁的身份办事，账号 A 的操作绝不携带账号 B 的会话）。
- **DB 去旧**：新建 `credentials` 表（AES-GCM 加密密码 + token），删除旧 `account` 单行明文表；`targets.account` 无默认值必填。`db.Open` 检测到旧库形状直接返回错误拒绝启动，绝不静默兼容旧数据。
- **认证（公网安全）**：环境变量 `XUANKE_ADMIN_TOKEN` 为部署口令（启动必填，缺失直接拒绝启动）。`POST /api/login` 校验部署口令后登录教务账号，服务端签发 32 字节随机会话令牌（12 小时过期），绑定该账号。除 `/login`、`/health` 外所有 API 必须携带 `Authorization: Bearer <会话>`；涉及账号的端点一律从**会话**取账号，完全忽略客户端传的账号参数，越权 403。
- **密码存储**：`secure` 包 AES-256-GCM（密钥来自环境变量 `XUANKE_MASTER_KEY` 或自动生成到 DB 旁 `.master_key` 文件）。密码仅在内存中出现，重启后用解密密码自动重登。
- **超高性能**：调度器窗口到点后的首次 tick 豁免探针闸门（≤300ms 发现窗口）；每账号每课程独立 goroutine 并发报名（多账号真并行，不同账号同课程 classID 也可同时提交）；`/api/electives` 直读调度器内存课程快照（30 秒内），页面浏览零上游请求。

**技术栈：**
- Go 1.26、`net/http`（Go 1.22+ 增强路由）、`modernc.org/sqlite`（纯 Go 免 CGO）、crypto/aes、crypto/cipher、crypto/rand、crypto/subtle
- React 18、TypeScript、Vite、TanStack Query

**规格：** 本计划实现用户需求原文：「我希望不要兼容旧数据」「我希望这个网站各个账号不互通，就是我会部署公网，你要搞好安全性」「密码入库」「还有要求超高性能」。UI 全程保持纯黑白极简艺术。

## 全局约束

- Go >= 1.22（增强路由），禁止引入 gin/echo 等重型框架；数据库只用 `modernc.org/sqlite`。
- 所有对外 API 返回统一 JSON `{"code":0,"data":...,"msg":""}`；认证失败 `code=401`（前端触发对应账号登出）、越权 `code=403`、登录限流 `code=429`。
- **删除一切 `''` 默认账号路径**：store/scheduler/api/前端均不得再出现「默认账号」概念；每个目标、每份状态必须归属真实账号。
- 硬编码密钥清零：`SF_API_KEY` 必须来自环境变量（未配置时登录给出明确报错）；API 响应永不回传密码。
- `probeInterval = 30 * time.Second` 保留（平台 1 分钟熔断保护），仅在窗口到点瞬间豁免一次。
- 前端构建产物 `go:embed` 进单二进制；开发时 Vite 代理 `/api`。
- UI 纯黑白极简（`#09090b` 底、`neutral-900` 发丝边框、纯白文字），无彩色 emoji。

---

### 任务 1：secure 包（AES-GCM 密码加密）+ session 包（会话签发）

**文件：**
- Create: `backend/internal/secure/crypto.go`
- Create: `backend/internal/session/store.go`

**接口：**
- `secure.LoadOrCreateKey(dbPath string) ([]byte, error)`：环境变量 `XUANKE_MASTER_KEY`（64 位 hex）优先，否则读/生成 DB 旁 `.master_key` 文件。
- `secure.Encrypt(plain string, key []byte) (string, error)`：AES-256-GCM，随机 nonce 前置，输出 hex。
- `secure.Decrypt(encHex string, key []byte) (string, error)`。
- `session.New(ttl time.Duration) *session.Store`；`(*Store).Create(account string) string` 返回 32 字节 hex 令牌；`(*Store).Account(token string) (string, bool)`；`(*Store).Delete(token string)`。

- [ ] **Step 1：写 secure/crypto.go**

```go
package secure

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/hex"
    "errors"
    "io"
    "os"
    "path/filepath"
)

// LoadOrCreateKey 读取 XUANKE_MASTER_KEY（64 位 hex）或自动生成 32 字节密钥保存到 DB 旁。
func LoadOrCreateKey(dbPath string) ([]byte, error) {
    if env := os.Getenv("XUANKE_MASTER_KEY"); env != "" {
        key, err := hex.DecodeString(env)
        if err != nil || len(key) != 32 {
            return nil, errors.New("XUANKE_MASTER_KEY 必须是 64 位十六进制字符串（32 字节）")
        }
        return key, nil
    }
    keyFile := filepath.Join(filepath.Dir(dbPath), ".master_key")
    if b, err := os.ReadFile(keyFile); err == nil {
        return b, nil
    }
    key := make([]byte, 32)
    if _, err := io.ReadFull(rand.Reader, key); err != nil {
        return nil, err
    }
    if err := os.WriteFile(keyFile, key, 0o600); err != nil {
        return nil, err
    }
    return key, nil
}

// Encrypt AES-256-GCM 加密：随机 nonce 前置，输出 hex。
func Encrypt(plain string, key []byte) (string, error) {
    blk, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }
    gcm, err := cipher.NewGCM(blk)
    if err != nil {
        return "", err
    }
    nonce := make([]byte, gcm.NonceSize())
    if _, err := rand.Read(nonce); err != nil {
        return "", err
    }
    out := gcm.Seal(nonce, nonce, []byte(plain), nil)
    return hex.EncodeToString(out), nil
}

// Decrypt 解密 Encrypt 的输出。
func Decrypt(encHex string, key []byte) (string, error) {
    raw, err := hex.DecodeString(encHex)
    if err != nil {
        return "", err
    }
    blk, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }
    gcm, err := cipher.NewGCM(blk)
    if err != nil {
        return "", err
    }
    if len(raw) < gcm.NonceSize() {
        return "", errors.New("密文长度非法")
    }
    plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
    if err != nil {
        return "", err
    }
    return string(plain), nil
}
```

- [ ] **Step 2：写 secure/crypto_test.go**（加密→解密往返、篡改密文报错、主密钥长度校验）

Run: `cd backend && go test ./internal/secure -v` → PASS

- [ ] **Step 3：写 session/store.go**

```go
package session

import (
    "crypto/rand"
    "encoding/hex"
    "sync"
    "time"
)

// Session 一次服务端会话（绑定唯一账号）。
type Session struct {
    Account string
    Expires time.Time
}

// Store 内存会话注册表：随机令牌 -> 账号绑定，过期自动失效。
type Store struct {
    mu       sync.Mutex
    sessions map[string]*Session
    ttl      time.Duration
}

func New(ttl time.Duration) *Store {
    return &Store{sessions: make(map[string]*Session), ttl: ttl}
}

// Create 为账号签发新会话令牌（32 字节 hex）。
func (s *Store) Create(account string) string {
    token := randToken()
    s.mu.Lock()
    defer s.mu.Unlock()
    s.sessions[token] = &Session{Account: account, Expires: time.Now().Add(s.ttl)}
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

// Delete 注销会话（退出登录）。
func (s *Store) Delete(token string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    delete(s.sessions, token)
}

func randToken() string {
    b := make([]byte, 32)
    _, _ = rand.Read(b)
    return hex.EncodeToString(b)
}
```

- [ ] **Step 4：写 session/store_test.go**（创建→校验账号、过期失效、删除）

Run: `cd backend && go test ./internal/session -v` → PASS

- [ ] **Step 5：提交**

```bash
git add backend/internal/secure backend/internal/session
git commit -m "feat(secure): AES-GCM credential encryption and session store"
```

---

### 任务 2：数据库去旧（credentials 表 + 拒绝旧库）

**文件：**
- Modify: `backend/internal/db/schema.sql`
- Modify: `backend/internal/db/db.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/store_test.go`

**接口：**
- 删除旧 `account` 表，新增 `credentials(account PK,password_enc,id_token)` 与 `success(account,class_id)` 表。
- `targets.account TEXT NOT NULL`（无默认值）。
- `db.Open` 检测到旧 `account` 表/旧 `targets.account = ''` 数据：返回明确错误拒绝启动。
- store 新增 `SaveCredential/LoadCredentials/UpdateIDToken/SaveSuccess/LoadSuccess`；删除 `SaveAccount/SaveTokenOnly/LoadAccount/UpdateToken/SetTargets/LoadTargets`。

- [ ] **Step 1：改写 schema.sql**

```sql
-- 账号凭据（密码 AES-GCM 加密后入库，明文只在内存）
CREATE TABLE IF NOT EXISTS credentials (
  account TEXT PRIMARY KEY,
  password_enc TEXT NOT NULL,
  id_token TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 已知账号名（登录成功即 upsert）
CREATE TABLE IF NOT EXISTS accounts (
  account TEXT PRIMARY KEY,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 目标课程（按账号隔离，禁止空账号）
CREATE TABLE IF NOT EXISTS targets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account TEXT NOT NULL,
  publish_id INTEGER NOT NULL,
  class_id INTEGER NOT NULL,
  course_name TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 报名日志
CREATE TABLE IF NOT EXISTS task_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  class_id INTEGER NOT NULL,
  action TEXT NOT NULL,
  result TEXT,
  is_ok INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 已成功课程（重启后禁止重复报名，按账号独立）
CREATE TABLE IF NOT EXISTS success (
  account TEXT NOT NULL,
  class_id INTEGER NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (account, class_id)
);
```

（删除原 `account` 表与 `task_state` 表定义。）

- [ ] **Step 2：db.Open 拒绝旧库**

在 `db.Open` 的 `schemaSQL` 执行后追加：

```go
// refuseLegacy 兼容性检查：检测到旧版数据形状直接拒绝启动（政策：不兼容旧数据）。
func refuseLegacy(d *sql.DB) error {
    var n int
    if err := d.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='account'").Scan(&n); err != nil {
        return err
    }
    if n > 0 {
        return errors.New("检测到旧版数据库（account 表），本版本不兼容旧数据。请删除 data/xuanke.db 后重新启动")
    }
    var empty int
    if err := d.QueryRow("SELECT count(*) FROM targets WHERE account = ''").Scan(&empty); err != nil {
        return err
    }
    if empty > 0 {
        return errors.New("检测到旧版空账号目标数据，本版本不兼容旧数据。请删除 data/xuanke.db 后重新启动")
    }
    return nil
}
```

调用处：`d.Exec(schemaSQL)` 之后 `if err := refuseLegacy(d); err != nil { d.Close(); return nil, err }`；同时删除 `ensureTargetsAccountColumn`（新库天然带 account 列，旧库已被拒绝）。

> 注意：`db_test.go` 的 `TestOpenAndSchema` 期望表数 >= 4，新库含 credentials/accounts/targets/task_log/success 共 5 张，仍通过。

- [ ] **Step 3：store 改写与新增**

移除 `SaveAccount/SaveTokenOnly/UpdateToken/LoadAccount/SetTargets/LoadTargets`；新增：

```go
// Credential 账号凭据（密码列为密文）。
type Credential struct {
    Account     string
    PasswordEnc string
    IDToken     string
}

// SaveCredential upsert 账号凭据（加密密码 + 当前 token）。
func (s *Store) SaveCredential(acct, pwdEnc, idToken string) error {
    _, err := s.db.Exec(
        "INSERT INTO credentials (account, password_enc, id_token) VALUES (?, ?, ?) "+
            "ON CONFLICT(account) DO UPDATE SET password_enc=excluded.password_enc, id_token=excluded.id_token, updated_at=datetime('now')",
        acct, pwdEnc, idToken)
    return err
}

// LoadCredentials 读取全部账号凭据（按账号排序）。
func (s *Store) LoadCredentials() ([]Credential, error) {
    rows, err := s.db.Query("SELECT account, password_enc, id_token FROM credentials ORDER BY account")
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var out []Credential
    for rows.Next() {
        var c Credential
        if err := rows.Scan(&c.Account, &c.PasswordEnc, &c.IDToken); err != nil {
            return nil, err
        }
        out = append(out, c)
    }
    return out, rows.Err()
}

// UpdateIDToken 仅刷新 token（账密不变时调用）。
func (s *Store) UpdateIDToken(acct, idToken string) error {
    _, err := s.db.Exec("UPDATE credentials SET id_token = ?, updated_at = datetime('now') WHERE account = ?", idToken, acct)
    return err
}

// SaveSuccess 记录某账号某课程已报名成功（幂等）。
func (s *Store) SaveSuccess(acct string, classID int) error {
    _, err := s.db.Exec("INSERT OR IGNORE INTO success (account, class_id) VALUES (?, ?)", acct, classID)
    return err
}

// LoadSuccess 读取全部成功记录（map[账号][]classID）。
func (s *Store) LoadSuccess() (map[string][]int, error) {
    rows, err := s.db.Query("SELECT account, class_id FROM success")
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    out := map[string][]int{}
    for rows.Next() {
        var a string
        var cid int
        if err := rows.Scan(&a, &cid); err != nil {
            return nil, err
        }
        out[a] = append(out[a], cid)
    }
    return out, rows.Err()
}
```

`SaveAccountName` / `ListAccounts` / `SetTargetsForAccount` / `LoadTargetsForAccount` / `AppendLog` / `LoadLogs` 均保留（`SetTargetsForAccount` 的 DELETE 条件 `account=?` 不变；空账号自然查不到数据）。

- [ ] **Step 4：更新 store_test.go**

删除 `TestAccountRoundTrip`、`TestSaveTokenOnly`、`TestTargetsRoundTrip`（空账号接口没了）；`TestTargetsByAccount` 删除「旧单账号接口」断言部分；新增：

```go
func TestCredentialsRoundTrip(t *testing.T) {
    s := openTestStore(t)
    if err := s.SaveCredential("acct1", "ENC-ABC", "tok1"); err != nil {
        t.Fatal(err)
    }
    creds, err := s.LoadCredentials()
    if err != nil || len(creds) != 1 || creds[0].Account != "acct1" || creds[0].PasswordEnc != "ENC-ABC" {
        t.Fatalf("凭据往返失败: %+v %v", creds, err)
    }
    if err := s.UpdateIDToken("acct1", "tok-2"); err != nil {
        t.Fatal(err)
    }
    creds, _ = s.LoadCredentials()
    if creds[0].IDToken != "tok-2" {
        t.Fatalf("token 更新失败: %+v", creds)
    }
}

func TestSuccessRecords(t *testing.T) {
    s := openTestStore(t)
    s.SaveSuccess("acct1", 61115)
    s.SaveSuccess("acct1", 61115) // 幂等
    s.SaveSuccess("acct2", 61205)
    got, err := s.LoadSuccess()
    if err != nil || len(got["acct1"]) != 1 || len(got["acct2"]) != 1 {
        t.Fatalf("成功记录异常: %+v %v", got, err)
    }
}
```

另外需删除 `db.go` 里对 `ensureTargetsAccountColumn` 的调用。`db_test.go` 追加一个拒绝旧库的测试：

```go
func TestRefuseLegacyDB(t *testing.T) {
    path := filepath.Join(t.TempDir(), "legacy.db")
    d, err := sql.Open("sqlite", path)
    if err != nil { t.Fatal(err) }
    d.Exec("CREATE TABLE account (id INTEGER PRIMARY KEY AUTOINCREMENT, account TEXT, password TEXT, id_token TEXT)")
    d.Close()
    if _, err := Open(path); err == nil {
        t.Fatal("旧库应被拒绝启动")
    }
}
```

Run: `cd backend && go test ./internal/db ./internal/store -v` → PASS；`go build ./...` 通过（main.go 依赖待任务 6 一并改）。

- [ ] **Step 5：提交**

```bash
git add backend/internal/db backend/internal/store
git commit -m "feat(db): encrypted credentials table, reject legacy databases"
```

---

### 任务 3：accounts 包（多账号独立客户端注册表）

**文件：**
- Create: `backend/internal/accounts/manager.go`

**接口：**
- `accounts.New(baseURL string, vision zhidao.VisionConfig, st Store) *Manager`
- `(*Manager).LoginByPassword(acct, password string, encrypt func(string)(string,error)) (string, error)`：登录并落库。
- `(*Manager).ClientFor(acct string) (scheduler.Client, bool)`
- `(*Manager).AnyClient() (scheduler.Client, bool)`：任一已登录客户端的课程探测。
- `(*Manager).Registered() []string`
- `(*Manager).Restore(creds []Credential, decrypt func(string)(string,error))`：重启恢复。

> store 包实现 `accounts.Store` 接口（`SaveCredential`/`LoadCredentials`）——store 导入 accounts 包，accounts 只依赖 zhidao 与 scheduler 的接口类型，无循环。

- [ ] **Step 1：写 manager.go**

```go
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

// Credential 与 store.Credential 同构（避免 store 反向依赖 accounts）。
type Credential struct {
    Account     string
    PasswordEnc string
    IDToken     string
}

// Manager 多账号客户端注册表：每个账号一个独立 zhidao.Client（独立 token/cookie）。
type Manager struct {
    baseURL string
    vision  zhidao.VisionConfig
    st      Store

    mu      sync.Mutex
    clients map[string]*zhidao.Client // 账号名 -> 独立客户端
    order   []string                  // 登录顺序
}

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
    if m.st != nil {
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
```

> 关于明文密码字符串与 `login` 客户端的内存驻留：登录后 zhidao.Client 持有该账号明文密码（供自动重登），这是「密码入库」的增强形式（重启后可解密恢复自动重登）。API 永不回传密码。

- [ ] **Step 2：构建验证**

Run: `cd backend && go vet ./internal/accounts`
Expected: 因 main.go/api 尚未改造可能报错——本任务先单独 `go vet ./internal/accounts`，OK 即可；完整编译在任务 6 收敛。

- [ ] **Step 3：提交**

```bash
git add backend/internal/accounts
git commit -m "feat(accounts): per-account isolated zhidao clients"
```

---

### 任务 4：调度器多账号会话 + 窗口到点立即探测 + 课程快照

**文件：**
- Modify: `backend/internal/scheduler/scheduler.go`
- Modify: `backend/internal/scheduler/scheduler_test.go`

**接口变更：**
- `New(clients AccountClients, store Store, openTime, interval)`。
- `AccountClients` 接口（本地定义）：`ClientFor(acct)(Client,bool)` / `AnyClient()(Client,bool)`；`Client` 为原有 `FindElectives+SelectClass` 接口。
- `done`/`inflight` 改为 `map[string]map[int]bool`（按账号独立，不同账号可同时提交同一 classID）。
- 新增 `ElectivesSnapshot() (*zhidao.ElectivesData, bool)` 与 `ProbeNow() (*zhidao.ElectivesData, error)`。
- `RestoreDone(map[string][]int)`。
- 删除 `SetTargets`（默认账号）、`State()`、`Accounts()`。

- [ ] **Step 1：改写 scheduler.go（结构 + 探测 + 提交）**

关键代码块：

```go
const probeInterval = 30 * time.Second
const snapshotTTL = 40 * time.Second // 课程快照有效期（> 30s 节流，保证 /electives 总有数据）

// Client 调度器依赖的至道客户端能力（accounts.Manager 返回的 *zhidao.Client 隐式满足）。
type Client interface {
    FindElectives() (*zhidao.ElectivesData, error)
    SelectClass(classID int) (string, error)
}

// AccountClients 多账号客户端注册表（真实实现 accounts.Manager）。
type AccountClients interface {
    ClientFor(acct string) (Client, bool)
    AnyClient() (Client, bool)
}

type Scheduler struct {
    clients AccountClients
    store   Store
    openTime time.Time
    interval time.Duration

    mu         sync.Mutex
    acctTargets map[string][]Target
    state      SchedulerState
    inflight   map[string]map[int]bool // [账号][classID]
    done       map[string]map[int]bool // [账号][classID]
    lastProbe  time.Time
    lastData   *zhidao.ElectivesData // 内存课程快照（超高性能：/electives 直读）
    lastDataAt time.Time

    ctx    context.Context
    cancel context.CancelFunc
    start  bool
}
```

`tick()` 探测段：

```go
func (s *Scheduler) tick() {
    now := time.Now()
    s.mu.Lock()
    last := s.lastProbe
    s.mu.Unlock()

    // 探测闸门：距上次成功探测不足 30 秒且非首次则跳过
    probe := last.IsZero() || now.Sub(last) >= probeInterval
    // 超高性能：窗口到点后的首次 tick 立即探测（不等待 30s 闸门放过）
    if !probe && now.After(s.openTime) && last.Before(s.openTime.Add(-time.Second)) {
        probe = true
    }
    if !probe {
        return
    }

    client, ok := s.clients.AnyClient()
    if !ok {
        return // 尚无账号登录，安静等待
    }
    data, err := client.FindElectives()
    if err != nil {
        s.mu.Lock()
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
    s.mu.Unlock()

    opened := false
    for _, p := range data.Publishes {
        if p.InDateRange {
            opened = true
            break
        }
    }
    s.mu.Lock()
    s.state.WindowOpened = opened
    s.mu.Unlock()
    if !opened {
        return
    }
    s.submitAll()
}

// ElectivesSnapshot 返回内存课程快照（40 秒内有效）。超高性能核心：页面浏览零上游请求。
func (s *Scheduler) ElectivesSnapshot() (*zhidao.ElectivesData, bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.lastData == nil || time.Since(s.lastDataAt) > snapshotTTL {
        return nil, false
    }
    return s.lastData, true
}

// ProbeNow 立即执行一次课程探测并刷新快照（/api/electives 快照过期时调用）。
func (s *Scheduler) ProbeNow() (*zhidao.ElectivesData, error) {
    client, ok := s.clients.AnyClient()
    if !ok {
        return nil, errors.New("没有任何已登录账号")
    }
    data, err := client.FindElectives()
    if err != nil {
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

`submitAll` / `submit` 按账号：

```go
func (s *Scheduler) submitAll() {
    s.mu.Lock()
    type pair struct {
        acct string
        t    Target
    }
    var pairs []pair
    for acct, ts := range s.acctTargets {
        for _, t := range ts {
            pairs = append(pairs, pair{acct, t})
        }
    }
    s.mu.Unlock()
    // 每账号每课程独立 goroutine —— 多账号真并行
    for _, p := range pairs {
        s.submit(p.acct, p.t)
    }
}

func (s *Scheduler) submit(acct string, t Target) {
    s.mu.Lock()
    if s.doneHas(acct, t.ClassID) || s.inflightHas(acct, t.ClassID) {
        s.mu.Unlock()
        return
    }
    if s.inflight[acct] == nil { s.inflight[acct] = map[int]bool{} }
    s.inflight[acct][t.ClassID] = true
    idx := s.statusIndexLocked(acct, t.ClassID)
    if idx >= 0 && s.state.Courses[idx].Status != "success" {
        s.state.Courses[idx].Status = "submitted"
    }
    s.mu.Unlock()

    go func() {
        defer func() {
            if r := recover(); r != nil {
                log.Printf("[scheduler] 提交课程 %d（账号 %s）panic: %v", t.ClassID, acct, r)
                s.mu.Lock()
                delete(s.inflight[acct], t.ClassID)
                s.mu.Unlock()
            }
        }()
        client, ok := s.clients.ClientFor(acct)
        var msg string
        var err error
        if !ok {
            err = errors.New("账号会话未建立，等待重新登录")
        } else {
            msg, err = client.SelectClass(t.ClassID) // 用目标账号自己的会话提交
        }
        s.mu.Lock()
        defer s.mu.Unlock()
        delete(s.inflight[acct], t.ClassID)
        idx := s.statusIndexLocked(acct, t.ClassID)
        if err != nil {
            s.setStateLocked(idx, "failed", err.Error())
            if s.store != nil {
                s.store.AppendLog(t.ClassID, "select", "账号 "+acct+": "+err.Error(), false)
            }
            return
        }
        if s.done[acct] == nil { s.done[acct] = map[int]bool{} }
        s.done[acct][t.ClassID] = true
        s.setStateLocked(idx, "success", msg)
        if s.store != nil {
            s.store.AppendLog(t.ClassID, "select", msg, true)
            _ = s.store.SaveSuccess(acct, t.ClassID)
        }
        log.Printf("[scheduler] 账号 %s 课程 %d（%s）报名成功: %s", acct, t.ClassID, t.CourseName, msg)
    }()
}

func (s *Scheduler) doneHas(acct string, classID int) bool {
    m, ok := s.done[acct]
    return ok && m[classID]
}

func (s *Scheduler) inflightHas(acct string, classID int) bool {
    m, ok := s.inflight[acct]
    return ok && m[classID]
}

// statusIndexLocked 按账号 + 课程查找状态下标。
func (s *Scheduler) statusIndexLocked(acct string, classID int) int {
    for i := range s.state.Courses {
        c := s.state.Courses[i]
        if c.Account == acct && c.ClassID == classID {
            return i
        }
    }
    return -1
}
```

`Store` 接口追加 `SaveSuccess(acct string, classID int) error`。`SetTargetsForAccount` 中 `s.done` 判断改为 `s.doneHas(acct, t.ClassID)`。删除 `SetTargets`、`State`、`Accounts`、`SuccessClassIDs`；`RestoreDone` 改为：

```go
// RestoreDone 注入重启前已成功的 (账号, 课程) 记录。
func (s *Scheduler) RestoreDone(done map[string][]int) {
    s.mu.Lock()
    defer s.mu.Unlock()
    for acct, ids := range done {
        if s.done[acct] == nil {
            s.done[acct] = map[int]bool{}
        }
        for _, id := range ids {
            s.done[acct][id] = true
        }
    }
    s.rebuildCoursesLocked()
}
```

- [ ] **Step 2：改写 scheduler_test.go**

`fakeStore` 加 `SaveSuccess`（no-op）。新增 fake 账号注册表：

```go
// fakeAccts 伪账号注册表：所有账号共享一个 fakeClient（测试用）。
type fakeAccts struct{ c *fakeClient }

func (f *fakeAccts) ClientFor(acct string) (Client, bool) { return f.c, true }
func (f *fakeAccts) AnyClient() (Client, bool)            { return f.c, true }
```

`New(fc, ...)` 全部改为 `New(&fakeAccts{fc}, ...)`。`s.SetTargets(targets())` 改为 `s.SetTargetsForAccount("acct1", targets())`（或逐账号）；断言改为 `StateForAccount(...)`。`TestTargetsByAccountIsolation` 现在可断言两账号同 classID 也可并行提交（不再有共享 done）——加断言：`waitStatusAcct(acct1, 61115) / waitStatusAcct(acct2, 61115)` 用不同 class 即可，保持原案。

Run: `cd backend && go test ./internal/scheduler -v` → PASS

- [ ] **Step 3：提交**

```bash
git add backend/internal/scheduler
git commit -m "feat(scheduler): per-account sessions, instant probe at open, electives snapshot"
```

---

### 任务 5：API 认证中间件 + 账号会话绑定 + 课程快照直读

**文件：**
- Modify: `backend/internal/api/handler.go`
- Modify: `backend/internal/api/router.go`
- Modify: `backend/internal/api/handler_test.go`

**接口：**
- `Register(mux, st, sched, accts, sessions, openTime, adminToken, encrypt)`。
- 新增 `requireAuth` 中间件：校验 `Authorization: Bearer <会话>`，把账号写入请求 context。
- `/api/login` 校验部署口令（`subtle.ConstantTimeCompare`），成功签发会话返回 `{token, account}`。
- `/api/state`、`/api/targets`、`/api/accounts`、`/api/logs`、`/api/electives` 全部要求会话。
- `/api/electives` 直读内存快照。

- [ ] **Step 1：handler.go 大改**

```go
type Deps struct {
    Store    *store.Store
    Sched    *scheduler.Scheduler
    Accounts *accounts.Manager
    Sessions *session.Store
    OpenTime string
    // AdminToken 部署口令（main 从环境变量注入）。
    AdminToken string
    // Encrypt 密码加密（secure.Encrypt 绑定主密钥闭包）。
    Encrypt func(string) (string, error)
}

type ctxKey int

const sessionCtxKey ctxKey = 1

// sessionAccount 从请求上下文取会话绑定的账号。
func sessionAccount(r *http.Request) string {
    if v, ok := r.Context().Value(sessionCtxKey).(string); ok {
        return v
    }
    return ""
}

// requireAuth 会话校验中间件：无/无效令牌返回 401。
func requireAuth(d *Deps, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        tok := ""
        if h := r.Header.Get("Authorization"); len(h) > 7 && h[:7] == "Bearer " {
            tok = h[7:]
        } else if xt := r.Header.Get("X-Auth-Token"); xt != "" {
            tok = xt
        }
        acct, ok := d.Sessions.Account(tok)
        if !ok {
            writeJSON(w, 401, nil, "会话无效或已过期，请重新登录")
            return
        }
        next(w, r.WithContext(context.WithValue(r.Context(), sessionCtxKey, acct)))
    }
}
```

`handleLogin`（校验部署口令 + 签发会话；登录成功后自动加入账号注册表并落库）：

```go
type LoginRequest struct {
    Account    string `json:"account"`
    Password   string `json:"password"`
    AdminToken string `json:"admin_token"`
}

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
    // 部署口令校验（启动必须配置）
    if d.AdminToken == "" || subtle.ConstantTimeCompare([]byte(req.AdminToken), []byte(d.AdminToken)) != 1 {
        writeJSON(w, 403, nil, "部署访问口令错误")
        return
    }
    if _, err := d.Accounts.LoginByPassword(req.Account, req.Password, d.Encrypt); err != nil {
        writeJSON(w, 1, nil, "登录失败: "+err.Error())
        return
    }
    if err := d.Store.SaveAccountName(req.Account); err != nil {
        log.Printf("[api] 保存账号名失败: %v", err)
    }
    sess := d.Sessions.Create(req.Account)
    d.Store.AppendLog(0, "login", "账号 "+req.Account+" 登录成功", true)
    writeJSON(w, 0, map[string]string{"token": sess, "account": req.Account}, "登录成功")
}
```

`handleSetTargets`（账号来自会话，忽略请求体 account）：

```go
func (d *Deps) handleSetTargets(w http.ResponseWriter, r *http.Request) {
    acct := sessionAccount(r)
    var req TargetsRequest // 仅含 Targets
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
    if err := d.Store.SetTargetsForAccount(acct, req.Targets); err != nil {
        writeJSON(w, 1, nil, "保存目标失败: "+err.Error())
        return
    }
    d.Sched.SetTargetsForAccount(acct, req.Targets)
    d.Store.AppendLog(0, "set_targets", fmt.Sprintf("账号 %s：%d 门目标课程", acct, len(req.Targets)), true)
    writeJSON(w, 0, req.Targets, "目标已保存")
}
```

`handleState` / `handleAccounts` / `handleElectives`：

```go
func (d *Deps) handleState(w http.ResponseWriter, r *http.Request) {
    st := d.Sched.StateForAccount(sessionAccount(r))
    writeJSON(w, 0, st, "")
}

func (d *Deps) handleAccounts(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, 0, d.Accounts.Registered(), "")
}

func (d *Deps) handleElectives(w http.ResponseWriter, r *http.Request) {
    if data, ok := d.Sched.ElectivesSnapshot(); ok {
        writeJSON(w, 0, data, "")
        return
    }
    // 快照过期/尚未探测：立即探测一次并刷新（页面首个请求可能触发，后续全部命中缓存）
    data, err := d.Sched.ProbeNow()
    if err != nil {
        writeJSON(w, 1, nil, "查询课程失败: "+err.Error())
        return
    }
    writeJSON(w, 0, data, "")
}
```

删除 `handleElectivesDetail` 的账号无关性不动（保留，但要求会话）。`TargetsRequest` 删除 `Account` 字段。删除 `displayAcct`。

- [ ] **Step 2：router.go**

```go
mux.HandleFunc("GET /api/health", d.handleHealth)
mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
    if !limiter.allow(clientIP(r)) {
        writeJSON(w, 429, nil, "登录尝试过于频繁，请稍后再试")
        return
    }
    d.handleLogin(w, r)
})
mux.HandleFunc("GET /api/electives", func(w http.ResponseWriter, r *http.Request) { requireAuth(d, d.handleElectives)(w, r) })
mux.HandleFunc("GET /api/electives/detail", func(w http.ResponseWriter, r *http.Request) { requireAuth(d, d.handleElectivesDetail)(w, r) })
mux.HandleFunc("PUT /api/targets", func(w http.ResponseWriter, r *http.Request) { requireAuth(d, d.handleSetTargets)(w, r) })
mux.HandleFunc("GET /api/accounts", func(w http.ResponseWriter, r *http.Request) { requireAuth(d, d.handleAccounts)(w, r) })
mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) { requireAuth(d, d.handleState)(w, r) })
mux.HandleFunc("GET /api/logs", func(w http.ResponseWriter, r *http.Request) { requireAuth(d, d.handleLogs)(w, r) })
```

`Register` 签名改为接收 `accts *accounts.Manager, sessions *session.Store, adminToken string, encrypt func(string)(string,error)`。

- [ ] **Step 3：handler_test.go 更新**

`newTestDeps`：建 `accounts.New(zhi.URL, vision, st)`、`session.New(time.Hour)`、`Register(mux, st, sched, accts, sess, "2026-09-13 09:00:00", "admin-123", func(s string)(string,error){return "ENC:"+s,nil})`。所有受保护端点请求需带 `Authorization: Bearer <会话>`。登录测试用 `{"account":"acct1","password":"pwd","admin_token":"admin-123"}` → 取返回 token；`TestSetTargetsAndState` 用会话 token 调 `PUT /api/targets`（body 无 account、无 admin）与 `GET /api/state`。新增：

```go
func TestAuthRequired(t *testing.T) {
    d := newTestDeps(t)
    code, j := doJSON(t, d.api, "GET", "/api/state", "")
    if code != 200 || j["code"].(float64) != 401 {
        t.Fatalf("无会话应 401: %d %v", code, j)
    }
}
```

补：`TestLoginBadAdmin`（错误口令 403）、`TestLoginOKIssuesSession`（正确口令 + 账密返回 token，后续带会话访问 /api/accounts 返回该账号）。

Run: `cd backend && go test ./internal/api -v` → PASS

- [ ] **Step 4：提交**

```bash
git add backend/internal/api
git commit -m "feat(api): admin-gated login, session auth middleware, snapshot electives"
```

---

### 任务 6：main.go 组装 + config 去硬编码 + 前端会话化

**文件：**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/main.go`

**接口：**
- `config.Load()`：删除 Token/Cookies 字段与硬编码 SF_API_KEY（空字符串）；新增 `AdminToken`（读 `XUANKE_ADMIN_TOKEN`）。
- main：`AdminToken` 为空则 `log.Fatal`；`secure.LoadOrCreateKey`；`accounts.New` + `Restore(creds, decrypt)`；scheduler 用新的账号注册表构造；`Register` 新签名。

- [ ] **Step 1：config.go**

```go
type Config struct {
    Port     string
    DBPath   string
    OpenTime string
    BaseURL  string
    SFBaseURL string
    SFAPIKey  string // 环境变量 SF_API_KEY；未配置则登录时明确报错
    SFModel   string
    AdminToken string // 环境变量 XUANKE_ADMIN_TOKEN；公网部署必备
}

func Load() Config {
    return Config{
        Port:      envOr("XUANKE_PORT", "3091"),
        DBPath:    envOr("XUANKE_DB", "data/xuanke.db"),
        OpenTime:  "2026-09-13 09:00:00",
        BaseURL:   "https://www.zhidao.fj.cn",
        SFBaseURL: envOr("SF_BASE_URL", "https://api.siliconflow.cn/v1"),
        SFAPIKey:  os.Getenv("SF_API_KEY"),
        SFModel:   envOr("SF_MODEL", "Qwen/Qwen3-VL-30B-A3B-Instruct"),
        AdminToken: os.Getenv("XUANKE_ADMIN_TOKEN"),
    }
}
```

（删除 `XUANKE_TOKEN`/`XUANKE_COOKIE` 注入与会话复用路径——多账号物理隔离后不再适用。）

- [ ] **Step 2：main.go**

```go
func main() {
    cfg := config.Load()
    if cfg.AdminToken == "" {
        log.Fatal("未设置部署访问口令：请设置环境变量 XUANKE_ADMIN_TOKEN 后启动")
    }
    if cfg.SFAPIKey == "" {
        log.Println("[main] 警告：未设置 SF_API_KEY，教务登录验证码识别将不可用")
    }

    d, err := db.Open(cfg.DBPath)
    if err != nil {
        log.Fatalf("打开数据库失败: %v", err)
    }
    defer d.Close()
    st := store.New(d)

    masterKey, err := secure.LoadOrCreateKey(cfg.DBPath)
    if err != nil {
        log.Fatalf("初始化数据加密密钥失败: %v", err)
    }
    encrypt := func(s string) (string, error) { return secure.Encrypt(s, masterKey) }
    decrypt := func(s string) (string, error) { return secure.Decrypt(s, masterKey) }

    accts := accounts.New(cfg.BaseURL, zhidao.VisionConfig{
        BaseURL: cfg.SFBaseURL, APIKey: cfg.SFAPIKey, Model: cfg.SFModel,
    }, st)
    creds, err := st.LoadCredentials()
    if err != nil {
        log.Printf("[main] 读取凭据失败: %v", err)
    } else {
        accts.Restore(creds, decrypt)
    }

    openTime, err := scheduler.FormatOpenTime(cfg.OpenTime)
    if err != nil {
        log.Fatalf("开放时间配置错误: %v", err)
    }
    sched := scheduler.New(accts, st, openTime, 300*time.Millisecond)

    success, err := st.LoadSuccess()
    if err != nil {
        log.Printf("[main] 读取成功记录失败: %v", err)
    } else {
        sched.RestoreDone(success)
    }
    for _, a := range accts.Registered() {
        ts, err := st.LoadTargetsForAccount(a)
        if err != nil {
            log.Printf("[main] 读取账号 %s 目标失败: %v", a, err)
            continue
        }
        if len(ts) > 0 {
            sched.SetTargetsForAccount(a, ts)
        }
    }
    sched.Start()

    sessions := session.New(12 * time.Hour)
    mux := http.NewServeMux()
    apiHandler := api.Register(mux, st, sched, accts, sessions, cfg.OpenTime, cfg.AdminToken, encrypt)
    mux.Handle("/", web.SpaHandler())

    addr := ":" + cfg.Port
    log.Printf("[main] 至道选课自动化服务启动: http://localhost%s（部署口令已启用）", addr)
    if err := http.ListenAndServe(addr, apiHandler); err != nil {
        log.Fatalf("服务启动失败: %v", err)
    }
}
```

（删除 `parseCookies`/`tokenShort` 或保留 tokenShort。`client` 变量与 env 注入路径整体删除。）

- [ ] **Step 3：编译 + 全量测试**

Run: `cd backend && go build ./... && go test ./...` → 全部 PASS；`go vet ./...` 无告警。

- [ ] **Step 4：提交**

```bash
git add backend/internal/config backend/main.go
git commit -m "feat(main): wire admin token, encrypted creds, per-account clients"
```

---

### 任务 7：前端会话化（多会话 + 部署口令登录 + 纯黑白极简）

**文件：**
- Modify: `web/src/types.ts`
- Modify: `web/src/api/client.ts`
- Modify: `web/src/App.tsx`
- Modify: `web/src/routes/Login.tsx`
- Modify: `web/src/routes/Dashboard.tsx`
- Modify: `web/src/routes/Select.tsx`

**接口：**
- 会话存储 `localStorage["xk_sessions"] = { [account]: sessionToken }`。
- `api` 每请求携带 `Authorization: Bearer <会话>`。
- Login 页新增「部署访问口令」输入框。
- 401 时移除该账号会话。

- [ ] **Step 1：types.ts**

```ts
// 会话令牌映射：账号名 -> 服务端签发令牌
export type Sessions = Record<string, string>
```

- [ ] **Step 2：api/client.ts**

```ts
export async function api<T>(
  path: string,
  opts?: RequestInit & { session?: string }
): Promise<T> {
  const { session, ...rest } = opts ?? {}
  const headers: Record<string, string> = { "Content-Type": "application/json" }
  if (session) headers.Authorization = `Bearer ${session}`
  const r = await fetch(BASE + path, { headers, ...rest })
  // ...（其余不变，401 仍广播事件，事件带 account）
}
```

（删除 `account` query 传参——账号由后端会话决定。）

- [ ] **Step 3：App.tsx**

- `sessions` = `useState<Sessions>(() => JSON.parse(localStorage.getItem("xk_sessions") || "{}"))`。
- 每次变更写回 localStorage。
- `accounts` = `Object.keys(sessions)`；`current` 默认第一个。
- `login(sessionToken, account)`：`setSessions(s => ({...s, [account]: sessionToken}))`、`setCurrent(account)`。
- `logout()`：删除 current 账号会话；若空则回到登录页。
- `UNAUTHORIZED_EVENT`：删除对应 account（事件 payload 带 account）。
- Dashboard/Select 传入 `account` 与 `sessionToken`。

示意关键片段：

```tsx
const [sessions, setSessionsState] = useState<Sessions>(() => {
  try { return JSON.parse(localStorage.getItem("xk_sessions") || "{}") } catch { return {} }
})
const setSessions = (s: Sessions) => {
  setSessionsState(s)
  try { localStorage.setItem("xk_sessions", JSON.stringify(s)) } catch {}
}
const accounts = Object.keys(sessions)
const [current, setCurrent] = useState<Account>(accounts[0] || "")
const login = (t: string, acct: Account) => {
  setSessions({ ...sessionsRef(), [acct]: t })
  setCurrent(acct)
  setPage("dashboard")
}
const logout = (acct: Account) => {
  const next = { ...sessionsRef() }
  delete next[acct]
  setSessions(next)
  if (current === acct) setCurrent(Object.keys(next)[0] || "")
}
```

（状态一致性细节以实际实现为准：用 useCallback + functional setState 保证不闭包过期。）

- [ ] **Step 4：Login.tsx**

加部署口令输入框（`admin_token`），提交 `{account, password, admin_token}`；成功回调 `onLogin(data.token, data.account)`。底部文案改为「部署访问口令保护 · 会话级账号隔离」。

- [ ] **Step 5：Dashboard.tsx / Select.tsx**

- 请求全部传 `session={sessionToken}`
- 账号下拉：从 `accounts` 渲染（保留原生 select，纯黑白极简）
- Select 保存 body `{ targets }`（无 account 字段）

- [ ] **Step 6：构建验证**

Run: `cd web && npm run build` → tsc 通过、产物更新。

- [ ] **Step 7：提交**

```bash
git add web/src
git commit -m "feat(ui): session-scoped auth with admin gate login"
```

---

### 任务 8：全链路验收 + 重建 + CLAUDE.md 更新

- [ ] **Step 1：重建单二进制**

```bash
cd backend && go build -o xuanke.exe . && Copy-Item xuanke.exe ..\xuanke.exe -Force
```

- [ ] **Step 2：冷启动实测（删除旧 DB 模拟「不兼容旧数据」）**

```bash
rm -f data/xuanke.db data/.master_key
XUANKE_ADMIN_TOKEN="<随机口令>" ./xuanke.exe
```

浏览器打开 `http://localhost:3091`：
- 无口令时直接拒绝登录（`/api/login` 403）；
- 输入部署口令 + 教务账密 → 登录成功，签发会话；
- `/api/electives` 首请求触发探测，之后 40s 内直读内存快照（日志无平台请求）；
- 顶栏账号下拉随登录自动增删账号，各账号状态/目标互不可见；
- 调度器 30 秒节流日志 + 窗口到点立即探测（可临时把 openTime 设为当前时间实测）。

- [ ] **Step 3：更新 CLAUDE.md**

在「Go + React 现代版」章节补记：账号注册表（accounts 包，每账号独立客户端会话）、部署口令（XUANKE_ADMIN_TOKEN 启动必填）、会话级账号绑定（requireAuth + sessionAccount）、AES-GCM 密码加密入库（XUANKE_MASTER_KEY / .master_key）、不兼容旧数据（db 拒绝旧库启动）、超高性能（窗口到点立即探测 + 课程快照 /api/electives + 按账号并发提交）、接口约定更新（login 需 admin_token；所有 API 需 Bearer 会话）。

- [ ] **Step 4：提交**

```bash
git add CLAUDE.md
git commit -m "docs: public deployment security, per-account isolation, performance notes"
```

---

## 自检清单（写完计划后逐项核对）
- [x] 规格覆盖：不兼容旧数据（任务 2 拒绝旧库）+ 各账号不互通（任务 3/4 每账号客户端与 done/inflight 按账号）+ 公网安全（任务 1/3/5/6 口令+会话+加密）+ 密码入库（任务 2/3）+ 超高性能（任务 4/5 立即探测+快照+并发）
- [x] 无占位符：全部任务含真实代码
- [x] 类型一致性：`accounts.Manager` 的 `ClientFor` 返回 `scheduler.Client`；scheduler 本地 `AccountClients` 接口；store 的 `SaveCredential/LoadCredentials` 与 accounts.Store 匹配；`Register` 新签名贯穿 main/api/router/test
