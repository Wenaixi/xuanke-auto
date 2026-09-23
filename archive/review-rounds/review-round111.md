# review-round111 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + OBSERVE 3 延续 + 新 OBSERVE-111-01**；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续第五十七轮零严重级）。**双端零修复需求**。核心产出：**身份防线矩阵第二十六轮闭合（B101-01 起零产品改动链延续）+ B110-01 修复回首轮通过 + AppendLog 四族审计链闭环 + OBSERVE-111-01 新观察（MarkDone/RemoveDone 返回值丢弃风险点）**。

## 审查发现（写入 archive/review-rounds/round111-{backend,frontend}-findings.md）

### 后端（OBSERVE 3 延续 + 新 OBSERVE-111-01）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-111-01 | OBSERVE 新 | handler.go:377/:457 `_ = d.Sched.MarkDone/RemoveDone` 丢弃状态写回返回值——当前恒返 nil 不构成吞错（内部落库全 `if err != nil { log.Printf }`），但未来若补错误返回值会重蹈契约 17 覆辙 | ⚠️ 立观察不修（主控读原码实证：MarkDone :1973-1978 / RemoveDone :2020-2031 内部落库点全自留痕，`_ =` 丢弃的仅函数外层返回值；未来补错误返回需同步两处调用点） |
| OBSERVE-111-02 | OBSERVE 延续 | B110-02 reloginAt 本地钟写（:1231/:1261）读（:1219/:1227）同基自洽，无混用，对 30s 节流无实质影响 | ⚠️ 维持 |
| OBSERVE-111-03 | OBSERVE 延续 | B109-01 RemoveFull 死方法（scheduler.go:2037-2049）定义外零调用（含前端），真实满员解封已由 releaseFullIfFreedLocked + MarkDone 双路闭环 | ⚠️ 维持（契约 20 不主动删非请求死代码） |
| OBSERVE-111-04 | OBSERVE 延续 | O105-01 抖动基线：api 全量 race 首轮 1 FAIL（测试名未抓到）→ 二三轮绿，zhidao 首轮即绿，夹具 socketPreheat/readyProbe 逐字符零漂移 | ⚠️ 维持（Windows 回环冷启动低频残余既有口径） |
| 身份防线矩阵延续 | — | **第二十六轮闭合**：`git log 20c5882..HEAD -- backend/` 实证仅 1 条提交 = 57bf401（B110-01 审计修复，**非产品行为改动——零产品改动链延续**）；sameClientFor 7 调用点行号零漂移逐点实证；写点全家福 12 类均有防线；偶发写点专项 + 手动四路 accountExists 全覆盖；maybeRelogin 双侧双闭合 | ✅ 闭合 |
| AppendLog 四族审计链 | — | 全写点清点（api 11 处 + scheduler 9 处）：手动（失败三分支 + 成功两行）/自动（全失败分支）/会话/管理四族零静默动作——**B110-01 后审计完备性第三维完整闭环**，任何动作在 /api/logs 可核对 | ✅ 通过（暴露 OBSERVE-111-01） |

### 前端（零严重级 + M-1 第四十七轮闭合 + 零新增）
- **M-1 第四十七轮闭合**：四处消费点（:509/:597/:605/:699）全传 echoedRef.current 第三参逐字符一致 + `shouldDeferSave(` grep 恰 4 消费点无第五消费处 + echoedRef 运行时读点恰 6 处 + 置位三路径（:200/:240/:297）+ 首帧不置位边界（:234 stateData===undefined 前置短路 + :247 pubs 空短路 + :229 回显盾 + :319 清理盾）+ target-guard 18 断言全绿。
- **OBSERVE-93-01 残余面复核（106-01 转闭合后首轮家族册复核）**：7 个带文本裸按钮（Admin :417/:425/:441/:452/:651/:661/:701）逐点位键盘链路完整（原生 button 可聚焦可激活文本可读）、缺口仍纯 display 维度（global.css:174 outline:none 抹原生焦点、focus-visible 全仓恰 4 处、7 处零 ring）；**651/661 引擎二选一带强 active 态仍是优先修复面**（已入家族册，F6-02 迁移统一收敛）；残余面清单零增零减（grep `<button` 17 处与 R110 逐项一致）。
- **F93-01 第十九轮**：Button.tsx:42 ring 逐字符在位零回归 + `git log -5 -- web/` 实证 1351fa4 后前端零代码提交漂移 + build 产物哈希（index-1KHlpqcc.css / index-Bm7TtkV4.js）与 R110 逐字节一致。
- **新契约角度（会话状态机纵深）**：四条会话清理链（logout/401 onUnauthorized/onDeleted/onBackToStudent）对称完整 + 管理令牌标记全路径清除 + 401 双通道广播单发不风暴 + 激活票据链完整——零缺陷。
- **格式卫生**：R67-69 半程态教训零复发。

