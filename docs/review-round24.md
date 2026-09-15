# 第 24 轮全模块审查记录（2026-09-15）

> 审查范围：backend（api/scheduler/accounts/zhidao/session/store/runtime/config/db/main）
> + web 全部模块。两个只读 opus 子代理并行产出发现，主 gate 逐条核实（读源码 + 推演
> 真实触发路径），确认后端 1 项 MINOR（核实为真并 TDD 修复），前端 1 项疑似直击为
> 误报（前端纯 SPA 无任何 URL 导航能力——无路由无 history API，URL 参数残留场景
> 不可达，已逐文件 grep 铁证核实）。修复走 TDD（先红灯后绿灯）后独立中文 commit。

## 后端（1 项，确认修复）

### B24-01（MINOR）登录失败清理误删"此前持有效 token 的已注册客户端"——自动抢课静默停摆
**缺陷**：`LoginByPassword` 先 `m.ensure(acct)`（写入 clients+order）再 `c.Login`，失败
路径无条件摘除该账号客户端（B23-02 修复）。但该修复没有区分"本次新建的空壳"与"此前
已持有效 token 的工作客户端"——只要这次 `c.Login(acct, password)` 失败（密码输错 /
平台瞬时限流拒绝），即使注册表里躺着的是 token 有效、正在正常工作的客户端，也会被
`delete(m.clients, acct)` + 从 `order` 摘除。
**触发路径**：
1. 账号已登录且客户端持有效 token、已设目标（调度器正按 `acctTargets` 自动抢课），
   数据库 `credentials` 行保留该 token；
2. 浏览器会话过期（12h TTL）或主动重登，走 `handleLogin → LoginByPassword`
   （handler.go:124）——该路径对已注册账号返回 `ensure` 的既有客户端，本次密码
   输错/平台瞬时拒绝 → 失败；
3. B23-02 清理把该**有效客户端**摘除：`submitAll` 遍历时 `ClientFor(acct)` 返回不存在
   → `continue`（scheduler.go:1138）→ 该账号自动抢课静默停止；`handleElectives` 对
   目标账号 `ElectivesSnapshotFor` 返回 `(nil,false)` 后走 `ProbeForAccount`
   （scheduler.go:658-660）→ 报"账号未登录或不存在"，选课大厅持续报错；
4. 恢复**只能**靠该账号下次登录成功（重新 `ensure`+Login），无任何自动自愈；期间
   无管理员可见错误（自动抢课停摆的唯一征兆是要到开窗才暴露）。
**影响边界**：仅限该账号自身；不破坏持久化数据（`credentials` 行仍在，重启 `Restore`
可重建）。但"窗口黄金期手滑输错密码即让该账号全程停摆"是本系统刻意避免的状态
（自动重登/token 失效恢复都围绕"不因人为失误失联"），故值得修。
**修复**：失败清理前快照 `wasShell := c.Token() == ""`——纯新建空壳按 B23-02 摘除；
已持有效 token 的既有客户端保留（旧 token 是否失效交给调度器现有失效检测+自动重登链，
远优于直接失联）。**TDD**：`TestLoginFailKeepsExistingValidClient` 修复前红（既有有效
客户端被误删）→ 修复后绿；对偶守卫 `TestLoginFailRemovesFreshShell` 空壳摘除契约
（B23-02）保持。**commit**：`见下方`。

## 前端（0 项）

本轮前端通道通读全 SPA（App/Login/Dashboard/Select/Admin + client/api/lib/ui/types +
vite/tsconfig/package），并对子代理提出的 F24-01 疑似缺陷（URL `?account=` 参数残留
→ 401 踢除后下一用户被自动接上他人数据）做了**主 gate 逐文件核实**：

- **判定为误报**。证据链：
  1. 前端**零路由能力**——无 react-router（package.json 无依赖）、无任何
     `history.pushState/replaceState`、无 `useSearchParams/URLSearchParams`、
     无 `window.location` 读取，全部 `?account=` 参数只由组件**内存 state**
     `account` prop 拼接进 API path（client.ts:88/102、Select.tsx:52/111/268），
     从不过 URL 传递；
  2. 登录成功直接 `onLogin` 切 App 层 `current` state，URL 无任何 `?account=`
     残留机会；登出/401 剔除后页面回 Login，URL 恒为干净的部署根路径；
  3. 即使访问者手动在地址栏粘贴过 `/select?account=xxx`，那是**纯静态 SPA**——
     Vite 单页入口（index.html → /src/main.tsx），每次刷新/直接访问都由服务端
     SPA 兜底返回 index.html，页面重新挂载后 App state 全空，渲染条件
     `sessionToken ? … : <Login/>` 直接落登录页，**任何 query 都不进入渲染逻辑**。
  4. 后端 `allowAccountOverride` 只认管理员会话 Bearer（`IsAdminToken`）——普通学生
     会话即使带 `?account=` 也被忽略，会话绑定账号才是唯一真相源。
- 结论：**URL 参数残留 → 下一用户串数据 场景在前端不可达**，F24-01 不成立。
- 与 B23-02 修复的关联法检：`TestLoginFailRemovesFreshShell` 对偶守卫确认
  B24-01 的 token 判别绝不放行"空壳不被摘除"的旧缺陷回归。

## 主 gate 核实结论
- **B24-01 确认为真实缺陷**（读源码 + 推演真实触发路径：会话过期重登 + 手滑输错
  密码 + 平台瞬时拒绝，任一路径即可触发；golden hour 即全程失联）。走 TDD 修复。
- 观察项沿用（24-01~24-10：task_log 无容量上限（纯网络失败期累积）、快照 TTL 读侧
  本地钟 vs 写入对齐钟偏差 ~640ms/40s 无实质影响、PUT config 空变更幂等 UI 不发包、
  maybeRelogin 退避读写自洽、gateWait Cond 等待有界 30s、WindowClosed 闩锁逐轮翻转
  自愈有界、spawnChain 捕获后删除单次调用、reloginResults 缓冲 8、emptyRunsFor 死
  代码/syncFailedWindow 写而不读/Decrypt 死字段/RemoveFull 死方法、HTTP 侧探测单飞）。
  维持观察不修。
- 已核无缺陷：WindowClosed 三判据/tick 提交守卫顺序/三条写回路径 ClientFor 复核/
  B23-01 满员分支 done 让位/B23-03 链顶 tokenValidForLocked/B19-02 只清 refused/
  B22-01 目标账号专属帧绝不回退/探测时间戳对齐钟/probeSem cap4/probing 单飞/时钟校准
  三防线（syncing 调用侧置位+失败退避+无客户端复位）/锁纪律 reloginMu→s.mu/手动路径
  只 MaybeRelogin 不先 MarkTokenValid/激活票据 ConsumeTicket 先于激活码校验/登录限流
  token bucket 按 IP/日志脱敏全链路。

## 回归
- backend：`go build ./... && go vet ./... && go test -race ./...` 全绿。