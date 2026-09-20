# review-round43 总结（2026-09-20）

## 概述
按主控协议走完整循环：两个 opus 只读审查代理（后端/前端）并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 派两个 sonnet 修复代理串行 TDD 修复 → 收尾全量回归。本轮发现后端 MAJOR 5 / MINOR 4 / OBSERVE 5，前端 MAJOR 1 / MINOR 3 / OBSERVE 4；确认修复后端 5 条（含 1 条实测归因降级）+ 前端 4 条。本轮特色：后端"身份防线决策侧裸奔"（maybeRelogin 入口无复核 + 实时复核失效分支缺指针比对）成族闭合；前端"全清空目标永不落库"（F42 纯数据判据与清空语义分叉）升级为 MAJOR 复证。

## 审查发现（写入 archive/review-rounds/round43-{backend,frontend}-findings.md）

### 后端 5 条（MAJOR 5 / MINOR 4 / OBSERVE 5，主表列 MAJOR）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| B43-01 | MAJOR | maybeRelogin 入口无 ClientFor 复核——探测定时三处（ProbeForAccount/ProbeNow/probe）直调可污染同名重建账号 tokenValid/reloginFail，B21-03 只护写回侧、决策侧裸奔 | ✅ 修复（f75ddff） |
| B43-02 | MAJOR | 实时复核 ErrUnauthorized 分支缺 sameClientFor 指针身份复核——身份防线最后一块裸露写点 | ✅ 修复（f6fd89e） |
| B43-03 | MAJOR | 成功分支身份复核失败时 inflight 位漏删——同名重建账号该课永久阻塞手动报名 | ⚠️ 实测归因（红不了，1502 行公共清位已存在；固化为契约回归 0812611） |
| B43-04 | MAJOR | handleLogin `Account==adminName` 单判——学生撞名必吃"管理口令错误"永无法登录（DoS） | ✅ 修复（f1d6792） |
| B43-05 | MINOR | handleAdminStats targetsCount 吞错静默计 0 | ✅ 修复（d30a562） |
| MINOR-43-01~04 | MINOR | 凭据加密失败静默 / 死字段 / 学生失败路径时延 / reloginBackoff 注释微偏 | ⚠️ 观察 |
| OBSERVE-43-01~05 | OBSERVE | 双槽分叉 / reloginResults 满丢弃 / 复核网络段重复 / tick 无 recover / 双判共存 | ⚠️ 观察延续 |

### 前端 8 条（MAJOR 1 / MINOR 3 / OBSERVE 4）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| F43-M1 | MAJOR | 全清空目标永不落库——shouldDeferSave 纯数据判据把"清空意图"误判为"回显未完成"，无自愈信号，返回后旧目标复活（round42 N-2 复证升级） | ✅ 修复（28116f3） |
| F43-N1 | MINOR | 报名/退选按钮绑 in_date_range\|\|window_opened，窗口信号缺失时手动抢课通道锁死（N-3 复证） | ✅ 修复（05e3797） |
| F43-N2 | MINOR | 激活失败票据滞留误导（N-4 复证）——核实代理确认"死循环"不成立（服务端单次硬保证 + 逾期守卫收敛），真问题仅误导文案 | ✅ 修复（43fbd4d） |
| F43-N3 | MINOR | Tailwind 4 未启用 animate 插件，全部动效类构建产物缺失（grep -c=0 实证，新发现） | ✅ 修复（b3e5cdd） |
| O-1~O-4 | OBSERVE | resetRetry 退避语义分叉 / useTickingCountdown 整帧重渲 / 模态无焦点陷阱 / App 401 保守窗口 | ⚠️ 观察延续 |

## 修复（TDD 严格模式，独立 commit，未 push）

### 后端 5 条（修复代理 ab69b2635e3b70222，六 commit）
| 缺陷 | commit | 测试形态 | 红→绿 |
|---|---|---|---|
| B43-01 | `f75ddff` | TestMaybeReloginDeletedAccountSkipsMaps | 四 map 残留 key → 全无 key |
| B43-02 | `f6fd89e` | TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin | relogCalls 1 → 0 |
| B43-03 | `0812611` | TestDeletedAccountRebuiltSameNameChainSuccessDropsInflight | 恒绿契约固化（实测红不了，1502 公共清位已存在） |
| B43-04 | `f1d6792` | TestLoginAdminNameCollisionStudentCredential（+ 两个既有断言按新契约更新） | 撞名学生被封 → 教务登录签发 |
| B43-05 | `d30a562` | TestAdminStatsTargetsLoadFailureReturns500（实现复查 + 正常路径回归） | 吞错计 0 → 报错 500 |

### 前端 4 条（修复代理 a435e1f9b624ea166，四 commit + 报告）
| 缺陷 | commit | 测试形态 |
|---|---|---|
| F43-M1 | `28116f3` | shouldDeferSave 增 hasSelected 六断言（全清空+首帧带旧目标→放行）红→绿 |
| F43-N1 | `05e3797` | 逻辑走查 + tsc + build（btn_type 唯一判据拆块） |
| F43-N2 | `43fbd4d` | 逻辑走查（票据生命周期短） |
| F43-N3 | `b3e5cdd` | 产物 grep animate-in 0→1 实证（先红后绿） |

报告 commit：前端 `e2fe319`、后端 `a5b6d13`（round43-{frontend,backend}-fix-report.md 落盘）。

## 新增决策锚（已沉淀进根 CLAUDE.md）
35. **守卫必须区分"用户意图"与"数据缺席"**（F43-M1）：`shouldDeferSave` 增 `hasSelected` 第二参数——显式全清空绝不与慢首帧混判，清空意图放行 PUT []。
36. **身份防线"决策侧"与"写回侧"必须双闭合**（B43-01）：maybeRelogin 入口补 `ClientFor` 存在性复核——凡是发起重登/提交/探测的决策点先复核账号仍存在，写回侧复核只是最后防线。
37. **identity 复核必须全分支清点**（B43-02）：实时复核失效分支补 `sameClientFor` 闭合六分支——grep 所有 err 归并 + 状态写入点，"上个分支修了"不等于"这条分支有"。
38. **管理员入口必须"账号+口令"双条件**（B43-04）：撞名学生放行教务登录，口令错才归"管理口令错误"。
39. **审查走读推断必须经 TDD 实测验证**（B43-03）：红不了的问题不是问题，但用契约回归测试把行为钉死防回归。

## 收尾全量回归
- `cd backend && go build ./... && go vet ./...` → exit 0（后台验证）
- `cd backend && go test -race -count=1 ./...` → 9 包全绿（scheduler 17.5s 含 race；zhidao 6s 无抖动）
- `cd web && npx tsc -p tsconfig.app.json --noEmit` → exit 0（前端代理已验证 build 绿）

## 观察项延续（下轮复核）
后端：MAJOR-43-03 双槽分叉 / reloginResults cap8 满丢弃 / classFullRealtime 网络段重复 / tick 提交分支无 recover / ClientFor+sameClientFor 双判冗余 / round42-43 其余延续；前端：M-2 降频覆盖 / O-1~O-4 / 上一轮 N 项观察延续