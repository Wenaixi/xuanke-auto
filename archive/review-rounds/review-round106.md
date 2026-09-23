# review-round106 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + OBSERVE 1（O106-01 新）+ 知识位 1（B106-01）**；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 新增 OBSERVE 2（OBSERVE-106-01/106-02）**（连续第五十二轮零严重级）。**后端零新增代码修改（O106-01 量级可控维持观察）；前端落地 1 条实质修复（OBSERVE-106-02 日志失败态伪装空态，f5fdb36）**。核心产出：**身份防线矩阵第二十一轮闭合（全家福聚类复核）+ O106-01 票据内存态观察 + 激活码生命周期闭环走查 + 前端 OBSERVE-105-01 转闭合 + OBSERVE-106-02 修复 + M-1 第四十二轮闭合**。

## 审查发现（写入 archive/review-rounds/round106-{backend,frontend}-findings.md）

### 后端（MINOR 0 + OBSERVE 1 新 + 知识位 1 + 延续）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| O106-01 | OBSERVE 新 | 激活票据依赖内存态（store.go:102-138 只碰内存 tickets map）、5 分钟 TTL 只在消费时判过期、无钟表清扫（过期票不主动回收、重启即丢）——量级可控（每票 ~200+B）+ 无安全后果（票熵 32 字节 crypto/rand + 先 ConsumeTicket 防穷举封死） | ⚠️ 维持低风险观察（N 账号部署下短时大量未激活登录可累积过期票，属可控量级；运维重启空窗内已拿票账号重登再激活——概率极小非灾难） |
| B106-01 | 知识位 | ddddocr 两本地引擎（native_ocr.go:108/local_ocr.go:34）同样套全局并发限流 withConcurrency——`captcha_concurrency` 热改对全引擎生效，管控「平台熔断」的并发收敛面完整 | ✅ 落档 |
| O106-02 | OBSERVE（新角度） | 激活码生命周期闭环——票据先占/已激活回滚不扣/激活一次永久免激活/DeleteAccount 清 activations 行，**闭环无缺陷**，三处交接语义注释在案 | ✅ 闭合 |
| 身份防线矩阵延续 | — | **第二十一轮闭合**（sameClientFor 7 调用点零漂移 + 写点全家福聚类复核：openTimeDetected/acctData/tokenValid/done/refused/rateLimited/full/inflight/Courses/acctTargets 全部对应防线——无新裸露写点） | ✅ 闭合 |
| 登录状态机二查 | — | submitLogin 四字段/R96 一致、Login 主循环 maxCaptchaAttempts=3、成功 SetCredentials、gate 全链路 2 次/分钟——无漂移 | ✅ 通过 |
| 知识位盯守 | — | B101-01 六测试 race 全绿、doRequest :422 仍唯一 token 通道、B102-01/B103-01/B104-01 无退化、B105-01 维持观察 | ✅ 通过 |

### 前端（零严重级 + OBSERVE-105-01 转闭合 + OBSERVE-106-02 修复）
- **OBSERVE-105-01 转闭合**——Admin.tsx:470/:473 `aria-label="复制"/"删除"` 逐字符在位、`git log a12a09c..HEAD -- web/` 实测空集、build css 哈希与 R105 逐字节一致——第三道无障碍修复确认稳固转闭合（观察项生命周期继续运转）。
- **OBSERVE-106-02（新，实锤→修复）**：Dashboard 学生端日志区 `logs && logs.length > 0 ? … : "NO RECENT LOGS"` 二态——`/logs` 查询失败（isError 无缓存）时把**加载失败伪装成无日志**，调度日志静默隐身（对照 Admin LogsTab 同场景显「日志加载失败（网络异常或服务端不可达）」）；加载中亦无独立态。修复：日志区改四态（logsErr→失败文案 / !logs→加载中 / 有数据→列表 / 空→NO RECENT LOGS），display 级无行为面。
- **OBSERVE-106-01（新，维持观察）**：Admin 7 个带文本裸按钮（:417/:425/:441/:452/:651/:661/:701）无 focus-visible ring——历轮残余面清单遗漏补录（带文本读屏可读故无 aria-label 缺口、但焦点环仍缺），随 OBSERVE-93-01 残余面族维持观察。
- **M-1 第四十二轮闭合**（四处消费点全传 echoedRef.current 逐字符一致 + 读点 6 处无第五消费处 + 六防零回潮）+ target-guard 18/18 + admin-auth + unauthorized + tsc EXIT 0 实测全绿。
- **F93-01 第十四轮复核**：Button.tsx:42 ring 逐字符在位零回归 + 残余面清单与历轮一致 + 本轮补录 7 个带文本裸按钮。
- **新契约角度（表单与交互走查）**：六类提交幂等双闸全链闭合（登录/激活/目标保存/报名退选/配置/删号删码——入口幂等 + disabled 双闸，目标保存有指数退避 5 次停手、配置加载完成前绝不保存）+ 错误恢复族全覆盖 + 键盘可达族达标。

