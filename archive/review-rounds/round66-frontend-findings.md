# review-round66 前端发现（只读审查，基线 b1742a6）

## 概述

M-1 修复**第三轮确认：仍然闭合，无新缺陷**。本轮把确认从「实现层端到端证据」（R65）推进到**亚帧窗口穷举推演 + React 状态队列语义最小构造验证**，找到的仍然只是两个「对象式 setState 覆盖在飞函数式 updater」的亚帧竞态（保守方向：至多一次重试，绝不丢后端旧目标）——属于稳态瓶颈而非新缺陷，维持 OBSERVE 级别。六防保存链逐字符零回归；新发现 1 条 OBSERVE（audit.mjs 违例报告与代码实际命名不一致）。tsc+build 全绿；target-guard 断言 18/18、admin-auth 6/6、unauthorized 5/5、audit 1 违例（下述）。仓库工作区零污染（只读铁律遵守）。

## M-1 第三轮闭合确认

### 核证路径清单（全部逐段核读 + 实测）

1. **`shouldDeferSave` 签名与实现**（web/src/lib/targetGuard.ts:64-71）：`(stateData, hasSelected, echoed)` 三参齐全，与 R63 规范逐字符一致。首行 `undefined → true` 恒先于 `echoed`，优先级保护成立（首帧未到绝不放行）。

2. **三消费点都真传 `echoedRef.current`**：
   - 防抖 effect `Select.tsx:686`：`shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)` ✔
   - `flushTargets` `Select.tsx:501`：`shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)` ✔
   - `handleBack` 判定 `Select.tsx:594` + `while` 等待循环 `Select.tsx:602`：两处均传 `echoedRef.current` ✔（判定与循环同参，无判定放行后循环又拦回的分叉）

3. **echoedRef 全部置位路径穷举**（三处置位 + 一处不置位）：
   - `Select.tsx:238-239` courses 空分支：置 `echoedRef.current=true` + `setEchoDone(true)`。courses 空 = 后端确证无旧目标，置位后 selected 只含用户改动，整包 PUT 与后端一致 ✔
   - `Select.tsx:295-296` 合并完成分支：置位在**全部合并/清理完成之后**。置位时 selected 已含后端旧目标 ✔
   - `Select.tsx:200` account reset：置 `false`（复位）。账号切换整组件 key 重建，此分支仅兜底 ✔
   - `Select.tsx:232` 首帧未到：`stateData === undefined` 直接 return，**绝不置位** ✔（注释与 R63 承诺一致）
   - 唯一边界：`pubs.length === 0 && courses 非空`（Select.tsx:245）不置位、不合并、不清理。该边界下 shouldDeferSave 走 `echoed=false` 分支恒推迟 + handleBack 5s 兜底后放行——保守方向正确（与 R64 判定一致，本轮维持）。

4. **反向推演「echoed=true → selected 必完整」**：
   - 三条置位路径的 selected 完整性逐条验证通过（见上）。
   - **新发现的亚帧窗口**（反向推演首次穷举到，见 OBSERVE-66-03）：echoed 置位（effect 同步段）与合并 setSelected 提交（渲染帧）非原子，同一事件循环拍内存在「守卫读 echoedRef 已为 true、但合并结果尚未提交」的微秒级窗口；同一拍内又叠加「回显合并是函数式 updater、用户点击 pick 用渲染闭包 selected 发对象式 setState，队列序上对象式覆盖在飞合并 updater」的覆盖窗口。验证结论：机制上存在，但被下游五道防线兜住，全部保守方向（详见 OBSERVE-66-03）。

5. **target-guard 断言脚本 18 条全绿**：`node --import jiti/register scripts/target-guard-check.ts` → `✓×18 + target-guard 断言全绿`（本轮起统一用 `jiti/register` 子路径运行——README 头注释的 `node --import jiti` 在 jiti 2.7 下抛 ERR_MODULE_NOT_FOUND，见 OBSERVE-66-02）。

### 判定

**M-1 第三轮确认：闭合。** 依据：
- 三消费点、全部置位路径、首帧不置位边界逐段核读通过；
- 新增的亚帧竞态（OBSERVE-66-03）是 React 更新队列的既有语义（先 merge 再整包 PUT 本就要求一帧完成合并），非 M-1 修复引入，且下游五道防线（F17 空集守卫 / F16 发布 id 全数校验 / 回显合并自愈 / handleBack 5s / saveNow 串行化）全部兜住、方向保守；本轮无任何「误放行→未回显整包 PUT 抹除后端目标」的路径。

