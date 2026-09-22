# review-round74 总结（2026-09-22）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端连续**二十一轮零 MAJOR 零 MINOR**（R74 前端 0/0/0，M-1 第十轮闭合）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 2 / OBSERVE 2**（生产逻辑连续十二轮零 MINOR）。R73 四项修复经双代理逐行核证全部正确无引入新问题。

**修复**：后端 2 处收尾残留——scheduler.go 三处"管理员配置/reparse/OpenTimeParsed"残留注释对齐唯一事实源契约 + scheduler_test.go 测试注释行号漂移改语义指位。前端本轮零修复（4 条 OBSERVE 全一致性观察）。

## 审查发现（写入 archive/review-rounds/round74-{backend,frontend}-findings.md）

### 后端（MINOR 2 / OBSERVE 2）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MINOR-74-01 | MINOR | scheduler_test.go:3420/3421 测试注释「(1575 行…)」行号漂移——修复当时行号，现 1575 已注释行、sameClientFor 复核实为 :1596 | ✅ 修复（改语义指位「实时复核入口」） |
| MINOR-74-02 | MINOR | scheduler.go:46-48/:50/:79-80 三处"管理员配置/reparse/OpenTimeParsed"残留注释——配置链路整体移除后已无实体，误导维护者以为仍有配置层注入路径 | ✅ 修复（对齐唯一事实源表述，删死引用，grep 确认生产代码 reparse/OpenTimeParsed 清零） |
| OBSERVE-74-01 | OBSERVE | CI 矩阵仍无 Linux CGO=1 构建检查——Linux 桌面托盘零自动化编译防线（R73 建议未落实） | ⚠️ 长期项延续 |
| OBSERVE-74-02 | OBSERVE | zhidao 五处裸 mock 宿主延续观察——连跑 3 轮无样本（0.35-0.55s），TestLoginNetworkError 已补 readyProbe 后无边角裸宿主 | ⚠️ 延续 |
| R73 复核 | — | 四项修复逐项核证正确（tray_linux 编译干净 fmt/os/exec 真实使用 / :1705「见下方实现」语义指向 isRateLimitedLocked now 参数与 markRateLimitedLocked nowAlignedLocked 落点完整 / :1746「见 spawnChain 内 classFullInSnapshot 调用」精确指向 1453 / 行号族仅剩 980「973 行」精确命中） | ✅ |
| 契约抽查 | — | 窗口关闭三判据 / 删号 memory-first / sameClientFor 六分支（六测试 -race 绿）/ IsReadErr 四形态（超时双文案与 Go 1.26 stdlib client.go:737/994 逐字一致）/ doLogin 双门共享计数 / 开放时间唯一事实源 / 零吞错落库点（16 处 store 访问全带 nil 守卫）/ httpDo 仅 dial-write / config 双默认值七处——9 条全成立 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 4 / 可疑 1）
连续二十一轮零 MAJOR 零 MINOR。**M-1 第十轮闭合** + **OBSERVE-66-03 setSelected 五调用点无新增**。R73 两项修复逐行核证全部语义准确无误伤（:161 行号→语义指位、:56 useMemo→useState 字面量）。OBSERVE-74-01（SELECTED 徽章 open_time_known 双源展示一致性）/ 74-02（Progress value/max=0 渲染边界）/ 74-03（主倒计时与折叠行差 1 tick 注释承诺）/ 74-04（Admin apiKey 保存后清空时机）——4 条全为行为自洽的一致性观察零行为缺陷，**维持续**；可疑-1（徽章分母短暂空集）闭合。六防保存链逐字符零回归 + 三组断言全绿（18/18·6/6·5/5）+ 撞名学生管理态与后端 handler.go 签发双条件交叉核证无旁路。

## 修复（主控核实后直修）
| commit | 内容 |
|---|---|
| `bb619a1` | scheduler.go 三处管理员配置残留注释对齐唯一事实源 + scheduler_test.go 测试注释行号改语义指位 |

## 收尾全量回归
- `go test -race -count=1 -p 1 -timeout 900s ./...` → **全 10 包全绿 exit 0**（accounts 1.3s / api 14.7s / scheduler 20.1s / store 13.0s / zhidao 2.5s）
- `go build ./...` / `go vet` / `gofmt -l` 零输出；前端 `npm run build` 全绿（三组断言脚本全绿）

## 观察项延续（下轮复核）
后端：OBSERVE-74-01（CI Linux CGO=1 检查长期项）/ OBSERVE-74-02（五处裸 mock）/ flake 全量零样本；前端：M-1 延续管理（第十一轮）/ OBSERVE-66-03 维持 / N-1~N-3 / O-1~O-12 / OBSERVE-71-01/02/70-03 续。

## 教训
1. **配置链路移除后注释死引用必须成族扫净**：开放时间配置链路（管理员 PUT open_time / runtime.reparse / OpenTimeParsed）整体移除后，scheduler.go 三处注释仍描述已不存在的机制——grep 死引用（reparse/OpenTimeParsed）是发现残留的可靠手段，注释与"唯一事实源"契约必须持续对齐。
2. **测试注释的历史行号引用同样漂移**：scheduler_test.go「(1575 行…)」是修复当时行号的历史叙述，代码演进后 1575 已是他处——测试注释的行号引用与生产注释同族，改语义指位（"实时复核入口"）同样适用。
3. **审查链连续正向验证**：R74 双代理独立核证 R73 修复全对、无引入新问题，前端 R74 修复项已收敛为零（连续两轮零修复）——代码稳定进入纯观察阶段，256 轮循环的审查重点从"抓新缺陷"转向"修复正确性复核 + 残余面监控"。
