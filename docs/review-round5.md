# 第 5 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/main/cmd）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI（ci.yml/release.yml）。
> 审查方式：两个 opus 子代理并行只读审查（后端 131 次工具调用、前端 129 次），全部发现定位到具体行号并经读码推演成立；修复按"先红灯后绿灯"TDD 或等价验证后独立 commit。
> 本轮结论：无 CRITICAL；后端 5 MAJOR + 5 MINOR + 2 INFO，前端 1 MODERATE(待实证) + 6 MINOR/INFO。

---

## 一、后端发现与修复状态（B5 系列）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **B5-01** | MAJOR | 自动重登失败后 `tokenValid` 永久滞留——用户手动登录成功也不清，`/state` 恒显"已失效·自动恢复中" | ✅ 新增 `Scheduler.MarkTokenValid(acct)`，`issueSession` 手动登录成功调用（清 tokenValid/reloginFail/relogging） |
| **B5-02** | MAJOR | A8 只修"先生成再落库"，落库循环仍逐条无事务——第 i 条失败时前 i-1 个"隐身码"滞留 | ✅ 新增 `store.CreateActivationCodes` 单事务批量落库，handler 改调它 |
| **B5-03** | MAJOR | admin 登录与学生登录共享同一 per-IP 限流桶——校舍 NAT 下学生爆登锁死管理员 | ⏳ 需路由层前置分流（admin 名独立高预算桶）。私有部署人工风险评估，列待办 |
| **B5-04** | MAJOR | 手动报名成功后进程在 HTTP 响应窗口崩溃 → 平台已报名而 success 未落库，重启自动链可能二次报名 | ⏳ 手动路径先进 `SaveSuccess` 再置 done。列待办（依赖平台语义兜底） |
| **B5-05** | MINOR | `lastProbe` 混用对齐时钟与本地时钟，探测节流周期恒定偏差 ~0.6s | ⏳ 统一 `nowAlignedLocked` 写入。列待办 |
| **B5-06** | MINOR | `clockOffset` 无合理性钳制，CDN 错误 Date 头可一次推到分钟级偏差 | ⏳ 加 `abs(offset)>5min` 丢弃。列待办 |
| **B5-07** | MINOR | refused 跨窗口永久保留；手动重选成功但未重设目标时 refused 卡死，自动引擎永久跳过 | ✅ `MarkDone` 成功分支同时 delete refused |
| **B5-08** | MINOR | `ConsumeActivationCode` 对已激活账号 INSERT OR IGNORE 静默跳过但仍扣次、"激活成功"假象 | ✅ 先查 `IsActivated`，已激活返回 false 且事务回滚不扣次 |
| **B5-09** | MINOR | `/api/electives` 快照过期瞬间 N 并发同时打穿到上游探测，无 single-flight | ⏳ 对 ProbeForAccount 加 per-account 节流/单飞。列待办 |
| **B5-10** | MINOR | `ProbeForAccount` 写全局 lastData，管理员穿透探测把全局帧污染为错年级快照 | ✅ 只写 acctData/acctDataAt，不动全局；lastProbe 仍更新 |
| **B5-12** | INFO | `/api/xxx` 不存在的路径回退 SPA index.html（200 text/html 语义错乱） | ⏳ 回退前加 `/api/` 前缀 → http.NotFound。列待办 |
| **B5-13** | INFO | 正确口令分支 300ms 延迟（A5）——实现正确、舍小取大，无需修改 | ✅ 已确认无需修改 |
| **B5-14** | INFO | 学生登录 500ms+ vs admin 300ms 量级差仍可枚举管理员名（可配置名缓解） | ⏳ 已知可配置缓解，默认不暴露，列待办 |

