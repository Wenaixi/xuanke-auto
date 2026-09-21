# review-round65 总结（2026-09-21）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端**连续十一轮零 MAJOR 零 MINOR**（M-1 修复第二轮闭合确认）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 3**——**生产逻辑连续三轮零 MINOR**。

**修复 3 处**（均为 OBSERVE 级工程卫生：db 平台注释机制精确化 + accounts mock CT 补全 + readyProbe 双保险注释）。

## 审查发现（写入 archive/review-rounds/round65-{backend,frontend}-findings.md）

### 后端（MINOR 0 / OBSERVE 3）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-65-01 | OBSERVE | TestOpenOnReadonlyPath 平台注释机制欠精确——Linux 实测真实行为是 Open 把 `C:` 当普通目录名**真实创建 SQLite 数据库文件并跑完 schema**（err==nil → 断言红），非只"MkdirAll 成功" | ✅ 修复（注释补精确机制叙述） |
| OBSERVE-65-02 | OBSERVE | loginRejectSrv/gateSrv 的 `/login` 响应无 Content-Type（与真实 text/html 差异，识别链路不消费无害） | ✅ 修复（两处补 CT 保持夹具语义对齐） |
| OBSERVE-65-03 | OBSERVE | accounts 第三处 mock 无 socketPreheat 双保险（8 轮全绿实证无残余，防御性观察） | ✅ 修复（readyProbe 注释补第一候选说明） |
| 新视角 A-D | — | A R64 三卫生修①②完全正确（probeIntervalFor 无残留误导 / accounts readyProbe 三处完备无漏点无新 flake）/ B flake **8/8 全绿**（accounts 归零 + api 残余收窄）/ C gofmt 零输出 / D 全包通读零新缺陷 + Linux 跨平台实证 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 12）
连续十一轮零 MAJOR 零 MINOR。**M-1 修复判定「仍然闭合，第二轮确认」**——本轮核心把「机制论证」升级为「端到端实现层证据」：逐段核对 `PUT /api/targets` → `SetTargetsForAccount`（scheduler.go:459-495 持锁）→ `rebuildCoursesForAccountLocked`（575-609 删旧重建 + 发布元数据透传）→ `/state` 只回本账号 courses 完整链路，确认 O-11 全部「保守命中」路径的前提（前端整包 PUT 与后端同构）在实现层成立。新增 O-12（Dashboard `nowMs` 渲染期 `Date.now()` 读取 + extras 折叠行 1s 陈旧窗口——微秒级成本方向安全）；A-④ 新稳态假阳性命中路径新增 1 条（新增改备选后 2s 轮询 + rev>0 回显短路）。六防保存链逐字符零回归 + 五个实证链维持 + react-query 全站 8 键清洁度 + 调试残留/TODO 零命中。tsc+build 全绿；target-guard 断言 18/18。

## 修复（主控核实后 TDD 直修）
| 文件 | 内容 | 验证 |
|---|---|---|
| `db/settings_test.go` | 平台注释补精确机制（Linux 下真实建库得 schema 成功 → err==nil → 断言红，非只 MkdirAll 成功） | 测试绿 |
| `accounts/manager_test.go` | loginRejectSrv/gateSrv 的 `/login` 两处补 `Content-Type: text/html; charset=utf-8` + readyProbe 注释补 socketPreheat 第一候选 | 测试绿 |

**核实方法**：OBSERVE-65-01 我对照报告 Linux 实测证据（Alpine + GOOS=linux 交叉编译二进制：该测试红、且机制是真实建库）核实 R64 注释的表述缺漏。收尾回归 api 110.7s FAIL 无测试名 → **api 隔离复跑全绿（25s）+ 2 连跑全绿（55.9s）** 确认冷启动残余（与 R64 判定同构）。

## 收尾全量回归
- `go build ./... && go vet ./...` → exit 0；`gofmt -l .` → 零输出
- 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` → api 110.7s 冷启动残余 FAIL + 隔离复跑全绿 + 2 连跑全绿
- db/accounts 受影响测试全绿

## 观察项延续（下轮复核）
后端：flake 8/8（R65 全绿收官——R57 3/11 起连续 9 轮趋势，accounts 归零 + api 残余收窄到单测试）/ probe 非可用工具 / stats 半真测试 / OBSERVE-65-01~03 已修 + 历轮延续；前端：M-1 闭合（第三轮确认）/ N-1~N-3 / O-1~O-12 / Dashboard key 不对称 / ui 模板残宽 / NaN 防御 / 多标签页。

## 教训
1. **修正"跨平台断言"推断时必须到目标平台实测而非推演**：R64 推断"Linux 下 `C:\nul\` 是普通目录、断言反转"——推演方向对（Linux 红）但机制没摸全：真实行为是 Linux 下 Open 会把 `C:` 当普通目录名**真实创建数据库文件并跑完 schema**。跨平台假设类问题必须用目标平台运行结果钉死机制（本轮 Alpine + 交叉编译二进制补实证）。
2. **flake 的"地区漂移"印证 R64 预判**：R64 报告已点名"api 包残余收窄后下一个残余点会是 mock 首请求 deadline exceeded 形态"——R65 R1 正是该形态（20.01s 精确=15s 首请求超时窗口），配合 accounts 已归零，残余面收窄到 api 包单测试且趋于瞬时。**持续预判"残余会流向哪个最薄弱的夹具"是收敛工作的正确方法**。
3. **夹具与真实平台差异要登记而非默认为零**：mock /login 无 Content-Type 与真实 text/html 有差异——虽然识别链路不消费 CT，但登记让未来维护者排查 mock 行为偏差时有据可查。
4. **连续多轮零提交的稳态审查重点 = 「上一轮修复闭合的证据升级」而非「找全新缺陷」**：M-1 修复从「机制论证」升级为「端到端后端契约证据」（断言层 → 时序层 → 实现层逐层渗透），证明层次升级本身就是审查进展。