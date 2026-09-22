# R83 后端只读审查报告

审查对象：xuanke-auto HEAD commit `dca1281`（R82 收官）。本轮重点：R82 api 夹具 `t.Cleanup(sessions.Close)` 修复（336fce0）回归、R82 M82-02 tray 回归钉平台盲区记录确认、build tag 四文件隔离 + trayPNG/ICO 双回归钉（含 R80 硬锚 4264/22）全量走查、构建验证实测、契约 20 全仓扫描、生产逻辑契约抽核、api/scheduler 测试抖动复现实测。

审查方式：全程只读。唯一写入文件为本报告（任务指定输出），仓库工作树零改动。

---

## CRITICAL

无。

## MAJOR

无。

## MINOR

### M83-01：`TestWindowOpenSubmitsWithoutProbeReset` 存在真实可复现的偶发失败——测试前提"探测被 30s 节流挡住"被 tick 的"到点立即探测"分支（`now.After(open) && last.Before(open.Add(-1s))`）破坏

**文件 + 行号：** `backend/internal/scheduler/scheduler_test.go:1275`（失败断言行 `waitStatusAcct(..., "pending", 2s)` 超时，课程状态变 failed）对照 `backend/internal/scheduler/scheduler.go:982-989`（tick 探测闸门双分支）与 `scheduler.go:289-294`（submitIntervalFor/inflight）。

**实测记录（本机 Windows 宿主、Go 工具链，R83 独立复现）：**

| 尝试批次 | 结果 |
|---|---|
| 单跑 10 次 | 失败 2 次（如 `--- FAIL 课程 61115 状态 "failed"，期望 "pending"（结果 "connection reset"）`） |
| count=20 / count=12 / count=15×4 | 各批次 0~1 次 FAIL，累计约 90 次尝试中失败 4 次（约 4~8%） |
| race 下 count=4 | 全绿（race 稀释 tick 时序，反而掩盖） |
| 同族 `TestWindowOpenRetriesWithoutWaitingProbe` 单跑 4 次 | 全绿（对侧证明非系统性缺陷） |

**失败时的完整时序日志（基线锚点）：**

```
[scheduler] 探测成功：3 个发布，窗口状态 false（已关闭 false）   ← 首发探测，窗口未开，pending
[scheduler] 服务端时钟对齐成功，校准偏差: 0s
[scheduler] 探测成功：3 个发布，窗口状态 false（已关闭 false）   ← 第二发探测在 setAllOpened 前发生
--- FAIL: 课程 61115 状态 "failed"，期望 "pending"（结果 "connection reset"）
```

**根因推演：** 测试构造 `openTime = time.Now().Add(-time.Second)`（过去 1 秒）。首轮探测写入 `lastProbe`（已微秒级新近）。随后 tick 进入探测闸门判定（scheduler.go:982-989）：

```go
probe := last.IsZero() || now.Sub(last) >= s.probeIntervalForOpen(now, open)
// 超高性能：窗口到点后的首次 tick 立即探测（不等待节流闸门放过）
if !probe && now.After(open) && last.Before(open.Add(-time.Second)) {
    probe = true
}
```

