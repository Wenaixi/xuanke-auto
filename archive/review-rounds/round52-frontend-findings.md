# round52 前端审查原始发现

> 审查基线：master @ `7a8a177`（round51 收官：O-1 终裁已执行，c2ed5b5 删 Dialog/Sheet/Table + react-dialog/select/switch 三依赖）。工作树干净（仅根目录 5 个未跟踪社区文档）。
> 校验：`cd web && git status --short` 干净（仅根目录 5 个 `??` 社区文档）；`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`cd web && npm run build` **成功**（tsc -b + vite 全绿，1947 modules，产物 416.74 kB js / 41.04 kB css 与 round51 同尺寸，构建后 web 目录零 git 变更）。范围 `web/src/` 全部 `.ts/.tsx`，绝对只读。
> 后端契约对照：`backend/internal/api/handler.go`（handleLogin B43-04 撞名双条件 / requireAdminSession IsAdmin）与 `backend/internal/session/store.go`（CreateAdmin Admin=true / IsAdmin）。
> 上轮（round51）前端 1 修已闭环（O-1 执行），本轮为跨轮第 7 次复证 + **撞名路由缺口新发现**。

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮无 MAJOR。）

---

## MINOR（展示 / 边界一致性 / 协议冗余）

### M-1.【新发现】撞名学生教务登录后永锁管理页死胡同——前端侧 B43-04 对侧未闭合

**一句话问题**：学生账号名恰等于管理员名（默认 `admin` 或 `XUANKE_ADMIN_NAME` 自定义名）时，教务登录成功但响应不含 `adminName` 字段，App 渲染判据 `inAdmin || current === adminName`（App.tsx:256）恒命中 → 撞名学生掉进 Admin 页，五个管理 Tab 全部 403（`requireAdminSession` 判会话级 `Session.Admin`，普通会话必拒），且 403 不广播 401（client.ts 只广播 HTTP/body 401）→ 无自动回退。

**复现链（逐环铁证）**：
1. 后端 B43-04 双条件（handler.go:121）：`req.Account == adminName && 口令==管理口令` 才签发 `CreateAdmin(adminName)`（store.go:148，`Admin=true`）；口令不匹配的撞名学生走 `issueSession` 普通教务签发（handler.go:218），响应体 `{token, account}` **无 `adminName` 字段**（handler.go:229）。
2. 前端 `Login.submit` → `onLogin(data.token, data.account, data.adminName)`（Login.tsx:49），`data.adminName === undefined` → `App.login` 的 `if (adminName)` 分支跳过，本地 `adminName` 状态保持 `loadAdminName()` 值（默认 `"admin"`，App.tsx:59）。
3. `App.login` 内 `setInAdmin(account === adminName)`（App.tsx:77）：撞名学生 `account="admin" === adminName="admin"` → **true**。
4. 渲染链 `inAdmin || current === adminName ? <Admin>`（App.tsx:256）：`current === adminName` 恒真，**即使 setInAdmin 未执行也必命中 Admin**——inAdmin 只是第二道，主判据就是账号名字符串相等。
5. account-reselect effect 兜底 `if (current !== adminName) setInAdmin(false)`（App.tsx:113）：撞名学生 `current === adminName` → **不置 false**，兜底失效。
6. Admin 五 Tab 全走 `/api/admin/*` → `requireAdminSession` → `d.Sessions.IsAdmin(tok)`（store.go:176）判 `Session.Admin` 标志，普通会话 **false → HTTP 403 + body code 403**；client.ts 不广播 403（只有 401 广播），用户无提示卡在管理页骨架。
7. 逃生舱核对：点"学生端"（onBackToStudent，App.tsx:263-283）`others = accounts.filter(a => a !== adminName)` 为空 → 走完整登出清空 sessions 回登录页；**再登录又复现第 3-4 步** → 循环死锁，撞名学生**永远无法到达学生 Dashboard**（唯一手动逃生：改 localStorage `xk_admin_name`）。

**定级理由**：后端 B43-04 把撞名当真实风险并修成 MAJOR（"该学生永远无法登录（DoS）"），前端侧判据（账号名==管理员名即进管理态）未同步收口，撞名学生变成"可登录但永久锁死在 403 管理页"的等价格式 DoS。触发前提（学生账号恰等于管理员名）较罕见，故定 MINOR 而非 MAJOR；但修复方向明确——登录响应补 `is_admin` 布尔（或前端在 admin 页首屏 403 时回退学生端），修复后此缺陷连根消除。

- **严重级**：MINOR（边界前提 + 无数据破坏，但为死锁路径，用户完全无法使用学生功能）。

---

## OBSERVE（观察项，未加重）

### O-1.【延续】O-1 残件终裁执行复核——闭合，仅注释残留

- **执行面复核**：`c2ed5b5` 后 `src/components/ui/` 仅剩 7 组件（Badge/Button/Card/Input/Progress/Tabs/Toast），全量有消费（Badge 3 / Button 4 / Card 4 / Input 3 / Progress 1 / Tabs 2 / Toast 3 个文件引用）；`package.json` 依赖已收敛为 9 个运行时包（react-slot/tabs/toast + react-query/clsx/lucide/tailwind-merge/react/react-dom），react-dialog/select/switch 三行确认移除；构建产物不降（1947 modules 与 round51 相同，js/css 尺寸逐字节一致——三文件自始未进产物，删除零影响）；全 src 无 `ui/Dialog|ui/Sheet|ui/Table` 残 import。**执行闭合。** ✓
- **唯一残留**：`Admin.tsx:207` 与 `Login.tsx:222` 两处**注释文本**仍提及 "Radix Dialog"（"完整焦点陷阱迁移到 Radix Dialog 属 F6-02 后续候选"），属 F6-02 未来锚点注释，非 import。零功能影响，若后续清理注释可顺手更新措辞（不列为修复项）。

### O-2.【延续】N-2/N-3 + O-3~O-6 维持原判

- **N-2（官网 btn_type 按钮 + 冲刺按钮双形态并存）**——复证成立（Select.tsx:1090-1149），双轨设计意图。维持 MINOR 级观察延续。
- **N-3（App 401 管理员代理保护窗口闭包 current，App.tsx:148-192）**——复证成立（:174 保护窗口读闭包 current，保守方向）。维持 MINOR 级观察延续。
- **O-3（手写模态无焦点陷阱）** / **O-4（401 保护窗口数据面）** / **O-5（Admin 五 Tab 后台轮询常跑）** / **O-6（useTickingCountdown 每秒整页重渲）**——复证仍在，维持 OBSERVE。

### O-3.【延续】Dashboard key 不对称（round49 确立）维持原判

- App.tsx:286 Dashboard 挂载无 `key={account}`，Select 两处挂载点（:251/294）有 `key={account}`——不对称自 round49 确立。效果：账号切换时 Dashboard 的 `expandedDates` 折叠状态跨账号残留（纯展示灰尘，内容按新账号数据渲染，无数据串线），Select 则整体重建。**维持 OBSERVE，不升级**（折叠状态残留无功能影响，宁缺毋滥）。

### O-4.【延续】ui 组件模板残宽：CardFooter 零消费 + Button/Badge 死变体

- `Card.tsx:46` CardFooter 全仓库唯一引用是自身定义（shadcn 模板五件套残留）；Button 的 `invert/success/warning/secondary/icon` 变体、Badge 的 `default/success/warning/destructive/secondary/active` 变体零消费（实测 `variant=` 全量动态/静态传值仅 primary/outline/ghost/destructive 四值命中）。属 shadcn 组件**通用性模板**而非 O-1 同款"独立残件"（组件本体被消费、仅部分变体闲置），扩大清剿收益极低。**维持 OBSERVE，不列修复**。

---

## 重点核对结论（任务书逐条裁决）

### 1. F51-O1（删 Dialog/Sheet/Table + 三依赖）——闭合，见 O-1 复核

构建/类型全绿（tsc exit 0 + build 成功 + 产物同尺寸）、无残引（全 src grep `ui/Dialog|ui/Sheet|ui/Table` 零命中，三依赖从 package.json 确认移除）、注释残留零功能影响。✓

### 2. 上轮观察项（O-2 五组延续 / N-2~N-3 / Dashboard key 不对称 / O-1 已闭合）——逐一裁决见 OBSERVE 专节

全部维持原判，无升级。

### 3. 本轮六项新视角——逐条核实

- **Select actionLoading 错误重试后清理**：handleSelectClass（Select.tsx:86-112）finally 函数式删本课程 id（:106-110），网络失败 → toast → finally 清 Set → 按钮 `disabled={actionLoading.has(c.id) || !c.can_select}` 解锁 → 再点正常。错误重试后无残留置位、无跨课程互踩（M29-01 Set 独立跟踪）。**正确**。✓
- **Dashboard electives 缓存与 Select 同步**：两页同 key `["electives", account, sessionToken]`（Dashboard.tsx:172 / Select.tsx:56）+ 同 URL，共享 react-query 缓存，两路由互斥挂载切页零重复请求；Dashboard 静态 30s（本页只消费 begin_times/publishes 元数据，TTL 40s>30s），Select 按 2s/10s/30s 动态（消费 classes 实时名额）——**两页轮询独立、key 一致不互踩**，缓存最后一次成功数据被另一页复用时 interval 各归各实例。**正确**。✓
- **Admin 删除激活码后 SessionList 刷新**：remove（Admin.tsx:345-367）成功分支 `codesQuery.refetch()`（:357）立即拉新列表，失败分支不清缓存（保留旧数据防抖动误删展示）；generate 成功后同样 refetch（:336）；另有手动刷新按钮兜底。**正确**。✓
- **Login adminName 撞名登录后的路由**——**发现 M-1 死锁**（见 MINOR 专节）。其余子场景复核：管理员正常登录（响应带 adminName）→ `if (adminName)` 更新本地名 + setInAdmin(true)，渲染 Admin 五 Tab 正常；普通学生登录（响应无 adminName）→ 若账号≠adminName 走 Dashboard 正常。**仅撞名分支存在缺陷**。
- **App UNAUTHORIZED_EVENT 监听器清理**：App.tsx:148-192 effect 依赖 `[adminName, inAdmin]`，addEventListener + return removeEventListener 成对（:189-190），App 单次挂载不重挂，adminName/inAdmin 变化时重建监听器刷新闭包。**多账号频繁 401 事件是单监听器接收多次 dispatch，绝不堆积监听器**。**正确**。✓
- **components/ui import 隔离**：7 组件全消费、无跨组件残引（grep `ui/<name>` 引用计数全 >0）、无 barrel 无未用 import（tsc noUnusedLocals 门禁已证）。仅模板残宽见 O-4。**正确**。✓

---

## 结论

- **MAJOR 0 / MINOR 1（M-1 新发现）/ OBSERVE 4（O-1 复核闭合 + 延续 + Dashboard key + 模板残宽）**。tsc exit 0、build 成功、工作树零变更。
- 最需主控留意 3 条：
  1. **M-1 撞名学生锁死管理页**（本轮唯一实质产出）：后端 B43-04 已修复撞名学生 DoS（handler.go 双条件），前端 `current === adminName` 渲染判据（App.tsx:256 + login:77）未同步收口——撞名学生教务登录后恒进 Admin 页、五 Tab 全 403、403 不广播 401、点"学生端"完整登出后重登复现 = 循环死锁，该学生永远无法使用学生功能。修复方向：登录响应补 `is_admin` 布尔或 admin 页首屏 403 回退学生端。
  2. **F51-O1 执行闭合**：三组件 + 三依赖确认移除，tsc/build 双绿、产物尺寸逐字节不变、无残引，仅两处注释残留（零功能影响）。
  3. **本轮其余新视角全部核实无果**（actionLoading 清理正确 / electives 双页 key 同步正确 / 删除激活码 refetch 正确 / 401 监听器单监听不堆积 / ui import 隔离全绿），N-2/N-3/O-2~O-6/Dashboard key 不对称全部维持原判。
