# 第 18 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（main.go/api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/cmd/web-embed）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI。
> 审查方式：两个 opus 权威子代理并行只读审查（后端 + 前端独立通道，均附"宁缺毋滥 + 文件行号 + 触发场景 + 已知观察项去重"模板），发现全部经主通道逐一读码推演定案；修复独立 commit。
> 本轮结论：**前端 1 项 CRITICAL（F18-01 Select.tsx 死代码致前端构建中断）+ 1 项 MAJOR（F18-02 配置加载前保存覆盖真实配置）+ 1 项 MINOR（F18-03 发布重建 Tabs 悬空）**；后端报告尚在途（本文件后端段落随后端报告到达补全）。

---

## 一、前端发现与修复状态（第 18 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F18-01** | CRITICAL | **Select.tsx 死代码常量致前端构建彻底中断**：F17-02 把假清空守卫判据改为消费时刻实时判据后，渲染期常量 `publishesMissing`（Select.tsx:284）遗留成死代码，且其 TDZ 初始化引用**声明于其后的** `publishesRef`（342）/`selectedCount`（400），直接触发 `tsc -b` 5 个编译错误（TS6133/TS2448/TS2454）、`npm run build` 退出码 2——CI（ci.yml）与 Tag 发布流水线（release.yml）必然红，发版会把旧前端嵌进单二进制。**修复**：删除该常量声明与过时注释，功能零影响。`npm run build` 实证红灯→绿灯（407.71 kB）。**警示**：项目根 `tsc --noEmit` 是 references 空壳不报错，只有 `npm run build`（`tsc -b`）真校验——此前"tsc 通过"验收结论失真，前端回归一律以 `npm run build` 为准 |
| **F18-02** | MAJOR | **系统配置加载前保存会用初始空值覆盖真实配置**：ConfigTab（Admin.tsx:409-464）在 `configQuery` 返回前保存按钮即可点，表单仍为初始值（`baseUrl=""`/`model=""`/`openTime=""`/`activationOn=true`），保存会整体覆盖生效配置——`open_time` 被清空 = B11-A1 明确语义"显式清空开放时间"（调度器挂起提交）、Vision 配置清空、激活码机制误开。**修复**：按钮 `disabled={saving || !loaded}` + `save()` 入口 `!loaded` 守卫 + 加载中提示文案，双保险 |
| **F18-03** | MINOR | **选课大厅 Tabs 非受控 defaultValue 发布重建后悬空**：`<Tabs defaultValue={String(tabs[0].publish_id)}>` 只在首次挂载生效；发布集合整体重建（F16-01 记载的开窗瞬间平台清空又恢复、publish_id 全变）后激活 value 的 Trigger 消失，Radix Tabs 无 fallback，主内容区空白直到用户手点。**修复**：改受控 `value={activeTab ?? String(tabs[0].publish_id)}` + `onValueChange`，发布重建自动回落首个 Tab 绝不悬空 |

## 二、后端发现与修复状态（第 18 轮）

> 后端 opus 审查 agent 尚在途，报告到达后补全本节并同步独立 commit。

## 三、修复细节（本轮前端 3 项，独立 commit）

- **F18-01**（commit c97eb0c）：Select.tsx 删除死代码 `publishesMissing` + 过时 F16-01 注释
- **F18-02 + F18-03**（commit 0c46ef5）：Admin.tsx 配置加载守卫 + Select.tsx Tabs 受控化

## 四、回归证据（提交时点）

- `cd web && npm run build`（`tsc -b` + `vite build`）— 通过（407.93 kB / gzip 122.21 kB）
- 后端回归随后端 agent 报告到达后补跑

---

## 提交索引（本轮前端 2 个 commit，后端段待补）

```
c97eb0c fix(web): 第18轮F18-01 CRITICAL Select.tsx死代码致前端构建中断（publishesMissing遗留TDZ引用, tsc -b 5错, 删除即绿）
0c46ef5 fix(web): 第18轮F18-02 MAJOR配置加载前保存覆盖真实配置 + F18-03 MINOR发布重建Tabs悬空
```

## 下轮待办

- 后端报告到达后核实 + 独立 commit + 补全本文件后端段
- 全量回归（backend `-race` + web `npm run build`）→ review-round18.md 终稿 → CLAUDE.md 沉淀 → memory → 18/38 收官宣言