# R124 前端只读审查报告（web/，React 18 + Vite + TS + Radix UI + Tailwind）

审查基线：commit `0333009`（R123 收尾提交），web/ 自 `1351fa4` 起零改动。全链路只读，唯一写入为本报告与构建产物 `backend/web/dist/`。

## 必查项结论

### 1. M-1 第六十轮闭合 — 通过（零漂移）

**shouldDeferSave 消费点恰 4 处 + 1 定义点**（`grep -n shouldDeferSave` 全仓实证，定义 1 + 消费 4）：

| 消费点 | 位置 | 第三参 `echoedRef.current` |
|--------|------|--------------------------|
| flushTargets 守卫 | Select.tsx:509 | 逐字符一致 |
| handleBack 首闸 | Select.tsx:597 | 逐字符一致 |
| handleBack 等待循环 | Select.tsx:605 | 逐字符一致 |
| 防抖回调守卫 | Select.tsx:699 | 逐字符一致 |

- 定义点 `web/src/lib/targetGuard.ts:64-72`，三参数签名 `(stateData, hasSelected, echoed)`，判据 `stateData===undefined→true; echoed→false; courses非空&&hasSelected`，与本轮任务描述逐字符吻合。
- **echoedRef 置位三路径**：`:200`（账号切换复位 false，守卫路径）、`:240`（/state 首帧 courses 空分支置 true）、`:297`（回显合并完成置 true）。三处 `echoedRef.current = true` 全部在位。
- **首帧不置位四边界**：`:229`（`if (echoedRef.current) return` 只合并一次短路）、`:234`（`stateData === undefined` 提前返回不置位）、`:247`（courses 空分支置位后 return——本行是置位路径非边界，边界为 courses.length===0 判定本身）、`:319`（独立清理 effect `if (!echoedRef.current || publishes.length === 0) return` 未回显不清理）。边界语义完整：首帧未到、publish 缺席、幽灵条目、全清空绝不合并均正确拦截。
- **断言脚本实测**：`target-guard-check` 18 条断言全绿（含"混合→只清非空旧 key"、"无变更→返回原引用"尾两条实证）。

### 2. OBSERVE-93-01 残余面第十四轮 — 通过（零增零减）

`grep -c "<button"` 全仓实证：Admin.tsx **13** / Dashboard.tsx **2** / Login.tsx **2** = **17 处、3 文件**，与 R123 基线一致。651/661 引擎二选一仍为强 active 态按钮（`border-white bg-white text-black`），非 `ui/Button` 收敛，仍为优先修复面候选。

### 3. F93-01 第三十二轮 — 通过（双空实证）

- `git log --oneline 1351fa4..HEAD -- web/`：**空输出**（无提交触及 web/）。
- `git diff --stat 1351fa4 HEAD -- web/`：**空输出**（无差异）。
- `Button.tsx:42` focus-visible ring 逐字符在位：`"focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]"`。

### 4. OBSERVE-116-01 跟踪项第八轮维持 — 通过

- `web/src/lib/useTickingCountdown.ts:3-8` 注释如实口径在位：「每秒 setNow 实际触发宿主路由组件整树重渲染……DOM 差分成本可忽略；「只重渲染倒计时一处」需拆 memo 叶子组件（潜在优化，非当前承诺——本注释已按实现如实口径，不再声称局部渲染）」。
- 轮询链路契约未触碰：Dashboard /state `3000/30000`（:140-145）、logs 同款 `3000/30000`（:156-160）、Dashboard /electives 恒 `30000`（:174）；Select /state `2000/30000`（:148-154）、/electives `2000/10000/30000`（:58-81）。全链路逐一实证无漂移。

### 5. OBSERVE-115-01 弹窗族 — 通过（三处最小语义门全在位）

| 弹窗 | 文件:行 | role=dialog | aria-modal | aria-labelledby | Esc 关闭 | autoFocus |
|------|---------|------------|------------|-----------------|----------|-----------|
| Select 退选 | Select.tsx:1200-1210 | :1203 | :1204 | `exit-modal-title` :1205 | :1208-1210（退选中不响应防误关） | 取消按钮 :1242 |
| Login 激活 | Login.tsx:226-234 | :226 | :227 | `activate-dialog-title` :228 | :232-235（activating 中不响应） | 输入框 :267 |
| Admin 删除 | Admin.tsx:213-218 | :213 | :214 | `delete-acct-modal-title` :215 | :216-218（deleting 中不响应） | 取消按钮 :241 |

