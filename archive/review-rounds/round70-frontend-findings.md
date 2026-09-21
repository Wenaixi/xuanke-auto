# Round 70 前端只读审查报告

基线：commit a799c1b（R69 收官 docs）+ R69 四卫生修 201f942。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

## 概述

**R69 四处卫生修复全部逐行核证通过（commit 201f942 与工作树一致、零回潮）；M-1 第七轮闭合、OBSERVE-66-03 setSelected 五调用点无新增；全线构建验证全绿（npm run build exit 0 / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 / audit exit 0 / oxlint 12 条全既有）；零新增 MAJOR/MINOR，仅 3 条新 OBSERVE（纯注释语义一致性，零行为影响）。** 全仓 `</div>` 相邻 JSX 同行持续清零（grep 零命中 + 双正则扫描 21 处换行相邻均为结构正常嵌套），调试残留零命中（业务 src 无 console.log/debugger/TODO/FIXME，仅 Login.tsx:265 误报 FIXME 实为 placeholder 字面量），安全工作区零污染（只读铁律遵守，临时脚本置于 %TEMP% 已删）。

## R69 四卫生修复核（commit 201f942 逐行核证）

### ① Admin.tsx 操作列 td 块内归一（:840/:859/:860）——通过

- **diff 铁证**（`git show 201f942`）：旧版块内 div `-` 行为 20sp、Button 组 22sp、`</div>` 20sp、`</td>` 18sp；新版全部 +2sp 归一到 td 20 / div 22 / Button 24 / 闭 div 22 / 闭 td 20。
- **当前工作树复核**（Admin.tsx:839-860）：`<td` 20sp、`<div className="flex items-center justify-end gap-2">` 22sp、两个 `<Button` 24sp、`</Button>` 24sp、`</div>` 22sp、`</td>` 20sp——与同表其余 td（:821/:822/:836 开 20 / 内容 22 / 闭 20）完全一致，级差体系 2sp 无破坏。
- **JSX 配对零破坏**：tr > td > div > [Button×2] > /div > /td > /tr 结构完整，npm run build 全绿佐证。

### ② Dashboard.tsx:519 注释「3 门重点看护」→「重点看护课程卡片」——通过

- 当前工作树 :519 精确为 `{/* 预选目标矩阵（重点看护课程卡片） */}`，无「3 门」字样。
- **展示文案一致性核**：:484 显 `{courses.length} 门`（动态目标数），:530 显 `TARGETS`——注释与「动态目标集合」语义一致，不再暗示固定 3 门。
- **全仓「3 门」残留扫描**：仅剩 Dashboard.tsx:482-483（F10-06 解释性注释，说明"去掉 / 3 门"的来龙去脉）与 :529（F10-06 注明"3 门是旧约束残留"）三处，全部是**说明旧约束的注释**而非展示文案，非残留。R69 OBSERVE-69-02 判定方向"只要展示文案清掉、注释保留 F10-06 说明语义即可"与此一致。

### ③ api/client.ts body 层 401 分支缩进（:85-94）——通过

- **diff 铁证**：旧版块内 4 行（`let account`/`if`/`window.dispatchEvent`）比外层低 2sp；新版整块右移 2sp 归 6/8/10 体系。
- **当前工作树复核**（client.ts:85-94）：`if (r.status !== 401)` 6sp、块内 `let account` 8sp、`if (path.includes...)` 10sp、`window.dispatchEvent` 10sp、闭 `}` 6sp——与上下文缩进体系一致。
- **逻辑零变化**：`if (r.status !== 401)` 判断、`extractAccountFromPath` 未复用的内联反查逻辑（此处刻意保留内联 form、非抽出调用，与 R45 N-1 注释语义一致）、广播的 detail 结构与 throw 顺序全部逐字符与 R69 基线一致，纯排版变更。

### ④ Select.tsx 搜索框 aria-label（:862）——通过

- 当前工作树 :861-862 为 placeholder「搜索课程名称、教师或教室」+ `aria-label="搜索课程名称、教师或教室"`，两文本精确一致。
- **全站可达性基线对齐核**：
  - Login.tsx：账号框 `id="login-account"` + `label htmlFor`、密码框 `id="login-password"` + `label htmlFor`、可见性切换按钮 `aria-label` + `aria-pressed`、激活弹窗 `aria-labelledby`——全齐。
  - Admin.tsx：激活码开关 `aria-label="激活码机制"` + `role="switch"` + `aria-checked`、删除弹窗 `aria-labelledby`、ConfigTab 各输入框均有 `<label>` 包裹（接口地址/密钥/模型/并发均 label 在前）——全齐。
  - Select.tsx 搜索框是**唯一**原生 input 直接放 `aria-label`（非 label 关联），与 placeholder 同文本、无重复 aria-label（Select 内仅此一处）。
  - Dashboard 无原生输入控件，不涉及。
