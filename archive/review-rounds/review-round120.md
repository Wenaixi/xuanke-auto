# review-round120 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（OBSERVE-111-01/112-03 盯守第九轮 + OBSERVE-117-01 知识位第三轮确认在位）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续六十五轮零严重级）。**双端零修复需求——连续第十轮纯观察轮**。核心产出：**身份防线矩阵第三十五轮闭合（零产品改动链第十五轮延续）+ OBSERVE-117-01 知识位第三轮确认在位（支撑链持续成立）+ 登录链路与限流闸门纵深走查 + 前端 M-1 第五十六轮闭合 + OBSERVE-116-01 跟踪项第四轮维持**。

## 审查发现（写入 archive/review-rounds/round120-{backend,frontend}-findings.md）

### 后端（零缺陷轮，OBSERVE 盯守第九轮 + 知识位第三轮）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-117-01 | OBSERVE 知识位第三轮 | maybeRelogin 写回侧（scheduler.go:1263-1274）成功分支先清内存标记再取**当前注册表** client → client.Token() → UpdateIDToken 落库新 token 恒为重登后新值；支撑链持续成立（ReloginIfNeeded→Login 更新实例 token→UpdateIDToken 按主键写新值，client.go:392-394） | ✅ 在位（活化条件 = 未来改动破坏「落库取当前注册表 token」关键性质，连续三轮零漂移） |
| OBSERVE-111-01 | OBSERVE 延续 | MarkDone/RemoveDone `_ =`（handler.go:377/:457）恒 nil 非吞错（:100 的 `_ =` 仅测试内） | ⚠️ 维持（第九轮零漂移） |
| OBSERVE-112-03 | OBSERVE 延续 | 手动失效分支与自动链同文案双日志——动作维度 select/exit 可区分 | ⚠️ 维持（第九轮零漂移） |
| 身份防线矩阵延续 | — | **第三十五轮闭合**：`git log 20c5882..HEAD -- backend/` COUNT=1（57bf401 审计修复，零产品改动链第十五轮延续）；sameClientFor 7 调用点零漂移；写点全家福 12 类逐一对应防线（主控维持全量核位）；偶发写点专项四类零裸写复证；手动四路 accountExists + maybeRelogin 双侧 + 6 入口收口；B110-01 审计链第十轮零漂移 | ✅ 闭合 |
| 新契约角度 | — | 登录链路与限流闸门纵深：config 死配置清除（B40-02）、识别引擎三态回退（原生 ddddocr/Python 桥接/Vision）、doLogin 全局频率闸门族（gateWait/gateTryAcquire/GatePump）、基础设施状态码家族（500/401/403/429/CSRF-403/SPA-404）核对齐全——零漂移 | ✅ 通过 |

### 前端（M-1 第五十六轮闭合 + 零新 OBSERVE）
- **M-1 第五十六轮闭合**：shouldDeferSave 四消费点（Select.tsx:509/:597/:605/:699）三参形态逐字符一致 + echoedRef 置位三路径（:200/:240/:297）+ 首帧不置位四边界（:229/:234/:247/:319）+ target-guard 18/18 实测全绿 + tsc -b 构建通过双实证。
- **OBSERVE-93-01 第十轮家族册复核**：grep `<button` 清点 17 处（Admin 11 / Login 3 / Dashboard 3 + Toast Close）零增零减 + 651/661 引擎二选一优先修复面维持。
- **F93-01 第二十八轮**：Button.tsx:42 ring 逐字符在位 + git log/diff 1351fa4..HEAD -- web/ 双空。
- **OBSERVE-116-01 跟踪项第四轮维持不修**：每秒 setNow + 轮询双层重渲染、无卡顿证据、活化条件未触发、轮询链路契约（3s/30s/2s/10s）零漂移。
- **OBSERVE-115-01 弹窗族**：三处裸 div 弹窗最小语义门（role=dialog/aria-modal/aria-labelledby/Esc/autoFocus）逐处在位零漂移。
- **新契约角度（数据展示一致性对照）**：window_closed 三态四处同源（Dashboard:346/:751 + Admin:740 + Select:829，判据序 window_closed ≥ window_opened ≥ 待命中，与 B39-05/契约 2 对齐）+ max_count 判据簇六处同源（Select:36/:967/:977/:1006/:1009/:1103，0=名额未公布语义，`max_count>0 && selected_count>=max_count` 主判据）——零漂移，无缺陷。

