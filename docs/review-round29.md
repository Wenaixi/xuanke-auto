# 第 29 轮全模块审查记录（2026-09-15）

> 审查范围：backend（api/scheduler/accounts/zhidao/store/session/runtime/config/secure/db/main）
> + web 全部模块。两个只读子代理并行产出发现，主 gate 逐条核实（读源码 + 推演
> 真实触发路径）。本文件先写前端已确认部分（后端报告到达后追加）。

## 前端（2 项，确认修复）

### M29-01（MINOR）手动报名/退选在飞操作改 Set 按课程独立跟踪——单值被并发不同课程互相覆盖
**缺陷**（review29-frontend）：Select.tsx 的 `actionLoading` 是单值 `useState<number | null>`
（47 行），报名与退选共用，F26-03 幂等守卫 `if (actionLoading === c.id) return` 只在
"同一课程双击"场景有效。真实触发链：课程 A 报名在飞 → 用户点课程 B，
`setActionLoading(B)` 覆盖 A 的标记 → A 的 finally `setActionLoading(null)` 把 B 的在飞态
一并抹掉（单值无法区分归属）→ B 按钮恢复可点 → 用户再点 B 发第三发 → 后端
TryAcquireSubmit 拒绝 → finally invalidateQueries 照跑 → **假失败 toast 在同一课程维度
之外复发**（正是 F26-03 想消除的缺陷换一种课程组合重演）。
**修复**：`actionLoading` 改 `ReadonlySet<number>`；守卫 `has(c.id)`；置位 `new Set(prev).add(c.id)`
（函数式）；finally `new Set(prev).delete(c.id)` 只清自己的 id（报名/退选共享同一 Set，
互不覆盖）；按钮 disabled/文案/Esc 守卫/F21-03 确认按钮 disabled 全部同步 `has()`。
**验证**：`npm run build`（tsc -b + vite）全绿。commit `b79991a`。

### M29-02（MINOR）登出/401 吊销未重置 page 视图态——重登后跳过 Dashboard 直接掉进选课大厅
**缺陷**（主 gate 复核自 review29-frontend 说服链）：App 组件永挂载（Login 只是条件渲染
分支），`page` 不随 sessions 清空而重置。主动登出路径：学生在选课大厅（page="select"）
点登出 → 重新登录 → page 残留 "select" → 渲染分支跳过 Dashboard 直接掉进选课大厅
（与 F15-07 targetAccount 同族，D4"回登录页"契约被凿穿）。401 被动吊销路径同源：
唯一账号在选课大厅被吊销 → account-reselect effect `setCurrent("")` 回登录页 → 重新登录
后 page 仍未复位。F15-07 只清了 targetAccount，page 是同类视图态遗漏。
**修复**：两处复位——logout() 分支 `setPage("dashboard")`；account-reselect effect
`accounts.length === 0` 分支同样 `setPage("dashboard")`（吊销不经 logout，两路对称）。
**验证**：`npm run build` 全绿。commit `1a014d3` + 补丁 `2087ebf`。

## 可疑待核裁决（review29-frontend）
- **① logout 不重置 page → 确认为真实缺陷，M29-02 修复**（上文）。
- **② tsconfig.app.json 缺 `strict` → 维持观察**（S27-02 延续）：系统性类型安全隐患，
  长期维持观察；开 strict 是全量工作（当前代码必有解构可选链/隐式 any 等报错），
  且已有 tsc -b 兜底类型基础错误，非本轮可顺手落地。
- **③ 守卫拦截吞改动与"下次进入恢复"注释语义落差 → 定不修**：flushTargets/防抖回调
  置 `dirtyRef=true` 的脏块不持久化，卸载即丢（handleBack 循环 3 轮后 onDone）。
  推演：守卫拦下的是"发布缺席/错位 = 假清空"（F15/F16/F17 链刻意安全方向）——
  发布缺席属平台过渡态（开窗瞬间清空，下一轮防抖/下次改动正常落库）；publishes 恒空
  （窗口关闭）时目标本无实际提交意义，"绝不强行假清空"优先于"尽量保存"（F21-01
  注释明示）。无真实缺陷。

## 观察项（本轮追加/延续）
- tsconfig 缺 strict（S27-02/S29-② 延续）。
- 历轮观察项全表延续（下节已核无缺陷清单后端部分到达后合并列出）。

## 已核无缺陷（前端，主 gate 复核）
- 目标保存串行化链：savingRef/dirtyRef/rev 驱动 + flushTargets/handleBack 补发收敛
  （F21-01/F26-01 时序读码复验，pendingSaving 判据含在飞 PUT 与退避 timer 双闸）。
- 假清空守卫链：F15-01→F16-01→F17-02 消费时刻判据三处守卫、publishes 缺席/联查空集/
  publish_id 漂移三路分类置脏，无绕过路径。
- 回归回显 effect：F19-01 幽灵 publish_id 过滤 + rev>0 短路 + prev 有内容即返回。
- 401 吊销链：快照式落盘、session 反查归属、F25-01 管理员自我吊销清代理态齐备。

## 回归
- frontend：`npm run build`（tsc -b + vite build）通过（三提交均验证）。
- backend：待 review29-backend 报告到达后补全量回归。

（后端部分待补）