三处 Esc 关闭均带在飞守卫（退选/激活/删除进行中不响应防误关），autoFocus 均落在最安全默认项（取消/激活码输入框）。

### 6. OBSERVE-117-02 timer 类型卫生 — 通过

- `Select.tsx:393` 声明：`const retryState = useRef({ attempt: 0, timer: null as ReturnType<typeof setTimeout> | null })`。
- resetRetry（:409-414）与 scheduleRetry（:420-428）配对：挂载处赋 `ReturnType<typeof setTimeout>`、复位处 `clearTimeout + timer=null`；attempt≥5 封顶。
- 卸载 cleanup（:399-406）同时清 `unmountedRef` 与退避 timer；StrictMode 重挂载复位注释在位。
- **`npm run build`（tsc -b + vite）全绿**：1948 模块变换，产物 `index-Bm7TtkV4.js`（420.77 kB / gzip 125.47 kB）425ms 构建完成，tsc -b 零错误零警告（此前 tsc 阶段若有类型错误会直接中断，未出现）。

### 7. 新契约角度：手动操作在飞守卫族全路径 — 通过（无卡死路径）

完整生命周期实证（Select.tsx）：

- **置位点**：handleSelectClass :90-91（`if (actionLoading.has(c.id)) return` → `new Set(prev).add(c.id)`）；handleConfirmExit :117-119 同款。Set 按课程 id 独立跟踪，单值互相覆盖根因（注释 :49-51 自述）已消。
- **守卫消费点**：按钮 disabled 三处 :1120 / :1133 / :1232 + :1242（弹窗取消/确认双按钮）；入口短路 `if (actionLoading.has(c.id)) return` 双处 :92 / :119（注释 :88-89 自述 disabled 渲染落地前双击双发，后端 TryAcquireSubmit 拒第二个但 finally 仍 invalidate 假失败 toast——入口短路堵住）。
- **复位点**：两 handler 的 finally 均为函数式清除 `n.delete(c.id)`（只删自己的 id，绝不抹其他课程在飞标记），覆盖成功 / 失败 / 异常三路。弹窗关闭路径：取消按钮（在飞时 disabled）与 Esc（在飞时不响应）双闸，退选中弹窗不消失。
- **边界结论**：无「置位后永不复位」卡死路径——finally 结构性保证（try/finally 无提前 return），函数式清除不受闭包过期影响；`handleSelectClass` 成功分支 toast 后无手动清 actionLoading 的重复路径（finally 已覆盖）；`handleConfirmExit` 成功分支先 `setExitModalClass(null)` 后 finally 清标，弹窗关闭与标记复位顺序无冲突。
- **TryAcquireSubmit 冲突边界**：入口短路即停（同课程双击不产生第二发请求），后端 inflight 占位由调度器 `TryAcquireSubmit` 拒绝语义兜底，前端只在真正发起请求时置位；冲突提示「该课程正在提交中」为后端错误文案映射（cErr 归并路径），前端在飞守卫先于后端拦截，两者为同一防线族的两道闸，无遗漏。

## 契约 20 轮次标签扫描

`grep -rn "第.*轮|(round|R1[0-9][0-9]" web/src/`：**零命中**。代码注释无轮次前缀标签残留，全部为「为什么/契约/陷阱」本体语义。

## 验证表

| 验证项 | 结果 |
|--------|------|
| target-guard-check 断言 | 18/18 全绿 |
| admin-auth-check 断言 | 6/6 全绿 |
| unauthorized-check 断言 | 5/5 全绿 |
| npm run build（tsc -b + vite） | 通过（1948 modules，425ms） |
| git log 1351fa4..HEAD -- web/ | 空（F93-01 双空实证之一） |
| git diff --stat 1351fa4 HEAD -- web/ | 空（F93-01 双空实证之二） |
| `<button` 全仓清点 | 17 处 3 文件（Admin 13 / Dashboard 2 / Login 2）零增零减 |

## 分级发现

- **必查项**：7/7 全部通过，无一发现。
- **观察项维持**：OBSERVE-93-01（651/661 引擎二选一非收敛按钮仍为候选修复面）、OBSERVE-116-01（每秒 setNow 整树重渲染潜在优化注释如实口径）。
- **无新发现**：手动操作在飞守卫族生命周期完整闭合，无「置位后永不复位」卡死路径；轮次标签扫描零命中。
- **全仓库文件改动**：零（除构建产物 `backend/web/dist/` 与报告本身）。
