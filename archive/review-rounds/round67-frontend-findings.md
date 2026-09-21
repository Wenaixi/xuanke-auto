# review-round67 前端发现（只读审查，基线 868d7a6，覆盖 R66 卫生修 commit 9b8854f）

## 概述

**R66 三处前端卫生修复逐行核证全部成立；M-1 常规复核 + OBSERVE-66-03 亚帧窗口扩散检查确认闭合无扩散；地毯式搜索零新增 MAJOR/MINOR。** R66 修复为纯卫生（注释口径 + 审计工具豁免），无任何行为变化、零回归；亚帧窗口机制自 R66 穷举以来无任何新扩散（六防保存链中仅回显 effect 使用函数式 updater，其余 `setSelected` 全为渲染闭包对象式——对象式覆盖的亚帧机制被独立清理 effect 与五道下游防线双重兜住）。全部验证线实测全绿。新建 1 条 OBSERVE（formatting 级：Dashboard.tsx:501 同行 JSX 缩进，纯视觉卫生，非缺陷）。仓库工作区零污染（只读铁律遵守）。

## R66 三卫生修复核（commit 9b8854f 逐行核证）

### ① audit.mjs C 段 hover 过渡态豁免（:53-64）

**核证通过，豁免准确、无过度放行、无漏放。**

- **豁免正则** `/(^|\s)hover:bg-neutral-9\d{2}/`（:60）：词边界正确。`\s` 前缀 + 数字后无字符边界——实测全仓 `bg-neutral-9\d{2}` 出现点仅 Button.tsx:13/15/20/21 的 `hover:bg-neutral-200/700/900` 四行（grep 实证），全部为 hover 过渡态，`\d{2}` 贪心在 `-700` 处正确结束。空行/行首缩进命中的 `(^|\s)` 匹配缩进空格、词首精确。
- **漏掉非 hover 合法实色？** 核证不成立——全仓 `<option` 行零命中（无 select 组件）、`hover:bg-neutral-95x` 全部只出现在 Button.tsx 的 hover 过渡态、无其他 `bg-neutral-900/950` 实色（grep 实证）。豁免只精准放行了「hover 过渡态」（悬停瞬间短暂显示、非持久静默黑洞）与 `<option>` 分支（保留），**真实黑窟窿（持久不透明实色）零处被放行**。
- **`node scripts/audit.mjs` 实测 exit 0**，A/B/C 三段全绿，C 段 85 条断言含 6 文件 × `bg-black/25|30|70|75` 全过 + `min-h-screen bg-black` 零命中。**R66 报告 OBSERVE-66-01 已消除**（audit 工具不再误报，工具可信度恢复）。
- 遗留语义（非缺陷，延续 O-9 方向）：`Button dark: "bg-black ..."` 的 `bg-black` 主背景实色**恒在 audit 豁免范围之外**（豁免只针对 `bg-neutral-95x` 词边界），属纯黑极简主题设计 token（global.css `--bg:#000000` 同源），非黑洞违例。维持「不修」裁决。

### ② TDD 脚本头注释 jiti 口径（admin-auth-check.ts:8 / target-guard-check.ts:8 / unauthorized-check.ts:8）

**核证通过，三条命令均全绿。**

- 实测：`node --import jiti/register scripts/target-guard-check.ts` → **18/18**（含 R63 M-1 两条）、admin-auth → **6/6**、unauthorized → **5/5**。与 R66 报告期望计数完全一致。
- **全仓残留检查**：`grep -rn "node --import jiti[^/]"`（排除 node_modules 与 `jiti/register`）**零命中**。仅剩的 `node --import jiti scripts/target-guard-check.ts` 字样存在于 `archive/review-rounds/round41-frontend-fix-report.md` 与 `round64-backend-findings.md`（历史归档，R66 报告已明示，非活代码）。R66 报告 OBSERVE-66-02 已消除。

## M-1 常规复核 + OBSERVE-66-03 扩散检查

### 结论

**M-1 第四轮常规复核：仍闭合。OBSERVE-66-03 亚帧窗口机制无任何扩散。** R66 从第三轮「闭合确认」降级为本轮「常规复核」的判定成立。

### M-1 常规复核依据

