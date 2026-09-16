# 第 6 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/main/cmd）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI（ci.yml/release.yml）。
> 审查方式：op 权威子代理并行只读审查（后端聚焦登录/限流/调度重登/客户端账密生命周期，前端聚焦会话广播/模态/轮询），全部发现定位到具体行号并经读码推演成立；修复按"先红灯后绿灯"TDD 或等价验证后独立 commit。
> 本轮结论：**2 个 CRITICAL/MAJOR 真实缺陷连根拔除**（B6-01 运行时登录后自动重登失效、B6-04 管理员穿透探测旁路全校节流闸门）；B6-05 限流 IP 伪造面加固（默认关闭）。CI 一周前部署的 Windows 构建"假绿"被实证复现并根治。

---

## 一、后端发现与修复状态（B6 系列）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **B6-01** | CRITICAL | **运行时账密登录后自动重登永远失效**——`zhidao.Login` 成功分支从不 `SetCredentials`，客户端内部账密只有 `Restore` 恢复路径才注入；线上每次 /api/login 账密登录后，token 一失效调度器 `ReloginIfNeeded` 永远报"未登录且无保存账密"→ 只能人工重新登录，黄金期窗口失效即全程停摆 | ✅ `Login` 成功后 `SetCredentials(account,password,token)` + 回归测试覆盖运行时路径 |
| **B6-04** | MAJOR | `ProbeForAccount` 写全局 `s.lastProbe`——管理员在开窗前点一次课程页（穿透探测）就把全校正规探测的 30s 节流闸门吞掉，等于探测节流被任意旁路，存在向教务平台高频轰炸风险（触发平台熔断→课程拉空） | ✅ `ProbeForAccount` 不再写 `lastProbe`；新增回归断言 `HasProbed()` 不被穿透探测置 true |
| **B6-05** | MAJOR | 登录/激活限流按 IP 分桶（5 次/分钟）但 `clientIP` 只取 `RemoteAddr`——反代后全校 NAT 共享一桶互锁；若简单改信 XFF 又会被公网伪造任意 IP 绕过/刷爆他人桶 | ✅ `XUANKE_TRUSTED_PROXY=on` + RemoteAddr 回环（真实本机反代）才信任 XFF 最右非空值，默认关闭；8 子场景回归 |
| **B6-02** | MINOR | `ConsumeActivationCode` 并发超额激活竞态——多请求同时消费同一码时，事务内 `INSERT OR IGNORE` 对已激活账号"静默跳过但仍扣次"之外，还有多账号并发抢占同一激活码的宽限窗口 | ⏳ 需要 `UNIQUE(account)` 数据库约束或 DELETE-based 消费；**决策：B5-08 已修的"先查已激活"已覆盖同账号重复激活主路径，并发跨账号抢占单码属人为分发过失，列待办** |
| **B6-03** | MINOR | admin 登录与学生共享同一 per-IP 限流桶（B5-03 追踪）——学校 NAT 下学生爆登锁死管理员 | ✅ 已决策**不在 Scheduler 层修复**：admin 名独立高预算桶属路由层分流，私有部署人工风险评估，依赖 WAF/Nginx 层 admin 限流 |
| **B6-07** | INFO | `/api/xxx` 不存在的路径回退 SPA index.html（200 text/html 语义错乱）——前端 fetch 拿到 200 HTML 解析 JSON 报错，掩盖真实 404 | ⏳ 回退前加 `/api/` 前缀 → `http.NotFound`。列待办 |
| **B6-08** | INFO | `clockOffset` 无合理性钳制（B5-06 追踪）——CDN 错误 Date 头可一次推到分钟级偏差 | ✅ 已决策**不修改**：现有 `syncFailStreak≥3 → offset=0` 回退已足够，钳制 5min 阈值收益有限 |
| **B6-09** | MINOR | `/api/electives` 快照过期瞬间 N 并发同时打穿到上游探测无 single-flight（B5-09 追踪） | ⏳ 对 `ProbeForAccount` 加 per-account 节流/单飞。列待办 |
| **B6-10** | MINOR | 手动报名成功后进程在 HTTP 响应窗口崩溃 → 平台已报名而 success 未落库（B5-04 追踪） | ✅ 已决策**不修改持久化顺序**：`SaveSuccess` 幂等 + 崩溃窗口以分钟计，平台语义兜底（重复报名报"已选该课程"）已足够 |

### 后端明确无问题区（读码推演确认）
- router.go 已分离 login/activate 双限流桶（TestLoginActivateSeparateBuckets 覆盖）
- requireAuth/requireAdminSession、issueSession + MarkTokenValid、session 12h TTL 与单票单发
- `maybeRelogin` 的 reloginMu→s.mu 锁序、reloginBackoff 封顶、login/relogin 全局每分钟 gate
- syncServerTime RTT/2 中点近似、probeIntervalFor 窗口关闭降频
- zhidao 双通道（idToken + Cookie）、doRequest 不自动重登、sharedTransport 预热
- accounts.Manager ensure/LoginByPassword/Restore 三条建链路径的账密 binder 归属

## 二、前端与构建发现（F6 系列）