- 分支 A：`now.Sub(last) >= 30s` 正常不成立（last 新近）。
- 分支 B：`!probe && now.After(open) && last.Before(open.Add(-time.Second))` —— **`open = now-1s` 恰落在 `last` 时间戳开户之后、`open+1s` 边界处**。当首发探测恰在 setAllOpened 调用前的 1s 窗口内写入 `lastProbe`（tick 10ms 间隔 + 并发计时漂移，极端常见），分支 B 的 `last.Before(open.Add(-1s))` 对 open 使用已捕获的旧值（tick 开头快照，恒为 now-1s 的过去时刻），`open.Add(-1s) = 起始时刻-2s`，而 `last` 是 1s 前的首发探测时刻，因此 `last.Before(open.Add(-1s))` 恒成立——**第二个探测必然触发**。setAllOpened 尚未执行 → 第二次探测发现窗口仍未开（窗口状态 false），但它**不改变 lastProbe**（probe() 只写 `s.lastProbe = now`，此处二次探测在单飞守卫下若与首发探测重叠则直接放弃）。随后 setAllOpened 执行、提交循环放行（opened 此时仍 false，但 `now.After(open)` 兜底判据放行，见 scheduler.go:1009），`spawnChain` 进入 `SelectClass`，fakeClient 预置 `selectErr[61115] = errors.New("connection reset")` → 走实时复核（`classFullRealtime` → `IsClassFull` 满员 false）→ 非满员分支 `!doneHas` → `setStateLocked(failed, "connection reset")`（scheduler.go:1650-1660）→ 断言 `waitStatusAcct(pending, 2s)` 超时红。

**本质：** 测试作者前提 "探测永不被重置、提交与探测解耦" 被 tick 中"到点立即探测"分支打破——`open = now-1s` 使得探测节流从"30 秒内不重探"被分支 B 无条件绕过，第二次探测在 setAllOpened 之外抢先执行（或与发布 `selectErr` 到早发探测的亲相互撞），导致 `"connection reset"` 的失败先于窗口开启的 pending 观察出现。

**修复建议（最小改动、不改变测试断言语义）：** 将 `time.Now().Add(-time.Second)` 改为过去更远时刻（如 `-2*time.Minute`）——`last.Before(open.Add(-1s))` 对过去 2 分钟的 `open` 恒不成立（last 恒晚于 `open+1s`），分支 B 永不触发；测试断言（提交不被探测节流挟制、1 秒重试成功）在窗口开启后仍成立（submitIntervalFor 对"过去 2 分钟"的 open 走 250ms sprint，3s 断言窗口充裕）。同时更新该处注释"对 open 过去 1 秒的来源"与"探测永不重置"的表述，注明分支 B 的脆弱边界。回归钉：修复后 50 次 count=1 连跑零失败，并把该测试纳入回归族。

**严重度论证：** MINOR。测试夹具时序假设缺陷，不构成产品逻辑缺陷；但失败频率 4~8%、带 `-race` 反而被掩盖（race 下多次全绿），属高频轮次审查会反复踩中的真实抖动源，值得一个最小改动收口。无产品代码需修改。

### M83-02：api 全量 race 首跑出现一次 FAIL（60.464s api 包），R82 归因的"CGRect 四个抖动测试"之外的形态——api 包偶发失败仍存在，本轮高频复跑未捕获断言行

**文件 + 行号：** `backend/internal/api/handler_test.go`（R82 定位四个时序敏感测试：TestAccountOverrideRequiresAdminSession / TestLoginRateLimit / TestAdminAuth / TestHandleElectivesSelectUnauthorizedRelogin）对照 R82 M82-01 结论。

**实测记录（本轮首次全量 race `-p 1` 实跑）：**

| 轮次 | 结果 |
|---|---|
| 首跑 `-p 1` 全量 race | **api 包 FAIL（60.464s）**，其余 10 包全绿 |
| 立即全量 race 第 2 次 | **11 包全绿** |
| 全量 race 第 3 次 | **11 包全绿** |
| api 包 `-count=3 -p 1`（race） | 全绿 |
| 四抖动测试定向 `-count=5`（race） | 全绿 |
| 四抖动测试单跑×3 / `-count=3` | 全绿 |
| 全包 `-count=2`（race，两个 go test 进程并行） | **全绿** |

高达 10 轮以上的族样本看，api 包抖动仍为低频偶发，首跑失败未留下断言行（grep 无 `Error `/`FAIL:` 明细被覆盖，与 R81/R82 所述"断言失败但逐断言捕获需复现环境"一致）。R82 的根因归因（夹具共享全局态：登录频率闸门/限流桶/全局验证码识别信号量 `globalLimiter`/mock Handler 整体替换窗口）仍然成立，本轮无新证据推翻或深化。

