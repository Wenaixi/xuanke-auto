# review-round60 总结（2026-09-21）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端零缺陷轮（MAJOR 0 / MINOR 0 / OBSERVE 12 含新增 N-4）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 3 / OBSERVE 9**——产品逻辑连续四轮零 CRITICAL、零 MAJOR。

**修复 2 处**（IsReadErr 形态全集补齐 + 手动路径文案对称）。

## 审查发现（写入 archive/review-rounds/round60-{backend,frontend}-findings.md）

### 后端（MINOR 3 / OBSERVE 9 + 延续 9）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MINOR-60-01/02 | MINOR | R59 IsReadErr 只覆盖 RST 形态——**FIN（服务端读完 body 后正常 Close，真实平台"已处理未响应"的典型形态）序列化成纯 io.EOF 不带 net.OpError，errors.As 不命中**；超时形态（awaiting headers）同样 miss → scheduler 仍落"报名失败: EOF"误导文案（R59 修复半程生效） | ✅ 修复（IsReadErr 增 FIN io.EOF + 超时两分支 + 形态矩阵契约测试） |
| MINOR-60-03 | MINOR | 手动报名/退选路径（handler.go:353/:417）未区分 read 文案——`zhidao.IsReadErr` 已导出但全仓仅 scheduler 一处消费，前端手动路径 UX 与自动链不对称 | ✅ 修复（handler 两处复用 IsReadErr 区分文案） |
| OBSERVE-60-01~09 | OBSERVE | flake 2/12（R9 TestCaptchaConcurrency 53s + R12 TestRecognizeCaptcha，隔离复跑+5 连跑全绿）/ stats 注释残留 / targets nil / 撞名文案 / probe 工具 / bench 默认 401 / logout 单会话 / config.Load 写盘 / **access_limit_cookie 占位值"1"语义漂移（新）** | ⚠️ 延续 |
| 新视角 A-F | — | A 404 修复完全闭合 + IsReadErr RST 正确但集合不完整 / B 全量 12 轮 10 绿 / C gofmt 零输出 / D url.Error 穿透实证 + FIN/超时 miss / E MINOR-59-01 闭合 + 59-02 升级 / F 产品零缺陷 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 12）
连续第六轮零缺陷。新增 N-4（极轻）：toast 去重 key 全仓字形复核——模板字面量 title 与普通字符串在合并 key 维度等价（插值后恒为字符串），去重语义不受影响，信号弱不改动。N-1（冲刺文案）连续四轮后明确裁决维持 + 给出一行落位点（若整治 = Select.tsx:1134 统一「后台目标」删"冲刺"）。六防保存链逐字符零回归 + 三个关键时序推演无丢失路径。tsc+build 全绿；TDD 断言 16+6+5 全绿。

## 修复（主控核实后直修）
| 文件 | 内容 | 验证 |
|---|---|---|
| `zhidao/client.go` | IsReadErr 增补三形态覆盖：`errors.Is(err, io.EOF)`（FIN 正常关闭）+ `strings.Contains(..., "Client.Timeout exceeded while awaiting headers")`（超时）+ 原 RST（net.OpError read，errors.As 穿透 url.Error 链） | build+vet 绿 |
| `zhidao/isreaderr_test.go` | 新增 `TestIsReadErrCoversAllForms` 形态矩阵契约测试（RST/url.Error 包装/FIN/超时命中 + dial/write/业务/nil 不命中） | 测试绿 |
| `api/handler.go` | 手动报名（353）+ 退选（417）错误分支复用 `zhidao.IsReadErr` 区分 read 文案（与 scheduler 自动链同款） | api 包测试绿 |

## 收尾全量回归
- `go build ./... && go vet ./...` → exit 0；`gofmt -l .` → 零输出
- 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` → **全包全绿**（10 包 0 FAIL）
- api/zhidao/scheduler 三包测试全绿（含新形态矩阵测试）
- 前端 tsc + tsc -b + npm run build 全绿（1948 modules / 417.54 kB js / 41.07 kB css）

## 观察项延续（下轮复核）
后端：flake 2/12（OBSERVE-60-01，收敛趋势 R57 3/11→R58 2/10→R59 2/11→R60 2/12 短暂反弹）/ stats 注释残留 / targets nil / 撞名文案 / probe 工具 / bench 默认 401 / logout 单会话 / config.Load 写盘 / access_limit_cookie 占位语义 + 历轮延续（accountExists 全表 / tick 无 recover / 孤儿登录 / 双槽分叉 / reloginResults / 429 头 / 本地钟）；前端：N-1~N-4 / O-1~O-8 / Dashboard key 不对称 / ui 模板残宽 / NaN 防御 / 多标签页。

## 教训
1. **修复正确性必须按"错误形态全集"验证**——R59 的 IsReadErr 只按报告复现形态（RST）验证"正确"，但真实平台"已处理未响应"的典型形态是 FIN（正常关闭连接）——Go 把 FIN 序列化成纯 `io.EOF`（不带 net.OpError），errors.As 不命中。修复验证要枚举该判定函数需要覆盖的全部输入形态，不只验证报告的复现样本。
2. **RST vs FIN 的错误链差异**——服务端 RST（有未读数据 SO_LINGER(0) 关闭）→ `read tcp ...`（带 net.OpError）；服务端 FIN（正常 Close）→ `url.Error{Err: io.EOF}`（不带 OpError）。判定"平台是否已处理"必须同时覆盖两形态，FIN 恰是"服务端已完整消费 body 正常结束"的最强信号。
3. **对称修复要全路径清点**——R59 只在 scheduler 自动链区分 read 文案，手动报名/退选路径漏了（grep 全仓 `zhidao.IsReadErr` 消费点仅 scheduler 一处即可发现）；与 R37 教训"每次补身份复核都要全分支清点"同构——新工具/新判定函数落地后必须 grep 全部潜在消费点。
4. **flake 收敛有波动性**——R60 2/12 短暂反弹（R9 TestCaptchaConcurrency 53s 异常时长提示信号量计数在并发首请求 connectex 时被误判），但隔离复跑 + 5 连跑全绿证明非确定性缺陷；收敛趋势长期向好，CI `||` 重跑吸收是工程常态。