- **结论**：全站输入控件均已具备 label/aria-label，无缺口。

### ⑤ 全仓卫生扫描

- `</div>` 相邻 JSX 同行：`grep -rnE '</div>[[:space:]]*<div' src/` **零命中**（R68/R69 判定同正则，持续清零）。另用双正则扫描"换行相邻闭合对"得 21 处，逐一为 CardHeader/div 内层结构正常嵌套（Admin 8 处、Dashboard 6 处、Select 2 处、Toast 1 处等），无 JSX 结构问题。
- 「3 门」残留：见 ②，仅 F10-06 说明注释，非展示文案。
- 新增输入控件 label/aria-label：见 ④，无缺口。

## M-1 延续 + OBSERVE-66-03 setSelected 清点

### 结论

**M-1 第七轮低成本核对：闭合。OBSERVE-66-03 setSelected 调用点清点：五调用点无新增、无第三来源；独立清理 effect 零改动。**

### M-1 核对依据（三消费点 + 置位路径 + 纯函数）

- **三消费点逐字符传 echoedRef.current**（全部为 `shouldDeferSave(stateDataRef.current, <hasSelected>, echoedRef.current)` 三参数签名）：
  - 防抖回调 Select.tsx:686 —— `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :501 —— `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :594 + while :602 —— 两者均为 `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
  - 三个消费点读的均是最新 ref（stateDataRef/selectedRef/revRef），与 R63 M-1 基线一致，第三参数稳态放行语义（echoed=true 放行普通编辑）完整保留。
- **echoedRef 置位路径无回潮**：courses 空分支 :238-239（`echoedRef.current = true; setEchoDone(true)`）、合并完成分支 :295-296（合并/清理之后置位）、account reset :200（置 false）。首帧未到 :232 前置 return 绝不置位；独立清理 effect :316-327 首行 `if (!echoedRef.current || publishes.length === 0) return` 守卫不变。
- **target-guard 18/18 复跑全绿**（含 R63 M-1 两条稳态语义断言：courses 非空+有选中+已回显→放行 / 首帧未到+已回显→仍推迟）。

### OBSERVE-66-03 setSelected 调用点清点（全仓 grep + 逐行核读）

仍为**五个实质调用点，无新增，无第三来源**：

| 行号 | 形式 | 归属 |
|---|---|---|
| :198 | `setSelected({})` | account reset（Sync 事件环，非 updater） |
| :250 | `setSelected((prev) => …)` 函数式 | 回显 effect 合并（唯一函数式 updater 源） |
| :288 | `setSelected((prev) => cleanStaleSelected(…))` 函数式 | 回显 effect 内清理（同 effect 同拍） |
| :320 | `setSelected((prev) => cleanStaleSelected(…))` 函数式 | 独立清理 effect（:316-327） |
| :351 / :361 | `setSelected({ ...selected, … })` 对象式 | 用户 pick（渲染闭包快照） |

- **无第三来源确认**：其余 `setSelected` 命中（:341/:345/:605/:680）均为注释文本非调用。
- **独立清理 effect（:316-327）零改动**：依赖 `[publishes, selected, echoedRef, toast]` + `// eslint-disable-next-line react-hooks/exhaustive-deps` 与 R67 基线一致。
- **亚帧窗口判定维持**：对象式覆盖 vs 函数式合并双来源模型无新增扩散，下游五道防线兜底不变。

## 新发现

### 无 MAJOR / 无 MINOR（连续第十六轮）

通读全部前端源码（App.tsx / Login / Dashboard / Select / Admin / api/client.ts / lib 三件 / components/ui 七件 / types.ts / scripts 四脚本）地毯式排查：状态一致性、异步时序（回显合并/防抖/退避重发/handleBack 收敛）、react-query 缓存一致性（key 含 account 的规范全站一致、无跨账号串数据）、路由切换账号残留（key={account} 双挂载点 + F36-01 兜底守卫）、安全（localStorage 全程 try/catch 降级、token 仅存 xk_sessions / xk_admin_token 且 adminToken 判定绑定会话 token 防撞名穿透、密码输入 `type=password`、ApiError 不泄漏原始 stack、401 广播归因以 session 为准）均无新缺陷证据。

### 新 OBSERVE（纯注释语义一致性，零行为影响）

**OBSERVE-70-01 — Dashboard.tsx:529 F10-06 注释残留「3 门」字样（TARGETS 旁）**

