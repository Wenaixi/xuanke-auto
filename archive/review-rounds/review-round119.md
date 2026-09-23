# review-round119 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（OBSERVE-111-01/112-03 盯守第八轮 + OBSERVE-117-01 知识位第二轮确认在位）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续第六十四轮零严重级）。**双端零修复需求——连续第九轮纯观察轮**。核心产出：**身份防线矩阵第三十四轮闭合（零产品改动链第十四轮延续）+ OBSERVE-117-01 知识位第二轮确认在位（结构性保证链补取证）+ 前后端契约字段级对照 + 前端 M-1 第五十五轮闭合 + OBSERVE-116-01 跟踪项第三轮维持**。

## 审查发现（写入 archive/review-rounds/round119-{backend,frontend}-findings.md）

### 后端（零缺陷轮，OBSERVE 盯守第八轮 + 知识位第二轮）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-117-01 | OBSERVE 知识位第二轮 | maybeRelogin 写回侧（:1263-1274）成功分支先清内存标记再取**当前注册表** client → client.Token() → UpdateIDToken 落库**新 token 恒为重登后新值，不串旧身份/旧 token**；补取证结构性保证链完整：ReloginIfNeeded→Login 成功在 c.mu 内更新 c.token 与 cookies（client.go:387-395）→ UpdateIDToken 按 account 主键写新值（store.go:56-59） | ✅ 在位（活化条件 = 未来改动破坏「落库取当前注册表 token」关键性质） |
| OBSERVE-111-01 | OBSERVE 延续 | MarkDone/RemoveDone `_ =`（:377/:457）恒 nil 非吞错（:100 的 `_ =` 仅测试内非产品吞错点） | ⚠️ 维持（第八轮零漂移） |
| OBSERVE-112-03 | OBSERVE 延续 | 手动失效分支与自动链同文案双日志——动作维度 select/exit 可区分 | ⚠️ 维持（第八轮零漂移） |
| 身份防线矩阵延续 | — | **第三十四轮闭合**：`git log 20c5882..HEAD -- backend/` COUNT=1（57bf401 审计修复，零产品改动链第十四轮延续）；sameClientFor 7 调用点零漂移；写点全家福 12 类逐一对应防线（主控裁定维持全量核位）；偶发写点专项（MarkTokenValid/TryAcquireSubmit/releaseFullIfFreedLocked/SubmitAll 四类零裸写复证）；手动四路 accountExists + maybeRelogin 双侧 + 探测定时三处/失效/实时复核/handler 手动共 6 入口收口；B41-01 家族 10 测试定向 race 全绿 | ✅ 闭合 |
| 新契约角度 | — | accounts 注册表生命周期综合走查（ensure/ClientFor/Remove memory-first/Relogin gateWait/LoginByPassword gateTryAcquire/Restore）+ doRequest token/cookie 双通道契约（:422 idToken + :430-435 Cookie 头统一注入 + sanitizeError + ErrUnauthorized 归口）+ 前后端契约字段级对照（/state+/electives+/logs 字段名对齐）——零漂移 | ✅ 通过 |

### 前端（M-1 第五十五轮闭合 + 零新 OBSERVE）
- **M-1 第五十五轮闭合**：四处消费点（:509/:597/:605/:699）全传 echoedRef.current 第三参逐字符一致 + echoedRef 写 3 读 6 + 置位三路径 + 首帧不置位四边界 + target-guard 18/18 实测全绿 + tsc -b 构建通过双实证。
- **OBSERVE-93-01 第九轮家族册复核**：grep `<button` 清点（Admin 15 / Login 3 / Dashboard 3 + Toast Close）+ 残余面零增零减 + 651/661 引擎二选一优先修复面。
- **F93-01 第二十七轮**：Button.tsx:42 ring 在位 + git log/diff 1351fa4..HEAD -- web/ 双空 + merge-base 确认 1351fa4 为 HEAD 祖先（12 个 commit 全 docs）。
- **OBSERVE-116-01 跟踪项第三轮维持不修**：useTickingCountdown 每秒 setNow + 轮询双层重渲染（三处注释口径完全一致）+ 无卡顿证据 + 活化条件未触发 + 轮询链路契约（3s/30s/2s/10s）原样在位。
- **OBSERVE-115-01 弹窗族**：三处裸 div 弹窗最小语义门逐处在位零漂移。
- **OBSERVE-117-02 timer 类型卫生**：`ReturnType<typeof setTimeout>` 声明构建全绿，纯类型卫生。
- **新契约角度（Dashboard 数据消费与分组链）**：begin_date 三源分组（course 自带 / pubById 映射 / 未知恒末）+ 本地零点 parse（parseDateKey "T00:00:00" 锁零点防 UTC 8h 偏移）+ 折叠种子 null/[] 语义分离 + 倒计时 begin_times 兜底双页同构——与后端知识位（publish 元数据随目标持久化）完全闭合，无缺陷。

