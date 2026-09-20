# round44 前端审查原始发现

> 审查基线：master `b275270`（R43 四修已提交：F43-M1 shouldDeferSave 增 hasSelected / F43-N1 报名按钮 btn_type 判据 / F43-N2 激活票据清空 / F43-N3 tailwindcss-animate 插件）。
> 校验：`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`cd web && npm run build` **exit 0**（构建产物 `backend/web/dist/assets/index-BvE-Yqfx.css` 由本审查代跑验证，dist/ 已被 gitignore，未污染工作树——`git status` 仍只有 5 个未跟踪根文档）。
> 范围：`web/src/` 全部 `.ts/.tsx`，绝对只读。
> 上轮（round43）四项修复核对结论：
> - **F43-M1（shouldDeferSave 增 hasSelected）——落地正确**。`shouldDeferSave(stateData, hasSelected)` 在防抖回调 676 / flushTargets 499 / handleBack 589+595 三处消费点均传对判据：防抖与 flush 用 `selectedCount > 0`（渲染闭包捕获，消费时刻只偏保守，绝不过放）、handleBack 用闭包内 `hasSelectedNow()`（读 `selectedRef.current` 活值）。全清空主路径（rev>0 + selectedCount===0 + courses 非空）三处均放行 PUT []；rev===0 纯浏览在 flushTargets 490 行已被 `latestRev===0` return 短路，防抖 effect 645 行入口 `rev===0 return`，均不受影响。TDD 断言脚本 16 项全绿。
> - **F43-N1（btn_type 判据）——方向正确、存在一条宣传/行为一致性 gap（见 N-1）**。开窗前 btn_type=2+can_select=false 的置灰「报名」按钮 `disabled:opacity-40` + `title` 悬浮层级为渲染期行为，与官网 `btn disabled` + lay-tips 语义对齐；窗口开后可点；冲刺/预选按钮（isSelected 分支）随窗口信号正常切换形态，恢复 publish 后官网按钮与冲刺按钮并存无布局分叉（各占一个 flex 子项）。
> - **F43-N3（tailwindcss-animate 插件）——落地正确**。`web/package.json` 已装 `tailwindcss-animate@^1.0.7`（devDeps），`web/src/styles/global.css:2` 在 `@import "tailwindcss";` 之后紧接 `@plugin "tailwindcss-animate";`（Tailwind 4 指定的 plugin 位置正确）。构建产物实测：`data-[state=open]:animate-in` / `fade-in-0` / `zoom-in-95` / `slide-in-from-bottom-full` 等工具类全部生成（detect-all 出现次数 > 0），且新增 `@keyframes enter / exit` 关键帧（类级 enter/exit + CSS 变量在鼠标悬停非 hover 类 `transition` 之上生效——唯一形态完全正确）。**注意点**：本审查重建产物将动态生成的新 CSS 写入 `backend/web/dist/`（gitignore，无污染），运行期产物仍为 R43 构建的旧 hash。
> - **F43-N2（激活票据清空）——落地正确且生命周期自洽**。`Login.tsx:95` 非「过期/已用」分支（激活码错）补 `setPendingTicket("")`，与 88 行过期分支对称；与后端 `handleActivate`（handler.go:199 ConsumeTicket 在激活码校验**之前**，失败即销毁票据）契约对齐。二次激活路径：失败清票 → 用户点「激活」空票必 4014 报「不能为空」→ Esc/「取消」清 pendingAccount → 重登触发新一轮 1001 下发新票覆盖。Esc 分支（233-235）、取消分支（303-305）与成功分支（80-81）三处清票，状态全生命周期无滞留。
> 上轮观察项核对结论：O-1 / O-2 / O-3 / O-4 确认仍在，本轮各给裁决见下。

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮回合无 MAJOR。M-1「清空永不落库」已被 F43-M1 关闭，详见【已核对无问题】。）

---

## MINOR（展示 / 边界一致性）

### N-1.【round43 M-1 修复衍生的新观察】开窗前 hide-away 差异化动效（`animate-in` 生效后）与「快捷操作」按钮窗口门控语义不完全一致

- **文件路径:行号**：`web/src/routes/Select.tsx:1083-1150`（操作按钮区）+ `web/src/routes/Select.tsx:1177 / Admin.tsx:212`（animate-in 手工模态）+ `web/src/styles/global.css:2`（@plugin）
- **严重级**：MINOR（展示一致性问题，功能零影响）
- **一句话问题**：F43-N3 插件生效后，四类手工模态（Select 退选 1177 / Admin 删除 212 / Login 激活 223）都挂 `animate-in fade-in duration-150` → 均获得差异化渐入动画；而卡片操作按钮区内：
  - 开窗前置灰「报名」走 `disabled:opacity-40`（无过渡），「设为预选目标」（primary）与「设为后台冲刺目标」（ghost）切换时无动画；
  - 同一渲染分支里，官方原样按钮与项目冲刺/预选按钮并存，前者无动效、后者无动效——整个开窗过渡期卡面「官方结构零变化 + 项目按钮换文字换色」，F43-N1 注释声称「官网按钮以 btn_type 为唯一渲染判据」只解决了**渲染条件**。
- **触发场景推演**：开窗瞬间平台 publishes 短暂清空→恢复，恢复后 présence-card `btn_type=2+can_select=false` 置灰的同时「设为预选目标」主按钮 toggles 为 ghost——两套按钮同一卡面并存发展，动画缺席只是纯视觉，「开窗瞬间首页微过渡」无行为风险。真正要注意的是一致性方向：官网逆向契约里 `can_select=false` 的按钮是**静态 disabled**（无移动端过渡），本贡献的 `disabled:opacity-40` 配合 Footer 注释是对齐的；唯一「不完整」的是**任何一处都没把二级提示（title→lay-tips）做成 hover 浮层**，而是依赖原生 title。行为无错，纯展示层级观察。
- **建议修法**（点到为止，非本轮必需）：无需修。如未来想统一动效语言，可考虑给禁用单选做 CSS `transition:opacity .15s`，或给 replace 保持原生 title。本轮按 MINOR 记录而非 MAJOR——无用户可感知的功能挣扎，且已在注释里声明了部分（1177/212 的淡入）。

（注：N-1 隐含的 subtler 层）**开窗前置灰按钮的悬浮提示**：`Select.tsx:1109` title 回退 `"不在选修报名时间范围内，无法选课！"`，与 CLAUDE.md 实证文案一致，且 platform 下发的 `title` 字段为空时不产生任何提示（与前端降级语义同源）。这不是问题。

### N-2.【round43 N-1 复证升级为落地对比】「官网 btn_type 按钮 + 冲刺/预选按钮」同卡双形态并存，与官方逆向契约的"单操作按钮"比多一个按钮

- **文件路径:行号**：`web/src/routes/Select.tsx:1083-1150`
- **严重级**：MINOR（并发主冲突——官网按钮逻辑对，双按钮是功能增强。行为正确性已由编解码确认）
- **一句话问题**：F43-N1 打开官网按钮后，F42 时代就存在的「设为后台冲刺目标」（开窗后收敛为 ghost 小按钮）继续在**同一张卡上**与官网「报名/退选」并存。官网逆向契约（select.js 逆向行）是每行一个操作按钮（btn_type 1/2 决定一个）；本项目现在每行最多两个按钮（官网 + 冲刺）。
- **触发场景推演**：这是本项目"自动抢课 + 手动通道"的双轨设计，官网按钮是手动通道、冲刺按钮是预选目标规划通道，**两者使命不同、并存合理**；唯一要确认的是持久化其实有两条：官网按钮走 `selectElective`（立即报名），冲刺按钮写 `selected`（目标计划）——二者写入后端**互不冲突**（前者是选课记录，后者是目标集合），无数据分叉。布局上两个按钮都 `w-full` flex 各占一行，无重叠/分叉。
- **建议修法**：无需修，这是设计意图。唯一值得一提的：冲刺按钮在**窗口开放后**文字「已设为后台冲刺N」+ ghost，官网按钮「报名/退选」primary——两个按钮视觉权重相当，主操作（官网）与规划（冲刺）容易混淆。可选：让冲刺按钮在 `window_opened=true` 时更弱化（纯文字下链）。

### N-3.【O-4 复证】App.tsx 401 处理器管理员代理态保护分支仍依赖闭包 `current`，与前置代理复位语义叠加后存在保守保护窗口

- **文件路径:行号**：`web/src/App.tsx:174-176`
- **严重级**：MINOR（保守方向、注释有意图声明，无触发实证——续报观察）
- **一句话问题**：`if (current === adminName && inAdmin && lostAccount !== adminName) return` 在管理员于管理页正常操作时收到学生 401 事件会整体跳过剔除（含落盘与 setState）。该事件大概率来自陈旧响应（学生页不在场），跳过可避免误杀事件竞态；但学生会话若在被代理期间失效，会一直残留到学生端下次真正请求时才被剔除。
- **核对结论**：端到端追查后确认是**保守方向**，非漏洞——`lostAccount` 判据经 `loadSessions()` 反查（同令牌/账号名），代理分支下事件总带「管理员令牌」，`lostAccount===adminName` 已在上一行被排除，此处 return 恰好兜住「陈旧学生事件误杀管理员会话」竞态。维持 MINOR（保守窗口存在，属取舍）。

---

## OBSERVE（观察项，未加重）

### O-1.【round40 N-3 / round41 O-1 / round42 O-1 / round43 O-1 延续】防抖 effect 入口 `resetRetry()`（Select.tsx:648）仍清零指数退避——非用户动作（/state 数据到达、echoDone 置位、发布重建）也会清零退避，网络抖动中"重发退避长不大"

- **文件路径:行号**：`web/src/routes/Select.tsx:648`
- **一句话问题**：F43 未触及。建议仅在 pick()/清空操作调用 resetRetry，观察项维持（行为无错，仅与 n14「指数退避」承诺分叉）。

### O-2.【延续】`useTickingCountdown`（lib/useTickingCountdown.ts:8-12）每秒 `setNow` 驱动消费页整帧重渲染

- **文件路径:行号**：`web/src/lib/useTickingCountdown.ts:8-12` + Select.tsx:749 / Dashboard.tsx:175/183
- **一句话问题**：Select 数百卡片、Dashboard 日期分组每 1s 全量重建 JSX，与注释「只重渲染倒计时一处」承诺分叉。Dashboard 注释（90-92 行）自认依赖整页重渲染驱动相对时间戳，纯性能。延续观察。

### O-3.【延续】三处手写模态（Select 退选 / Admin 删除 / Login 激活）无焦点陷阱——背景可 Tab 穿出

- **文件路径:行号**：`Select.tsx:1175-1224 / Admin.tsx:210-284 / Login.tsx:223-316`
- **一句话问题**：F7-03 补 role/aria/Esc、F20/F21 补进行中防误关，但 backdrop 外内容仍可聚焦。F43-N3 使桌面端淡入生效后，撑起背景的 backdrop 与手势正常。延续观察。

### O-4.【O-4 复证延续｜降级】App.tsx 401 管理员代理态保护窗口

- **文件路径:行号**：`web/src/App.tsx:174-176`
- **一句话问题**：与 N-3 不同，O-4 是对「保护窗口」本身的观察（学生会话失效残留到下次请求）。端到端确认无数据风险（学生页不在场时无请求），保守方向成立——维持 OBSERVE 即可。

---

## 已核对无问题的重点区域（铁证）

1. **F43-M1 全链**：`shouldDeferSave(stateData, hasSelected)`（targetGuard.ts:57-62）三消费点判据与数据源同源（`stateDataRef.current` 消费时刻快照）：防抖回调 `676` 传 `selectedCount`、flushTargets `499` 传 `latestSelectedCount>0`、handleBack `589` 与 `595` 传 `hasSelectedNow()`（读 `selectedRef.current` 活值）。全清空主路径（rev>0 + selectedCount===0 + courses 非空）三处均放行 PUT []；**rev===0 纯浏览**在 flushTargets 490（`latestRev===0 return`）与防抖 effect 645（入口 return）双层跳过，绝不受影响。TDD 断言脚本 16/16 全绿（含「首帧未到+全清空→推迟」「courses 非空+全清空→放行」两分叉）。安全方向残余：首帧未到（stateData===undefined）+ 全清空 仍推迟（占位，等首帧确证无旧目标），与设计一致。
2. **F43-N1 渲染判据**：`btn_type===1` 退选 / `===2` 报名独立渲染，`disabled={actionLoading.has(c.id) || !c.can_select}` 双守卫，title 回退文案与官网实证 CSR 一致；`===0`/其他值不渲染官网按钮、仅有冲刺/预选按钮——与官网逆向「其他值不渲染按钮」对齐。与后端 `zhidao.Class.BtnType`（client.go:493，int）+ handler_test 夹具（btn_type:2 组合 can_select 各态）契约一致。**「开窗前 btn_type=2+can_select=false 置灰报名按钮」不遮挡预选按钮**——两者 `w-full` 各占一行，可同时操作。
3. **F43-N3 插件**：`global.css:2 @plugin "tailwindcss-animate"` 位置正确（紧跟 `@import "tailwindcss"`）；产物实测 `data-[state=open]:animate-in`、`fade-in-0`、`zoom-in-95`、`slide-in-from-bottom-full` 等工具类齐全，且新增 `@keyframes enter/exit`；`.animate-in{animation-name:enter;animation-duration:.15s}` 生效。twMerge 不会砍掉这些类（实测合并保留）：Dialog（组件未在业务消费，纯 shadcn 准备）、Sheet 同理、Toast（消息退场 slide-out-to-right-full）。`duration-200/300` 与 `transition-all` 与动效类不冲突。
4. **F43-N2 票据生命周期**：`Login.tsx` 四个清票点（成功 80-81 / 票据过期 88 / 其他失败 95 / Esc 233-235 / 取消 303-305）对称关闭，后端 handler.go:199 ConsumeTicket 失败即毁票契约对齐——不存在残留旧票触发「票据已用」误导。
5. **round43 无继承性回归**：M-1 清空放行、N-1 btn_type 判据、N-2 票据清空、N-3 动效插件落地全部核对属实。round42 的 F42-M1/M3/N1 继续正确（防抖 effect 依赖组、独立清理 effect 依赖 `[publishes, selected, echoedRef, toast]`、`/electives` 失败态 30s 降频 + 缓存 window_opened 升频、`/state` refetchInterval 失败 30s）。
6. **其余**：targetGuard 纯函数三件、Echo 合并（rev>0 && !anyHas 全清空不复活）、F15/F16/F17 假清空守卫链、`ReadonlySet<number>` 在飞幂等、unmountedRef 双向、401 归属 session 优先、App 快照式三连落盘、Dashboard 本地零点分组与 begin_times 兜底、admin stats 三态——均正确。

---

## 结论

- **MAJOR 0 / MINOR 3 / OBSERVE 4**，共 7 条（上轮 1 MAJOR 修复后归零；MINOR 由 3 降级为 3——M-1 关闭、N-1 新开、N-2 复证升级、N-3 复证）。
- 最重 3 条（按功能影响排序）：
  1. **N-1**（开窗过渡期动效语言不统一，纯视觉一致性，功能零风险——已核对无 MAJOR）。
  2. **N-2**（双按钮并存是设计意图，无数据分叉；仅视觉权重混淆，可选微调）。
  3. **N-3**（401 管理员保护窗口保守方向，端到端确认无数据风险，维持观察）。
- F43 四件套全部落地正确，本轮无新的功能级错误；圆了 F43 的目标——「F43-M1 后全清空永不落库」的 MAJOR 已关闭，「正式清空」主路径齐放行。若后续精力允许，唯一值得动的是动效与双按钮视觉层的一致性微调（N-1/N-2），均为零风险 polish。