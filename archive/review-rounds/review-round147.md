# review-round147 总结（2026-09-24）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + LOW 0（无新增）**（身份防线矩阵**第六十二轮**闭合 + OBSERVE-117-01 知识位第三十轮确认在位 + B110-01 审计链第三十七轮零漂移 + O105-01 实测绿 + LOW-132/133 修复回首通过）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + OBSERVE 4（维持）**（连续第八十四轮零严重级）。**双端零修复需求——连续第三十七轮零 MAJOR，本轮纯观察**。核心产出：**身份防线矩阵第六十二轮闭合（窗口三判据单源/httpDo 判型纵深）+ 前端 M-1 第八十三轮闭合 + 在飞守卫族/复制降级链纵深**。

## 审查发现（写入 archive/review-rounds/round147-{backend,frontend}-findings.md）

### 后端（零缺陷轮 + 纵深族走查）
| 项 | 核实裁决 |
|---|---|
| 身份防线矩阵第六十二轮 | ✅ 闭合：sameClientFor 定义 :204-210（反射指针比对）+ 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）零漂移（grep 全仓库确认无第四处）；每条调用点追到终局写点（done/full/rateLimited/openTimeDetected/acctData/AppendLog/SaveSuccess/markRateLimited/markFull 等）；写点换类 5 类全持锁 + warnedNoTargets（:1371-1372）唯一无锁点宿主唯一性已证（submitAll 仅由 tick 单 goroutine 调用 + -race 实证） |
| OBSERVE-117-01 知识位第三十轮 | ✅ 在位：maybeRelogin 决策侧 :1208 复核 / 写回侧 :1254 复核 / :1265-1273 二次 ClientFor 重取当前 token 落库，同名重建场景下旧链被前置同锁内复核拦截，结构上不可能出现旧身份写新 token |
| B110-01 审计链第三十七轮 | ✅ 零漂移：手动 6 失败位 AppendLog 逐一在位；零吞错穷举仅 handler.go:387/:467 两处恒 nil 合规豁免；脱敏延续（sanitizeError + maskedToken）判型穿透族八形态实测全 PASS |
| O105-01 抖动基线 | ✅ 实测绿：socketPreheat/readyProbe 夹具在位；四包定向 race 全绿（scheduler 15.258s / api 14.590s / zhidao 2.095s / accounts 1.383s）+ 全仓库 CGO=1 race 13 包全绿 + 双回归锚 PASS |
| **LOW-132-01 / LOW-133-01 回首核** | ✅ 修复在位且绿：time.Now()/time.Since 全量扫零，残余（reloginAt/gateWindow/limiter/session TTL/SyncServerTime）全部写读同基自洽或平台契约需求 |
| 新契约角度 ×2 | ✅ 窗口状态三判据单源 open 快照（windowClosedLocked 三判据共享单次 open 快照，probe 入账侧主判据与 EmptyProbeRuns 两处 10s 裕量同快照，StateForAccount 与 WindowClosed 共用同一实现，两套真相分叉根除）+ 连接活性自愈族 httpDo 判型（isConnErrRetryable dial/write 与 IsReadErr 读中断四形态正交互斥，POST 绝不误重试防双报，判型穿透 sanitizerErr.Unwrap 链不被脱敏破坏，八形态实测全 PASS） |
| 观察维持 | ⚠️ handler.go:387/:467 两处忽略返回值（未来演化需收口）、warnedNoTargets 无锁写（宿主唯一性已证）、accounts 测试夹具无 socketPreheat 双保险（仅 readyProbe，8 轮全绿实证无残余） |

### 前端（M-1 第八十三轮闭合 + OBSERVE 4 维持）
- **M-1 第八十三轮闭合**：shouldDeferSave 判据三行（targetGuard.ts:69-71）逐字符核验 + 调用级 grep -c= 恰 4（Select.tsx:509/:597/:605/:699 逐字符一致）；echoedRef 三置位（:200/:240/:297）无第四处写 true；首帧四边界（:229/:234/:247/:319）全在位；target-guard 18 断言全绿实测。
- **OBSERVE-93-01 第三十七轮**：`<button` 全仓 17 处（Admin 13/Dashboard 2/Login 2）零增零减；651/661 候选行号吻合。
- **F93-01 第五十五轮**：`git log/diff 3c3e915..HEAD -- web/` 双空实证成立（主控复现，git diff wc -l=0 三路实证 + Select.tsx/targetGuard.ts/useTickingCountdown.ts 三文件与基线逐字节相同 + 产物与 R146 同哈希）。
- **OBSERVE-116-01 第三十一轮**：注释口径统一；五路轮询契约逐键零漂移（error 与 window_closed 均降 30000）。
- **OBSERVE-115-01 弹窗族**：三处语义门（Select:1204 / Login:226 / Admin:213）与 R146 一致，autoFocus 三处（1234/267/241）齐备。
- **R125 候选复核**：Select.tsx:850 内联 cd.* 维持不实现（缓解因子无回归，perf 守卫 3 断言绿）。
- **新契约角度 ×2**：actionLoading ReadonlySet 四路消费（报名 :92 / 退选 :119 / disabled / 弹窗防误关）+ flush 出口全链（unmountedRef 三保险、指数退避、三轮收敛、pendingUnsaved 三信号）；Admin 复制降级链（clipboard→execCommand→toast 展示完整码）+ client.ts 401 三形态单次广播（HTTP 401 先行 / body 401 去重）+ 无障碍基线 25 处属性全覆盖，均零偏离。
- **OBSERVE 维持 4**：Select.tsx:204-207 注释口径残留（R124 起延续）、Admin 五 Tab 轮询带宽（观察级）、perf 守卫弱断言、Button.tsx 裸 button 候选名义。

## 验证表
| 验证项 | 结果 |
|--------|------|
| 后端 go build/vet | 通过 |
| 四包定向 race（借 mingw64 gcc） | 全绿（scheduler 15.258s / api 14.590s / zhidao 2.095s / accounts 1.383s） |
| 全仓库 CGO=1 race 13 包 | 全绿 |
| 双回归锚 | PASS |
| 前端 npm run build（tsc -b + vite） | 通过（产物与 R146 同哈希） |
| 四守卫（target/perf-countdown/admin-auth/unauthorized） | 全绿（18/3/6/5 = 32 断言） |
| XSS 面 dangerouslySetInnerHTML | 零命中 |
| 零吞错穷举（`_ =` 双形式） | 零命中（仅 2 处恒 nil 合规豁免） |
| 契约20轮次标签扫描 | 零命中（产品代码） |
| 工作区 | 干净（仅两份新 findings 未跟踪） |

## 归档
- 后端 findings：`archive/review-rounds/round147-backend-findings.md`（18317 字节 / 152 行）
- 前端 findings：`archive/review-rounds/round147-frontend-findings.md`（15904 字节）
- 收尾 commit：`docs(review): R147 双 findings + 收尾总结`（进度 148/256）