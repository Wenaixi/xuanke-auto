# review-round129 总结（2026-09-24）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（身份防线矩阵第四十四轮闭合 + OBSERVE-117-01 知识位第十二轮确认在位 + B110-01 审计链第十九轮零漂移 + O105-01 实测绿）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续第六十六轮零严重级）。**双端零修复需求——连续第十九轮纯观察轮**。核心产出：**身份防线矩阵第四十四轮闭合（写点换类 lastProbe/lastData/probing/syncing 时钟族）+ tick 状态机/恢复链/SubmitAll 锁外 spawn 纵深 + 前端 M-1 第六十五轮闭合 + 保存链 flush 出口 B 纵深**。

## 审查发现（写入 archive/review-rounds/round129-{backend,frontend}-findings.md）

### 后端（零缺陷轮，矩阵第四十四轮 + 知识位第十二轮）
| 项 | 核实裁决 |
|---|---|
| 身份防线矩阵第四十四轮 | ✅ 闭合：sameClientFor :204 + 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）全对位；写点换类 5 类（lastProbe/lastData+lastDataAt/probing/syncing+lastSyncTime/syncFailStreak）全持锁，唯一无锁写点 reloginResults 主循环 :675-681 天然串行无竞态；*Locked 后缀写函数族清点成立；手动五路 accountExists + maybeRelogin 双侧（:1208/:1254） |
| OBSERVE-117-01 知识位第十二轮 | ✅ 在位：:1265-1274 写回侧复核后重取当前注册表 client.Token() 落库不串旧身份 |
| B110-01 审计链第十九轮 | ✅ 零漂移：手动 6 失败位 + 成功行 + 自动族 + 零吞错穷举（单双下划线双形式）零命中 |
| O105-01 抖动基线 | ✅ 实测绿：借 /d/mingw64 gcc 五包 race 全绿（zhidao 2.624s / accounts 1.321s / scheduler 14.238s / db 2.100s / api 17.276s）+ 全量非 race 全包 ok + 回归锚测试全在位全绿 |
| 新契约角度 | ✅ tick 主循环状态机纵深 / 快照 TTL 40s 与轮询关系 / 恢复链 RestoreDone→RestoreTargets→RestoreRefused 全序 / SubmitAll 锁外 spawn 与 lastSubmit 对齐钟 / 删账号 memory-first 全序——均无新盲区 |
| 观察维持 | ⚠️ probeSem cap=4、handler 目标双写冗余、maybePrewarm 无单测（三连续轮维持） |

### 前端（M-1 第六十五轮闭合 + 零新 OBSERVE）
- **M-1 第六十五轮闭合**：shouldDeferSave 定义 targetGuard.ts:64-72 + 恰 4 消费点（Select.tsx:509/:597/:605/:699）逐字符一致；echoedRef 三置位（:200/:240/:297）+ 首帧四边界全在位；target-guard 18/18 实测全绿。
- **OBSERVE-93-01 第十九轮**：`<button` 全仓 17 处 3 文件零增零减；651/661 候选维持。
- **F93-01 第三十七轮**：`git log/diff 53a163f..HEAD -- web/` 双空实证成立。
- **OBSERVE-116-01 第十三轮**：useTickingCountdown.ts:3-8 + Dashboard 注释口径统一；五路轮询契约逐行零漂移。
- **OBSERVE-115-01 弹窗族**：三处最小语义门行号与 R128 完全一致。
- **R125 候选复核**：Select.tsx:844/:850 内联 cd.* 仍裸消费（2s 轮询吸收），维持记录不立条。
- **新契约纵深**：保存链 flush 出口 B 补发窗口（dirtyRef 两处置位/3 出口/21s 收敛）契约自洽；Dashboard 日期分组三源链 parseDateKey+localTodayMs 同基准锁本地零点；主倒计时 begin_times 兜底常态路径两路由同构。
- **观察维持**：Select.tsx:204-207 注释口径残留（R124 起延续）。

## 验证表
| 验证项 | 结果 |
|--------|------|
| 后端 go build/vet | 通过 |
| 五包 race（借 mingw64 gcc） | 全绿 |
| 前端 npm run build（tsc -b + vite） | 通过（1948 modules，682ms） |
| 四守卫（target/perf-countdown/admin-auth/unauthorized） | 全绿（18/3/6/5） |
| 契约20轮次标签扫描 | 零命中 |
| 工作区 | 干净 |

## 归档
- 后端 findings：`archive/review-rounds/round129-backend-findings.md`
- 前端 findings：`archive/review-rounds/round129-frontend-findings.md`
- 收尾 commit：`docs(review): R129 双 findings + 收尾总结`（进度 130/256）