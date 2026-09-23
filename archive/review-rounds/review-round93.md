# review-round93 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 2**；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 1**（连续第三十九轮零严重级）。核心：**F93-01 无障碍焦点可见性实质修复 + O93-01 注释澄清**。

## 审查发现（写入 archive/review-rounds/round93-{backend,frontend}-findings.md）

### 后端（OBSERVE 2）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| O93-01 | OBSERVE | probeIntervalFor（scheduler.go:83）为仅测试锚定的生产死方法——tick 直接调 probeIntervalForOpen(:991)、probeIntervalFor 内部自行重取 open 单快照（与 tick 单快照复用语义不一致），13 处测试锚定、零生产调用 | ✅ **采纳注释澄清**（1dfd116）：补「仅测试锚定的便利包装」注释（生产 tick 刻意传单快照、保留测试价值），零行为变更 |
| O93-02 | OBSERVE | 内嵌 ddddocr 资产体积记录——onnxruntime.dll 16MB + onnx 13.6MB + charsets 56KB ≈ 29.7MB，Windows CGO=1 单文件交付的既定代价 | ⚠️ 记录（非缺陷仅体量记录，历轮已述单二进制交付取舍） |
| 身份防线矩阵延续 | — | 16 项矩阵逐点复核（spawnChain 六分支 + 第七分支 + maybeRelogin 双侧 + ProbeForAccount + 手动两路 + SubmitAll + Restore）无新裸露写点 | ✅ 闭合 |
| B88-01 持续复核 | — | probe_identity_test.go 三钉 + 八轮 race 回归全绿 | ✅ 闭环 |
| O86-01（延续） | OBSERVE | api 抖动**第八轮零复现**——race 全绿（11 包 exit 0）connectex 零样本 | ⚠️ 维持基线 |
| O92-01 复核 | — | allow_swap 注释澄清（ed6e237）git 溯源确认语义精确、零行为变更、db 测试全绿 | ✅ 复核通过 |
| O92-02 | — | logintest 引擎分叉维持低优先级不修（本轮复核关闭） | ⚠️ 维持 |
| M87-01 | — | Shutdown 窗口维持 MINOR + 注释兜底 | ✅ 维持 |

### 前端（OBSERVE 1 / M-1 第二十九轮闭合）
- **OBSERVE-93-01（新，无障碍焦点可见性）**：全站 Button 组件无键盘焦点环——global.css:174 对 button 统一 `outline:none` 抹掉原生 focus，Button.tsx 全文件 focus 关键字 0 命中零补偿（历轮 round91「Button 继承原生 button focus」口径与 global.css 直接冲突，系历轮无障碍横查盲区）。✅ **采纳修复（F93-01，f08937e）**：base class 一行补 `focus-visible:ring-2`（Tab 键盘导航焦点环可见、鼠标点击不显示、全站收敛），与 Admin 开关/Login 密码切换自带 focus-visible 一致。
- **M-1 第二十九轮闭合** + 六防保存链零回归 + F92-01 剥离复核通过（git show 623c1ce 仅 5 处注释内锚点剥离零行为 diff、全仓轮次锚点族两形态零残留）。
- **OBSERVE-92-01 闭环**（623c1ce 落地）从延续清单移除。
- OBSERVE-90-01/88-01/85-02/84-01/83-01/77-02/76 族/O-3 族延续。

## 核实记录（关键）
- **F93-01**：Grep Button.tsx focus 零命中 + Read global.css:168-176（button 统一 outline:none + background transparent）确认审查论断成立——global.css 的重置确实抹掉原生 focus、Button 组件零补偿。补 focus-visible ring 后 tsc/build/三组断言/audit 全绿（CSS 41.72 kB 增 0.61 kB 即 ring 样式）。
- **O93-01**：Grep probeIntervalFor 生产调用零命中（tick 直接调 probeIntervalForOpen:991）、probeIntervalFor:83-87 仅包装自取 open——与审查一致；「仅测试锚定」注释 + 保留测试价值。build/vet/TestProbeInterval 全绿。

## 收尾全量回归
- 后端 `go test -race -count=1 -p 1 -timeout 900s ./...` → 全 11 包全绿（后台实证）
- `go build ./...` / `go vet ./...` 零输出 + Windows CGO1 / Linux CGO0 双平台交叉编译通过
- 前端 `tsc -b` EXIT 0 + `npm run build` 全量成功 + target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 + audit.mjs 77 项

## 提交链
- `f08937e` fix(web): F93-01 Button 组件补键盘焦点可见性（OBSERVE-93-01）
- `1dfd116` docs(backend): O93-01 probeIntervalFor 注释澄清

## 观察项延续（下轮复核）
后端：身份防线矩阵 / O93-02 资产体积记录 / O86-01 抖动基线（连续八轮零复现）/ M87-01 窗口留档；前端：M-1 延续管理（第三十轮）/ OBSERVE-90-01 日志区三态 / 88-01 / 85-02 / 84-01 / 83-01 + 77-02 + 76 族 + O-3 族全表续；OBSERVE-92-01/93-01 归档（已修复）。

## 教训
1. **历轮「零补偿」声明要追到 global 层 CSS**：F93-01 揭示历轮无障碍横查盲区——Button 组件内 focus 零命中不等于全站有焦点环，`global.css` 对 button 统一 `outline:none` 的重置把原生 focus 抹掉、组件层必须补偿。**「组件内零命中」与「全站无焦点环」是两回事，无障碍审计要追全局样式重置**。
2. **历轮观察项口径冲突要实证修正**：round91「Button 继承原生 button focus」口径与 global.css:174 直接冲突——不是「继承」，是「被重置抹掉」。**观察项结论的复核要能识别与前轮口径的矛盾，矛盾即重新实证的信号**。
3. **测试锚定的「死方法」注释保留优于删除**：O93-01 的 probeIntervalFor 生产零调用但 13 处测试锚定——删除会破坏测试可读性，注释澄清（仅测试锚定 + 生产为何不用）保留测试价值。**「死代码清理」要与测试锚定价值权衡**。
4. **api 抖动八连零复现（R86-R93）**：基线持续被实测支持，归因链稳定。