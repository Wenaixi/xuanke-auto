# round39 前端修复报告（TDD 严格模式）

修复人：round39-frontend（只碰 web/ 目录，未动 backend/）
日期：2026-09-20

## 修复清单

### F39-C1（CRITICAL，数据丢失）— 已修
- **缺陷**：目标保存防抖/flush 链对"selected 残留旧 publish_id 非空条目"放行，build() 只产出新发布课程，整包 PUT 覆盖删除后端已保存目标。
- **测试形态**：纯函数断言脚本 `web/scripts/target-guard-check.ts`（`npx --no-install tsx scripts/target-guard-check.ts`，5 个场景）。TDD 红绿：守卫初始实现恒 false → 场景 A（旧发布 P1 残留 + 新发布 P9 新课）断言失败（exit 1）→ 恢复真实判据全绿。
- **修复**：新增可导出纯函数 `web/src/lib/targetGuard.ts` 的 `selectedHasStalePublish`（空数组 key = 用户清空，绝不判过期），接入 Select.tsx 防抖回调（594-601 前置守卫）与 flushTargets（443-449 前置守卫）两处消费时刻构建之前；命中即 `dirtyRef.current = true` 置脏跳过，绝不产出可 PUT 的目标。与回显 effect 的 currentIds 过滤同判据。
- **commit**：`11c865b`

### F39-M1（MAJOR）— 已修
- **缺陷**：`onDeleted` 删除管理员自己时未退出管理态，inAdmin 残留 true → 学生令牌渲染 Admin 五 Tab 连环 401。
- **测试形态**：类型校验 + 渲染分支逻辑走查（与 onUnauthorized 166 行对称）。
- **修复**：App.tsx onDeleted 补 `if (acct === adminName) setInAdmin(false)`。
- **commit**：`9c50626`

### F39-N1（MINOR）— 已修
- **缺陷**：主倒计时 `useTickingCountdown(openTimeStr)` 未吃 begin_times 兜底，识别槽未建立时"主矩阵全 00 + 预计开放时间显未来"自相矛盾。
- **测试形态**：类型校验 + 逻辑走查。
- **修复**：Dashboard.tsx 倒计时输入改为 `openTimeStr ?? (begin_times[0] 存在 ? new Date(...).toISOString() : null)`。
- **commit**：`1fdb83c`

### F39-N3（MINOR）— 已修（含对后端的依赖）
- **缺陷**：管理后台"运行状态"无窗口关闭信号。
- **测试形态**：类型校验（types.ts AdminStats 补 `window_closed?: boolean`）+ 逻辑走查。
- **修复**：Admin.tsx StatsTab 窗口状态行三态 `window_closed ? "已关闭" : window_opened ? "已开放" : "待命中"`。
- **依赖后端**：`/api/admin/stats` 当前不下发 `window_closed` 字段，字段未定义时走"待命中"（安全，不假报关闭）。**需要后端修复代理在 handler.go stats 补发 `window_closed`（统计口径与学生端 /state 同源）后，本前端三态才显示"已关闭"**——前端容错已就绪。
- **commit**：`013b51d`

### F39-N4（MINOR）— 已修
- **缺陷**：硬编码文案"单次熔断冷却"与后端失败分级退避分叉。
- **测试形态**：纯文案，无测试。
- **修复**：Dashboard.tsx 452-455 改"分级退避 · 自动恢复"。
- **commit**：`3749fdf`

### F39-N5（MINOR）— 已修
- **缺陷**：`accounts = Object.keys(sessions)` 每次渲染新建数组 → effect 每次渲染重跑（反模式）。
- **测试形态**：类型校验 + npm run build。
- **修复**：App.tsx `const accounts = useMemo(() => Object.keys(sessions), [sessions])`（导入 useMemo）。
- **commit**：`c7627a4`

## 回归结果
`cd web && npx tsc -b && npm run build` 全绿（tsc exit 0；vite build 成功，产物落 backend/web/dist 供 //go:embed）。守卫断言脚本 `target-guard-check.ts` 5 场景全绿。

## 提交清单（未 push）
| commit | 说明 |
|---|---|
| `11c865b` | F39-C1 前置守卫 + TDD 断言脚本 |
| `9c50626` | F39-M1 删管理员退管理态 |
| `1fdb83c` | F39-N1 主倒计时 begin_times 兜底 |
| `013b51d` | F39-N3 窗口三态（含类型） |
| `3749fdf` | F39-N4 文案对齐 |
| `c7627a4` | F39-N5 accounts useMemo |
