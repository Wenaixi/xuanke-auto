# 第 36 轮全模块审查记录（2026-09-16）

> 审查范围：backend 全部模块 + web 全部模块 + countList 契约专项。并行子代理产出：
> 后端全模块只读审查（review36-backend，1 MINOR + 0 可疑 + 2 否决）、前端全模块只读审查
> （review36-frontend，1 MAJOR + 3 可疑）、契约专项核查（review36-contract-countlist，
> countList.maxCount 实证定案）。主 gate 逐条现场核实（读源码 + 推演真实触发路径）后决策。

## 前端（F36 系列）

### F36-01（MAJOR）Select 跨账号复用导致目标串号与旧目标永不回显
**缺陷**（review36-frontend MAJOR 1）：App.tsx 条件渲染链两处 `<Select>` 挂载点
（targetAccount 代理分支 / current 学生分支）均**无 `key={account}`**——React 同位置同类型
组件实例复用，账号切换时组件状态整链从旧账号保留到新账号。
**触发路径**：同一浏览器登录两学生账号 A、B（App 支持多账号 sessions）→ 停在 A 选课大厅
（page==="select"）→ A 的 Bearer 会话过期（12h TTL 挂机过夜）或被管理员删除 → 下次轮询 401
→ onUnauthorized 摘除 A → account-reselect effect 自动把 current 切到 B、page 仍 "select" →
`<Select account={B}>` 复用 A 的实例（`selected`/`echoedRef`/`rev`/`actionLoading` 全保留）→
回显 effect `if (echoedRef.current) return` 对 B 的 /state 直接跳过（B 后端旧目标永不合并）→
B 点一门课 → rev+1 → 防抖回调（echoedRef 恒 true 守卫通过）→ build() 用 `[B publishes × A 残留
selected]` 联查构造目标 → PUT 整包覆盖删掉 B 后端目标（"添加一门"变"替换全部"，静默无提示）。
**影响**：多账号浏览器部署下跨账号目标静默串号/被覆盖，B 账号自动引擎按错误目标抢课；
单账号部署不触发。
**修复**（双层防线）：
1. App.tsx 两处挂载点补 `key={account}`——账号切换（401 被动吊销自动切剩余账号/管理员代理
   切换/登出重登）即整体重建组件实例，selected/echoedRef/rev 全复位。commit `bcd31c2`。
2. Select.tsx 补 `accountKey` 复位守卫——account prop 变化同步复位 selected/rev/echoedRef/
   echoDone，兜底"未来改为不重置挂载"的意外回归。**注意**：守卫必须声明于 echoedRef/rev/
   setRev/setEchoDone 之后（首次插在 selected 后即 TS2448/TS2454，F18-01 同款 TDZ 陷阱），
   移至 activeTab 声明后构建即绿。commit `ab59b24`。
**验证**：npm run build（tsc -b + vite）通过。

### 可疑项裁决（review36-frontend s-1/s-2/s-3）
- s-1（返回按钮最长约 68s 无反馈挂起）：真实网络下 21s deadline 与 20s api 超时收敛远快于
  上界；按钮幂等（disabled+等待循环）不叠加；加"正在保存目标..."反馈属 UI 增强非缺陷。
  维持观察，不修。
- s-2（/electives 持续失败期间回显永不完成、用户改动被永久置脏拦截）：安全方向——改动延迟
  落库而非错误落库；data 变化 → 回显 effect 重跑自愈（publishes 到达即合并+置位），失败恢复
  即收敛。维持观察，不修。
- s-3（回显 effect 依赖不含 account）：与 F36-01 同源，已被双层防线（key 重置 + accountKey
  复位守卫）覆盖，同批处理完毕。

## 后端（B36 系列）

