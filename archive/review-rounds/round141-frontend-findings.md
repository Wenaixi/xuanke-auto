# R141 前端只看审查 — xuanke-auto web/（React 19 + Vite + TS）

> 审查对象：M-1 第七十七轮（保存链守卫闭合）+ 维持观察族复核 + 新契约角度纵深
> 基线：`60ea9d6`（R140 归档，M-1 第七十六轮闭合）
> 模式：绝对只读，唯一写文件为本报告
> 实测时间：2026-09-24

## 结论前置

| 等级 | 数量 | 内容 |
|------|------|------|
| CRITICAL | 0 | — |
| HIGH | 0 | — |
| MEDIUM | 0 | — |
| MINOR | 0 | — |
| OBSERVE | 3 | Select 倒计时候选维持、Admin refetch 出口维持、perf 守卫弱断言维持 |

**总评：建议 APPROVE。** 全部聚焦清单逐项实测通过，`web/` 自 `60ea9d6` 基线双空实证成立，前端零改动，无任何新增缺陷。M-1 第七十七轮保存链守卫六要素全部逐字符在位；OBSERVE-93/116/115 全数零漂移；R125 候选维持记录不实现；四守卫脚本全绿；build 通过；XSS 面零命中。两个新契约角度（401 吊销切号整体重建 / client.ts abort 与 401 统一映射）纵深核对零偏离。

## 验证表（全部实测）

| 项目 | 实测方式 | 结果 | 实测数据 |
|------|----------|------|----------|
| 基线确认 | `git rev-parse HEAD` | 通过 | `60ea9d65db903bc0158cb3d7f4c36fb61203b898` |
| 工作树漂移 | `git status --porcelain` | 通过 | 全空（HEAD 即为基线，构建产物不受版本控制） |
| 前置构建守护 | 后台 `npm run build` | 通过 | `tsc -b && vite build`，vite 8.3.0，1948 模块，473ms，`../backend/web/dist` 三资产落盘 |
| target-guard 断言 | `node --import jiti/register scripts/target-guard-check.ts` | 通过 | **18/18 全绿**，退出码 0 |
| perf-countdown 断言 | 同款 jiti 运行 | 通过 | 3/3 全绿（自 tick 契约 / memo 叶子存在 / 无裸 cd.* 消费 0 处） |
| admin-auth 断言 | 同款 jiti 运行 | 通过 | 6/6 全绿（管理令牌恢复、撞名学生区分、吊销退出、自定义名） |
| unauthorized 断言 | 同款 jiti 运行 | 通过 | 5/5 全绿（extractAccountFromPath 五场景） |
| XSS 面 | `grep -rn "dangerouslySetInnerHTML" src/` | 通过 | 零命中（exit=1） |
| web 至基线 commit 漂移 | `git log 60ea9d6..HEAD -- web/` + `git diff --stat 60ea9d6..HEAD -- web/` | 通过 | 双空（0 commit / 0 diff，F93-01 第四十九轮实证） |
| `<button` 全仓计数 | `grep -rc "<button" src/` | 通过 | 全仓 17 处 3 文件（Admin 13 / Dashboard 2 / Login 2），Select 与全部 ui/ 组件 0 |
| echoedRef 写位计数 | `grep -c "echoedRef.current =" src/routes/Select.tsx` | 通过 | 3（:200/`:240`/`:297`），无第四处写 true |
| shouldDeferSave 调用计数 | `grep -c "shouldDeferSave(" src/routes/Select.tsx` | 通过 | 4（整词含导入行为 5） |
| 弹窗语义门行号 | `sed -n` 三处 | 通过 | Select:1204 / Login:226 / Admin:213 |

## 聚焦清单逐项裁决

### 1. M-1 第七十七轮闭合（保存链守卫）— ✅ 在位

**shouldDeferSave 定义**：`web/src/lib/targetGuard.ts:64-71`，判据本体与 R140 逐字符一致：
- `:69` `if (stateData === undefined) return true`
- `:70` `if (echoed) return false`
- `:71` `return (stateData.courses?.length ?? 0) > 0 && hasSelected`

第三参 `echoed` 的稳态放行语义由函数上方 9 行注释（:57-63）面述，含"已回显完成的账号 courses 永驻非空，继续推迟会把后续所有编辑永久闷死"的守卫盲区契约，注释与实现零漂移。targetGuard.ts 全文 72 行 / 31 条注释，三函数（selectedHasStalePublish / cleanStaleSelected / shouldDeferSave）各含出口，均为目标合约编写。

