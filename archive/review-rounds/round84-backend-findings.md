# R84 后端只读审查报告

审查对象：xuanke-auto HEAD commit `42508fe`（R83 收官，进度 84/256）。本轮重点：R83 `TestWindowOpenSubmitsWithoutProbeReset` 夹具重构（4460dc3）时序正确性复核、M83-02 api 抖动归因维持、build tag 全文件互斥 + tray ICO/PNG 双回归钉（含硬锚 4264/22）全量走查、构建/交叉编译实测、契约 20 全仓扫描、生产逻辑契约新角度抽核。

审查方式：全程只读。唯一写入文件为本报告，仓库工作树零改动（`git status --short --branch` 为 `## master` 洁净，无未提交变更）。

---

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE

### O84-01：M83-01 实际修复方案（未来 open）优于原报告建议（过去 -2m）——建议方案与根因同源、实为无效修复，作者实现正确且验证充分

**文件 + 行号：** `backend/internal/scheduler/scheduler_test.go:1271`（`time.Now().Add(5*time.Second)`）对照 R83 报告 M83-01 修复建议（"open 改过去更远时刻如 -2m"）。

**推演：** R83 报告建议的 `-2m` 方案只消除了 tick"到点立即探测"分支（scheduler.go:984 `now.After(open) && last.Before(open.Add(-1s))`）对探测节奏的干扰，但未触及真正的 pending 断言击败源——提交守卫 scheduler.go:1009 `!opened && !now.After(open)` 对过去 open 恒为 `true && false = false`（不挂起），窗口未开（InDateRange=false）时 spawnChain 即打 SelectClass，connection reset 把状态打上 failed，首段 `waitStatusAcct(pending, 2s)` 断言仍会被击败。实际提交 4460dc3 采用未来 open + opened=false 让提交正确挂起（守卫 `!now.After(open)` 对未来 open 恒 true → return），再 `setAllOpened` + `resetProbe` 让下一 tick 探测置 opened=true 走正常 1s 闸门——这才真正修复根因。作者提交信息对根因的表述（"tick 提交守卫对过去 open 恒放行（黄金期兜底刻意语义），窗口未开时也提交"）与代码一致，实现方案优于 R83 建议且验证充分（本次 count=10 × 3 组连跑全绿，见构建验证表）。

**修复建议：** 无需修复。仅记录：后续审查若再遇同类夹具时序问题，根因定位应同时覆盖"提交守卫的黄金期兜底语义"与"到点立即探测分支"，二者独立作用、缺一不可。

### O84-02：handleSetTargets 目标双重落库存在毫秒级中间态——api 层先落未补全元数据版本、scheduler 层补全后覆盖，最终一致且有重启自愈

**文件 + 行号：** `backend/internal/api/handler.go:517-521`（`d.Store.SetTargetsForAccount(acct, req.Targets)` → `d.Sched.SetTargetsForAccount`）对照 `backend/internal/scheduler/scheduler.go:477-482`（scheduler 内部 `enrichTargetPubMetaLocked` 补全后再落库）。

**推演：** api 层先落库时 targets 未补全 publish_name/begin_date（HTTP 请求体只带 publish_id/class_id 等核心字段），随后 scheduler 层 `enrichTargetPubMetaLocked` 从快照补全元数据并再次落库覆盖。若 scheduler 层落库失败（磁盘满/IO 故障，failStore 场景），库内残留未补全版本——重启后 `RestoreTargets` 走 `enrichTargetPubMetaLocked` 兜底（scheduler.go:535，启动后 probe 很快填充全校帧再补全）自愈。功能正确、无数据丢失，属已知接受行为（代码注释已声明 HTTP 直存时专属帧常过期/为空的补全时序），记录为观察。

### O84-03：api 夹具 Cleanup 注释的编号锚点持续存在（延续记录）

**文件 + 行号：** `backend/internal/api/handler_test.go:152` `t.Cleanup(sessions.Close) // 防清扫协程泄漏（O82-01：55 测试 × 高频轮次产生数百常驻协程窗口）`。

**推演：** 与 session/store.go:117 的 `docs/review-round13.md` 同类——叙述性历史锚点，非"X-XX（第 N 轮）"前缀标签形态，契约 20 判定许可；但 O82-01 是"轮次-编号"格式，后续维护者可顺手化为纯语义表述。不构成缺陷，仅记录边界（前轮已记，本轮延续确认无回潮）。

