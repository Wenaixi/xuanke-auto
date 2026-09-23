# review-round115 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新 OBSERVE**（OBSERVE-111-01/112-03 盯守第四轮延续）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 新 OBSERVE-115-01**（连续第五十八轮零严重级）。**双端零修复需求——连续第五轮纯观察轮 + 身份防线矩阵第三十轮里程碑**。核心产出：**身份防线矩阵第三十轮里程碑闭合（零产品改动链第十轮延续）+ 矩阵方法论三十轮演进复盘（无新盲区）+ OBSERVE-76-03 二勘措辞实证确认 + OBSERVE-115-01 弹窗族观察并入 O-3 族**。

## 审查发现（写入 archive/review-rounds/round115-{backend,frontend}-findings.md）

### 后端（里程碑轮，零缺陷 + OBSERVE 2 延续）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-111-01 | OBSERVE 延续 | MarkDone/RemoveDone `_ =`（:377/:457）恒 nil 非吞错（内部落库点全日志化 :1944/:1973/:1976/:2020/:2026/:2029）；返回值丢弃仅「内存态同步失败不阻断前端响应」 | ⚠️ 维持（第四轮零漂移） |
| OBSERVE-112-03 | OBSERVE 延续 | 手动失效分支与自动链同文案双日志——动作维度 select/exit 双可辨 | ⚠️ 维持（第四轮零漂移） |
| 身份防线矩阵延续 | — | **第三十轮里程碑闭合**：`git log 20c5882..HEAD -- backend/` COUNT=1（57bf401 审计修复，零产品改动链第十轮延续）；sameClientFor 7 调用点零漂移（:204/:850/:1489/:1521/:1551/:1571/:1600/:1635）；写点全家福 12 类逐一对应防线；偶发写点专项（MarkTokenValid/TryAcquireSubmit/releaseFullIfFreedLocked/submitAll 双防线）；手动四路 accountExists + maybeRelogin 双侧；**方法论三十轮演进复盘无新盲区**（判据/写侧/读侧三角形闭合 + reloginResults 非阻塞 / syncing 复位均为注释化刻意设计） | ✅ 闭合 |
| 新契约角度 | — | 错误文案四族可辨性全量对照（自动/手动/会话/管理，动作维度 6 类可区分 select vs exit 双维可辨）+ 基础设施状态码家族（panic 500/401/403/429 双/404 兜底 writeJSONStatus 齐备） | ✅ 通过 |

### 前端（M-1 第五十一轮闭合 + 新 OBSERVE-115-01）
- **M-1 第五十一轮闭合**：四处消费点（:509/:597/:605/:699）全传 echoedRef.current 第三参逐字符一致 + shouldDeferSave 恰 4 消费点 + echoedRef 置位三路径 + 首帧不置位边界 + target-guard 18/18 实测全绿。
- **OBSERVE-93-01 第五轮家族册复核**：7 带文本裸按钮 + Toast Close 逐点位键盘链路完整缺口纯 display；651/661 优先修复面；grep `<button` 17 处零增零减。
- **OBSERVE-76-03 二勘措辞实证确认**：ToastClose 无自动注入 aria-label（Radix dist index.js:570-583）+ lucide X 默认 aria-hidden（dist :65 + buildLucideIconNode.mjs:48 实证）——「无自动注入标签、按钮无可访问名」措辞成立。
- **F93-01 第二十三轮**：Button.tsx:42 ring 在位 + git log/diff 1351fa4..HEAD -- web/ 双空（web/ 零漂移）。
- **OBSERVE-115-01（新）**：三处裸 div 弹窗（Select 退选 / Login 激活 / Admin 删除）最小语义门完整（role=dialog/aria-modal/aria-labelledby/Esc+在飞守卫/autoFocus）但均无焦点陷阱——注释声明的刻意取舍，优先修复面 = Select 退选弹窗（黄金期高频 + 聚焦面最小）；与 O-3 族同源，并入族册。
- **新契约角度（Select 选课大厅交互纵深）**：搜索/排序/筛选/徽章/进度条五处与后端 IsClassFull 同源零分叉 + btn_type 渲染契约 + actionLoading ReadonlySet 在飞互斥。
- **OBSERVE-112-02 维持**：/state /logs /electives 三查询函数式 refetchInterval 无闭包陈旧（Select 主查询 queryClient 缓存规避 TDZ）。

