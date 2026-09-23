# review-round126 总结（2026-09-24）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（身份防线矩阵第四十一轮闭合 + OBSERVE-117-01 知识位第九轮确认在位 + B110-01 审计链第十六轮零漂移 + O105-01 实测绿）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续第六十三轮零严重级）。**双端零修复需求——连续第十六轮纯观察轮**。核心产出：**身份防线矩阵第四十一轮闭合（零产品改动链第二十二轮延续）+ OBSERVE-117-01 第九轮 + 会话吊销 RevokeAccount 全链路新角度 + 前端 M-1 第六十二轮闭合 + 保存链五道防线四个出口新角度**。

## 审查发现（写入 archive/review-rounds/round126-{backend,frontend}-findings.md）

### 后端（零缺陷轮，矩阵第四十一轮 + 知识位第九轮）
| 项 | 核实裁决 |
|---|---|
| 身份防线矩阵第四十一轮 | ✅ 闭合：sameClientFor 定义 scheduler.go:204-210 + 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）零漂移（与 R125 逐行一致）；写点换类 5 类（acctDataAt/openTimeDetected/refused/reloginAt/tokenValid）全持锁 + 复核后写入，reloginAt 全仓 grep 仅 2 写入点（:1231/:1261）无隐藏写点；偶发写点四类零裸写；手动五路 accountExists（handler.go:255/:305/:397/:497-510/:573）+ maybeRelogin 双侧（:1208/:1254）全数在位 |
| OBSERVE-117-01 知识位第九轮 | ✅ 在位：写回侧 :1254 ClientFor 复核 → :1265 重取当前注册表 client.Token() 落库不串旧身份 |
| B110-01 审计链第十六轮 | ✅ 零漂移：手动 6 失败位 + 成功行 + 自动族 + set_targets/logout/delete_account 全零吞错；穷举 grep 零命中 |
| O105-01 抖动基线 | ✅ 实测绿：本机默认 PATH 无 gcc，探测到 /d/mingw64/bin/gcc.exe 借工具链跑定向 race 双包绿（zhidao 1.759s / accounts 1.755s），环境差异已覆盖 |
| 新契约角度 | ✅ 会话吊销 RevokeAccount 全链路 memory-first 四步收敛（Accounts.Remove → PurgeAccount → DeleteAccount → RevokeAccount）无漏洞；ProbeNow/时钟同步写全校共享帧无账号身份绑定语义，无需身份复核，设计自洽 |
| 观察维持 | ⚠️ probeSem cap=4 常驻、handler.go:571 目标保存双写库路径（均非本轮引入、确定性边界已知） |

### 前端（M-1 第六十二轮闭合 + 零新 OBSERVE）
- **M-1 第六十二轮闭合**：shouldDeferSave 定义 targetGuard.ts:64-72 + 恰 4 消费点（Select.tsx:509/:597/:605/:699）逐字符吻合；echoedRef 三置位（:200/:240/:297）+ 首帧不置位四边界全在位；target-guard 18/18 实测全绿。
- **OBSERVE-93-01 第十六轮**：`<button` 全仓 17 处 3 文件（Admin 13/Dashboard 2/Login 2）零增零减，行号与 R125 逐行一致；651/661 候选维持。
- **F93-01 第三十四轮**：`git log/diff 51ce8b0..HEAD -- web/` 双空实证成立，工作树全净。
- **OBSERVE-116-01 第十轮**：useTickingCountdown.ts 注释为 P-1 落地后如实口径；五路轮询键值逐位零漂移。
- **OBSERVE-115-01 弹窗族**：三处 role=dialog + aria-modal + aria-labelledby + Esc 在飞守卫 + autoFocus 逐处在位。
- **新候选复核**：Select.tsx:850 倒计时内联 cd.* 观察维持成立（5 处消费，不在 perf-countdown-guard 覆盖内），2s 轮询吸收边际成本、性能层无正确性影响，记录不实现不立条。
- **新契约角度（保存链五道防线四个出口）**：:509 回显未完成 / :515 发布缺席假清空 / :526 stale 残留 / :553 联查空集 / :558 id 漂移——五个出口（置脏跳过/补发标记/saveNow/返回收敛）全路径核对无缺口；react-query 轮询升/降频双向可逆无高频粘滞。

## 验证表
| 验证项 | 结果 |
|--------|------|
| 后端 go build/vet | 通过 |
| 定向 race（借 /d/mingw64 工具链） | 双包全绿 |
| 前端 npm run build（tsc -b + vite） | 通过（1948 modules，654ms） |
| 四守卫（target/perf-countdown/admin-auth/unauthorized） | 全绿 |
| 契约20轮次标签扫描（产品+测试） | 零命中 |
| 工作区 | 干净 |

## 归档
- 后端 findings：`archive/review-rounds/round126-backend-findings.md`
- 前端 findings：`archive/review-rounds/round126-frontend-findings.md`
- 收尾 commit：`docs(review): R126 双 findings + 收尾总结`（进度 127/256）