### 后端明确无问题区（子代理读码推演确认）
- session.Store 全量（票据绑定+单次消费+5min TTL+sweepLoop+RevokeAccount）
- secure AES-256-GCM、config loadDotEnv（保留 #）、db WAL+busy_timeout+SetMaxOpenConns(1)
- A2 refused 核心机制（refused_test.go 覆盖）、A4 maskKey 与既有测试不冲突、A5 等时本身、A8 随机 panic 策略
- zc 双通道、doRequest 不自动重登、reloginBackoff 封顶、prewarm/时钟对齐失败回退

## 二、前端与构建发现（F5 系列）

| 编号 | 严重度 | 问题 | 状态 |
|---|---|---|---|
| **F5-01** | MODERATE(待实证) | release.yml Windows CGO=1 构建在无 MinGW runner 上会整步失败（不假绿）；`XUANKE_MASTER_KEY` 经 os.Setenv 全局覆盖有变量提升语义陷阱 | ⏳ CI 真机实证；.env 覆盖属真实但需人工评估 |
| **F5-02** | MINOR | Toast id 用 Math.random 7 位短串，20 分钟高频提示碰撞概率 ~1/2000，React key 冲突误删 toast | ✅ `Toast.tsx` 改自增计数器 id |
| **F5-03** | INFO | 同一 toast 文案重叠可能隐藏点击目标，非状态损坏 | ⏳ 记录 |
| **F5-04** | MINOR | Toast 清理顺序与 aria 语义，Close 缺 aria 补充方向 | ⏳ D6 无障碍候选 |
| **F5-05** | MINOR | electives 轮询未并入 window_closed——窗口关闭后仍 10s 高频打 findElectivesData | ✅ `Select.tsx` electives refetchInterval 并入 window_closed → 30s |
| **F5-06** | MINOR | Select 与 Dashboard 共享 `["state",account,sessionToken]` key，挂载时数据互相引用 | ⏳ 语义已注释覆盖（M-9 C-2 n14），记录 |
| **F5-07** | MINOR | 401 广播无 account 参数时用会话令牌回传，数字学号账号名可能被误判 | ⏳ 建议服务端下发账号名，记录 |
| **F5-08** | INFO | Progress max=0 时 aria-valuemax=0/valuenow 超界 | ⏳ D6 候选 |
| **F5-09** | INFO | Admin config switch 缺键盘切换（Space） | ⏳ D6 候选 |

### 前端明确无问题区
client.ts 401 闭环、轮询降频（除 F5-05 已修）、登录链路（RSA/captcha/Vision/双通道）、授权边界、会话安全、重登、时钟对齐、窗口关闭语义、构建 CI（.env.example 纯占位、无 data 泄漏、go-version 无漂移）。

## 三、修复细节（本轮 7 处生产改动，均独立验证）

- **B5-01**：`MarkTokenValid` 新方法 + `issueSession` 调用；回归 `go test ./internal/api ./internal/scheduler` 绿
- **B5-02**：`CreateActivationCodes` 单事务；handler 改调；`go test ./internal/store ./internal/api` 绿
- **B5-07**：`MarkDone` 成功分支 delete refused
- **B5-08**：`ConsumeActivationCode` 事务内先查已激活、不扣次
- **B5-10**：`ProbeForAccount` 不再写全局 lastData/lastDataAt
- **F5-02**：Toast 自增 id（`tsc` + `npm run build` 绿）
- **F5-05**：Select electives 轮询并入 window_closed（`tsc` + `npm run build` 绿）

## 四、回归证据
- 后端：`go test -race ./...`（含 scheduler/api/store/zhidao）全绿，exit 0
- 前端：`npx tsc --noEmit` + `npm run build` 通过，dist 同步

## 五、待办与决策记录（不阻塞线上）
- 前端 D6-D8 无障碍/aria 系列 + F5-04/08/09 → 集中到无障碍收尾轮
- F5-01 CI 真机实证（MinGW 探测）；F5-07 服务端下发账号名
- B5-03/04/05/06/09/12/14 → 下一轮按优先级逐项 TDD

## 六、Commit 索引
- `020e89b` F5-02/F5-05
- `dfefde3` B5-01/02/07/08/10