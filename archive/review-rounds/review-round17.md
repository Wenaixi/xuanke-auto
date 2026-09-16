# 第 17 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（main.go/api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/cmd/web-embed）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI。
> 审查方式：两个 opus 权威子代理并行只读审查（后端 + 前端独立通道，均附"宁缺毋滥 + 文件行号 + 触发场景"模板），发现全部经主通道逐一读码推演定案；修复独立 commit。
> 本轮结论：**后端 1 项 MAJOR（F17-01 per-account 探测 N+1 并发封顶）+ 1 项 MINOR 加固（F17-04 .master_key 损坏文件校验）+ 前端 1 项 MINOR（F17-02 目标保存空集假清空根治）**；无 CRITICAL，无新安全漏洞。

---

## 一、后端发现与修复状态（第 17 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F17-01** | MAJOR | **per-account 探测 N+1 并发轰炸上游**：`probe()`（scheduler.go:745-749）为每个 `AccountsWithTargets()` 账号各起 goroutine 调 `ProbeForAccount`，**不受 F12-B2 的 probing 单飞守卫保护**（守卫只护 probe() 主体的全局 FindElectives）。触发链：临门/开窗期 `probeIntervalFor` 返回 `probeIntervalNear` **2s 周期**（B15 修正前误记 30s），N 账号部署开窗期每 2s 变 **N+1 并发 `findElectivesData`**（N=20~40 时 21~41 个/周期），与"访问过于频繁 1 分钟熔断"实证契约直接冲突。**修复**：新增 `probeSem` 结构化信号量（cap 4，New 初始化），per-account 探测入场前 Acquire / 结束 Release——并发峰值从 N 降到 4，跨批（2s 周期短于一批耗时）受同一信号量约束绝不叠加；全局 FindElectives 主体不受影响（per-account 全部入场后才执行）。`go build ./...` + scheduler 全量测试通过 |
| **F17-04** | MINOR | **`.master_key` 损坏文件无长度校验**：`LoadOrCreateKey`（crypto.go:24-26）只校验环境变量路径（hex + 32 字节），预生成密钥文件路径**原样返回任意字节**——损坏/截断/空文件会让 AES-256-GCM 初始化失败且 store 层无对账，加密凭据静默不可读；更糟的是新部署的 `WriteFile` 会用随机 32 字节**覆盖损坏文件**，数据库变成"不可解密"永久损坏。**修复**：文件路径同校 32 字节，不符显式报错拒绝带伤启动。**TDD**：`TestLoadOrCreateKeyRejectsTruncatedFile` 先红（返回短密钥无错）→ 后绿（显式报错） |
| 17-02 | 观察 | `RemoveFull`（scheduler.go:1475）文档说"外部手动或快照更新时解除满员标记"，但 grep 全后端仅定义无调用（死方法）。文档分叉无行为危害；删除需动契约面，按"精准修改"观察不修 |
| 17-03 | 观察 | `WindowClosed`（scheduler.go:791）语义与 17-06 同源：`!opened && len(Publishes)==0 && now.After(openTime)` 判定"窗口关闭"，但若平台对某账号年级返回空快照（该年级无发布），会误标"已关闭"并让 `probeIntervalFor` 降回 30s。行为可接受（降频不影响该账号开窗发现，临门分支仍优先），观察记录 |
| 17-05 | 观察 | `Deps.Decrypt` 死字段（handler.go:39-40）延续 B16-I1。纯死代码无触发；删除需动 constructor 签名与多处调用，观察不修 |
| 17-06 | 观察 | `probe()` 主体 `client.FindElectives()` 失败时 `probing` 复位 + `lastProbe` 记录，但 per-account goroutine 的 `ProbeForAccount` 错误被 `_, _ =` 静默丢弃——无日志、无状态暴露。失败频率已受 30s 节流闸门约束，静默失败仅影响该账号快照陈旧，非熔断风险；观察记录 |
| 17-07 | 观察 | 默认 `open_time` 过期（未配置/已过）时 `probeIntervalFor` 与 `submitIntervalFor` 的分支行为已在 B11-A1/B15-M2 收敛（零值降频 + 提交挂起），无新缺陷 |

