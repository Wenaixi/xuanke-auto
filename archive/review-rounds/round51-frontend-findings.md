# round51 前端审查原始发现

> 审查基线：master @ `f49d348`（round50 总结落盘：半百里程碑、后端 2 修、前端零修）。工作树干净（仅根目录 5 个未跟踪社区文档）。
> 校验：`cd web && git status --short` 干净（仅根目录 5 个 `??` 社区文档）；`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`cd web && npm run build` **成功**（tsc -b + vite 全绿，1947 modules，产物 416.74 kB js / 44.73 kB css，构建后 web 目录零 git 变更）。范围 `web/src/` 全部 `.ts/.tsx`，绝对只读。
> 后端契约对照：`backend/internal/api/handler.go`（handleAdminStats 500 对齐 d7ac801 / handleState / handleActivate 1001+ticket / handleLogin adminName）。
> 上轮（round50）无前端修复项，本轮为跨轮第 6 次复证 + **O-1 五件残件终裁**（主控依约提请）。

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮无 MAJOR。）

---

## MINOR（展示 / 边界一致性 / 协议冗余）

（本轮无新增 MINOR。）

- **N-2 / N-3 / O-3~O-6 延续**（见 OBSERVE 专节，历轮已确立，无新的功能影响）。

---

## OBSERVE（观察项，未加重）

### O-1.【终裁建议】五件零消费残件 → 支持"删除"方针，但删除清单必须补 `@radix-ui/react-dialog`（主控两行遗漏）

**一句话结论**：跨轮第 6 次复证（round46 O-1 起）零消费状态不变；对主控倾向的"删除 Dialog/Sheet/Table + react-select + react-switch"终裁**独立评估后支持删除**，同时给出一个精确补强——**`@radix-ui/react-dialog` 必须同批列入删除**，否则删 Dialog/Sheet 后它变成第四个零消费残件（且其专属间接包 react-remove-scroll/aria-hidden 全部零消费继续污染）。

**独立风险面评估（任务书要求逐条回答）**：

1. **删除 ui/Dialog/Sheet/Table 是否有隐藏消费点？** —— **无**。
   - Grep 全 src + archive：三组件运行时消费点仅存在于历史 plan 文档与审查报告**文本**中（`archive/superpowers/plans/*.md`、`archive/review-rounds/round4X`），`web/src/src` 内零 import（连 ui 目录内其他组件也不引用）；`components/ui/index.ts` barrel 不存在。
   - 构建产物实测：`backend/web/dist/assets/index-Lj3J1gwb.js` grep `Dialog|Sheet|Table|Switch` 计数 **0**——Vite 只打包可达模块，三文件自始未进产物，删除对产物零影响（删除前后产物 hash 理论不变）。
   - node_modules 内自引用：无（Dialog/Sheet 各自直接 import react-dialog，组件间无交叉引用）。
   - 未来升级路径：git 历史 `4f45422`（zero-radius dialog/table/tabs）/ `5018ebf`（deep shadcn + sheet/toast）含全量源文件可随时恢复；shadcn CLI 也可一键重生。恢复成本（git checkout / 重新生成）显著低于保留成本（3 份模板代码 + 2 个依赖 + 数十个提升间接包常驻 node_modules）。
2. **npm 移除 react-select + react-switch 是否破坏 react-dialog/react-toast 的 peer 依赖？** —— **不会**。
   - 实测三包 package.json：peerDependencies 只声明 `react/react-dom/@types(optional)`，**不互相 peer 依赖**；lock 查询 "DEPENDENT:" 为空（无任何包依赖 select/switch）。react-dialog/react-toast 各自 deps 均为独立 `@radix-ui/react-*` 子包（react-portal/dismissable-layer/presence/primitive/collection/visually-hidden 等），版本与 select 的提升副本相同（如 toast 用 react-collection 1.1.15 与 select 一致）。
   - npm install 语义：卸载 select/switch/dialog 后，仍被 toast/tabs 依赖的间接包（collection/direction/id/visually-hidden/dismissable-layer/portal 等）因有消费者被保留；只卸无消费者的专属包（react-popper/react-arrow/react-use-size/react-use-previous/react-number + react-dialog 专属的 react-remove-scroll/aria-hidden）。TypeScript `moduleResolution: "bundler"` 从 `node_modules/@radix-ui/react-tabs/dist` 内部解析，不依赖顶层提升布局。**peer 断裂风险 = 0，编译不会断。**
3. **若保留（加注释"未消费，供未来 Radix 迁移"）的评估**：历轮 5 次复证无任何使用倾向；Login.tsx:222 注释"完整焦点陷阱迁移到 Radix Dialog 属 F6-02 后续候选"是唯一未来锚点，但届时可随时从 git/shcn 恢复。保留成本（维护 3 份模板 + 3 依赖 + 提升包）> 恢复成本。**删除更优**，与主控倾向一致。

**精确删除清单（给修复代理）**：
1. 文件：`web/src/components/ui/Dialog.tsx`、`web/src/components/ui/Sheet.tsx`、`web/src/components/ui/Table.tsx`
2. `web/package.json` 删 **三行**（非两行）：`:13 "@radix-ui/react-dialog": "^1.1.23"`、`:14 "@radix-ui/react-select": "^2.3.7"`、`:16 "@radix-ui/react-switch": "^1.3.7"`（react-dialog 仅被将删的 Dialog/Sheet 消费）
3. `cd web && npm install` 同步 `package-lock.json`（npm 自动摘除对应条目与无消费者提升包）
4. 验证：`cd web && npx tsc -p tsconfig.app.json --noEmit` exit 0 + `cd web && npm run build` 成功 + 产物无回归
5. **保留**三处手写模态最低语义门（role=dialog/aria-modal/aria-labelledby/Esc/autoFocus，Select.tsx:1175-1224 / Admin.tsx:210-284 / Login.tsx:223-315），本轮不迁移（最少改动）。

- **严重级**：OBSERVE（纯依赖清洁度 + 维护成本，零功能影响），交付主控决策，修复由后续代理执行。

### O-2.【延续】N-2/N-3 + O-3~O-6 维持原判

- **N-2（官网 btn_type 按钮 + 冲刺按钮双形态并存）**——复证仍成立（Select.tsx:1089-1149，`btn_type===1/===2` 渲染官网按钮、窗口信号决定冲刺按钮形态），双轨设计意图。维持 MINOR 级观察延续。
- **N-3（App 401 管理员代理保护窗口闭包 current）**——复证仍成立（App.tsx:148-192，:174 保护窗口读闭包 current；保守方向）。维持 MINOR 级观察延续。
- **O-3（三处手写模态无焦点陷阱）**、**O-4（401 保护窗口数据面）**、**O-5（Admin 五 Tab 后台轮询常跑：codes/stats/logs 5000 + accounts 10000 无失败态/关闭态降频）**、**O-6（useTickingCountdown 每秒整页重渲）**——复证仍在（Admin.tsx:319/705/879/791），维持 OBSERVE。本轮新视角"codes 5s 轮询后台开销"即 O-5 的复证：激活码管理不因选课窗口关闭失效、5s 轮询合理，量级可忽略，不加重。

---

## 重点核对结论（任务书逐条裁决）

### 1. F50-M1（stats 目标数失败 body code 1→500）——前端无感知，无回归

- **变更面**（`git show d7ac801` 逐行确认）：仅 `backend/internal/api/handler.go` `handleAdminStats` 一处 `writeJSONStatus(body code 1→500)`，附注释"前端契约只读 body code!=0，行为不变"。
- **前端契约核对**：`client.ts` `api()` **从不读 HTTP status**，只 `r.json().code` 判 `401`/`0`/非0（:71-97）——stats 失败路径拿到 body `code=500` 抛 `ApiError(500, "统计目标数失败: ...")`；StatsTab（Admin.tsx:701-773）**不 catch 不消费 ApiError.code**，react-query 记 error 后 `s` 恒 undefined → 渲染"加载中..."（:742）。失败码从 1→500 对前端唯一影响是 `ApiError.code` 数值变化，StatsTab 不读它。**无感知、无回归**。✓
- **边界注**：StatsTab 无失败态分支（`!s ? "加载中..." : rows`，失败永不消失）是历轮已确立的行为（writeJSON 失败同 500），非 F50-M1 引入，不报。

### 2. 上轮契约族复证

- **F48-M1（Toast 定位）**——view 复核闭合：布局类并入 viewport className（Toast.tsx:87 `fixed bottom-4 right-4 z-50 ...`）；`toasts.map` 在 children 之后、Viewport 之前（:52-86）；round49/50 已实证 Radix portal 机制与构建产物动画类。闭合。✓
- **F46-F1/F2（Dashboard 三态 + 双查询失败态降频）**——复核闭合：/state 三态（Dashboard.tsx:342-345 状态卡 + :728 手机悬浮栏，`window_closed ? 已关闭 : window_opened ? 已开放 : 待命中`）；/state（:140-145）/logs（:156-161）refetchInterval 顶部 `error||status==="error"→30000`、关闭态→30000、常态 3000。闭合。✓
- **B45-N1（401 双广播收敛）**——复核闭合：client.ts:64-69 HTTP 401 前置广播 + :75-94 `if (r.status !== 401)` 兜 HTTP 200+body 401 旧形态，三种形态各单次广播。闭合。✓
- **F43 四件套**——复核闭合：M1 `shouldDeferSave(stateData, hasSelected)` 三消费点同源（Select.tsx:499 flush / 589+595 handleBack / 676 防抖）；N1 `btn_type===1/===2` 双守卫 `disabled={... || !c.can_select}` + title 回退文案；N2 激活票据五清票点（Login.tsx:80-81/88/95/233-235/303-305）；N3 tailwindcss-animate 插件（global.css:2 `@plugin`）。闭合。✓
- **targetGuard 三函数**——与 Select 消费点逐字符同步（shouldDeferSave :57-62 纯数据判据、selectedHasStalePublish :9-19、cleanStaleSelected :26-43 无变更返回原引用）。✓

### 3. 本轮五项新视角——逐条核实

- **Select 目标回显"后端目标与前端 selected 均空"首帧边界（空态是否误触假清空守卫）**：courses 空 = 确证后端无旧目标 → 回显 effect（Select.tsx:234-241）置 echoedRef+echoDone、不写 selected → 用户点课（pick→rev=1）→ 防抖 400ms 后消费时刻 `shouldDeferSave(stateData, selectedCount>0)`：stateData 已到 + courses 空 → **false 放行** → 后续守卫全过 → 正常落库。用户纯浏览（rev=0）→ flushTargets `latestRev===0` return（:490）与防抖入口 `rev===0 return`（:646）双层短路，**绝不 PUT []**。假清空守卫（"selectedCount>0 却 targets 空"）在空态下 selectedCount=0 根本不命中。**空态绝不误触**。✓
- **Dashboard dateGroups 在 publish 重建后 courses 旧 publish_id 的临时错组**：分组键优先 CourseStatus 自带 `begin_date`（调度器随目标持久化，契约"关闭≠时间消失"）→ 兜底 /electives pubById 映射 → 兜底"未知"组。发布重建瞬间（开窗 publishes 清空又恢复、publish_id 全变）：旧目标课程若自带 begin_date **仍按自带日期正确分组**；仅"老库未迁移 + 无 begin_date + 发布重建"叠加极端场景才临时落"未知"组（带"窗口已关闭，发布信息不可用"标签），且 /state 2s 轮询立即纠正。**保守正确方向（不编造日期），非铁证 bug，不报**。✓
- **Admin 激活码表格窗口关闭后的刷新（codes 5s 轮询开销 vs StatsTab 依赖）**：codesQuery（:316-320）与 statsQuery（:702-706）相互独立无依赖；激活码生成删除不因窗口关闭失效、5s 刷新合理；窗口关闭后三个管理接口 5s/10s 常轮的请求成本（每 5s 4 个轻接口）量级可忽略。**延续 O-5，不加新问题**。✓
- **Login 1001 激活模态在 session 过期后的重登（ticket 与 account 双失效）**：场景—管理员会话被剔（onUnauthorized 清 sessions）后 1001 模态仍开（Login 永挂载）。路径自愈：激活成功 → onLogin 重写 sessions 复活；ticket 过期（5 分钟）→ "激活票据无效或已过期，请取消后重新登录即可进入"引导 → 重登直进；account 失效由 login 覆盖。**双失效均无死锁**，与 F43-N2 五清票点闭环。✓
- **App accounts useMemo 快照式三连后的引用稳定性**：`accounts = useMemo(() => Object.keys(sessions), [sessions])`（App.tsx:63）——只在 sessions 引用变时重算；sessions 引用仅被四处快照式 `saveSessions(next); setSessions(next)`（login/logout/onUnauthorized/onDeleted）改变，每次都是有意变更；onDeleted 幂等 `if (snap[acct]===undefined) return` 防无效新引用。**无"sessions 未变而 accounts 抖动"路径**，account-reselect effect（:101-114）由 accounts 变化正确驱动。**引用稳定**。✓

---

## 结论

- **MAJOR 0 / MINOR 0（新增）/ OBSERVE 2（O-1 终裁 + 延续）**。无功能级错误，tsc exit 0、build 成功、工作树零变更。
- 最需主控留意 3 条：
  1. **O-1 五件残件终裁（本轮唯一实质产出）**：跨轮第 6 次复证，独立风险面评估完成（零消费点无隐藏、peer deps 不裂、构建产物本就不含），**支持删除**；精确清单需含**三行**依赖（补 `@radix-ui/react-dialog`——删 Dialog/Sheet 后它即第四个零消费残件），三组件文件 + `npm install` 同步 lock，保留手写模态最低语义门，验证 tsc/build 双绿。
  2. **F50-M1 前端无感知**（client.ts 只读 body code、StatsTab 不消费失败码）——本轮核实为闭合，无回归。
  3. **本轮五项新视角全部核实无果**（空态不误触假清空守卫 / dateGroups 重建错组为保守方向 / Admin 轮询延续 O-5 / Login ticket+account 双失效自愈 / accounts useMemo 引用稳定），N-2/N-3/O-3~O-6 全部维持原判。