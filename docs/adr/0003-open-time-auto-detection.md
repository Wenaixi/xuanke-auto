# 0003 开窗时间由平台 beginTimes 自动识别，不可配置

## Status

已接受。

## Context

本工具必须知道报名窗口何时开启，才能把提交压在窗口刚开的那一瞬。早期版本用一个配置项（以及配套的 `XUANKE_OPEN_TIME` 环境变量）指定开窗时间，还带一个硬编码的默认日期；这个字段在代码里没有消费点，却让运维以为「改了配置就改了开窗时间」，把过期日期当成开窗点写进配置。而平台每次探测都会下发 `beginTimes`，真值一直在手边。

## Decision

开窗时间的唯一事实源是平台响应里的 `beginTimes`，由本工具在探测时自动识别，**不提供任何配置入口**——配置结构体不再有开窗时间字段，配置层既不注入也不读 `XUANKE_OPEN_TIME`，启动时以零值构造调度器；管理员后台可热改的配置表里同样没有这一项。识别结果分两级存放：教务账号自己识别到的槽与全校共享槽，账号槽优先。

## Consequences

- 识别到之前，界面只能显示「未识别到开窗时间」，绝不猜一个时间。因此至少要有一次成功的平台探测，倒计时才有值；完全离线时本工具不会编造开窗时刻。
- 识别值过期只影响展示（标记为「未识别」），**识别槽本身不删**——窗口关闭不等于时间消失，否则窗口重开或批次热更时会把已知事实退回未知。空快照只在下发非空 `beginTimes` 时才覆盖识别槽。
- 回归由测试钉死：配置层禁用该项由 `TestConfigDoesNotInjectOpenTime` 守护，识别入账与关闭后保留分别由账号级、全校级探测测试覆盖。
- 代价是开窗时间无法离线预置，必须联网探测一次；平台若改变 `beginTimes` 的层级或语义，识别会整体失效，且没有配置项可以临时顶上——这是刻意的，宁可显式「未识别」，也不要一个会误导人的过期值。
- **核对的源文件**：`backend/internal/config/config.go`（`:64-66` 写明「开放时间唯一事实源 = 平台 beginTimes 自动识别，配置层不再注入，也不读取 XUANKE_OPEN_TIME」；`:32-54` 的 `Config` 已无该字段）；`backend/internal/runtime/config.go`（`:85-180` 的可热改配置表内无开窗时间项）；`backend/server.go`（`:113-115` 以零值构造调度器）；`backend/internal/scheduler/window_state.go`（`:78-100` 识别槽的两级读写；`:112-116` 的清理）；`backend/internal/scheduler/scheduler.go`（`:862-865` 账号级识别入账；`:1095-1100` 全校级识别入账；`:711-723` 识别过期只降展示）；测试锚点 `backend/internal/config/env_test.go`（`:29-35`）、`backend/internal/scheduler/open_detect_test.go`、`backend/internal/scheduler/open_retain_test.go`。
