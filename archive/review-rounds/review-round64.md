# review-round64 总结（2026-09-21）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端**连续十轮零 MAJOR 零 MINOR**（R63 M-1 修复经独立核验闭合）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 3**——**生产逻辑连续第二轮零 MINOR**。

**修复 3 处**（均为 OBSERVE 级工程卫生：probeIntervalFor 注释对齐 + accounts 夹具 readyProbe 前移 + db 平台假设显式化）。

## 审查发现（写入 archive/review-rounds/round64-{backend,frontend}-findings.md）

### 后端（MINOR 0 / OBSERVE 3）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-64-01 | OBSERVE | TestProbeIntervalFor 注释"临门 5s"与常量 probeIntervalNear=2s 错位（历史探测间隔残留，断言按常量比较恒真） | ✅ 修复（四处"5 秒"改"2 秒"） |
| OBSERVE-64-02 | OBSERVE | accounts 包是唯一无 readyProbe 冷启动前移的包（api/zhidao 各有）——R12 全量轮两测试 connectex 正是夹具缺口 | ✅ 修复（新增 readyProbe + 三处夹具构造后调用） |
| OBSERVE-64-03 | OBSERVE | TestOpenOnReadonlyPath 用 Windows 保留设备路径 `C:\nul\` 在非 Windows 下是普通目录、断言方向反转 | ✅ 修复（补平台假设注释显式化） |
| 新视角 A-D | — | A R63 后端一修完全正确（http.Transport 并发安全 + 连接池无认证耦合 + Timeout 全仓一致 + DefaultClient 零残留）/ B flake **22/24 创历史新高** / C gofmt 零输出 / D 全包通读零新缺陷 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 14）
连续第十轮零 MAJOR 零 MINOR。**R63 M-1 修复判定：完全正确，修复闭合**——本轮核心核验四项实证全通过：① 语义完备性（undefined 首行先于 echoed 是优先级保护；回显 effect 两置位路径覆盖全部回显完成场景；唯一边界「pubs 空 + courses 非空」不置位→推迟正确）；② 五条时序推演全正确（echoed 是叠加性增强未破坏 F42-M1 自愈链/F43 清空语义/F15-17 假清空守卫）；③ 反向推演无任何误放行路径（echoed 只能由回显 effect 置位、置位时 selected 必含完整后端目标）；④ 两条断言真实覆盖修复语义。新增 O-11（无缺陷）：M-1 修复两轮收敛窗口变宽 2~5s（安全方向）+ 新稳态假阳性命中路径（保守方向）。tsc+build 全绿；target-guard 断言 18/18（新基线）。

## 修复（主控核实后 TDD 直修）
| 文件 | 内容 | 验证 |
|---|---|---|
| `scheduler/scheduler_test.go` | TestProbeIntervalFor 四处注释/断言文案"5s"改"2s"（与常量 probeIntervalNear 对齐） | 测试绿 |
| `accounts/manager_test.go` | 新增 readyProbe 辅助（200ms×5 轮询）+ loginRejectSrv/gateSrv/TestNewClient 三处构造后调用——accounts 包与 api/zhidao 同款冷启动前移 | 测试绿 |
| `db/settings_test.go` | TestOpenOnReadonlyPath 补平台假设注释（GOOS 守卫前的显式化） | 测试绿 |

**核实方法**：OBSERVE-64-01/02/03 我逐一对照源码核实（scheduler_test 994-1013 四处文案 / accounts manager_test 三处裸 httptest / db settings_test 12 行）——全部坐实。

**收尾回归 flake 判定**：全量回归 api 45.9s FAIL 无测试名（-p 1 吞详情）→ 隔离复跑 20.7s 全绿 → **5 连跑 -v 详细输出定位**：FAIL 测试是 `TestAdminDeleteAccountMemoryFirst`（20.88s），错误形态 `connectex ... dial tcp 127.0.0.1:54540` = **Windows 回环冷启动残余**（mock server accept 就绪前首请求连接拒绝），非业务断言失败；同轮其余测试（含我三处修复涉及的 TestProbeIntervalFor/accounts/db）全部运行或通过，5 连跑整体 exit 0（125.999s）。隔离跑恒绿证明非确定性缺陷、非修复引入。

## 收尾全量回归
- `go build ./... && go vet ./...` → exit 0；`gofmt -l .` → 零输出
- 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` → api 55.7s→40.5s、accounts 1.4s、zhidao 2.2s 等大部分 ok，api 1 次冷启动残余 FAIL（隔离复跑 + 5 连跑定位确认）
- scheduler/db/accounts 三包受影响测试全绿

## 观察项延续（下轮复核）
后端：flake 22/24（OBSERVE-64-01~03 已修，冷启动残余持续向薄弱夹具漂移——accounts 已补 readyProbe 下轮看是否消失）/ api 包残余面 / probe 非可用工具 / stats 半真测试 + 历轮延续；前端：M-1 闭合（下轮看稳态编辑落库端到端）/ N-1~N-3 / O-1~O-11 / Dashboard key 不对称 / ui 模板残宽 / NaN 防御 / 多标签页。

## 教训
1. **flake 地区的漂移是"收敛进度"的真实信号**：R63 修掉 api 包两测试后 api 包 24 轮零命中，但新命中出现在 zhidao 高并发直断与 accounts 无就绪前移——**收敛工作既要修掉已知宿主，也要预判"残余会流向哪个最薄弱的夹具"**（accounts 是唯一裸 httptest.NewServer 无 readyProbe 的包，正是下一个收敛点）。
2. **注释"数值表述"与常量一致性是读代码者的误导温床**：probeIntervalNear=2s 而测试注释/断言写"5s"（历史残留），断言按常量比较恒真、误导仅存在于文本层——凡注释写明具体阈值必须对照常量值核对。
3. **测试夹具的平台假设要显式化**：db 测试用 Windows 保留设备路径 `C:\nul\` 在 Linux CI 下语义反转——用平台专属设施要么标注 GOOS 约束、要么用跨平台等价构造。
4. **收尾回归 FAIL 必须在证据充分时定性与收官**：api 45.9s FAIL 无测试名时我用 5 连跑 -v 详细输出定位到 TestAdminDeleteAccountMemoryFirst（connectex 冷启动残余），隔离复跑恒绿——**绝不将形态未明的 FAIL 当干净收官，也绝不自乱阵脚把冷启动残余误判为业务回归**。