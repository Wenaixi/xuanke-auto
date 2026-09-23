# review-round112 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + OBSERVE 3 延续 + 新 OBSERVE-112-03**；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 新 OBSERVE 2（112-01/112-02）**（连续第五十八轮零严重级）。**双端零修复需求——连续第二轮纯观察轮**。核心产出：**身份防线矩阵第二十七轮闭合 + B110-01 审计链第二轮零漂移 + OBSERVE-111-01 盯守确认 + OBSERVE-112-03 审计行为新观察 + 前端 OBSERVE-112-01 并入 F93-01 家族册**。

## 审查发现（写入 archive/review-rounds/round112-{backend,frontend}-findings.md）

### 后端（OBSERVE 3 延续 + 新 OBSERVE-112-03）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-112-01 | OBSERVE 延续 | O105-01 抖动基线 zhidao 包全量 race 首轮 1 FAIL（登录日志类 TestLoginLogsFailureSummary 风格）→ 非 race 全量/定向 count=5/二次全量全 PASS——低频残余 | ⚠️ 维持（CI `\|\|` 重跑吸收，多轮复现同根才升格） |
| OBSERVE-112-02 | OBSERVE 延续 | O111-01 MarkDone/RemoveDone `_ =`（:377/:457）恒 nil 非吞错；函数体落库点全 `if err != nil { log.Printf }`（MarkDone :1973/:1976 / RemoveDone :2020/:2026/:2029）；防回归锚点 = 未来补错误返回需同步两处调用点 + 补 StoreFailuresLogged 族手动路径 | ⚠️ 维持 |
| OBSERVE-112-03 | OBSERVE 新 | 手动失效分支落库文案「教务令牌失效，自动重登中」（isOK=false）与自动链同文案——两条日志动作维度 `select`/`exit` 不同，审计可区分，不构成重复留痕 | ⚠️ 维持（主控独立核实：动作维度区分成立） |
| 身份防线矩阵延续 | — | **第二十七轮闭合**：`git log 20c5882..HEAD -- backend/` COUNT=1（57bf401 审计修复，零产品改动链延续）；sameClientFor 7 调用点行号零漂移（:204/:850/:1489/:1521/:1551/:1571/:1600/:1635）；写点全家福 12 类逐一对应防线；偶发写点专项（MarkTokenValid/TryAcquireSubmit/releaseFullIfFreedLocked/SubmitAll）不涉身份写回缺口；手动四路 accountExists（:245/:295/:387/:563）+ maybeRelogin 双侧（:1208/:1254）全在位 | ✅ 闭合 |
| 新契约角度 | — | 手动报名/退选 handler 全链路状态码与 body 契约（缺账→缺账号→解析失败→无效课程→在飞→快手复核→ClientFor 缺→平台失败三支→成功 writeJSON 0）全路径 writeJSON + 基础设施状态码家族分离正确；db/schema/迁移规范（Open→migrateAddPublishMeta 纯增量→refuseLegacy 剔除已迁移列）；accounts 注册表生命周期全锁内读写衔接完整 | ✅ 通过 |

