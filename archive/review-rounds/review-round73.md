# review-round73 总结（2026-09-22）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端连续**二十轮零 MAJOR 零 MINOR**（R73 前端 0/0/1 MINOR，M-1 第九轮闭合）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 3 / OBSERVE 3**（生产逻辑连续十一轮零 MINOR）。R72 五项修复经双代理逐行核证全部正确无引入新问题。

**修复**：前后端 5 处收尾残留（防错桩未删 + 两处行号悬空 + 前端行号微漂 + useMemo 措辞）——均为 R72 修复的收尾小项。

## 审查发现（写入 archive/review-rounds/round73-{backend,frontend}-findings.md）

### 后端（MINOR 3 / OBSERVE 3）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MINOR-73-01 | MINOR | tray_linux.go:106-107 两行防错桩未删——R72 补 showZenityOrPrint 后 fmt 桩死代码、strings 桩是 import 遮羞布 | ✅ 修复（删两行桩 + strings import，fmt 仍有真实使用保留） |
| MINOR-73-02 | MINOR | scheduler.go:1705 注释「(1021 行)」错指 WindowClosed 守卫 return——isRateLimitedLocked 内 nowAlignedLocked 实为 :1710 写入 | ✅ 修复（改「见下方实现」语义指位） |
| MINOR-73-03 | MINOR | scheduler.go:1746 注释「(1273 行 classFullInSnapshot)」行号全错指——实为 :1453 调用/:1717 定义 | ✅ 修复（改「见 spawnChain 内 classFullInSnapshot 调用」语义指位） |
| OBSERVE-73-01 | OBSERVE | CI 矩阵仍无 Linux CGO=1 构建检查——Linux 桌面托盘零自动化编译验证 | ⚠️ 长期项（真改 tray_linux.go 时才咬人） |
| OBSERVE-73-02 | OBSERVE | zhidao 五处无 readyProbe 裸 mock 宿主延续观察——本机全量+race 无样本 | ⚠️ 延续 |
| OBSERVE-73-03 | OBSERVE | logintest 识别引擎读环境变量而非 DB settings——管理员后台热改引擎在工具中不生效（四级回退链兜底非缺陷） | ⚠️ 延续（可选修复） |
| R72 复核 | — | 五项修复逐项核证正确（showZenityOrPrint 实现/LookPath+exec+双降级、两处语义指位准确、config 模板注释一致、logintest 删硬校验后四级引擎链完整、乱码已修）；Linux 托盘 build tag go list 平台文件集实证互斥穷举 + 交叉 vet 零未定义符号；trayPNG 手写 PNG CRC/zlib 独立校验合法 | ✅ |
| 契约抽查 | — | 窗口关闭三判据 / 删号 memory-first / sameClientFor 六分支 / IsReadErr 四形态 / doLogin 闸门 / 开放时间唯一事实源 / 零吞错落库点 / httpDo dial-write 互斥 / config 双默认值七处——9 条全成立 | ✅ |

### 前端（MAJOR 0 / MINOR 1 / OBSERVE 4 / 可疑 2）
连续二十轮零 MAJOR 零 MINOR。**M-1 第九轮闭合** + **OBSERVE-66-03 setSelected 五调用点无新增**。R72 八处改动逐行核证全部语义准确无误伤（:490 行号→语义指位、:569 F15/F16/F17 链→双闸口径均正确）。SELECT-73-01（Select.tsx:161 注释「见 166 行 effect」行号漂移 60 行——R72 收尾同族遗漏）✅ 修复（改「见下方回显 effect」语义指位）；OBSERVE-73-01（Admin.tsx:56 注释谓 useMemo 实为 useState 措辞误差）✅ 修复（改 useState 字面量）；OBSERVE-73-02/03/04 核证通过延续；可疑 2 条（handleBack 最长等待 ≤63s / 徽章分母短暂空集）全归因设计留白闭合。六防保存链逐字符零回归 + 三组断言全绿（18/18·6/6·5/5）。

## 修复（主控核实后直修）
| commit | 内容 |
|---|---|
| `9f5c217` | 前后端 5 处：删 tray_linux 防错桩+strings import + scheduler 两处行号悬空改语义指位 + 前端行号微漂/useMemo 措辞 |

## 收尾全量回归
- `go test -race -count=1 -p 1 -timeout 900s ./...` → **全 10 包全绿 exit 0**（accounts 1.4s / api 16.3s / scheduler 14.2s / store 14.2s / zhidao 41.8s）
- `go build ./...` / `go vet` / `gofmt -l` 零输出；Linux CGO=0 交叉编译绿；前端 `npm run build` 452ms 全绿（三组断言脚本全绿）

## 观察项延续（下轮复核）
后端：OBSERVE-73-01（CI Linux CGO=1 检查长期项）/ OBSERVE-73-02（五处裸 mock）/ OBSERVE-73-03（logintest 引擎读取源）/ flake 全量零样本；前端：M-1 延续管理（第十轮）/ OBSERVE-66-03 维持 / N-1~N-3 / O-1~O-12 / OBSERVE-71-01/02 续。

## 教训
1. **防错桩清理要成族**：R72 补 showZenityOrPrint 后只删了问题点的两行防错桩的语义已变化——fmt 桩从"防 import 未使用"变"死代码"、strings 桩仍是遮羞布，同一文件的修复残留必须在下一轮审查中成族收净（R73 抓到的正是 R72 修复未收尾的面）。
2. **行号引用族扫描是修复后的收尾动作**：R72 修 scheduler.go 两处行号（1589/1628）后，R73 又抓两处（1705/1746）——行号引用问题必须以"全仓 grep 行号模式"成族扫净而非逐条点名修，且统一改语义指位是唯一根治（行号天然随增删行漂移）。
3. **审查链自我纠错闭环**：R72 修复的正确性由 R73 双代理独立核证（五项全对、无引入新问题），前端 SELECT-73-01 恰是"R72 修 :490 行号时遗漏的 :161 同族"——同一收尾动作的相邻遗漏由下一轮审查兜住，256 轮循环的逐轮渗透价值持续验证。
