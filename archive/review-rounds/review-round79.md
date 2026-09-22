# review-round79 总结（2026-09-22）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实（复刻字节独立实证 LoadImageW 语义）→ 决策 → 修复 → 回归。本轮后端 **CRITICAL 0 / MAJOR 1 / MINOR 0**；前端 **MAJOR 0 / MINOR 3 / OBSERVE 1**。**双端都有真实缺陷修复**——尤其后端 MAJOR-79-01 揭示"回归钉假绿"的深层教训：R78 的托盘回归钉只校验 ICONDIR 头与 DIB 头、不解析 ICONDIRENTRY 的 entry 字段，恰好盲区掩盖了自 R71 就存在的 ICO 字段双重错位。

## 审查发现（写入 archive/review-rounds/round79-{backend,frontend}-findings.md）

### 后端（MAJOR 1 / OBSERVE 2，生产逻辑契约全表核证通过）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MAJOR-79-01 | MAJOR | **Windows 托盘图标必然加载失败**——trayIcon() 的 ICONDIRENTRY 把 `dwBytesInRes`（应 4264）填成 22、`dwImageOffset`（应 22）填成 4286，两个字段值双重写错；R79 审查用独立程序调用 `user32.LoadImageW` 实测当前字节 FAILED、修正后 OK（GetIconInfo 确认 32x32 bpp=32 黑底白心） | ✅ **实修**（binary.LittleEndian.PutUint32 重写两字段 + 回归钉补 [14:18]/[18:22] 断言，先写反测试红→修正后绿） |
| O79-01 | OBSERVE | 回归钉不解析 entry 字段（M79-01 的测试盲区） | ✅ 随修复闭合 |
| O79-02 | OBSERVE | 前端工作树未提交（R78 事后改动） | ✅ 已由 R79 前端 commit 覆盖 |
| 复核 20 项 | — | 托盘 Linux/Windows 双回归钉同口径、DIB 头逐位正确、build tag 四文件隔离、windowClosedLocked 三判据单源、spawnChain 6 处 sameClientFor、maybeRelogin 入口复核、登录闸门 gateWait/gateTryAcquire 收口、httpDo 仅 dial/write（bytes.Reader 重放实测完整）、零吞错、删号 memory-first、管理员双条件、ElectivesSnapshotFor 回退链——全通过 | ✅ |

### 前端（MAJOR 0 / MINOR 3 / OBSERVE 1）
连续**二十六轮**零 MAJOR 零 CRITICAL。**M-1 第十五轮闭合** + **setSelected 七调用点无新增**。
- **MINOR-79-01**（R78 StatsTab 失败态 error-first 次序与同族三 Tab data-first 不一致——5s 轮询瞬时失败把最近一次有效数据整卡替换成错误卡）✅ **修复**（改 `s ? rows : isError ? errorCard : 加载中`，错误卡仅在后端从未成功时渲染）。
- **MINOR-79-02**（R78 终局 toast 修复只闭合 rev=0 残留脏，rev>0 + 守卫拦下假清空仍误归因"目标保存失败"）✅ **修复**（新增 `guardBlockedRef` 分流十处守卫置脏，`dirtyRef` 仅承载真实保存失败，终局判据抽 `pendingUnsaved()` + handleBack 入口清标记）。
- **MINOR-79-03**（契约 20 轮次标签残留 1 处：target-guard-check.ts:68 "R63"）✅ **修复**（剥离 R63 前缀，全仓复扫零残留）。
- **OBSERVE-79-01**（LogsTab 缺 isLoading 短路闪"暂无日志"）✅ **修复**（补 isLoading 分支）。
- 全部延续观察 + 可疑-1（卸载竞态）物理不可达维持。

## 修复（主控核实后直修）
| commit | 内容 |
|---|---|
| `f1d98de` | 前端修复：Admin StatsTab data-first 次序 + 终局 toast 守卫脏拆分 guardBlockedRef + LogsTab isLoading 短路 |
| `d14303a` | 契约 20 剥离 target-guard-check.ts 注释 "R63 " 轮次标签 |
| `51dc151` | 后端修复：Windows ICO ICONDIRENTRY 字段错位（dwBytesInRes/dwImageOffset 双重写反）+ 回归钉补 entry 断言 |

**TDD 过程实证**：初版回归钉断言写反（[14:18] 判 offset）→ 红（got 4264）；修正为 bytesInRes/offset 各自正确语义后绿——红色阶段即拦下"字段值错位"这一类散失。

## 收尾全量回归
- `go test -race -count=1 -p 1 -timeout 900s ./...` → 全 10 包全绿（后台跑）
- `go build` / `go vet` / `gofmt -l` 零输出
- 前端 `npm run build` 631ms 绿 + 三组断言全绿（target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5）
- 托盘双回归钉独立验证：TestTrayPNGAsset（png.Decode 严格校验 CRC）+ TestTrayIconAsset（含 entry 字段断言）全绿 + 独立程序复刻字节确认字段值

## 观察项延续（下轮复核）
后端：O79-01/02 随本轮闭合；契约 20 全仓零残留；flake 基线与历轮一致。前端：M-1 延续管理（第十六轮）/ OBSERVE-66-03 维持 / 可疑-1 物理不可达维持。

## 教训
1. **"修复不完全 + 测试盲区"双遗留是审查链最高价值捕获**：R78 修对了 DIB 头（biWidth/planes）但 entry 字段延续 R71 的双重错位，且回归钉恰好不覆盖 entry——测试全绿但功能报废。机器断言要按"格式规范逐字段全解析"而非"抽查几个显式字段"，否则测试本身成为假绿温床。
2. **格式字段语义要用目标 API 实测锚定**：R79 审查用 LoadImageW（真实消费方）实测字节、并用系统 OneDrive.ico 逐字节解析佐证字段语义——比人手对照规范文档更可靠。二进制资产的验证三板斧：目标解码器（png.Decode/LoadImageW）+ 规范字段全解析 + 系统基准文件对照。
3. **TOCTOU 风格"字段值互置"要防**：dwBytesInRes/dwImageOffset 都写"目录后/数据区"但语义恰好相反，最易手滑互置；回归钉必须按语义各断言一个具体值（4264/22）而非只查非零。
4. **前端缺陷族"只修一个 Tab/一半路径"成规律性复发**：R77 只修 CodesTab → R78 补四 Tab → R79 又发现 StatsTab 次序回归——每轮修复都要问"同族兄弟节点是否对称"，与后端身份防线"全分支清点"同理。