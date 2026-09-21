# review-round67 总结（2026-09-21）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端连续**十三轮零 MAJOR 零 MINOR**（M-1 第四轮常规复核闭合 + OBSERVE-66-03 无扩散）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 2**——**生产逻辑连续五轮零 MINOR**，flake 残余面自 R56 后首次回流 zhidao 包并定位根因。

**修复 4 处**（前端格式卫生二修 + 后端测试夹具两修：zhidao readyProbe 栅栏对齐 api 宽窗 + 本地 ddddocr 死测试补验证力）。

## 审查发现（写入 archive/review-rounds/round67-{backend,frontend}-findings.md）

### 后端（MINOR 0 / OBSERVE 2）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-67-01 | OBSERVE | zhidao 包 readyProbe 栅栏窄（200ms×5 无显式超时），flake 残余面自 R56 后首次回流——全量 R1 TestLoginLogsFailureSummary readyProbe 5 次全败 connectex（store 包 59s 高耗时后最末 mock 包窗口最恶劣）；隔离 4 连+5 连全绿确证冷启动残余 | ✅ 修复（readyProbe 对齐 api 包 200ms×10+显式 2s 超时宽栅栏） |
| OBSERVE-67-02 | OBSERVE | TestLocalDdddOcrAvailable 三态恒绿死测试——available=false+装了 python 无 ddddocr 时走到函数结尾无断言，零验证力 | ✅ 修复（补方向断言：有 python 时必须非零验证力） |
| R66 复核 | — | OBSERVE-66-01 成立（全仓 handler 单次 writeJSON 无「写响应后 panic」路径）/ OBSERVE-66-02 成立且护栏在位（TestSetVisionKeepsLocalRecognizer + 对偶） | ✅ 两条登记防御均无需动作 |
| 契约抽查 | — | 窗口关闭三判据 / 删号 memory-first / sameClientFor 六分支 / 重启恢复顺序 / 落库必记日志 / doLogin 闸门 / 时钟兜底 / 引擎热切换——8 条全成立 | ✅ |
| flake | — | 全量 5 轮 1 FAIL（zhidao 首现）+ mock-heavy 5 包 4 轮全绿；残余面动态流动宿主 = 包序最末 mock 包 + 最窄栅栏组合 | ⚠️ 定位 |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 1）
连续十三轮零 MAJOR 零 MINOR。**M-1 第四轮常规复核：仍闭合**（三消费点 :686/:501/:594+:602 仍真传 echoedRef.current、置位三路径 + 首帧不置位边界无回潮），**下轮起移出重点清单按历轮观察延续管理**。**OBSERVE-66-03 亚帧窗口扩散检查：无扩散**——全仓 `setSelected` 五调用点清点：函数式 updater 仅回显 effect（:250/:288）与独立清理 effect（:320）两族、对象式仅用户 pick（:351/:361），无第三来源；独立清理 effect 函数式读 prev 兜住。R66 三处卫生修复逐行核证全部成立（audit hover 豁免词边界准确零放行/jiti 注释全仓无残留）。OBSERVE-67-01（Dashboard.tsx:501 相邻 JSX 同行 + Select.tsx:1076 容量块缩进）**采纳修复**。构建全绿。

## 修复（主控核实后 TDD 直修）
| 文件 | 内容 | 验证 |
|---|---|---|
| `backend/internal/zhidao/client_test.go` | readyProbe 从 200ms×5 升级 200ms×10 + 显式 `&http.Client{Timeout:2s}`（对齐 api 包宽栅栏）——消灭「包序最末 mock 包 + 最窄栅栏」组合的残余窗口 | zhidao 包 race 全绿 |
| `backend/internal/zhidao/local_ocr_test.go` | 死测试补方向断言（有 python 时必须非零验证力）+ 注释补三态语义 | 实测 ddddocr 可导入 → PASS |
| `web/src/routes/Dashboard.tsx` | 501 行相邻 JSX 拆行对齐 | build 全绿 |
| `web/src/routes/Select.tsx` | 1076 容量块缩进归一 | build 全绿 |

**核实方法**：OBSERVE-67-01 我对照 api 包 readyProbe（handler_test.go:184-215，200ms×10 + 显式 2s 超时 + /ready 路径）与 zhidao 旧实现（client_test.go:85-103，200ms×5 + http.Get 默认无超时）逐行对比，确认栅栏宽度差异即残余窗口根源——对齐后栅栏窗口从 ~1s 扩到 ~2s + 探测请求自身 2s 超时兜底。OBSERVE-67-02 我实测本机 python 3.12.8 + ddddocr 可导入，确认新断言在开发机真实 PASS（非恒绿死测试）。

## 收尾全量回归
- `go build ./... && go vet ./...` → exit 0；`gofmt -l .` → 零输出
- 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` → 待跑（第一轮后台运行中）
- 前端 `npm run build` 全绿（800ms）；audit exit 0；target-guard 18/18

## 观察项延续（下轮复核）
后端：flake 残余面动态（zhidao 接棒 api「最脆弱」标签——残余面消失不可一劳永逸，栅栏宽窄一体化配平是根治方向）/ OBSERVE-63-04 probe 工具 / OBSERVE-63-05 stats 半真 / OBSERVE-62-06/07 / OBSERVE-61-03/04/06/07/08 / OBSERVE-66-01/02 登记；前端：M-1 移出重点清单 / OBSERVE-66-03 维持 / N-1~N-3 / O-1~O-12 / OBSERVE-67-01 已修。

## 教训
1. **「残余面消失」的标签不能一劳永逸**：zhidao 包自 R56 socketPreheat+TestMain 修复后全量 0 FAIL 长达 10+ 轮，R67 R1 又现——残余面是「包序最末 mock 包 + 最窄 readyProbe 栅栏」的函数，store 包 59s 高耗时（30050 行插入）在每轮全量末尾制造最恶劣 TIME_WAIT 窗口。修复方向不是补丁式特殊处理，而是把 zhidao/accounts 的 readyProbe 统一对齐 api 包宽栅栏（夹具一次性配平，杜绝包间栅栏宽度差异）。
2. **「恒绿死测试」要在覆盖审计中专门排雷**：TestLocalDdddOcrAvailable 三态全过、零断言——这类「因环境检测跳过而恒通过」的测试是最隐蔽的无验证力代码，人工走读全包时极易顺读放行。判断标准：测试末端是否有可达的失败断言（skip 分支之外）。
3. **R66 结论复核的取证要「源码 + 既有护栏」双锚**：OBSERVE-66-02 的「末端兜底成立」仅凭源码走读会漏掉「护栏早已存在」的事实——对照既有测试（TestSetVisionKeepsLocalRecognizer）确认后再下「闭合」裁决，避免报告开空头支票。