## 可疑待核

- **api 首跑偶发 FAIL 本轮未复现**：全量 race 多轮连跑（scheduler+api 全量 / accounts+store+session / zhidao+db 等）全部零 FAIL。M83-02 归因（R82 四时序敏感测试族 + 夹具共享全局态：登录频率闸门 / 限流桶 / 全局验证码识别信号量）方向维持，无新证据。若主控要闭环精确断言行，需在持续重建失败的 runner 上抓 `go test -race -count=10 -p 1 ./internal/api/ -v` 输出。
- **linux-CGO1 交叉编译无法在 Windows 宿主验证**：`GOOS=linux CGO_ENABLED=1 go build` 报 `grp.h: No such file or directory`（宿主无 Linux C 头文件），属工具链环境限制非代码缺陷；release.yml 在 ubuntu runner 覆盖该形态，windows 原生 CGO1 已实测通过（内嵌 ddddocr 路径）。

## 已核无缺陷清单

### 1. R83 TestWindowOpenSubmitsWithoutProbeReset 重构（4460dc3）时序正确性四连核

| 检查项 | 结论 |
|---|---|
| a) 首段 pending 断言在 opened=false + 未来 open 下提交确已挂起 | 通过。守卫 scheduler.go:1006/1009 `open.IsZero() && !opened`、`!opened && !now.After(open)`——open=now+5s 未来、opened=false（首发探测 InDateRange=false）→ 两守卫均 return 挂起；fakeClient 无 BeginTimes，识别槽不写入，open 回退 New 传入的 now+5s（scheduler.go:439-441 遗留字段兜底）稳定成立。count=10 连跑全绿 |
| b) resetProbe 后下一 tick 探测读新数据置 opened=true、提交经正常 1s 闸门 | 通过。`resetProbe()` 置 lastProbe=零值（test:396-400）→ tick 探测判据 `last.IsZero()`（scheduler.go:982）立即探测 → InDateRange=true → opened=true → 提交放行；open 仍未来 → submitIntervalFor 走常态 1s（无 250ms 冲刺），3s 断言窗口充裕 |
| c) 与 TestSubmitSuspendedWhenOpenTimeCleared 等 tick 守卫测试无交叉破坏 | 通过。零值守卫测试用 `New(..., time.Time{}, ...)` + 注入 WindowOpened=true/手动 tick，与未来 open 夹具互不干扰；同族 4 测试 count=10 连跑全绿 |
| d) 修复后连跑稳定性 | 通过。`-count=10` 连跑 3 组（每组含该测试+守卫测试族）全绿；另有 race 下 count=2 全绿。原抖动源（90 次 4 FAIL ~4-8%）已根除 |
| 注释与实现一致 | 通过。1262-1267 注释描述的未来 open 语义、分支 B 对未来 open 不触发、setAllOpened 后 resetProbe 与实现逐条吻合 |

### 2. M83-02 api 抖动归因维持（本轮实测）

| 轮次 | 结果 |
|---|---|
| scheduler 全量 race（`-race -count=1 -p 1`） | PASS（15.2s） |
| api 全量 race（`-race -count=1 -p 1`） | PASS（27.4s） |
| scheduler+api 全量 race 连跑 | 双 PASS（15.3s / 18.8s） |
| accounts+store+session 全量 race | 三 PASS |
| zhidao+runtime+config+secure+db | 五 PASS |
| api 四抖动测试族 `-count=5`（race） | PASS（7.7s） |
| scheduler 删号竞态族 + 窗口空发布族 `-count=2`（race） | PASS（2.2s） |
| admin stats / logout 吊销 / 迁移族 `-count=2`（race） | PASS（2.9s） |

本轮全量 race 首跑及多轮连跑均零 FAIL，R82/R83 归因（时序敏感测试族 + 全局态串扰）方向维持，无产品逻辑涉洞。

### 3. build tag 互斥矩阵 + tray 双回归钉全量走查

实测全仓 `//go:build` 文件共 10 个，互斥矩阵完备：