**触发场景推演：** 首跑 `-p 1` 与前三轮全绿比更接近 R81 的 1/12 FAIL——高频压测吞吐（本机 8 核 + Windows 回环）下全局态排队窗口抖动阈值偶然命中。产品逻辑无涉。

**修复建议（可选，不阻塞）：** 长期仍建议把 `getGlobalLimiter`（CAPTCHA 信号量，`captcha.go:43-51`，`sync.Once` 进程级单例）与登录频率闸门在测试环境下隔离实例化，消除跨测试串扰；短期维持 CI `-p 1` + `||` 重跑基线。

**严重度论证：** MINOR。多轮高频压倒性全绿 + 首跑单次偶发，归因方向不变、非产品缺陷。

## OBSERVE

### O83-01：`internal/session/store_test.go` 中 10 处 `session.New(time.Hour)` 测试仍缺 `defer s.Close()`——与 R82 `O82-01` 在 api 夹具上收口的"清扫协程泄漏"同型，session 包内部测试同样存在，范围小于 api（10 测试 × 单包常驻）

**文件 + 行号：** `backend/internal/session/store_test.go:10/30/39/48/67/81/128/156/172/215`（仅 `:173` TestSweepExpired 带 `defer s.Close()`，`TestSweeperLoopStopsOnClose` 自身测 Close 幂等）。

**推演：** `session.New(time.Hour)`（ttl>0）启动 `sweepLoop` 协程，测试结束不 `Close` 则协程随进程存活至 go test 退出。session 包测试总量小、单进程内协程数 ≤10，无正确性影响。R82 已为 api 夹具注册 `t.Cleanup(sessions.Close)`（0加 0 冲突），session 包内部未同步收口——纯测试卫生观察，非缺陷。

**修复建议（可选）：** 各测试统一 `defer s.Close()`（`sync.Once` 幂等）。成本一行一测，收益是包内完全无清扫协程残留。

### O83-02：R82 `T82-01`（api 夹具 Cleanup 修复）的注释保留 "O82-01" 编号引用——契约 20 允许叙述性历史锚点，但编号引用与"轮次前缀标签"边界接近，后续维护可顺手化为语义表述

**文件 + 行号：** `backend/internal/api/handler_test.go:152` `t.Cleanup(sessions.Close) // 防清扫协程泄漏（O82-01：55 测试 × 高频轮次产生数百常驻协程窗口）`

**推演：** 与 `session/store.go:117` 的 `docs/review-round13.md` 同类——历史锚点叙述，非"X-XX（第 N 轮）"前缀标签形态，契约 20 判定为许可语义；但 O82-01 是"轮次-编号"格式，后续维护者误扫时可能触发一轮"清理"误判。不构成缺陷，仅记录边界。

## 可疑待核

- **api 首跑 FAIL 的精确断言行**：本轮未能捕获（4 次全量 race + count=3 + count=5 定向全绿）。若主控希望闭环，可在持续重建失败的 runner 上抓 `go test -race -count=10 -p 1 ./internal/api/` 的 `-v` 输出，定位逐断言红行后再对照 R82 四测试族归属归档。
- **`TestWindowOpenSubmitsWithoutProbeReset` 断言红行 1275 的稳定性**：M83-01 修复后建议以"50 次连跑零失败"作为新增守护基线（R82 已有 `TestWindowOpenSubmitsWithoutProbeReset` 列入 CI 回归族）。

## 已核无缺陷清单

### R82 api 夹具 `t.Cleanup(sessions.Close)` 回归（336fce0）