## 核实记录（关键）
- **O106-01 量级与安全双评估**：主控核实 store.go:102-138（CreateTicket/ConsumeTicket 只碰内存 map）+ handler.go:153-155（登录未激活 CreateTicket）——票不落库重启即丢（运维重启空窗极小）、5 分钟 TTL 消费时判过期（无钟表清扫）、每票 ~200B 可控量级、票熵 32B crypto/rand + 先占用防穷举。**低风险观察，不制造修复**。
- **OBSERVE-106-02 实锤**：主控读 Dashboard.tsx:696-718 确认二态 + Dashboard.tsx:148-162 确认 logs 查询有 error 态（refetchInterval 已按 error 降频 30s）但渲染无分支——失败伪装空态真实存在。四态修复 + tsc/三组断言/audit 全绿。
- **激活码闭环**：ConsumeActivationCode 单事务 UPDATE 扣次→查已激活→回滚不扣；已激活持真码回滚（票作废、码未扣、号未复激活）——无半生效窗口。
- **TDD 验证链**：前端 tsc EXIT 0 + target-guard 18/18 + admin-auth 6/6 + unauthorized 5/5 + audit 全绿（主控实测）。

## 收尾全量回归
- 后端定向 `go test -race`：scheduler 身份防线族 5.16s + api 手动四路/激活族 3.22s/1.95s + zhidao 脱敏族 2.08s + session/db + `go vet` 三包零输出（审查代理实测；主控全量 race 归档后跑）
- 前端 tsc EXIT 0 + build + 三组断言 18/18 + 6/6 + 5/5 + audit 全绿（主控实测）

## 观察项延续（下轮复核）
后端：身份防线矩阵（第二十二轮）/ O106-01 票据内存态 / O105-01 抖动基线 / O105-02 删除保护撞名 / B105-01 展示层 / 知识位盯守 / M87-01 窗口；前端：M-1 延续管理（第四十三轮）/ OBSERVE-106-02 修复回首轮 / OBSERVE-93-01 残余面（本轮补录 7 个带文本按钮）/ OBSERVE-90-01（106-02 修复后学生会话日志区四态已对齐——90-01 关注点部分闭合）/ 88-01 / 85-02 / 84-01 / 83-01 + 77-02 + 76 族 + O-3 族全表续。

## 教训
1. **「失败伪装空态」是展示层最隐蔽的误导**：OBSERVE-106-02 是「加载失败显 NO RECENT LOGS」——用户看到「无日志」会以为系统静默运行正常，实则日志信道已断。**对照 Admin LogsTab 四态发现不对称是审查的显影剂**——同一数据源在管理端与学生端的呈现分叉，往往是缺陷信号。另一教训：OBSERVE-90-01 挂了多轮的「日志区三态不对称」在本轮细化出具体危害面才转成修复——观察项到危害面成立时要果断转修复，不无限期挂账。
2. **「量级 + 安全」双评估决定观察是否升级**：O106-01（票据内存态）看似该修——但量级可控（每票 200B）+ 安全封死（crypto/rand 熵 + 先占用防穷举），实质无修复阈值。**观察项升级判据 = 实际危害 × 发生概率，不是「代码看起来不优雅」**。
3. **观察项生命周期已成熟**：OBSERVE-105-01 转闭合（第三道修复确认）+ 106-02 转修复 + 106-01 维持观察——「新立→跟踪→修复/升级/转闭合」的决策纪律连续六轮稳定运转，列表不膨胀、每轮有出清。