## 核实记录（关键）
- **OBSERVE-117-01 知识位第三轮主控独立复核**：主控读 scheduler.go:1200-1215（决策侧锁内 `ClientFor(acct)` 存在性复核，注释明确「探测定时三处对 ErrUnauthorized 直调本入口……账号已删不发起重登、不写任何 map」）+ :1250-1276（写回侧先复核客户端仍存在 → 成功分支 delete reloginFail + 刷新 reloginAt + **重取当前注册表 client.Token() 才 UpdateIDToken 落库**，uerr 记日志不吞错）——「落库取当前注册表 token 不串旧身份」关键性质连续三轮在位。
- **身份防线矩阵第三十五轮**：主控 grep 实证 sameClientFor 恰 8 处（定义 :204 + 7 调用点 :850/:1489/:1521/:1551/:1571/:1600/:1635）；PurgeAccount（:495-519）12 类 map 键 + state.Courses 行全清；零吞错穷举扫描（`_ = AppendLog|SaveSuccess|SaveRefused|DeleteSuccess|DeleteRefusedClass|UpdateIDToken|DeleteRefused`）全仓零命中。
- **B110-01 审计链第十轮**：手动失败三分支 AppendLog 六处在位（select :352/:364/:371 + exit :433/:442/:449）+ 成功路径审计行（MarkDone :1976 / RemoveDone :2029）未回归。
- **零吞错穷举扫描**：契约 17 全仓持续成立。
- **TDD 验证链**：后端 build/vet PASS + 全量 11 包 race 回归（后台）；前端 npm run build EXIT 0 + 三守护脚本全绿（18/6/5 断言）。

## 观察项延续（下轮复核）
后端：身份防线矩阵（第三十六轮）/ OBSERVE-111-01 + 112-03 盯守（第十轮）/ **OBSERVE-117-01 知识位（第四轮）** / B110-01 审计链（第十一轮）/ B110-02 / O105-01 抖动基线 / task_log 审计保留；前端：M-1 延续管理（第五十七轮）/ OBSERVE-93-01 残余面（7 裸按钮 + Toast Close，651/661 优先修复面）/ OBSERVE-116-01 双层重渲染跟踪项（第五轮）/ OBSERVE-115-01 弹窗族 / OBSERVE-117-02 timer 类型卫生 / 其余家族册续。

## 教训
1. **35 分钟硬性时限的执行细化**：R120 后端代理在 ~35 分钟窗口尾部仍未落盘报告时，主控先 TaskStop 再 SendMessage 恢复（携带「核心必查项已实证完毕、只差新契约角度收尾 + 写报告」的压缩续接指令）——代理恢复后按指令仅做剩余走查并顺利落盘。**TaskStop + SendMessage 恢复是比「直接等待超时」或「直接杀弃重派」更优的止损路径**：不浪费已投入的实证工作，又强制收敛剩余工作量。
2. **后台代理的 output 日志是 0 行是正常态**：a803f58a 的 .output 文件在代理运行全程保持 0 字节（transcript 走 JSONL 流），用文件行数/大小判断活跃度是无效信号——正确判据是 `TaskOutput status` 与 transcript 尾部时间戳（上一工具调用的时间持续推进即活跃）。
3. **连续第十轮纯观察轮的产出仍是「新角度纵深走查」**：本轮后端选登录链路与限流闸门（config 死配置 → 识别引擎三态回退 → doLogin 频率闸门 → 基础设施状态码家族）、前端选数据展示一致性（window_closed/max_count 判据簇）——稳定期每轮换一个链路做「契约 vs 实现」全链核对，与结论「无新证据够格立条」互为因果。