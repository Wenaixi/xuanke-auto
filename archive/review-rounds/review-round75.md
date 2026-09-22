# review-round75 总结（2026-09-22）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端连续**二十二轮零 MAJOR 零 MINOR**（R75 前端 0/0/0，M-1 第十一轮闭合）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 1 / OBSERVE 2**（生产逻辑连续十三轮零 MINOR）。R74 修复经双代理逐行核证全部正确 + 死引用收净目标达成。

**修复**：后端 1 处收尾——scheduler.go:99/:928 + scheduler_test.go:1442 三处「管理员热改开放时间」残留注释对齐平台 beginTimes 唯一事实源（R74 收净族的漏网 sibling）。前端本轮零修复。

## 审查发现（写入 archive/review-rounds/round75-{backend,frontend}-findings.md）

### 后端（MINOR 1 / OBSERVE 2）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MINOR-75-01 | MINOR | scheduler.go:99/:928 + scheduler_test.go:1442 三处「管理员热改开放时间」残留注释——配置链路移除后无任何管理热改路径，真实触发源是平台 beginTimes 新批次覆盖识别槽 | ✅ 修复（改「平台下发新一轮 beginTimes」表述 + 全仓扫描确认该表述清零，仅剩 handler.go:168 激活码开关真实热改路径正确保留） |
| OBSERVE-75-01 | OBSERVE | CI 矩阵仍无 Linux CGO=1 构建检查（R73/R74 连续建议未落实） | ⚠️ 长期项延续 |
| OBSERVE-75-02 | OBSERVE | zhidao 五处裸 mock 宿主延续观察——靶向连跑无样本 | ⚠️ 延续 |
| R74 复核 | — | 两项修复逐项核证正确（三处注释改唯一事实源语义准确无死引用 + 测试「实时复核入口」语义指位精确对应 scheduler.go:1595）；reparse/OpenTimeParsed 全仓零残留；XUANKE_OPEN_TIME 无生产消费；trayPNG 经 Go stdlib 实测合法 | ✅ |
| 契约抽查 | — | 窗口关闭三判据（10s 裕量两侧对称）/ 删号 memory-first（含识别槽清理）/ sameClientFor 六分支（实时复核入口前置统一覆盖）/ IsReadErr 四形态 / doLogin 双门共享计数 / 零吞错落库点（抽查十处全带日志+全带 nil 守卫）/ httpDo 仅 dial-write / config 双默认值 / tick 零值守卫让位 WindowOpened / probe 空快照不删槽——10 条全成立 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 4 / 可疑 1）
连续二十二轮零 MAJOR 零 MINOR。**M-1 第十一轮闭合** + **OBSERVE-66-03 setSelected 五调用点无新增**。前端 R74 零修改确认（git log 谱系核证 web/src 自 9f5c217 后无提交），本轮全链重走读 + 与 R74 基线逐字符比对零回潮。OBSERVE-75-01（handleBack 等待期无进度反馈——R73 可疑-1 增强延续）/ 75-02（Dashboard extrasMs 重算引用）/ 75-03（Admin uses 输入缺前端 1000 上限，后端兜底完备）/ 75-04（空态与 window_closed 弱相关延续）——4 条全提示级零行为维持续；可疑-1（前端 PUT 不带发布元数据）经后端 enrichTargetPubMetaLocked 双段补全 + 启动恢复兜底交叉核证闭合。六防保存链逐字符零回归 + 三组断言全绿（18/18·6/6·5/5）+ 后端交叉核证族全成立。

## 修复（主控核实后直修）
| commit | 内容 |
|---|---|
| `6f221c3` | scheduler 三处「管理员热改」残留注释对齐平台 beginTimes 唯一事实源 |

## 收尾全量回归
- `go build ./...` / `go vet` / `gofmt -l` 零输出；scheduler 测试 13.2s 绿
- 前端 `npm run build` 全绿（三组断言脚本全绿）
- exe 重建（含 R75 修复 + 最新前端产物）

## 观察项延续（下轮复核）
后端：OBSERVE-75-01（CI Linux CGO=1 检查长期项）/ OBSERVE-75-02（五处裸 mock）/ flake 全量零样本；前端：M-1 延续管理（第十二轮）/ OBSERVE-66-03 维持 / N-1~N-3 / O-1~O-12 / OBSERVE-75-01~04 续。

## 教训
1. **同族清理必须一次扫净、逐轮追漏**：R74 收净 scheduler.go:46-48/:50/:79-80 三处「管理员配置」残留后，R75 又抓到 :99/:928 + 测试 :1442 三处「管理员热改」sibling——同一族表述的清理必须全仓 grep 同关键词族（管理员配置/管理员热改/管理员 PUT/reparse/OpenTimeParsed）一次扫净，且后续轮次要继续追漏直到零残留。
2. **"管理员热改"有唯一合法保留点**：handler.go:168「管理员热改立即生效」描述的是激活码开关的真实运行时热改路径（Runtime.Get().ActivationEnabled），与开放时间无关——扫净死引用时先判断该表述指向的机制是否真实存在，方向正确保留不误删。
3. **前端连续多轮零修复印证代码稳定**：R75 前端已连续第二轮零修复（R74/R75），审查重点从"找新缺陷"转向"修复正确性复核 + 残余面监控"——256 轮循环的价值从抓 bug 转为防回归与持续核证。
