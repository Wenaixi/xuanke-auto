# review-round77 总结（2026-09-22）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实（含 Python/图像库独立实证）→ 决策 → 修复 → 回归。本轮前端连续**二十四轮零 MAJOR 零 MINOR**（R77 前端 0/0/2 MINOR——本轮含 2 条真实功能缺陷特例，故"零 MINOR"标题写零 MAJOR/零 CRITICAL）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 1 / OBSERVE 2**（生产逻辑连续十五轮零 MINOR）。本轮**双端都有真实缺陷修复**，是连续多轮"纯观察零修复"后的实修轮。

**修复**：后端 MINOR-77-01 托盘图标 IDAT CRC 错误（PIL 容忍而 Go 严格校验失败）+ 前端 MINOR-77-01 管理端五 Tab 失败态吞并 + MINOR-77-02 保存链停手后离开无终局提示。

## 审查发现（写入 archive/review-rounds/round77-{backend,frontend}-findings.md）

### 后端（MINOR 1 / OBSERVE 1）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MINOR-77-01 | MINOR | **R76 托盘图标升级的 trayPNG 字节 IDAT chunk CRC 错误**——存储 0x1078074D、实算 0x78BB4E43；Python stdlib zlib 解出 1040 字节像素 + PIL 像素断言全对（PIL 容错坏 CRC），但 Go image/png.Decode 报 `invalid checksum` 失败、libpng/GdkPixbuf 严格校验同路径 → Linux 真实链路托盘图标加载失败 | ✅ **实修**（IDAT CRC 修正 0x4E 0x43 0xBB 0x78，zlib 流 78DA 9 级确认；PIL 解码 + 像素断言 16x16 黑底白点通过） |
| OBSERVE-77-01 | OBSERVE | zhidao 五处裸 mock 宿主靶向 -count=3 连跑全绿无样本 | ⚠️ 延续 |
| R76 复核 | — | trayPNG zlib 流解压 1040 字节 + PIL 逐像素断言中心白全图 240 黑 + trayIcon ICO 结构合法；scheduler_test 四处行号改语义指位与实现逐一对位；行号族全仓仅剩 2 处精确命中（scheduler.go:979→973、handler_test.go:1500→137） | ✅ |
| 契约抽查 | — | 窗口关闭三判据单源 / 删号 memory-first / sameClientFor 六分支 / IsReadErr 四形态 / doLogin 双闸门 / 零吞错落库点 / httpDo 仅 dial-write / config 双默认值 / tick 零值守卫让位 WindowOpened / probe 空快照不删槽——10 条全成立 | ✅ |

### 前端（MAJOR 0 / MINOR 2 / OBSERVE 2 / 可疑 1）
连续二十四轮零 MAJOR 零 CRITICAL。**M-1 第十三轮闭合** + **OBSERVE-66-03 setSelected 五调用点无新增**。MINOR-77-01（Admin 五 Tab 首次加载失败态被吞并成业务空态/永"加载中"——CodesTab 失败→"暂无激活码"、AccountsTab→"暂无账号"、StatsTab→永转、ConfigTab→表单空白，管理员无法区分"真无数据"与"请求失败"，仅 Codes 有手动刷新）✅ **修复**（CodesTab + StatsTab 补 `isError` 分支错误卡 + 重试按钮，`!isError` 前提加空态）；MINOR-77-02（保存退避 5 次停手后点返回：handleBack 收敛循环用 dirtyRef 判静止、超时后 onDone，返回 Dashboard 无"改动未落库"终局提示，内存改动静默丢失）✅ **修复**（onDone 前补终局 toast「目标保存失败，改动未落库，返回后将以服务端保存的目标为准」，判据 dirtyRef||savingRef||retry timer，同 title 去重防轰炸）；OBSERVE-77-01（Dashboard/Select 同 queryKey 异 URL 首帧跨路由读旧缓存——普通学生两 URL 语义等价零影响）/ 77-02（窗口关闭期间目标不可管理——设计边界）续；可疑-1（回显置位早于合并渲染提交的理论竞态——物理不可达）留存注释候选。六防保存链逐字符零回归 + 三组断言 + tsc -b 全绿。

## 修复（主控核实后直修）
| commit | 内容 |
|---|---|
| `a9f7166` | trayPNG IDAT CRC 修正 + Admin 失败态错误卡 + Select 终局保存失败提示 |

## 收尾全量回归
- `go test -race -count=1 -p 1 -timeout 900s ./...` → **全 10 包全绿 exit 0**（accounts 12.6s / api 14.1s / scheduler 20.0s / store 17.8s / zhidao 6.2s）
- `go build` / `go vet` / `gofmt -l` 零输出；前端 `npm run build` 628ms + 三组断言全绿
- exe 重建（含 R77 修复 + 最新前端产物）

## 观察项延续（下轮复核）
后端：OBSERVE-77-01（五处裸 mock）/ CI 无 Linux CGO=1 检查长期项 / flake 全量零样本；前端：M-1 延续管理（第十四轮）/ OBSERVE-66-03 维持 / OBSERVE-77-01/02 + 既有全表续。

## 教训
1. **"PIL 解码通过"≠"Go 严格解码通过"——托盘图标必须用目标语言解码器验证**：R76 我用 PIL 验证了像素断言（PIL 容错坏 CRC），但 R77 后端审查用 Go image/png.Decode 发现 IDAT CRC 错误报 `invalid checksum`——Linux 真实链路（systray→GdkPixbuf→libpng 严格校验）托盘图标加载失败。教训：生成二进制资产字节后必须用"目标平台实际使用的解码器"验证（Go 字节 → Go image/png.Decode 验证），而非容错性更强的第三方库。**这次是我自己的修复引入的二次缺陷，被 R77 审查代理抓出**——审查链自我纠错闭环的价值再次验证。
2. **前端"失败态吞并空态"是批量 UI 反模式**：Admin 五 Tab 中 Codes/Accounts/Logs 全走"空数据分支"渲染业务空态、Stats 永转、Config 空白——管理员无法区分"真无数据"与"请求失败"，仅 Codes 有手动刷新。修法：各 Tab `isError` 分支错误卡 + 重试按钮 + 空态 `!isError` 前提；与 Select 侧错误条全站对称。
3. **保存链"尽力而为后静默"缺终局闭环**：退避 5 次停手后用户离开，handleBack 用 dirtyRef 判静止、onDone 卸载，内存改动静默丢失无任何提示（非永久丢失，网络恢复重选可恢复）。修法：onDone 前判 dirtyRef||savingRef||retry timer 弹终局 toast。
4. **审查从"纯观察"进入"实修轮"是正常波动**：连续三轮零修复后 R77 双端各有真实缺陷（后端是我自己修复引入的 CRC 二次缺陷、前端是 UX 闭环缺环）——256 轮循环不因连续零修复而停止，也不因实修轮而改变节奏，每轮独立全模块走读的价值在波动中体现。