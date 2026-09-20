# round55 前端只读审查发现报告

> 审查基线：master @ `93e07ef`（R54 收官，工作树预期仅根目录 5 个未跟踪社区文档 + archive/review-rounds/ 内 R54 相关文件）。审查期间绝对只读（未创建/修改/删除任何文件，唯一新建文件为本报告）。
> 校验实况：`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`cd web && npm run build` **成功**（tsc -b + vite 全绿，1948 modules，产物 417.54 kB js / 41.07 kB css，构建后 `git status --porcelain -- web/` **零输出** + `git show 93e07ef:web/src/routes/Select.tsx` 与工作树逐字节一致）；TDD 断言脚本 `npx jiti scripts/target-guard-check.ts` **16 项全绿**、`admin-auth-check.ts` **6 项全绿**、`unauthorized-check.ts` **5 项全绿**。
> 范围 `web/src/` 全部 `.ts/.tsx`（18 文件）；后端契约对照 `backend/internal/api/handler.go`（handleAdminStats 865-954 open_time_set 与 open_time 同源 / window_closed 935-952）、`backend/internal/scheduler/scheduler.go`（StateForAccount 704-730 / openTimeForLocked 715-721 / WindowClosed 903-934 / probe 探测量变入账 1097-1159 / tick 提交守卫 1003-1031）。
> Radix 运行时实证：`web/node_modules/@radix-ui/react-toast/dist/index.js`——ToastViewport 121-258（wrapper=Branch + viewport=Primitive.ol + FocusProxy 对，pointerEvents 由 `hasToasts ? void 0 : "none"` 控制、wrapper 上挂 pointermove/pointerleave/焦点暂停恢复）、ToastImpl 339-454（duration effect 402-404：`[open, duration, ...]` 依赖变化即 clearTimeout 重启 startTimer）。

## 本轮结论先行

**MAJOR 0 / MINOR 0 / OBSERVE 8（4 条新视角结论 + 4 条延续/上轮项）。** R54 前端五修（M-A 横幅分支 / M-B duration 合并 / M-C viewport 上限 / M-D 右栏兜底 / M-E open_time_set）**全路径复核通过（5 条核验零缺陷）**；五修未引入任何保存链回归（F43/F42/F40/F39/F36/F48-M1 六件套逐字符复证零回归，targetGuard.ts 未被触碰）。

本轮无 MAJOR、无 MINOR——R54 修复的语义与边界全部正确；新视角 A（viewport 滚动）的核心结论是"修复达成目标但存在 Radix 官方 pointer-events 模式下的自然交互残余（滚轮需悬停在 toast 上、滚动条不可拖、无自动滚动到底部）"，定 OBSERVE 而非 MINOR（F48-M1 的"仅 toast 可交互"设计本意如此）。

**最致命 3 条（按影响排序）**：本轮实际无致命缺陷；前三优先关注方向见文末结论节。

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮无 MAJOR。）

---

## MINOR（展示 / 边界一致性 / 协议冗余）

（本轮无 MINOR。）

---

## OBSERVE（观察项，未加重）

### O-1.【新视角 A 结论】Toast viewport 滚动交互残余——滚轮需悬停 toast 上、滚动条不可拖、无自动滚动到底部

**一句话问题**：M-C 修复（`max-h-[80vh] overflow-y-auto`）达成"堆叠超限可滚动查看旧 toast"的目标，但 Radix 官方 pointer-events 模式（viewport `pointer-events-none` + Root `pointer-events-auto`，F48-M1 有意为之）下存在三种自然交互残余。

**证据链**：
1. `index.js:121-128`：wrapperRef 挂在 DismissableLayer.Branch（外层 div）、viewport ref 挂在 Primitive.ol（className 落点）——`pointer-events-none` 与 `overflow-y-auto` 同落 ol。
2. CSS 命中：鼠标悬停 toast 卡（`pointer-events-auto`）滚轮 → wheel 事件 target 命中 toast → 冒泡到最近的 scrollable 祖先 = viewport（overflow-y-auto）→ **toast 列表滚动可用**。
3. 鼠标悬停卡片间隙（viewport 的 p-4/gap-2 区域，pointer-events:none）滚轮 → hit-test 跳过 viewport → 事件穿透到下层页面 → **页面滚动而非列表滚动**。
4. 滚动条拖动：overflow-y-auto 的滚动条轨道拖动依赖 pointer 事件在 viewport 上命中——pointer-events:none → **滚动条不可拖**（Windows overlay 滚动条 hover 出现但按住无效）。
5. 无自动滚动：Radix ToastViewport 不管理滚动位置（源码无 scrollTo 逻辑），新 toast 追加在 flex-col 底部时用户需手动滚上去查看——但 toast 3.5s 自动消失、同刻并存 >4 条概率极低（M-2 去重 + 7 个 destructive title 实测并存上限）。
6. Radix duration 暂停机制受 pointer-events 影响：wrapper（Branch）挂 `pointermove→handlePause`（index.js:165-168）——鼠标悬停 **toast 上**时 pointermove 命中 toast（auto）→ 冒泡到 wrapper（auto）→ 暂停计时 ✓；悬停间隙穿透时不触发暂停（F48-M1 设计的一部分，非缺陷）。
7. 替代方案对比：任务书提到的"限 4 条挤最旧"会牺牲"查看旧 toast 内容"（dirtyRef 保留 + 失败文案可读性），当前滚动方案更优——**不建议替换**。
8. 影响：纯交互便利性（用户需把鼠标悬停到 toast 上滚），无数据破坏、无保存链影响。维持 OBSERVE。

### O-2.【延续】useTickingCountdown 非法日期串防御（R54 OBSERVE 延续）

**复核结论**：`new Date(非法串).getTime()` → NaN → diff=NaN → `NaN<=0` false → `Math.floor(NaN/1000)` → `pad(NaN)` → **"NaN" 四位 + isExpired=false**。`diff<=0` 分支建议改 `!Number.isFinite(diff) || diff <= 0`（安全方向）。当前调用点全为合法输入（Dashboard 190-195 / Select 749-754：openTimeStr 来自后端 `2006-01-02 15:04:05` 格式化、begin_times 走 `new Date(number).toISOString()`）。零触发、全源受控，一行防御收益极低——维持 OBSERVE 不动（ponytail 权衡：不值得加）。

### O-3.【延续】手写模态无焦点陷阱 + F6-02 锚点注释
三处手写 modal（Login 224-316 / Select 1182-1231 / Admin 210-284）均有 role=dialog/aria-modal + Esc 关闭 + autoFocus，但均无完整焦点陷阱（Tab 可逃出弹层）；grep `Radix Dialog|F6-02` 仅命中 Login.tsx:222 一处锚点注释。维持 OBSERVE。

### O-4.【延续】useTickingCountdown 每秒整页重渲（量化收敛可接受）+ O-1 Admin 单查询轮询
同 R53/R54 结论：React bailout 使叶子不重渲、实际 DOM 写入仅倒计时文本节点；Radix TabsContent 懒渲染（`present && children`）实证非激活 Tab 查询无观察者不轮询——管理员停留任一 Tab 时轮询 = 该 Tab 单查询（O-1 修正持续成立）。维持 OBSERVE。

### O-5.【延续】Dashboard key 不对称 + expandedDates 跨账号残留
App.tsx:332 Dashboard 挂载点仍无 `key={account}`（Select 两处 App.tsx:294/340 有 `key={account}`）——账号切换原地重渲，`expandedDates`（Dashboard.tsx:268）残留旧账号折叠态。纯展示层。维持 OBSERVE。

### O-6.【延续】ui 模板残宽：CardFooter 零消费 + Button/Badge 死变体
grep 复证：`CardFooter` 仅 Card.tsx:46/51 自身定义，全仓零消费；Button 12 变体（Button.tsx:7）实际 `variant=` 传值仅 primary/outline/ghost/dark/destructive 子集、Badge 8 变体（Badge.tsx:5）实际仅 primary/outline 子集。维持 OBSERVE（组件本体被消费，清剿收益低）。

### O-7.【延续】xk_admin_token/xk_admin_name 多标签页生命周期
无 storage 事件监听；多标签页共享 localStorage 的 B 标签登出清标记 → A 刷新回学生端。方向安全（永不误进管理页）。维持 OBSERVE。

### O-8.【延续】O-2 401 保护窗口闭包 current + N-2 双形态按钮
App.tsx:188-235 onUnauthorized 闭包捕捉 current、effect 依赖 [adminName, inAdmin, adminToken] 重建刷新闭包，保守方向确认；Select.tsx:1090-1156 官网 btn_type / 冲刺双轨按钮设计意图确认。维持观察。

---

## 重点核对结论（任务书逐条裁决）

### 1. R54 前端五修全路径复核（5 条核验）

| 核验点 | 证据链 | 裁决 |
|---|---|---|
| ①　M-A 横幅分支 `!openTimeStr && data?.begin_times?.[0] == null` | Select.tsx:821 三态分界：`openTimeStr 缺席 && begin_times[0] 缺席` → "未识别"；`openTimeStr 缺席 && begin_times[0] 存在` → 落到 `cd.isExpired` 分支（826，此时 cd 已吃 begin_times 兜底，未来点 → isExpired=false → 确切倒计时）；分支顺序 `!stateData → window_closed → window_opened → !openTimeStr&&begin_times缺 → cd.isExpired → 倒计时` 与 Dashboard F15-03（window_closed → window_opened 前置优先）完全同构；`!openTimeStr && begin_times==null` 与矩阵兜底条件（750 `data?.begin_times?.[0] != null`）**互补同源**。开窗瞬间短暂空快照时 window_opened 前置分支优先——完全正确 | **正确** |
| ②　M-B duration 合并保留首值 | Toast.tsx:50 合并分支 `{...prev[idx], description, variant}` 不再携带 duration → 保留 `prev[idx].duration`（首次挂载配置）；Radix 实证 index.js:402-404 `useEffect(... , [open, duration, ...])`——duration prop 不变 → effect 不重跑 → 计时器不因合并重启 → 无限延寿语义根除；新增分支 `{...msg, id}`（54）携带新 duration 生效；全仓 grep `duration:` 在 toast 调用点零命中（tokyo 15 个调用点全字符串 title + 无 duration 传参）→ 默认 3500 恒生效 | **正确** |
| ③　M-C viewport `max-h-[80vh] overflow-y-auto` | Toast.tsx:105 加在 viewport className；Radix viewport = Primitive.ol（index.js:242），`flex flex-col gap-2 p-4` + `overflow-y-auto` 标准可滚动集合；`pointer-events-none`（viewport）+ `pointer-events-auto`（Root 78）下滚动交互见 O-1（悬停 toast 滚轮可用）；overflow-y 单轴设置导致 overflow-x 计算为 auto，但 toast `w-full` + viewport `max-w-[380px]` 无横向溢出（gap/padding 不被 overflow 裁剪，padding-top 在 flex-col 顶部、padding-bottom 含滚动内容内） | **正确（交互残余 O-1）** |
| ④　M-D 右栏兜底 + 空态安全 | Select.tsx:838-842：`stateData?.open_time_known && openTimeStr ? 识别值 : data?.begin_times?.[0] != null ? new Date(...).toLocaleString : "未知"`——`data?.begin_times?.[0] != null` 数组空 → undefined → null 兜底；`new Date` 只在非空分支执行；与 Dashboard.tsx:407-414 文案行同构（F39-N1 配套） | **正确** |
| ⑤　M-E `open_time_set !== true` 保守未识别 | Admin.tsx:714 `s.open_time_set !== true ? "未识别" : s.open_time`——undefined/false → "未识别"、true → open_time；types.ts:126 可选声明与消费兜底一致；后端 handler.go:950-952 恒下发四键且 `open_time_set` 与 `open_time` **同源**（同一 `!open.IsZero()` 计算，886-889 + 952）→ "已识别但 open_time 空串"边界不可能出现 | **正确** |

### 2. R54 五修后保存链/Toast 语义回归复证（六防逐字符零回归）

| 防护族 | 复证结论 |
|---|---|
| **F43 全清空 ≠ 数据缺席** | targetGuard.ts:57-61 `shouldDeferSave(stateData, hasSelected)` 双参三消费点（防抖 676 / flushTargets 499 / handleBack 589）R54 commit（仅改 Toast/Admin/Select 三处 12 行）未触碰 targetGuard.ts——逐字符零回归；16 项断言全绿 |
| **F42-M1 判据与数据源解耦** | targetGuard.ts 纯数据判据 + 防抖 effect 依赖含 stateData（Select.tsx:737）——未触碰。零回归 |
| **F40-M1 targetGuard 三角** | selectedHasStalePublish/cleanStaleSelected 消费点（回显 287 / 独立清理 316-327 / 防抖 696 / flush 518）均未触碰。零回归 |
| **F39-C1 selectedHasStalePublish** | 空 key 绝不判过期（targetGuard.ts:9-19）。零回归 |
| **F36-01 key={account} 双保险** | App.tsx:294/340 双 keyed + Select accountKey 守卫（195-202）声明于 echoedRef/rev 之后 TDZ 安全。零回归 |
| **F48-M1 Toast 定位** | Toast.tsx:105 viewport `fixed bottom-4 right-4 z-50` 定位类未动；M-C 仅追加 max-h/overflow。零回归 |
| **Toast 去重语义** | 去重 key = `typeof title === "string"`（字符串全 15 调用点）；合并保留原 id → React key 稳定不重建；变体取最新（视觉反馈即时）——R54 M-B 只去掉 duration 覆盖，其余语义未动。零回归 |

### 3. 本轮新视角六项——逐条裁决

- **A. Toast viewport 滚动交互深度核** → 见 O-1：滚轮悬停 toast 可用、间隙穿透（F48-M1 设计产物）、滚动条不可拖、无自动滚动到底部——四类残余均属 Radix 官方 pointer-events 模式的自然边界，替代方案（限条数挤最旧）更差。**滚动可用，观察级**。
- **B. Select 横幅/矩阵/右栏三处 begin_times 兜底一致性** → 矩阵（750 `data?.begin_times?.[0] != null`）/ 横幅（821 `== null` 互补同源）/ 右栏（840 `!= null`）判据完全一致，window_closed 空快照（begin_times nil）时三处同步回退（全 00 + 未识别 + 未知）。**无缺陷**。
- **C. Dashboard/Select 双页倒计时终态一致性** → 识别建立瞬间（openTimeStr null→值触发 cd effect setNow 校正 F10-07）、识别过期瞬间（StateForAccount OpenTimeKnown=false → openTimeStr=null → 兜底 begin_times，过期 → isExpired=true"本地已到开窗点"）、窗口关闭瞬间（window_closed 前置分支优先 + 双页 /state 各自轮询降频回 30s，路由互斥不同时挂载）——三态跳变两页同构同步。**无缺陷**。
- **D. AdminStats open_time_set 消费点全量清点** → grep `open_time_set` 仅 types.ts:110-111/126（声明注释）+ Admin.tsx:714（StatsTab 唯一消费点）；`!== true` 语义（undefined/false → 未识别）与 types.ts 可选声明匹配；`=== false` 曾在 R54 前把 undefined 误判"已识别"——已修；后端同源保证"已识别但 open_time 空"不存在。**无缺陷**。
- **E. types.ts 契约 vs 消费点全量清点** → 逐一核：`window_closed?`（Dashboard 342-345/728、Select 811、Select refetchInterval 80、Dashboard 143/159——全部三元缺省 undefined→false 方向）；`token_valid?`（Admin 756 `Object.values(s.token_valid ?? {})` 正确兜底、Dashboard 489 `state?.token_valid === false` 缺省视有效，与 types.ts:128 注释一致）；`captcha_engine?`（Admin 726 `=== "ddddocr"` undefined→Vision，与 handler.go:925-928 空串兜底 vision 一致）；`captcha_concurrency?`（727 `?? 1`）；`open_time_set?`（M-E 已修）；ElectivesData `begin_times: number[]` 必选但空数组解构 `[0]` undefined → 全消费点 `!= null` 兜底。**全部有缺省兜底**。
- **F. useTickingCountdown 非法日期串防御** → 见 O-2：NaN → "NaN" 四位 + isExpired=false；当前调用点全为受控合法输入（后端格式化 / 毫秒数）。零触发，防御一行收益极低。**维持 OBSERVE**（ponytail：不值得加）。

---

## 上轮观察项延续复核表

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| N-2（官网 btn_type + 冲刺按钮双形态） | MINOR 观察延续 | Select.tsx:1090-1156 双轨设计意图确认 | 延续 |
| N-3（App 401 保护窗口闭包 current） | MINOR 观察延续 | App.tsx:188-235 闭包 + deps 重建确认，方向安全 | 延续 |
| O-2（手写模态无焦点陷阱） | OBSERVE | 三处 modal 均无焦点陷阱，F6-02 锚点仅 Login.tsx:222 | 延续 |
| O-3（401 保护窗口数据面） | OBSERVE | targetAccount 清代理态 + lostAccount 反查正确 | 延续 |
| O-1（Admin 单激活查询轮询） | OBSERVE 修正确认 | Radix TabsContent `present && children` 懒渲染实证延续成立 | 确认延续 |
| O-2（useTickingCountdown 每秒重渲） | OBSERVE | 量化 bailout 可接受 + React 19 语义 | 延续 |
| O-5（Dashboard key 不对称） | OBSERVE | App.tsx:332 无 key + expandedDates 残留 | 延续 |
| O-6（ui 模板残宽） | OBSERVE | CardFooter 零消费 + 死变体 grep 复证 | 延续 |
| O-7（多标签页） | OBSERVE | 无 storage 同步，方向安全 | 延续 |
| O-8（F51-O1 注释残留） | OBSERVE | 仅 Login.tsx:222 一处 | 延续 |
| F 非法日期串（R54 O-1） | OBSERVE | 零触发、全源受控 | 延续（O-2） |

---

## 结论

- **MAJOR 0 / MINOR 0 / OBSERVE 8（O-1 新视角结论 + O-2~O-8 延续）**。tsc exit 0、build 成功（1948 modules / 417.54 kB js / 41.07 kB css）、构建后 web 零 git 变更、TDD 断言 16+6+5 全绿、源码与 HEAD 逐字节一致。
- **最致命 3 条（按影响排序）**：
  1. （本轮无真实缺陷）首要关注 = **O-1：Toast viewport 滚轮需悬停 toast 上才滚动、滚动条不可拖**——M-C 修复达成防堆叠目标，残余交互为 Radix 官方 pointer-events 模式的自然产物，无需动作，仅记录边界供未来迭代（若用户反馈"滚不到旧 toast"再评估限条数方案）。
  2. **O-2：useTickingCountdown NaN 防御**——当前零触发，维持观察；若未来开放时间来源从后端格式化/毫秒数扩到不可信字符串源，先补 `!Number.isFinite` 再上线。
  3. **O-5：Dashboard 挂载点无 key={account}**——expandedDates 跨账号残留，纯展示层；若做切号视觉一致性整治，一行加 key 即可。
- **已核对无缺陷的高风险区域**：R54 五修全路径（5 条核验含 M-B duration 计时器机制、M-E 后端同源保证）；F43/F42/F40/F39/F36/F48-M1 六防保存链零回归；begin_times 三处兜底同源一致性；双页倒计时三态终态同步；types.ts 全部可选字段消费缺省兜底（open_time_set/token_valid/window_closed/captcha_engine/captcha_concurrency 逐一清点）。
- **建议优先修复方向**：本轮零修复项，全部 OBSERVE 无需动作；下轮重点可转向（若做体验整治）O-1 的"悬停穿透"提示（可选加 `title` tooltip 引导）或 O-5 的 Dashboard key 对齐。R54 收官后前端防线持续稳定。