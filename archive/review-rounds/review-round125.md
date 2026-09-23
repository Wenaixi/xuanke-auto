# review-round125 总结（2026-09-24）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（身份防线矩阵第四十轮闭合 + OBSERVE-117-01 知识位第八轮确认在位 + B110-01 审计链第十五轮零漂移 + O105-01 实测绿）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续第六十二轮零严重级）。**双端零修复需求——连续第十五轮纯观察轮**。核心产出：**身份防线矩阵第四十轮闭合（零产品改动链第二十一轮延续）+ OBSERVE-117-01 第八轮 + 前端 M-1 第六十一轮闭合 + P-1 memo 叶子重渲染路径复核**。

## 审查发现（写入 archive/review-rounds/round125-{backend,frontend}-findings.md）

### 后端（零缺陷轮，矩阵第四十轮 + 知识位第八轮）
| 项 | 核实裁决 |
|---|---|
| 身份防线矩阵第四十轮 | ✅ 闭合：sameClientFor 定义 scheduler.go:204 + 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）零漂移；写点换类抽查 done/rateLimited/full/inflight/state.Courses 全持锁 + 复核后写入；手动五路 accountExists（handler.go:255/:305/:397/:497-510/:573）+ maybeRelogin 双侧（scheduler.go:1208/:1254）全数在位 |
| OBSERVE-117-01 知识位第八轮 | ✅ 在位：:1254 写回侧先 ClientFor 复核 → :1265 成功分支内重取当前注册表 client.Token() 落库不串旧身份 |
| B110-01 审计链第十五轮 | ✅ 零漂移：手动 select/exit 各三失败位 + 成功行 + 自动族齐位；零吞错穷举全仓零命中 |
| O105-01 抖动基线 | ✅ 实测绿：夹具 socketPreheat/readyProbe 在位；代理以 WinLibs 工具链跑 race 双包绿；主控本机 PATH 无 gcc 未能复现 race（工具链限制非缺陷），非 race 定向双包全绿 |
| 新契约角度 | ✅ 主控叠加 P-2 writeJSON struct 独立核验零行为变更（TDD 等价 + 前端 client.ts 逐字段 body 契约 + HTTP 状态族不变，commit 自带 revert 路径） |
| 观察维持 | ⚠️ probeSem cap=4 常驻（>50 账号部署时探测并发排队，确定性边界已知连续维持）；handler.go:571 目标保存双写库路径防御性冗余（崩溃一致性已确认成立） |

### 前端（M-1 第六十一轮闭合 + 零新 OBSERVE）
- **M-1 第六十一轮闭合**：shouldDeferSave 定义 targetGuard.ts:64-72 + 恰 4 消费点（Select.tsx:509/:597/:605/:699）三参形态逐字符一致；echoedRef 置位三路径（:200/:240/:297）+ 首帧不置位四边界全在位；target-guard 18/18 实测全绿。
- **OBSERVE-93-01 第十五轮**：`<button` 全仓 17 处 3 文件（Admin 13/Dashboard 2/Login 2）零增零减；Admin 651/661 引擎二选一仍为非 ui/Button 收敛候选。
- **F93-01 第三十三轮**：`git log/diff 60bc15f..HEAD -- web/` 双空；f0f9bfc 为本轮唯一 web 改动（perf-countdown-guard 3/3 全绿压实闭环）。
- **OBSERVE-116-01 第九轮**：useTickingCountdown.ts 注释口径已随 P-1 如实升级为「已拆并 memo」落地版本；轮询链路契约零漂移。
- **OBSERVE-115-01 弹窗族**：Select 退选 / Login 激活 / Admin 删除三处最小语义门逐处在位。
- **新契约角度（P-1 memo 叶子重渲染路径复核）**：Dashboard 叶子化正确且闭环（字符串 props + Object.is 浅比较按值等价）；**新观察 Select.tsx:850 倒计时仍内联 cd.\* 未收敛 memo 叶子**（P-1 仅覆盖 Dashboard）——被 2s 轮询边际成本吸收，性能层无正确性影响，列为候选不动。

## 验证表
| 验证项 | 结果 |
|--------|------|
| 后端 go build/vet | 通过 |
| 前端 npm run build（tsc -b + vite） | 通过（1948 modules） |
| 四守卫（target-guard/perf-countdown/admin-auth/unauthorized） | 全绿 |
| 契约20轮次标签扫描（产品+测试） | 零命中 |
| 工作区 | 干净 |

## 归档
- 后端 findings：`archive/review-rounds/round125-backend-findings.md`
- 前端 findings：`archive/review-rounds/round125-frontend-findings.md`
- 收尾 commit：`docs(review): R125 双 findings + 收尾总结`（进度 126/256）