| 检查项 | 结论 |
|---|---|
| 只加一行 | 是。一行 diff，`session.New(time.Hour)` 后紧跟注册，无其他改动 |
| 与其他 Cleanup 顺序无干扰 | `t.Cleanup` LIFO 语义：注册序为 zhi.Close → d.Close → sessions.Close → sched.Stop；Close 与 Stop 相互独立（session.Store 与 scheduler 无共享对象），任一序全安全 |
| sessions 变量后续使用不受影响 | `testDeps.sessions` 字段/`Register` 参数均复用同一指针（handler_test.go:174-176/301/819/867 等），Close 只在测试终点触发，期间读写无碍 |
| Close 幂等 | `sweeperClose sync.Once`（store.go:34/91-97）；`sweeperStop == nil`（ttl<=0 未启动）早退安全 |
| 是否存在未注册 Close 的其他 New | `store_test.go` 全部 `New(time.Hour)` 实测同上（O83-01 记录） |
| gofmt | `gofmt -l .` 零输出 |
| go vet | `go vet ./ ./internal/...` 零输出 |
| TestSweeperLoopStopsOnClose（-race 死锁检测） | PASS |
| TestCreateAndAccount 等 session 全量 | PASS（`go test -race ./internal/session/` 1.437s） |

### R82 M82-02 tray 回归钉平台盲区确认（无需修改，事实记录）

- Windows ICO 回归钉 `TestTrayIconAsset`：本机 `GOOS=windows CGO_ENABLED=1 go build -o` 编译通过；`go test -race`（windows 宿主跑 `//go:build windows`）**实测 PASS**。
- Linux PNG 回归钉 `TestTrayPNGAsset`（`//go:build linux && cgo`）：Windows 宿主不可执行，CI ubuntu runner 覆盖 + 静态走读（`png.Decode` 严格 CRC + 16x16 + 中心 6..9 白不透明断言与实现 ROI 匹配）。
- 分工事实无变化，维持 R82 结论。

### build tag 四文件隔离 + trayPNG/ICO 双回归钉全量走查

| 文件 | build tag | 结论 |
|---|---|---|
| `tray_windows.go` | `//go:build windows` | Windows 活托盘；ICONDIR/BITMAPINFOHEADER 逐字节铺布通过 4264 硬锚验证（R80 硬锚三加数独立推导） |
| `tray_linux.go` | `//go:build linux && cgo` | Linux 桌面托盘（systray + PNG + zenity） |
| `tray_linux_cgo0.go` | `//go:build linux && !cgo` | Linux CGO=0 占位 |
| `tray_other.go` | `//go:build !windows && !linux` | darwin 等占位 |
| `tray_asset_windows_test.go` | `//go:build windows` | 回归钉，Windows 实测 PASS |
| `tray_asset_linux_test.go` | `//go:build linux && cgo` | 回归钉，CI ubuntu 执行 + 静态走读 |
| `internal/zhidao/native_ocr.go` | `//go:build windows && cgo` | 内嵌 ddddocr 实测路径，`dumpIfDiff` 释出 + 官方 OCR 模式（`ModelDir` 固定名），与 stub 互斥 |
| `internal/zhidao/native_ocr_stub.go` | `//go:build !windows \|\| !cgo` | 回退存根 |

四组合交叉编译实测：win-CGO1 / linux-CGO0 / darwin-CGO0 / win-CGO0 全绿。R80 硬锚 4264（`tray_windows.go:96` 写入、`tray_asset_windows_test.go:27` 独立推导）与 offset=22 断言一致。

### 构建验证（本轮实测）

| 命令 | 结果 |
|---|---|
| `go build ./...` | BUILD_OK |
| `go vet ./...`（前台）+ `go vet ./ ./internal/...` | VET_OK（均 0 输出） |
| `gofmt -l .` | 零输出 |
| `go test -race -count=1 -p 1 -timeout 900s ./...` 第 1 次 | **api FAIL（单次偶发，M83-02）**，其余 10 包全绿 |
| 同上第 2 次 / 第 3 次 | **11 包全绿**（api 35.8s / store 39.3s / scheduler 14.3s 等） |
| `go test -race -count=2 -p 1 ./...`（与另一进程并行） | **11 包全绿**（api 45.3s / scheduler 27.4s） |
| `go test -race -count=3 -p 1 ./internal/api/` | 全绿 58.3s |
| 四抖动测试定向 `-count=5`（race）+ 单跑×3 | 全绿 |
| `TestWindowOpenSubmitsWithoutProbeReset` 连跑（90 余次） | 失败 4 次（M83-01） |
| 关键回归测试族（scheduler/api/db/session/store）定向 | 全绿（见下） |
| 四组合交叉编译（win-CGO1 / linux-CGO0 / darwin-CGO0 / win-CGO0） | 全绿 |

