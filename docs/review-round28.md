# 第 28 轮全模块审查记录（2026-09-15）

> 审查范围：backend（api/scheduler/accounts/zhidao/store/session/runtime/config/secure/db/main）
> + web 全部模块。两个只读 opus 子代理并行产出发现，主 gate 逐条核实（读源码 + 推演
> 真实触发路径），确认后端 1 项 MINOR + 前端 1 项 MINOR（均核实为真并 TDD 修复）。
> 修复走 TDD（先红灯后绿灯）后独立中文 commit。

## 后端（1 项，确认修复，commit `f546304`）

### B28-01（MINOR）无目标账号专属帧过期中间态回退错年级全局帧——年级串线残留
**缺陷**（review28-backend 可疑待核 1 实证）：`ElectivesSnapshotFor` 的回退链对无目标
账号（仅浏览）的"专属帧存在但已过期"中间态存在漏洞：高二 browse 账号 A 曾由
`ProbeForAccount("A")` 留下专属帧，之后 A 无目标、probe() 的 per-account 遍历不覆盖它，
40s 后专属帧过期；此时全局 lastData 恰被其他账号（首个注册账号 m.order[0] 高三）刷新为
新鲜帧 → 旧实现回退这份**错年级**全局帧且返回 ok=true → `handleElectives` 不触发
`ProbeForAccount("A")` → A 持续看到高三年级课程直到全局帧再次过期（循环可一直持续）。
B22-01 标称"无目标账号回退全局帧（快、无网络开销）"只对**从未有过专属帧**的纯浏览成立；
对曾有过专属帧但已过期的中间态，回退并不比刷新快、且数据是错年级。
**修复**：专属帧存在但过期 → 返回 `(nil,false)` 触发本账号刷新，绝不回退全局帧（与目标
账号同语义，B22-01 封堵面的补全）。
**TDD**：`TestManualSnapshotFallbackOnlyWhenOwnFresh` 追加断言 3（student1 无目标专属帧
过期 + 全局帧新鲜 → 必须 false）。修复前红灯（回退高三帧 ok=true）→ 修复后绿灯。
附带：`handleElectives` 透传校验复用 `accountExists` 辅助（消除与 B27-01/02 的
LoadCredentials 逐账号比对重复代码，行为不变）。

## 前端（1 项，确认修复，commit `478d33b`）

### M28-01（MINOR）管理员删除账号后本地会话残留——被删账号占位 + 刷新复活
**缺陷**（review28-frontend）：删除账号的持久化（DELETE /api/admin/accounts 清 6 表事务 +
RevokeAccount）与前端 localStorage xk_sessions 脱钩——删除只发后端请求、从不清理本地
该账号条目。触发链路：管理员登录 admin、学生 A/B 均已登录 → 删除 A → 后端 6 表清掉 A 的
凭据/token、本地 sessions 仍含 A 条目 → A 继续占账号槽位 → 首次刷新浏览器
`loadSessions()` 恢复出 A，"已删除账号仍可操作/仍显示"假活状态 → 直到下次请求 401 才被
吊销链摘除。与 B18-M2/B20-01/B21-03/B26-01 后端"删账号绝不残留"整套理念不对称。
**修复**：Admin.tsx 加 `onDeleted` 回调（删除成功分支调用）；App.tsx 快照式三连
（saveSessions+setSessions+从最新快照删条目，与 logout/onUnauthorized 同款）移除被删
账号，并同步退出视图态——被删账号恰是 current 或 targetAccount 时置空（渲染
`targetAccount ? <Select>` 在 Admin 之前，代理态残留会让管理员卡死在已删账号代理页，
F15-07 同族）。绝不调用 logout()：删除的是他人账号，用管理员令牌调 /logout 会登出管理员
自己；且服务端会话已由后端 RevokeAccount 吊销，本地清除不造成令牌残留。
**验证**：`npm run build`（tsc -b）全绿。

## 可疑待核裁决（review28-backend）