### 前端（OBSERVE-93-01 维持 + 新 OBSERVE-112-01/112-02）
- **M-1 第四十八轮闭合**：四处消费点（:509/:597/:605/:699）全传 echoedRef.current 第三参逐字符一致 + `shouldDeferSave(` grep 恰 4 消费点 + echoedRef 运行时读点恰 6 处 + 置位三路径 + 首帧不置位边界 + target-guard 18 断言全绿。
- **OBSERVE-93-01 残余面（第二轮家族册复核）**：7 个带文本裸按钮键盘链路完整、缺口纯 display 维度（global.css:174 outline:none）、651/661 带强 active 态优先修复面、grep `<button` 16 处净面与历轮一致零增零减、F93-01 修复面（Button.tsx:42）逐字符在位。
- **F93-01 第二十轮**：Button.tsx:42 ring 在位 + `git log 1351fa4..HEAD -- web/` 空输出 + `git diff --stat` 空——web/ 自 R108 修复后零代码漂移双重实证。
- **OBSERVE-112-01（新，并入 F93-01 家族册）**：Toast.tsx:100 `ToastPrimitive.Close` 仅 `focus:outline-none` 无 focus-visible ring 补偿（global.css:174 统一抹原生焦点）——纯 display 维度缺口，优先级低于 651/661（close 无任何视觉焦点提示），F6-02 迁移统一收敛。
- **OBSERVE-112-02（新，行为观察）**：Dashboard.tsx:140 react-query `refetchInterval: (query) => ...` 回调读取组件闭包 state 降频——/state 每 3s 刷新必触发重渲染，闭包不陈旧，与 Select 读缓存写法行为等价，纯观察不构成缺陷。
- **新契约角度（登录/激活表单全链路错误态与恢复）**：submit/activate 双幂等守卫 + 激活错误独立 state 不污染主表单 + 票据过期/已用明确引导 + 非过期失败清 pendingTicket + 取消激活全清 + 模态 Esc/autoFocus——闭环零缺陷。

## 核实记录（关键）
- **B110-01 审计链第二轮**（主控读 handler.go:340-379）：手动报名失败三分支（:352 失效 / :364 read 类 / :371 业务失败）AppendLog 全覆盖 + 零吞错 + 成功路径 :377 MarkDone 审计行未回归；退选侧代理同证（:433/:442/:449 + :457）。
- **OBSERVE-112-03 独立裁决**：手动失效分支与自动链同文案两条日志——动作维度 `select`/`exit` 区分成立，审计不混淆，维持观察不修（不是重复留痕问题）。
- **OBSERVE-112-01 独立核实**：grep Toast.tsx:100 实证 Close 仅 `focus:outline-none` 无 ring 补偿——并入 F93-01 家族册（「无 ring 残余面」+1），651/661 优先修复面在册不变。
- **TDD 验证链**：后端代理三组 race 定向全绿（身份防线家族 4.0s + db 迁移 2.1s + 脱敏族 2.2s）+ 非 race 全量 PASS；前端 npm run build exit 0 + 四守护脚本全绿。

## 观察项延续（下轮复核）
后端：身份防线矩阵（第二十八轮）/ O105-01 抖动基线 / O105-02 删除保护撞名 / B105-01 展示层 / B110-02 reloginAt 时间基 / O106-01 票据 / B109-01 死方法 / OBSERVE-111-01 MarkDone/RemoveDone 返回值 / **OBSERVE-112-03 手动失效双日志** / 知识位盯守 / M87-01 窗口；前端：M-1 延续管理（第四十九轮）/ OBSERVE-93-01 残余面（7 带文本 + 3 refetch + **Toast Close**，651/661 优先修复面）/ **OBSERVE-112-02 react-query 闭包轮询** / 88-01 / 85-02 / 84-01 / 83-01 + 77-02 + 76 族 + O-3 族全表续。

## 教训
1. **审计链闭环后的「同文案双日志」要按动作维度分判**：手动失效分支与自动链同文案「教务令牌失效，自动重登中」两条日志——不是重复留痕，动作维度 `select`/`exit` 在 /api/logs 可区分（同类动作才叫重复）。**审计完备性审查的判据 = 每条日志能否唯一追溯一个动作**，不是「文案不重复」。
2. **「无 ring 残余面」清点是持续增量的**：OBSERVE-112-01 Toast Close 是历轮 15 处裸按钮清单外的第 16 处——F93-01 家族册从「7 带文本 + 3 refetch」扩为「+Toast Close」，「残余面零增零减」的判定要覆盖**每次新出现的关键可聚焦控件**（关闭钮/切换钮等小控件最容易漏），并入家族册统一收敛。
3. **连续两轮纯观察轮的稳定形态**：R111/R112 双轮零修复 + 前端连续五十八轮零严重级——稳定期审查的产出是「确认无新证据够格立条 + 观察项增量并入册 + 矩阵核位不漂移」，不是「每轮必须修点什么」。观察项生命周期（新立→跟踪→修复/升级/转闭合）持续运转是唯一正确的稳定态管理。