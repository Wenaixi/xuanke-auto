# review-round130 总结（2026-09-24）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（身份防线矩阵第四十五轮闭合 + OBSERVE-117-01 知识位第十三轮确认在位 + B110-01 审计链第二十轮零漂移 + O105-01 实测绿）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续第六十七轮零严重级）。**双端零修复需求——连续第二十轮纯观察轮**。核心产出：**身份防线矩阵第四十五轮闭合（"无锁写点天然串行"升级为宿主 goroutine 唯一性证明）+ 激活码生命周期纵深 + 前端 M-1 第六十六轮闭合 + 弹窗/保存链在飞补发总账**。

## 审查发现（写入 archive/review-rounds/round130-{backend,frontend}-findings.md）

### 后端（零缺陷轮，矩阵第四十五轮 + 知识位第十三轮）
| 项 | 核实裁决 |
|---|---|
| 身份防线矩阵第四十五轮 | ✅ 闭合：sameClientFor 定义 :199-224 + 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）零漂移；写点换类 5 类（lastSubmit/lastPrewarm/reloginFail+relogging/clockOffset+lastSyncStart/warnedNoTargets）全持锁零裸写；**"无锁写点天然串行"升级为宿主 goroutine 唯一性证明**（reloginResults 收口 :679 在 Start for+select 单线程循环 :666-684 独占通道消费）；*Locked 写函数族 + state.Courses 七写点全量复走零漂移；手动五路 accountExists + maybeRelogin 双侧（:1208/:1254） |
| OBSERVE-117-01 知识位第十三轮 | ✅ 在位：:1254 复核 → :1265-1274 重取当前注册表 client.Token() 落库不串旧身份 |
| B110-01 审计链第二十轮 | ✅ 零漂移：手动 6 失败位 + 成功行 + 自动族 + set_targets/logout/delete_account/config/登录全零吞错；穷举（单双下划线双形式）零命中 |
| O105-01 抖动基线 | ✅ 实测绿：夹具零漂移；借 mingw64 gcc 四包 race 全绿 + 全量 api race + 全量非 race 全包 |
| 新契约角度（激活码生命周期纵深） | ✅ 票据单次防重放 / 绑定账号防跨账号 / 已激活不扣次三闭环无一漏洞；风控文案映射链（isRateLimitError/isWindowClosedError/IsReadErr 三分支互斥）；删账号 memory-first + RevokeAccount 无盲区 |
| 观察维持 | ⚠️ probeSem cap=4、目标保存双写库、maybePrewarm 无单测、zhidao race 时长波动（四条连续维持） |

### 前端（M-1 第六十六轮闭合 + 零新 OBSERVE）
- **M-1 第六十六轮闭合**：shouldDeferSave 定义 targetGuard.ts:64-71 + 恰 4 消费点（Select.tsx:509/:597/:605/:699）逐字符一致；echoedRef 三置位（:200/:240/:297）+ 首帧四边界全在位；target-guard 18/18 实测全绿。
- **OBSERVE-93-01 第二十轮**：`<button` 全仓 17 处 3 文件（Admin 13/Dashboard 2/Login 2）零增零减；651/661 候选维持。
- **F93-01 第三十八轮**：`git log/diff 81f28d2..HEAD -- web/` 双空实证成立。
- **OBSERVE-116-01 第十四轮**：useTickingCountdown + Dashboard 注释口径统一；五路轮询契约零漂移。
- **OBSERVE-115-01 弹窗族**：三处最小语义门行号与 R129 完全一致。
- **R125 候选复核**：Select.tsx:850 内联 cd.* 维持成立（守卫盲区复核无漂移），不实现。
- **新契约角度（弹窗/保存链在飞补发总账）**：弹窗 Esc/在飞/autoFocus 全路径 + 保存链 `void saveNow()` 全仓 3 处 + finally 唯一自动续发出口 + dirtyRef 仅 2 置位点——均无缺口。
- **观察维持**：Select.tsx:204-207 注释口径残留（R124 起延续）。

## 验证表
| 验证项 | 结果 |
|--------|------|
| 后端 go build/vet | 通过 |
| 四包 race + 全量 api race（借 mingw64 gcc） | 全绿 |
| 前端 npm run build（tsc -b + vite） | 通过（1948 modules，391ms） |
| 四守卫（target/perf-countdown/admin-auth/unauthorized） | 全绿（18/3/6/5） |
| 契约20轮次标签扫描 | 产品零命中（测试 3 处核实为语义描述非决策前缀） |
| 工作区 | 干净 |

## 归档
- 后端 findings：`archive/review-rounds/round130-backend-findings.md`
- 前端 findings：`archive/review-rounds/round130-frontend-findings.md`
- 收尾 commit：`docs(review): R130 双 findings + 收尾总结`（进度 131/256）