### 关键回归测试族（定向执行，全绿）

| 测试族 | 覆盖契约 |
|---|---|
| TestTrayIconAsset（Windows 实测）/ TestTrayPNGAsset（linux&&cgo 走读） | 托盘 ICO/PNG 回归钉 + 4264/offset 硬锚 |
| TestWindowOpenSubmitsWithoutProbeReset（M83-01，本批 90 余次 4 次绿） | tick 守卫 + 提交/探测节流解耦 |
| TestSubmitSuspendedWhenOpenTimeCleared / TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime | 零值守卫与 WindowOpened 统一 |
| TestWindowOpenRetriesWithoutWaitingProbe | 提交不依赖探测节流复位（旁证 M83-01） |
| TestMigrateAddsPublishMetaColumns / TestRefuseLegacyDB / TestOpenOnReadonlyPath | 数据库增量迁移 + 拒绝形状 |
| TestSweeperLoopStopsOnClose | sweeper 幂等 Close |
| TestDeletedAccountRebuiltSameNameChainDrops{...} | spawnChain 身份防线六分支 |
| TestMaybeReloginDeletedAccountSkipsMaps | maybeRelogin 入口存在性复核（scheduler.go:1205-1211 实测含 `ClientFor` 复核） |

### 契约 20 扫描（全仓）

扫 `（O|B|M|F|C|R|[0-9]+-[0-9]+[）)]` 与 `(第\s*\d+\s*轮|R\d{2,}|round\d+)`：

- 生产代码（非测试）：**零命中**（唯一 `session/store.go:117` 的 `docs/review-round13.md` 文档路径，许可）。
- 测试代码：`scheduler_test.go` 的 B29-02/B42-02/B43-02、`tray_windows.go:93` 的 R79、`tray_asset_*` 的 R76/R77/R79、`handler_test.go:152` 的 O82-01——均为历史锚点叙述，非 "X-XX（第 N 轮）" 前缀标签形态，契约 20 判定许可；O83-02 记录边界。
- `XK-` 激活码样例文本、`only `connection reset`` 等错误字符串为假阳性。
- **结论：零轮次前缀标签残留，符合契约 20。**

### 生产逻辑契约抽核