## R65 后续/本轮修复复核

R65 无涉及 web 的代码修复（三处均为后端卫生：db 注释/accounts mock CT/readyProbe 注释）。本轮复核 R65 前端相关断言全部维持：
- M-1 稳态假阳性命中路径（新增改备选后 2s 轮询 + rev>0 回显短路）在回显 effect 三条置位路径下均不会误置位，维持 R65 判定；
- O-12（Dashboard `nowMs` 渲染期 `Date.now()` + extras 折叠行 1s 陈旧窗口）：维持观察，无新证据升级。

## 新发现

### OBSERVE-66-01 — audit.mjs 违例报告与代码实际命名不一致（报「bg-neutral-95x」实为 Button `dark` 变体 `bg-black`）

- **位置**：web/scripts/audit.mjs:49-59 + web/src/components/ui/Button.tsx:21
- **一句话**：audit.mjs 在 C 段对 `bg-neutral-950/900` 的逐行检查把 `Button.tsx:21` 的 `dark: "bg-black text-white border border-white/25 ..."` 报告为「含 bg-neutral-95x 实色」，但该行实际是 `bg-black`（前置 `bg-black/25|30|70|75` 循环已放过，`bg-neutral-95x` 检查又误捕）。
- **影响面**：`node scripts/audit.mjs` 恒 exit 1（1 违例），CI 未接入该脚本所以不被卡。工具本身的报告不可信。
- **机制说明**：`bg-black` 与 `bg-black/25` 字符串包含关系 + 检查逻辑扫描「含 bg-neutral-950 或 bg-neutral-900」的每一行（非 <option> 即 bad），`border-white/25` 触发子串 `/25` 命中 `bg-black/25` 名单？不——实测逐条核对：实际命中点在 `assertFile(!text.includes(cls))` 的全文件包含检查，`bg-black/25` 等四个类 `Button.tsx` 里没有；唯一违例来自 `bg-neutral-95x` 的逐行分支，误报行 `dark: "bg-black ... border-white/25 font-medium hover:bg-neutral-900 hover:border-white/50"`——子串 `bg-neutral-900` 命中。
- **修复建议**：dark 变体是 R61 O-9 已知观察（「black 主背景实色」），audit 的 C 段语义本意是「实心不透明黑洞」，`bg-black` 恒在视觉设计里（纯黑极简主题的 body/html 本身就是 `--bg:#000000`）。建议：① audit.mjs 的 bg-neutral-95x 检查改用**词边界**（`(?:^|\s)bg-neutral-9\d{2}`）；② 或如 R61 建议，将 Button dark 变体加入豁免名单，并在报告中显式说明「bg-black 是设计 token 而非黑洞违例」。裁决建议：**续**（审计工具语义修正，非产品 bug）。
- **注意**：audit.mjs 头注释声明运行方式 `node scripts/audit.mjs` 是正确的；head 注释「exit 0」与实测 exit 1 分叉即此违例所致，修后自愈。

### OBSERVE-66-02 — 三个 TDD 脚本头注释的运行命令在新 jiti 2.7 下失效

- **位置**：web/scripts/target-guard-check.ts:8、admin-auth-check.ts:8、unauthorized-check.ts:8
- **一句话**：三脚本头注释写 `node --import jiti scripts/*.ts`，实测 jiti 2.7 抛 `ERR_MODULE_NOT_FOUND`（jiti 的 import 注册入口迁移为 `jiti/register`）。
- **影响面**：按注释操作的人/流程跑不通；用 `node --import jiti/register ...` 实测三条全绿。
- **修复建议**：三处头注释统一改为 `node --import jiti/register`。裁决建议：**续**（注释口径修复，可随下次清理提交）。

### OBSERVE-66-03 — 回显合并与 echoed 置位非原子的亚帧竞态（保守方向，非新缺陷）