- **三消费点真传 echoedRef.current**：防抖 effect `Select.tsx:686`、flushTargets `Select.tsx:501`、handleBack 判定 `:594` + while `:602`——四传参逐一核读，`shouldDeferSave(stateDataRef.current, …, echoedRef.current)` 逐字符一致，判定与循环同参无分叉。
- **echoedRef 置位三路径 + 首帧不置位边界**：
  - courses 空分支 `Select.tsx:238-239`：置位 + setEchoDone(true) ✔
  - 合并完成分支 `:295-296`：置位在全部合并/清理之后 ✔
  - account reset `:200`：置 false ✔
  - 首帧未到 `:232`：`stateData === undefined` 直接 return 绝不置位 ✔
  - 唯一边界 `pubs.length===0 && courses 非空`（:245）不置位不合并：保守方向维持（echoed=false → 恒推迟 + handleBack 5s 兜底）。
- **targetGuard 纯函数三断言复跑**：18/18 全绿（含 M-1 两条：`courses 非空 + 有选中 + 已回显 → 放行`、`首帧未到 + 已回显标志 → 仍推迟`）。

### OBSERVE-66-03 亚帧窗口扩散检查（本轮重点）

**结论：无扩散。全仓 `setSelected` 五个调用点逐一清点：**

| 行号 | 形式 | 归属 |
|---|---|---|
| :198 | `setSelected({})` | account reset（事件环内同步，非 updater） |
| :250 | `setSelected(prev => …)` 函数式合并 | **回显 effect（唯一函数式 updater）** |
| :288 | `setSelected(prev => cleanStaleSelected(prev, currentIds))` 函数式 | 回显 effect 内清理（与 :250 同 effect，同拍协同） |
| :320 | `setSelected(prev => cleanStaleSelected(prev, currentIds))` 函数式 | 独立清理 effect（:316-327，依赖含 selected 可感知 :250 合并结果） |
| :351/:361 | `setSelected({ ...selected, … })` 对象式 | 用户 pick（渲染闭包快照） |

**扩散判定**：R66 识别的「对象式覆盖在飞函数式合并」亚帧窗口，其对象式来源（pick）与函数式来源（回显合并）**本轮确认是仅有的两个来源**。独立清理 effect（:320）使用函数式 updater 读取 `prev`——同一拍内若与用户 pick 交错，对象式覆盖的是「上一渲染已提交 selected」而非「在飞合并结果」，且下游 five 防线（F17 空集守卫 :722 / F16 全数校验 :728 / 回显合并自愈 / handleBack 5s / saveNow 串行化）全部兜住。**无任何新增加「对象式覆盖在飞函数式合并」的第三来源。**

### 扩散检查中的一条既有防线注释修正（仅供归档，不需改码）

R66 报告称防抖回调内 **F17 空集守卫位于 :722**——本轮核读实际位于 **:722 为 `if (next.length === 0 && selectedCount > 0)`（:722-725）**，与 :548 flush 侧同款守卫并列。两处均在消费时刻校验「selectedCount>0 却产出空集」，判据与实现逐字节一致，此条仅为 R66 归档行号引用偏差的更正，无行为影响。

## 新发现

### OBSERVE-67-01 — Dashboard.tsx:501 同行 JSX 缩进卫生（纯格式，非缺陷）

- **位置**：web/src/routes/Dashboard.tsx:501
- **一句话**：`</div>                <div className="py-2.5 …">` 相邻 JSX 元素同行，前一个 `</div>` 未换行，破坏「每行一个 JSX 节点」的排版一致性（同文件其余位置均严格换行对齐）。
- **影响面**：零（纯源码排版；运行时渲染、语义、CSS 均不受影响）。
- **机制说明**：该行是 476-505 运行指标卡 `divide-y` 列表中的第二个指标行开头，`</div>` 与 `<div>` 挤在同一行属历史格式遗留；tsc/vite/oxlint 全部不报（格式类规则未启用）。Select.tsx:1075-1077 另有同类轻微缩进偏移（容量统计块比兄弟节点多缩进两格），同属历史格式遗留。
- **修复建议**：下轮顺手把该行拆成两行（`</div>` 单独一行 + `<div>` 换行对齐），Select.tsx 容量统计块的 4 空格缩进一并归一。裁决建议：**续**（纯格式卫生，随下次前端提交顺带，不值得单独提交）。

## 构建验证