**恰 4 消费点逐字符一致**（统一三参形态 `(stateDataRef.current, 选中数判据, echoedRef.current)`）：
- `:509` flushTargets 首闸 —— 第二参 `latestSelectedCount > 0`（ref 派生实数值）
- `:597` handleBack 首闸 —— 第二参 `hasSelectedNow()`（:579-580 定义，消费时刻读 selectedRef 重算，注释明确 handleBack 无渲染闭包可用）
- `:605` handleBack 5s 等待轮询 while —— 第二参同 `hasSelectedNow()`，第三参同为 `echoedRef.current`（触发等待立即结束语义）
- `:699` 防抖回调 —— 第二参 `selectedCount > 0`（渲染期派生）

四参形态统一、注入源各异（ref 实数值 / 消费时刻函数 / 渲染派生）但判据类型全为 boolean，与函数签名 `(stateData, hasSelected: boolean, echoed: boolean)` 一致。四处注释全部面述"判据为纯数据、不依赖 echoedRef；stateData 到达触发 effect 重跑自愈"，链路闭环。

**echoedRef 三置位、无第四处写 true**：`grep -c` = 3：
- `:200` 账号复位守卫 `= false`（复位语义，账head 切换 instance 整体重建时的重置）
- `:240` courses 空分支 `= true`（/state 首帧到达且确证后端无旧目标，注释"绝不提前置位"后的完成信号）
- `:297` 合并完成 `= true`（回显合并执行终点的唯一写位）

:229（回显首闸）、:234（stateData undefined 返回）、:319（清理 effect 未回显短路）均为只读位，不写值。写位与 R140 逐行一致，无新增。

**首帧四边界**：:229（echoedRef 短路）/ :234（`stateData === undefined` 提前返回，绝不提前置位——注释明确"否则 /state 晚于首次点击到达时整包覆盖删除旧目标"）/ :247（`pubs.length === 0` 返回）/ :319（清理 effect `!echoedRef.current` 短路）。四边界均附带注释面述语义，与 R140 逐行一致。

**F40-M1 cleanStaleSelected / F43-M1 hasSelected 清空分判**：
- `:26-43` cleanStaleSelected 无变更返回原引用（`return changed ? next : selected`），空数组键保留（清空语义绝不复活），泛型化 `Record<number, T[]>` 双消费（:290 回显内、:322 独立清理 effect）。
- F43-M1 hasSelected 第二参已承担"首帧携带旧目标但用户一个都没选 = 清空意图确凿"的分判，targetGuard-check 断言 #7「courses 非空 + 全清空 → 放行」与 #9「首帧未到 + 已回显标志 → 仍推迟」实测绿，确认清空永不与慢首帧混判。

**flush/handleBack/防抖三闸双闸等回显**：flush（:509）与防抖（:699）为纯守卫置脏跳过态；handleBack（:597）为 5s 轮询等待（50ms 间隔，注释明确 10ms 会让 5s 窗口连开约 500 个定时器空转）+ 等待后 `await new Promise(setTimeout r, 0)` 落地 selectedRef，三点同一判据单源。flush 自身 `shouldDeferSave` 命中即置脏跳过、不置 dirtyRef（终局绝不误报保存失败）——该契约在 :503/:510/:700 三处注释反复确认。

### 2. OBSERVE-93-01 第三十二轮 — ✅ 在位

`grep -rc "<button" web/src/` 全仓 17 处 3 文件：Admin 13 / Dashboard 2 / Login 2，零增零减，Select 与 ui/ 组件保持 0（全部经 `<Button>` 抽象）。651/661 候选继续维持（识别引擎切换 Vision/ddddocr 二选一按钮，display 级单选态无自动升级面，金融/交互升级候选维持观察，不实现）。

### 3. F93-01 第四十九轮 — ✅ 双空实证

- `git log --oneline 60ea9d6..HEAD -- web/`：0 commit（exit=0，空输出）
- `git diff --stat 60ea9d6..HEAD -- web/`：0 行（空输出）

双空实证成立，前端全库维持 `60ea9d6` 快照。

### 4. OBSERVE-116-01 第二十五轮 — ✅ 在位

