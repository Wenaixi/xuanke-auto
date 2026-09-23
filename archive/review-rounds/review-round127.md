# review-round127 总结（2026-09-24）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（身份防线矩阵第四十二轮闭合 + OBSERVE-117-01 知识位第十轮确认在位 + B110-01 审计链第十七轮零漂移 + O105-01 实测绿）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续第六十四轮零严重级）。**双端零修复需求——连续第十七轮纯观察轮**。核心产出：**身份防线矩阵第四十二轮闭合（类级结构证据新增）+ OBSERVE-117-01 第十轮 + 登录闸门族 gateWait/gateTryAcquire 纵深 + 前端 M-1 第六十三轮闭合 + P-1 memo 端到端复核**。

## 审查发现（写入 archive/review-rounds/round127-{backend,frontend}-findings.md）

### 后端（零缺陷轮，矩阵第四十二轮 + 知识位第十轮）
| 项 | 核实裁决 |
|---|---|
| 身份防线矩阵第四十二轮 | ✅ 闭合：sameClientFor 定义 scheduler.go:204 + 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）零漂移；写点换类 5 类（done/full/inflight/rateLimited/state.Courses）全持锁 + 复核后写入；**新增类级结构证据**：full/rateLimited 唯一写函数 markFullLocked(:1760)/markRateLimitedLocked(:1710) 全部 *Locked 后缀（写点必持锁）+ 均经 sameClientFor 复核后调用；手动五路 accountExists + maybeRelogin 双侧（:1208/:1254）；spawnChain 三条 err 归并路径同族闭合 |
| OBSERVE-117-01 知识位第十轮 | ✅ 在位：:1254 复核 → :1265-1274 重取当前注册表 client.Token() 落库不串旧身份 |
| B110-01 审计链第十七轮 | ✅ 零漂移：手动 6 失败位 + 成功行 + 自动链 6 失败族全 if err != nil；零吞错穷举零命中 |
| O105-01 抖动基线 | ✅ 实测绿：夹具在位；PATH 无 gcc、/d/mingw64/bin/gcc.exe 存在 → 借工具链 5 包 race 全绿（zhidao 3.324s / accounts 2.236s / scheduler 14.649s / db 2.375s / api 20.267s） |
| 新契约角度（登录闸门族纵深） | ✅ 自动化/手动两路共享单一 gateUsed 预算（全仓仅 2 写点）、阻塞/非阻塞语义各自正确、管理员换绑同收口、测试双断言钉死计数共享，无新缺口 |
| 观察维持 | ⚠️ probeSem cap=4 常驻、目标保存双写库防御性冗余、**maybePrewarm:293-305 无独立单测（本轮新列候选）** |

### 前端（M-1 第六十三轮闭合 + 零新 OBSERVE）
- **M-1 第六十三轮闭合**：shouldDeferSave 定义 targetGuard.ts:64-72 + 恰 4 消费点（Select.tsx:509/:597/:605/:699）逐字符一致；echoedRef 三置位（:200/:240/:297）+ 首帧四边界全在位；target-guard 18/18 实测全绿。
- **OBSERVE-93-01 第十七轮**：`<button` 全仓 17 处 3 文件（Admin 13/Dashboard 2/Login 2）零增零减；651/661 候选维持。
- **F93-01 第三十五轮**：`git log/diff b42a539..HEAD -- web/` 双空实证成立，HEAD 即 b42a539，web/ 零产品改动。
- **OBSERVE-116-01 第十一轮**：useTickingCountdown.ts:3-8 注释 P-1 落地后如实口径，Dashboard :90-93/:206-212/:394-398 同口径统一；五路轮询键值逐位零漂移。
- **OBSERVE-115-01 弹窗族**：三处 role=dialog + aria-modal + aria-labelledby + Esc 在飞守卫 + autoFocus 逐字段在位，行号与 R126 完全一致。
- **R125 候选复核**：Select.tsx:850 观察维持成立（844/850 路由顶层裸 cd.*、2s 轮询吸收、perf-countdown-guard 只扫 Dashboard 无守卫漂移）；维持记录不立条，落地需改一增二（扩展守卫第三断言 + Select 注释口径升级）。
- **新契约角度（P-1 memo 端到端复核）**：对照 f0f9bfc diff 全量核对 memo 叶子 bail out 语义、宿主仍重渲染如实披露、守卫三断言逐条匹配无正确性问题；Dashboard 分组链 begin_date 三源 + parseDateKey/localTodayMs 本地零点封堵无新发现。
- **观察维持**：Select.tsx:204-207 注释口径残留（"潜在优化" vs 顶层"已拆并 memo"，两处均如实披露各自事实，无过度承诺）。

## 验证表
| 验证项 | 结果 |
|--------|------|
| 后端 go build/vet | 通过 |
| 定向/全量 race（借 mingw64 gcc） | 5 包全绿 |
| 前端 npm run build（tsc -b + vite） | 通过（1948 modules，691ms） |
| 四守卫（target/perf-countdown/admin-auth/unauthorized） | 全绿（18/3/6/5） |
| 契约20轮次标签扫描 | 零命中 |
| 工作区 | 干净 |

## 归档
- 后端 findings：`archive/review-rounds/round127-backend-findings.md`
- 前端 findings：`archive/review-rounds/round127-frontend-findings.md`
- 收尾 commit：`docs(review): R127 双 findings + 收尾总结`（进度 128/256）
