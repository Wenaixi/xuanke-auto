# 第 10 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/main.go/cmd/web-embed）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI。
> 审查方式：op 权威子代理并行只读审查（后端 + 前端两个独立通道），全部发现定位到具体行号并经读码推演（部分写最小复现测试验证）成立；修复按 TDD（红灯→绿灯）或等价先行验证后独立 commit。
> 本轮结论：**后端 6 项真实修复（B10-01 重启恢复顺序、B10-02 退选路径补删 MarkTokenValid、B10-03 重设目标遇 refused 不掩盖退选意图、B10-04 AccountsWithTargets 排序、B10-05 删死字段、B10-08 Decrypt 注入备注明）**，**前端 5 项真实修复（F10-02 updater 副作用移出、F10-05 401 归属判定以 session 令牌为准、F10-06 去 3 门旧约束残留、F10-07 倒计时目标变化校正 now、F10-04 注释与 20s 超时对齐）**；前端 2 项经复核撤销（F10-01 已被 B10-02 修复、F10-03 sortTightest 排序等价性成立）；后端 2 项记录为观察项（B10-06 parseElectives code=1 语义、B10-07 handleActivate 先消费票据）。

---

## 一、后端发现与修复状态（B10 系列）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **B10-01** | MAJOR | **重启恢复顺序用了 SetTargetsForAccount，B9-02 被抵消**：第 9 轮加 Refused 持久化后，main.go 恢复目标仍走 `SetTargetsForAccount`——它内部 `DeleteRefused` 会删库行，紧随其后的 `LoadRefused` 读到空 map，重启后自动引擎把用户手动退选掉的课当新目标重新抢回 | ✅ 新增专用 `RestoreTargets`（与 SetTargetsForAccount 唯一区别是不清 refused），main.go 恢复顺序改为 `RestoreDone → 循环 RestoreTargets → LoadRefused + RestoreRefused`。新增 `TestRefusedRestartOrderRealDB`（用真实持久化语义的 persistentStore 复刻 SQLite，把顺序契约固化） |
| **B10-02** | MAJOR | **退选路径仍先调 MarkTokenValid，B9-01 修复不完整**：第 9 轮只改了报名分支，`handleElectiveExit` 的 ErrUnauthorized 分支仍在 `MaybeRelogin` 前调 `MarkTokenValid`——`delete(reloginFail)` 击穿指数退避的漏洞依然存在（半成品） | ✅ 删除 exit 分支的 `MarkTokenValid`，与报名路径对称只留 `MaybeRelogin`。审查 agent 复核定稿时确认此修复已随 d0eb18c 落地，原拟报的 F10-01 撤销 |
| **B10-03** | MAJOR | **重设目标遇 refused 不掩盖退选意图**：`rebuildCoursesForAccountLocked` 先判 done 再判 refused——课程曾被报成功后用户手动退选，done 恢复成功文案"重启恢复：已报名成功"掩盖退选意图，前端显示假成功 | ✅ refused 优先：先判 refused → pending + "已手动退选（自动引擎不再接管，可重新设为目标恢复）"，再判 done。新增 `TestRefusedPrecedesDoneOnReTarget` |
| **B10-04** | MINOR | **AccountsWithTargets 无序返回**：Go map 迭代随机，管理员省略 `?account=` 时 `targetAccts[0]` 不稳定——核心账号可能错选 | ✅ 返回前 `sort.Strings(out)`，管理员免 account 时核心账号稳定。新增 `TestAccountsWithTargetsStableOrder` |
| **B10-05** | MINOR | **prevWindowOpened 死字段**：struct 字段只写不读（B8 窗口状态裸指针根治后的残留） | ✅ 删除字段与赋值 |
| **B10-08** | INFO | **Decrypt 仅注入未消费**：api.Register 的 `decrypt` 参数传递链路完好但无人调用，易误导"加密入库"的完整性 | ✅ 保留注入（后续管理员查看 vision_key 明文可能用到），router.go 加注释澄清"B10-08 起未消费，仅备用" |
| **B10-06** | 观察项 | **parseElectives 将 code=1 当窗口关闭**：code=1 业务错误（如"未登录"之外的其他原因）与"窗口已关闭"（code=0 空 publishes）语义混同 | ⏳ 记录待办，合并入 B9-04/B7-M5 待办链；当前 code=1 极罕见且表现安全（空快照 + 30s 闸门），不急于拆 |
| **B10-07** | 观察项 | **handleActivate 先消费票据再验证**：激活请求先 `ConsumeTicket` 扣减票据再校验激活码——校验失败时票据已消耗 | ⏳ 记录为可接受（用户必须输入错误 code 才会触发，且激活码校验失败本身不产生副作用；与限流桶协同不构成放大攻击） |

