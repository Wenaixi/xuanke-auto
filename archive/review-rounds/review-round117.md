# review-round117 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 新知识位 OBSERVE-117-01**（OBSERVE-111-01/112-03 盯守第六轮延续）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 新观察 OBSERVE-117-02（timer 类型卫生，撞号去重后编号）**（连续第六十二轮零严重级）。**双端零修复需求——连续第七轮纯观察轮**。核心产出：**身份防线矩阵第三十二轮闭合（零产品改动链第十二轮延续）+ OBSERVE-111-01/112-03 盯守第六轮 + B110-01 审计链第七轮零漂移 + 后端新知识位 OBSERVE-117-01 + tick 状态机纵深复查 + 前端 OBSERVE-116-01 跟踪项首次复核维持不修**。

## 审查发现（写入 archive/review-rounds/round117-{backend,frontend}-findings.md）

### 后端（零缺陷轮，OBSERVE 2 延续 + 新知识位）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-117-01 | OBSERVE 新（知识位） | maybeRelogin 写回侧仅「存在性复核」而非「指针身份复核」（B21-03 安全审查定版即如此）——残余面实证：删号同名重建后旧重登 goroutine 成功写回会清新身份 reloginFail/tokenValid 标记（ClientFor ok），但 **UpdateIDToken 落库时重新取当前注册表 token（:1269-1274）不串旧 token**，残余面仅新身份展示层瞬态自愈 | ⚠️ 维持（设计合理不改；防未来改动破坏「落库取当前注册表 token」关键性质） |
| OBSERVE-111-01 | OBSERVE 延续 | MarkDone/RemoveDone `_ =`（:377/:457）恒 nil 非吞错 | ⚠️ 维持（第六轮零漂移） |
| OBSERVE-112-03 | OBSERVE 延续 | 手动失效分支与自动链同文案双日志——动作维度 select/exit 可区分 | ⚠️ 维持（第六轮零漂移） |
| 身份防线矩阵延续 | — | **第三十二轮闭合**：`git log 20c5882..HEAD -- backend/` COUNT=1（57bf401 审计修复，零产品改动链第十二轮延续）；sameClientFor 7 调用点零漂移（:204/:850/:1489/:1521/:1551/:1571/:1600/:1635）；写点全家福 12 类逐一对应防线（主控裁定维持全量核位被执行）；偶发写点专项（MarkTokenValid 双锁序/TryAcquireSubmit once 幂等/releaseFullIfFreedLocked 空快照不解封/SubmitAll 对齐钟+ClientFor 过滤）；手动四路 accountExists + maybeRelogin 双侧 | ✅ 闭合 |
| tick 状态机纵深 | — | open 零值补触发符号分析（`last.Before(open.Add(-1s))` 恒 false 恒不误触发高频探测）+ 提交守卫三判据（B41-02 零值让位/未到点/WindowClosed 单源）+ EmptyProbeRuns 入账/判定双 10s 裕量对称 + syncFailStreak 先 ++ 再判 ≥3 自愈 + 探测节流三件套——零漂移无新缺陷 | ✅ 通过 |
| task_log 审计保留 | — | DeleteAccount（store.go:388-414）六表清除不触 task_log（审计线索）；LoadAllLogs 保留读取 | ✅ 通过（延续纯记录） |

### 前端（M-1 第五十三轮闭合 + OBSERVE-116-01 首次复核维持 + 新观察）
- **M-1 第五十三轮闭合**：四处消费点（:509/:597/:605/:699）全传 echoedRef.current 第三参逐字符一致 + shouldDeferSave 恰 4 消费点 + echoedRef 置位三路径 + 首帧不置位边界 + target-guard 18/18 实测全绿。
- **OBSERVE-93-01 第七轮家族册复核**：grep `<button` 全仓 18 处 + 带文本裸按钮 8 点位零增零减 + 651/661 缺 aria-pressed 优先修复面 + OBSERVE-76-03 措辞与代码事实相符（Radix Close 有隐式标签）。
- **OBSERVE-116-01 首次复核维持不修**：双层重渲染已注释为潜在优化、无卡顿证据、活化条件未触发；**轮询链路契约（Dashboard /state、/logs 3s/30s、Select /state 2s、/electives 30s/2s/10s 升降频）逐点与 B41/F52 契约一致未被动**。
- **OBSERVE-115-01 弹窗族**：三处裸 div 弹窗最小语义门逐处在位零漂移。
- **F93-01 第二十五轮**：Button.tsx:42 ring 在位 + git log/diff 1351fa4..HEAD -- web/ 双空（web/ 零漂移）。
- **OBSERVE-117-02（新，撞号去重）**：retryState.current.timer（Select.tsx:393）`ReturnType<typeof setTimeout>` 泛型声明与 NodeJS.Timeout 返回值的类型张力——当前构建通过说明类型兼容，纯类型卫生观察非缺陷。
- **新契约角度（数据展示一致性）**：window_closed 三态（Dashboard 徽章/运行指标/移动悬浮栏同源三态）+ max_count 判据簇（Select 六处同源 + 后端 IsClassFull 对齐）+ Dashboard 分组链（关闭≠元数据丢失契约）全核对一致。

