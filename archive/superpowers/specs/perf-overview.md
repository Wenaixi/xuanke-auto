# 性能优化专项总览（第 1.5 轮收官）

日期：2026-09-24 ~ 2026-09-25
方法论：**先立 P0 基线红绿灯 → 每优化独立 commit（commit 带前后数据）→ 复测对比 → 退步 revert**。
全部数据来自 `-benchmem -count=3` 同机实测；基准夹具带 shape 自检（`TestParseElectivesBenchShape` 断言 3 发布 82 门课），跨轮可复现。

## 已落地 10 项（每项独立 commit，可单条 `git revert`）

| # | commit | 模块 | 优化 | 数据（改前 → 改后） |
|---|--------|------|------|------------------------|
| 1 | `eca1d53` | zhidao | doRequest 响应体按 Content-Length 预分配一次到位 | **B/op 71730→39683（-44.7%）**·allocs 96→82（-14.6%），apples-to-apples 隔离对照 |
| 2 | `3bdeed0` | zhidao | readBody 统一收口（验证码图+登录响应同走预分配） | 三处 io.ReadAll 路径分叉消除，同合同实现 |
| 3 | `611c9de` | web | Select 页倒计时抽 memo 叶子 CountdownLeaf（延续 OBSERVE-116-01） | 每秒 tick 重渲染从 1254 行整树收敛到六数字叶子；守卫扩展红绿闭环 |
| 4 | `58d36c9` | web | 水墨背景图 JPEG q75 渐进式压缩 | **608.3KB→153.0KB（-74.6%）**；视觉零感知 |
| 5 | `49c87de` | web | Admin 各 Tab 轮询门控（4 查询 5s/10s → 仅当前 Tab） | 管理页停留期轮询 **-75% 带宽**；切 Tab 不丢缓存 |
| 6 | `349904f` | 全栈 | /electives 响应瘦身（Class 剔四零消费字段） | **响应体 29523→22307 字节（-24.4%）**·ParseElectives 613→451 allocs（-26.4% 同 payload 隔离） |
| 7 | `0e81afb` | web | 路由级懒加载（Select/Admin 拆独立 chunk） | **登录首屏 422.57→363.15kB（-14%）**；嵌入 exe 兼容（SpaHandler fs.Stat） |
| 8 | `d3eb54a` | web | lazy-route-guard 守卫锁 P-4 不回归 | 改回静态 import → 红灯实证 |
| 9 | `0b16cd1` | ci | npm run guard 一键五守卫接入 CI | 守卫红灯即 CI 失败 |
| 10 | `df3bb4c` | store | /admin/stats 日志计数改 COUNT（LoadAllLogs 只取 len 漏洞） | **COUNT 10.2µs/568B/18 allocs vs 1000 行 1058µs/551KB/13042 allocs → 耗时 -99.0% 等** |

## 裁定不动的（量化留档，YAGNI）

- **parseElectives Publishes 预分配**：3 发布规模 B/op 仅 -0.97%/allocs -0.33%（82 门课 Classes 分配淹没 97%）；**>50 发布时才值得上**
- **Cookie 头预计算**：160.5ns vs 271.5ns（-41%）但总量 ~270ns 微秒级，非热路径
- **StateForAccount Courses 索引化**：300 条过滤 2.2µs/8016B，/state 3s 轮询下微秒级，索引引入写链复杂度
- **acctData 每账号快照常驻**：12 字段瘦身后 17.5KB/账号，1000 账号才 17MB（对比 ddddocr 模型常驻 13MB onnx+16MB dll）
- **ListAdminAccounts N+1**：10 账号 21 条 SQL 实测 7.5-8.1µs（SQLite 内存库单查询 ~0.4µs），100 账号外推 ~80µs 可忽略
- **handleAdminConfig GET**：内存热配置 + 8 字段 struct，零 DB 读，无热点
- **前端主 bundle**：Select/Admin 拆走后 363kB 大头为 React vendor + TanStack Query + Radix 跨路由共享，不可再拆
- **路由 lazy 的 gzip 合计 +3.5%**：chunk 边界 vendor 元数据重复，换取首屏与并行加载双赢

## 撤返回退点

| 想回退哪项 | 命令 |
|-----------|------|
| readBody 预分配（1） | `git revert eca1d53` |
| readBody 收口（2） | `git revert 3bdeed0` |
| Select 叶子化（3） | `git revert 611c9de` |
| 背景图压缩（4） | `git revert 58d36c9` |
| Admin 门控（5） | `git revert 49c87de` |
| 响应瘦身（6） | `git revert 349904f` |
| 路由懒加载（7） | `git revert 0e81afb` |
| lazy 守卫（8） | `git revert d3eb54a` |
| CI 守卫（9） | `git revert 0b16cd1` |
| COUNT 优化（10） | `git revert df3bb4c` |

## 基准复现

```bash
cd backend && go test -run 'TestParseElectivesBenchShape' -bench 'BenchmarkDoRequest|BenchmarkParseElectives|BenchmarkRead' -benchmem -count=3 ./internal/zhidao/
cd backend && go test -run '^$' -bench 'BenchmarkCountAllLogs|BenchmarkLoadAllLogs' -benchmem ./internal/store/
cd web && node --import jiti/register scripts/lazy-route-guard.ts   # lazy 守卫
cd web && npm run guard   # 五守卫一键
```

## 性能记忆

详细决策与过程纪律（每轮候选、裁定依据、红绿灯方法论）见 `memory/perf-optimization-sprint.md`。