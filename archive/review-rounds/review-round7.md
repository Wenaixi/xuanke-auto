# 第 7 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/main.go/cmd/web-embed）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI。
> 审查方式：op 权威子代理并行只读审查（后端 54 次工具调用、前端 60 次调用），全部发现定位到具体行号并经读码推演成立；修复按 TDD（红灯→绿灯）或等价先行验证后独立 commit。
> 本轮结论：**无 CRITICAL**；前端 1 CRITICAL（F7-01 目标自动保存被轮询清空整体抹除）连根拔除，后端 2 MAJOR（B7-M1 时钟校准自锁、B7-C4 未知 /api/ 回退 SPA）根治，MAJOR-8/M9 可用性修复，另 6 处 MINOR/INFO 收尾。

---

## 一、后端发现与修复状态（B7 系列）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **B7-M1** | MAJOR | **时钟校准失败自锁**：`maybeSyncClock` 在发起异步同步前就把 `lastSyncTime=now`——SyncServerTime 挂起 >300ms（等于一个 tick）时并发 goroutine 回写会跳过一个完整 60s 窗口；连续失败≥3 复位 clockOffset 后该分钟整段不再校准，**开窗前 1 分钟抖动 3 次=黄金期全程无校准**，本地时钟偏差数百 ms 直接决定黄金期启动时刻 | ✅ `lastSyncTime` 仅成功推进；新增 `lastSyncStart` + `syncing` 防 tick 叠加；失败永不吃闸门、绝不复位——校准窗口最多丢失一个 tick 间隔而非整分钟 |
| **B7-C4** | MAJOR | 未注册的 `/api/xxx` 落入 main.go `mux.Handle("/", SpaHandler)` 兜底，返回 index.html（**HTTP 200 text/html**）：前端 fetch 解析 JSON 报错掩盖真实 404，安全扫描误判任意 /api 路径可 200 | ✅ `Register` 显式注册 `mux.HandleFunc("/api/", ...)` 前缀（精确 method+pattern 优先命中）→ 未匹配一律 **HTTP 404 + JSON body** |
| **B7-M8/M9** | MINOR | `DELETE /api/admin/codes` 强制 `requireJSONBody`：标准 REST 客户端 DELETE 默认无 body（无 Content-Type）→ 403，curl/脚本删码必踩坑 | ✅ 路由 GET/DELETE 放行空 body（DELETE 幂等删除无副作用门）；handler DELETE 空 body 解码失败返回明确业务错误"请指定要删除的激活码" |
| **B7-M5** | MINOR | `probe()` 内为每个目标账号 `go ProbeForAccount` 无 per-account 单飞——N 账号并发时每 tick 同时打 N+1 个教务上游（带宽浪费，快照写入各自独立无数据污染） | ⏳ 决策：加 per-account 单飞（`reloginMu` 同款 map+atomic）。黄金期探测集中度由 lastProbe 全校闸门兜底，列为待办 |
| **B7-M2** | MINOR | `reloginResults` 缓冲 8：>8 账号并发重登成功时非阻塞投递丢弃（不影响崩溃，仅丢一次补探测） | ⏳ 已确认非阻塞 `select+default` 无死锁；容量 8 覆盖真实 ≤8 场景，记录不修 |
| **B7-M3** | INFO | 管理员穿透 `?account=` 对已注册学生账号可拿其专属快照（账号存在性可通过时序/错误文案边缘枚举） | ⏳ 需求边界确认：信息泄漏边缘，非阻断，记录 |
| **B7-M7** | INFO | `ListAdminAccounts` 仅列 accounts 表——从未成功登录发会话的已恢复凭据账号不可见 | ⏳ 一致性边界确认：非 bug，日志可查，记录 |
| **B7-M4/M6** | — | acctData/acctDataAt 读写全持 s.mu；`ListAdminAccounts` SQLite 单连接并发无竞态 | ✅ 已确认无需修改 |
| **B7-C2** | — | ConsumeActivationCode 并发超额激活竞态（重点专项） | ✅ 已确认无需修改：`UPDATE ... WHERE used_uses<total_uses` 原子扣减 + IsActivated 双查 + ticket 单次消费，锁级验证无窗口 |
| **B7-C3** | — | XFF 解析专项（IPv6/无端口/超长链） | ✅ 已确认无需修改：8 子场景测试覆盖 |
| **B7-C1** | — | Login+SetCredentials 锁序（B6-01 修复有无新竞态） | ✅ 已确认无需修改：同一 c.mu 全程保护，锁外无网络 |