## 核实记录（关键）
- **OBSERVE-115-01 独立核实**：代理发现的弹窗族无焦点陷阱与 O-3 族（聚焦陷阱/滚动穿透/焦点恢复）同源——注释已声明刻意取舍（F6-02 Radix Dialog 迁移是唯一出口），量级正确立观察不修，并入族册。
- **OBSERVE-76-03 二勘实证确认**：主控 grep lucide-react dist 源码（:65 aria-hidden: "true"）与代理实证链吻合——二勘定案措辞最终成立。
- **B-110-01 审计链第五轮**：六处失败 AppendLog（:352/:364/:371 + :433/:442/:449）全在位零吞错 + 成功路径审计行未回归 + TDD 定向 race 全绿。
- **主控独立裁决（抽样化建议否决）**：代理建议「下轮起全家福 grep 降抽样」——主控**否决降档**：身份防线矩阵是三十轮核心防线资产、写点全家福核位成本可控（每轮 ~5 分钟）、三十轮同态恰证明核位纪律可靠，维持全量核位（sameClientFor 7 点全量 + 全家福全量）。
- **TDD 验证链**：后端六包定向 race 全绿（scheduler 3.6s/api 3.0s/zhidao 3.7s/accounts 2.2s/store 33.6s/db 2.1s）+ go build/vet PASS；前端 npm run build EXIT 0 + 四守护脚本全绿。

## 观察项延续（下轮复核）
后端：身份防线矩阵（第三十一轮）+ O105-01 抖动基线 / O105-02 删除保护撞名 / B105-01 展示层 / B110-02 reloginAt 时间基 / O106-01 票据 / B109-01 死方法 / OBSERVE-111-01 + 112-03 盯守（第五轮）/ 知识位盯守 / M87-01 窗口；前端：M-1 延续管理（第五十二轮）/ OBSERVE-93-01 残余面（7 带文本 + 3 refetch + Toast Close，651/661 优先修复面）/ **OBSERVE-115-01 弹窗族焦点陷阱（并入 O-3 族册）**/ OBSERVE-76-03 二勘定案措辞 / OBSERVE-112-02 / 88-01 / 85-02 / 84-01 / 83-01 + 77-02 + 76 族 + O-3 族全表续。

## 教训
1. **里程碑轮的价值 = 方法论复盘 + 主控独立裁决**：身份防线矩阵第三十轮不仅是「又一次闭合」，更做了方法论三十轮演进复盘（逐点查调用点 → 写点全家福 grep → 偶发写点专项 → 核位+实证不漂移）确认无盲区；代理建议「全家福 grep 降抽样」被主控否决——**核心防线资产（sameClientFor 7 点 + 全家福）维持全量核位，三十轮同态恰证明核位纪律可靠**，不冒降档风险省微不足道的成本。
2. **观察项并入族册的判据 = 同源 + 注释声明**：OBSERVE-115-01 弹窗族无焦点陷阱与 O-3 族同源（都是「屏蔽背景交互/焦点管理」缺口）+ 注释已声明刻意取舍（F6-02 迁移方向）——无需新增独立观察项生命周期，并入族册统一迁移即可。
3. **「错误文案四族可辨性」是审计完备性的纵深延伸**：B110-01 补手动失败审计后，四族 AppendLog 动作维度 6 类全部可区分（login/logout/set_targets/config/delete_account/select/exit）、select vs exit 双维可辨——审计链不仅「有留痕」更是「可追踪」，这是审计完备性第三维的完整形态。