## 二、前端发现与修复状态（第 17 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F17-02** | MINOR | **目标保存"空集假清空"最后拼图**：F16-01 的 `targetsUseCurrentPublishes` 用 `.every()` 校验 publish_id 归属——**空 targets 时 every 恒 true（恒过）**；且 `publishesMissing`（Select.tsx:284）是渲染期常量，防抖/flush 回调 400ms 后读的是旧闭包值。触发链：窗口关闭后 publishes 恒空 → 用户清空再添加目标（rev>0）→ 防抖回调里旧闭包 `publishesMissing=false` 不触发守卫 → `build()` 拿空 publishesRef 产出 `[]` → `.every` 恒 true 放行 → **PUT `[]` 抹掉后端既有目标**。这是 F7-01"仅 rev 驱动"契约的最后一道漏网。**修复**：防抖回调与 flushTargets **两处消费时刻**统一补"`selectedCount>0` 却构建出空集 = 假清空"守卫（selectedCount 只随用户改动所在渲染更新，只会偏保守绝不放过真实假清空）；并同步在消费时刻用最新 publishesRef 判"发布缺席 + 已有选中"（升级 F15-01 的渲染期判据）。tsc 通过 |
| R17-03 | 观察（误报澄清） | **壁纸滚动"CSS/JS transform 分叉"**：agent 判断 `.canvas-bg-img { transform: translateY(var(--bg-shift)) }` 的 CSS 路径与 App.tsx inline transform 分叉导致不滚动。**读码推演推翻**：App.tsx 每次滚动直接写 `img.style.transform = translateY(${shift}px)`，inline style 优先级恒高于 class，CSS 的 `var(--bg-shift, 0px)` 只是从未被覆盖时的兜底默认——**滚动机制实际工作正常**，且 `measure()` 用 `getBoundingClientRect` 量测真实渲染高度。观察不修 |
| R17-01 | 观察 | `logout` 闭包捕获 `current`（登录态引用）的防御性 MAJOR，实际被 client.ts 静默 catch 兜住（登出失败仅服务端 token 最多残留 12h，受威胁模型"刷新即自毁"约束）。观察记录 |
| R17-02 | 观察 | `useState("admin")` 初始 adminName 与后端重命名不一致，仅管理员改名后刷新一次闪屏（登录页账号占位），无行为危害。观察不修 |
| R17-05~07 | 观察 | INFO：轮询窗口状态降频闭包澄清（与 F9-05 同源正确）、useTickingCountdown 依赖无误、弹窗焦点无新问题 |

## 三、修复细节（本轮 1 后端 MAJOR + 1 后端 MINOR + 1 前端 MINOR，独立 commit）

- **F17-01**（commit 4ce4185）：scheduler.go 新增 `probeSem chan struct{}`（cap 4）+ `New` 初始化；probe() 的 per-account goroutine 入场 Acquire / defer Release——并发峰值 N→4，跨批同信号量约束
- **F17-04**（commit 89e6542）：crypto.go `LoadOrCreateKey` 文件路径补 32 字节校验；crypto_test.go 新增 `TestLoadOrCreateKeyRejectsTruncatedFile`（TDD 红灯→绿灯）
- **F17-02**（commit 2782d8a）：Select.tsx 防抖回调 + flushTargets 两处消费时刻补"selectedCount>0 却构建出空集 = 假清空"守卫；注释同步说明 every 空集恒真的防御位置

## 四、回归证据（提交时点 + 收尾复跑）

- `cd backend && go build ./... && go vet ./...` — 通过
- `cd backend && go test -race ./...` — 全包绿（api 18.5s / scheduler 18.3s / store 7.6s / secure 1.2s；config/db/runtime/session/zhidao cached）
- 专项：TestLoadOrCreateKey* / TestProbeInterval* / TestWindowOpenSubmitsWithoutProbeReset 全 PASS
- `cd web && npx tsc --noEmit` — 通过；`npx vite build` — 通过（407.73 kB / gzip 122.10 kB）

---

## 提交索引（本轮 3 个 commit）

```
4ce4185 fix(backend): 第17轮F17-01 MAJOR per-account探测N+1并发封顶（probeSem信号量cap4）
89e6542 fix(backend): 第17轮F17-04 .master_key损坏文件显式报错（32字节校验 TDD）
2782d8a fix(web): 第17轮F17-02目标保存空集假清空根治（消费时刻双闸守卫）
(review-round17.md + CLAUDE.md 沉淀)
```

## 下轮待办

- HTTP 侧探测接入全校单飞/节流（B14-I2/B16-M2 定案维持观察，最坏每 30s 周期至多多 1 次直打）
- B16-I1/F17-05 Decrypt 死字段删除（需动 constructor 签名，观察）
- F17-03/F17-06 观察项复核（WindowClosed 误标 / probe per-account 错误静默）
- 激活弹窗焦点陷阱（F6-02 长期遗留，Radix Dialog 迁移）
- Dashboard 消费 window_closed 显示"已关闭"（F13-B3 观察延续）