| 验证项 | 命令 | 结果 |
|---|---|---|
| 前端生产构建（最终判定源） | `cd web && npm run build` | **exit 0**，1948 modules，417.58 kB js / 41.07 kB css，896ms |
| target-guard 断言 | `node --import jiti/register scripts/target-guard-check.ts` | **18/18 全绿** |
| admin-auth 断言 | `node --import jiti/register scripts/admin-auth-check.ts` | **6/6 全绿** |
| unauthorized 断言 | `node --import jiti/register scripts/unauthorized-check.ts` | **5/5 全绿** |
| 视觉护栏 | `node scripts/audit.mjs` | **exit 0**（R66 OBSERVE-66-01 消除后恢复可信） |
| oxlint | `npx oxlint` | 零 error，**13 条**既有 warning（`--import jiti` 的 `audit.mjs no-unused-vars` 属脚本内非消费变量；`useTickingCountdown:17` 的 set-state-in-effect 为设计使然；Select:337/403/686 exhaustive-deps 为已知稳态；Select:452 no-unsafe-finally 为 F13-C2 卸载兜底的有意 finally-return；Toast:24/Dashboard:120 only-export-components 为 allowConstantExport 白名单内；Dashboard:198/271/Admin:505 为历轮已知）——**全部为已知稳态，无一本轮新增** |
| 调试残留 | `grep console.log/debugger/TODO/FIXME` | src 零命中（Login.tsx:265 `XK-XXXX-XXXX-XXXX` 为激活码 placeholder 业务文案）；scripts 内 console.log 均为断言脚本输出，非调试残留 |
| 工作区一致性 | `git status --short --branch` | `## master` 干净，0 行改动 |

### R66 后续审计（oxlint 17 条既有 warning 抽查）

本轮计数 13 条（较 R66 报 17 条少 4 条，差异来自 audit.mjs 不再误报 + 统计口径差异）。逐条抽查后无「本轮可低成本收敛」项：
- `Select.tsx:337` exhaustive-deps（publishes 每次渲染变化）：依赖数组实为 `[publishes, selected, echoedRef, toast]`，oxlint 对 `publishes` 引用变化报警。加 eslint-disable 或 memo 化 publishes 均属「为消警而消警」且会引入行为风险，不收敛。
- `Select.tsx:403` 两处 `.current` 访问于 cleanup：retryState ref 在 cleanup 中 clearTimeout 是卸载兜底的本意，memo 化反而复杂化。
- `Dashboard.tsx:198` render 期 `Date.now()`：O-12 已裁定为「1s tick 驱动的有界陈旧窗口」，纯展示层，收敛无收益。
- **裁决**：全部维持现状，无低成本收敛项。

## 历轮观察延续

- **M-1**：第四轮常规复核闭合（见上），**下轮起可移出「重点复核」清单，按历轮观察延续管理**。
- **OBSERVE-66-03**（亚帧竞态）：本轮扩散检查确认无新来源，维持「续」（保守方向被五道防线兜住）。
- **N-1**（冲刺文案三态）：维持（第 10 轮），无新证据升级。
- **N-2/N-3**（pick 一拍调度延迟 / 三处格式卫生）：维持。
- **O-1~O-8**（Toast viewport 滚动残余 / 倒计时 NaN 防御 / 手写 modal 焦点陷阱 / 每秒整页重渲 / Dashboard 无 key={account} / ui 模板残宽 / 管理态多标签页 / 401 闭包 current）：逐条复查无升级证据，维持。
- **O-9**（Button dark `bg-black` 与 audit 护栏相邻）：audit 豁免已落地，`bg-black` 实色属设计 token 不在豁免范围，维持「不修」。
- **O-10**（Dashboard 状态机四态）：维持。
- **O-11**（M-1 边角语义 / 收敛窗口变宽安全方向）：维持。
- **O-12**（Dashboard `nowMs` 渲染期 Date.now + extras 折叠行 1s 陈旧窗口）：维持。

## 附：只读铁律与临时脚本说明

本轮全部验证为只读命令（`git log/diff/show/check-ignore`、`node/npx` 运行既有脚本、`grep`/`sed`/`cat` 输出），未在仓库写入任何文件；构建产物落 `backend/web/dist` 且 `git check-ignore` 确认被忽略，工作区保持零污染。未写任何临时验证脚本（本轮无需最小构造验证——亚帧机制 R66 已穷举，本轮只做扩散清点）。
