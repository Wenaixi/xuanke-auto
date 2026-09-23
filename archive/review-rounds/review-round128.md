# review-round128 总结（2026-09-24）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（身份防线矩阵第四十三轮闭合 + OBSERVE-117-01 知识位第十一轮确认在位 + B110-01 审计链第十八轮零漂移 + O105-01 实测绿）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续第六十五轮零严重级）。**双端零修复需求——连续第十八轮纯观察轮**。核心产出：**身份防线矩阵第四十三轮闭合（*Locked 写函数族全量清点）+ OBSERVE-117-01 第十一轮 + 三族契约钉死（状态码/登录闸门/数据库迁移）+ 前端 M-1 第六十四轮闭合 + react-query 升降频双向可逆新角度**。

## 审查发现（写入 archive/review-rounds/round128-{backend,frontend}-findings.md）

### 后端（零缺陷轮，矩阵第四十三轮 + 知识位第十一轮）
| 项 | 核实裁决 |
|---|---|
| 身份防线矩阵第四十三轮 | ✅ 闭合：sameClientFor 定义 :204（clientIdentity :215-224）+ 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）对位；写点换类 5 类（acctData/openTimeDetected/refused/reloginAt/tokenValid）全持锁 + 复核后写入；**类级结构证据延续**：*Locked 后缀写函数族全量清点 + state.Courses 全量写点（PurgeAccount 滤行/rebuildCourses 重建/MarkDone 追加/RemoveDone 就地改/setStateLocked 闸门/releaseFullIfFreedLocked 解封）零裸写；识别槽删除点仅 PurgeAccount（关闭≠时间消失契约）；手动五路 accountExists + maybeRelogin 双侧（:1208/:1254） |
| OBSERVE-117-01 知识位第十一轮 | ✅ 在位：:1244-1284 写回侧先 ClientFor 复核 → :1265-1274 重取当前注册表 client.Token() 落库不串旧身份 |
| B110-01 审计链第十八轮 | ✅ 零漂移：手动六失败位（:362/:374/:381/:443/:452/:459）+ 成功行 + 自动族 + set_targets/logout/delete_account 全 if err != nil；零吞错穷举（含双下划线双形式）全仓零命中 |
| O105-01 抖动基线 | ✅ 实测绿：夹具零漂移；借 /d/mingw64 gcc 五包 race 全绿（zhidao 2.661s / accounts 1.532s / scheduler 14.112s / db 2.224s / api 16.836s）；全量非 race 全包 ok |
| 新契约角度（三族契约钉死） | ✅ 登录闸门族（gateWait/gateTryAcquire/GatePump 三路共享单一预算 + LoginByPassword 空壳失败清理语义）；状态码家族（writeJSONStatus 11 调用点覆盖鉴权 401/管理 403/CSRF-403/限流 429/panic+落库 500/未知 404，与前端只读 body code 契约零冲突）；数据库迁移规范（migrateAddPublishMeta 在 refuseLegacy 之前 + 缺列清单已剔除 + TDD 守护）；快照 TTL 40s/轮询关系 + maybePrewarm 行为极简进维持观察 |
| 观察维持 | ⚠️ probeSem cap=4 常驻、目标保存双写库防御性冗余、maybePrewarm:293-305 无独立单测（R127 候选延续） |

### 前端（M-1 第六十四轮闭合 + 零新 OBSERVE）
- **M-1 第六十四轮闭合**：shouldDeferSave 定义 targetGuard.ts:64-72 + 恰 4 消费点（Select.tsx:509/:597/:605/:699）逐字符一致；echoedRef 三置位（:200/:240/:297）+ 首帧四边界全在位；target-guard 18/18 实测全绿。
- **OBSERVE-93-01 第十八轮**：`<button` 全仓 17 处 3 文件（Admin 13/Dashboard 2/Login 2）零增零减，行号逐一比对；651/661 候选维持。
- **F93-01 第三十六轮**：`git log/diff 08b43da..HEAD -- web/` 双空实证成立。
- **OBSERVE-116-01 第十二轮**：倒计时注释如实口径 + 五路轮询键值逐位零漂移（Select 2s/10s/30s 升降频 + Dashboard 3s/30s/30s 恒）。
- **OBSERVE-115-01 弹窗族**：三处最小语义门行号与 R127 完全一致。
- **R125 候选复核**：Select.tsx:850 内联 cd.* 观察维持成立（2s 轮询吸收、无正确性影响），维持候选不立条。
- **新契约角度**：react-query 升降频双向可逆性走查（三处函数式 interval 判定源均取自身最新状态，无卡档路径）+ Admin 五 Tab 失败态错误卡（互不牵连、各自自愈、configTab 锁定保存按钮防覆盖）均无缺口。
- **观察维持**：Select.tsx:204-207 注释口径残留（R124 起延续）。

## 验证表
| 验证项 | 结果 |
|--------|------|
| 后端 go build/vet | 通过 |
| 五包 race（借 mingw64 gcc） | 全绿 |
| 前端 npm run build（tsc -b + vite） | 通过（1948 modules，908ms） |
| 四守卫（target/perf-countdown/admin-auth/unauthorized） | 全绿（18/3/6/5） |
| 契约20轮次标签扫描 | 零命中 |
| 工作区 | 干净 |

## 归档
- 后端 findings：`archive/review-rounds/round128-backend-findings.md`
- 前端 findings：`archive/review-rounds/round128-frontend-findings.md`
- 收尾 commit：`docs(review): R128 双 findings + 收尾总结`（进度 129/256）