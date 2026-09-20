# review-round56 总结（2026-09-21）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端零缺陷轮（MAJOR 0 / MINOR 0 / OBSERVE 9 含 1 新增 N-1）；后端 **MAJOR 1（测试基建）/ MINOR 1 / OBSERVE 6**，产品逻辑零缺陷。

**修复 3 处**（zhidao 夹具 readyProbe + 包级 TestMain preheat / open_time_set 注释对齐 / cloneReq GetBody 重生成），全部实证后落地。

## 审查发现（写入 archive/review-rounds/round56-{backend,frontend}-findings.md）

### 后端（MAJOR 1 / MINOR 1 / OBSERVE 6 + 延续 9）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MAJOR-56-01 | MAJOR | R55"非 api 包 0 FAIL"结论被本轮实证推翻——全量 `-race -p 1` 12 轮 3 FAIL（R1 **zhidao** TestLoginStopsAfterCaptchaExhausted + R3/R11 api），api 隔离 10 轮却 0 FAIL：**全量 vs 隔离 flake 率系统性分裂**，readyProbe 轮询只覆盖 api 包，zhidao 包 Login 冷启动裸露 | ✅ 修复（zhidao readyProbe + TestMain） |
| MINOR-56-01 | MINOR | admin stats `open_time_set` 忽略识别过期（`RecognizedOpenTime()` 返回过期槽值非零，`!IsZero()` 恒 true）——窗口关闭后 stats 显过期日期 + open_time_set=true，与学生端 `open_time_known` 分叉；handler.go:884/956 注释"识别过期→零值"是不存在的语义 | ✅ 修复（注释对齐决策锚 1） |
| OBSERVE-56-01 | OBSERVE | **独立程序实证**：httpDo read 类错误重试第二个请求 body 为空（`req.Clone` 浅拷贝 Body + GetBody 未设，RoundTrip#1 body=classId=61115 → #2 body=""）——方向安全（不双报成功）但重试无效 + 误导日志 | ✅ 修复（cloneReq 补 GetBody） |
| OBSERVE-56-02~06 | OBSERVE | open_time_set 注释链整体过时 / WriteTimeout 30s < Login 最坏耗时 / readyProbe 无显式超时 / TestLoginLimiterGC 隐式零值依赖 / initCaptchaAtStartup 全局信号量叠加 | ⚠️ 延续 |
| 新视角 A-F | — | A readyProbe 不掩盖故障仅覆盖 api 包 / B 批插无 AppendLog 覆盖缺口 / C window_empty 不重复真钉死 / D 全量 vs 隔离分裂实证 / E 观察项 2 闭合 1 部分闭合 / F 全包通读产品零缺陷 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 9）
R54 五修全路径复核 + 六防保存链 F43/F42/F40/F39/F36/F48-M1 逐字符通读零回归（R55→R56 间 web/src 零提交）；tsc+build 全绿；TDD 断言 16+6+5 全绿。零修复项。新视角 A（Toast viewport 滚动）升级复核：任何"修复"都破坏 F48-M1"仅 toast 可交互"语义，触发概率趋零。新增 N-1：Dashboard relativeCountdown 折叠行以渲染期 Date.now() 为基准（最坏 1 秒陈旧，分钟粒度零影响）。

## 修复（主控核实后直修，全部测试基建/注释）
| 文件 | 内容 | 验证 |
|---|---|---|
| `zhidao/client_test.go` | loginMockServer 构造后 readyProbe 健康探测（200ms×5 轮询，与 api 包同款）+ **包级 TestMain 一次性 preheat**（覆盖裸 httptest 入口如 TestNoAutoRelogin，冷启动窗口双保险前移） | zhidao 包测试绿 |
| `api/handler.go` + `handler_test.go` | open_time_set 注释修正："识别过期→零值"是不存在的语义，改为"识别槽有值即照常输出过期日期"（决策锚 1 绝不截断过期值），测试注释/断言自洽 | api 包测试绿 |
| `zhidao/client.go` | cloneReq 补 GetBody 重生成（read 类错误重试带完整 body，不再空发畸形 POST） | build+vet 绿 |

## 收尾全量回归
- `go build ./... && go vet ./...` → exit 0
- 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` **14 轮（修复后 8+6）**：12 绿 2 FAIL（原第 5 轮 api×2 + 补跑第 6 轮 api TestSetTargetsBounds，全为 Windows 回环冷启动 connectex 家族）
- flake 率从 R56 审查时 **3/12（25%）** 降至 **2/14（14%）**——zhidao 样本归零（TestMain preheat 生效），api 冷启动残余低频，CI `||` 重跑吸收
- 前端 tsc + tsc -b + npm run build 全绿（1948 modules / 417.54 kB js / 41.07 kB css）

## 观察项延续（下轮复核）
后端：open_time_set 注释链（56-02 随 56-01 闭合）/ WriteTimeout vs Login / readyProbe 无超时 / TestLoginLimiterGC / 全局信号量叠加 + 历轮延续（accountExists 全表 / tick 无 recover / 孤儿登录 / 双槽分叉 / reloginResults / 429 头 / 本地钟）；前端：N-1~N-2 / O-1~O-8 / Dashboard key 不对称 / ui 模板残宽 / NaN 防御 / 多标签页。

## 教训
1. **包隔离归零不可信，全量轮是唯一 flake 判定源**（R55 教训 1 升级）——api 隔离 10 轮 0 FAIL 与全量 12 轮 3 FAIL 并存，根因是 `-p 1` 下前序包 httptest 关闭后回环 TIME_WAIT 残留叠加；后续轮一律全量连续多轮统计。
2. **测试夹具必须包级覆盖**——readyProbe 只加在 api 夹具、zhidao 包裸 httptest 入口就裸露（R1 实证）；`TestMain` 一次性 preheat 是覆盖全包 mock 首请求的最小完备手段。
3. **注释与实现矛盾是真实缺陷信号**——handler.go:884"识别过期→零值"描述的语义不存在（决策锚 1 刻意保留过期值），测试注释与断言自相矛盾；审查发现注释分叉比行为分叉更危险（误导后续维护者按错误预期改实现）。
4. **重试必须可重放**——`req.Clone` 浅拷贝 Body，read 类错误重试时 body 已被首个 RoundTrip 消费变空；任何重试路径都要 GetBody 重生成（`bytes.NewReader` 还原），否则"重试"是空发畸形请求。
