# review-round61 总结（2026-09-21）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端连续第七轮零缺陷轮（MAJOR 0 / MINOR 0 / OBSERVE 12）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 3 / OBSERVE 8**——产品逻辑连续五轮零 CRITICAL、零 MAJOR。

**修复 4 处**（IsReadErr 四形态全集补齐 + REMOVED 占位对齐 + 手动路径 read 文案测试 + 注释矛盾清理）。

## 审查发现（写入 archive/review-rounds/round61-{backend,frontend}-findings.md）

### 后端（MINOR 3 / OBSERVE 8）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MINOR-61-02 | MINOR | IsReadErr 超时形态只覆盖 awaiting headers（client.go:737），漏 reading body（client.go:994，响应头已到达=更强"已处理"） | ✅ 修复（超时分支补第二文案 + 矩阵测试） |
| MINOR-61-01 | MINOR | `access_limit_cookie` 兜底 `"***REMOVED***"`（git 溯源实证为历史硬编码字面量非脱敏）与 Restore `"1"` 双占位并存 | ✅ 修复（client.go:397 + probe/main.go:25 统一 `"1"`） |
| MINOR-61-03 | MINOR | 手动报名/退选 read 文案分支零测试覆盖（R60 修复未带回归钉） | ✅ 修复（新增 TestHandleElectivesSelectReadErrMessage） |
| OBSERVE-61-01~08 | OBSERVE | flake 15/16（R16 zhidao 2 测试 connectex 冷启动残余，隔离复跑+5 连跑全绿）/ stats 注释残留（**本轮修复**）/ targets nil / 撞名文案 / probe 工具（同根修复）/ bench 默认 401 / logout 单会话 / config.Load 写盘 | ⚠️ 延续 7 项，注释残留已修 |
| 新视角 A-F | — | A R60 两修基本正确但收敛未闭合（IsReadErr 三形态→四形态）/ B 15/16 全绿 / C gofmt 零输出 / D 标准库 EOF 语义实证 FIN 分支正确不误伤 / E 10 项延续 9 延续 1 升级 / F 全包通读零新缺陷 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 12）
连续第七轮零缺陷。N-1（冲刺文案）连续五轮后维持 + 补三处点位全量枚举（Select.tsx:1123/1134 + Dashboard.tsx:648 两行落位点）；新增 O-9（Button dark variant `bg-black` 与「实心黑洞清零」护栏相邻，倾向维持并考虑 audit.mjs 豁免）；四新实证链（pick fallback 判定源 / Dashboard 分组纯 useMemo / App onUnauthorized 闭包 / 共享 react-query 缓存两路由）全部成立。六防保存链逐字符零回归 + 两个新时序推演（「全清空 PUT 在飞 + 首帧携带旧目标」双闸闭环 / 「Admin 代理态与 Dashboard 同屏互斥」）。tsc+build 全绿；TDD 断言 16+6+5 全绿。

## 修复（主控核实后 TDD 直修）
| 文件 | 内容 | 验证 |
|---|---|---|
| `zhidao/client.go` | IsReadErr 增补第四形态全集：reading body 超时文案（client.go:994）+ `io.ErrUnexpectedEOF` 短读（transfer.go 对未读满 body FIN 的包装，响应头已到达=更强"已处理"） | build+vet 绿 |
| `zhidao/isreaderr_test.go` | 形态矩阵测试增补 reading body 超时断言（红→绿 TDD 闭环） | 测试绿 |
| `zhidao/client.go` | `access_limit_cookie` 兜底 `"***REMOVED***"` → `"1"`（对齐 manager.go:313）+ 注释说明真实值由 Set-Cookie 收集 | build 绿 |
| `cmd/probe/main.go` | 同根修复：Cookie 头 `***REMOVED***` → `"1"` | build 绿 |
| `api/handler_test.go` | 新增 TestHandleElectivesSelectReadErrMessage（FLUSH+Hijack 直断触发真实短读，断言报名/退选 read 文案含"平台可能已处理"）；stats 注释对齐决策锚 1 | 测试绿 |

**TDD 实测关键发现**：写手动路径 read 文案测试时，FLUSH+Hijack 直断的**真实错误是 `unexpected EOF`（短读形态），不在 R60/R61 报告判定的集合里**——实测触发第三缺口（UnexpectedEOF）。与标准库对照（transfer.go 对未读满 body 的 FIN 包装为 ErrUnexpectedEOF）确认语义：短读 = 响应头已到达 = 平台必然已开始响应 = 比 awaiting headers 更强的"已处理"信号。IsReadErr 最终四形态全集闭环：

> RST（net.OpError read）/ FIN（io.EOF）/ 短读（io.ErrUnexpectedEOF）/ 超时（awaiting headers + reading body 双文案）

## 收尾全量回归
- `go build ./... && go vet ./...` → exit 0；`gofmt -l .` → 零输出
- 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` → **全包全绿**（10 包 0 FAIL，api 19.8s / store 33.7s / scheduler 20.8s）
- api/zhidao 相关测试全绿（含新形态矩阵 + 手动路径 read 文案测试）
- 前端 tsc + build 由前端代理验证全绿（1948 modules / 417.54 kB js / 41.07 kB css）

## 观察项延续（下轮复核）
后端：flake 15/16（OBSERVE-61-01，趋势 R57 3/11→R58 2/10→R59 2/11→R60 2/12→R61 15/16 连续 6 轮正向后单轮反弹）/ targets nil / 撞名文案 / probe 工具（cookie 已修，token 仍环境变量注入）/ bench 默认 401 / logout 单会话 / config.Load 写盘 + 历轮延续；前端：N-1~N-3 / O-1~O-9（O-9 新增）/ Dashboard key 不对称 / ui 模板残宽 / NaN 防御 / 多标签页。

## 教训
1. **同类错误形态必须穷举标准库**：R60 修超时形态只匹配 awaiting headers（client.go:737），但同一超时机制有第二处 wrap 文案（client.go:994 reading body）——`Client.Timeout` 是整请求周期计时，两个阶段各自 wrap。R59→R60→R61 连续三轮形态收敛仍未闭合，直到本轮标准库逐段实证 + TDD 实测（UnexpectedEOF 短读）才真正收口。
2. **审查脱敏产物会变成活代码**：`"***REMOVED***"` 从 R2 审查时被粘贴进 submitLogin 兜底起就是运行时字面量，此后 60 轮审查只当"脱敏回显"看待。教训：涉及 `***REMOVED***`/`xxx`/`REDACTED` 这类串的代码在通读时必须当作真实运行时值验证（git show 溯源 + 消费点 grep）。
3. **TDD 实测能发现走读推断与静态枚举都漏的形态**：审查代理穷举了标准库两处超时文案，但"写测试时真实 mock 直断连接返回 unexpected EOF"这个实测信号暴露了第三缺口。结论：形态判定函数（IsReadErr/isConnErrRetryable 这类）收敛判定 = 走读枚举标准库形态 + 写测试实测触发路径 + git 溯源，三方互补才算闭环。
4. **手动路径修复必须带回归钉**：R60 MINOR-60-03 只补分支不带测试，本轮若不做测试，IsReadErr 语义变化（本轮恰好大改）会静默漂移手动路径文案。测试夹具成本极低（FLUSH+Hijack 直断 3 行），收益是"语义变化必红"。
