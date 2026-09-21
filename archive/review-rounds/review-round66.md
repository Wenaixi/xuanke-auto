# review-round66 总结（2026-09-21）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端连续**十二轮零 MAJOR 零 MINOR**（**M-1 修复第三轮闭合确认**）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 2**——**生产逻辑连续四轮零 MINOR**，flake **近 12 轮全量全绿**（R65 8/8 → R66 4/4）。

**修复 3 处前端卫生**（TDD 脚本 jiti 注释口径 + audit.mjs hover 过渡态豁免）；后端零改动（两条 OBSERVE 登记防御，其一经核实护栏已存在）。

## 审查发现（写入 archive/review-rounds/round66-{backend,frontend}-findings.md）

### 后端（MINOR 0 / OBSERVE 2）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-66-01 | OBSERVE | recoverMiddleware 对已提交响应后 panic 会追加写 500（net/http 忽略记 superfluous 警告），全仓当前无该类路径 | ✅ 续（登记防御，不修） |
| OBSERVE-66-02 | OBSERVE | accounts.Manager.SetVision 模板保留依赖 zhidao.Client.SetVision 末端 else 分支——%TEMP% 独立程序实证初版复刻缺 else FAIL、补 else PASS | ✅ 续（核实 client_test.go:231 TestSetVisionKeepsLocalRecognizer 护栏**已存在**，实际闭合） |
| R65 复核 | — | ①db 平台注释机制精确化与 Alpine 实测逐段一致（关键=真实建库成功非只 MkdirAll）/ ②accounts 两处 mock CT 与真实 text/html 语义对齐无副作用 / ③第三处 readyProbe 在位 + socket 双保险注释声明 | ✅ 三处全部闭合 |
| 契约抽查 | — | 窗口关闭三判据 / 删号 memory-first / sameClientFor 六分支 / 重启恢复顺序 / httpDo 仅 dial-write 重试 / IsReadErr 四形态 / 落库失败必记日志 / 引擎热切换模板 / 状态码家族 / doLogin 闸门 / 时钟兜底——11 条全成立 | ✅ |
| flake | — | **4 轮全量 -race 0 FAIL**（api 56.2s/21.7s/19.7s/24.8s 全绿、accounts 全绿）+ 隔离复跑全绿；残余面收敛到统计不可见 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 3）
连续十二轮零 MAJOR 零 MINOR。**M-1 修复第三轮闭合判定：仍然闭合**——核心从「实现层端到端证据」（R65）推进到「**亚帧窗口穷举推演 + React 状态队列语义最小构造验证**」：三消费点（防抖 :686 / flush :501 / handleBack :594+602）全部真传 echoedRef.current；echoedRef 置位三路径（courses 空 :238 / 合并完成 :295 / account reset :200）+ 首帧不置位（:232）穷举核验通过；反向推演穷举到唯一新窗口 OBSERVE-66-03（回显合并函数式 updater vs 用户 pick 对象式 setState 对象式覆盖在飞合并，亚帧竞态）——被下游五道防线（F17 空集守卫 / F16 发布 id 全数校验 / 回显合并自愈 / handleBack 5s / saveNow 串行化）全部兜住，保守方向、至多一次重试、不丢后端旧目标。六防保存链逐字符零回归。OBSERVE-66-01（audit.mjs 把 Button dark `hover:bg-neutral-900` 误报黑窟窿）+ 66-02（三脚本头注释 `node --import jiti` 在 jiti 2.7 抛 ERR_MODULE_NOT_FOUND）。

## 修复（主控核实后 TDD 直修）
| 文件 | 内容 | 验证 |
|---|---|---|
| `web/scripts/audit.mjs` | C 段 bg-neutral-95x 检查补 `(?:^|\s)hover:bg-neutral-9\d{2}` 词边界豁免——hover 悬停过渡态非静默黑洞 | 实测 exit 0 全绿 |
| `web/scripts/target-guard-check.ts` / `admin-auth-check.ts` / `unauthorized-check.ts` | 头注释 `node --import jiti` → `node --import jiti/register`（jiti 2.7 入口迁移） | 三条全绿（target-guard 18/18 + admin-auth 6/6 + unauthorized 5/5） |

**核实方法**：OBSERVE-66-02 我对照 client_test.go:231 `TestSetVisionKeepsLocalRecognizer`（断言本地 ddddocr 引擎在 SetVision 后保留）+ :258 `TestSetVisionRebuildsWhenCurrentIsVisionOrNil` 双护栏——客户端级 else 分支回归护栏**已存在**，代理的「可选补护栏」建议已满足，登记闭合。

## 收尾全量回归
- `go build ./... && go vet ./...` → exit 0；`gofmt -l .` → 零输出
- 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` → 待跑（第一轮后台运行中）
- 前端 `npm run build` 全绿（tsc -b + vite，568ms）；audit exit 0；target-guard 18/18

## 观察项延续（下轮复核）
后端：flake 近 12 轮全量全绿（残余不可见 ≠ 根除，CI `||` 重跑 + 首包夹具就绪前移永久保留）/ OBSERVE-63-04 probe 非可用工具 / OBSERVE-63-05 stats 半真测试 / OBSERVE-62-06/07 / OBSERVE-61-03/04/06/07/08 延续 / **OBSERVE-66-01 新登记**（recover 双层响应防御）/ **OBSERVE-66-02 登记闭合**（护栏已存在）；前端：M-1 闭合降级常规复核 / OBSERVE-66-03 亚帧竞态待复核机制无扩散 / N-1~N-3 / O-1~O-12 / Dashboard key 不对称 / ui 模板残宽 / NaN 防御 / 多标签页。

## 教训
1. **跨包设置器链的契约描述必须追到最末端消费者**：accounts.Manager.SetVision 的「模板保留」注释宣称模板层保留引擎，但模板层对「引擎为 nil 时」无能为力——真正保住本地引擎的是 zhidao.Client.SetVision 的 else 分支（客户端级兜底）。%TEMP% 独立程序初版复刻（缺 else）FAIL / 补 else 后 PASS 实证哪一层承担最终语义；但主控进一步核实发现**这层的回归护栏早已存在**（TestSetVisionKeepsLocalRecognizer），审查代理的「可选补护栏」建议已是既有实现。教训：报告的「可选修复建议」也要对照测试现有覆盖判定——别把已存在的东西当缺口。
2. **flake 收敛判据是「长尾不可见」而非「零残余」**：R65 8/8 + R66 4/4（近 12 轮全量全绿）已把 Windows 冷启动残余压到统计不可见，但 R64 曾预估残余流向 api 首包首测试、R65 R1 即命中、R66 又归零——残余不可见 ≠ 根除，CI `||` 重跑与首包夹具就绪前移是永久保留防线而非可移除补丁。
3. **亚帧竞态的价值判断要看「下游防线兜得住吗」**：OBSERVE-66-03（对象式 setState 覆盖在飞函数式合并 updater）是 React 更新队列既有语义、非 M-1 引入，最坏结果至多一次重试、绝不丢后端旧目标——被五道防线兜住的方向无需本轮动作，连续多轮零缺陷稳态下引入重构风险与收益不对称。同理 audit 豁免（hover 过渡态非黑窟窿）是工具词边界瑕疵而非产品缺陷，修工具不碰产品代码。