- 位置：web/src/routes/Dashboard.tsx:529 `{/* F10-06：3 门是旧约束残留，现为动态目标集合，展示名改 TARGETS */}` 紧邻 :530 `TARGETS`。
- 一句话：R69 只清了 :519 主注释（「3 门重点看护」→「重点看护课程卡片」），:529 紧贴展示名的说明注释仍保留「3 门是旧约束残留」字样——语义正确（说明旧约束）但视觉上与 :482-483 同款说明共存三处，属可收口的注释冗余。
- 影响面：零行为影响，仅注释整洁度。
- 建议裁决：**续**（随下次前端注释卫生提交顺带归一为一句即可，如「F10-06：展示名 TARGETS = 动态目标集合」；不单独提交）。

**OBSERVE-70-02 — Dashboard.tsx:482-483 与 :529 注释内容重叠**

- 位置：web/src/routes/Dashboard.tsx:482-483（说明去掉 "/ 3 门"）与 :529（说明 TARGETS 的 3 门旧约束）两处注释讲同一件事（F10-06 旧约束清理），分散在两行上下文。
- 一句话：F10-06 的「3 门旧约束」说明重复出现在两个区块注释里，第三处（:519）已由 R69 清理，剩余两处可合并为一处更贴近消费点的说明。
- 影响面：零行为影响。
- 建议裁决：**续**（与 OBSERVE-70-01 合并处理，随下次卫生提交）。

**OBSERVE-70-03 — R69 搜索框 aria-label 与 placeholder 文案同串重复书写**

- 位置：web/src/routes/Select.tsx:861-862。
- 一句话：`placeholder="搜索课程名称、教师或教室"` 与 `aria-label="搜索课程名称、教师或教室"` 逐字符重复，未来改文案需两处同步（轻微维护摩擦）。
- 影响面：零行为影响；当前两串一致、无障碍正确。
- 机制说明：React 的 aria-label 会覆盖 placeholder 派生的可访问名称，同串书写保证两者一致；若不同步会导致读屏与视觉提示分叉。
- 建议裁决：**续**（可接受现状；若追求 DRY 需自定义 hook，属过度工程，不符合简洁优先）。

## 构建验证

| 项 | 结果 |
|---|---|
| `npm run build`（tsc -b + Vite） | ✅ exit 0，1948 modules，dist 正常产出 |
| `node scripts/audit.mjs` | ✅ exit 0，全部通过，视觉表面协调一致 |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| `npx oxlint src` | ⚠️ 12 条 warning，全为既有（R69 13 条中有 1 条已消除？——逐条比对为同一组、无新增） |
| 调试残留 | ✅ 业务 src 零命中（console.log/debugger/TODO/FIXME），Login.tsx:265 的 FIXME 匹配为 placeholder 字面量 `XK-XXXX-XXXX-XXXX` 的 XXX 段误报；scripts/ 下 console.log 均为断言脚本正常输出 |

> **oxlint 12 条 vs R69 13 条说明**：逐条核读 12 条清单与 R69 记录的既有 warning 完全同组（Select.tsx:337/403×2/452/686、useTickingCountdown:17、Toast:24、Dashboard:120/198/263/271、Admin:505），无新增。R69 记录的 13 条含 R69 当时新增的 4 条 OBSERVE 对应位置，本轮逐条复核这 4 处均为修复后位置（Admin 操作列、Dashboard 注释、client.ts 缩进、Select aria-label 均不产生新 lint），故 12 条 = 既有集合，判定无新增加。

## 历轮观察延续

- **M-1**（echoedRef 第三参数稳态语义）：第七轮闭合，三消费点逐字符一致，见上。
- **OBSERVE-66-03**（亚帧竞态 setSelected 五调用点）：无新增、无第三来源，独立清理 effect 零改动，维持「续」。
- **N-1~N-3**（冲刺文案三态 / pick 一拍调度延迟 / 三处格式卫生）：R68/R69 已把 N-3 三处全部落位，N-1/N-2 维持，无新证据升级。
- **O-1~O-12**（Toast viewport 滚动残余 / 倒计时 NaN 防御 / 手写 modal 焦点陷阱 / 管理态多标签页 / Dashboard key 不对称 / ui 模板残宽 / Button dark bg-black / 401 闭包 current / 状态机四态 / M-1 边角语义 / Dashboard nowMs 渲染期 Date.now + extras 1s 陈旧窗口）：逐条复查无升级证据，维持。
- **OBSERVE-69-01~04**（R69 四卫生）：本轮已全部核证修复到位，从延续清单摘除。

## 备注

- 只读铁律全程遵守：未修改任何仓库文件（git status 基线 a799c1b 洁净确认）；临时扫描脚本置于 %TEMP% 已删除。
- 复核过程中未发现与 CLAUDE.md 既有决策锚冲突的证据；全部判定以实测源码为准。
