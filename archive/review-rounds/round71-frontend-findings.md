# Round 71 前端只读审查报告

基线：commit fde2321（R70 收官 docs）。R70 前端注释归一 commit 923d335 为本轮核证对象。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（git status 工作树 `web/` 与 `backend/web/dist` 洁净，`M backend/internal/config/*` 为并行 backend 审查代理改动，与本前端审查无关）。

## 概述

**R70 注释归一 commit 923d335 逐行核证通过（目标语句 :529 已归一且正确引用 :482-483 详情，全仓「3 门」残留仅存 F10-06 说明注释、展示文案零分叉）；M-1 第八轮闭合、OBSERVE-66-03 setSelected 五调用点无新增；构建验证全绿（npm run build exit 0 / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 / audit exit 0 / oxlint 12 条全既有）；零新增 MAJOR / 零 MINOR，仅 2 条新 OBSERVE（均为纯注释/细节语义，零行为影响）。连续第十七轮零严重级发现。**

## R70 注释归一核（commit 923d335 逐行核证）

目标 `web/src/routes/Dashboard.tsx` 单文件 1 行改动，diff 铁证：

```diff
- {/* F10-06：3 门是旧约束残留，现为动态目标集合，展示名改 TARGETS */}
+ {/* F10-06：展示名 TARGETS = 动态目标集合（旧"3 门"约束详见上方注释） */}
```

逐项核证：

- **归一复核**：现 :529 精确为 `{/* F10-06：展示名 TARGETS = 动态目标集合（旧"3 门"约束详见上方注释） */}`。语义完整：F10-06 标签保留、「TARGETS = 动态目标集合」点题、「3 门」降级为"约束详见上方"的指位词不再独立成句。
- **引用关系正确**：:529 明确指向"上方注释"，其上 :482-483 确为 F10-06 完整说明（"去掉 / 3 门——后端上限 100 门且每发布可配多条备选，3 门是早期『每账号至多 3 门』旧约束残留，硬编码展示与真实能力分叉"）——指位准确、无悬空引用，链接语义闭环。
- **「3 门」全仓残留扫描**（`grep -rn "3 门" src/`）：仅 3 处命中，全部为 **F10-06 说明性注释**（:482 / :483 / :529），无一为展示文案。展示侧 :484 显 `{courses.length} 门`（动态目标数）、:530 显 `TARGETS`、:732 移动端 `TARGETS {courses.length}`——注释与"动态目标集合"语义完全一致，零展示分叉。判定：非残留。
- **F10-06 语义完整保留**：三处注释合力完整讲述「旧 3 门约束 → 现动态集合」的来龙去脉，无信息丢失。

**结论：通过。** OBSERVE-70-01 / OBSERVE-70-02（R70 针对 :529 / :482-483 的注释冗余观察）随 commit 923d335 落地归一，本轮完全闭合，从延续清单摘除。

### 全仓卫生扫描

- **相邻 JSX div**：`</div>` 与 `<div` **同行零命中**（精确正则 `</div>[ \t]*<div`，无一行同行相邻）——R70 判定持续维持。换行相邻 28 处（Router 双正则核查全部）：Toast 1 / Admin 9 / Dashboard 12 / Login 1 / Select 5，逐一核读均为 CardHeader/div 包裹的正常嵌套结构（顶栏、倒计时矩阵格、指标行、弹窗内层、配置卡片等），无 JSX 结构问题。
- **新增输入控件 label/aria-label**：全站原生 input 均具备（Login 账号/密码/激活码 均 label htmlFor；Admin 接口/密钥/模型/并发/生成数均 label 包裹；Select 搜索框 aria-label 与 placeholder 同串；激活开关/可见性切换均有 aria-* 标注）——无新增无缺口。

## M-1 延续 + OBSERVE-66-03 setSelected 清点

### 结论

**M-1 第八轮低成本核对：闭合。OBSERVE-66-03 setSelected 调用点清点：五调用点无新增、无第三来源；独立清理 effect 零改动。**

### M-1 核对依据

