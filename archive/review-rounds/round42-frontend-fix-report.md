# round42 前端修复报告

日期：2026-09-20
修复人：前端修复代理（web/ 目录范围，未触碰 backend/——工作区 5 个 backend 文件为并行修复代理在改，本轮未 commit 它们）

## 回归基线

- `cd web && npx tsc -b`：通过
- `cd web && npm run build`：通过（vite build 绿，产物落 backend/web/dist，1.2s）
- 纯函数断言脚本 `./node_modules/.bin/jiti.cmd scripts/target-guard-check.ts`：全绿（jiti 随项目自带；`node --import tsx` 不可用，与 round41 报告记录一致）

## F42-M1（MAJOR）：防抖保存链死锁——守卫命中置脏后 setSelected 返回同引用被 React bailout，订阅永不重入

- 根因：守卫判据 `!echoedRef.current && (stateDataRef undefined || courses>0)` 依赖 echoedRef——/state 持续失败期间守卫命中置脏，selected 无变化（setSelected 返回同引用被 React bailout）→ 防抖 effect 不重跑 → 保存链死锁至整页刷新
- 修复（主控既定三件套）：
  1. 守卫判据抽纯函数 `shouldDeferSave(stateData)`（`stateData===undefined || courses 非空` → 推迟），不依赖 echoedRef
  2. flushTargets 同款守卫同步改纯数据判据
  3. 防抖 effect 依赖补 `stateData`（react-query 数据到达触发重跑 → 新 timer → 守卫通过 → 落库自愈）
  handleBack 等待循环同步改为同一数据判据（含 while 轮询），三处判据同源
- 测试形态（TDD 先红后绿）：`web/scripts/target-guard-check.ts` 追加 shouldDeferSave 三条断言（undefined→true / courses 空→false / courses 非空→true）——红（未实现时 `is not a function` 抛错）→ 绿（实现后全过）
- commit：`3d6f357 fix(select): 防抖保存守卫改纯数据判据，补 stateData 依赖解除保存链死锁（F42-M1）`

## F42-M3（MAJOR）：/electives 轮询升频被 /state 失败吞掉——st===undefined 时 window_opened 信号丢失

- 根因：/electives refetchInterval 回调在 /state 缓存无 data（st===undefined）期间只看 inRange——开窗瞬间 publishes 短暂为空时 inRange=false → 10s 慢轮询进黄金期，升频失效；且失败态恒不降频（F40-M3 对 /state 做了、对 /electives 没做的另一半）
- 修复（主控既定，采纳 F40-M3 同款）：refetchInterval 回调顶部加 `if (query.state.error || query.state.status === "error") return 30000`——失败态降频 30s（一并解决失败态 2s/10s 轰炸），成功态维持现有 `st?.window_closed ? 30000 : inRange || st?.window_opened ? 2000 : 10000`
- 测试形态：逻辑走查 + `npx tsc -b` 类型校验（interval 回调内联改）
- commit：`9e2cbab fix(select): /electives 轮询失败态降频 30s，失败期间不吞窗口升频信号（F42-M3）`

## F42-N1（MINOR）：F41-M1 独立清理 effect 依赖漏 selected——发布重建与 /state 首帧交错时清理落在回显合并之前

- 根因：独立清理 effect 依赖 `[publishes, echoedRef, toast]` 不含 selected——发布重建瞬间 effect 用"重建前旧 selected"清理，若 /state 首帧交错到达，清理可能先于回显合并执行，掉队的 stale 残留无人清理
- 修复：依赖数组补 `selected`（`[publishes, selected, echoedRef, toast]`）——合并带出 stale 非空 key 时 effect 随 selected 变化重跑补清；cleanStaleSelected 无变更返回原引用保证幂等、不引出多余重渲染
- 测试形态：类型校验 + 逻辑走查（cleanStaleSelected 幂等性论证，既有 10 断言回归绿）
- 注：本项修复随 Select.tsx 一次提交（与 F42-M3 同文件，软回退重提交时同批落盘于 `9e2cbab`；git 历史说明见 commit message）

## 收尾确认

- 每处缺陷独立 commit（M1 / M3+N1），未 push
- 工作区 web/ 目录无残留改动（临时副本已删）；未跟踪文件为仓库根 pre-existing 文件与 round42 两份 findings 报告
- 未触碰 backend/ 任何文件（dist 产物由 npm run build 自动生成，属 web 构建输出）