| 文件 | tag | 结论 |
|---|---|---|
| `tray_windows.go` | `windows` | Windows 活托盘 |
| `tray_linux.go` | `linux && cgo` | Linux 桌面托盘 |
| `tray_linux_cgo0.go` | `linux && !cgo` | Linux CGO=0 占位（与上互斥） |
| `tray_other.go` | `!windows && !linux` | darwin 等占位 |
| `tray_asset_windows_test.go` | `windows` | ICO 回归钉 |
| `tray_asset_linux_test.go` | `linux && cgo` | PNG 回归钉 |
| `browser_windows.go` | `windows` | Windows 开浏览器 |
| `browser_unix.go` | `!windows` | Linux/macOS 开浏览器（与上互补） |
| `native_ocr.go` | `windows && cgo` | 内嵌 ddddocr 实测路径 |
| `native_ocr_stub.go` | `!windows \|\| !cgo` | 回退存根（与上互斥） |

**R80 硬锚 4264 复核**：`tray_windows.go:96` `dwBytesInRes = len(ico)-headerSize = 4286-22 = 4264`；测试 `tray_asset_windows_test.go:27` 三加数独立推导 `40(BITMAPINFOHEADER) + 4096(32×32×4 像素) + 128(AND mask 32×4) = 4264` 一致——两处不同源计算同值，字段算错仍能红。`dwImageOffset` 两处均 22；BITMAPINFOHEADER 双高 64、像素 62 起 BGRA、中心 4x4 白其余黑不透明，逐像素断言与实现 ROI 匹配。`trayPNG` 经 `image/png.Decode`（严格 CRC）验证 16x16 中心 4x4 白块。`native_ocr.go` `dumpIfDiff` 按大小比对释出、官方 OCR 模式（ModelDir 固定名）与注释一致；`NativeDdddOcrAvailable` 以内嵌切片非空判定。

### 4. 契约 20 全仓扫描

- 生产代码（非测试）：`//go:build` 外零轮次前缀标签命中；唯一行号族引用为 `internal/zhidao/client.go:405/512/516/537/552` 的"manager.go:313""transfer.go:865""client.go:737/994""request.go:932-945"——均为标准库源码语义锚（client.go 与 transfer.go 是 Go 标准库 net/http 内部文件）与跨文件语义指位，非行号漂移风险，契约 20 许可。
- 测试代码：`scheduler_test.go:1794/1806/3174/3181/3484/3493/3498` 的 B29-02/B42-02/B43-02、`tray_asset_*.go` 的 R76/R77/R79、`handler_test.go:152` 的 O82-01——均为叙述性历史锚点，非"X-XX（第 N 轮）"前缀标签形态，许可；O84-03 记录边界。
- `XK-` 激活码样例文本、`connection reset` 等错误字符串为假阳性。
- **结论：零轮次前缀标签残留，符合契约 20。**

### 5. 生产逻辑契约抽核（新角度 10 项）