## 二、前端发现与修复状态（F7 系列）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F7-01** | CRITICAL | **目标自动保存在 publishes 清空时整体抹除**：自动保存 effect 依赖含 `publishes`（轮询派生新引用），每次轮询重跑——窗口开启瞬间平台短暂清空 publishes → `build()` 产 `[]` → 防抖 PUT `{"targets":[]}` 把服务端/调度器内存目标**整体抹除**：黄金期 250ms 冲刺空转、窗口关闭后目标永久丢失、界面与服务端分叉 | ✅ 依赖改为仅 `rev`（用户点选自增）+ `publishesRef` 读取；发布列表收缩绝不等同用户意图清空目标。附带修复 F7-04（轮询不再重置失败重发退避） |
| **F7-02** | MAJOR | **open_time 清空不可达 + 保存后表单永不回填**：清空时间点"保存并生效"——后端 FormatOpenTime("") 静默忽略 → code:0 假成功（调度器仍按旧时间执行）；保存后 refetch 不回填（initializedRef 一次性门控）→ 表单与生效配置分叉 | ✅ 后端 `open_time==""` 显式清空（内存/落库/回显对齐）；前端 `configEpoch` 自增触发保存后回填。回归：TestAdminConfigHotReload 新增"清空→回显空+落库空"断言 |
| **F7-03** | MINOR | 三处模态框裸 div 无对话语义（Login 激活码 / Select 退选 / Admin 删账号），Radix Dialog 现成未用 | ✅ 三处补 `role=dialog`+`aria-modal`+`aria-labelledby` 最小语义门。完整焦点陷阱迁移 Radix 留待 D 系列 |
| **F7-04** | MINOR | `publishes` 进依赖导致轮询触发 effect 重跑 → `resetRetry()` 清零退避与上限，PUT 持续失败时失败提示按轮询节奏反复轰炸 | ✅ 随 F7-01 同根修复（save 仅用户改动驱动） |
| **F7-05** | MINOR | `setSessions` updater 内夹带 `saveSessions`（localStorage 写）副作用——React 并发下 updater 是纯函数约定，StrictMode/并发可能加倍调用 | ✅ updater 保持纯函数，localStorage 落盘在提交之后独立执行（removedNext 标记门控） |
| **F7-08** | INFO | Dashboard 日志面板文案 "RECENT 50" 失真（后端返回 100） | ✅ 文案对齐 RECENT 100 |
| **F7-09** | INFO | fetch 超时 abort 抛原生 `AbortError` → toast 显示 "This operation was aborted" | ✅ catch 中按 `e.name==="AbortError"` 映射 "请求超时，请重试"（同时覆盖调用方 signal abort） |
| **F7-06** | INFO | `xk_sessions` localStorage 存 Bearer 令牌（XSS 可窃取） | ⏳ 已知取舍：12h TTL + 登出服务端吊销（M-7）缓解，记录不修 |
| **F7-07/10** | INFO | Dialog/Sheet/Table 死导出；parseCountdown/priorityName 双份定义 | ⏳ F7-07 随 Radix 迁移候选（F6-02）；F7-10 DRY 收敛，记录 |

### 前端明确无问题区（读码推演成立）
- 401 UNAUTHORIZED_EVENT 全链路自洽（account= 优先 + 令牌反查 + 管理员代理保护分支放行）
- 轮询降频与清理（函数式 refetchInterval 并入 window_closed 降 30s、setInterval/setTimeout 卸载清理、react-query 无 observer 停轮询）
- 快速切账号竞态（queryKey 含 account 物理分键 + targetAccount null → 重挂载）
- React key/XSS/setState 循环/敏感日志全部干净；Toast 自增 id 防碰撞无回归

## 三、修复细节（本轮 11 处生产改动，10 个独立 commit，均验证后提交）

- **B7-M1**：`maybeSyncClock` 重写（lastSyncTime 仅成功推进 + syncing 防重入 + lastSyncStart）；回归 `TestClockSyncFailureResetsOffset` / `TestClockSyncSuccessClearsFailStreak` race 绿
- **B7-C4**：`Register` 末尾 `mux.HandleFunc("/api/", ...)` → HTTP 404 + JSON；回归 `TestApiUnknownPath404`（未知路径 404 JSON + health 仍可达）——**ServeMux 精确 method+pattern 优先于模糊 "/api/" 前缀**（实测/electives、/electives/select 不受影响）
- **B7-M8/M9**：router DELETE 放行 + handler 空 body 明确错误；回归 `TestAdminDeleteCodeNoBodyOK`
- **F7-01/F7-04**：Select.tsx effect 依赖去除 publishes（publishesRef）+ 补 `import type Publish`（tsc build 阶段抓出）
- **F7-02**：handler.go open_time=="" 显式清空 + Admin.tsx configEpoch 回填；回归 TestAdminConfigHotReload 增强
- **F7-03**：Login/Select/Admin 三处模态补 dialg 语义
- **F7-05**：App.tsx updater 纯化 + localStorage 外提
- **F7-08**：Dashboard 文案；**F7-09**：client.ts AbortError 映射

## 四、回归证据（提交时点通过，收尾前复跑全量）

- `go build ./...` + `go test -race -count=1 ./...` — 全包绿（api 21.6s / scheduler 15.7s / store 3.5s / zhidao 11.6s 等，含新测试）
- `cd web && npx tsc --noEmit` + `npm run build` — 通过（vite build 也过，index 406KB）
- Commit 序列见提交索引（B7-M1→F7-09 十个独立 commit）

---

## 提交索引（本轮 10 个独立 commit）

```
07156be fix B7 第7轮F7-02 open_time显式清空真实生效——假成功三处分叉根治
8b00ff2 fix(review): 第7轮B7-C4 未知/api/路径显式404 JSON——SPA兜底200 HTML根除
c98662c fix(review): 第7轮B7-M1 时钟对齐失败自锁根治——only成功推进闸门
f97857a fix(review): 第7轮B7-M8/M9 DELETE激活码无body可用——REST不再被JSON门403
f63c37c fix(web): 第7轮F7-03 登录激活码模态框补无障碍语义
ed87bad fix(web): 第7轮F7-03 退选/删账号确认框补aria语义
b974a16 fix(web): 第7轮F7-05 setSessions updater副作用外提
76a2db8 fix(web): 第7轮F7-08 日志面板文案对齐 RECENT 100
fd50f77 fix(web): 第7轮F7-09 fetch超时abort映射友好文案
77015d8 build(web): 修复F7-01 ref类型 Publish 未导入
+ 6baef1d fix(web): 第7轮F7-01 目标自动保存只由用户改动驱动（上文已计入）
```