- **三消费点逐字符传 `echoedRef.current`**（全部为 `shouldDeferSave(stateDataRef.current, <hasSelected>, echoedRef.current)` 三参数签名）：
  - 防抖回调 Select.tsx:686 —— `selectedCount > 0` 与 :683 注释、:678-685 引述 M-1 语义完整。
  - flushTargets :501 —— `latestSelectedCount > 0`，:499-500 注释 R63 M-1 语义完整。
  - handleBack 判定 :594 + while :602 —— 两处均 `hasSelectedNow()`，:591-593/:600-601 注释完整。
  - 三消费点消费前逐字符读最新 ref（stateDataRef/selectedRef/revRef），与 R63 M-1 基线逐字符一致，稳态（echoed=true）放行普通编辑、暂态（false）仍推迟的语义无回潮。
- **echoedRef 置位路径无回潮**：courses 空分支 :238-239、合并完成分支 :295-296、account reset :200（置 false）；首帧未到 :232 前置 return 绝不置位；独立清理 effect :316-327 依赖 `[publishes, selected, echoedRef, toast]` + 首行 `if (!echoedRef.current || publishes.length === 0) return` 守卫 + eslint-disable 注释——与 R67-R70 基线完全一致。
- **target-guard 18/18 复跑全绿**（含 R63 M-1 稳态两条断言：courses 非空+有选中+已回显→放行 / 首帧未到+已回显→仍推迟）。

### OBSERVE-66-03 setSelected 调用点清点

| 行号 | 形式 | 归属 |
|---|---|---|
| :198 | setSelected({}) | account reset |
| :250 | setSelected(prev=>) 函数式 | 回显合并 |
| :288 | setSelected(prev=>cleanStale) 函数式 | 回显内清理 |
| :320 | setSelected(prev=>cleanStale) 函数式 | 独立清理 effect(:316-327) |
| :351/:361 | setSelected(对象式快照) 函数式更新 | 用户 pick |

无新增、无第三来源（其余 setSelected 命中均为注释文本）；亚帧双来源模型无扩散，下游五道防线兜底不变。

## 新发现

### 无 MAJOR / 无 MINOR（连续第十七轮）

通读全部前端源码地毯式排查（App / Login / Dashboard / Select / Admin / client / lib×3 / components/ui×7 / types / scripts×4）：
- **react-query 缓存一致性**：全站 queryKey 均含 `<account>`（App/Select 的标准命、Admin 各 Tab 的 `[admin-x, account, sessionToken]`），无跨账号串数据；Dashboard 与 Select 共享 `["electives",account,sessionToken]` 缓存、互斥挂载零重复请求。
- **路由切换账号残留**：App.tsx 双 Select 挂载点均 `key = {account|targetAccount}` 整体重建 + F36-01 兜底守卫（:195-202），targetAccount 切换、401 吊销、onDeleted 全路径 account 复位自洽。
- **Dashboard.tsx:269-273 折叠种子 effect**：依赖 `[expandedDates, dateGroups]`，dateGroups 由 `state?.courses` 派生、随轮询重建产生新引用，但种子内部 `expandedDates === null` 且 setExpandedDates 置非 null 后再不重种——「null=未初始化播种 / []=用户全折叠不复活」语义在 state 轮询下稳定成立，无退化。
- **安全**：localStorage 全程 try/catch 降级、token 仅存 xk_sessions / xk_admin_token 且 adminToken 判定绑定会话 token 防撞名穿透（isCurrentAdminSession 纯函数）、密码 `type=password`、无 dangerouslySetInnerHTML / innerHTML / eval、ApiError 不泄漏 stack、401 广播归因以 session 为准——均无新缺陷。
- **XSS 敏感性**：无。

### 新 OBSERVE（纯注释/细节语义，零行为影响）

**OBSERVE-71-01 — Select.tsx 中 `selectedCount` 在防抖回调闭包中使用却未入依赖数组**

