# 第 18 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（main.go/api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/cmd/web-embed）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI。
> 审查方式：两个 opus 权威子代理并行只读审查（后端 + 前端独立通道，均附"宁缺毋滥 + 文件行号 + 触发场景 + 已知观察项去重"模板），发现全部经主通道逐一读码推演定案；修复独立 commit。
> 本轮结论：**前端 1 项 CRITICAL（F18-01 Select.tsx 死代码致前端构建中断）+ 2 项 MAJOR/MINOR（F18-02 配置加载前保存覆盖真实配置 / F18-03 发布重建 Tabs 悬空）**；**后端 2 项 MAJOR（B18-M1 窗口关闭防轰炸 / B18-M2 删除账号竞态写回）+ 1 项 MINOR（B18-m1 复核快照过期无 TTL 校验）**。

---

## 一、前端发现与修复状态（第 18 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F18-01** | CRITICAL | **Select.tsx 死代码常量致前端构建彻底中断**：F17-02 把假清空守卫判据改为消费时刻实时判据后，渲染期常量 `publishesMissing`（Select.tsx:284）遗留成死代码，且其 TDZ 初始化引用**声明于其后的** `publishesRef`（342）/`selectedCount`（400），直接触发 `tsc -b` 5 个编译错误（TS6133/TS2448/TS2454）、`npm run build` 退出码 2——CI（ci.yml）与 Tag 发布流水线（release.yml）必然红，发版会把旧前端嵌进单二进制。**修复**：删除该常量声明与过时注释，功能零影响。`npm run build` 实证红灯→绿灯（407.71 kB）。**警示**：项目根 `tsc --noEmit` 是 references 空壳不报错，只有 `npm run build`（`tsc -b`）真校验——此前"tsc 通过"验收结论失真，前端回归一律以 `npm run build` 为准 |
| **F18-02** | MAJOR | **系统配置加载前保存会用初始空值覆盖真实配置**：ConfigTab（Admin.tsx:409-464）在 `configQuery` 返回前保存按钮即可点，表单仍为初始值（`baseUrl=""`/`model=""`/`openTime=""`/`activationOn=true`），保存会整体覆盖生效配置——`open_time` 被清空 = B11-A1 明确语义"显式清空开放时间"（调度器挂起提交）、Vision 配置清空、激活码机制误开。**修复**：按钮 `disabled={saving || !loaded}` + `save()` 入口 `!loaded` 守卫 + 加载中提示文案，双保险 |
| **F18-03** | MINOR | **选课大厅 Tabs 非受控 defaultValue 发布重建后悬空**：`<Tabs defaultValue={String(tabs[0].publish_id)}>` 只在首次挂载生效；发布集合整体重建（F16-01 记载的开窗瞬间平台清空又恢复、publish_id 全变）后激活 value 的 Trigger 消失，Radix Tabs 无 fallback，主内容区空白直到用户手点。**修复**：改受控 `value={activeTab ?? String(tabs[0].publish_id)}` + `onValueChange`，发布重建自动回落首个 Tab 绝不悬空 |