| 契约 | 结论 |
|---|---|
| DeleteRefusedClass 幂等单课删除 | 通过。store.go:176-179 `DELETE FROM refused WHERE account=? AND class_id=?` 只删单课、幂等（删除不存在的 99999 不报错）；`TestDeleteRefusedClassOnlyRemovesOneClass` 跨账号/跨课保留断言全绿 |
| StateForAccount window_closed 镜像 | 通过。scheduler.go:707 `st.WindowClosed = s.windowClosedLocked()` 与 WindowClosed() 同源（三条判据单源），主判据/时钟失败/幽灵窗口兜底全镜像；`TestWindowClosedState`/`TestGhostWindowFallback` 全绿 |
| LoadAllLogs 空库窗口 | 通过。store.go:417-439 `id > (SELECT max(id)-20000)` 空库 max(id)=NULL → 谓词恒 false → 0 行不报错；`TestLoadAllLogsEmptyDB` 断言空库返回 0 行全绿 |
| accounts gateWait 唤醒链路 | 通过。gateWait 阻塞 `gateCond.Wait()`（manager.go:62），GatePump 每 30s 广播唤醒（main.go:151-157）；gateWait 内部自带窗口翻页重置（manager.go:54-57），GatePump 空转幂等 |
| ReloginIfNeeded 幂等 | 通过。client.go:566-577 只用客户端内部 account/password（不受调用方参数影响），成功重登一次返回 (true, nil)；无保存账密明确报错 |
| maskKey 脱敏 | 通过。handler.go:704-712 空值返回空（=未配置）、≤4 位全掩码、否则 `****`+后 4 位；前端 Admin.tsx 只读 `vision_api_key_masked` 字段回显，PUT 留空不改 |
| handleLogout 吊销联动 | 通过。handler.go:576-584 requireAuth 后 `Sessions.Delete(tok)` 服务端立即吊销；App 端 logout/onUnauthorized/onDeleted/onBackToStudent 全路径清 `xk_admin_token` 标记（前轮已核，无回潮） |
| Register 会话绑定 | 通过。router.go 只建路由，会话绑定由 requireAuth 的 `Sessions.Account(tok)`（handler.go:1095-1103）决定；普通会话只操作绑定账号，管理员透传仅 `IsAdminToken` 会话可穿透（allowAccountOverride handler.go:1064-1068） |
| 删账号 memory-first 四步序 | 通过。handler.go:1004-1018 `Remove → PurgeAccount → DeleteAccount → RevokeAccount`；PurgeAccount 清 16 处 map key + Courses 行（scheduler.go:499-523）；`TestPurgeAccount` 断言 done/full/rateLimited/inflight 全清全绿 |
| sameClientFor 六分支闭合 | 通过。scheduler.go:1484(失效)/1516(成功)/1546(风控)/1566(窗口关闭)/1595(实时复核回锁)/1630(确证满员) 全部先指针身份比对再写状态；`TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull}` 五分支独立红绿 + `waitChainExit` 活跃标记等待（不依赖 inflight map）全绿 |

### 6. 附加复核

| 项 | 结论 |
|---|---|
| OpenTime 死配置彻底清除（B40-02） | 通过。config.go 结构体无 OpenTime 字段、不读 XUANKE_OPEN_TIME（config.go:55-57 注释载明）；`TestConfigDoesNotInjectOpenTime` 反射断言字段不存在 + t.Setenv 验证不读取；data/.env 模板无 XUANKE_OPEN_TIME 行 |
| 数据库增量迁移（契约规范） | 通过。db.go:28-35 `migrateAddPublishMeta` 在 refuseLegacy **之前**调用、逐列 columnExists 缺才 ALTER；refuseLegacy 缺列清单已剔除已迁移两列（db.go:79 注释）；`TestMigrateAddsPublishMetaColumns` 旧库真实行 → Open 成功 → 两列补全 + 旧行保留（db 包全量 race PASS） |
| 契约 17 零吞错落库点 | 通过。scheduler 全部落库点 `if err != nil { log.Printf }`；handler.go:363/431 的 `_ = d.Sched.MarkDone/RemoveDone` 返回值恒 nil（内部落库失败已记日志），非吞错 |
| 契约 2 窗口判据单源 | 通过。windowClosedLocked 三条判据（主判据带 +10s 裕量 / 时钟失败≥3 须带"开放时间已过" / 幽灵窗口 EmptyProbeRuns≥3 同 +10s 裕量），StateForAccount 与 WindowClosed() 共用；probe() 入账侧同快照复用 open（scheduler.go:1140-1153） |

## 契约抽查表

