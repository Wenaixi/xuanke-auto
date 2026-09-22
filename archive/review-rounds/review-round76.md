# review-round76 总结（2026-09-22）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端连续**二十三轮零 MAJOR 零 MINOR**（R76 前端 0/0/0，M-1 第十二轮闭合）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 1 / OBSERVE 3**（生产逻辑连续十四轮零 MINOR）。R75 修复经双代理逐行核证全部正确 +「管理员热改」收净目标达成。

**修复**：后端 2 处——trayPNG 托盘图标字节与注释不符（1x1 全透明→16x16 黑底白点实修）+ scheduler_test 四处行号引用改语义指位。前端本轮零修复。

## 审查发现（写入 archive/review-rounds/round76-{backend,frontend}-findings.md）

### 后端（MINOR 1 / OBSERVE 3）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MINOR-76-01 | MINOR | tray_linux.go:66-70 trayPNG 注释宣称「纯黑+中心 4x4 白块」、字节实为 1x1 全透明像素（zlib 解压 RGBA=[0,0,0,0]）——协议层合法不崩但 Linux 托盘图标不可见，与注释不符 | ✅ **实修**（升级 16x16 黑底中心 4x4 白块，PIL 生成 + 像素级验证角落黑/中心白 + Go 源码提取 86 字节逐字节复核合法，与 Windows trayIcon 同语义） |
| OBSERVE-76-01 | OBSERVE | scheduler_test.go 残留 4 处行号引用漂移（:1262「702 行」实测 :1006、:2848「1338 行」实测 :1622、:3501/:3503「1479/1490/1502 行」实测 :1477/:1508）——R74 修过同族残余漏网 | ✅ 修复（全部改语义指位，全仓该模式清零） |
| OBSERVE-76-02 | OBSERVE | CI 矩阵仍无 Linux CGO=1 构建检查（R73/R74/R75 连续三轮未落实） | ⚠️ 长期项延续 |
| OBSERVE-76-03 | OBSERVE | zhidao 五处裸 mock 宿主延续观察——靶向连跑无样本 | ⚠️ 延续 |
| R75 复核 | — | 三项修复逐项核证正确（三处「管理员热改」→「平台下发新一轮 beginTimes」与识别槽覆盖路径 ProbeForAccount:849/probe:1103 逐字一致）；全仓「管理员热改」仅剩 handler.go:168（激活码开关真实热改路径保留）+ 三处「热改」泛指（不点名管理员方向未误导） | ✅ |
| 契约抽查 | — | 窗口关闭三判据（单 open 快照+10s 裕量两侧对称）/ 删号 memory-first（含识别槽清理）/ sameClientFor 六分支 / IsReadErr 四形态 / doLogin 双门共享计数 / 零吞错落库点（`_ =` 逐个核验非落库点）/ httpDo 仅 dial-write / config 双默认值 / tick 零值守卫让位 WindowOpened / probe 空快照不删槽——10 条全成立 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 3 / 可疑 0）
连续二十三轮零 MAJOR 零 MINOR。**M-1 第十二轮闭合** + **OBSERVE-66-03 setSelected 五调用点无新增**。前端 R75 后零修改确认（git log 谱系核证），全链重走读零回潮。OBSERVE-76-01（handleBack 等待期进度反馈——R73 可疑-1/OBSERVE-75-01 延续）/ 76-02（CodesTab uses 无前端上限——OBSERVE-75-03 延续）/ 76-03（Toast 关闭按钮缺 aria-label 新增）——3 条全提示级零行为维持续；无可疑待核。六防保存链逐字符零回归 + 三组断言 + audit.mjs 全绿。

## 修复（主控核实后直修）
| commit | 内容 |
|---|---|
| `40a8bf2` | trayPNG 16x16 黑底白点实修 + scheduler_test 四处行号引用改语义指位 |

## 收尾全量回归
- `go test -race -count=1 -p 1 -timeout 900s ./...` → **全 10 包全绿 exit 0**（accounts 1.4s / api 18.5s / scheduler 14.4s / store 16.8s / zhidao 2.5s）
- `go build ./...` / `go vet` / `gofmt -l` 零输出；前端 `npm run build` + 三组断言 + audit.mjs 全绿
- exe 重建（含 R76 修复 + 最新前端产物）

## 观察项延续（下轮复核）
后端：OBSERVE-76-02（CI Linux CGO=1 检查长期项）/ OBSERVE-76-03（五处裸 mock）/ flake 全量零样本；前端：M-1 延续管理（第十三轮）/ OBSERVE-66-03 维持 / N-1~N-3 / O-1~O-12 / OBSERVE-76-01~03 续。

## 教训
1. **托盘图标必须像素级验证而非"协议合法"**：trayPNG 注释宣称 16x16 黑底白块、字节实为 1x1 全透明——PNG 协议合法（CRC/IHDR/IDAT/IEND 全对）掩盖了"图标不可见"的功能缺陷。生成图标字节后必须用图像库解码验证实际像素（尺寸 + 关键点颜色），"协议合法"≠"视觉正确"。
2. **行号引用族整风后必须复查全文件**：R74 修 scheduler_test「1575 行」后，R76 又抓四处（:1262/:2848/:3501/:3503）——同文件的同族问题修复后必须 grep 全文件所有「N 行」模式复查，只修报告点名处必然漏网。
3. **前端连续三轮零修复**：R74/R75/R76 前端全零修复，六防保存链/M-1/登录激活链全部稳定——256 轮循环的审查重点已完全转向"修复正确性复核 + 残余面监控 + 人性化细节观察"。
