# review-round72 总结（2026-09-22）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端连续**十九轮零 MAJOR 零 MINOR**（R72 前端 0 CRITICAL/0 MAJOR/0 MINOR，5 OBSERVE 全注释/维护性）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 2 / OBSERVE 5 / 可疑 1**（生产逻辑连续十轮零 MINOR）。R71 三项核心变更（注释剥离 / 系统托盘 / config 双默认值）经双代理逐行核证全部无损正确。

**修复**：前端 5 处注释残留/指位对齐 + 后端 6 项实修（含 Linux 桌面托盘不可构建的 OBSERVE 升级实修）。

## 审查发现（写入 archive/review-rounds/round72-{backend,frontend}-findings.md）

### 后端（MINOR 2 / OBSERVE 5 / 可疑 1）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MINOR-72-01 | MINOR | scheduler.go:1628 注释行号「1551 行」陈旧——当前 1551 已是 sameClientFor 指针复核，注释所指"存在性复核"已不存在 | ✅ 修复（改语义指位「入口处的存在性复核」） |
| MINOR-72-02 | MINOR | config.go:148 模板注释仍写「默认识别引擎 vision」——R71 双默认值六处一致中漏网的注释 | ✅ 修复（改 ddddocr 免密钥 vision 才需密钥） |
| OBSERVE-72-01 | OBSERVE | **tray_linux.go:90 调用未定义符号 showZenityOrPrint（全仓无定义）**——`linux && cgo` 下必然编译失败，现有 CI 矩阵（Windows CGO=1 + Linux/macOS CGO=0）恰好绕过，Linux 桌面托盘（R71 需求）不可构建 | ✅ **升级实修**（补 zenity/控制台双降级实现 + 删两行防错桩；Linux CGO=1 交叉失败归因为缺 Linux headers 环境预期） |
| OBSERVE-72-02 | OBSERVE | tray_linux_cgo0.go:6 注释乱码「liヒン」疑似日文混入 | ✅ 修复（改「GTK3 等 Linux 桌面库」） |
| OBSERVE-72-03 | OBSERVE | cmd/logintest 硬校验 SF_API_KEY——默认引擎已改 ddddocr（免密钥），ddddocr 部署下工具直接拒跑 | ✅ 修复（删过时 Fatal，跟随引擎分支） |
| OBSERVE-72-04 | OBSERVE | MINOR-71-01 测试环境塑形根因未隔离——config 测试仍裸调 Load() 读真实 data/.env | ⚠️ 延续观察（本机现值为 off/ddddocr 同向） |
| OBSERVE-72-05 | OBSERVE | 跨轮已知权衡复述（托盘已落地） | ✅ 闭合 |
| SUSPECT-72-01 | 可疑 | scheduler.go:1589 注释「inflight 已在 1333 行清掉」行号不符——实为 :1509 SelectClass 返回后统一清位 | ✅ 逻辑无缺陷 + 注释改语义指位（与 B39-03 同构：代码走读推断以全量绿为准） |
| R71 复核 | — | 注释剥离逐 commit 无损（警告类保留、B29-02 等属断言文案内嵌指位合理保留）；托盘 build tag 穷举 + trayStartOnce 通道 + trayCurrent happens-before 全成立；双默认值六处一致；readyProbe 成族 + 全量 go test/race 全绿 | ✅ |
| 契约抽查 | — | 窗口关闭三判据 / 删号 memory-first / sameClientFor 六分支 / IsReadErr 四形态 / doLogin 闸门 / 开放时间唯一事实源 / 零吞错落库点——7 条全成立 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 5 / 可疑 3）
连续十九轮零 MAJOR 零 MINOR。**M-1 第八轮闭合** + **OBSERVE-66-03 setSelected 五调用点无新增无第三来源**。R71 剥离逐行 diff 核证无损（唯一"指位改写"方向正确可追溯）。OBSERVE-72-01（Admin.tsx:207/:752 JSX 块注释 N3/N5 残留——R71 两提交均漏）✅ 修复；OBSERVE-72-03（web/scripts 三处 TDD 脚本标签残留 F42-M1/F40-M1/F41-N2）✅ 修复；OBSERVE-72-05（Select.tsx:569 注释误引已删渲染期常量 F15/F16/F17 链）✅ 修复（改「防抖/flush 消费时刻双闸」）；OBSERVE-72-06（Select.tsx:490 注释行号 618 陈旧 94 行）✅ 修复（改语义指位）；OBSERVE-72-04（TDD 脚本 2 参调用第三参 echoed 无默认值）⚠️ 续（当前语义正确）；可疑 3 条（清空守卫时序/handleBack 最长等待/aria-controls 悬空）全归因设计留白闭合。六防保存链逐字符零回归 + 三组断言全绿（target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5）。

## 修复（主控核实后直修）
| commit | 内容 |
|---|---|
| `382b7a6` | 前端收尾：Select/Admin/scripts 5 处残留标签剥离 + 注释指位对齐（契约 20） |
| `4f10764` | 后端六项：showZenityOrPrint 实现 + 乱码注释 + 两处陈旧行号 + config 模板注释 + logintest 过时硬校验 |

## 收尾全量回归
- `go test -race -count=1 -p 1 -timeout 900s ./...` → **全 10 包全绿 exit 0**（accounts 1.8s / api 21.6s / scheduler 14.8s / store 41.8s / zhidao 4.0s）
- `go build ./...` / `go vet` / `gofmt -l` 零输出；前端 `npm run build` 1.24s 全绿（三组断言脚本全绿）
- 最终 exe 重建（含 R72 修复 + 最新前端产物）

## 观察项延续（下轮复核）
后端：OBSERVE-72-04（config 测试环境塑形根因未隔离）/ flake 本轮全量全绿零样本 / OBSERVE-71-03 前身五处裸 mock（client_test.go）无 readyProbe 延续；前端：M-1 延续管理 / OBSERVE-66-03 维持 / N-1~N-3 / O-1~O-12 / OBSERVE-72-04 续。

## 教训
1. **审查代理报告的"残留清零"不可全信，主控必须全仓 grep 总扫描**：R71 清理代理报告"全部完成"后，R72 前端审查又抓到 Admin.tsx 两处 JSX 块注释 + web/scripts 三处 TDD 脚本标签（R71 只剥离 src/ 未覆盖 scripts/）——代理剥离范围声明外的文件必须主控兜底扫描（含 JSX 注释 `{/* */}` 与 scripts 目录）。
2. **未定义符号是"防错桩"掩盖的编译期炸弹**：tray_linux.go 的 `var _ = strings.Builder{}` / `var _ = fmt.Sprintf` 两行防 import 未使用的桩掩盖了 showZenityOrPrint 全仓未定义——补丁式写法把"缺函数"伪装成"缺 import"，且现有 CI 矩阵（Windows CGO=1 + Linux CGO=0）恰好绕过 `linux && cgo` 路径。多平台 build tag 文件的编译验证必须覆盖全部 tag 组合，防错桩是强烈信号要追根。
3. **陈旧行号注释随剥离漂移**：R71 注释剥离净删行后 scheduler.go 两处（:1628「1551 行」、:1589「1333 行」）与 Select.tsx:490「618 行」行号全部漂移——行号引用类注释天然易腐，统一改语义指位（"入口处的存在性复核"）比维护行号更稳。
4. **模板注释是默认值契约的第七处**：R71 双默认值实现/兜底/README/模板/测试五处核证一致后，config.go:148 模板注释（"默认 vision"）仍是漏网——改默认值必须连模板内嵌注释一起核（模板文件里的说明文字也属于契约面）。
