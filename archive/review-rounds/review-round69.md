# review-round69 总结（2026-09-21）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端连续**十五轮零 MAJOR 零 MINOR**（M-1 第六轮低成本核对闭合）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 2 / OBSERVE 3**——**生产逻辑连续七轮零 MINOR**，栅栏三包配平终态在全量 `-p 1` 形态下维持成立。

**修复 5 处**（前端四卫生 + 后端注释锚点一修）。

## 审查发现（写入 archive/review-rounds/round69-{backend,frontend}-findings.md）

### 后端（MINOR 2 / OBSERVE 3）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MINOR-69-01 | MINOR | scheduler.go:980 注释引用的「830 行前 open 快照」行号过期（实际 tick 取 open 在 973 行）——语义正确仅锚点陈旧 | ✅ 修复（830→973） |
| MINOR-69-02 | MINOR | accounts readyProbe 与 api 版存在路径(/login vs /ready)与读体判定的微小账差——几何一致注释未明示差异 | ⚠️ 不修（记档，各家 mock 语义固化） |
| OBSERVE-69-03 | OBSERVE | Deps.Decrypt 自 B10-08 起未消费仍传参（显式备用契约非死代码） | ⚠️ 不修（如整理 API 签名可移除） |
| OBSERVE-69-04 | OBSERVE | **api 包多包并行复跑暴露 3 例 connectex/await-timeout 冷启动残余**——全量 -p 1 3 轮 + api 单包 -p 1 3 轮全绿，残余面宿主=非 -p 1 的多包并行形态 | ⚠️ 观察（CI 固定 -p 1 收口） |
| OBSERVE-69-05 | OBSERVE | /api/logout 未挂 requireJSONBody/限流——复核认定不适用（仅吊销自身会话、requireAuth 已护） | ⚠️ 不修（记档） |
| R68 复核 | — | ①accounts readyProbe 宽栅栏与 api/zhidao 逐字段对齐（三调用点全接上）/ ②zhidao:47 注释已同步 / ③store 批插量化 ~33s 与实测吻合 | ✅ 三处全部闭合 |
| 契约抽查 | — | 开放时间唯一事实源 / 窗口关闭三判据 / 删号 memory-first / 撞名管理态 / doLogin 闸门 / sameClientFor 六分支——6 条全成立 | ✅ |
| flake | — | **全量 -p 1 3 轮全绿**（store 最慢 36.6~71.6s）；隔离 zhidao/accounts -count=3 全绿；**api+scheduler 并行 -count=2 暴露 api 3 例冷启动残余**——栅栏终态维持、残余面暂歇非根除 | ⚠️ 定位新宿主 |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 4）
连续十五轮零 MAJOR 零 MINOR。**M-1 第六轮低成本核对闭合** + **OBSERVE-66-03 setSelected 清点无新增无第三来源**。R68 三处格式修复逐字核证全部到位（容量块 2sp 级差体系与参照信息块完全一致、lastJson/Admin td 归一、`</div>` 相邻 JSX 持续清零）。OBSERVE-69-01（Admin 操作列 td 块内残留——R68 只修 td 开行、块内 div/td 闭合仍低 2sp 的同类半程态）**采纳修复**；OBSERVE-69-02（Dashboard:519 注释残留「3 门重点看护」F10-06 只清了展示文案）**采纳修复**；OBSERVE-69-03（client.ts:86-94 body 层 401 分支缩进 6sp）**采纳修复**；OBSERVE-69-04（Select 搜索框缺 aria-label 与全站可达性基线不齐）**采纳修复**。构建全绿。

## 修复（主控核实后 TDD 直修）
| 文件 | 内容 | 验证 |
|---|---|---|
| `backend/internal/scheduler/scheduler.go` | 注释锚点 830→973（tick 实际取 open 行号对齐） | build+vet+gofmt 全绿 |
| `web/src/routes/Admin.tsx` | 操作列 td 块内 div 840/859 22sp、`</td>` 860 20sp（对齐同表 20/22/20 体系） | build 全绿 |
| `web/src/routes/Dashboard.tsx` | :519 注释「3 门重点看护」→「重点看护课程卡片」（F10-06 注释残留清旧） | build 全绿 |
| `web/src/api/client.ts` | body 层 401 分支整块右移 2sp 归 6/8/10 体系 | build 全绿 |
| `web/src/routes/Select.tsx` | 搜索框补 `aria-label="搜索课程名称、教师或教室"` | build 全绿 |

**核实方法**：MINOR-69-01 我用 grep 定位 tick/probe 的 open 采样行——tick 实际在 973 行取 open（`s.openTimeForLocked("")`），注释引用的「830 行前」确已过时；语义（单快照复用）正确，仅锚点修正。OBSERVE-69-04 我核实了 api 并行复跑 3 例失败的形态（TestLogoutRevokesToken await-headers 超时 / TestLoginOKIssuesSession readyProbe connectex / TestElectiveSelectRejectsFullClass connectex）——全量 -p 1 3 轮 + api 单包 -p 1 3 轮全绿证明残余面宿主是「非 -p 1 多包并行」形态，Windows 回环冷启动 + CPU 争用，CI 固定 -p 1 即收口。

## 收尾全量回归
- `go build ./... && go vet ./internal/... ./cmd/...` → 全绿；`gofmt -l .` → 零输出
- 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` → 待跑（第一轮后台运行中）
- 前端 `npm run build` 全绿（661ms）；audit exit 0；target-guard 18/18

## 观察项延续（下轮复核）
后端：flake 残余面宿主更新（栅栏终态下全量 -p 1 不可见、非 -p 1 多包并行可见——CI 固定 -p 1 收口）/ MINOR-69-02 记档 / OBSERVE-69-03 Decrypt 停用字段 / OBSERVE-69-04 观察 / OBSERVE-69-05 登出 CSRF 记档 / OBSERVE-63-04 probe 工具 / OBSERVE-63-05 stats 半真 / OBSERVE-66-01 登记防御；前端：M-1 延续管理 / OBSERVE-66-03 维持 / N-1~N-3 / O-1~O-12 / OBSERVE-69-01~04 已修。

## 教训
1. **注释行号锚点是易腐资产**：MINOR-69-01 的「830 行前」引用在代码演进中失锚——凡注释引用具体行号（「N 行前/上方」），代码移动时必须同步更新，否则维护者按错误锚点定位（grep 定位 open 采样行即可发现）。
2. **残余面宿主随运行形态变化**：栅栏三包配平解决了「包序最末 mock 包」形态（全量 -p 1），但非 -p 1 的多包并行形态下 Windows 回环冷启动 + CPU 争用让残余面重新可见（api 3 例）——「残余面不可见」必须在声明前明确其可见性前提（运行形态），CI 固定 -p 1 是锁死形态的收口手段。
3. **「只对齐块头未收内部」半程态成规律性复发**：R67 容量块 → R68 容量块内部 → R69 Admin td 块内，连续三轮同类「几何平移只做了块头」——凡格式平移（整体 ±Nsp）必须整块原子应用，拆分两步是复发温床。逐行前导空格实测 + git diff 核对是发现半程态的可靠手段。
4. **无障碍基线要逐控件核**：OBSERVE-69-04 搜索框缺 aria-label 是「placeholder 非 label」的经典可达性缺口——读屏用户无法得知输入用途。全站可达性基线（Login/Admin 输入框均有 label）下任何新增输入控件都要对照核。