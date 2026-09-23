# review-round136 总结（2026-09-24）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + LOW 0（无新增）**（身份防线矩阵**第五十一轮**闭合 + OBSERVE-117-01 知识位第十九轮确认在位 + B110-01 审计链第二十六轮零漂移 + O105-01 实测绿 + LOW-132/133 修复回首通过）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续第七十三轮零严重级）。**双端零修复需求——连续第二十六轮零 MAJOR，本轮纯观察**。核心产出：**身份防线矩阵第五十一轮闭合（7 调用点终局追写延续）+ 连接活性自愈族/登录闸门族纵深 + 前端 M-1 第七十二轮闭合 + flush 出口收敛/在飞守卫全路径纵深**。

## 审查发现（写入 archive/review-rounds/round136-{backend,frontend}-findings.md）

### 后端（零缺陷轮 + 纵深族走查）
| 项 | 核实裁决 |
|---|---|
| 身份防线矩阵第五十一轮 | ✅ 闭合：sameClientFor 定义 :204 + 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）逐一确认零漂移，每条追到写状态/落库终局（探测回写 acctData+识别槽 / 成功 done+SaveSuccess / 风控 markRateLimitedLocked / 窗口关闭+实时满员 markFullLocked / 失效 delete inflight）；写点换类 5 类全持锁；*Locked 写函数族 13 个 + 外部写函数首行取锁双向射证；唯一无锁写点宿主 goroutine 唯一性成立；手动五路 accountExists + maybeRelogin 双侧 |
| OBSERVE-117-01 知识位第十九轮 | ✅ 在位：写回侧 :1254 复核 → :1265-1273 二次 ClientFor 重取当前 token 落库，绝不串旧身份 |
| B110-01 审计链第二十六轮 | ✅ 零漂移：手动 6 失败位 + 成功审计行 + 自动链失败族全部显式 err 分支；零吞错穷举（`_ =`/`_, _ =`/直接赋值忽略）测试外零命中；token 脱敏抽查（sanitizeError:581 / maskedToken:1283）在位 |
| O105-01 抖动基线 | ✅ 实测绿：socketPreheat/readyProbe 三包夹具在位；借 mingw64 四包定向 race 全绿 + 双回归锚 + 实时复核第六分支测试（2.399s）全绿 |
| **LOW-132-01 / LOW-133-01 回首核** | ✅ 修复在位且绿：git show d75f38c 白线核对判读侧四处全对齐钟；time.Since/time.Now() 全量扫残余仅 reloginAt :1220/:1227 写读同基自洽 |
| 新契约角度 ×2 | ✅ 连接活性自愈族 httpDo 判型族（dial/write 重试与 read 四形态不重试互补，无双报风险）+ 登录闸门族 gateWait/gateTryAcquire 共享计数双侧收口（TestLoginByPasswordRejectsWhenGateBudgetExhausted race 绿） |
| 观察维持 | ⚠️ maybePrewarm 无独立单测、probeSem cap=4 常驻、Prewarm 错误静默、reloginAt 本地钟时间基（自洽写读） |

### 前端（M-1 第七十二轮闭合 + 零新 OBSERVE）
- **M-1 第七十二轮闭合**：shouldDeferSave 定义 targetGuard.ts:64-71（判据本体 :69-71 `undefined→true / echoed→false / courses非空&&hasSelected`）+ 恰 4 消费点（Select.tsx:509/:597/:605/:699）逐字符一致；echoedRef 三置位 + 首帧四边界（:229/:234/:247/:319）行号与 R135 一致；F40-M1 cleanStaleSelected 原引用返回、F43-M1 hasSelected 清空分判、纯数据判据自愈链在位；target-guard 18 断言全绿实测。
- **OBSERVE-93-01 第二十六轮**：`<button` 全仓 17 处 3 文件（Admin 13/Dashboard 2/Login 2）零增零减；651/661 候选维持。
- **F93-01 第四十四轮**：`git log/diff 665bbb6..HEAD -- web/` 双空实证成立。
- **OBSERVE-116-01 第二十轮**：注释口径统一（MemoCountdownMatrix :117 在位）；五路轮询契约逐键零漂移。
- **OBSERVE-115-01 弹窗族**：三处最小语义门行号与 R135 完全一致。
- **R125 候选复核**：Select.tsx:850 内联 cd.* 维持成立（缓解因子 2s 轮询吸收 + memo 叶子无回归），记录不实现。
- **新契约角度 ×2**：flush 出口收敛全路径（串行化单链 + 十二处守卫 return 全部不置 dirtyRef 对齐记账）+ 手动操作在飞守卫全路径（课程级 Set / 弹窗级 / 删除族 / 登录激活双守卫）均无缺口。
- **观察维持**：Select.tsx:204-207 注释口径残留（R124 起延续）。

## 验证表
| 验证项 | 结果 |
|--------|------|
| 后端 go build/vet | 通过 |
| 四包定向 race（借 mingw64 gcc） | 全绿（scheduler 15.405s / api 15.484s 最重全 ok） |
| Linux CGO=0 交叉编译 | 通过 |
| 身份防线族 + 实时复核第六分支 | 全绿（2.399s） |
| 前端 npm run build（tsc -b + vite） | 通过（1948 modules，513ms） |
| 四守卫（target/perf-countdown/admin-auth/unauthorized） | 全绿（18/3/6/5） |
| XSS 面 dangerouslySetInnerHTML | 零命中 |
| 零吞错穷举（`_ =` 双形式 + 直接赋值忽略） | 零命中（测试外） |
| 契约20轮次标签扫描 | 零命中（产品代码） |
| 工作区 | 干净（仅两份新 findings 未跟踪） |

## 归档
- 后端 findings：`archive/review-rounds/round136-backend-findings.md`（16975 字节 / 106 行）
- 前端 findings：`archive/review-rounds/round136-frontend-findings.md`（17555 字节）
- 收尾 commit：`docs(review): R136 双 findings + 收尾总结`（进度 137/256）
