# review-round68 前端发现（只读审查，基线 96e0f50，覆盖 R67 卫生修 commit 506ab3c）

## 概述

**R67 格式修复两处逐行核证：Dashboard 相邻 JSX 拆行完全正确；Select 容量块缩进归一仅完成"块头对齐"、块内子元素残留 +2sp 级差（4sp 级差体系），属纯格式卫生非缺陷。M-1 第五轮低成本核对 + OBSERVE-66-03 setSelected 清点确认双闭合；全线构建验证全绿（npm run build exit 0 / target-guard 18/18 / audit exit 0 / oxlint 13 条全既有无新增）；零新增 MAJOR/MINOR。** 全仓 `</div>\s*<div` 相邻 JSX 同行已清零。本报告新建 2 条 OBSERVE（Select 容量块块内缩进残余 / Admin+Select 两处历史缩进瑕疵），均为纯格式卫生零行为影响，延续 R67「续」裁决方向。仓库工作区零污染（只读铁律遵守）。

---

## R67 格式修复核（commit 506ab3c 逐行核对）

### ① Dashboard.tsx:501 相邻 JSX 拆行（`</div>` + 独立行 `<div>`）

**核证通过，结构正确、配对无破坏。**

- **diff 铁证**（`git show 506ab3c`）：删除行 `-                </div>                <div className="py-2.5 flex items-center justify-between">`，新增两行 `+                </div>` 与 `+                <div className="py-2.5 flex items-center justify-between">`——纯拆行，无任何内容改动。
- **前导缩进测量**（abs 空格）：`:501 </div>` = **16sp**，`:502 <div>` = **16sp**，与运行指标块全部指标行对齐（476/480/486 的 `<div className="py-2.5 ...">` 均为 **16sp**）。拆行后两个 JSX 节点各自独立成行且缩进与兄弟完全一致。
- **JSX 树结构核证**（区块 474-515 逐行）：
  ```
  474 14sp <CardContent ...>
  475 14sp <div divide-y>          （容器）
  476 16sp <div 指标行 A> … 479 16sp </div>
  480 16sp <div 指标行 B> … 485 16sp </div>
  486 16sp <div 指标行 C> … 501 16sp </div>   ← 拆行处闭合
  502 16sp <div 指标行 D> … 505 16sp </div>   ← 独立行开始
  506 14sp </div>                   （容器闭合）
  508 14sp <div 独立第二块>
  515 12sp </CardContent>
  ```
- **闭合配对计数**：Dashboard.tsx `open <div> = 52` / `close </div> = 52` / selfClose = 0，**完全平衡**。
- **全仓相邻 JSX 同行清零**：`grep -rn "</div>\s*<div"`（src 全目录）**零命中**——Dashboard 拆行后全仓唯一一处已消除，R67 报告的整治目标达成。

### ② Select.tsx:1075-1077 容量统计块缩进归一（4 空格偏移 → 2 空格）

**部分归一，块头已对齐兄弟、块内子元素残留 +2sp 级差（实质发现，见 OBSERVE-68-01）。**

- **diff 铁证**（`git show 506ab3c`）：四行内容层面仅两行变化——`{/* 容量统计 */}` 注释行与 `<div className="space-y-1.5 pt-1.5">` 容器行各自 **从 30sp 归一到 28sp**；容器块内其余所有行（子 div/span/Progress/F41-N1 注释）**未动**。
- **缩进体系对照**（abs 空格测量）：
  | 元素 | 参照信息块（1060-1066） | 容量块（1076-1087） |
  |---|---|---|
  | 容器 div | 1060=**28sp** | 1076=**28sp**（已对齐） |
  | 直接子 div | 1061=**30sp** | 1077=**32sp**（应 30） |
  | 孙 span | 1062=**32sp** | 1078=**34sp**（应 32） |
  | 曾孙 span | 1064=**34sp** | 1080=**36sp**（应 34） |
  | Progress（容器子级） | — | 1092=**32sp**（应 30） |
  | 闭合 `</div>` | 1073=**28sp** | 1097=**30sp**（应 28） |
- **判定**：R67 修复将块头（注释 + 容器 div）从 30sp 归一为 28sp 与 CardContent 对齐，**正确**；但**块内全部子元素仍保留原 +2sp 偏移**，导致容器(28sp)→直接子级(32sp)出现 **4sp 级差**，与文件内兄弟块（信息块 28→30 的 2sp 级差）不一致。修复前块内是"整体偏移 +2sp 的连贯 2sp 体系"（30/32/34/36），修复后变成"块头对齐但块内级差损坏成 4sp"（28/32/34/36）。**非缺陷**——tsc/vite/oxlint 全不报，渲染 DOM 结构逐字节相同，属纯源码排版卫生。
- **结构安全**：容器→子 div：编号对照正确（1087 闭合 1077）、Progress 自闭合 `/>` 同区块、闭合 `</div>` 1097 深度=容器子级 30sp（虽比规范 28sp 高 2sp 但配对正确）。Select.tsx 全文件 div 平衡（open 31 / close 30 / selfClose 1，selfClose 为 :1186 底部留白 div，平衡成立）。

