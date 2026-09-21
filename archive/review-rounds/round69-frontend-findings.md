# review-round69 前端发现（只读审查，基线 d9edf8e，覆盖 R68 卫生修 commit b5b1fdc）

## 概述

**R68 三处格式修复逐行核证全部到位：Select 容量块整体归一 2sp 级差体系、lastJson 归一 20sp、Admin 操作列 td 归一 20sp，JSX 树结构与配对零破坏。M-1 第六轮低成本核对 + OBSERVE-66-03 setSelected 清点确认双闭合；全线构建验证全绿（npm run build exit 0 / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 / audit exit 0 / oxlint 13 条全既有无新增）；零新增 MAJOR/MINOR，仅 4 条新 OBSERVE（均为纯格式/注释/无障碍卫生，零行为影响）。** 全仓 `</div>` 相邻 JSX 同行持续清零，调试残留零命中，安全工作区零污染（只读铁律遵守）。

---

## R68 格式修复核（commit b5b1fdc 逐字核证）

### ① Select.tsx:1075-1097 容量统计块整体归一（R67 OBSERVE-68-01 落地）

**核证通过，块内全部行整体 -2sp 归一到位，级差体系与参照信息块完全一致。**

| 元素 | 参照信息块（1060-1073） | 容量块（1075-1097） | 判定 |
|---|---|---|---|
| 容器 div | 1060 = **28sp** | 1076 = **28sp** | ✓ |
| 直接子 div | 1061 = **30sp** | 1077 = **30sp** | ✓ |
| 孙 span | 1062 = **32sp** | 1078/1082 = **32sp** | ✓ |
| 曾孙内容 | 1064 = **34sp** | 1079/1080/1083 = **34sp** | ✓ |
| 三元续行 | — | 1084/1085 = **36sp**（34 内 2sp 子级） | ✓ |
| Progress | — | 1092 = **30sp**，props 1093-1095 = **32sp**，自闭合 1096 = **30sp** | ✓ |
| F41-N1 注释 | — | 1088 = **30sp**，续行 1089-1091 = **34sp**（30 内注释续行 +4sp，与文件内多行注释惯例一致） | ✓ |
| 闭合 `</div>` | 1073 = **28sp** | 1097 = **28sp** | ✓ |

- **diff 铁证**（`git show b5b1fdc`）：旧版 `-` 行即原 28/32/34/36/32/36/30 系（块内 4sp 级差），新版 `+` 行替换为 28/30/32/34/32/30/28 系——ACD 逐字节同步平移 -2sp，含 F41-N1 注释块与 Progress 两个子块，非内容改动。R67 报告 OBSERVE-68-01 修复承诺逐条兑现。
- **JSX 打包配对核证**：容器 1076 `open` → 1077 子 div → 1087 子 div 闭合 → 1092 Progress 自闭合 `/>` → 1097 容器闭合，深度链 28→30→28 完全平衡；Select.tsx 全文件 `open div=31 / close=30 / selfClose=1`（:1186 底部留白 div）平衡不破。
- **判定**：与文件内兄弟块（参照信息块 28/30/32/34 体系）级差完全一致，R68 该修已闭环。

### ② Select.tsx:379 lastJson 归一 20sp

**核证通过。** `const lastJson = useRef("")` 现为 **20sp**，同段 `targetRef/savingRef/dirtyRef/unmountedRef`（:380-388）全部 20sp；diff 为单行 `-24sp +20sp`，段内注释（:374-378）不受影响。

### ③ Admin.tsx:839 操作列 td 归一 20sp

**td 起始行核证通过，但块内关闭行残留半程态（见 OBSERVE-69-01）。**

- td 开行 839 = **20sp**，与同表 822/836 td 完全一致；diff 为单行 `-18sp +20sp`。
- 表格结构核证：thead th（808-811 = 16sp）→ tbody td（822/836/839 = 20sp）→ `</tr>` 820 = 18sp，整体体系正确。

### ④ 全仓格式一致性扫描

- `</div>` 后端 JSX 同行零命中：`grep -rn -E '</div>[[:space:]]*<div' src/` **零结果**（R67 之后 Dashboard 拆分保持，R68 未引入新同类）。
- 无新增缩进错位（对通读的 20 个 src 文件中的 JSX 块抽查：Login/Dashboard/App/Admin 主体缩进正常）。

---

## M-1 延续 + OBSERVE-66-03 setSelected 清点

### 结论

**M-1 第六轮低成本核对：闭合。OBSERVE-66-03 setSelected 调用点清点：五调用点无新增、无第三来源；独立清理 effect 零改动。**

### M-1 核对依据