- **可疑待核 1（ElectivesSnapshotFor 中间态串线）→ 确认为真缺陷，B28-01 修复**（上文）。
- **可疑待核 2（删除后立即重登同名账号时旧链复核放行）→ 定不修**：B18-M2/B20-01/B21-03
  的"落库前锁内复核 ClientFor"防线对"账号被删后同名立即重登（新客户端已进注册表）"窗口
  会复核放行——但旧链 SelectClass 的真实平台报名确实成功，RestoreDone 恢复为"已报名成功"
  是**合理结果**而非"删除即全清"违反：删除的是旧身份状态，新身份重建后该课成功记录是
  真实平台事实。边缘场景（删除后立即重登同名），危害可接受。
- **可疑待核 3（store 原地排序副作用）→ 不处理**：`sort.SliceStable` 原地改写调用方 slice，
  但 LoadTargetsForAccount 读库 ORDER BY priority、submitAll 内也按 priority 重排，最终
  一致；无调用方依赖原顺序，防御性清理非缺陷。
- **MINOR 1（ConfigTab 空串清空 vision_api_key 被静默忽略）→ 维持现状**：与 F7-02 空串
  显式清空 open_time 对比——清空 key 无合法用途（只会让验证码识别立即报错），"空串忽略
  保留旧值"是安全方向；"补明确提示"属 UI 层优化，留观察。
- **MINOR 3（lastSyncFailAt 本地钟 vs 判定对齐钟）→ 维持观察**：写侧在持锁段，改调
  nowAlignedLocked 会重入 Lock 死锁（需重构）；偏差 ~640ms 对 30s 退避窗口无实质影响。
- **review28-frontend 可疑待核 S28-01/S28-02 → 均经 gate 判不修**：
  - S28-01（Select 跨实例缓存复活）：App 两个 `<Select>` 调用点（targetAccount 分支 /
    current 分支）是不同 reconciliation 位置、跨挂载不复用实例；回显 effect `rev > 0`
    短路 + `setSelected(prev)` "选中有内容即返回 prev"双守卫保证回显不覆盖用户改动；
    react-query 缓存按含 account 的 queryKey 隔离、组件重建时查询重跑（staleTime:0）
    拉取后端真值。不构成静默丢失。
  - S28-02（ConfigTab refetch 覆盖未提交输入）：PUT 后 refetch 拉回的是"保存后的真值"，
    回填与用户刚提交的 body 字段一致（concurrency/engine 都在 PUT body 里），与 R27-01
    "与生效配置对齐"设计意图一致；仅首次加载慢 + 用户手快改字段的窄窗，无实际读写冲突。

## 主 gate 核实结论
- **M28-01 确认为真实缺陷**：删除账号本地会话残留与后端四段防线的"绝不残留"理念不对称，
  且代理态残留（targetAccount）有卡死管理员的具体 UI 路径。
- **B28-01 确认为真实缺陷**：B22-01 封堵目标账号串线后，无目标账号的"专属帧过期"中间态
  是剩余串线窗口——读取侧最后一块拼图，返回 false 触发刷新的语义与目标账号一致。
- 已核无缺陷：review28-backend 详列——锁序无环（scheduler.mu→accounts.mu→client.mu /
  reloginMu→s.mu）、删除账号四段防线位置正确、spawnChain 链顶短路顺序逐条核实、
  zhidao 解析/限流/重登自洽、config/db/secure/session/runtime/accounts/main 恢复顺序契约
  全部对齐；review28-frontend 详列——Select 保存链三件套闭环、假清空守卫链无绕过路径、
  401 吊销链四步齐全、壁纸滚动无泄漏、Login/激活幂等到位、Admin 查询键隔离、F9-05 闭包
  复验、类型安全防御点。
- 观察项：23-11 PUT config 空变更先落库、22-01 HTTP 探测单飞、22-02 Decrypt 死字段、
  22-03 RemoveFull 死方法、22-04 probe per-account 错误静默、22-06 emptyRunsFor 死代码、
  22-07 syncFailedWindow 写而不读、B28-MINOR1 空串忽略 UI 提示、B28-MINOR3 时钟双钟偏差。

## 回归
- backend：`go build ./... && go vet ./... && go test -race ./...` 全绿。
- frontend：`npm run build`（tsc -b + vite build）通过。