**注释口径统一**：`useTickingCountdown.ts:3-8` 顶部注释面述"每秒 setNow 实际触发宿主路由组件重渲染；消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo）"。Dashboard.tsx:90-93 组件顶部同类注释明确"整树 770 行 DOM 不再每秒重建，只有 4 个数字文本节点重渲染"，:206-212 路由注释（"已收敛到 MemoCountdownMatrix 叶子"）三处同源，均与 lib 顶部口径一致。

**五路轮询契约逐键零漂移**（error 与 window_closed 降 30000 全站统一）：
- Dashboard /state（:169-174）：error/status==="error" → 30000；`window_closed` → 30000；否则 3000
- Dashboard /logs（:185-190）：error → 30000；`state?.window_closed` → 30000；否则 3000
- Dashboard /electives（:203）：恒 30000（begin_times 静态 + 快照 TTL 40s）
- Select /electives（:58-82）：error → 30000；`st?.window_closed` → 30000；`inRange || window_opened` → 2000；否则 10000（升频判定来源 in_date_range 与 window_opened 双信号合并，注释 :59-68 面述）
- Select /state（:148-155）：error/status==="error" → 30000；`window_closed` → 30000；否则 2000

error 与 window_closed 降频 30000 在全部轮询位统一，升频率按信息新鲜度分层（2s/3s/10s），与「失败分级退避」防轰炸理念一致，零漂移。

### 5. OBSERVE-115-01 弹窗族 — ✅ 在位

三处最小语义门行号与 R140 一致：
- **Select.tsx:1204-1211**：退选二次确认 Modal。`role="dialog" + aria-modal="true" + aria-labelledby="exit-modal-title"`，Esc 关闭带 `actionLoading.has(exitModalClass.id)` 在飞防误关（:1208），注释面述"退选中不响应防误关"。
- **Login.tsx:226-238**：激活码 Modal。`role="dialog" + aria-modal="true" + aria-labelledby="activate-dialog-title"`，Esc 关闭带 `activating` 在飞防误关（:233），激活码输入 autoFocus（:267）。
- **Admin.tsx:213-218**：删除账号 Modal。`role="dialog" + aria-modal="true" + aria-labelledby="delete-acct-modal-title"`，Esc 关闭带 `deleting` 在飞防误关（:217）。

三处均为「最小语义门」承诺形态（裸 div + 三属性 + Esc + 在飞防误关），注释同步确认"完整焦点陷阱迁移到 Radix Dialog 属后续候选"，零漂移。

### 6. R125 候选复核（Select:850 内联 cd.*）— ✅ 维持成立，不实现

- **Select.tsx:844 / :850-851 内联 cd.\***：`:844` `cd.isExpired`（等待态文案），`:850` `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒`。选课大厅为整屏一体化倒计时（文案 + 数字区块一体，无 Dashboard 四格独立矩阵），拆 memo 叶子收益低（Select.tsx:204-207 注释原语"DOM 差分成本可忽略"）。
- **缓解因子无回归**：Dashboard 已拆 MemoCountdownMatrix（:117 `memo(CountdownMatrix)`），perf 守卫断言「路由组件顶层无裸 cd.* 消费（当前 0 处）」实测绿；Select 侧 cd 仅 3 处引用（:823 注释内的历史引述、:844/850 渲染），无扩散。
- **守卫盲区契约零漂移**：Select.tsx:821-826 状态横幅注释面述"window_closed 优先显『已关闭』，否则按 window_opened 判定"，:829/834/839/844 渲染链一致。
- **维持记录不实现** ✅。

## 新契约角度（自选 ×2）

### 角度 A：401 吊销切号整体重建（key={account} 守卫纵深）

**选择理由**：这是保存链守卫族最后一道防线——前面所有守卫都假设 selected/echoedRef 属于单一账号实例，若账号切换不清状态，旧账号目标会经防抖 PUT 整包覆盖新账号。

**实测发现**：App.tsx 两处 Select 挂载点均带 `key`：
- `:293-298` 代理分支 `key={targetAccount}`（管理员代看学生大厅）
- `:339-344` 学生分支 `key={current}`（page==="select"）

key 变化即整体卸载重建实例，selected/echoedRef/rev 全复位。Select.tsx:190-202 另有守卫兜底：`accountKey` state 与 `account` 不一致时同步复位 `setSelected({}) / setRev(0) / echoedRef.current=false / setEchoDone(false)`（注释明确"仅兜底未来改为不重置挂载的意外回归"）。守卫声明于 echoedRef/rev/setRev/setEchoDone 之后（:194 注释"TDZ 不触发"），TS2454 陷阱已规避。

