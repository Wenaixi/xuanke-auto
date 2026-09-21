# review-round63 总结（2026-09-21）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端**连续九轮零 MAJOR 后首现 MINOR**（M-1 稳态编辑闷死）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 1 / OBSERVE 5**——产品逻辑连续七轮零 CRITICAL、零 MAJOR。

**修复 2 处**（前端 shouldDeferSave 增"回显已完成"维度 + 后端 cmd/probe 连接池契约族对齐）。

## 审查发现（写入 archive/review-rounds/round63-{backend,frontend}-findings.md）

### 后端（MINOR 1 / OBSERVE 5）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MINOR-63-01 | MINOR | cmd/probe 用 http.DefaultClient（Timeout=0 + 默认 2 连接池）绕过共享连接池——R52 连接活性自愈族遗漏面，真实平台 RST/FIN 时工具挂起至内核超时（分钟级） | ✅ 修复（zhidao 导出 SharedTransport + probe 改用 15s 超时共享连接池） |
| OBSERVE-63-01~05 | OBSERVE | flake 18/21（ReadErr 27s + UnauthorizedRelogin 74s 全量轮 FAIL，隔离恒绿）/ mock 替换漏识别分支 / api 包最脆弱 / probe 工具不可用 / stats 半真测试 | ⚠️ 延续 |
| 新视角 A-D | — | A R62 两修完全正确（标准库逐段实证 + 独立程序实测 + Do 层/ReadAll 层两路径形态区分）/ B 18/21 / C gofmt 零输出 / D 全包通读零新缺陷 | ✅ |

### 前端（MAJOR 0 / MINOR 1（新增 M-1）/ OBSERVE 13）
连续九轮零 MAJOR。**M-1【新增 MINOR】`shouldDeferSave` 在「已回显账号再次编辑」稳态下恒推迟 + 无解锁**——/state 轮询永驻携带后端旧目标（StateForAccount 按账号全量下发 courses、SetTargetsForAccount→rebuildCourses 只要还有目标就非空）→ `courses 非空 && 有选中` 恒真 → 防抖置脏后 selected 无变化被 bailout → effect 不重入 → 无任何解锁信号。编辑永不落库，返回后下次进页回显合并后端旧目标 = 编辑被静默撤销。**连续九轮报告全部只论证「首帧回显完成前」暂态，稳态盲区本轮首次补齐**。无数据丢失（后端始终旧目标）判 MINOR。审查报告教训：走读盲区不在代码本身，而在"把同一判据的两个语义阶段当成一个"。

## 修复（主控核实后 TDD 直修）
| 文件 | 内容 | 验证 |
|---|---|---|
| `web/src/lib/targetGuard.ts` | shouldDeferSave 增第三参数 echoed——undefined 无条件推迟保留；echoed=true 放行（selected 已含后端旧目标、整包 PUT 与后端一致）；仅 echoed=false 才按"courses 非空 && 有选中"推迟 | tsc 绿 |
| `web/src/routes/Select.tsx` | 三消费点（防抖 676 / flush 499 / handleBack 589 + while 595）全部传入 echoedRef.current——已回显稳定态编辑不再闷死、未回显仍等 5s 兜底；守住 F42-M1 初衷（/state 持续失败自愈语义不受破坏） | tsc 绿 |
| `web/scripts/target-guard-check.ts` | 补两条断言（稳态放行 / 首帧仍推迟）先红后绿 | TDD 断言全绿（18+6+5）+ build 绿 |
| `zhidao/client.go` + `cmd/probe/main.go` | SharedTransport() 导出访问器 + probe 改 `&http.Client{Timeout:15s, Transport: zhidao.SharedTransport()}` | build+vet 绿 |

## 收尾全量回归
- `go build ./... && go vet ./...` → exit 0；`gofmt -l .` → 零输出
- 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` → **全包全绿**（10 包 0 FAIL，api 55.7s / store 42.5s / scheduler 20.6s）
- 前端 tsc + 3 断言脚本（18+6+5）+ npm run build 全绿

## 观察项延续（下轮复核）
后端：flake 18/21（OBSERVE-63-01~03，api 包 mock 冷启动残余 + mock 替换漏识别分支）/ probe 工具（已修连接池）/ stats 半真测试 + 历轮延续；前端：M-1（已闭合闭环——下轮复核稳态编辑真正落库）/ N-1~N-3 / O-1~O-10 / Dashboard key 不对称 / ui 模板残宽 / NaN 防御 / 多标签页。

## 教训
1. **「安全方向」论证必须区分暂态与稳态**：F42/F43 的「courses 非空 → 推迟」判据在「首帧回显完成前」是安全方向，但在「回显完成后 courses 永驻」稳态下变成永久推迟且无解锁信号。连续九轮审查全部聚焦暂态推演，无人追问稳态——审查走读的盲区不在代码本身，而在"把同一判据的两个语义阶段当成一个"。**设计纯数据守卫时必须验证"重跑后判据能否翻转"（自愈信号可达性）**。
2. **标准库错误包装链的"两路径"形态差异必须区分**：`http.Client.Do` 阶段错误经 url.Error 包装，而 `io.ReadAll(resp.Body)` 阶段错误直接裸上抛——R62 注释"Do 层包 url.Error"在手动 ReadAll 路径下不成立（`errors.Is` 两形态都命中，功能正确但注释契约需写清两路径）。
3. **测试 mock 替换 handler 时必须把替换后仍可达的路径全量实现**：UnauthorizedRelogin 替换后漏 /chat/completions 分支，重登链路 3 连识别失败断言超时。
4. **flake 趋势第 9 轮**：R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → R61 1/16 → R62 17/21 → R63 18/21——冷启动残余未根除但被 readyProbe/socketPreheat 持续压低；CI `||` 重跑吸收残余仍是正确姿势。