---

## M-1 延续 + OBSERVE-66-03 setSelected 清点

### 结论

**M-1 第五轮低成本核对：闭合。OBSERVE-66-03 setSelected 调用点清点：五调用点无新增、无第三来源；独立清理 effect 无改动。**

### M-1 核对依据（R67 已移出重点清单，本角色按历轮观察延续做低成本核对）

- **三消费点真传 echoedRef.current**：防抖 effect `Select.tsx:686`、flushTargets `Select.tsx:501`、handleBack 判定 `:594` + while `:602`——`shouldDeferSave(stateDataRef.current, …, echoedRef.current)` 逐字符一致，判定与循环同参。均带 R63 M-1 注释块（:499-500 / :591-593 / :599-601 三处）标识。
- **echoedRef 置位路径无回潮**：courses 空分支 `:238-239`（置位 + setEchoDone）、合并完成分支 `:295-296`（全部合并/清理之后置位）、account reset `:200`（置 false）。首帧未到 `:232` 前置 return 绝不置位。
- **target-guard 断言 18/18 复跑全绿**（含 R63 M-1 两条稳态语义断言）。

### OBSERVE-66-03 setSelected 调用点清点（本轮重点）

全仓 `setSelected` 出现行逐一核读，仍为 **五个实质调用点，无新增，无第三来源**：

| 行号 | 形式 | 归属 |
|---|---|---|
| :198 | `setSelected({})` | account reset（Sync 事件环，非 updater） |
| :250 | `setSelected(prev => …)` 函数式 | 回显 effect 合并（**唯一函数式 updater 源**） |
| :288 | `setSelected(prev => cleanStaleSelected(prev, currentIds))` 函数式 | 回显 effect 内清理（与 :250 同 effect 同拍） |
| :320 | `setSelected(prev => cleanStaleSelected(prev, currentIds))` 函数式 | 独立清理 effect（:316-327，依赖含 selected） |
| :351 / :361 | `setSelected({ ...selected, … })` 对象式 | 用户 pick（渲染闭包快照） |

- **无第三来源确认**：`grep -n setSelected` 其余命中（:341/:345/:605/:680）均为注释文本，非调用。
- **独立清理 effect（:316-327）未被改动**：与 R67 基线逐行一致（`git diff 506ab3c 96e0f50 -- web/src` 零输出），依赖 `[publishes, selected, echoedRef, toast]` 不变。
- **亚帧窗口判定维持**：「对象式覆盖在飞函数式合并」的双来源模型（pick 对象式 × 回显合并函数式）无新增扩散，下游五道防线（F17 空集守卫 :722 / F16 全数校验 :728 / 回显合并自愈 / handleBack 5s / saveNow 串行化）兜底不变。

---

## 新发现

### OBSERVE-68-01 — Select.tsx 容量统计块块内缩进残余：块头已归一、块内级差 4sp（R67 修复未完全落位）

- **位置**：web/src/routes/Select.tsx:1075-1097
- **一句话**：R67 将容量块块头（注释 + 容器 div）从 30sp 归一到 28sp 与兄弟对齐后，**块内全部子元素未同步归一**，容器(28sp)→直接子级(32sp) 成 4sp 级差，与文件内 2sp 级差体系（参照 1060→1061 的 28→30）不一致。
- **影响面**：零。纯源码排版；tsc/vite/oxlint 零报告（格式类规则未启用）；DOM 渲染结构逐字节相同。
- **机制说明**：R67 修复前块内是"整体偏移 +2sp 的连贯 2sp 体系"（30/32/34/36 与 28/30/32/34 同级差、整体高一档）；修复只把块头降 2sp，块内动过的 30sp 起始 → 28sp，但子元素坐标未跟着整体平移，级差从 2sp 变 4sp。属 R67 修复的"只对齐块头、未收内部"半程状态。
- **修复建议**：下轮顺手把容量块内全部行整体 -2sp（:1077-1097 的行分别归一到 30/32/34/36，闭合 `</div>` 归 28sp），与信息块 2sp 级差体系统一。裁决建议：**续**（纯格式卫生，资质不独立提交，随下次前端提交顺带）。

### OBSERVE-68-02 — Admin.tsx:839 与 Select.tsx:379 两处历史缩进瑕疵（R67 报告未收录）