**零偏离**。key 驱动重建为第一道防线、accountKey 守卫为第二道，双保险覆盖主路径与意外回归，与契约 13「Select 挂载点必须 key={account}」逐字对齐。

### 角度 B：client.ts abort 统一映射与 401 广播族

**选择理由**：client.ts 是全站唯一 HTTP 通道，abort 映射与 401 广播直接决定「失败分级退避」与「会话吊销链」行为是否会被半途破坏。

**实测发现**（`web/src/api/client.ts`）：
- 20s 超时兜底（:56 `setTimeout(() => ctrl.abort(), 20000)`），注释面述"本 api() 是全站共用通道，收紧到 2 秒会掐断大列表刷新"——与后端 fail/retry 退避分级兼容。
- signal 显式接入（:58 `signal: rest.signal ?? ctrl.signal`）——调用方（TanStack Query 卸载清理）优先于兜底超时。
- abort 统一映射（:99-105）：`AbortError` 一律转 `new ApiError(-2, "请求超时，请重试")`，绝不把原生 "This operation was aborted" 泄漏进 toast。**边界核对**：AbortError 与 -2 的组合在 Select 保存链 catch 进 saveNow catch → `e.message` 显示"请求超时，请重试"并 scheduleRetry（:445-456 指数退避 2/4/8/16/16s），与前端"目标保存挂了还有指数退避重发兜底"的注释契约一致。
- 401 广播族三种形态各单次（:64-69 HTTP 401 先广播 + :75-96 body 401 兜底 + `if (r.status !== 401)` 双发抑制）——批量化吊销时事件风暴减半，App.onUnauthorized 幂等兜底。广播先于 `r.json()`（网关/反代 HTML 401 不丢事件）。
- 调用方传参正确性核验：saveNow 的 PUT 未传 signal（正常——卸载由 unmountedRef 短路守卫 + 超时兜底双保险），selectElective/exitElective 也未传 signal（手动报名在飞守卫由 actionLoading Set 承担）；无调用方 signal 被静默覆盖的反模式残留在本文件外。

**零偏离**。

## 维持观察项（延续 R140）

1. **Select.tsx:204-207 注释口径残留**——自 R124 起延续：注释仍写"如需真正做到『只重渲染倒计时一处』需拆独立 memo 叶子组件（潜在优化，非当前承诺）"，而 Dashboard 侧已拆 MemoCountdownMatrix（perf 守卫第 2/3 断言验证）。Select 侧为整屏集成式倒计时（收益低，:848-851 一体化文案块），口径残留属上下文差异而非错误——此注释描述 Select 自身实现意图（其组件确实未拆），与 Dashboard 的差异分属两种形态，维持记录不实现。若未来将 Select 倒计时拆叶子，此为已记录的原位注释。
2. **OBSERVE-93-01 Admin refetch 出口维持**——Admin.tsx 651/661 等 refetch 按钮（识别引擎切换二选一）无自动升级语义，display 级单选态不触发任何状态级副作用，维持观察不实现。其余 refetch 按钮（:441/:452 激活码、:701 配置、:792 状态、:898 账号、:944 日志）均为页级手动刷新出口，为「浏览器提供 refetch 即用户意图」的常规交互，无自动升级面。
3. **perf-countdown-guard 弱断言属性**——3 断言基于正则结构匹配（自 tick 契约 / memo 叶子存在 / 无裸 cd.* 裸消费），非真实渲染剖面。memo 失效但正则仍通过是理论候选，但 memo 是性能界限的分水岭，结构性事实守卫足以拦截回归，继续维持。

## 建议

**建议 APPROVE。**

M-1 第七十七轮保存链守卫（shouldDeferSave 定义 + 恰 4 消费点逐字符一致 + echoedRef 三置位无第四处 + 首帧四边界 + cleanStaleSelected 原引用 + hasSelected 清空分判 + 三闸双闸等回显）全部实测在位；F93-01 第四十九轮双空实证成立；OBSERVE-93/116/115 全数零漂移；R125 候选（Select:850 内联倒计时）维持记录不实现；两个新契约角度（401 吊销 key={account} 整体重建 / client.ts abort 与 401 广播族）均零偏离。四守卫脚本（target-guard 18 / perf-countdown 3 / admin-auth 6 / unauthorized 5）全绿；`npm run build` 通过；XSS 面零命中；工作树零漂移（HEAD=60ea9d6 即基线）。前端自 `60ea9d6` 零改动，无需要修复项。