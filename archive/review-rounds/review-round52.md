# review-round52 总结（2026-09-20）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮后端 MAJOR 1（flake 本体/根因收敛三通道）/ OBSERVE 1（flake 伪装面）+ 延续观察 27+，前端 **MINOR 1（M-1 撞名学生死锁新发现）/ OBSERVE 4（O-1 闭合 + 延续）**。

**修复 4 处**（本轮最多实修轮）：前端 1（撞名学生锁死管理页）+ 后端 3（flake 根因三通道收敛）。总 commit 5 个（含 2 个 flake 修复）。

## 审查发现（写入 archive/review-rounds/round52-{backend,frontend}-findings.md）

### 后端（MAJOR 1 / OBSERVE 1 新 + 延续 27+）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MAJOR-52-01 | MAJOR（CI 基础设施延续） | httptest mock 连接 flake 本体仍在——失败断言面扩至 3 个新测试（612/979/1006），全部 connectex 同根，样本累至 6+ 断言行 | ✅ 本轮根治（F52-M2/M3/M4/M5/M6 三通道收敛） |
| OBSERVE-52-01 | OBSERVE | flake 伪装业务断言新样本面（TestAdminConfigHotReload 等三个确切断言行）——佐证登录链路首请求重试根治 | ✅ 同根一并收敛 |
| 新视角六项 | — | Start 幂等性 / handleLogout 吊销联动 / Restore×LoginByPassword 并发 / LoadCredentials 单账号失败 / zhidao 非 200 响应 / initCaptcha 幂等——全确认无问题 | ✅ 留档观察 |
| F51-O1 + 上轮实修 | — | F51-O1 前端删依赖对后端零影响；F50-M1/M2 双修闭合；F48-O3 / F46-O1 / B45/B43/B44 全部复核闭合 | ✅ |

### 前端（MAJOR 0 / MINOR 1 / OBSERVE 4）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| M-1【新发现】 | MINOR | 撞名学生教务登录后永锁管理页死胡同——后端 B43-04 放行撞名，前端渲染判据 current===adminName 未同步收口（五 Tab 全 403、403 不广播 401、登出重登复现 = 循环死锁） | ✅ 修复（F52-M1，commit 73abe92） |
| O-1 | OBSERVE | F51-O1 残件终裁执行复核——闭合（仅注释残留） | ✅ 闭合 |
| O-2~O-4 | OBSERVE | N-2/N-3 + O-3~O-6 + Dashboard key 不对称 + ui 模板残宽（CardFooter 零消费 + Button/Badge 死变体）——全维持原判 | ⚠️ 延续 |

## 修复（主控核实后直修 + TDD）
| 编号 | commit | 内容 |
|---|---|---|
| F52-M1 | `73abe92` | 前端：撞名学生永锁管理页——管理态判定改绑会话令牌（新增 adminToken 态 + adminAuth 纯函数；登录响应带 adminName 才标记管理 token，刷新恢复判据 = 会话 token===标记；loadSessions 快照判据同源；logout/onDeleted/onUnauthorized/onBackToStudent 全路径清标记）。TDD：admin-auth-check.ts 纯函数断言先红 2 项后绿 |
| F52-M2 | `54d0099` | fetchLoginPage 网络瞬时失败重试一次（纯 GET /login 不消耗验证码限额，不违背"失败即返回"契约）。TDD：TestLoginRetriesTransientInitError 先红后绿 |
| F52-M3 | `54d0099` | api 夹具套接字预创建（newTestDepsModeName 前手动 listen+close 排空 TIME_WAIT 冷启动） |
| F52-M4 | `0f23910` | httpDo 活性自愈（doRequest 连接层错误 dial/read/write 重试一次，业务取消原样上抛） |
| F52-M5 | `0f23910` | captcha/go:160 Vision 识别请求走同一 httpDo 自愈 |
| F52-M6 | `0f23910` | zhidao socketPreheat 抽包级 helper + TestRecognizeCaptcha/TestCaptchaConcurrency 补预加热 |

## 收尾全量回归
- `cd backend && go build ./... && go vet ./...` → exit 0
- `go test -race -count=1 -p 1 ./...` → **全包全绿**（首轮全量回归曾 FAIL 2：zhidao flake 自身 + TestExitClass，经 M4/M5/M6 根因收敛后终局全绿）
- zhidao 隔离连跑统计（修复演进）：修复前 10 轮 3 FAIL（30%）→ 预加热+httpDo 30 轮 2 FAIL（7%）→ M4/M5/M6 后 30 轮 2 FAIL + 10 轮详情 0 Read 中断样本 + 最终 10 轮 0 FAIL
- `cd web && node --import jiti/register scripts/admin-auth-check.ts` 全绿 + `npx tsc -p tsconfig.app.json --noEmit` exit 0 + `npm run build` 成功（417.05 kB js / 41.04 kB css）

## 决策锚沉淀（CLAUDE.md 追加第 40 条）
- **连接活性自愈族（httpDo）**：Windows 回环 keep-alive 池连接保持期内服务端静默关闭（仅对端知道），发送端复用写出 → connectex / read tcp 中断 / 403 / 429 四形态同根。自愈只覆盖连接层（dial/read/write）错误一次，绝不含业务重试。
- **首请求失败重试 + 夹具套接字预创建**：测试与产品双侧根因收敛——产品层 fetchLoginPage/httpDo 网络瞬时抖动自愈一次；夹具层预创建-关闭一个 127.0.0.1 套接字排空冷启动窗口。CI `||` 重跑仅吸收低频残余。

## 观察项延续（下轮复核）
后端：task_log 无清理（超 50 轮）/ tick 无 recover + 优雅退出 / 孤儿登录 / 双槽分叉 / reloginResults 满丢弃 / failingTargetsStore 无消费点 / 429 头不消费 / XUANKE_PORT 校验 / flake 残余（M4/M5/M6 后 R53 复核收敛是否彻底）；前端：N-2~N-3 / O-2~O-5 / Dashboard key 不对称 / ui 模板残宽 / Admin 轮询常跑 / focus 陷阱。