- **三消费点逐字符传 echoedRef.current**：防抖回调 `Select.tsx:686`、flushTargets `:501`、handleBack 判定 `:594` + while `:602`——`shouldDeferSave(stateDataRef.current, …, echoedRef.current)` 三参数签名逐字符一致，且不依赖 echoedRef 的纯数据判据（`targetGuard.ts:64-72`）、第三参数稳态放行语义（R63 M-1）与 R67 基线零 diff。
- **echoedRef 置位路径无回潮**：courses 空分支 `:238-239`（置位 + setEchoDone）、合并完成分支 `:295-296`（全部合并/清理之后置位）、account reset `:200`（置 false）。首帧未到 `:232` 前置 return 绝不置位；独立清理 effect `:316-327` 首行 `!echoedRef.current` 守卫不变。
- **target-guard 18/18 复跑全绿**（含 R63 M-1 两条稳态语义断言：courses 非空+有选中+已回显→放行 / 首帧未到+已回显→仍推迟）。

### OBSERVE-66-03 setSelected 调用点清点

全仓 `setSelected` 出现行逐一核读，仍为 **五个实质调用点，无新增，无第三来源**：

| 行号 | 形式 | 归属 |
|---|---|---|
| :198 | `setSelected({})` | account reset（Sync 事件环，非 updater） |
| :250 | `setSelected((prev) => …)` 函数式 | 回显 effect 合并（唯一函数式 updater 源） |
| :288 | `setSelected((prev) => cleanStaleSelected(…))` 函数式 | 回显 effect 内清理（与 :250 同 effect 同拍） |
| :320 | `setSelected((prev) => cleanStaleSelected(…))` 函数式 | 独立清理 effect（:316-327，依赖含 selected） |
| :351 / :361 | `setSelected({ ...selected, … })` 对象式 | 用户 pick（渲染闭包快照） |

- **无第三来源确认**：其余 `setSelected` 命中（:341/:345/:605/:680）均为注释文本非调用。
- **独立清理 effect（:316-327）与 R67 基线一致**：依赖 `[publishes, selected, echoedRef, toast]` 与 `// eslint-disable-next-line react-hooks/exhaustive-deps` 不变。
- **亚帧窗口判定维持**：对象式覆盖 vs 函数式合并双来源模型无新增扩散，下游五道防线（F17 空集守卫 :722 / F16 全数校验 :728 / 回显合并自愈 / handleBack 5s / saveNow 串行化）兜底不变。

---

## 新发现

### OBSERVE-69-01 — Admin.tsx:840-860 操作列 td 块内缩进残余：R68 只修了 td 开行，块内 div/td 闭合仍低 2sp

- **位置**：web/src/routes/Admin.tsx:840（`<div className="flex items-center justify-end gap-2">`）、:859（`</div>`）、:860（`</td>`）
- **一句话**：R68 把操作列 td 开行 839 从 18sp 归一到 20sp 与同表对齐，但该 td 块内 `<div>` 仍 **20sp**（应为 22sp，td 直接子级 2sp 级差）、闭合 `</div>` **20sp**（应 22sp）、`</td>` **18sp**（应 20sp）——与同表其余 td（开 20 / 内容 22 / 闭 20）不一致，属 R68 修复的"只对齐块头、未收内部"半程态（与 R67 容量块同型）。
- **影响面**：零。tsc/vite/oxlint 不报，DOM 渲染逐字节相同，纯源码排版；8 行 `</tr>` 正确 18sp 不受影响。
- **机制说明**：该块历史以 18sp 为整块基准（td 开/块内/td 闭同 18sp 同级），R68 只把 td 开行 +2sp 平移到 20sp，块内 div 与 td 闭合未随动，形成 td(20)→div(20)→Button(22) 的 0sp 级差断点。
- **修复建议**：随下次格式卫生提交把 840 与 859 改 22sp、860 改 20sp，与同表 td 块内体系（td 20 / 内容 22 / 闭 20）对齐。裁决建议：**续**。

### OBSERVE-69-02 — Dashboard.tsx:519 过时注释残留"3 门重点看护"字样（F10-06 只清了展示文案）

- **位置**：web/src/routes/Dashboard.tsx:519 `{/* 预选目标矩阵（3 门重点看护课程卡片） */}`
- **一句话**：F10-06 修复（R37 前后）把运行时展示的"3 门"旧约束全部改成动态目标集（:484 `{courses.length} 门`、:530 `TARGETS`），但 :519 区块标题注释仍写"3 门重点看护课程卡片"——展示层与注释语义分叉。
- **影响面**：零行为。纯文档卫生；toc 无工具消费该注释。
- **机制说明**：F10-06 改的是 JSX 文案与注释变量名（:482-483、:529），:519 顶部区块注释未在 edit 范围内，历史残留。
- **修复建议**：随下次卫生提交把注释改为"预选目标矩阵（重点看护课程卡片）"或直接删"3 门"。裁决建议：**续**。

### OBSERVE-69-03 — api/client.ts:86-94 body 层 401 分支缩进错位（R45 N-1 改造遗留）

- **位置**：web/src/api/client.ts:86-94（`if (r.status !== 401) {` 块内 `let account = ""` 与内层 `if` 缩进均为 6sp，按 2sp 体系应为 8/10sp）
- **一句话**：R45 N-1"HTTP 401 前置广播 + body 401 只兜 HTTP 200 形态"防双发改造时，新增分支块内缩进未随 2sp 体系排版，`let account` 与 `if (path.includes…)` 比外层低 2sp。
- **影响面**：零行为。逻辑正确（`r.status !== 401` 时走 body 广播兜底），tsc/oxlint 不报缩进；纯格式。
- **机制说明**：该分支是 R45 补丁追加行，未按上下文 2sp 缩进级差排版。
- **修复建议**：随下次格式卫生提交把 86-94 整体右移 2sp。裁决建议：**续**。

