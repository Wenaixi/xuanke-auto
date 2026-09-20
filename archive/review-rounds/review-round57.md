# review-round57 总结（2026-09-21）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端零缺陷轮（MAJOR 0 / MINOR 0 / OBSERVE 10 含 1 新增 N-1）；后端 **CRITICAL 1（测试基建） / MAJOR 1 / MINOR 1 / OBSERVE 7**——最重一轮：R56 的 cloneReq 修复被证为死代码，且正确方向是 read 错误不重试。

**修复 3 处**（httpDo read 错误不重试 + TestMain 去 Discard + gofmt 规范化），全部实证后落地。

## 审查发现（写入 archive/review-rounds/round57-{backend,frontend}-findings.md）

### 后端（CRITICAL 1 / MAJOR 1 / MINOR 1 / OBSERVE 7 + 延续 9）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| CRITICAL-57-01 | CRITICAL | **R56 cloneReq "GetBody 重生成"修复是死代码**——标准库 `http.NewRequest(bytes.NewReader)` 已自动设 GetBody（request.go:932-945 核证），R56 分支 `req.GetBody == nil` 对真实 doRequest 恒不命中；独立程序实证 read 类错误重试仍空 body + ContentLength 恒 13（畸形 POST 发送端拒绝）；**若放宽用 GetBody 重放则 SelectClass/ExitClass 双报**（第二实证：服务端 2 次收到报名请求 200 OK） | ✅ 修复（read 错误不重试） |
| MAJOR-57-01 | MAJOR | 全量 `-race -p 1` 11 轮 3 FAIL（api×3，4 个不同测试样本 TestSetTargetsBounds/TestLoginUnactivatedNeedsCode/TestAdminStatsOpenTimeFromRecognized/TestElectiveSelectRejectsWindowClosed），隔离复跑全绿——Windows 回环冷启动残余，R56"14%"未稳定 | ⚠️ 延续观察（CI `||` 吸收） |
| MINOR-57-01 | MINOR | zhidao TestMain `log.SetOutput(io.Discard)` 与 `TestLoginLogs*` 日志捕获缓冲切换竞态（偶发 FAIL）+ gofmt 违规（import 多 tab/无尾换行） | ✅ 修复（删 Discard + gofmt） |
| OBSERVE-57-01~07 | OBSERVE | cloneReq 误导注释 / Discard 吞排查线索 / readyProbe 无超时 / open_time_set 测试注释残留 / httpDo 注释对 read 不成立 / TestSetTargetsBounds 类型断言 panic 形态 / config.Load ensureEnvFile 写盘 | ⚠️ 1 闭合（57-01 随 CRITICAL）+ 6 延续 |
| 新视角 A-F | — | A TestMain preheat 正确生效、cloneReq 不正确、open_time_set 注释已对齐 / B 全量 11 轮 3 FAIL / C TestSetTargetsBounds 冷启动非断言脆弱 / D httpDo 重试真实路径实证 / E 观察项 8 延续 2 闭合 1 升级 / F 产品零缺陷 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 10）
连续三轮零修复依旧零缺陷。新增 N-1：Select.tsx:1126「开始冲刺」按钮文案与后端冲刺实际逻辑（仅开窗后 10 秒黄金期 250ms）描述性出入——纯文案零行为影响。六防保存链逐字符通读零回归；tsc+build 全绿；TDD 断言 16+6+5 全绿。

## 修复（主控核实后直修）
| 文件 | 内容 | 验证 |
|---|---|---|
| `zhidao/client.go` | httpDo 只对 **dial/write 错误**重试（请求未到达，重发安全），**read 错误不上抛重试**（服务端已消费 body、可能已处理，重发双报）——`isConnErr` 改 `isConnErrRetryable` 只含 dial/write；cloneReq 简化回 `req.Clone` + 注释说明标准库已设 GetBody 的透明语义 | 全量 race 全包绿 |
| `zhidao/client_test.go` | TestMain 删 `log.SetOutput(io.Discard)`（日志测试完整性优先）+ gofmt 规范化 | 日志测试连跑 5 次绿 |
| gofmt | zhidao 两文件 `gofmt -w` 规范化 | gofmt -l 零输出 |

## 收尾全量回归
- `go build ./... && go vet ./...` → exit 0
- 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` → **全包全绿**（10 包 0 FAIL）
- zhidao 日志测试 `-race -count=5` 连跑全绿
- 前端 tsc + tsc -b + npm run build 全绿（1948 modules / 417.54 kB js / 41.07 kB css）

## 观察项延续（下轮复核）
后端：api 冷启动残余（57-01）/ readyProbe 无超时 / open_time_set 测试注释残留 / TestSetTargetsBounds panic 形态 / config ensureEnvFile + 历轮延续（accountExists 全表 / tick 无 recover / 孤儿登录 / 双槽分叉 / reloginResults / 429 头 / 本地钟）；前端：N-1~N-2 / O-1~O-8 / Dashboard key 不对称 / ui 模板残宽 / NaN 防御 / 多标签页。

## 教训
1. **"修复生效性"必须在真实路径验证**——R56 的 cloneReq GetBody 修复在独立程序（fake RoundTripper）实证"生效"，但真实 `http.NewRequest(bytes.NewReader)` 已自动设 GetBody、修复分支恒不命中——**用真实调用链复现是审查修复的底线**，假想路径的实证可能完全误导。
2. **重试契约必须按错误类型分类**——dial/write（请求未到达）可重发；read（服务端已消费 body、可能已处理）不可重发 POST——"连接层错误重试不双报"的注释只对 dial/write 成立，read 错误重发=双报。R52-M4 注释长期未区分，本轮实证彻底修正。
3. **测试夹具的全局副作用必须最小化**——TestMain 里 `log.SetOutput(io.Discard)` 是包级全局副作用，与依赖日志捕获的测试产生竞态；夹具静默应只针对"已知噪音"路径，绝不以全局 Discard 换便利。
4. **审查链自我纠错的价值**——R56 我主动补的 cloneReq 修复被 R57 审查代理证伪（死代码 + 若生效双报），这恰是 256 轮循环的意义：每轮新视角让前一轮的"自以为修复"接受独立检验。