| 契约编号 | 内容 | 结果 |
|---|---|---|
| 1 | 开放时间唯一事实源 beginTimes 自动识别、识别槽不截断零值 | 通过（probe/ProbeForAccount 只在非空 BeginTimes 覆盖，空快照不删槽；openTimeForLocked 返回识别值本身） |
| 2 | WindowClosed 判据单源 windowClosedLocked | 通过（见上） |
| 3 | 关闭≠时间消失契约 | 通过（空快照不删识别槽；目标发布元数据持久化 + RestoreTargets enrich 兜底） |
| 4 | 删账号 memory-first | 通过（四步序 + PurgeAccount 全量） |
| 5 | 落库前锁内复核 ClientFor 防线族 | 通过（spawnChain 链顶/成功/失效/实时复核回锁；MarkDone/RemoveDone 手动路径同款） |
| 6 | 重启恢复顺序契约 | 通过（RestoreDone → RestoreTargets → LoadRefused+RestoreRefused；main.go:115-139 实测顺序与契约一致） |
| 7 | ?account= 凭据表校验 | 通过（课程读/目标写/手动报名退选/状态读四路全覆盖 accountExists） |
| 8 | ElectivesSnapshotFor 回退链 | 通过（有目标账号过期专属帧 → (nil,false) 绝不回退全局；无目标从未有专属帧才回退） |
| 17 | 落库失败必须记日志 | 通过（零吞错点） |
| 18 | 识别引擎热切换同步模板 | 通过（SetVision 保留当前引擎 + SetRecognizer 写入模板，manager.go:191-216） |
| 20 | 代码注释严禁轮次前缀标签 | 通过（全仓扫描零残留，仅叙述性历史锚点） |
| 31/36/37 | sameClientFor 全分支闭合 | 通过（六分支独立测试全绿） |
| 33 | doLogin 全入口全局频率闸门 | 通过（gateTryAcquire 非阻塞收口 LoginByPassword，与 gateWait 共享 gateMu/gateUsed；`TestGateTryAcquireBlocksWhenQuotaFull` 断言 doLogin 0 次全绿） |
| 42/43 | 连接活性自愈 / 撞名学生管理态 | 通过（httpDo 只重试 dial/write、read 不上抛；登录响应 adminName 标记管理令牌） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go build ./...` | 通过（EXIT 0） |
| `go vet ./...`（全仓） | 通过（EXIT 0，零输出） |
| `gofmt -l .`（全仓） | 零输出 |
| `go test -race -count=1 -p 1 ./internal/scheduler/` | PASS（15.2s） |
| `go test -race -count=1 -p 1 ./internal/api/` | PASS（27.4s） |
| `go test -race -count=1 -p 1 ./internal/scheduler/ ./internal/api/` 连跑 | 双 PASS |
| `go test -race ./internal/accounts/ ./internal/store/ ./internal/session/` | 三 PASS（store 35.9s） |
| `go test -race ./internal/db/` | PASS（2.0s） |
| `go test ./internal/zhidao/ ./internal/runtime/ ./internal/config/ ./internal/secure/ ./internal/db/` | 五 PASS |
| `TestWindowOpenSubmitsWithoutProbeReset` 等守卫族 `-count=10` 连跑 ×3 组 | 全绿（每组 33.6s，含 3 测试） |
| 删号竞态五分支族 + 窗口空发布族 `-count=2`（race） | 全绿 |
| api 四抖动测试族 `-count=5`（race） | 全绿（7.7s） |
| `TestAdminStatsWindowOpenedUsesScheduler`/`TestLogoutRevokesToken`/迁移族 `-count=2`（race） | 全绿 |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build .` | 通过（内嵌 ddddocr 路径） |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build .` | 通过 |
| `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build .` | 通过（Docker/无头托盘占位路径） |
| `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build .` | 通过（tray_other 占位） |
| `GOOS=linux CGO_ENABLED=1 go build` | 宿主缺 Linux C 头（grp.h），工具链环境限制非代码缺陷（见可疑待核） |

## 结论

R84 后端只读审查**零 CRITICAL、零 MAJOR、零 MINOR、3 条 OBSERVE**（均无代码改动要求）。核心结论：

1. **R83 夹具重构（4460dc3）四连核全部通过**：未来 open 语义下首段 pending 断言正确挂起（守卫 `!opened && !now.After(open)` 对未来 open 恒挂起）、resetProbe 后探测置 opened=true 走正常 1s 闸门、与零值守卫测试无交叉破坏、count=10 × 3 组连跑全绿——原 4~8% 抖动源已根除。实际修复方案（未来 open）优于 R83 报告建议（过去 -2m），后者与根因同源实为无效（记录 O84-01）。
2. **M83-02 api 抖动归因维持**：本轮全量 race 首跑及多轮连跑全部零 FAIL，无新证据推翻归因。
3. **build tag 10 文件互斥矩阵完备**、tray ICO/PNG 双回归钉与 R80 硬锚 4264/22 一致、四组合交叉编译全绿。
4. **契约 20 扫描零轮次前缀标签残留**；生产逻辑契约新角度抽核 10 项 + 契约表 15 项全部通过，含 DeleteRefusedClass 幂等、window_closed 镜像、LoadAllLogs 空库窗口、gateWait 唤醒、ReloginIfNeeded 幂等、maskKey 脱敏、handleLogout 吊销联动、Register 会话绑定。

后端整体健康状况良好，连续多轮零严重级发现。
