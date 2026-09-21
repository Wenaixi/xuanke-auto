# review-round68 总结（2026-09-21）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端连续**十四轮零 MAJOR 零 MINOR**（M-1 第五轮低成本核对闭合）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 3**——**生产逻辑连续六轮零 MINOR**，flake **栅栏三包一次性配平**（api/zhidao/accounts 全对齐 200ms×10+2s）。

**修复 6 处**（前端格式卫生三修 + 后端测试卫生三修）。

## 审查发现（写入 archive/review-rounds/round68-{backend,frontend}-findings.md）

### 后端（MINOR 0 / OBSERVE 3）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-68-01 | OBSERVE | accounts 包 readyProbe 栅栏仍窄（200ms×5 无超时），是 R67 收敛后残余面最低收敛点——本轮无样本被包序天然保护（store 高耗时后接 zhidao 而非 accounts） | ✅ 修复（对齐 api/zhidao 宽栅栏 200ms×10+2s，三包一次性配平） |
| OBSERVE-68-02 | OBSERVE | zhidao loginMockServer 注释残留"200ms×5"过时参数（实际已改 200ms×10） | ✅ 修复（一行注释成族核对） |
| OBSERVE-68-03 | OBSERVE | store 测试注释"批事务实测 ~0.5s"过时（-race 实测 33.37s） | ✅ 修复（补 -race 测量条件） |
| R67 复核 | — | ①readyProbe 宽栅栏正确（循环逻辑逐字段对齐 api、读 body 不管状态码语义未变、单独复跑曾 FAIL 测试 0.03s PASS）/ ②local_ocr 断言正确（本机实测 PASS；CI ubuntu 无 python 走 Skip 无误红） | ✅ 两条修复全部成立 |
| 契约抽查 | — | 窗口关闭三判据 / 删号 memory-first / sameClientFor 六分支 / doLogin 闸门 / doneHas 优先 / IsReadErr 全路径 / 关闭≠时间消失——7 条全成立 | ✅ |
| flake | — | **3 轮全量 + 2 轮隔离 30 次包运行零 FAIL**；zhidao 残余面**归零且未流动**（残余面暂歇非根除） | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 2）
连续十四轮零 MAJOR 零 MINOR。**M-1 第五轮低成本核对：闭合**（三消费点逐字符一致、置位路径无回潮、target-guard 18/18）。**OBSERVE-66-03 setSelected 清点：五调用点无新增无第三来源**、独立清理 effect 无改动。R67 两处格式修复核证：Dashboard 拆行**完全正确**（16sp 对齐、div 配对 52/52、全仓相邻 JSX 同行清零）；Select 容量块归一是**半程**（块头 30→28sp 已对齐但块内子元素残留 +2sp 偏移成 4sp 级差）。OBSERVE-68-01（容量块块内缩进残余 4sp 级差）**采纳修复**（整体 -2sp 归一 2sp 级差体系）；OBSERVE-68-02（Admin:839 td 18sp + Select:379 lastJson 24sp 两处历史错位）**采纳修复**。构建全绿。

## 修复（主控核实后 TDD 直修）
| 文件 | 内容 | 验证 |
|---|---|---|
| `backend/internal/accounts/manager_test.go` | readyProbe 200ms×5+http.Get → 200ms×10+显式 2s 超时+NewRequest 每次重建（三包配平终态） | accounts race 全绿 |
| `backend/internal/zhidao/client_test.go` | 调用点注释 200ms×5 → 200ms×10+2s（R67 修复遗漏的成族注释） | zhidao race 全绿 |
| `backend/internal/store/store_test.go` | 批事务量化值补 -race 测量条件（~0.5s → ~33s -race 下） | build 全绿 |
| `web/src/routes/Select.tsx` | 容量块整体 -2sp 归一（对照信息块级差体系）+ lastJson 缩进 24→20sp | build 全绿 936ms |
| `web/src/routes/Admin.tsx` | 操作列 td 18→20sp（同表对齐） | build 全绿 |

**核实方法**：OBSERVE-68-01 我对照 api（handler_test.go:184-215）/zhidao（client_test.go:82-113）宽栅栏与 accounts 旧实现（manager_test.go:23-43）逐行对比——accounts 的 `http.Get`+200ms×5（总窗口 ~1s 无超时）与 zhidao 修复前逐字同构，是包序变化后残余面流向的天然通道。OBSERVE-68-02/03 为注释值过时，grep "readyProbe"/"200ms" 成族核对后修正。

## 收尾全量回归
- `go build ./... && go vet ./...` → exit 0；`gofmt -l .` → 零输出
- 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` → 待跑（第一轮后台运行中）
- 前端 `npm run build` 全绿（936ms）；audit exit 0；target-guard 18/18

## 观察项延续（下轮复核）
后端：flake 残余面暂歇（「栅栏三包配平」终态后若再流动，宿主必是新增 z 开头包或包序变化）/ OBSERVE-63-04 probe 工具 / OBSERVE-63-05 stats 半真已自我注释化 / OBSERVE-62-06/07 / OBSERVE-61-03/04/06/07/08 / OBSERVE-66-01 登记防御；前端：M-1 延续管理 / OBSERVE-66-03 维持 / N-1~N-3 / O-1~O-12 / OBSERVE-68-01/02 已修。

## 教训
1. **「栅栏配平」要一次性扫清全包**：R67 只配平了 zhidao（当时的残余宿主），accounts 的 readyProbe 仍是同构窄栅栏（200ms×5）——本轮虽被包序天然保护（store 高耗时后接 zhidao 而非 accounts），但配平动作是「最低收敛点修补」而非「包间栅栏宽度差异根除」。下次任何包序变化/新增包都可能让残余面流动到 accounts。一次性把三个 mock 包（api/zhidao/accounts）全部对齐 200ms×10+2s 才是终态。
2. **注释更新要成族核对调用点**：R67 修改 readyProbe 函数时更新了函数自身注释，但漏了调用点 loginMockServer 上方的同款注释（仍写 200ms×5）——同一概念的注释分布在两处时，改动只同步一处是常见遗漏。grep 注释关键词（如"readyProbe""200ms"）核对全部出现点是复查手段。
3. **量化注释要标注测量条件**：store 测试「批事务实测 ~0.5s」与 -race 实测 33s 差 60 倍——量化值若不标注是否含 -race/平台，会随时间漂移成误导。注释里保留数量级语义即可，精确值最好带上测量上下文。
4. **前端半程修复要盘一遍整体**：R67 容量块「只对齐块头、未收内部」把连贯 2sp 体系搞成 4sp 级差——同一几何变换（整体 -2sp）必须整块应用，拆成两步是给自己留半残状态。逐行前导空格实测是发现此类残留的最快手段。