## 二、后端发现与修复状态（第 18 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **B18-M1** | MAJOR | **窗口关闭后平台真实文案"无效的课程ID"不在防轰炸判定集合内 → 未成功目标永续轰炸报名接口**：`isWindowClosedError` 只匹配"关闭/未开启/报名时间/已结束"，平台对关闭后报名的真实返回是 `code:1 "无效的课程ID"`——不命中 → 不记 full → 走实时人数复核（`classFullRealtime`）→ 窗口关闭后 `countList` 空 → `IsClassFull` 报"课程无人数数据"→ 下个 tick 重打 `SelectClass`，对未成功目标每 1s（黄金期 250ms）永续轰炸。现有测试注入"已结束"文案，掩盖了真实形态（测试假阳）。**修复**（commit 6f3cf78）：① tick 提交守卫新增 `if s.WindowClosed() { return }`（与 B11-A1 零值守卫并列），以探测状态而非脆弱错误文案作为关闭判定；② `WindowClosed` 判定升级为"**至少开过窗**"前提——`prevOpened` 先捕获再覆写（顺序 bug 实证：先覆写后捕获读到恒为本轮值，三个窗口关闭契约测试全红），未开过窗即空快照（学期无发布/平台异常）不算关闭，防黄金期被误挂起。测试改真实文案 + 预置开过窗 + 三契约测试补反向断言 |
| **B18-M2** | MAJOR | **删除账号与在飞 spawnChain 网络往返竞态写回 success 行**：`DeleteAccount`（清 credentials/accounts/targets/success 表 + `Accounts.Remove`）与在飞链的 `SelectClass`（最长 15s）竞态时，本链在删除完成后才返回成功，`SaveSuccess` 把已删账号的 success 行写回，重启后重新登录被 `RestoreDone` 恢复成"已报名成功"假状态。重登完成路径已有同款 `ClientFor` 复核（891 行），提交链成功分支漏了同一防线。**修复**（commit f6ee556）：写 done/落库前锁内复核 `s.clients.ClientFor(acct)` 仍存在，已删则放弃落库。TDD：`TestDeletedAccountInFlightDropsSuccess` 用 `selectBlock` 阻塞钩子卡住网络往返、期间标记账号删除 → 红灯（1 行）→ 绿灯（0 行） |
| **B18-m1** | MINOR | **CheckClassSelectable 无快照 TTL 校验，过期快照仍拦截真实操作**：手动报名复核只判 `data == nil`，超过 `snapshotTTL`（40s）的旧快照仍按旧数据拒绝（满员/窗口），而名额/窗口早已变化。**修复**（commit f6ee556）：以 `acctDataAt` 时间戳判过期（与 `ElectivesSnapshotFor` 同源），过期一律按"无快照"放行交给平台把关。测试补"快照过期后放行"断言，TDD 红灯（"该课程已满员或不可选"）→ 绿灯 |

## 三、修复细节（本轮前端 3 项 + 后端 3 项，独立 commit）

- **F18-01**（commit c97eb0c）：Select.tsx 删除死代码 `publishesMissing` + 过时 F16-01 注释
- **F18-02 + F18-03**（commit 0c46ef5）：Admin.tsx 配置加载守卫 + Select.tsx Tabs 受控化
- **B18-M1**（commit 6f3cf78）：窗口关闭防轰炸（tick 提交守卫 + WindowClosed 判定升级）
- **B18-M2 + B18-m1**（commit f6ee556）：删除账号竞态防线 + CheckClassSelectable TTL 校验

## 四、回归证据（提交时点）

- `cd web && npm run build`（`tsc -b` + `vite build`）— 通过（407.93 kB / gzip 122.21 kB）
- `go test -race -count=1 ./...` — 全绿（api / config / db / runtime / scheduler / secure / session / store / zhidao）
- scheduler 包防轰炸契约测试 `TestWindowClosedSelectStopsBombing`：先红（0 次断言实际 1 次，首 tick 探测覆写预置 WindowClosed）→ 改空快照形态后绿灯

---

## 提交索引（本轮前端 2 个 commit + 后端 2 个 commit）

```
c97eb0c fix(web): 第18轮F18-01 CRITICAL Select.tsx死代码致前端构建中断（publishesMissing遗留TDZ引用, tsc -b 5错, 删除即绿）
0c46ef5 fix(web): 第18轮F18-02 MAJOR配置加载前保存覆盖真实配置 + F18-03 MINOR发布重建Tabs悬空
6f3cf78 fix(backend): 第18轮B18-M1 MAJOR窗口关闭防轰炸漏洞（真实文案无效的课程ID不命中, tick提交守卫+WindowClosed至少开过窗前提）
f6ee556 fix(backend): 第18轮B18-M2 MAJOR删除账号与在飞spawnChain竞态写回success行 + B18-m1 CheckClassSelectable快照过期无TTL校验
```

## 第 18 轮观察项（未修复，留档）

- **HTTP 探测单飞**（B14-I2/B16-M2 延续）：`ProbeForAccount`/`ProbeNow` 未接入 probing/lastProbe 节流——触发条件已精确化（30s 周期至多多 1 次），维持观察
- **Decrypt 死字段**（B16-I1 延续）：加密工具中未使用的解密函数字段，删除待定
- **RemoveFull 死方法**：手动退选解封后已无调用方，删除待定
- **probe per-account 错误静默**：`ProbeForAccount` 返回值被丢弃（`_, _ =`），频率受 30s 节流约束，维持观察

## 下轮待办

- 第 19 轮：发射两枚 opus 只读 agent（后端 + 前端），沿用同一模板（读全部模块、宁缺毋滥、已知观察项去重）