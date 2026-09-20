# round43 前端修复报告

日期：2026-09-20
修复人：前端修复代理（web/ 目录范围，未触碰 backend/——工作区 2 个 backend findings 为并行修复代理在用，本轮未 commit 它们）

## 回归基线

- `cd web && npx tsc -p tsconfig.app.json --noEmit`：exit 0
- `cd web && npm run build`：通过（vite build 绿，产物落 backend/web/dist）
- 纯函数断言脚本 `node --import jiti/register scripts/target-guard-check.ts`：全绿（jiti 随项目自带，需 `/register` 后缀；`--import jiti` 直接跑会 ERR_MODULE_NOT_FOUND，此前报告记录一致）

## F43-M1（MAJOR）：全清空目标永不落库——shouldDeferSave 增 hasSelected 判据

- 根因：`shouldDeferSave(stateData)` = `stateData===undefined || courses 非空`，把「用户显式清空全部目标」与「数据缺席/慢首帧」一视同仁。用户点选又全取消（rev>0, selected={}）时回显 effect 的 `rev>0 && !anyHas` 守卫承认清空语义绝不合并旧目标，但防抖回调 / flushTargets / handleBack 等待三闸仍按 `courses 非空` 打回置脏、无自愈信号 → 清空永不 PUT，返回后重进被旧目标复活（静默撤销）
- 修复：`shouldDeferSave(stateData, hasSelected: boolean)` = `stateData===undefined || (courses 非空 && hasSelected)`——首帧携带旧目标但用户一个都没选 = 清空意图确凿（回显 effect 已承认），放行 PUT []，绝不把清空当"待回显"。三处消费点改签名：
  - 防抖回调（Select.tsx 671）：`shouldDeferSave(stateDataRef.current, selectedCount > 0)`（selectedCount 渲染闭包，effect 依赖含 selected 保证新鲜）
  - flushTargets（499）：`shouldDeferSave(stateDataRef.current, latestSelectedCount > 0)`（复用既有 latestSelectedCount 变量）
  - handleBack（584 初始判定 + 590 while 轮询）：消费时刻抽局部函数 `hasSelectedNow()` = `Object.values(selectedRef.current).reduce((n,a)=>n+a.length,0) > 0`（handleBack 无渲染闭包，两处同源）
- 测试形态（TDD 先红后绿）：`target-guard-check.ts` 追加/改造 shouldDeferSave 六条断言——首帧未到+有选中→true / 首帧未到+全清空→true / courses 空+有选中→false / courses 空+全清空→false / courses 非空+有选中→true / **courses 非空+全清空→false（本轮新绿）**。先改签名红（未实现时类型不匹配编译失败）→ 实现后全绿
- commit：`28116f3 fix(select): shouldDeferSave 增 hasSelected 判据，全清空目标显式清空放行（F43-M1）`

## F43-N1（MINOR）：手动报名/退选按钮改以 btn_type 为唯一渲染判据

- 根因：按钮区整体绑 `t.in_date_range || stateData?.window_opened`。官网真实契约：btn_type 1=退选/2=报名/其他值不渲染操作按钮，can_select 决定可点性（disabled + title），与窗口开关无关——平台在窗口未开时照样下发 btn_type=2+can_select=false（title="不在选修报名时间范围内，无法选课！"）。本项目隐藏了官网按钮，开窗瞬间窗口信号缺失（识别槽未建立/in_date_range 刹那 false）时手动抢课通道被锁死
- 修复：官方按钮块拆成 `{c.btn_type === 1 && (退选按钮)}` + `{c.btn_type === 2 && (报名按钮)}` 无条件渲染（disabled 与 title 逻辑保持 `!c.can_select` 不变）；本项目特冲刺/预选按钮保留原两形态（窗口开时 ghost 小按钮"设为后台冲刺目标"、窗口关时 primary"预选目标"），外层窗口条件只决定它自己。删了原 1080 行整个三元条件块
- 测试形态：逻辑走查（开窗前 btn_type=2+can_select=false → 置灰「报名」+title 提示，与官网一致；开窗后正常可点）+ `npx tsc -p tsconfig.app.json --noEmit` exit 0 + `npm run build` 绿
- commit：`05e3797 fix(select): 手动报名/退选按钮以 btn_type 为唯一渲染判据，窗口信号缺失不隐藏官网按钮（F43-N1）`

## F43-N2（MINOR）：激活失败缓存票据清空

- 根因：Login.tsx 激活 catch 的 else 分支（激活码错误等非「过期/已用」）只 setActivateError 不清 pendingTicket。服务端 handleActivate 先 ConsumeTicket 再校验激活码——失败也销毁票据；前端滞留旧票重试会弹误导文案「本账号已开通」
- 修复：else 分支 setActivateError 后补 `setPendingTicket("")`（与过期/已用 `:88`、激活成功 `:81`、登录成功 `:231/:300` 处清空点一致），error 文案保持。清空后下次 1001 覆盖新票，UI 永不显示已消费票据为可用态
- 测试形态：逻辑走查（票据生命周期短、无可自动断言路径）+ 类型校验 + 构建绿
- commit：`43fbd4d fix(login): 激活失败即清空票据，杜绝已消费票据滞留误导（F43-N2）`

## F43-N3（MINOR）：Tailwind 4 animate-in 动效类缺失

- 根因：全站 Dialog/Sheet/Toast/Modal 声明 `animate-in fade-in duration-150`/`zoom-in-95`/`slide-in-from-bottom-full` 等 tailwindcss-animate 工具类，但 Tailwind 4 原生不含且未装插件——构建产物 dist CSS `grep -c "animate-in"` = 0（已实证），全部动效静默失效
- 修复：
  1. `npm install -D tailwindcss-animate`（Tailwind 4 通过 `@plugin` 指令加载）
  2. `web/src/styles/global.css` 顶部 `@import "tailwindcss"` 之后加 `@plugin "tailwindcss-animate";`
- 回归验证（先红后绿实证）：红——dist CSS `grep -c .animate-in` = 0；绿——装插件后 `npm run build` 产物 dist CSS grep 命中 `.animate-in` 规则 1 处、`-c` > 0，`npx tsc -p tsconfig.app.json --noEmit` exit 0
- package.json 的 devDependencies 新增 tailwindcss-animate（预期改动，`^1.0.1`）
- commit：`b3e5cdd fix(ui): 启用 tailwindcss-animate 插件，修复全站 animate-in 动效静默失效（F43-N3）`

## 收尾确认

- 每处缺陷独立 commit（M1 / N1 / N2 / N3 共 4 个），未 push
- 全量回归：`cd web && npx tsc -p tsconfig.app.json --noEmit` exit 0 + `npm run build` 绿 + `node --import jiti/register scripts/target-guard-check.ts` 16 条断言全绿
- 工作区 web/ 目录无残留改动；未跟踪文件为仓库根 pre-existing 文件（CODE_OF_CONDUCT 等）与 round42/round43 的 findings 报告
- 未触碰 backend/ 任何文件（dist 产物由 npm run build 自动生成，属 web 构建输出）