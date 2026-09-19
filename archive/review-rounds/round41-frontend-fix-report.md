# round41 前端修复报告

日期：2026-09-20
修复人：前端修复代理（web/ 目录范围，未触碰 backend/）

## 回归基线

- `cd web && npx tsc -b`：通过
- `cd web && npm run build`：通过（vite build 绿，产物落 backend/web/dist，1700ms）
- 纯函数断言脚本 `node --import jiti scripts/target-guard-check.ts` 与新增 `scripts/unauthorized-check.ts`：全绿
- 说明：`tsx` 运行器不在项目依赖（`npm run build` 用 vite8/rolldown 与 esbuild 无关），已按约定改用同款零装依赖 `jiti`（随项目附带）运行 TS 断言脚本

## F41-M1（MAJOR）：发布重建 stale 清理失效

- 根因：F40-M1 清理逻辑挂在回显 effect 内、且位于其首行 `if (echoedRef.current) return` 之后——已完成回显的账号 echoedRef 恒 true，后续发布集合整体重建（开窗瞬间 publish_id 全变）时清理永不执行，stale 守卫把保存链静默锁死至整页刷新
- 修复方向（主控既定）：清理下沉为独立 `useEffect`，声明于 `const publishes` 之后（TDZ 安全，F18-01 同款防护），依赖 `[publishes, echoedRef, toast]`；`echoedRef.current && publishes.length>0 && selectedHasStalePublish(selected, publishes)` 三条件命中即 `setSelected(cleanStaleSelected)` + warning toast；setSelected 触发防抖 effect（依赖含 selected）重跑自动落库当前目标。回显 effect 内的原清理块删除，只保留"回显数据 currentIds 过滤"职责
- 时序论证（文字）：已回显账号（echoedRef=true）发布重建 → 本次等值的情况下 independent effect 依赖 `publishes` 引用变化 → effect 重跑 → stale 命中 → setSelected 清理（selected 变化进入下次渲染）→ 防抖 effect 依赖 `selected` 重跑 → 400ms 后消费时刻守卫通过（stale 已清、回显已置位）→ saveNow 落库。未回显（echoedRef=false）时不清理：保证不干扰回显合并时序（合并与 currentIds 同判据过滤幽灵条目）
- 测试形态：类型校验 + 逻辑走查（纯函数 cleanStaleSelected 逻辑未变，target-guard-check 既有 10 断言回归绿）
- commit：`16d37cb fix(select): 发布重建清理下沉独立 effect，已回显账号残留旧目标自动清理（F41-M1）`

## F41-M2（MAJOR）：flushTargets 缺回显未完成守卫

- 根因：防抖回调有回显未完成守卫，flushTargets（handleBack 内部使用）没有——/state 首帧持续失败超过 handleBack 5s 等待后，flush 拿"只含用户新点击"的 selected 通过其余守卫整包 PUT，把后端已保存旧目标替换成用户本意"新增"的几门（"加一门"变"替换全部"）
- 修复方向（主控既定）：flushTargets 顶部、`latestRev === 0` 跳过之后补与防抖回调同款守卫——`!echoedRef.current && (stateDataRef.current === undefined || (stateDataRef.current.courses?.length ?? 0) > 0)` → `dirtyRef.current = true; return`（消费时刻读 ref，与防抖同源）。handleBack 5s 超时后仍放行返回，flush 内守卫拦截，dirty 保留，下次进入/刷新/回显完成后再落库（安全方向）
- 守卫链顺序走查：latestRev===0 → 回显未完成守卫（新增）→ 发布缺席+已有选中假清空守卫 → stale 守卫 → 联查产物为空守卫 → targetsUseCurrentPublishes 全 id 校验 → 保存。新增守卫位于最前，任何可能"整包覆盖旧目标"的路径先被拦截
- 测试形态：类型校验 + 逻辑走查（评审全守卫链顺序）
- commit：`8ac4563 fix(select): flushTargets 补回显未完成守卫，超时后不整包覆盖后端旧目标（F41-M2）`

## F41-N1（MINOR）：名额未公布进度条误导

- 根因：`<Progress max={c.max_count || 1}>` 在 max_count=0（名额未公布）时分母变 1，selected_count 数十到数百渲染满条，与"名额未公布"文案并存误导
- 修复：`value={unannounced ? 0 : c.selected_count}`、`max={unannounced ? 1 : c.max_count}`——未公布恒空条，aria-valuenow 如实反映"无进度"语义；名额定档的课程行为不变
- 测试形态：纯渲染修复，类型校验 + build
- commit：`bc77e58 fix(select): 名额未公布时进度条显空条，消除满条误导（F41-N1）`

## F41-N2（MINOR）：非 JSON 401 不广播失效事件

- 根因：`r.json()` 先于 `j.code === 401` 判断——网关/反代返回 HTML/文本 401 时 `r.json()` 抛错走 -2 文案，UNAUTHORIZED_EVENT 永不派发，App.onUnauthorized 不执行，失效会话账号永久残留
- 修复方向（主控既定）：`r.status === 401` 时在 `r.json()` **之前**先广播（复用既有 detail{account, session} 形状）；JSON 解析失败仍抛 -2 文案但事件已到达
- TDD 增强：把"从路径反查 ?account 穿透目标"抽为纯函数 `extractAccountFromPath`（原本内联在响应处理分支），新增 `web/scripts/unauthorized-check.ts` 5 断言（居中/末尾/无参数/伪路径/空路径）；先红后绿（导出前断言报 `is not a function`）
- 注：HTTP 状态码 401 与业务 code=401 双广播（后者为原逻辑保留）——事件幂等，detail 相同，App 按令牌反查无副作用
- commit：`7b5028c fix(client): HTTP 401 在 JSON 解析前广播失效事件，摘除失效会话账号（F41-N2）`

## 收尾确认

- 每处修复独立 commit，共 4 条，未 push
- 工作区仅剩仓库根目录未跟踪文件（CODE_OF_CONDUCT 等 pre-existing 与 round41 两份 findings 报告），web/ 目录内无残留改动
- 未触碰 backend/ 任何文件（dist 产物由 npm run build 自动生成，属 web 构建输出）