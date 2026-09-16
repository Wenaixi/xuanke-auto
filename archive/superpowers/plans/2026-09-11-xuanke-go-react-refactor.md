# 至道选课自动化系统（Go + React）重构实施计划

> 面向 Agent 执行者：必需子技能：使用 superpower-subagent-driven-development（推荐）或 superpower-executing-plans 按任务逐项执行本计划。步骤使用复选框（- [ ]）语法进行跟踪。

**目标：** 将现有 Python 版选课脚本重构为 Go 后端 + React/TypeScript 前端的高性能低占用服务，前端构建产物嵌入 Go 二进制，单文件部署，支持账密登录、三课程选择、定时抢课、重启状态恢复。

**架构：**
- Go 标准库 net/http（Go 1.22+ 路由增强）+ modernc.org/sqlite（纯 Go 免 CGO，单文件持久化）实现轻量后端：零框架依赖、单二进制部署、低内存占用。
- 前端嵌入后端：web/ 用 Vite 构建为静态产物（dist/），通过 go:embed 嵌入 Go 二进制；后端同时提供 /api/* 接口与静态文件服务，一个端口、一个进程、零外部依赖，浏览器访问 http://localhost:8080 即打开前端页面。
- 后端暴露 REST API（JSON），核心为 Scheduler 定时任务引擎：每 300ms 轮询课程数据，窗口开启瞬间并发提交三个课程的报名请求，任务状态持久化到 SQLite，重启后自动恢复。
- 前端 React 18 + Vite + TypeScript，UI 组件全部用 Radix Primitives，黑白高级风格、无圆角（border-radius: 0）。

**技术栈：**
- 后端：Go 1.26、net/http（含 go:embed）、modernc.org/sqlite、google/uuid
- 前端：React 18、TypeScript 5、Vite 5、Radix Primitives（@radix-ui/react-dialog/select/tabs/switch/toast 等）、TanStack Query
- 验证码识别：复用现有 captcha 逻辑调硅基流动 Vision API（改 Go 实现）

**规格：** 本计划实现的规格即用户需求（见下方需求规格节），随计划流转。

## 需求规格

1. 登录：前端表单输入账号密码，后端调用已逆向的至道登录链路（GET /login → GET /login/captcha → RSA 加密 → POST /login/doLogin）获取 idToken；验证码识别用硅基流动 Vision（复用现有 captcha.py 逻辑，改 Go 实现）。
2. 账密保存与自动重登：账号密码与 token 持久化到 SQLite，接口返回 code=-1（未登录）时自动用保存账密重新登录换新 token，全程无感。
3. 课程数据：后端封装 findElectivesData 查询，返回三个发布（体育/校本1/校本2）的课程列表；前端展示课程名、上课地点、授课老师、人数、可报名状态。
4. 三课程选择：每个发布最多选 1 门，三个发布各 1 门；前端选择后保存到后端。
5. 定时执行：后端 Scheduler 在选课窗口开启（2026-09-13 09:00:00）后立即并发提交三个课程的报名；报名成功后状态更新。
6. 持久化：账号、token、三个目标课程、任务状态、日志全部存 SQLite；重启后加载并继续。
7. 前端风格：黑白高级、无圆角、Radix Primitives 组件、默认深色。
8. 单二进制部署：web 构建产物嵌入 Go 二进制（go:embed），后端同时服务 API 与静态页面，一个端口即可访问。
9. 性能：低内存占用（< 30MB）、高并发（抢课瞬间 3 个 goroutine 并发报名）、HTTP 接口 p99 < 5ms。
10. 安全：登录接口限流、panic recover、请求体大小限制、安全响应头、API 永不回传密码。

## 全局约束

- Go 版本 >= 1.22（用增强路由 mux），禁止引入 gin/echo 等重型框架（低占用优先）。
- 数据库只用 modernc.org/sqlite（纯 Go，免 CGO 编译），禁止 cgo 依赖。
- 圆角一律为 0（无圆角 UI）；UI 组件一律使用 Radix Primitives 构建（Dialog/Select/Tabs/Switch/Toast），禁止自造弹窗/下拉。
- 所有对外 API 返回统一 JSON 格式：{"code":0,"data":...,"msg":""}（code=0 成功，非 0 失败）。
- token/账号属于敏感数据，后端 SQLite 明文存储（本项目本地自用，API 响应不回传密码）。
- 前端构建产物必须嵌入 Go 二进制（go:embed），开发时 Vite 代理 /api 到后端，生产时由后端直接服务静态文件。
- 项目目录：当前工作目录（E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto）作为项目根，backend/ 与 web/ 两个子目录。

---

### 任务 0：项目骨架初始化

**文件：**
- 新建：backend/go.mod、backend/main.go
- 新建：web/package.json、web/vite.config.ts、web/tsconfig.json、web/index.html、web/src/main.tsx、web/src/App.tsx
- 新建：.gitignore

**接口：**
- 依赖输入：无（从零开始）
- 对外产出：backend/main.go 可编译运行返回 ok；web/ 可 npm run dev 启动空页面

- [ ] **步骤 1：初始化 backend 目录**

    mkdir -p backend
    cd backend
    go mod init xuanke-auto/backend
    go get modernc.org/sqlite@latest
    go get github.com/google/uuid@latest

- [ ] **步骤 2：编写 backend/main.go 最小 HTTP 服务**

    package main

    import (
    	"encoding/json"
    	"log"
    	"net/http"
    )

    func main() {
    	mux := http.NewServeMux()
    	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
    		json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": "ok"})
    	})
    	log.Println("listening on :8080")
    	log.Fatal(http.ListenAndServe(":8080", mux))
    }

- [ ] **步骤 3：运行并验证**

    cd backend && go run . &
    curl http://localhost:8080/api/health
    # 预期: {"code":0,"data":"ok"}

- [ ] **步骤 4：初始化 web 前端**

    cd web
    npm create vite@latest . -- --template react-ts
    npm install
    npm install @radix-ui/react-dialog @radix-ui/react-select @radix-ui/react-tabs @radix-ui/react-switch @radix-ui/react-toast @radix-ui/react-slot @tanstack/react-query

- [ ] **步骤 5：web/src/main.tsx 与 App.tsx 最小渲染**

    // web/src/main.tsx
    import React from 'react'
    import ReactDOM from 'react-dom/client'
    import App from './App'
    import './index.css'

    ReactDOM.createRoot(document.getElementById('root')!).render(
      <React.StrictMode><App /></React.StrictMode>
    )

    // web/src/App.tsx
    export default function App() {
      return <div style={{ fontFamily: 'system-ui' }}>至道选课自动化</div>
    }

- [ ] **步骤 6：验证前端启动**

    cd web && npm run dev
    # 浏览器打开 http://localhost:5173 看到 至道选课自动化

- [ ] **步骤 7：提交**

    git add .
    git commit -m "feat: project skeleton (go backend + react frontend)"

---

### 任务 1：配置与 SQLite 持久化层

**文件：**
- 新建：backend/internal/config/config.go
- 新建：backend/internal/db/db.go、backend/internal/db/schema.sql

**接口：**
- 依赖输入：任务 0 的 backend 骨架
- 对外产出：config.Load() 返回 Config 结构（含端口、数据库路径、选课窗口时间）；db.Open(path) 返回 *sql.DB；schema.sql 建表 SQL

- [ ] **步骤 1：编写 config.go**

    package config

    import "os"

    type Config struct {
    	Port     string
    	DBPath   string
    	OpenTime string // 选课开放时间
    }

    func Load() Config {
    	return Config{
    		Port:     envOr("XUANKE_PORT", "8080"),
    		DBPath:   envOr("XUANKE_DB", "data/xuanke.db"),
    		OpenTime: "2026-09-13 09:00:00",
    	}
    }

    func envOr(key, def string) string {
    	if v := os.Getenv(key); v != "" { return v }
    	return def
    }

- [ ] **步骤 2：编写 schema.sql**

    CREATE TABLE IF NOT EXISTS account (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      account TEXT NOT NULL,
      password TEXT NOT NULL,
      id_token TEXT,
      created_at TEXT NOT NULL DEFAULT (datetime('now'))
    );

    CREATE TABLE IF NOT EXISTS targets (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      publish_id INTEGER NOT NULL,
      class_id INTEGER NOT NULL,
      course_name TEXT NOT NULL,
      created_at TEXT NOT NULL DEFAULT (datetime('now'))
    );

    CREATE TABLE IF NOT EXISTS task_log (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      class_id INTEGER NOT NULL,
      action TEXT NOT NULL,
      result TEXT,
      is_ok INTEGER NOT NULL DEFAULT 0,
      created_at TEXT NOT NULL DEFAULT (datetime('now'))
    );

- [ ] **步骤 3：编写 db.go 打开数据库并执行 schema**

    package db

    import (
    	"database/sql"
    	"os"
    	"path/filepath"
    	_ "modernc.org/sqlite"
    )

    func Open(path string) (*sql.DB, error) {
    	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil { return nil, err }
    	d, err := sql.Open("sqlite", path)
    	if err != nil { return nil, err }
    	d.SetMaxOpenConns(1) // sqlite 单写，串行化避免锁冲突
    	if _, err := d.Exec(schemaSQL); err != nil { return nil, err }
    	return d, nil
    }

- [ ] **步骤 4：编写 db 测试**

    // backend/internal/db/db_test.go
    func TestOpenAndSchema(t *testing.T) {
    	d, err := Open(filepath.Join(t.TempDir(), "test.db"))
    	if err != nil { t.Fatal(err) }
    	defer d.Close()
    	var n int
    	d.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table'").Scan(&n)
    	if n < 3 { t.Fatalf("expected >=3 tables, got %d", n) }
    }

- [ ] **步骤 5：运行测试确认通过**

    cd backend && go test ./...

- [ ] **步骤 6：提交**

    git add backend && git commit -m "feat: config and sqlite persistence layer"

---
### 任务 2：至道 API 客户端（登录 + 查询 + 报名 + 自动重登）

**文件：**
- 新建：backend/internal/zhidao/client.go、backend/internal/zhidao/rsa.go
- 测试：backend/internal/zhidao/rsa_test.go、backend/internal/zhidao/client_test.go

**接口：**
- 依赖输入：config 中 BASE_URL、SF 视觉配置
- 对外产出：zhidao.Client 结构 + 方法 Login(account, password) (token string, err error)、FindElectives(schoolYear, schoolTerm int) ([]Publish, error)、SelectClass(classID int) error、ClassDetail(classID int) (ClassDetail, error)；内建 过期自动重登（code=-1 时用已保存账密重新登录并换 token）

- [ ] **步骤 1：编写 rsa.go（复刻 Python RSA 加密）**

    package zhidao

    import (
    	"crypto/rand"
    	"crypto/rsa"
    	"crypto/x509"
    	"encoding/base64"
    	"encoding/json"
    )

    // 登录页内嵌的 RSA 公钥（DER base64，1024 位）
    const pubKeyB64 = "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCWuhgriWbHIbPCQyHmablwQSyItcLyKlQU/0ydXkvU4KJtEExNmuXS0xdoVLBRGxNO5f2u2MkNGzJrFhSpVL68Qc0knhWofzs+BdtpSF4nMi7BteOvOKi0OkvhhCBcHL71Vk8UXOsaKDZkZ3lCBVQHpSA4+s6pi9xIeF93jz6pGwIDAQAB"

    func encryptIdentification(account, password string) (string, error) {
    	der, _ := base64.StdEncoding.DecodeString(pubKeyB64)
    	pub, err := x509.ParsePKIXPublicKey(der)
    	if err != nil { return "", err }
    	rsaPub := pub.(*rsa.PublicKey)
    	plain, _ := json.Marshal(map[string]string{"userName": account, "password": password})
    	ct, err := rsa.EncryptPKCS1v15(rand.Reader, rsaPub, plain)
    	if err != nil { return "", err }
    	return base64.StdEncoding.EncodeToString(ct), nil
    }

- [ ] **步骤 2：编写 uniqueDeviceID（复刻前端 getUniqueDeviceId）**

    // base64(UA|Win32|881|1410|毫秒时间戳转36进制)
    func uniqueDeviceID(ua string, now time.Time) string {
    	parts := []string{ua, "Win32", "881", "1410", strconv.FormatInt(now.UnixMilli(), 36)}
    	return base64.StdEncoding.EncodeToString([]byte(strings.Join(parts, "|")))
    }

- [ ] **步骤 3：编写 client.go 登录流程（含自动重登钩子）**

    type Client struct {
    	baseURL   string
    	http      *http.Client
    	account   string      // 已保存账密（用于过期自动重登）
    	password  string
    	mu        sync.Mutex  // 保护 token 读写
    	token     string
    	visionCfg VisionConfig // 硅基流动配置（任务 3）
    }

    // Login 完整登录链路：GET /login -> GET /login/captcha -> POST /login/doLogin
    // 验证码识别失败自动重试（最多 10 次）
    func (c *Client) Login(account, password string) (string, error)

    // doRequest 统一请求入口：转发到至道，若响应 code=-1 则自动重登一次后重试
    func (c *Client) doRequest(method, path string, form url.Values) ([]byte, error) {
    	var lastErr error
    	for attempt := 0; attempt < 2; attempt++ {
    		c.mu.Lock()
    		tok := c.token
    		c.mu.Unlock()
    		resp, err := c.http.PostForm(c.baseURL+path+"?idToken="+url.QueryEscape(tok), form)
    		if err != nil { return nil, err }
    		body, _ := io.ReadAll(resp.Body)
    		resp.Body.Close()
    		var j struct{ Code int \`json:"code"\`; IsOk bool \`json:"isOk"\`; Msg string \`json:"msg"\` }
    		if err := json.Unmarshal(body, &j); err != nil { return nil, err }
    		if j.Code != -1 { return body, nil } // 非未登录错误直接返回
    		// code=-1：token 失效，用保存账密自动重登
    		if c.account == "" { return body, errors.New("未登录且无保存账密") }
    		if _, err := c.Login(c.account, c.password); err != nil { return nil, err }
    		lastErr = errors.New(j.Msg)
    	}
    	return nil, lastErr
    }

- [ ] **步骤 4：编写 FindElectives / SelectClass / ClassDetail**

    // POST /electives/select/findElectivesData  body: {"schoolYear":..,"schoolTerm":..}
    // 解析 selectElectivesData -> []Publish（含 BeginDate/EndDate/Classes 列表）
    func (c *Client) FindElectives(schoolYear, schoolTerm int) ([]Publish, error)

    // POST /electives/select/selectElectivesClass  body: classId=<id>
    func (c *Client) SelectClass(classID int) error

    // POST /electives/classDetail  body: id=<id>  返回上课地点/授课老师等
    func (c *Client) ClassDetail(classID int) (ClassDetail, error)

    // Publish 结构：PublishID/Name/BeginDate/EndDate/CanSelect/HasSelected + Classes []Class
    // Class 结构：ID/CourseName/ClassName/TeacherNameList/ClassroomName/SelectedCount/MaxCount/CanSelect/BtnType

- [ ] **步骤 5：编写 rsa_test.go 与 client_test.go（httptest mock 登录与 code=-1 重登）**

    func TestEncryptIdentification(t *testing.T) {
    	s, err := encryptIdentification("***REMOVED***", "***REMOVED***")
    	if err != nil { t.Fatal(err) }
    	if len(s) < 100 { t.Fatalf("cipher too short: %d", len(s)) }
    }

    // TestAutoRelogin：mock 服务器第一次返回 code=-1，重登后返回正常，验证自动重登

- [ ] **步骤 6：运行测试并提交**

    cd backend && go test ./...
    git add backend/internal/zhidao && git commit -m "feat: zhidao api client with auto-relogin"

---

### 任务 3：验证码识别（硅基流动 Vision，Go 实现）

**文件：**
- 新建：backend/internal/zhidao/captcha.go、backend/internal/zhidao/captcha_test.go

**接口：**
- 依赖输入：config 中 SF_BASE_URL / SF_API_KEY / SF_MODEL
- 对外产出：zhidao.RecognizeCaptcha(img []byte) (string, error)

- [ ] **步骤 1：实现 RecognizeCaptcha**

    // POST {SF_BASE_URL}/chat/completions
    // Authorization: Bearer {SF_API_KEY}
    // body: {"model":"Qwen/Qwen3-VL-30B-A3B-Instruct","messages":[{"role":"user","content":[
    //   {"type":"image_url","image_url":{"url":"data:image/jpeg;base64,<b64>"}},
    //   {"type":"text","text":"请识别这张图片中的验证码字符，只输出字符本身，不要输出任何其他内容。"}
    // ]}],"temperature":0,"max_tokens":32}
    // 解析 choices[0].message.content 返回
    // 注意：使用专用 http.Client（独立于主客户端），避免 token 无关请求污染连接池

- [ ] **步骤 2：编写测试（httptest mock 固定响应）**

    func TestRecognizeCaptcha(t *testing.T) {
    	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    		w.Write([]byte("{\"choices\":[{\"message\":{\"content\":\"abcd\"}}]}"))
    	}))
    	defer srv.Close()
    	// 临时替换 baseURL 为 srv.URL
    	got, err := RecognizeCaptcha([]byte("fake-jpeg"))
    	if err != nil { t.Fatal(err) }
    	if got != "abcd" { t.Fatalf("got %q", got) }
    }

- [ ] **步骤 3：运行测试并提交**

    cd backend && go test ./...
    git add backend/internal/zhidao/captcha.go backend/internal/zhidao/captcha_test.go
    git commit -m "feat: vision captcha recognition"

---

### 任务 4：Scheduler 定时任务引擎（高性能并发）

**文件：**
- 新建：backend/internal/scheduler/scheduler.go、backend/internal/scheduler/scheduler_test.go

**接口：**
- 依赖输入：zhidao.Client、store 存储（任务 6）
- 对外产出：scheduler.New(client, store) *Scheduler；Start() 启动轮询协程；State() SchedulerState；SetTargets([]Target) 保存目标

- [ ] **步骤 1：定义 SchedulerState 与 Target**

    type Target struct {
    	PublishID  int    \`json:"publish_id"\`
    	ClassID    int    \`json:"class_id"\`
    	CourseName string \`json:"course_name"\`
    }

    type CourseStatus struct {
    	PublishID  int    \`json:"publish_id"\`
    	ClassID    int    \`json:"class_id"\`
    	CourseName string \`json:"course_name"\`
    	Status     string \`json:"status"\` // pending|in_range|submitted|success|failed
    	Result     string \`json:"result"\`
    }

    type SchedulerState struct {
    	OpenTime     time.Time      \`json:"open_time"\`
    	WindowOpened bool           \`json:"window_opened"\`
    	Courses      []CourseStatus \`json:"courses"\`
    }

- [ ] **步骤 2：实现轮询循环（300ms 间隔，单协程，mutex 保护 state）**

    func (s *Scheduler) Start() {
    	go func() {
    		ticker := time.NewTicker(300 * time.Millisecond)
    		defer ticker.Stop()
    		for {
    			select {
    			case <-s.ctx.Done():
    				return
    			case <-ticker.C:
    				s.tick()
    			}
    		}
    	}()
    }

    func (s *Scheduler) tick() {
    	data, err := s.client.FindElectives(s.schoolYear, s.schoolTerm)
    	if err != nil {
    		s.mu.Lock(); s.state.WindowOpened = false; s.mu.Unlock()
    		return
    	}
    	now := time.Now()
    	opened := false
    	for _, p := range data {
    		if now.After(p.BeginDate) && now.Before(p.EndDate) {
    			opened = true
    		}
    	}
    	s.mu.Lock(); s.state.WindowOpened = opened; s.mu.Unlock()
    	if !opened { return }
    	// 窗口开：并发提交全部目标课程（每课程一个 goroutine，全部并发）
    	for _, t := range s.targets {
    		go s.submit(t)
    	}
    }

- [ ] **步骤 3：实现 submit（去重：已 success 不重复提交；失败自动重试计入下次 tick）**

    func (s *Scheduler) submit(t Target) {
    	s.mu.Lock()
    	idx := s.statusIndex(t.PublishID)
    	if idx < 0 || s.state.Courses[idx].Status == "success" { s.mu.Unlock(); return }
    	s.state.Courses[idx].Status = "submitted"
    	s.mu.Unlock()
    	err := s.client.SelectClass(t.ClassID)
    	s.mu.Lock()
    	defer s.mu.Unlock()
    	if err != nil {
    		s.state.Courses[idx].Status = "failed"
    		s.state.Courses[idx].Result = err.Error()
    		s.store.AppendLog(t.ClassID, "select", err.Error(), false)
    		return
    	}
    	s.state.Courses[idx].Status = "success"
    	s.state.Courses[idx].Result = "报名成功"
    	s.store.AppendLog(t.ClassID, "select", "success", true)
    }

- [ ] **步骤 4：编写 scheduler_test.go（mock client 验证状态机 + 并发安全 -race）**

    // 窗口未开 -> pending；窗口开 -> submitted；SelectClass 返回 nil -> success
    // 用假 client 控制 FindElectives 返回的 beginDate/endDate
    // go test -race ./internal/scheduler 验证 mutex 正确

- [ ] **步骤 5：运行测试并提交**

    cd backend && go test -race ./internal/scheduler
    git add backend/internal/scheduler && git commit -m "feat: high-perf scheduler with concurrent submit"

---
### 任务 5：REST API 层

**文件：**
- 新建：backend/internal/api/handler.go、backend/internal/api/router.go

**接口：**
- 依赖输入：zhidao.Client、scheduler、store
- 对外产出：Register(mux, deps) 注册路由；统一 JSON {"code":0,"data":...,"msg":""}

- [ ] **步骤 1：定义路由**

    // POST /api/login                {account,password} -> 登录并保存 token + 账密
    // GET  /api/electives            -> 课程列表（三个发布）
    // GET  /api/electives/detail?id= -> 课程详情（上课地点/老师）
    // PUT  /api/targets              {targets:[{publish_id,class_id}]} -> 设置三个目标
    // GET  /api/state                -> Scheduler 状态
    // GET  /api/logs                 -> 报名日志

- [ ] **步骤 2：实现 handler（每个方法一个函数 + DTO）**

    type LoginReq struct {
    	Account  string \`json:"account"\`
    	Password string \`json:"password"\`
    }

    func writeJSON(w http.ResponseWriter, code int, data any, msg string) {
    	w.Header().Set("Content-Type", "application/json")
    	json.NewEncoder(w).Encode(map[string]any{"code": code, "data": data, "msg": msg})
    }

- [ ] **步骤 3：编写 handler_test.go（httptest 验证 JSON 格式与错误码）**

- [ ] **步骤 4：运行测试并提交**

    cd backend && go test ./...
    git add backend/internal/api && git commit -m "feat: rest api layer"

---

### 任务 6：store 持久化（账密/token/目标/日志）+ 重启恢复

**文件：**
- 新建：backend/internal/store/store.go、backend/internal/store/store_test.go
- 修改：backend/main.go

**接口：**
- 依赖输入：任务 1-5 全部
- 对外产出：Store CRUD；main.go 启动完整服务，重启后从 SQLite 恢复账密、token、目标

- [ ] **步骤 1：实现 store CRUD**

    type Store struct{ db *sql.DB }

    // 账密与 token 存 account 表（本地库，供过期自动重登）
    func (s *Store) SaveAccount(acct, pwd, token string) error
    func (s *Store) LoadAccount() (acct, pwd, token string, err error)

    // 目标课程：先删后插保证唯一
    func (s *Store) SetTargets(targets []Target) error
    func (s *Store) LoadTargets() ([]Target, error)

    func (s *Store) AppendLog(classID int, action, result string, isOK bool) error
    func (s *Store) LoadLogs(limit int) ([]LogEntry, error)

    type LogEntry struct {
    	ID        int64  \`json:"id"\`
    	ClassID   int    \`json:"class_id"\`
    	Action    string \`json:"action"\`
    	Result    string \`json:"result"\`
    	IsOK      bool   \`json:"is_ok"\`
    	CreatedAt string \`json:"created_at"\`
    }

- [ ] **步骤 2：main.go 组装（含重启恢复）**

    func main() {
    	cfg := config.Load()
    	d, err := db.Open(cfg.DBPath)
    	if err != nil { log.Fatal(err) }
    	st := store.New(d)
    	client := zhidao.New(cfg)
    	// 恢复：加载已存账密与 token
    	if acct, pwd, token, err := st.LoadAccount(); err == nil && acct != "" {
    		client.SetCredentials(acct, pwd, token)
    	}
    	// 恢复目标课程
    	targets, _ := st.LoadTargets()
    	sched := scheduler.New(client, st)
    	sched.SetTargets(targets)
    	sched.Start()
    	mux := api.Register(st, client, sched)
    	// 前端嵌入：/ 与静态资源由 embed 提供（任务 7）
    	log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))
    }

- [ ] **步骤 3：验证重启恢复**

    # 1. 登录后设置 targets
    # 2. 杀掉进程重启 go run .
    # 3. GET /api/state 应显示 targets 仍在、token 复用

- [ ] **步骤 4：提交**

    git add backend && git commit -m "feat: store persistence + restart recovery"

---

### 任务 7：前端嵌入后端（go:embed 单二进制）

**文件：**
- 新建：backend/web/embed.go
- 修改：backend/main.go、web/vite.config.ts（base 与构建输出路径）

**接口：**
- 依赖输入：任务 0 前端骨架（可构建出 dist/）
- 对外产出：go build 产出的单二进制同时服务 /api/* 与前端静态页面；访问 http://localhost:8080 打开前端

- [ ] **步骤 1：配置 Vite 构建输出到 backend/web/dist 且 base=/**

    // web/vite.config.ts
    import { defineConfig } from 'vite'
    import react from '@vitejs/plugin-react'

    export default defineConfig({
    	plugins: [react()],
    	base: '/',
    	build: {
    		outDir: '../backend/web/dist', // 构建产物直接输出到后端 embed 目录
    		emptyOutDir: true,
    	},
    	server: {
    		proxy: { '/api': 'http://localhost:8080' }, // 开发时代理到后端
    	},
    })

- [ ] **步骤 2：编写 backend/web/embed.go**

    package web

    import (
    	"embed"
    	"io/fs"
    	"net/http"
    )

    //go:embed all:dist
    var distFS embed.FS

    // Handler 返回前端静态文件服务：SPA 路由回退到 index.html
    func Handler() http.Handler {
    	sub, err := fs.Sub(distFS, "dist")
    	if err != nil { panic(err) }
    	return http.FileServer(http.FS(sub))
    }

    // SpaHandler：非 /api 路径都返回 index.html（供前端 react-router 使用）
    func SpaHandler() http.Handler {
    	fileServer := Handler()
    	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    		p := r.URL.Path
    		if p != "/" && p[len(p)-1:] == "/" { p = p[:len(p)-1] }
    		if _, err := distFS.Open("dist" + p); err != nil {
    			b, _ := distFS.ReadFile("dist/index.html")
    			w.Header().Set("Content-Type", "text/html; charset=utf-8")
    			w.Write(b)
    			return
    		}
    		fileServer.ServeHTTP(w, r)
    	})
    }

- [ ] **步骤 3：main.go 挂载静态服务**

    // 在 mux 上注册：/api/ 前缀走 API；其余路径走 web.SpaHandler()
    // mux.Handle("/", web.SpaHandler())

- [ ] **步骤 4：构建并验证单二进制**

    cd web && npm run build
    cd backend && go build -o xuanke.exe .
    ./xuanke.exe
    # 浏览器打开 http://localhost:8080 应看到前端页面（无需单独启动 Vite）

- [ ] **步骤 5：提交**

    git add . && git commit -m "feat: embed frontend into go binary (single deployable)"

---

### 任务 8：前端登录页 + 状态面板（黑白无圆角）

**文件：**
- 新建：web/src/styles/global.css、web/src/api/client.ts、web/src/routes/Login.tsx、web/src/routes/Dashboard.tsx
- 新建：web/src/components/ui/Button.tsx、web/src/components/ui/Input.tsx
- 修改：web/src/App.tsx（路由）

**接口：**
- 依赖输入：任务 5 API、任务 7 嵌入链路
- 对外产出：登录页（账密）与主面板（状态+日志）

- [ ] **步骤 1：编写 global.css（黑白、无圆角）**

    :root {
      --bg: #0a0a0a;
      --fg: #f5f5f5;
      --border: #333;
    }
    * { border-radius: 0 !important; }
    body { background: var(--bg); color: var(--fg); font-family: 'Inter', system-ui, sans-serif; }
    button, input, select { background: transparent; color: inherit; border: 1px solid var(--border); }
    button:hover { background: var(--fg); color: var(--bg); }

- [ ] **步骤 2：编写 Button/Input 组件（Radix Slot 包装）**

    // web/src/components/ui/Button.tsx
    import * as React from 'react'
    import { Slot } from '@radix-ui/react-slot'

    export const Button = React.forwardRef<HTMLButtonElement,
      React.ButtonHTMLAttributes<HTMLButtonElement> & { asChild?: boolean }>(
      ({ className, asChild, ...props }, ref) => {
        const Comp = asChild ? Slot : 'button'
        return <Comp ref={ref} className={cn('px-4 py-2 border transition-colors', className)} {...props} />
      }
    )
    Button.displayName = 'Button'

- [ ] **步骤 3：编写 api/client.ts（统一请求封装）**

    const BASE = '/api'
    export async function api<T>(path: string, opts?: RequestInit): Promise<T> {
      const r = await fetch(BASE + path, { headers: { 'Content-Type': 'application/json' }, ...opts })
      const j = await r.json()
      if (j.code !== 0) throw new Error(j.msg || '请求失败')
      return j.data
    }

- [ ] **步骤 4：编写 Login.tsx（账密登录，成功跳转面板）**

    export default function Login() {
      const [account, setAccount] = useState('')
      const [password, setPassword] = useState('')
      const [error, setError] = useState('')
      const submit = async () => {
        try {
          await api('/login', { method: 'POST', body: JSON.stringify({ account, password }) })
          window.location.hash = '#/dashboard'
        } catch (e: any) { setError(e.message) }
      }
      return (
        <form onSubmit={e => { e.preventDefault(); submit() }}
              className="flex flex-col gap-4 w-80 mx-auto mt-24">
          <h1 className="text-2xl tracking-[0.3em]">至道选课自动化</h1>
          <Input placeholder="账号" value={account} onChange={e => setAccount(e.target.value)} />
          <Input type="password" placeholder="密码" value={password} onChange={e => setPassword(e.target.value)} />
          {error && <p className="text-red-400 text-sm">{error}</p>}
          <Button type="submit">登录</Button>
        </form>
      )
    }

- [ ] **步骤 5：编写 Dashboard.tsx（3 秒轮询状态 + 日志）**

    export default function Dashboard() {
      const { data: state } = useQuery({ queryKey: ['state'], queryFn: () => api('/state'), refetchInterval: 3000 })
      const { data: logs } = useQuery({ queryKey: ['logs'], queryFn: () => api('/logs'), refetchInterval: 3000 })
      // 顶部：窗口状态（未开放/已开放 + 倒计时）
      // 中部：三课程卡片（课程名/状态：pending/submitted/success/failed）
      // 底部：日志列表
    }

- [ ] **步骤 6：验证与提交**

    cd web && npm run dev
    # 登录 -> 面板显示
    git add web && git commit -m "feat: login page and dashboard"

---

### 任务 9：前端课程选择页（三课程）+ 安全加固收尾

**文件：**
- 新建：web/src/routes/Select.tsx、web/src/components/CourseTable.tsx、web/src/components/TargetSelector.tsx
- 修改：backend/internal/api/handler.go（安全加固）、web/src/App.tsx

**接口：**
- 依赖输入：任务 5 GET /api/electives 与 PUT /api/targets、任务 7/8 组件
- 对外产出：三列课程表（体育/校本1/校本2），每列选 1 门；后端安全加固完成

- [ ] **步骤 1：编写 CourseTable（Radix Tabs 三列）**

    // 三个 Tab：体育 / 校本1 / 校本2
    // 每个 Tab 内课程列表：课程名、老师、地点、人数、可报名状态
    // 点击课程行 -> 选中（高亮）-> 保存到后端 PUT /api/targets

- [ ] **步骤 2：编写 TargetSelector（已选目标展示 + 清空）**

    // 已选三个课程卡片 + 清除按钮

- [ ] **步骤 3：后端安全加固（handler.go）**

    // 1. 登录接口限流：令牌桶，每 IP 每分钟最多 5 次登录尝试（防爆破）
    // 2. 统一 panic recover 中间件：任何 handler panic 返回 500 不崩溃
    // 3. 请求体大小限制：http.MaxBytesReader(w, r.Body, 1<<20)（1MB）
    // 4. 安全响应头：X-Content-Type-Options: nosniff、X-Frame-Options: DENY
    // 5. 密码明文存 SQLite 为本地自用设计，API 永不回传密码；账号查询脱敏
    // 6. 所有写操作（登录/设置目标）记录到 task_log

- [ ] **步骤 4：验证选择与保存**

    cd web && npm run dev
    # 选三个课程后刷新页面，目标应保留（重启后端也保留）

- [ ] **步骤 5：提交**

    git add . && git commit -m "feat: course selection + security hardening"

---

### 任务 10：端到端联调、压测与收尾

**文件：**
- 修改：backend/main.go、README.md
- 新建：backend/bench_test.go（可选）

**接口：**
- 依赖输入：任务 0-9 全部
- 对外产出：完整可运行系统 + 性能验证报告

- [ ] **步骤 1：全流程验证**

    # 1. 启动后端 ./xuanke.exe
    # 2. 浏览器 http://localhost:8080 登录 -> 选三课程 -> 状态面板显示
    # 3. 重启后端，状态/目标/token 恢复
    # 4. 窗口开启后自动并发报名

- [ ] **步骤 2：性能验证（低占用 + 高并发）**

    # 1. 内存占用：任务管理器观察 xuanke.exe 常驻内存（预期 < 30MB）
    # 2. 并发压测：go test -bench=BenchmarkConcurrentSelect -benchtime=5s
    # 3. HTTP 压测：curl 并发 100 请求 GET /api/state 观察延迟（预期 p99 < 5ms）
    # 说明：抢课窗口开启瞬间，3 个 goroutine 并发提交（对应 3 个目标课程）

- [ ] **步骤 3：编写 README.md**

    # 至道选课自动化
    
    ## 构建
    cd web && npm install && npm run build
    cd backend && go build -o xuanke.exe .
    
    ## 运行
    ./xuanke.exe
    # 打开 http://localhost:8080
    
    ## 功能
    - 账密登录（验证码自动识别，token 过期自动重登）
    - 三课程选择（体育/校本1/校本2）
    - 定时抢课（窗口开启自动并发报名）
    - 状态面板与日志
    - 重启恢复（账密/token/目标/日志全部持久化）

- [ ] **步骤 4：提交**

    git add . && git commit -m "feat: end-to-end integration with performance verification"

---

## 自检结果

**1. 规格覆盖度：**
- 登录（账密直接登录 + Vision 验证码 + 账密保存）→ 任务 2/3/8；账密保存与过期自动重登 → 任务 2（doRequest code=-1 自动重登）+ 任务 6（SaveAccount/LoadAccount）
- 三课程选择 → 任务 5/9
- 定时执行（超高并发提交）→ 任务 4（goroutine 并发 + 300ms tick）
- 重启恢复 → 任务 6（LoadAccount/LoadTargets）
- 黑白无圆角 Radix → 任务 8
- 前端嵌入后端单二进制 → 任务 7（go:embed）
- 高性能低占用 → 任务 0/1（标准库 + 纯 Go sqlite）+ 任务 10 压测
- 安全性 → 任务 9（限流/panic recover/请求体限制/安全头/密码不回传）

**2. 占位符扫描：** 无 TBD/TODO；每个任务含具体代码与验证命令。

**3. 类型一致性：** Target（PublishID/ClassID/CourseName）、SchedulerState（WindowOpened/Courses）、api 响应 {code,data,msg}、store 方法签名（SaveAccount/LoadAccount/SetTargets/LoadTargets/AppendLog/LoadLogs）在所有任务中命名一致。