### B36-01（MINOR）SetTargetsForAccount 重设目标时 DeleteRefused 落库错误被静默吞掉
**缺陷**（review36-backend MINOR）：scheduler.go:404 `_ = s.store.DeleteRefused(acct)` 是
**全仓库唯一吞错落库点**（review36-backend 落库错误处理全量扫描 11 处 AppendLog/SaveSuccess/
SaveRefused/DeleteSuccess + handler.go 6 处审计日志 + DeleteAccount 等全带 err 处理，独漏此处）。
**触发路径**：用户手动退选课程 X → RemoveDone 写 refused 行落库 → 用户把 X 重新设为目标 →
SetTargetsForAccount 内存 delete(s.refused) + `_ = DeleteRefused` 清库行 → 此刻 SQLite 写失败
（磁盘满/IO 错误/单写者事务阻塞超时）→ 内存已删、库内 refused 行残留 → 服务重启 → main.go
恢复序 LoadRefused + RestoreRefused（RestoreTargets 不清 refused，B10-01 契约）把 X 恢复成
"已手动退选（自动引擎不再接管）"→ **用户明确重新接管的课程被自动引擎永久跳过**。
**修复**：改 `if err := s.store.DeleteRefused(acct); err != nil { log.Printf(...) }`，日志含
"重启后该课会被恢复成'已手动退选'"警示语；**内存侧 delete 保留不补回**——它代表当前运行期
用户意图（补回会阻塞自动引擎对正常重新接管的课程，反而错误）。与 B33-01/B34-03/B35-01 决策
锚"任何落库点一律 `if err` 捕获 + log"全量对齐（至此零吞错落库点）。
**TDD**：failStore 补 DeleteRefused 失败注入 + `TestSetTargetsDeleteRefusedFailureLogged`
（fail=true 下 SetTargetsForAccount，断言日志含"清空退选记录落库失败"）。
**验证**：go build/vet + `go test -race -count=1 ./...` 9 包全量通过。

## countList.maxCount 契约认知缺口定案（review36-contract-countlist）

**结论：`maxCount` 判定不存在于 findElectivesStudentCount 真实响应**（观察项 34-01 从"需实测
确认"升级为"实证定案"）。
**证据链**：
1. 真实 select.js（legacy/website-source/www.zhidao.fj.cn___views__electives__select.js 4345 起）
   轮询回调只消费 `t.id`（匹配键）/`t.selectedCount`（写 selected_count 列）/`t.auditedCount`
   （写 audited_count 列）三键——selectedCount/auditedCount 全文件各 1 处、**maxCount 全文件
   0 处**（主 gate grep -c 复核确认）；可报数/满员在真实前端来自快照级 `max_count`（表列
   max_count/selected_count/audited_count 同屏静态列），可点性全由 `can_select[+btn_type+title]`
   双守卫判定、前端从不做数字对比。
2. HAR 面：主 HAR 实为 **173 条 entry**（CLAUDE.md 旧记"8468 条"为误记，顺手修正）——没有一条
   findElectivesStudentCount 请求或响应样本；两个 maxCount 均出现在 echarts 库配置，与平台无关。
3. Go 端对比：CountEntry 解 3 字段全实证；`IsClassFull` 的 `MaxCount>0 && SelectedCount>=MaxCount`
   判据因 MaxCount 恒 0 而**恒 false**——实时复核实际退化为"永不确证满员"。
**影响评级：非缺陷，契约认知缺口**——调度器真满员判定主路径是 `classFullInSnapshot`（快照
`max_count` 实证字段，spawnChain 1273 行已先于实时复核用快照判满员跳过），实时复核只是防御性
兜底；防轰炸契约由窗口关闭守卫链全权兜住，未造成停摆。
**处置**（commit `017b284` + `e05099b`）：
- client.go `CountEntry`/`IsClassFull` 补契约注释（maxCount 判定不存在、实时复核恒 false、
  主路径是快照判定、保留字段仅为未来下发的防御性解）
- scheduler.go `classFullInSnapshot`/`classFullRealtime` 补注释（快照 max_count 为实证主路径、
  实时复核为防御性兜底）
- CLAUDE.md countList 契约段定案 + HAR 条数误记修正（8468→173）

## 已核无缺陷清单（review36-backend 12 项 + review36-frontend 13 项）

