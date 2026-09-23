# review-round113 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新 OBSERVE**（OBSERVE-105-01/111-01/112-03 延续）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新 OBSERVE**（连续第五十六轮零严重级 + OBSERVE-76-03 勘校）。**双端零修复需求——连续第三轮纯观察轮**。核心产出：**身份防线矩阵第二十八轮闭合 + B110-01 审计链第三轮零漂移 + OBSERVE-76-03 措辞勘校（Toast Close 实为 Radix 内建隐式 aria-label）+ 前端定时器资源账零泄漏**。

## 审查发现（写入 archive/review-rounds/round113-{backend,frontend}-findings.md）

### 后端（零缺陷轮，OBSERVE 3 延续）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-105-01 | OBSERVE 延续 | O105-01 抖动基线：本轮 api+zhidao 全量 race 双双 PASS（zhidao 低频残余 R112 首轮 1 FAIL 本轮未复现） | ⚠️ 维持（低频残余有波动性，多轮复现同根才升格） |
| OBSERVE-111-01 | OBSERVE 延续 | MarkDone/RemoveDone `_ =`（:377/:457）恒 nil 非吞错；落库点全 `if err != nil { log.Printf }`；防回归锚点 = 未来补错误返回需同步两处调用点 | ⚠️ 维持 |
| OBSERVE-112-03 | OBSERVE 延续 | 手动失效分支与自动链同文案双日志——动作维度 select/exit 可区分不构成重复留痕 | ⚠️ 维持 |
| 身份防线矩阵延续 | — | **第二十八轮闭合**：`git log 20c5882..HEAD -- backend/` COUNT=1（57bf401 审计修复，零产品改动链延续）；sameClientFor 7 调用点行号零漂移（:204/:850/:1489/:1521/:1551/:1571/:1600/:1635）；写点全家福 12 类逐一对应防线；偶发写点专项（MarkTokenValid/TryAcquireSubmit/releaseFullIfFreedLocked/SubmitAll）；手动四路 accountExists + maybeRelogin 双侧（:1208/:1254） | ✅ 闭合 |
| 新契约角度 | — | zhidao 层 doRequest 全调用点契约（FindElectives/YearTerms/SelectClass/ExitClass/StudentCounts 全经 doRequest + sanitizeError/httpDo/IsReadErr 三函数全覆盖）+ scheduler 锁序（reloginMu→s.mu 恒定无反向嵌套 + chainMu 独立不嵌套 + classFullRealtime 明确锁外网络） | ✅ 通过 |

### 前端（M-1 第四十九轮闭合 + 零新 OBSERVE + OBSERVE-76-03 勘校）
- **M-1 第四十九轮闭合**：四处消费点（:509/:597/:605/:699）全传 echoedRef.current 第三参逐字符一致 + `shouldDeferSave(` grep 恰 4 消费点 + echoedRef 读点清点（代码级引用 9 处 = 置位 3 + 守卫 2 + 消费 4）+ 置位三路径 + 首帧不置位边界 + target-guard 18/18 实测全绿。
- **OBSERVE-93-01 第三轮家族册复核**：grep `<button` 全仓 17 处与历轮基线零增零减；7 个带文本裸按钮 + Toast Close 逐点位键盘链路完整、缺口仍纯 display 维度；651/661 引擎二选一带强 active 态优先修复面；**Toast Close 经 Radix 内建隐式 aria-label（grep react-toast index.js:226 实证）无需补**。
- **F93-01 第二十一轮**：Button.tsx:42 ring 在位 + `git log 1351fa4..HEAD -- web/` 空 + `git diff --stat` 空（web/ 零代码漂移双实证）。
- **OBSERVE-112-02 维持**：/state refetchInterval 用 query.state 非闭包 + /logs 3s 刷新必重渲染闭包不陈旧。
- **新契约角度（定时器资源账）**：22 处 setTimeout/setInterval 全生命周期清理闭环零泄漏 + 表单 htmlFor↔id 9 对全配对零悬空 + 三处 role=alert 与同款 ring 令牌在在位。
- **OBSERVE-76-03 勘校（主控批准）**：Toast.tsx:100 Close 实为 Radix ToastPrimitive.Close 内建隐式 aria-label（西文 "Close"）——「无 aria-label」措辞修正为「无中文 aria-label（西文隐式）」，display 维度不变；并入 F93-01 家族册（R112 已成册）结论维持。

## 核实记录（关键）
- **OBSERVE-76-03 勘校独立复核**：主控 grep node_modules/@radix-ui/react-toast/dist/index.js:226 实证 Radix Close 内建 aria-label 机制在位——代理勘校建议成立，措辞修正纳入归档。
- **B110-01 审计链第三轮**：手动失败三分支 AppendLog 六处（:352/:364/:371 + :433/:442/:449）全在位零吞错 + 成功路径审计行未回归（MarkDone :1976 / RemoveDone :2029）。
- **OBSERVE-111-01/112-03**：`_ =` 恒 nil 非吞错 + 双文案动作维度可区分——零漂移确认。
- **TDD 验证链**：后端代理 go build/vet PASS + 三组定向 race 全绿（身份防线家族 + 迁移 + doRequest 契约）；前端 npm run build EXIT 0 + 四守护脚本全绿（18/6/5/audit）。

## 观察项延续（下轮复核）
后端：身份防线矩阵（第二十九轮）/ O105-01 抖动基线 / O105-02 删除保护撞名 / B105-01 展示层 / B110-02 reloginAt 时间基 / O106-01 票据 / B109-01 死方法 / OBSERVE-111-01 + 112-03 盯守 / 知识位盯守 / M87-01 窗口；前端：M-1 延续管理（第五十轮）/ OBSERVE-93-01 残余面（7 带文本 + 3 refetch + Toast Close，651/661 优先修复面）/ OBSERVE-76-03 措辞已勘校（西文隐式 aria-label）/ OBSERVE-112-02 / 88-01 / 85-02 / 84-01 / 83-01 + 77-02 + 76 族 + O-3 族全表续。

## 教训
1. **审查勘校的价值 = 修正既有观察项的「事实措辞」**：OBSERVE-76-03「Toast Close 无 aria-label」被 R113 代理实证推翻——Radix Close 内建隐式 aria-label（西文），实为「无中文 aria-label」。**观察项的「事实基础」同样要接受后续轮次的实证勘校（与 R107 主控自卫同类）**——措辞不修正会让未来轮次基于错误事实重复评估。主控核实（grep node_modules 源码 :226）后批准勘校。
2. **「零新增 OBSERVE」连续三轮的稳定形态**：R111/R112/R113 三轮全零修复 + 前端连续第五十六轮零严重级——第三轮确认稳定期审查「宁缺毋滥」纪律的产出是观察项增量并入册 + 措辞勘校 + 矩阵核位不漂移，不是每轮硬找缺陷。
3. **doRequest 全调用点收口是 zhidao 层契约可复核性的基础**：FindElectives/YearTerms/SelectClass/ExitClass/StudentCounts 全经 doRequest + sanitizeError/httpDo/IsReadErr 三函数契约在位——「单点收口」让脱敏/重试/判型三类横切契约的一次性审计成为可能，这是 R52-R61 系列连接层修复的技术遗产，需持续盯守不腐化。