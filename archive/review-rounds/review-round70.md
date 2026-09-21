# review-round70 总结（2026-09-21）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端连续**十六轮零 MAJOR 零 MINOR**（M-1 第七轮低成本核对闭合）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 6 / OBSERVE 3**——**生产逻辑连续八轮零 MINOR**，flake 残余宿主重定位（串行全量形态首次出现流动样本）。

**修复 3 处**（前端注释归一 + 后端探活成族闭环）。

## 审查发现（写入 archive/review-rounds/round70-{backend,frontend}-findings.md）

### 后端（MINOR 6 / OBSERVE 3）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MINOR-70-01 | MINOR | api readyProbe 2s 单次超时在高负载/并发下覆盖不了 connectex 窗口（R7/R9 两样本） | ⚠️ 记档低优（与 OBSERVE-70-01/02 归入宿主家族，CI -p 1 收口） |
| MINOR-70-02 | MINOR | TestRecognizeCaptcha/TestLoginNetworkErrorAbortsImmediately 是 zhidao 包内仅剩的无 readyProbe 裸 mock 首请求宿主（R1 FAIL 正是前者） | ✅ 修复（两处补 readyProbe + 探活路径豁免业务头断言） |
| MINOR-70-03 | MINOR | bench 工具裸 http.Client 无 httpDo 自愈，瞬断即整体终止 | ⚠️ 不修（工具面观察） |
| MINOR-70-04 | MINOR | logintest 输出 token 前 8 位与 maskedToken 契约一致 | ⚠️ 不修（非缺陷） |
| MINOR-70-05 | MINOR | scheduler 顶层 openTime 字段生产恒零值、仅测试兼容，注释已载明 | ⚠️ 不修（删除牵动测试构造点低价值） |
| MINOR-70-06 | MINOR | admin DELETE 无 requireJSONBody——管理会话 token 在 localStorage 跨站表单无法伪造 Bearer，CSRF 天然不可达（与 OBSERVE-69-05 同族） | ⚠️ 不修（记档） |
| OBSERVE-70-01/02/03 | OBSERVE | R7 串行全量首现 api 冷启动 FAIL / R9 api+zhidao 并行 readyProbe 11 次全败（OBSERVE-69-04 宿主家族复现）/ store 批插 R10 达 180s 极值 | ⚠️ 续（记档，CI -p 1） |
| R69 复核 | — | TestCaptchaConcurrency readyProbe 双保险正确（探活与识别互不干扰、独立复跑 5 轮+count=10 全绿归零）/ 注释锚点 973 闭合 / tick 零值守卫双测试绿 | ✅ 全部闭合 |
| 契约抽查 | — | 窗口关闭三判据 / 删号 memory-first / sameClientFor 六分支+maybeRelogin 双闭合 / IsReadErr 全路径 / doLogin 闸门 / 开放时间唯一事实源——6 条全成立 | ✅ |
| flake | — | **十轮全量 -p 1 8 绿 2 红（20% 红率，均 connectex 冷启动）——串行形态首现流动样本**（R1 zhidao TestRecognizeCaptcha / R7 api TestAdminStatsAccountsLogs）；宿主重定位=包内无探活 mock 首请求 + store 前序高耗时 | ⚠️ 重定位 |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 3）
连续十六轮零 MAJOR 零 MINOR。**M-1 第七轮低成本核对闭合** + **OBSERVE-66-03 setSelected 五调用点无新增无第三来源**。R69 四处卫生修复逐行核证全部通过（Admin 块内 22/24/22/20 级差一致 / Dashboard :519 注释已改 / client 401 缩进归体系 / 搜索框 aria-label 全站无缺口）。OBSERVE-70-01（Dashboard:529 TARGETS 说明注释仍含「3 门」字样）**采纳修复**（归一为一句并指向上方详情，与 70-02 合并处理）；OBSERVE-70-02（:482-483 与 :529 注释重叠）**采纳修复**；OBSERVE-70-03（搜索框 placeholder 与 aria-label 同串重复 DRY 摩擦）**不修**（抽 hook 属过度工程）。构建全绿。

## 修复（主控核实后 TDD 直修）
| 文件 | 内容 | 验证 |
|---|---|---|
| `backend/internal/zhidao/captcha_test.go` | TestRecognizeCaptcha 补 readyProbe + 探活路径豁免业务头断言（Authorization 断言只针对 /chat/completions） | 两测试 + 全包 race 全绿 |
| `backend/internal/zhidao/client_test.go` | TestLoginNetworkErrorAbortsImmediately 补 readyProbe | 两测试全绿 |
| `web/src/routes/Dashboard.tsx` | :529 注释归一「TARGETS = 动态目标集合（旧 3 门约束详见上方注释）」 | build 全绿 566ms |

**核实方法**：MINOR-70-02 我先按代理建议给两测试补 readyProbe——**TDD 过程抓到连带问题**：探活请求命中 mock 的 Authorization 断言（探活 GET /login 无业务头被误报）→ 把断言改成仅对 `/chat/completions` 业务路径生效（探活路径豁免），修复后全绿。这正是「补探活夹具时 mock 断言要同步豁免探活路径」的教训实证。

## 收尾全量回归
- `go build ./... && go vet ./internal/... ./cmd/...` → 全绿；`gofmt -l .` → 零输出
- 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` → 第一轮全绿（store 77.2s 高耗时）+ 第二轮待跑
- 前端 `npm run build` 全绿（566ms）；audit exit 0；target-guard 18/18

## 观察项延续（下轮复核）
后端：flake 残余宿主重定位（串行全量形态首现流动——包内无探活 mock 首请求 + store 前序高耗时；CI -p 1 非绝对绿保证、2/10 概率级）/ MINOR-70-01 api 探活超时 / MINOR-70-03 bench 工具 / OBSERVE-70-03 store 批插 180s 极值 / OBSERVE-69-04 宿主家族 / OBSERVE-66-01 登记防御；前端：M-1 延续管理 / OBSERVE-66-03 维持 / N-1~N-3 / O-1~O-12 / OBSERVE-70-01~03 已修。

## 教训
1. **补探活夹具时 mock 断言要同步豁免探活路径**：TestRecognizeCaptcha 的 mock 原对「任何请求」断言 Authorization——补 readyProbe 后探活 GET /login 无业务头被误报 FAIL。凡给「带业务头断言/业务计数」的 mock 补探活，必须把断言/计数限定到业务路径（如 /chat/completions），探活路径豁免。
2. **「残余面消失」再次被证伪——串行全量形态首现流动**：R69 之前全量 -p 1 连续 9 轮零 FAIL，本轮十轮 8 绿 2 红（20% 红率）——残余宿主从「TestCaptchaConcurrency 并发首请求」重定位到「包内无探活 mock 首请求 + store 前序高耗时拉伸窗口」。每个新 flake 样本都是对「残余宿主判定」的再定位，收敛是持续过程。
3. **夹具探活成族要全量清点包内 mock**：R70 修复前 zhidao 包仍有 2 个裸 httptest 无探活（TestRecognizeCaptcha/TestLoginNetworkErrorAbortsImmediately）——「TestCaptchaConcurrency 修了」不等于「包内其他 mock 首请求安全」，grep httptest.NewServer 清点全包是复查手段。
4. **CSRF 判定要追「会话凭证存放形态」**：admin DELETE 无 JSON 门、logout 无 JSON 门——但管理会话 token 在 localStorage 不随 cookie 自动携带，跨站表单无法伪造 Bearer → CSRF 威胁面天然不存在。凡「无 JSON 门」端点先问「凭证在 cookie 还是 localStorage/header」，再决定是否算 CSRF 缺口。