## 核实记录（关键）
- **B110-01 修复回首轮**（主控读原码实证）：handleElectiveSelect/Exit 失败三分支（:352/:364/:371 + :433/:442/:449）AppendLog 全覆盖 + 错误落库 `if err != nil { log.Printf }` 零吞错；成功路径审计行未回归（MarkDone :1973-1978 SaveSuccess+AppendLog 双记错 / RemoveDone :2020-2031 DeleteSuccess+SaveRefused+AppendLog 三处记错）；新测试真实断言（TestManualElectiveFailureAppendsLog mock 业务失败断 LoadLogs 命中 `!IsOK` + TestManualElectiveReadErrAppendsLog FLUSH+Hijack 直断形态），race 定向跑绿。
- **OBSERVE-111-01 独立复核**：主控读 MarkDone/RemoveDone 原码（:1922-1982/:1989-2035）确认函数体内部全部落库点已 `if err != nil { log.Printf }`——`_ =` 丢弃的仅外层状态写回返回值，非吞错；「未来补错误返回需同步两处调用点」是真实防回归点，量级正确立观察不修。
- **身份防线矩阵第二十六轮**：主控补证 MarkDone（:1927）/RemoveDone（:1995）ClientFor 存在性复核防线在位且注释声明完整（R104 已裁定手动路径同步单 goroutine 无同名重建窗口，存在性复核足够——B104-01 知识位持续生效）。
- **TDD 验证链**：后端代理定向 race 全绿（api 首轮 1 FAIL 低频残余重跑绿 + 身份防线家族 + 脱敏族 + 观察项族）；前端 npm run build EXIT 0 + 四守护脚本全绿 + 产物哈希逐字节一致。

## 观察项延续（下轮复核）
后端：身份防线矩阵（第二十七轮）/ O105-01 抖动基线 / O105-02 删除保护撞名 / B105-01 展示层 / B110-02 reloginAt 时间基 / O106-01 票据 / B109-01 死方法 / **OBSERVE-111-01 MarkDone/RemoveDone 返回值丢弃** / 知识位盯守 / M87-01 窗口；前端：M-1 延续管理（第四十八轮）/ OBSERVE-93-01 残余面（7 个带文本按钮 + 3 个 refetch，651/661 优先修复面）/ 88-01 / 85-02 / 84-01 / 83-01 + 77-02 + 76 族 + O-3 族全表续。

## 教训
1. **「零产品改动链」与「审计链收口」要分判**：`git log 20c5882..HEAD -- backend/` 命中的 57bf401 是审计修复（补 AppendLog）非产品行为改动——身份防线矩阵的「零产品改动」口径是「无新代码引入写点」而非「无提交」，审计链收口不构成防线退化。矩阵复核的判定基准是「写点集合」，不是「提交数量」。
2. **`_ =` 丢弃返回值的风险要「读函数体」判**：OBSERVE-111-01 不是盲报「丢弃返回值=吞错」——主控读 MarkDone/RemoveDone 函数体确认内部落库点全自留痕，`_ =` 丢弃的仅函数外层返回值（当前恒 nil）。**判据 = 函数内部是否已有自己的错误留痕；「未来补错误返回」是防回归点不是当前缺陷**——量级正确立观察不修。
3. **审计完备性第三维闭环后的持续盯守形态**：B110-01 补齐手动失败审计后，AppendLog 四族（手动/自动/会话/管理）零静默动作——稳定期审查对审计链的价值从「找缺口」转为「盯不腐化」（每轮确认零漂移 + 新写点必进族），与身份防线矩阵同构。