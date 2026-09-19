# round40 前端修复报告（TDD 严格模式）

修复人：round40-frontend（只碰 web/ 目录，未动 backend/）
日期：2026-09-20

## 修复清单

### F40-M1（MAJOR）— 已修
- **缺陷**：round39 C-1 守卫命中即静默 `dirtyRef=true; return`——旧 publish_id key 对应 Tab 在发布重建后消失、用户无法通过界面清除，守卫永久拦截后续所有保存（每次防抖/flush 静默跳过），黄金期改目标永不落库，唯一恢复途径是整页刷新；且用户点课先弹"已设为首选"成功 toast，体验是"看起来保存成功实则永存不上"。
- **测试形态**：纯函数断言脚本 `web/scripts/target-guard-check.ts`（`npx --no-install tsx scripts/target-guard-check.ts`）追加 5 个场景（F-G-H-I-J）。TDD 红：脚本 import `cleanStaleSelected` 时目标模块无此导出 → SyntaxError exit 1；绿：实现后 10 场景全绿。
- **修复**（两路并做）：
  1. 新增纯函数 `cleanStaleSelected(selected, currentPublishIds)`（`web/src/lib/targetGuard.ts`，只删"非空且不在当前集合"的 key，空数组清空语义保留，无变更返回原引用防不必要重渲染）；
  2. 回显 effect 末尾在 `selectedHasStalePublish` 命中时随重建清理 selected 并弹一次提示 toast（自愈，无需手动刷新）；防抖回调与 flushTargets 两处守卫命中处补明确 toast「发布已更新/旧批次目标已失效，已停止保存。请刷新页面重新选择」（均判 `!unmountedRef.current` 才弹，卸载后不轰炸）。
- **commit**：`808fae7`

### F40-M2（MAJOR）— 已修
- **缺陷**：round39 N-1 只改了一半——倒计时矩阵吃 begin_times 兜底，文案行 `openTimeStr ? ... : state ? "未识别到开放时间" : ...` 未同步；open_time_known=false 但 begin_times[0] 非空时矩阵明确倒数、同屏文案宣告"未识别到开放时间"，两套事实自相矛盾。
- **测试形态**：类型校验（tsc -b exit 0）+ 逻辑走查（三元链按 openTimeStr → begin_times[0] → state → 同步中 四级降级，与矩阵 cd 输入同源）。
- **修复**：Dashboard.tsx 文案行吃同源兜底——`openTimeStr ?? (begin_times[0] 存在 ? 格式化(begin_times[0]) : state ? "未识别到开放时间" : "正在同步教务平台时间配置...")`。
- **commit**：`533ccde`

### F40-M3（MAJOR 复证）— 已修
- **缺陷**：`/state` 查询失败/无数据时 `refetchInterval` 恒 2s 高频重试——react-query 失败后 data 为最后一次成功值或 undefined，interval 全取 2000ms，网络挂断/后端重启期间 /state + /electives 双查询叠加固定 2s 轰炸。
- **测试形态**：逻辑走查（interval 回调内联改，三分支：error/status=error → 30s；成功 + window_closed → 30s；成功 + 未关 → 2s）+ 类型校验。
- **修复**：Select.tsx `/state` 查询 refetchInterval 前置失败判据——`if (query.state.error || query.state.status === "error") return 30000`，成功态按 window_closed 升/降频。
- **commit**：`7618a26`

### F40-N1（MINOR）— 已修
- **缺陷**：Dashboard 日期分组"今天"基准用 `new Date().toISOString().slice(0, 10)`（恒 UTC），UTC+8 凌晨 00:00-07:59 时 toISOString 是昨天日期 → todayMs=昨天零点 → 昨天课程组排最前、今天组排后，"距今天最近在前"排序整体错一档。
- **测试形态**：抽纯函数 `localTodayMs(now)`（本地零点，可测）；类型校验 + 逻辑走查（与 parseDateKey 同本地零点基准）。断言脚本未扩展（target-guard-check.ts 只锚 targetGuard 模块，跨模块 import 会破坏其聚焦性）。
- **修复**：Dashboard.tsx 抽 `localTodayMs()`（`setHours(0,0,0,0)` 锁本地零点）替换 `todayMs` 计算。
- **commit**：`63bee78`

### F40-N2（MINOR）— 已修
- **缺陷**：CollapseSection `aria-controls="collapse-body"` 悬空——正文 div 无 id 落位（对读屏无效），且组件多处实例化（extras 折叠段 + 每个日期组）补固定 id 必冲突。
- **测试形态**：类型校验 + npm run build（useId 返回 string，aria-controls 与正文 id 同源唯一）。
- **修复**：组件内 `const id = useId()` → `aria-controls={id}` + 正文 `id={id}`。
- **commit**：`dc74d7a`

## 回归结果
- `cd web && npx tsc -b` exit 0；
- `npx --no-install tsx scripts/target-guard-check.ts` 10 场景全绿（原 5 + 新增 5）；
- `npm run build` 成功（tsc -b + vite build，产物落 backend/web/dist 供 //go:embed）；
- 工作区无未提交改动（仅既有未跟踪文件），backend/ 目录零改动。

## 提交清单（未 push）
| commit | 说明 |
|---|---|
| `808fae7` | F40-M1 cleanStaleSelected 随重建清理 + 守卫命中 toast（TDD 断言脚本红→绿） |
| `7618a26` | F40-M3 /state 失败降频 30s |
| `533ccde` | F40-M2 文案行吃 begin_times 兜底 |
| `dc74d7a` | F40-N2 CollapseSection useId 唯一 id |
| `63bee78` | F40-N1 localTodayMs 本地零点 |
