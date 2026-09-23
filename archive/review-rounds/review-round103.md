# review-round103 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + B103-01 知识位 + OBSERVE 6 延续**；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / 零新增 OBSERVE**（连续第四十九轮零严重级）。**双端零新增代码修改需求——纯观察轮**。核心产出：**身份防线矩阵第十八轮闭合（连续十八轮无新裸露写点）+ B101-01/B102-01 持续盯守无回归 + B103-01 知识位（实时复核满员分支平台实证不可达）+ tick 主循环状态机复查无漏洞 + 前端 M-1 第三十九轮闭合**。

## 审查发现（写入 archive/review-rounds/round103-{backend,frontend}-findings.md）

### 后端（MINOR 0 + B103-01 知识位 + OBSERVE 6 延续）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| B103-01 | 知识位 | spawnChain 实时复核「确证满员」分支（:1621）在真实平台下实质不可达——平台未下发 countList.maxCount（`CountEntry.MaxCount` 恒 0，`IsClassFull` 恒 false，与 CLAUDE.md「maxCount 判定不存在」锚点一致）；真满员主路径是快照 `classFullInSnapshot`（`max_count` 实证） | ✅ 钉死契约（未来平台下发 maxCount 时该分支身份防线已在位、无需新代码即可活化；唯一需确认的是标记满员业务语义，与快照判满共用 markFullLocked） |
| O103-01 | OBSERVE | 抖动基线第十八轮——定向跑多批（zhidao/scheduler/api/session/store/db/accounts/admin config 族）全绿零 flake，socketPreheat/readyProbe 夹具走读无新脆弱点，~13% 低频口径维持 | ⚠️ 维持（CI `-p 1` + 失败重跑吸收） |
| O103-02 | OBSERVE | 删除保护撞名延续（handler.go:992 单判据 vs B43-04 双条件不对称）——三条历史理由复核仍成立 | ⚠️ 维持观察 |
| O103-03/04/05/06 | OBSERVE | logintest 维持关闭 / M87-01 维持 MINOR / CRLF 维持 / 契约 17 零吞错通过 | ⚠️ 均维持历轮结论 |
| 身份防线矩阵延续 | — | **第十八轮闭合**（sameClientFor 七调用点 :850/:1489/:1521/:1551/:1571/:1600/:1635 全在位；第七分支 ErrUnauthorized 落 :1600 统一复核后才 maybeRelogin、:1621 确证满员在 :1635 身份复核后才 markFullLocked、:1645 末端 doneHas 胜利让位与 :1627 满员 doneHas 守卫对称；maybeRelogin 双侧/ProbeForAccount 识别槽双条件/MarkDone/RemoveDone/SubmitAll/api 手动路径凭据表前置/PurgeAccount 全量清——无新裸露写点） | ✅ 闭合 |
| B101-01/B102-01 盯守 | — | 六测试实测全绿无回归；第九轮全量 grep 确认 doRequest 仍唯一携 token URL 通道且已闭环、五处登录链路辅助请求 URL 无 token | ✅ 通过 |
| tick 状态机新角度 | — | 探测→提交→时钟同步→退避全链路复查：锁序一致（reloginMu→s.mu 全路径）、spawnChain 网络往返全锁外、黄金期 DB 写仅持锁微秒段、windowClosedLocked 三判据单源、reloginResults 非阻塞回传 | ✅ 通过 |

### 前端（零新增 OBSERVE / M-1 第三十九轮闭合）
- **M-1 第三十九轮闭合**（shouldDeferSave 四消费点 :699/:509/:597/:605 全传 echoedRef.current 第三参逐字符一致、置位三路径 + 首帧边界完整、读点代码级引用 9 处无第五消费处）+ 六防保存链零回潮 + target-guard 18/18 实测全绿。
- **F93-01 第十一轮复核**：Button.tsx:42 ring 逐字符在位、`git log f08937e..HEAD -- web/` 实测空集、残余面清单与历轮逐项一致。
- **新契约角度（状态恢复与竞态）**：账号切换（key={account} 双挂载 + 兜底守卫）/ 登出（五态复位）/ 401 剔除（快照式 + targetAccount 双路清除 + 管理态保留）三路全走查无新漏洞。唯一新发现面（手动报名/退选 catch/finally 的 toast + invalidateQueries 无 unmountedRef 守卫）核定为可接受语义（一次性结果反馈、非循环轰炸），仅备注不立条。
- OBSERVE-90-01 / 88-01 / 85-02 / 84-01 / 83-01 / 77-02 / 76 族 / O-3 族延续。

## 核实记录（关键）
- **B103-01 知识位**：代理走读契约链实证（真实 select.js 轮询回调只读 id/selectedCount/auditedCount 三键、全文件 0 处消费 maxCount、两 HAR 无该接口响应样本、IsClassFull 恒 false）→ classFullRealtime 实际恒 `(false, nil/err)` → 实时复核确证满员分支恒不进入；与 CLAUDE.md「maxCount 判定不存在」锚点完全一致。**非缺陷，钉死契约**。
- **身份防线矩阵**：主控 grep 实证 7 调用点与代理逐点走读互证；第七分支读原码确认走统一复核；round41/43 七测试族 refresh 走读（waitChainExit 等待契约、第七条独立红绿）。
- **tick 状态机**：tick（:973-1036）时序自洽——对齐时钟→预热→时钟同步→探测闸门→提交守卫（零值挂起/未到点/WindowClosed/节流），锁序 reloginMu→s.mu 全路径对齐。

## 收尾全量回归
- 后端定向 `go test -race`：zhidao 脱敏/判型/识别 0.872s 全绿 + scheduler 身份防线族全绿 + session/store/db/api（管理后台+会话+撞名+config 热重载族）全绿 + `go vet` 六包零输出（审查代理实测；主控全量 race 在归档后统一跑）
- 前端 tsc EXIT 0 + npm run build（产物哈希与历轮逐字节一致）+ 四守护脚本 18/18 + 6/6 + 5/5 + audit 77/77（审查代理实测）

## 观察项延续（下轮复核）
后端：身份防线矩阵（第十九轮）/ O103-01 抖动基线 / O103-02 删除保护撞名 / B103-01 知识位盯守 / M87-01 窗口；前端：M-1 延续管理（第四十轮）/ OBSERVE-90-01 / 88-01 / 85-02 / 84-01 / 83-01 + 77-02 + 76 族 + O-3 族全表续。

## 教训
1. **知识位与缺陷的分界**：B103-01「实时复核确证满员分支不可达」——不是「代码没覆盖」，而是「平台实证数据形态让该分支恒不进入」。**契约类观察要把「钉死契约」（知识位）与「真实缺口」（缺陷）分开记账**：知识位标注活化条件（平台下发 maxCount），避免未来误当缺陷修、也避免误以为已覆盖。
2. **连续多轮零严重级的价值在「盯守」而非「找新」**：R103 后端核心是 B101-01/B102-01 修复的持续盯守（六测试零回归、doRequest 仍唯一携 token 通道）——稳定期审查的重心从「挖新洞」转向「确认修复不腐化、契约不漂移」。
3. **「不做错误假设」的走读纪律**：tick 状态机复查全部走「读原码 + 定向实测」双证据，无一处凭既往结论略过——连续 18 轮矩阵复核的可靠性正建立在这种每轮不偷懒的逐点确认上。