# review-round131 总结（2026-09-24）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（身份防线矩阵第四十六轮闭合 + OBSERVE-117-01 知识位第十四轮确认在位 + B110-01 审计链第二十一轮零漂移 + O105-01 实测绿）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续第六十八轮零严重级）。**双端零修复需求——连续第二十一轮纯观察轮**。核心产出：**身份防线矩阵第四十六轮闭合（*Locked 写函数族 14 个全量清点 + 外部写函数首行全取锁射证）+ 多账号并发调度一致性纵深 + 前端 M-1 第六十七轮闭合 + 在飞守卫组件级/弹窗级分界**。

## 审查发现（写入 archive/review-rounds/round131-{backend,frontend}-findings.md）

### 后端（零缺陷轮，矩阵第四十六轮 + 知识位第十四轮）
| 项 | 核实裁决 |
|---|---|
| 身份防线矩阵第四十六轮 | ✅ 闭合：sameClientFor 定义 :204 + 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）逐一对齐；写点换类 5 类（done/full/inflight/rateLimited/state.Courses 六写点）全持锁 + StateForAccount 读侧只拷贝不动源；**类级结构证据升级**（*Locked 写函数族 14 个全量清点 + 无 *Locked 后缀的外部写函数首行全取锁射证）；reloginResults 仅 Start() 单 goroutine select 消费（宿主唯一性射证延续）；手动五路 accountExists + maybeRelogin 双侧（:1208/:1254） |
| OBSERVE-117-01 知识位第十四轮 | ✅ 在位：写回侧 :1254 复核 → :1265-1274 重取当前注册表 client.Token() 落库不串旧身份 |
| B110-01 审计链第二十一轮 | ✅ 零漂移：手动 6 失败位 + 成功行 + 自动族全带 if err != nil；零吞错穷举双形式零命中（唯一 `_ =` 为内存态 MarkDone/RemoveDone 恒 nil 非库写吞错） |
| O105-01 抖动基线 | ✅ 实测绿：夹具三包在位；借 mingw64 gcc 四包 race 全绿 + 全量 11 包回归全 ok |
| 新契约角度（多账号并发调度一致性纵深） | ✅ submitAll 快照→spawnChain 二次 ClientFor、probe() 单飞 + probeSem(cap4) 封顶、maybePrewarm/maybeSyncClock 独立无共享字段、登录重试收敛（识别≤3 提交≤2 网络立即返回）与 httpDo 连接活性自愈族（仅 dial/write 重试一次防双报）全数实证在位 |
| 观察维持 | ⚠️ maybePrewarm 无独立单测、probeSem cap=4 常驻（ponytail 注释已载明升级条件，N>4 自然错峰无缺陷） |

### 前端（M-1 第六十七轮闭合 + 零新 OBSERVE）
- **M-1 第六十七轮闭合**：shouldDeferSave 定义 targetGuard.ts:64-71 + 恰 4 消费点（Select.tsx:509/:597/:605/:699）逐字符一致；echoedRef 三置位（:200/:240/:297）+ 首帧四边界全在位；target-guard 18/18 实测全绿。
- **OBSERVE-93-01 第二十一轮**：`<button` 全仓 17 处 3 文件（Admin 13/Dashboard 2/Login 2）零增零减；651/661 候选维持。
- **F93-01 第三十九轮**：`git log/diff 7e5597c..HEAD -- web/` 双空实证成立。
- **OBSERVE-116-01 第十五轮**：注释口径统一；五路轮询契约键值逐位零漂移。
- **OBSERVE-115-01 弹窗族**：三处最小语义门行号与 R130 完全一致。
- **R125 候选复核**：Select.tsx:850 内联 cd.* 维持成立（2s 轮询吸收），不立条不实现。
- **新契约角度**：手动操作在飞守卫组件级/弹窗级分界 + Dashboard 折叠 extrasOpen + Admin 激活码三重复制降级——均走查无缺口。
- **观察维持**：Select.tsx:204-207 注释口径残留（R124 起延续）。

## 验证表
| 验证项 | 结果 |
|--------|------|
| 后端 go build/vet | 通过 |
| 四包 race（借 mingw64 gcc） | 全绿 |
| 前端 npm run build（tsc -b + vite） | 通过（1948 modules，826ms） |
| 四守卫（target/perf-countdown/admin-auth/unauthorized） | 全绿（18/3/6/5） |
| 契约20轮次标签扫描 | 产品零命中（测试 3 处为语义描述合规） |
| 工作区 | 干净 |

## 归档
- 后端 findings：`archive/review-rounds/round131-backend-findings.md`
- 前端 findings：`archive/review-rounds/round131-frontend-findings.md`
- 收尾 commit：`docs(review): R131 双 findings + 收尾总结`（进度 132/256）