后端：落库错误处理全量扫描（零吞错落库点达成）；删账号 memory-first 四路 ClientFor 复核；
spawnChain 链顶双复核；窗口关闭三判据单源 + tick 守卫顺序；时钟校准（syncing 调用侧置位/无
账号复位/30s 退避/streak 真实累计）；重登体系（reloginMu 锁序/tokenValidForLocked 短路/指数
退避/成功分支复核）；探测防轰炸（30s 节流+probing 单飞+probeSem cap4）；手动报名/退选对称性
（ErrUnauthorized 对称/TryAcquireSubmit 排他/B23-01 让位/accountExists 矩阵）；ElectivesSnapshotFor
回退链（B22-01/B28-01）；API 层审计日志（B35-01）；open_time 零值契约（B25-01）；安全面
（口令拒启动/ConstantTimeCompare/AES-GCM/票据单次防重放/IP 限流/等时延迟/SPA /api 前缀 404/
nosniff）。否决候选 2：`s.state.Courses[:0]` 共享底层数组（全部 17 处读写路径持 s.mu 串行、
StateForAccount 全新 slice header 不逃逸锁外，无触发序列）；reloginAt 本地钟 vs 对齐钟
（~640ms/30s+ 可忽略，历史定案）。

前端：防抖自动保存链（savingRef/lastJson/targetRef/dirtyRef 串行化 + finally 补发 + 指数退避 +
unmountedRef 双向）；假清空守卫链三闸（发布缺席+已有选中/联查空集+已有选中/publish_id 漂移）
消费时刻实时判读；回显合并（echoedRef 一次/全清空守卫/已触碰空数组不复活非空补进/幽灵过滤/
TDZ 规避）；handleBack（33-01 回显等待/M30-02 等 PUT 静止/F26-01 pendingSaving/32-01 flushedRev
收敛/3 轮上限）；在飞幂等（actionLoading Set/M29-01、Admin removing Set/31-03、deleting、Login
submit/activate、ConfigTab saving）；视图态清理（logout page 重置/account-reselect 空账号/401
session 反查/F25-01/App-34-02/M28-01/adminName 持久化）；btn_type 三向契约；max_count=0 名额
未公布全语义五处同源；Tabs 受控兜底；手写弹窗 Esc+autoFocus；api client 20s 超时+AbortError+
401 事件 session 归属；Toast 自增 id。

## 契约一致性核对（对照 legacy/ 真实源码基线 + CLAUDE.md 逆向契约）

- form 编码非 JSON ✓（全部 POST body application/x-www-form-urlencoded，对齐 popReq jQuery 默认）
- 成败看 isOk 不靠 code ✓（SelectClass/ExitClass `j.Code != 0 || !j.IsOk` 双判）
- code=-1 → ErrUnauthorized → MaybeRelogin ✓（绝不自行重登）
- code:0 空 publishes = 窗口关闭 ✓（FindElectives 空快照 + now.After(open+10s) 裕量）
- isWindowClosedError 仅作二级兜底、主判据走探测状态 ✓
- idToken URL 参数 + Cookie 双通道 ✓（契约主体锚定 URL 参数 window.idToken）
- 满员判据 maxCount 未实证已定案 ✓（Go 解码但判定不依赖）
- btn_type 三向后端不消费（前端三向渲染）✓
- StudentCounts 只消费 selectedCount/auditedCount 不反写课程帧 ✓

## 回归

- 后端：`go build ./... && go vet ./...` 全绿；`go test -race -count=1 ./...` 9 包全量通过
  （含 B36-01 修复后 + TDD 新测试）。
- 前端：`npm run build`（tsc -b + vite）通过（含 F36-01 双层防线后）。

## 观察项（本轮追加/延续）

- 观察 36-01：s-1 返回按钮等待无视觉反馈（真实网络收敛远快于上界，UI 增强非缺陷）。
- 观察 36-02：s-2 /electives 持续失败期间改动延迟落库（安全方向，失败恢复即收敛）。
- 观察 36-03：TestClockSyncFailureResetsOffset 线 1343-1345 注释"第 3 轮后失败计数可能已触发
  回退而清零"与 B21-01 实现语义分叉（streak 只在同步成功时清零，回退不清零）——测试行为
  正确（核心断言 1368/1380 真实校验），仅注释过时，下次触碰该测试时顺手修正。
- 历轮观察项全表延续（task_log 无容量上限、死代码族、HTTP 探测单飞、probe per-account 错误
  静默、幽灵课程条目无 UI 提示等）。