- **位置**：web/src/routes/Select.tsx:238/295（置位）与 :250-279（合并 updater）、:687/501/594/602（守卫消费）
- **一句话**：echoedRef 置位（effect 同步段）与合并 setSelected 提交（下一渲染帧）之间，存在「守卫读 echoed=true 放行、但 selected 尚未含后端旧目标」的亚帧窗口；同一拍内叠加「回显合并是函数式 updater、用户 pick 用渲染闭包 selected 发对象式 setState」的覆盖窗口，对象式会覆盖在飞合并结果。
- **影响面**：窗口宽约 1 个事件循环拍（微秒-毫秒级），仅出现在「首帧 courses 非空 + 用户在合并提交前恰好改目标」的同拍叠加下。最坏结果 = 该次整包 PUT 只含用户新改动 → 被下游守卫拦下置脏 + 回显合并（selected 变化）触发防抖重跑自愈落库完整目标。**不丢后端旧目标，至多一次重试**。
- **机制说明**：① React 状态更新语义——effect 内函数式 updater 排队后未提交，同一渲染周期内事件处理器读到的 selected 是「最近已提交渲染值」；② React 19 对对象式与函数式 setState 按队列序执行，对象式覆盖在飞函数式 updater 的合并结果（最小构造验证实证，临时脚本已删除）；③ 回显 effect 在 :287 的 `selectedHasStalePublish(selected, pubs)` 处读的也是「未合并」快照，但该守卫是安全方向。
- **修复建议**：无需修复（保守方向被五道防线兜住：F17 空集守卫 :722 / F16 发布 id 全数校验 :728 / 回显合并自愈 / handleBack 5s 兜底 / saveNow 串行化）。若未来要做完整收敛，可将回显合并的 `setSelected` 从「函数式 updater 排队」改为「同步计算后直接提交」并用同一拍内的结果同时置位 echoedRef（两者原子化），但引入重构风险与当前零缺陷稳态不匹配，不建议本轮动。裁决建议：**续**（维持 OBSERVE，下轮复核确认亚帧窗口机制无扩散）。

## 构建验证

- **`npm run build`**（tsc -b && vite build）：**全绿**（BUILD_EXIT=0，417.58 kB JS / 41.07 kB CSS，963ms）。
- **target-guard 断言**：`node --import jiti/register scripts/target-guard-check.ts` → **18/18 全绿**（含 R63 M-1 的两条新断言：courses 非空+有选中+已回显 → 放行 / 首帧未到+已回显标志 → 仍推迟）。
- **admin-auth 断言** 6/6 全绿；**unauthorized 断言** 5/5 全绿。
- **audit.mjs**：**1 违例**（OBSERVE-66-01，Button dark 变体误报，非产品缺陷），exit 1。
- **oxlint**：零 error，17 条既有 warning（react-hooks/exhaustive-deps ×4、react set-state-in-effect ×3、react purity Date.now、no-unsafe-finally、only-export-components ×2、no-unused-vars 等）——全部为历轮已知稳态项，无新增。
- **调试残留**：`console.log`/`debugger`/`TODO`/`FIXME` 零命中（仅 Login.tsx:265 激活码输入框 placeholder 含 "XK-XXXX-XXXX-XXXX"，是业务文案非调试残留）。
- 仓库工作区零污染（只读铁律遵守，`git status` 干净）。

## 历轮观察延续

- **N-1**（冲刺文案三态）：维持，无新证据升级。
- **N-2/N-3**：维持。
- **O-1~O-8**（Dashboard key 不对称 / ui 模板残宽 / NaN 防御 / 多标签页等）：维持，逐条复查无升级。
- **O-9**（Button dark variant `bg-black` 与 audit 护栏相邻）：**本轮新增联动证据**——audit.mjs 对 dark 变体误报（OBSERVE-66-01），倾向「维持 + audit 工具豁免」的 R61 方向。
- **O-10**（toast key 副边）：维持。
- **O-11**（M-1 修复两轮收敛窗口变宽 2~5s 安全方向）：维持。
- **O-12**（Dashboard nowMs 渲染期 Date.now + extras 折叠行 1s 陈旧窗口）：维持。
- **M-1 闭合**：第三轮确认闭合（见上），下轮可降级为常规复核。

## 附：最小构造验证说明

本轮为 OBSERVE-66-03 在系统临时目录（%TEMP%）写了 30 行验证脚本，模拟 React 更新队列验证「对象式 setState 覆盖在飞函数式 updater」机制：实测合并后 selected 只含用户新改动（覆盖实证）+ 守卫放行时 selected 未合并（亚帧窗口实证）。脚本运行后已删除，仓库零污染。