| 契约 | 结论 |
|---|---|
| handleElectives 空发布映射 | 通过。`handleElectives`（handler.go:235-277）直读 `ElectivesSnapshotFor`/`ProbeForAccount`；窗口关闭后空 publishes 由 `/state.courses` 的 `publish_name/begin_date` 元数据透传（scheduler.go:19-28/604），前端分组零依赖 /electives。 |
| StateForAccount Courses 构造 | 通过。按账号过滤 `s.state.Courses`（scheduler.go:722-727）；`rebuildCoursesForAccountLocked`（574-616）按目标重建，refused 优先覆盖 done 恢复文案（599-608）。 |
| DeleteRefusedClass 单课删除 | 通过。`store.go:176-179` `DELETE FROM refused WHERE account=? AND class_id=?` 只删单课；`TestDeleteRefusedClassOnlyRemovesOneClass` PASS（跨账号/跨课保留断言全绿），幂等删除 99999 不报错。 |
| TryAcquireSubmit inflight 互斥 | 通过。`TryAcquireSubmit`（scheduler.go:1891-1909）`s.inflight[acct][classID]` 置位 + `sync.Once` release 幂等释放；spawnChain 内 `inflightHas` 前置跳过（1459-1462）+ MarkDone 同步清位（1945-1947），杜绝双发包。 |
| LoadCredentials 单账号 | 通过。SQLite `SELECT ... ORDER BY account` + 遍历比对（handler.go:1073-1084），O82-04 规模边界记录维持。 |
| submitAll 300ms 拍 | 通过。`submitIntervalFor`（289-294）黄金期 250ms / 常规 1s；tick 末尾 `now.Sub(lastSubmit) < submitInterval` 闸门（1026-1028）；`spawnChain` 逐发布分组 + 优先级排序（1353-1359）。 |
| maybeSyncClock 失败退避 | 通过。失败落地 `lastSyncFailAt`（376 行）→ 30s 退避（346 行）；`syncing` 单飞（355 行）；成功才推进 `lastSyncTime`（397 行）；streak≥3 复位 clockOffset（388 行）且在 windowClosedLocked 判据带"开放时间已过"（920 行）。 |
| native_ocr 资产释出 | 通过。`ensureInit` 懒加载释出 `%TEMP%\xuanke_ddddocr_assets`，`dumpIfDiff` 大小比对（native_ocr.go:95-103）；官方 OCR 模式（ModelDir 固定名）与 stub 按 build tag 互斥。 |
| handleLogin 管理员双条件 + 时延拉平 | 通过。`req.Account==adminName && ConstantTimeCompare==1` 才管理员签发（handler.go:121），错误分支 `loginTimingFlat` 300ms（136 行）；撞名学生走教务登录正常签发。 |
| LoginByPassword gateTryAcquire | 通过。非阻塞准入，quota 满即拒（manager.go:243-245）；`wasShell` 判别失败只摘新建空壳（253/264-274），既有客户端绝不误删。 |
| 删账号 memory-first 四步序 | 通过。`Remove → PurgeAccount → DeleteAccount → RevokeAccount`（handler.go:1001-1025）；PurgeAccount 清 16 处 map key + Courses（scheduler.go:499-522）。 |
| maybeRelogin 入口 ClientFor 复核 | 通过。决策侧锁内 `ClientFor` 存在性复核（scheduler.go:1205-1211），写回侧成功分支复核 + `sameClientFor` 六分支闭合。 |
| session sweepLoop / LoadAllLogs 窗口 | 通过。`Close` 幂等（store.go:90-98）；日志窗口 `id>(max(id)-20000)` + LIMIT（store.go:235/422）。 |

---

## 结论

R83 后端只读审查**无 CRITICAL、无 MAJOR**。核心工作：

1. **R82 api 夹具 Cleanup 修复回归唯一通过**：一行 diff、注册序安全、幂等 Close 实测、gofmt/vet 零告警。
2. **M83-01 新收口**：`TestWindowOpenSubmitsWithoutProbeReset` 是真实可复现的偶发（本机 90 余次 4 FAIL，~4-8%），根因在测试夹具的 `open=now-1s` 与 tick "到点立即探测"分支（scheduler.go:984）碰撞，非产品缺陷；给出最小修复（open 改过去偏差）与回归守护建议。
3. **M83-02 维持 R82 api 抖动归因**：全量 race 首跑一次 api FAIL，后续 10+ 轮高压全绿；根因（夹具共享全局态）方向不变，无产品逻辑涉洞。
4. **构建验证**：build/vet/gofmt 全绿；四组合交叉编译全绿；tray 双回归钉（Windows 实测 + Linux 走读）与 R80 硬锚 4264/22 完好。
5. **契约 20 扫描**零轮次前缀标签残留。
6. 新增 2 MINOR + 2 OBSERVE（1 个测试夹具卫生 + 1 个注释编号边界），无产品代码需修改。

后端整体健康状况良好。唯一建议尽快执行的是 M83-01 的最小测试修复（否则后续每轮 CI/审查都会随机踩红）。