## 核实记录（关键）
- **OBSERVE-117-01 知识位第二轮独立确认**：主控读代理取证——写回侧 :1265 重取当前注册表 client + :1266 client.Token() + :1269 UpdateIDToken，且结构性保证链（ReloginIfNeeded→Login 更新实例 token→UpdateIDToken 按主键写新值）完整——「落库取当前注册表 token 不串旧身份」关键性质持续在位。
- **前后端契约字段级对照**：handleElectives 经 ElectivesSnapshotFor（目标账号专属帧 + 过期不回退全局帧）/ handleState 三判据单源 / LoadLogs 窗口查询——前端字段（open_time/open_time_known/window_opened/window_closed/token_valid/courses/logs）与后端结构体字段名对齐。
- **零吞错穷举扫描**：`_ = AppendLog|SaveSuccess|SaveRefused|DeleteSuccess|DeleteRefusedClass|UpdateIDToken` 全仓（含测试）零命中——契约 17 持续成立。
- **TDD 验证链**：后端 build/vet PASS + 定向 race 全绿（scheduler 15.1s / zhidao 2.3s / api 18.4s 首轮即绿零残余 + 身份防线家族定向）；前端 npm run build EXIT 0 + 四守护脚本全绿（18/6/5/18 断言）。

## 观察项延续（下轮复核）
后端：身份防线矩阵（第三十五轮）/ O105-01 抖动基线 / O105-02 删除保护撞名 / B105-01 展示层 / B110-02 reloginAt 时间基 / O106-01 票据 / B109-01 死方法 / OBSERVE-111-01 + 112-03 盯守（第九轮）/ **OBSERVE-117-01 知识位（第三轮）** / task_log 审计保留 / 知识位盯守 / M87-01 窗口；前端：M-1 延续管理（第五十六轮）/ OBSERVE-93-01 残余面（7 裸按钮 + Toast Close，651/661 优先修复面）/ OBSERVE-116-01 双层重渲染跟踪项（第四轮）/ OBSERVE-115-01 弹窗族 / OBSERVE-117-02 timer 类型卫生 / OBSERVE-76-03 二勘措辞 / OBSERVE-112-02 / 88-01 / 85-02 / 84-01 / 83-01 + 77-02 + 76 族 + O-3 族全表续。

## 教训
1. **知识位盯守的结构性保证链补取证是「跨组件一致性」审查**：OBSERVE-117-01 第二轮不仅复证写回侧代码，更补取证三个组件（ReloginIfNeeded 调用侧 / Login 内部 token 更新 / UpdateIDToken 落库侧）组成的保证链——**知识位的可信度 = 全链各环节一致**，单点复证不足以证明性质在位。
2. **前后端契约字段级对照是稳定期「接口一致性」的有效角度**：/state+/electives+/logs 三个核心接口的字段名与前端消费逐一对齐——前后端各自零漂移条件下，契约对照能发现「两侧共识漂移」（一端改了字段名另一端没跟上）这类单侧审查看不见的缺陷。
3. **连续第九轮纯观察轮的产出仍是「新角度纵深走查」**：Dashboard 数据消费与分组链（begin_date 三源/本地零点/折叠种子）反映稳定期审查 = 每轮换一个模块内部链路做全量走查，从「找横向缺陷」转向「验证纵向数据链自洽」。