- **位置**：web/src/routes/Admin.tsx:839（`<td className="p-3 sm:p-4 align-middle text-right">` 为 18sp，同表其余 `<td>` 均为 20sp）；web/src/routes/Select.tsx:379（`const lastJson = useRef("")` 缩进 24sp，段内其余同层 ref 声名均 20sp）。
- **一句话**：两处历史 JSX/代码缩进错位（Admin tbody 操作列 td 比同表兄弟低 2sp；Select 保存串行化段 lastJson 比同层声明多 4sp），均为历史提交拼接所致、不影响运行。
- **影响面**：零。tsc/vite/oxlint 不报；Admin.tsx div 平衡（open 46 / close 46）且 `<td>` 属 HTML 表格结构无关缩进。
- **机制说明**：`git blame` 溯源——Admin.tsx:839 源自 `290191f4`（2026-09-12），同表 20sp td 由 `c05028d8` 在其后提交改写，形成新旧两代缩进并存；Select.tsx:379 源自 `9cca0ff8`（2026-09-14），R67 未覆盖（R67 修复范围仅 :501 Dashboard 与 :1075-1077 容量块）。
- **修复建议**：与 OBSERVE-68-01 一并随下次格式卫生提交消除（Admin:839 改 20sp；Select:379 改 20sp）。裁决建议：**续**。

---

## 构建验证

| 验证项 | 命令 | 结果 |
|---|---|---|
| 前端生产构建（最终判定源） | `cd web && npm run build` | **exit 0**（tsc -b 无报错 + Vite：1948 modules → 417.56 kB js / 41.07 kB css，1.17s），产物落 `backend/web/dist` 且 `git check-ignore` 确认被忽略 |
| tsc -b 独立复跑 | `npx tsc -b` | **exit 0**（基线同步，工作区零新增） |
| target-guard 断言 | `npx jiti scripts/target-guard-check.ts` | **18/18 全绿**（含 R63 M-1 两条） |
| admin-auth 断言 | `node --import jiti/register scripts/admin-auth-check.ts` | **6/6 全绿** |
| unauthorized 断言 | `node --import jiti/register scripts/unauthorized-check.ts` | **5/5 全绿** |
| 视觉护栏 | `node scripts/audit.mjs` | **exit 0**（A/B/C 三段全绿，C 段 85 条断言含 6 文件 × bg-black/25\|30\|70\|75 全过 + 整页 bg-black 零命中） |
| oxlint | `npx oxlint` | **零 error，13 条既有 warning**（audit.mjs all 未用变量 / useTickingCountdown:17 set-state-in-effect 设计使然 / Select:337·403·686 exhaustive-deps 已知稳态 / Select:452 no-unsafe-finally 为 F13-C2 卸载兜底 / Toast:24·Dashboard:120 only-export-components / Dashboard:198 purity·271 set-state-in-effect / Dashboard:263 exhaustive-deps / Admin:505 set-state-in-effect）——**逐条与 R67 报告 13 条对照一致，无一本轮新增** |
| 调试残留 | `grep console.log/debugger/TODO/FIXME` src | **src 零命中**（Login.tsx:265 `XK-XXXX-XXXX-XXXX` 为激活码 placeholder 业务文案；scripts 内 console.log 为断言脚本输出） |
| 安全工作区 | `git status --short --branch` | `## master` 干净，0 行改动 |

---

## 历轮观察延续

- **M-1**：第五轮低成本核对闭合（见上），维持 R67「按历轮观察延续管理」降级，无新证据。
- **OBSERVE-66-03**（亚帧竞态）：setSelected 五调用点清点无新增、无第三来源，维持「续」。
- **N-1~N-3**（冲刺文案三态 / pick 一拍调度延迟 / 三处格式卫生）：维持，无新证据升级。
- **O-1~O-12**（Toast viewport 滚动残余 / 倒计时 NaN 防御 / 手写 modal 焦点陷阱 / 管理态多标签页 / Dashboard key 不对称 / ui 模板残宽 / Button dark bg-black / 401 闭包 current / 状态机四态 / M-1 边角语义 / Dashboard nowMs 渲染期 Date.now + extras 1s 陈旧窗口）：逐条复查无升级证据，维持。

---

## 附：只读铁律与临时脚本说明

本轮全部验证为只读命令（`git log/show/diff/blame/check-ignore`、`node/npx` 运行既有脚本、`grep/sed`/node -e 输出），未在仓库写入任何文件；构建产物落 `backend/web/dist` 且 `git check-ignore` 确认被忽略、工作区保持零污染。未写任何临时验证脚本（本轮无需最小构造验证——R67 格式修复为纯文本排版、无行为可测，M-1/亚帧 R67 已穷举本轮只做扩散清点）。