## 核实记录（关键）
- **OBSERVE-117-01 知识位独立评估**：主控裁「设计合理不改」——B21-03 定版就是存在性复核（非指针身份），残余面仅新身份展示层瞬态（下轮探测/重登自愈）且 UpdateIDToken 取当前注册表 token 不串旧身份——知识位存「防未来改动破坏关键性质」活化条件，与 B104-01 知识位同族。
- **OBSERVE-116-01 首次复核**：活化条件（卡顿度量）未触发 + 轮询链路 6 处升降频逐点与 B41/F52 契约一致——维持不修（跟踪项生命周期正确运转：新立→跟踪→活化条件未触发继续跟踪）。
- **撞号去重**：前后端各自独立编号 OBSERVE-117-01（后端知识位 / 前端 timer 类型卫生），主控在归档中统一去重——后端保留 117-01、前端改记 117-02。
- **B110-01 审计链第七轮**：手动失败六处 AppendLog 全在位零吞错（32 处落库点全 if err 实证）+ 成功路径审计行未回归。
- **TDD 验证链**：后端 build/vet PASS + 定向 race 分轮（zhidao 6.2s ok / api 第 1 轮 TSAN OOM + 第 2 轮 FAIL 47s 环境压力 → 第 3 轮 37.6s ok + 第 4 轮 exit 0，重跑绿记低频残余不立条）；前端 npm run build EXIT 0 + 四守护脚本全绿（含 audit 77 断言）。

## 观察项延续（下轮复核）
后端：身份防线矩阵（第三十三轮）/ O105-01 抖动基线 / O105-02 删除保护撞名 / B105-01 展示层 / B110-02 reloginAt 时间基 / O106-01 票据 / B109-01 死方法 / OBSERVE-111-01 + 112-03 盯守（第七轮）/ **OBSERVE-117-01 知识位（防未来改动破坏「落库取当前注册表 token」）** / task_log 审计保留 / 知识位盯守 / M87-01 窗口；前端：M-1 延续管理（第五十四轮）/ OBSERVE-93-01 残余面（8 点位，651/661 优先修复面）/ OBSERVE-116-01 双层重渲染跟踪项（活化条件卡顿度量）/ OBSERVE-115-01 弹窗族 / **OBSERVE-117-02 timer 类型卫生** / OBSERVE-76-03 二勘措辞 / OBSERVE-112-02 / 88-01 / 85-02 / 84-01 / 83-01 + 77-02 + 76 族 + O-3 族全表续。

## 教训
1. **知识位与观察项的区别在「活化条件」**：OBSERVE-117-01 不是缺陷也不是纯记录——是「现状正确但有未来改动风险」的知识位（若未来有人把 UpdateIDToken 落库改成用旧 goroutine 捕获的 token 就破坏隔离性质）——知识位给活化条件（何种改动会触发），与 B104-01 同族，稳定期审查的第三类输出。
2. **跟踪项生命周期正确运转的实证**：OBSERVE-116-01 从新立（R116）→ 首次复核（R117）→「活化条件未触发维持不修」——跟踪项的价值就是「不修的决策有证据基础」（无卡顿 + 轮询契约未动），不是「不修就忘了」，每轮复核确认活化条件仍不满足。
3. **并行代理编号撞车要主控去重**：前后端代理各自独立编号易撞（本轮两条 OBSERVE-117-01）——主控归档时统一去重（后端保留、前端后缀区分），避免观察项库混乱。