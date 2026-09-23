# review-round102 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 5 延续 + B102-01 知识位**；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / 零新增 OBSERVE**（连续第四十八轮零严重级）。**双端零新增代码修改需求——纯观察轮**（后端原代理审查超时被停止、替代代理带时限完成；前端连续第九轮零修改）。核心产出：**身份防线矩阵第十七轮闭合 + B101-01 修复回首轮通过 + B102-01 遗漏透传路径逐点扫描（同族缺口不存在）+ 前端 M-1 第三十八轮闭合**。

## 审查发现（写入 archive/review-rounds/round102-{backend,frontend}-findings.md）

### 后端（MINOR 0 + B102-01 知识位 + OBSERVE 5 延续）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| B102-01 | OBSERVE 知识位 | 登录链路五处裸 `sess.Do`（Prewarm/SyncServerTime/fetchLoginPage/fetchCaptchaImage/submitLogin）错误文本带完整 URL 但不带 idToken——逐点枚举确认 **doRequest 是唯一把 token 拼进 URL 的通道且已闭环**，同族缺口不存在 | ✅ 钉死无缺陷（存知识位：未来平台把 token 移入 header/body 需在提交方重新审计） |
| O102-01 | OBSERVE | 抖动基线第十七轮——定向跑（脱敏+判型+识别+scheduler 身份防线族）零 flake，双保险夹具走读无新脆弱点，~13% 低频口径维持 | ⚠️ 维持（CI `-p 1` + 失败重跑吸收） |
| O102-02 | OBSERVE | 删除保护撞名延续（handler.go:992 单判据 vs B43-04 双条件不对称）——三条历史理由复核仍成立 | ⚠️ 维持观察 |
| O102-03/04/05/06 | OBSERVE | logintest 分叉维持关闭 / M87-01 窗口维持 MINOR / CRLF 维持 / 契约 17 零吞错通过 | ⚠️ 均维持历轮结论 |
| 身份防线矩阵延续 | — | **第十七轮闭合**（主控 grep 实证 sameClientFor 七调用点 :850/:1489/:1521/:1551/:1571/:1600/:1635 + 第七分支 ErrUnauthorized 落 :1600 统一复核后才 maybeRelogin；替代代理补复核 maybeRelogin 双侧/ProbeForAccount 识别槽双条件/MarkDone/RemoveDone/SubmitAll/PurgeAccount 全量清——无新裸露写点） | ✅ 闭合 |
| B101-01 回首轮 | — | sanitizeError 剥 URL 保留判型与可读性、business 错误原样透传、六测试 + 端到端 connectex 实测全绿 | ✅ 通过 |
| keep-alive 账号边界 | — | TCP/TLS 连接无账号态，隔离由逐请求 URL+Cookie 承载，无跨账号串线旁路 | ✅ 通过 |

### 前端（零新增 OBSERVE / M-1 第三十八轮闭合）
- **M-1 第三十八轮闭合**（shouldDeferSave 四消费点 :509/:597/:605/:699 全传 echoedRef.current 第三参逐字符一致、置位三路径 + 首帧边界完整、读点代码级引用 9 处无第五消费处）+ 六防保存链零回潮 + target-guard 18/18 实测全绿。
- **F93-01 第十轮复核**：Button.tsx:42 ring 逐字符在位、`git log f08937e..HEAD -- web/` 实测空集（web/ 零代码提交）、残余面清单与历轮逐项一致。
- **新契约角度四项全走查无发现**：错误与边界 UI 一致性（管理五 Tab 四态完整、Select 保存失败必有 toast）/ 表单幂等与重入（八类提交入口在飞守卫 + disabled + 去重全覆盖）/ 深链与刷新（无路由库、localStorage 六处 try/catch、管理态恢复唯一判据实测 6/6）/ 视觉与状态双向同步（课程卡数量态同源、audit 77/77）。
- OBSERVE-90-01 里程碑第五轮评估维持不落地 / 88-01 第六轮维持 / 85-02 / 84-01 / 83-01 / 77-02 / 76 族 / O-3 族延续。

## 核实记录（关键）
- **B101-01 回首轮**：代理走读 sanitizeError（client.go:581-593）+ sanitizerErr（:566-572）+ doRequest 接入点 :450 + 六测试实测全绿——errors.As/Is 判型未破坏（isConnErrRetryable/IsReadErr 对 net.OpError/io.EOF/超时穿透）、非 url.Error 业务错误原样透传、剥 URL 后可读性保留。
- **B102-01 遗漏透传路径扫描**：逐一枚举 zhidao 包全部 http 直调点——doRequest 唯一携 token URL 通道且已闭环；五处辅助请求（Prewarm/SyncServerTime/fetchLoginPage/fetchCaptchaImage/submitLogin）URL 均无 idToken（登录期独立会话、URL 恒静态），连接层错误文本进日志不泄露凭证；submitLogin 业务拒绝文案不携 URL/body（identification 为 RSA 密文）。**同族缺口不存在**。
- **主控 spot-check 与代理互证**：sameClientFor 七调用点主控 grep 实证 + 代理逐点走读一致；第七分支 ErrUnauthorized 读原码确认走统一复核后才 maybeRelogin。

## 收尾全量回归
- 后端定向 `go test`：zhidao 脱敏/判型/识别 2.985s 全绿 + scheduler 身份防线族 9.333s 全绿 + `go vet` 零输出（审查代理实测；主控 race 全量在 R103 启动前统一跑）
- 前端 tsc EXIT 0 + 四守护脚本 18/18 + 6/6 + 5/5 + audit 77/77（审查代理实测）

## 观察项延续（下轮复核）
后端：身份防线矩阵（第十八轮）/ O102-01 抖动基线 / O102-02 删除保护撞名 / B102-01 知识位盯守 / M87-01 窗口；前端：M-1 延续管理（第三十九轮）/ OBSERVE-90-01 / 88-01 / 85-02 / 84-01 / 83-01 + 77-02 + 76 族 + O-3 族全表续。

## 教训
1. **审查代理可能陷入无界度探索**：R102 原后端代理 65+ 分钟未产出（远超历轮上限）、findings 未写入、last action 仍在校验 R100/R101 文件——判断卡死应设明确阈值（历轮正常上限 × 1.5）而非无限期等待；停止后派带硬性时限的替代代理（25~30 分钟 + 明确聚焦清单）是低成本接管方式。
2. **「有 URL 无敏感」与「有 URL 有敏感」是同族不同面**：B102-01 逐点枚举确认登录链路五处辅助请求 URL 无 token——脱敏契约的审查要区分「URL 是否携带凭证」（doRequest 唯一携 token 通道已闭环）与「URL 文本是否回放」（全部 URL 都会回放但无敏感）。**契约锚定在「token 走 URL 参数通道」现状上，未来平台改通道需重新审计**（已存知识位）。
3. **keep-alive 连接池账号边界**：连接无账号态、隔离由逐请求 URL+Cookie 承载——「共享连接池会不会串账号」的审查要回到「连接是否有账号态」的本质，而非凭共享之名猜疑。