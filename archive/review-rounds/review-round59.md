# review-round59 总结（2026-09-21）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端零缺陷轮（MAJOR 0 / MINOR 0 / OBSERVE 10 含新增 N-3）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 2 / OBSERVE 8**——产品逻辑连续三轮零 CRITICAL、本轮零 MAJOR，R58 三处修复经 11 轮全量 + 独立程序实证全部正确。

**修复 2 处**（404 Content-Type 家族对齐 + read 错误文案区分）。

## 审查发现（写入 archive/review-rounds/round59-{backend,frontend}-findings.md）

### 后端（MINOR 2 / OBSERVE 8 + 延续 9）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MINOR-59-01 | MINOR | 404 handler 先 `WriteHeader` 再经 `writeJSON` 设 Content-Type——真实 net/http Server 上 CT 被丢弃错标 `text/plain`，httptest.ResponseRecorder（测试路径）恒绿假绿掩盖行为分叉；同族 401/403/429/500 全走 `writeJSONStatus`（先设头）正确，唯 404 漏检 | ✅ 修复（一行换 writeJSONStatus + 补真实 Server 断言） |
| MINOR-59-02 | MINOR | R57 后 read 错误（服务端已完整消费请求体但响应读取中断，平台可能已处理报名）上抛到 scheduler 非失效/风控/关闭分支落入实时复核"未现满员"分支置 failed + AppendLog"报名失败: read tcp..."误导文案——实际可能已抢到课，黄金期重复报名被拒时 failed 永久残留 | ✅ 修复（区分 read 错误文案） |
| OBSERVE-59-01~08 | OBSERVE | flake 2/11 低频残余（R9 api + R11 zhidao 隔离全绿）/ stats 测试注释残留 / targets nil 变空目标 / 撞名文案 / probe 工具脱敏 / bench 默认 401 / logout 单会话 / config.Load 写 .env | ⚠️ 延续 |
| 新视角 A-F | — | A R58 三修全正确（2s 超时不误杀/10 次不掩盖真故障/gofmt 零输出 BOM 无影响/cloneReq 逐句一致） / B 全量 11 轮 9 绿 / C gofmt 零输出 / D read 错误误导日志升级 MINOR-59-02 / E MAJOR-58-01+OBSERVE-58-01 闭合 / F 产品零缺陷 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 10）
连续第五轮零缺陷。新增 N-3（极轻）：Select.tsx:1123 渲染层英文注释残留「only affects itself」+ 两处既有格式瑕疵（缩进/缺换行），格式卫生级零运行影响。N-1（冲刺文案）连续三轮后明确裁决维持不动作（前端无法精确感知黄金期，三态文案伪精确；若整治极简方向=统一「后台目标」）。六防保存链逐字符通读零回归；tsc+build 全绿；TDD 断言 16+6+5 全绿。

## 修复（主控核实后直修）
| 文件 | 内容 | 验证 |
|---|---|---|
| `api/router.go` | 404 handler 改 `writeJSONStatus(w, http.StatusNotFound, 404, nil, "接口不存在")`——先设头后 WriteHeader，与 401/403/429/500 同族对齐（B39-02 家族） | build+vet 绿 |
| `api/handler_test.go` | TestApiUnknownPath404 补**真实 Server 端到端断言**（httptest.NewServer + http.Get 抓 HTTP 层 CT）——Recorder 假绿不再掩盖真实行为分叉 | api 包测试绿 |
| `zhidao/client.go` | 新增导出 `IsReadErr` 辅助（net.OpError Op=="read"，与 isConnErrRetryable 对称互斥） | build+vet 绿 |
| `scheduler/scheduler.go` | read 错误区分文案：「报名请求已发出但响应读取失败（平台可能已处理，请以选课大厅状态为准）」而非"报名失败"——黄金期重复报名被拒时 failed 残留的误导缓解 | scheduler 包测试绿 |

## 收尾全量回归
- `go build ./... && go vet ./...` → exit 0；`gofmt -l .` → 零输出
- 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` → **全包全绿**（10 包 0 FAIL）
- api/zhidao/scheduler 三包测试全绿
- 前端 tsc + tsc -b + npm run build 全绿（1948 modules / 417.54 kB js / 41.07 kB css）

## 观察项延续（下轮复核）
后端：flake 2/11 低频残余（OBSERVE-59-01，readyProbe 加宽已消灭"探测自身 Fatal"形态）/ stats 测试注释残留 / targets nil / 撞名文案 / probe 工具 / bench 默认 401 / logout 单会话 / config.Load 写盘 + 历轮延续（accountExists 全表 / tick 无 recover / 孤儿登录 / 双槽分叉 / reloginResults / 429 头 / 本地钟）；前端：N-1~N-3 / O-1~O-8 / Dashboard key 不对称 / ui 模板残宽 / NaN 防御 / 多标签页。

## 教训
1. **httptest.ResponseRecorder 与真实 Server 的行为分叉是假绿温床**——Recorder 允许 WriteHeader 后设 Header、真实 net/http Server 丢弃已提交响应后的 Header 设置（R59 独立程序双向实证）——凡断言响应头的测试都必须补真实 Server 端到端断言，Recorder 恒绿不证明生产行为。
2. **错误文案分类是用户体验的一部分**——read 错误（请求已发出、平台可能已处理）绝不能与"报名失败"同文案：黄金期下用户看到失败但实际抢到课、下个 tick 重复报名被拒时 failed 永久残留——"可能已处理，以大厅状态为准"既诚实又不误导。
3. **flake 收敛趋势持续向好**——R57 3/11 → R58 2/10 → R59 2/11 + 零"探测自身 Fatal"样本，readyProbe 加宽 + TestMain preheat 的组合持续压低冷启动残余；CI `||` 重跑吸收低频假红是工程常态，不必追求绝对归零。
4. **审查链连续三轮正向验证**——R57 httpDo 修复、R58 三修（readyProbe/gofmt/cloneReq 注释）被 R59 独立核证全部正确，256 轮循环的"修复经下一轮独立检验"机制正在稳定产出高质量结论。