## 二、前端发现与修复状态（F10 系列）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F10-02** | MINOR | **pick() 在 setSelected updater 内调 toast**：updater 必须纯函数，StrictMode 双调、并发渲染下丢弃并重放都会让"已设为首选/已取消目标"弹窗重复弹出；且 updater 内调另一组件 setState 属渲染期更新反模式 | ✅ 重构 pick()：在 updater 外基于当前 selected 先算 next 快照 → `setSelected(next)` + 独立 toast。事件处理器内 selected 恒为最近已提交渲染值（两次独立点击间有渲染提交），与函数式更新等价且无副作用混入 |
| **F10-05** | MINOR | **401 归属判定在管理员代理态下卡死**：client.ts 优先取 URL `?account=`（管理员代理看学生时为学生名），App 反查 localStorage 无该学生会话 → 无人被剔除 → 管理员自身会话过期后卡在代理页反复 401 | ✅ client.ts 事件 detail 同时携带 `{account, session}`，App 侧归属判定一律以 `detail.session`（发起请求的 Bearer 令牌，401 的真实主体）为准，`detail.account` 仅在 session 缺失时兜底且反查无果直接跳过——管理员会话过期正常剔除回登录页 |
| **F10-06** | INFO | **Dashboard 残留"3 门"旧约束**：`{courses.length} / 3 门`、`3 COURSES MAX`、`TARGETS {n}/3`——后端上限 100 门且每发布可配多条备选，硬编码展示与真实能力分叉 | ✅ 去掉 "/ 3 门" 与 "3 COURSES MAX"，标题角标改 "TARGETS"，移动端改 `TARGETS {n}` |
| **F10-07** | INFO | **useTickingCountdown 注释与实现不符**：注释说"目标变化时回到 now"，但 diff 用旧 now 计算——target 由 null 变为有效开窗时刻时最多 1 秒陈旧偏差（可能短暂误显为过期全 00） | ✅ target 变化时补 `setNow(Date.now())`（新 useEffect 依赖 target），注释意图落实 |
| **F10-04** | INFO | **client.ts 注释"2 秒超时"与实际 20 秒分叉**：服务端挂起时前端转圈 20 秒，与"不无限转圈"意图偏差 10 倍 | ✅ 注释改为"20 秒超时兜底"并说明保持 20 秒的权衡（本 api() 是全站共用通道，/electives 大列表在开窗黄金期响应偏慢，收紧到 2 秒会掐断大列表刷新；20 秒对 abort 兜底本意依然成立，目标保存还有指数退避重发兜底） |
| **F10-01** | 已撤销 | **handleElectiveExit 仍先调 MarkTokenValid** | ✅ 复核确认已随 d0eb18c 的 B10-02 修复，撤销 |
| **F10-03** | 已撤销 | **sortTightest 按 selected_count 排序而非剩余名额** | ✅ 复核为伪发现：sort 发生在单个发布 tab 的 filteredClasses 内，同一发布所有课程 max_count 恒定（生产数据体育 36/校本 29），selected_count 升序与 (max_count - selected_count) 升序等价，行为正确 |

## 三、修复细节（本轮 11 项生产改动，3 个独立 commit，均验证后提交）

- **B10-01+B10-02**（commit d0eb18c）：main.go 恢复顺序用 `RestoreTargets`（新增）+ scheduler 方法重构 + handler.go exit 分支删 MarkTokenValid；`TestRefusedRestartOrderRealDB`（真实持久化顺序契约）红灯→绿灯，`TestRefusedNeverResubmitted` 同步更新
- **B10-03/B10-04/B10-05/B10-08**（commit b129a10）：rebuildCoursesForAccountLocked refused 优先 + AccountsWithTargets 排序 + 删死字段 + Decrypt 注释；`TestRefusedPrecedesDoneOnReTarget` + `TestAccountsWithTargetsStableOrder` 红灯→绿灯
- **F10-02/F10-04/F10-05/F10-06/F10-07**（commit c74218e）：Select.tsx pick() 重构 + client.ts 注释/事件 detail + App.tsx 归属判定 + useTickingCountdown 校正 + Dashboard 文案

## 四、回归证据（提交时点通过，收尾前复跑全量）

- `cd backend && go test -race ./...` — 全包绿（api 17.9s / scheduler 17.3s / store 7.2s / zhidao 等）
- `cd web && npx tsc -b && npx vite build` — 通过（1946 modules，405.80 kB / gzip 121.70 kB）
- 提交序列见提交索引（B10-01+02 → B10-03/04/05/08 → F10 系列五个 → 本文档 + CLAUDE.md 沉淀）

---

## 提交索引（本轮 3 个独立 commit + 文档）

```
d0eb18c fix(backend): 第10轮B10-01重启恢复顺序用RestoreTargets(不清refused)+B10-02退选路径补删MarkTokenValid——B9-02/B9-01修复不再被启动顺序/半成品抵消，新增RestoreTargets与真实持久化顺序契约测试
b129a10 fix(backend): 第10轮B10-03重设目标遇refused不掩盖退选意图+B10-04 AccountsWithTargets排序(管理员免account时核心账号稳定)+B10-05删prevWindowOpened死字段+B10-08 Decrypt仅注入备注明——重启恢复与前端对齐稳固
c74218e fix(web): 第10轮前端F10-02 updater副作用移出toast+F10-05 401归属判定以session令牌为准(管理员代理页不再卡死)+F10-06去3门旧约束残留+F10-07倒计时目标变化校正now+F10-04注释与20s超时对齐——管理员会话过期收敛与弹窗幂等双修
(review-round10.md + CLAUDE.md 沉淀)
```

## 下轮待办（合并链）

- B10-06 parseElectives 区分 code=1 错误语义（合并入 B9-04/B7-M5/B8-M4 probe 并发单飞待办链）
- 重点复核 maybeRelogin 指数退避 reloginFail 在手动/自动双路径下的清零时机一致性
- 管理员穿透代理的全链路会话过期收敛路径（F10-05 后续回归观察）
