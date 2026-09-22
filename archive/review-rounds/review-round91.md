# review-round91 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 2**；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / 零新 OBSERVE**（连续第三十七轮零严重级）。**双端零代码修改需求**——连续第二轮纯观察轮，核心产出：O91-01/O91-02 记录性观察 + 身份防线矩阵延续闭合 + api 抖动六连零复现 + M-1 第二十七轮闭合。

## 审查发现（写入 archive/review-rounds/round91-{backend,frontend}-findings.md）

### 后端（OBSERVE 2）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| O91-01 | OBSERVE | runtime.Config 中心 `Get` 返回值拷贝语义——RLock 快照读 + Update 写锁原子生效，读一致性成立，防未来误改 | ✅ 记录（实证核实：config.go:35-39 RLock→拷贝 Config 值返回，读侧不受并发 Update 影响，无需动作） |
| O91-02 | OBSERVE | settings 表 `SaveSettings` 单事务全替换 + `LoadSettings` 全读的启动一致性——当前 5 键全替换无缺失；未来新增独立配置键需改增量 Upsert | ✅ 记录（走读推断：store.go:351-366 DELETE+全量 INSERT 事务原子；当前无缺陷，未来加键的注意点） |
| 身份防线矩阵延续 | — | 16 项矩阵逐点复核全闭合（spawnChain 六分支 + 实时复核第七分支 + maybeRelogin 双侧 + MarkDone/RemoveDone/SubmitAll + Restore） | ✅ 闭合 |
| B88-01 持续复核 | — | ProbeForAccount 回写段身份复核三钉实测绿 | ✅ 闭环 |
| O86-01（延续） | OBSERVE | api 抖动**第六轮零复现**——race 全绿（api 27.4s/store 47.3s）connectex 零样本 | ⚠️ 维持基线 |
| O90-01（延续） | OBSERVE | scheduler.go CRLF 噪音——无新变化、无需动作（git 仓库 LF 合规） | ⚠️ 维持观察 |
| M87-01 | — | Shutdown 窗口再评估维持 MINOR + 注释兜底 | ✅ 维持 |

### 前端（零新 OBSERVE / M-1 第二十七轮闭合）
- **M-1 第二十七轮闭合** + 六防保存链零回潮 + 新视角五组全收敛（时间链路 V8 实测零分叉——open_time 空格串本地时区解析=1789261200000 与 begin_times 同点收敛、识别槽 time.UnixMilli→Format 本地时区→前端回解析全链路零分叉 / 列表 key 稳定性 / 事件监听清理 / 深链接初始状态 / 表单可访问性）。
- **OBSERVE-90-01**（Dashboard 日志区三态）/ **88-01**（ErrorBoundary）/ **85-02** / **84-01** / **83-01** / **77-02** / **76-01/02/03** 全部维持「续」不落地。

## 核实记录（关键）
- **O91-01**：读 runtime/config.go:24-48——`Get` 持 RLock 返回 Config 值拷贝（结构体值语义）、`Update` 持写锁应用闭包；读一致性（快照语义）成立，无并发读撕裂。记录性观察确认。
- **O91-02**：读 store.go:351-384——`SaveSettings` 事务内 `DELETE FROM settings` + 全量 INSERT（原子，失败 Rollback）；`LoadSettings` 全读。当前 5 键（activation_enabled/vision_base_url/vision_key/vision_model/captcha_concurrency）全替换无缺失；未来新增独立键若走 SaveSettings 全替换语义需显式带全部键，或改增量 Upsert。记录为未来注意点。

## 收尾全量回归
- 后端 `go test -race -count=1 -p 1 -timeout 900s ./...` → **全 11 包全绿 exit 0**（api 27.4s / store 47.3s / scheduler 15.3s，审查代理实测）
- `go build ./...` / `go vet ./...` 零输出 + Windows CGO1 / Linux CGO0 双平台交叉编译通过
- 前端 `tsc -b` EXIT 0 + `npm run build` 全量成功 + target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 + audit.mjs 77 项

## 观察项延续（下轮复核）
后端：身份防线矩阵（16 项持续复核清单）/ O91-01/02 记录（防未来误改）/ O90-01 CRLF / O86-01 抖动基线（连续六轮零复现）/ M87-01 窗口留档。前端：M-1 延续管理（第二十八轮）/ OBSERVE-90-01 日志区三态（低优先级候选）/ 88-01 / 85-02 / 84-01 / 83-01 + 77-02 + 76-01/02/03 全表续。

## 教训
1. **纯观察轮的观察项也要实证核实后才归档**：O91-01（Get 拷贝语义）与 O91-02（SaveSettings 事务）主控都读了源码确认——「记录型观察」不等于「直接照抄报告」，每条仍过一遍源码级核实（快照读成立 / 事务原子成立）再记档。
2. **api 抖动六连零复现（R86-R91）——基线从「连续三轮」晋升「连续六轮」**：抖动类观察项的「零残余」持续被实测支持，归因链（夹具共享全局态 + Windows 回环冷启动）未被任何新样本推翻；CI `-p 1` + 失败重跑口径稳定。
3. **观察项「维持续」管理进入稳态**：前端 OBSERVE-90-01/88-01/85-02/84-01/83-01/77-02/76 系列连续多轮「续」不落地——每轮代理都复推一遍触发面、确认无新依据，宁缺毋滥不堆不丢。