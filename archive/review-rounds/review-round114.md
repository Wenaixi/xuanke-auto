# review-round114 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新 OBSERVE**（OBSERVE-105-01/111-01/112-03 延续）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新 OBSERVE + OBSERVE-76-03 措辞二勘定案**（连续第五十七轮零严重级）。**双端零修复需求——连续第四轮纯观察轮**。核心产出：**身份防线矩阵第二十九轮闭合 + B110-01 审计链第四轮零漂移 + OBSERVE-76-03 措辞最终定案（Radix Toast Close 无自动注入标签无可访问名）+ cmd 三工具契约新角度**。

## 审查发现（写入 archive/review-rounds/round114-{backend,frontend}-findings.md）

### 后端（零缺陷轮，OBSERVE 3 延续）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-105-01 | OBSERVE 延续 | O105-01 抖动基线：api 首轮 1 FAIL（TestHandleElectivesSelectUnauthorizedRelogin connectex 16.98s）→ 单测重跑绿 1.56s + api 全包重跑绿 22.7s——已知基线低频残余 | ⚠️ 维持（重跑绿不立条） |
| OBSERVE-111-01 | OBSERVE 延续 | MarkDone/RemoveDone `_ =`（:377/:457）恒 nil 非吞错；落库点全 `if err != nil { log.Printf }` | ⚠️ 维持 |
| OBSERVE-112-03 | OBSERVE 延续 | 手动失效分支与自动链同文案双日志——动作维度 select/exit 可区分 | ⚠️ 维持 |
| 身份防线矩阵延续 | — | **第二十九轮闭合**：`git log 20c5882..HEAD -- backend/` COUNT=1（57bf401 审计修复，零产品改动链延续第九轮）；sameClientFor 7 调用点零漂移；写点全家福 12 类逐一对应防线；偶发写点专项；手动四路 accountExists（:245/:295/:387/:563 定义 :1099）+ maybeRelogin 双侧（:1208/:1254） | ✅ 闭合 |
| 新契约角度 | — | cmd 三工具契约逐点对齐（bench -n≤0 参数防御 / probe XUANKE_PROBE_TOKEN env 读取 + 15s 超时 + SharedTransport + 只打 body 前 600 字节不泄 token / logintest limit 收敛 + 识别引擎回退链 + token[:min(8,len)]） | ✅ 通过 |

### 前端（M-1 第五十轮闭合 + 零新 OBSERVE + OBSERVE-76-03 措辞二勘）
- **M-1 第五十轮闭合**：四处消费点（:509/:597/:605/:699）全传 echoedRef.current 第三参逐字符一致 + shouldDeferSave 恰 4 消费点 + echoedRef 读点 9 处 + 置位三路径 + 首帧不置位边界 + target-guard 18/18 实测全绿。
- **OBSERVE-93-01 第四轮家族册复核**：grep `<button` 全仓 17 处零增零减；7 带文本裸按钮 + Toast Close 键盘链路完整缺口纯 display；651/661 优先修复面；**twMerge 冲突实测不吞 focus-visible ring 家族**（跨键保留语义在位）。
- **F93-01 第二十二轮**：Button.tsx:42 ring 在位 + git log/diff 1351fa4..HEAD -- web/ 双空（web/ 零漂移）。
- **OBSERVE-76-03 措辞二勘定案（主控独立实证）**：R113 勘校「Radix 内建隐式西文 Close」不实——主控读 Radix react-toast dist index.js:569-583 实证 ToastClose 构造 `Primitive.button` 仅注入 type + onClick，**:226 aria-label 仅 Provider/hotkey 非 Close**；lucide X 带 aria-hidden（renderToStaticMarkup 实测）→ Close 按钮确无可访问名。**最终措辞 = 「Toast Close 无自动注入标签、按钮无可访问名（X 图标 aria-hidden + Radix 无注入）——display 维度缺口」**。勘校方向（非缺陷）保留、细节修正；并入 F93-01 家族册结论不变。
- **新契约角度（Dashboard 折叠段/数据消费与分组链 + Radix Toast 深挖）**：dateGroups 三兜底链完整（关闭≠元数据丢失契约）+ 折叠种子三态（null/[]/组依赖语义分离）+ 派生纯函数零缺陷；Radix Toast 轻确认层与三模态焦点陷阱族独立、无焦点回归。
- **OBSERVE-112-02 维持**：query.state 非闭包 + /state 3s 刷新必重渲染。

## 核实记录（关键）
- **OBSERVE-76-03 二勘独立复核**：主控读 node_modules/@radix-ui/react-toast/dist/index.js:569-583 逐行实证 ToastClose 声明式 button 无 aria 注入 + :226 aria-label 为 Provider/hotkey（ToastAction label 注入）——R114 代理再勘校成立，R113 细节修正，最终措辞定案纳入归档。
- **B110-01 审计链第四轮**：`grep -c 'req.ClassID, "select"'` = 3 + `'"exit"'` = 3（六处 AppendLog 全在位零吞错）+ 成功路径审计行在位（MarkDone :1976 / RemoveDone :2029）。
- **身份防线矩阵第二十九轮**：代理逐点 grep 实证 + 主控补读确认（sameClientFor 定义 :204 clientIdentity :212-223、:1099 accountExists 定义同源）。
- **TDD 验证链**：后端 go build/vet PASS + 定向 race（api+zhidao 首轮 api 1 connectex 低频残余重跑全绿）；前端 npm run build EXIT 0 + 四守护脚本全绿（18/6/5/audit）+ twMerge 冲突实测。

## 观察项延续（下轮复核）
后端：身份防线矩阵（第三十轮）/ O105-01 抖动基线 / O105-02 删除保护撞名 / B105-01 展示层 / B110-02 reloginAt 时间基 / O106-01 票据 / B109-01 死方法 / OBSERVE-111-01 + 112-03 盯守（第四轮）/ 知识位盯守 / M87-01 窗口；前端：M-1 延续管理（第五十一轮）/ OBSERVE-93-01 残余面（7 带文本 + 3 refetch + Toast Close，651/661 优先修复面）/ **OBSERVE-76-03 措辞已二勘定案「无自动注入标签无可访问名」**/ OBSERVE-112-02 / 88-01 / 85-02 / 84-01 / 83-01 + 77-02 + 76 族 + O-3 族全表续。

## 教训
1. **观察项措辞勘校可以是两轮迭代**：R112 立「Toast Close 无 aria-label」→ R113 勘校「Radix 内建隐式西文 Close」→ R114 再勘「Radix 无自动注入、按钮无可访问名」——**每一版措辞都要有源码级实证背书**（主控 grep dist :569-583 + :226 独立复核第三次实证）。观察项的事实基础随轮次收敛是正常的，关键是「措辞与实现级事实一致」而非「哪一轮说得对」。
2. **twMerge 等工具链行为要实测**：代理用 node 实测 twMerge 不吞 focus-visible ring 家族（跨键保留）——「工具是否会覆盖我的样式类」这类假设必须实证而不是凭感觉（R93 曾因组件层与全局 reset 冲突误判，同族教训）。
3. **cmd 三工具契约是稳定期新角度的可持续面**：bench/probe/logintest 三个 CLI 工具（token 脱敏/env 替代硬编码/参数边界/引擎回退链）在连续多轮零产品改动后仍是「有内容可审」的面——工具面与产品面分开盯守是稳定期审查的资源分配经验。