# review-round78 总结（2026-09-22）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实（用 Go 官方 png.Decode 独立实证 + 临时程序验证）→ 决策 → 修复 → 回归。本轮后端 **CRITICAL 0 / MAJOR 1 / MINOR 0**；前端 **MAJOR 0 / MINOR 3 / OBSERVE 1**。**双端都有真实缺陷修复**——尤其回归钉测试在红色阶段当场抓出 Windows ICO 结构缺陷（biWidth 只写 1 字节 → 宽度 32 变 1），再次证明"零测试护城河的全手写二进制资产"是真实风险源。

## 审查发现（写入 archive/review-rounds/round78-{backend,frontend}-findings.md）

### 后端（MAJOR 1，前两轮进程外验证缺仓库内防线）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MAJOR-78-01 | MAJOR | **托盘资产无回归钉**——全仓无任何 `_test.go` 引用 trayPNG/trayIcon，R76/R77 两次手写字节失位（1x1 全透明 / IDAT CRC 错位）都只靠人工对照，没有机器检查能拦下"改压缩级忘同步 CRC" | ✅ **实修**（tray_asset_linux_test.go + tray_asset_windows_test.go 双回归钉，红色阶段即抓出 ICO 结构缺陷） |
| 可疑 3 项 | — | handleAdminStats log_count 用 LoadAllLogs(1000) 长度当总数（纯展示口径偏小）/ fetchLoginPage 4xx/5xx 重试语义边界 / close() 幂等 | ⚠️ 均不判级，记录供后续走读 |
| R77 修复复核 | — | trayPNG IDAT CRC 修正正确性（Go png.Decode 成功解码 16x16、16 白像素、四角黑、alpha 全 255、全 chunk CRC OK）——R76 0x9C 头时 CRC 应 0x78BB4E43（与存储错位），R77 换 0xDA 头后恰变 0x1078074D，修复正确 | ✅ |
| 契约全表 | — | 窗口关闭三判据单源 / tick 零值守卫让位 WindowOpened / 删号 memory-first / sameClientFor 六分支 / IsReadErr 四形态 / doLogin 频率闸门 / 零吞错落库点 / httpDo 仅 dial-write / config 双默认值 / probe 空快照不删槽 / flake 五处裸 mock 连续 8 轮聚焦零样本 / SPA /api 404 / go vet 干净——18 项全通过 | ✅ |

### 前端（MAJOR 0 / MINOR 3 / OBSERVE 1）
连续**二十五轮**零 MAJOR 零 CRITICAL。**M-1 第十四轮闭合** + **OBSERVE-66-03 setSelected 七调用点无新增**。
- **MINOR-78-01**（R77 只修了 CodesTab 一个 Tab 的错误卡，Accounts/Logs/Stats/Config 四个兄弟 Tab 失败态仍被吞并成"暂无账号/暂无日志/加载中永转/表单空白"）✅ **修复**（四 Tab 全补 isError 错误卡 + `refetch()` 重试按钮 + 空态 `!isError` 前提）。
- **MINOR-78-02**（终局 toast 判据 `dirtyRef||savingRef||timer` 把"守卫拦下的假清空脏块"计入 + 不区分用户有无改动：守卫命中场景误报"保存失败"、rev=0 用户全程未改动也误报）✅ **修复**（判据加 `revRef.current > 0`——无改动绝不报"改动未落库"；守卫拦下的假清空是安全拦截非保存失败，绝不误报）。
- **MINOR-78-03**（终局 toast 与失败红条同 title 去重合并的"文案突变"窗口）⚠️ 维持——去重防轰炸机制正确，"文案突变"是合并固有副作用，可接受的 UX 妥协。
- OBSERVE-78-01（63s 超时路径终局 toast 与失败红条同现——与 MINOR-78-03 同源已去重）+ 全部历轮观察延续 + 可疑-1（onDone 卸载竞态物理不可达）。

## 修复（主控核实后直修）
| commit | 内容 |
|---|---|
| `14f655e` | 托盘 PNG/ICO 双回归钉 + **Windows ICO 结构修正**（bitmapinfoheader biWidth 只写 1 字节致宽度=1→补全 4 字节 + 补漏写 biPlanes）+ Admin 四 Tab 失败态 + Select 终局 toast revRef 守卫 |

**回归钉红色阶段即抓出真实缺陷**：Windows ICO 回归钉首跑 FAIL——`biWidth` 字段只写了 `width, 0` 两字节（`width=32` 落低字节，高字节 `0` 恰把 32 写成 1），BITMAPINFOHEADER 结构被破坏；同时 biPlanes 字段从未写入（恒 0，ICO 要求 1）。真实链路（LoadImageW/DrawIconEx）可能图标加载失败或渲染错乱。修复后双层回归钉全绿。

## 收尾全量回归
- `go test -race -count=1 -p 1 -timeout 900s ./...` → 全 10 包全绿（待跑）
- `go build` / `go vet` 零输出；gofmt 格式化后零残留
- 前端 `npm run build` → 499ms 绿（tsc -b + vite）+ 三组断言全绿（target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5）
- 托盘资产回归钉独立验证：`go test -run 'TestTrayPNGAsset|TestTrayIconAsset'` 全绿 + 临时程序用 Go 官方 png.Decode 确认 16x16/alpha 全 65535/像素语义正确

## 观察项延续（下轮复核）
后端：可疑 3 项（stats log_count 口径 / 4xx/5xx 重试语义 / close 幂等）+ OBSERVE-77-01 五处裸 mock + CI Linux CGO=1 长期项；前端：M-1 延续管理（第十五轮）/ OBSERVE-66-03 维持 / OBSERVE-78-01 + 全表续。

## 教训
1. **回归钉的价值在"红色阶段即抓出缺陷"**：本轮 Windows ICO 回归钉首跑 FAIL——biWidth 手写两字节 (32, 0) 使宽度=1、biPlanes 从未写入恒 0。这是 R76 就存在的老缺陷，几十轮只读审查都因"只读代码推断+平台无关 CL"而漏过，唯独机器断言在写测试那一刻就当场显形。**全手写二进制资产必须配目标语言解码器/结构解析的回归测试**，只靠人工对照永远有盲区。
2. **"测试与实现同源编写"的双向价值**：回归钉测试在写出来那一刻就证明了自身必要性（抓到 ICO 缺陷），同时 tracer 用独立临时程序（非测试代码）交叉验证了 PNG 语义——同一资产值两次独立工具链验证，与 R77 的 PIL vs Go 解码器教训一脉相承。
3. **前端失败态"只修一个 Tab"是半截工程**：R77 修 CodesTab 时其余四个兄弟 Tab 同缺陷族被忽略，R78 前端审查抓出不对称。缺陷族修复必须"成族清点"（grep 兄弟节点），与后端身份防线"全分支清点"同理。