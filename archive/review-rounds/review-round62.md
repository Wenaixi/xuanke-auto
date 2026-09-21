# review-round62 总结（2026-09-21）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端连续第八轮零缺陷轮（MAJOR 0 / MINOR 0 / OBSERVE 13）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 2 / OBSERVE 7**——产品逻辑连续六轮零 CRITICAL、零 MAJOR。

**修复 2 处**（IsReadErr 短读注释对齐真实错误链路 + fetchLoginPage 4xx/5xx 重试语义契约对齐）。

## 审查发现（写入 archive/review-rounds/round62-{backend,frontend}-findings.md）

### 后端（MINOR 2 / OBSERVE 7）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MINOR-62-01 | MINOR | IsReadErr 的 ErrUnexpectedEOF 分支注释与真实覆盖形态错位——标准库逐段实证真实命中此分支的是正文读取阶段纯 EOF 短读（body.readLocked → bodyEOFSignal 透传 → Do 包 url.Error），与 RST（响应头阶段 OpError）互斥；功能正确但注释误导 | ✅ 修复（注释对齐准确链路 + 补 urlError 包装的 ErrUnexpectedEOF 穿透断言） |
| MINOR-62-02 | MINOR | fetchLoginPage 对 403/429 响应重试一次但注释归入"连接活性自愈"家族，与 httpDo 严格连接层边界双标准 | ✅ 修复（注释对齐"瞬时抖动重试独立于连接自愈、不承诺对限流生效"） |
| OBSERVE-62-01~07 | OBSERVE | flake 17/21（RUN4 captcha 超时 / RUN2 TestReloginIfNeeded / RUN5 TestElectivesSnapshot / RUN7 TestHealth readyProbe 全 connectex 冷启动残余，隔离复跑+5 连跑全绿）/ 首包首测试残余最大暴露点 / probe 工具 / config.Load 写盘 | ⚠️ 延续 |
| 新视角 A-D | — | A R61 四修基本正确（四分支互斥完备无第五形态 + "1" 占位三处一致 + 手动测试真实触达）/ B 17/21 / C gofmt 零输出 / D 全包通读零新缺陷 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 13）
连续第八轮零缺陷。新增 O-10（Dashboard「提交中」合并 in_range/submitted 两态，与 N-1 同源）；N-1 冲刺文案连续六轮维持 + 落位点字形级复核；audit.mjs 1 处违例实证为 `hover:bg-neutral-900` 裸匹配误报（基线既有，倾向豁免）；核心推演「/state 首帧未到即点新课 → 回显合并 → 推迟命中（安全方向）+ 三路解锁」正面确认 + 三个新时序推演（手动报名后旧 courses 推迟语义 / handleBack 5s 收敛 / 全清空 PUT 在飞+立刻点新课 lastJson 去重闭环）。六防保存链逐字符零回归。tsc+build 全绿；TDD 断言 16+6+5 全绿。

## 修复（主控核实后 TDD 直修）
| 文件 | 内容 | 验证 |
|---|---|---|
| `zhidao/client.go` | IsReadErr 注释对齐真实错误链路（短读 = body.readLocked 对 LimitedReader 短读包装、Do 层 url.Error、errors.Is 穿透、与 RST 在 body 读阶段互斥） | build+vet 绿 |
| `zhidao/isreaderr_test.go` | 形态矩阵增补 urlError 包装的 ErrUnexpectedEOF 穿透断言（原矩阵未覆盖） | 测试绿 |
| `zhidao/client.go` | fetchLoginPage 4xx/5xx 重试注释对齐「瞬时抖动重试独立于连接自愈、不承诺对限流生效」，行为保留 | build 绿 |

**核实方法**：MINOR-62-01 我亲自对照标准库 transfer.go:865 / transport.go:2422 / transport.go:2347 三段源码逐行核实报告证据链——完全坐实（body 读阶段不产生 transportReadFromServerError、短读是纯 ErrUnexpectedEOF 穿透 url.Error）。收尾全量回归出现 1 次 api 包 FAIL（90s 冷启动残余），隔离复跑全绿（19.1s）+ 3 连跑全绿（136.5s）确认非确定性缺陷、非本次改动引入。

## 收尾全量回归
- `go build ./... && go vet ./...` → exit 0；`gofmt -l .` → 零输出
- 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` → 除 api 1 次冷启动残余 FAIL 外全绿；**api 隔离复跑全绿 + 3 连跑全绿**（非确定性缺陷实锤）
- zhidao 形态矩阵测试全绿（含新 urlError 短读穿透断言）

## 观察项延续（下轮复核）
后端：flake 17/21（OBSERVE-62-01~05，首包首测试残余最大暴露点）/ probe 工具 / config.Load 写盘 + 历轮延续；前端：N-1~N-3 / O-1~O-10 / Dashboard key 不对称 / ui 模板残宽 / NaN 防御 / 多标签页。

## 教训
1. **注释必须追到真实错误链路逐段对齐**：R61 的 ErrUnexpectedEOF 注释（"不带 OpError/transfer.go 短读包装"）与实际链路错位——功能正确不代表注释正确，注释错位会让后续维护者按错误契约推断真实形态（如误以为 Hijack 直断走 RST 分支）。凡涉及标准库包装链的注释，必须把 Do→RoundTrip→body→transfer 每一层 wrap/透传追到底再落笔。
2. **"自愈/重试"语义必须与错误分类严格挂钩**：F52 连接活性自愈（dial/write/read 连接错误）与 fetchLoginPage 的 4xx/5xx 重试（业务响应）是两类——同一函数内重试分支，注释必须写清连接层还是业务层，不得混用语义。
3. **全量轮首包首测试是冷启动残余最大暴露点**：RUN7 失败者是 api 包首测试 TestHealth（readyProbe 自身 Fatal）——每轮全量连跑最先承受 TIME_WAIT 残余的是第一个 mock server 首连接。CI `||` 重跑吸收残余仍是正确姿势。
4. **flake 趋势第 8 轮**：R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → R61 1/16 → R62 17/21——连续全绿段（前 11 轮 10 绿 + 末 10 轮连续 10 绿）证明 readyProbe/socketPreheat 对常规残余有效；四轮反弹均为 TIME_WAIT 极端残余统计尾巴，无确定性缺陷证据。