| 编号 | 严重度 | 问题 | 状态 |
|---|---|---|---|
| **F6-01** | CRITICAL(CI 实证) | Windows PowerShell `CGO_ENABLED=1 go build` 前缀赋值是 **CommandNotFoundException**（exit 127，非终止）——go build 漏执行或 CGO=0 静默降级，CI 假绿 + release Windows 版实际没有内嵌 ddddocr | ✅ `$env:CGO_ENABLED='1'` 前缀；ci.yml（go build Windows 产物）+ release.yml（两个 Windows build step）全部修正 |
| **F6-02** | MINOR | 登录页激活码模态框用裸 `div` 掩膜，缺 `role=dialog/aria-modal/aria-labelledby/esc 关闭`（Radix Dialog 原语已存在未使用） | ⏳ 无障碍候选，列待办 |
| **F6-03** | MINOR | Admin「配置」页 open_time 清空保存后，`/state.open_time` 为 0 → 前端口径混乱；ConfigTab 重新获取配置不回填已设值 | ⏳ 列待办 |
| **F6-04** | MINOR | 无目标账号（no client）会话 `/electives` 时快照回退到全局 lastData，年级可能错配（B 系列的 fallback 语义，前端不可见） | ⏳ 列待办 |
| **F6-05** | INFO | 401 广播无 account 参数时用会话令牌回传，数字学号可能被误判账号名（B5-07 追踪） | ✅ 已决策**不可行**：服务端下发账号名不值得（碰撞概率可忽略），仅在登录响应已含 account 分支处理 |
| **F6-06** | INFO | `MASTER_KEY` 经 `loadDotEnv` os.Setenv 回填，有无"运行时覆盖竞态" | ✅ 已决策**不修改**：`loadDotEnv` "仅回填空值"（真实环境变量恒优先）+ main 启动单线程时序调用一次，无竞态；补注释收尾 |

### 前端明确无问题区
- client.ts 401 闭环（UNAUTHORIZED_EVENT → 清会话 → 回登录）、Authorization 头与消息总线会话绑定
- 登录链路（RSA/captcha/Vision/双通道）、校园网 NAT 下轮询降频语义
- Select 大厅 modal 为 Radix Dialog 展示层（功能完好，仅无障碍残缺，F6-02）

## 三、修复细节（本轮 6 处生产改动，均独立验证）

- **B6-01**：`zhidao.Login` 成功分支增加 `SetCredentials(account,password,token)`（11919a0）+ 回归测试 `TestReloginIfNeeded` 覆盖运行时登录→立即重登可用、restore 路径保持原语义（ab78087）。`go test -count=1 ./internal/zhidao ./internal/api` 绿
- **B6-04**：`ProbeForAccount` 移除 `s.lastProbe` 写入（只归 probe()/ProbeNow 管理全校节流闸门）（d8e216b）；回归断言 `TestCheckClassSelectable` 中 `HasProbed()` 不被穿透探测置 true。`go test -race ./internal/scheduler` 绿
- **B6-05**：`clientIP` 可信反代 IP 透传（XUANKE_TRUSTED_PROXY=on 且 RemoteAddr 回环才信 XFF 最右非空值）（c74e7a8）；回归 `TestClientIPTrustedProxy` 8 子场景。`go test -count=1 ./internal/api` 绿
- **F6-01**：`$env:CGO_ENABLED='1'` 前缀语法（48a796f），ci.yml + release.yml 三个 Windows step 全部修正。pwsh 实证：`VAR=x cmd` 为 CommandNotFoundException 假绿根因
- **F6-06**：`config.go` loadDotEnv 补"仅回填空值"注释收尾（49b8016），机制确认无需改代码

## 四、本轮决策记录（明确不做）

| 决策 | 理由 |
|---|---|
| B6-03 不在代码层加 admin 独立桶 | 路由层分流属部署拓扑职责，私有部署人手稀缺，WAF/Nginx 限流更合适 |
| B6-08 不加 clockOffset 钳制 | syncFailStreak≥3→0 回退已是够用保护，钳制 5min 阈值额外收益≈0 |
| B6-10 不改手动 select 持久化顺序 | SaveSuccess 幂等，崩溃窗口分钟级，平台"已选该课程"兜底 |
| F6-05 服务端不下发账号名 | 碰撞概率可忽略，1 行收益不值得 3 行协议修改 |
| F6-06 不改 loadDotEnv | "仅回填空值"语义本就无运行时竞态 |

## 五、回归证据（提交时点通过，收尾前复跑全量）

- `go build ./... && go vet ./internal/api/` — 通过
- `go test -count=1 ./internal/api/` — 9.457s 通过（含限流/admin/激活/授权全量）
- `go test -race ./internal/scheduler ./internal/zhidao` — 通过（B6-01 双路径回归 + B6-04 节流断言）
- `cd web && npx tsc --noEmit && npm run build` — 通过（c74e7a8 收尾阶段复跑确认）

---

## 提交索引（本轮 6 个独立 commit）

```
48a796f fix(ci): 第6轮F6-01 pwsh前缀赋值语法修复
d8e216b fix(review): 第6轮B6-04 ProbeForAccount不再写lastProbe
11919a0 fix(review): 第6轮B6-01 登录成功即写客户端账密
ab78087 test(review): 第6轮B6-01回归测试
49b8016 docs(review): 第6轮F6-06 loadDotEnv注释收尾
c74e7a8 fix(review): 第6轮B6-05 clientIP可信反代IP透传
```