### OBSERVE-69-04 — Select.tsx:858-866 搜索框缺 label/aria-label（placeholder 非 label）

- **位置**：web/src/routes/Select.tsx:858-866（`:860 <Input placeholder="搜索课程名称、教师或教室" …>`）
- **一句话**：搜索输入框只有 placeholder、无关联 `<label>` 或 `aria-label`——placeholder 在读屏下不构成输入用途说明（且实操中占位符文本可读但无控件语义），与全站可达性基线（Login 输入框有 `label htmlFor`、Admin 生成/次数有 label、可见性切换有 aria-label）不齐。
- **影响面**：仅无障碍。视障读屏用户无法明确得知该输入框"搜索课程/教师/教室"；键盘/鼠标用户无影响。非功能缺陷。
- **机制说明**：该输入框由装饰性 `Search` 图标 + placeholder 表达用途，无 DOM 关联 label；Radix/用户库无自动生成。
- **修复建议**：加 `aria-label="搜索课程名称、教师或教室"`（或 `<label htmlFor>` + `id`）即可闭合，一行改动。裁决建议：**续**。

---

## 构建验证

| 验证项 | 命令 | 结果 |
|---|---|---|
| 前端生产构建（最终判定源） | `cd web && npm run build` | **exit 0**（tsc -b 无报错 + Vite：1948 modules → 417.56 kB js / 41.07 kB css，1.80s），产物落 `backend/web/dist` 且 `git check-ignore` 确认被忽略 |
| target-guard 断言（含 R63 M-1 两条） | `node --import jiti/register scripts/target-guard-check.ts` | **18/18 全绿** |
| admin-auth 断言 | `node --import jiti/register scripts/admin-auth-check.ts` | **6/6 全绿** |
| unauthorized 断言 | `node --import jiti/register scripts/unauthorized-check.ts` | **5/5 全绿** |
| 视觉护栏 | `node scripts/audit.mjs` | **exit 0**（A/B/C 三段全绿） |
| oxlint | `npx oxlint` | **零 error，13 条既有 warning**（5×exhaustive-deps = Select:337/403×2/686 + Dashboard:263；3×set-state-in-effect = useTickingCountdown:17 + Dashboard:271 + Admin:505；2×only-export-components = Toast:24 + Dashboard:120；purity Dashboard:198 / no-unused-vars audit.mjs:30 / no-unsafe-finally Select:452）——**逐条与 R67 报告 13 条对照一致，无一新增** |
| 调试残留 | `grep -E 'console\.(log\|debug)\|debugger\|TODO\|FIXME'` src | **src 零命中**（Login:265 `XK-XXXX-XXXX-XXXX` 为激活码 placeholder 业务文案；scripts 内 console.log 为断言脚本输出） |
| XSS/注入面 | `grep dangerouslySetInnerHTML/innerHTML/document.write/eval/new Function` src | **零命中** |
| 安全工作区 | `git status --short --branch` | `## master` 干净，0 行改动（构建产物被忽略） |

---

## 历轮观察延续

- **M-1**：第六轮低成本核对闭合（见上），维持「按历轮观察延续管理」降级，无新证据。
- **OBSERVE-66-03**（亚帧竞态）：setSelected 五调用点清点无新增、无第三来源，独立清理 effect 零改动，维持「续」。
- **N-1~N-3**（冲刺文案三态 / pick 一拍调度延迟 / 三处格式卫生）：R68 已把 N-3 的 Select lastJson + Admin td 两处落位（第三处 = Select 容量块本轮亦已落位），N-1/N-2 维持，无新证据升级。
- **O-1~O-12**（Toast viewport 滚动残余 / 倒计时 NaN 防御 / 手写 modal 焦点陷阱 / 管理态多标签页 / Dashboard key 不对称 / ui 模板残宽 / Button dark bg-black / 401 闭包 current / 状态机四态 / M-1 边角语义 / Dashboard nowMs 渲染期 Date.now + extras 1s 陈旧窗口）：逐条复查无升级证据，维持。
- **新 OBSERVE-69-01~04**：全部「续」方向（随下次前端格式/无障碍卫生提交顺带），不单独提交。

---

## 附：只读铁律与临时脚本说明

本轮全部验证为只读命令（git log/show/diff/check-ignore、node/npx 运行既有脚本与 node -e 输出、grep 扫描），未在仓库写入任何代码；唯一写入为本归档报告 `archive/review-rounds/round69-frontend-findings.md`（审查产出，git 未跟踪该变动——工作区保持 `git status` 干净）。构建产物落 `backend/web/dist` 且被忽略。未写任何临时验证脚本（本轮无行为性疑点需最小构造验证——R68 为纯文本排版、M-1/亚帧 R68/69 连续两轮穷举无扩散）。