- 位置：web/src/routes/Select.tsx:686/697/722 读取层 `selectedCount`（防抖 effect 3 处接近开窗），:686:49 为 oxlint 既有 12 条中之一（React Hook useEffect has missing dependencies）。
- 一句话：防抖 effect 依赖含 `selected` 但未含其派生量 `selectedCount`；回调闭包捕获的是 effect 创建时的 selectedCount 快照。
- 影响面：零行为缺陷——selectedCount 由 selected 派生，effect 依赖含 selected（任一次用户 pick 改变 selected 即重建 effect、新版捕获的 selectedCount 同步正确）；selected 不变则 selectedCount 不变，不存在"闭包陈旧但数据新"的分叉。属既有告警，R70 同组记录、已接受。
- 建议裁决：**续**（既有，不新建；如需消除告警可将回调内三处 `selectedCount` 改为读 `selected` 派生或补依赖，属可选项非缺陷）。

**OBSERVE-71-02 — Dashboard.tsx 三处 F10-06 注释仍并存（归一后信息粒度固有）**

- 注释：:482（主解释）/ :483（续行）/ :529（指位行）三处仍需协作才能传达同一 F10-06 旧约束说明；:529 已改为"详见上方注释"指位、主解释收敛于 :482-483。
- 一句话：归一后仍是三行注释讲同一个旧案，可进一步收敛为单处完整 + 单处指位（两步式）或整段上移贴紧展示名。
- 影响面：零行为影响，纯注释整洁度微调。
- 建议裁决：**续**（当前 :529 指位式已满足"不误导、可追溯"；继续收敛属可选项，随下次前端卫生提交顺带处理即可，不单独提交）。

## 构建验证

| 项 | 结果 |
|---|---|
| `npm run build`（tsc -b + Vite） | ✅ exit 0，1948 modules，`../backend/web/dist` 正常产出（dist 被 .gitignore 忽略，不产生工作树变更） |
| `node scripts/audit.mjs` | ✅ exit 0，全部通过（画布 / 玻璃工具 / 实色黑洞清零） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| `npx oxlint src` | ⚠️ 12 条 warning，逐条与 R70 既有集合同组（Select.tsx:337/403×2/452/686、useTickingCountdown:17、Toast:24、Dashboard:120/198/263/271、Admin:505 + audit.mjs:30）——无新增 |
| 调试残留 | ✅ 业务 src 零命中（console.log/console.debug/debugger/TODO/FIXME/XXX/HACK）；Login.tsx:265 的 FIXME 匹配为 placeholder 字面量 `XK-XXXX-XXXX-XXXX` 的 XXX 段误报；scripts/ 下 console.log 均为断言脚本正常输出 |

临时验证脚本置 %TEMP%（r71_divscan.mjs / r71_verify.mjs）已删除；构建产物 dist 已被 gitignore 忽略，无需额外清理工作树本干净。

## 历轮观察延续

- **M-1**（echoedRef 第三参数稳态语义）：第八轮闭合，三消费点逐字符一致，见上。
- **OBSERVE-66-03**（亚帧 setSelected 五调用点）：无新增、无第三来源，独立 effect 零改动，维持「续」。
- **N-1~N-3**（三态冲刺文案 / pick 一拍调度延迟 / 格式卫生）：R68/R69 已把 N-3 落位，N-1/N-2 维持，本轮无新证据升级。
- **O-1~O-12**（Toast viewport / 倒计时 NaN / 手写 modal 焦点 / 管理多标签 / Dashboard key 不对称 / ui 模板残宽 / Button dark bg-black / 401 闭包 current / 状态机四态 / M-1 边角 / Dashboard nowMs 渲染期 Date.now + extras 1s 陈旧窗口）：逐条复查无升级证据，维持。
- **OBSERVE-70-01/02/03**（R70 三观察）：01/02 随 commit 923d335 归一闭合，从清单摘除；03（Select aria-label 与 placeholder 同串书）维持「续」。
- **oxlint 12 条既有**：全为既有集合，无新增，维持。

## 备注

- 只读铁律全程：未修改任何仓库文件（`git status` 下 `web/` 与 `backend/web/dist` 全干净，`M backend/config/*` 为并行 backend 审查代理改动）。
- 复核与 CLAUDE.md 既有决策锚零冲突（开放时间事实源、F10-06 动态集合、M-1 稳态载入、target-guard 判据完全对齐源实现）。