# R157 后端审查报告（绝对只读，身份防线矩阵第七十二轮）

- 日期：2026-09-24
- 基线：bffe587（R156 归档，身份防线矩阵第七十一轮闭合）
- 模式：绝对只读（唯一写文件为本报告；全仓库其余零修改）
- 实测环境：Go 1.26.8 windows/amd64，gcc 借 /d/mingw64
- 结论前置：**CRITICAL 0 / HIGH 0 / MEDIUM 0 / LOW 0，APPROVE，零漂移**，进度 158/256

## 验证表（全部实测）

| 验证项 | 结果 |
|--------|------|
| `go build ./...` / `go vet ./...` | 双 PASS（exit 0） |
| race·身份防线族十测 + 回归锚 | PASS（3.665s） |
| race·scheduler 全量 `-count=1` | PASS（14.995s） |
| race·zhidao 全量 `-count=1` | PASS（1.867s） |
| race·accounts 全量 `-count=1` | PASS（1.384s） |
| race·api 全量 `-count=1` | PASS（52.084s） |
| 回归锚 TestAdminStatsWindowOpenedUsesScheduler | PASS（1.852s） |
| 脱敏/判型族（TestSanitizeError* / TestIsReadErrCoversAllForms / TestDoRequestSanitizesDialError） | PASS（1.447s） |
| 定向·api 手动族+管理身份族 12 测 | PASS（4.285s） |
| 定向·scheduler 手动协同族 4 测 | PASS（1.418s） |
| 轮次标签扫描（backend 非测试） | 唯一命中 session/store.go:117 引用历史文档路径 `docs/review-round13.md`，属文档引用非轮次前缀标签，合规 |
| 工作区漂移 `git diff bffe587 -- backend/` | 0 行 |

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第七十二轮 —— 在位

`sameClientFor`(:204) + `clientIdentity`(:215) 逐字符核对（reflect 指针身份、nil 与非指针归 0）。7 调用点零漂移，终局逐一追证：

- :850 —— 放弃写 openTimeDetected/acctData/acctDataAt
- :1489 —— 删 inflight 后 return 且绝不再触发 maybeRelogin（:1494-1498 注释钉死）
- :1521 —— 不写 done/setState/AppendLog/SaveSuccess
- :1551 —— 不写 rateLimited
- :1571 —— 不写 markFullLocked
- :1600 —— 整块三路归并整体放弃
- :1635 —— 满员单路二次拦

链顶捕获 `chainClient := client`(:1417) 单次捕获无中途重取。maybeRelogin 决策侧 :1208（双锁内、任何 map 写之前，三处直调入口 ProbeForAccount:823 / ProbeNow:956 / probe:1094 全经此闸）+ 写回侧 :1254 复核 + :1265-1273 二次 ClientFor 重取当前注册表 Token 落库。手动五路 accountExists :255/:305/:397/:497-512（内联等价实现）/:573。写点换类 5 类（lastSubmit:1346 / lastSyncStart:356,:405 / syncing:355,:369,:404 / lastProbe:679,:964,:1089,:1114 / EmptyProbeRuns:1156,:1158）全持锁；warnedNoTargets:1371-1372 唯一读写点、宿主唯一、单写者单读者。*Locked 写函数族 13 个调用点全部锁内，外部写函数 19 个首行取锁/延迟解锁全量射证。

### 2. OBSERVE-117-01 知识位第四十轮 —— 在位

顺序严格：:1254 存在性复核 → :1265 二次 ClientFor 重取 → :1266 Token() 非空才赋值 → :1269 UpdateIDToken 失败留痕。恢复侧 manager.go:295-317 用库内 token，首尾一致。

### 3. B110-01 审计链第四十七轮 —— 在位

手动 6 失败位 :362/:374/:381/:443/:452/:459 全部 `if err != nil { log.Printf }`；成功审计行 :1976/:2029；自动链 8 处 AppendLog；零吞错穷举仅 4 处 `_ =`（captcha.go:150、client.go:222/:395、rsa.go:30，全为编码/解析 fire-and-forget，无一处落库点）；脱敏延续 client.go:581 sanitizeError + scheduler.go:1334 maskedToken + manager.go:319 tokenShort。

### 4. O105-01 抖动基线 —— 在位

socketPreheat(client_test.go:25) + readyProbe 三处夹具在位；四包 race 全绿无 DATA RACE；双回归锚绿。

### 5. LOW-132/133 回首核 —— 通过

time.Since/time.Now 残余仅 reloginAt（:1231/:1261 写，:1220/:1227 读）与 gateWindow（manager.go:55/:74/:227 写，:54/:71/:226 读）两处同基自洽；探测族时间戳全部对齐钟写读；`git diff bffe587 -- backend/` 0 行。

## 新契约角度纵深（自选 ×2）

### 角度 A：连接活性自愈族 httpDo 判型互斥

httpDo(:478-492) 仅 `isConnErrRetryable` 真才 cloneReq 重发一次；isConnErrRetryable(:496-505) 只认 `net.OpError.Op == dial|write`；IsReadErr(:523-552) 覆盖 io.EOF / io.ErrUnexpectedEOF / 两条 Timeout 文案 / OpError.Op=="read" 四形态。两判据在 OpError 维度天然互斥，不存在「既可重试又可能已处理」的重叠区。doRequest(:444) 与 captcha.go:162 双路径一致；调用方三处（spawnChain:1656、handler:371/:450）文案与「绝不重试 read」形成闭环。

### 角度 B：手动四方法协同族闭环

TryAcquireSubmit(:1896) sync.Once 幂等释放 + api 两处 defer release；MarkDone(:1922) 首行锁 + ClientFor 已删复核 + refused 内存与库双向清理(:1938-1948)；RemoveDone(:1989) 同款已删复核 + success 行删除(:2020) + refused 落库(:2026)。双向清理闭环成立（否则重启 RestoreDone/RestoreRefused 会静默撤销用户意图）。RestoreTargets(:525) 不清 refused 与 SetTargetsForAccount(:454) 清 refused 不清 done/full/rateLimited/inflight 的差异注释明确。RemoveFull(:2038) 仍无产品调用方，孤儿接口。

## 维持观察项（9 条）

1. IsClassFull 恒 false（CountEntry.MaxCount 未实证），真满员主判据为 classFullInSnapshot
2. RemoveFull 无调用方
3. syncFailedWindow 写而不读
4. ddddocr CGO=0 stub 回退
5. accounts 夹具无 socketPreheat 双保险
6. 两个 DELETE 端点刻意无 requireJSONBody 门（CSRF 面仍闭合）
7. warnedNoTargets 无锁写点
8. probeSem cap=4
9. httpDo 单次重试的连续两跳形态未观测

## 结论

**APPROVE** —— CRITICAL/HIGH/MEDIUM/LOW 全零，无修复需求，可归档。
