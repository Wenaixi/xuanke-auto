# R115 前端只读审查报告

## 审查对象与方式

- **HEAD**：`4c32a81`（docs(review): R114 双 findings + 收尾总结）
- **审查方式**：全程只读（零仓库文件修改），Grep/Read 逐点实证 + 前端回归以 `npm run build`（tsc -b 真校验）为准；断言脚本全量跑通。
- **聚焦范围**：M-1 第五十一轮闭合复核（shouldDeferSave 四消费点与 echoedRef 置位族）、OBSERVE-93-01 第五轮家族册（裸按钮残余面 + Toast Close 二勘定案 + F93-01 ring）、F93-01 第二十三轮零漂移双实证、无障碍纵深 + 弹窗族焦点管理、OBSERVE-112-02 react-query 函数式 refetchInterval、新契约角度（Select 选课大厅交互纵深）、格式卫生快扫、契约 20 轮次前缀残留扫描。

## 分级发现

**CRITICAL：无**
**MAJOR：无**
**MINOR：无**

## OBSERVE

### 延续
- **OBSERVE-93-01（第五轮家族册复核，维持）**：参与 7 个带文本裸按钮（Admin :417「收起」/ :425 复制 / :441「刷新」/ :452「重试」/ :651 / :661 引擎二选一 / :701「重试」）+ Toast.tsx:100 Close 逐点位确认——全部裸按钮都在 DOM、可 Tab 聚焦、绑定 onClick，键盘链路完整；缺口纯 display 维度（读屏无可访问名）。全仓 17 处裸按钮清点（含 Dashboard:67 CollapseSection 带 aria-expanded/aria-controls/useId 按钮）与家族册记录**零增零减**。**651/661 引擎二选一带强 active 态仍为优先修复面**（白底黑字 active 态 + 切换引擎的功能性视觉差异显著，读屏完全无感知）。
- **OBSERVE-76-03（二勘定案后措辞复核，确认成立）**：ToastClose X 图标按钮无可访问名。实证链完整——Radix ToastClose 源码（`@radix-ui/react-toast/dist/index.js:570-583`）为裸 `Primitive.button`，不注入任何 aria-label（与二勘定案结论一致）；lucide X 图标默认 `aria-hidden="true"`（`lucide-react` is/ariaHidden 源码:65）。「无自动注入标签、按钮无可访问名」措辞属实。
- **OBSERVE-112-03 / 111-01**：无新证据，维持盯守。

### 新增
- **OBSERVE-115-01**：三处裸 div 弹窗（Select 退选弹窗 :1201-1250 / Login 激活弹窗 :223-316 / Admin 删除弹窗 :210-283）具备完整最小语义门（role=dialog / aria-modal / aria-labelledby / Esc 关闭且各自绑定在飞守卫 / 取消按钮 autoFocus），但**均无焦点陷阱**——Tab 可穿出到背景页面；专注项：注释已声明「完整焦点陷阱迁移到 Radix Dialog 属后续候选」，为刻意取舍而非失察。优先修复面：Select 退选弹窗（黄金期用户高频交互且聚焦面最小）。
- **OBSERVE-115-02**：无需报告的行为缺陷闭合状态下无新增。本轮纵深（选课大厅搜索/筛选/排序/徽章）零发现。

## 必查项逐条结论

### 1. M-1 第五十一轮闭合——通过
- `shouldDeferSave`（`src/lib/targetGuard.ts:64`）签名 `(stateData, hasSelected, echoed)`：`undefined→true`；`echoed→false`；否则 `(courses?.length??0)>0 && hasSelected`。**全仓恰 4 消费点**（:509 flush / :597 handleBack 首轮 / :605 handleBack while / :699 防抖回调），四者**逐字符一致**传第三参 `echoedRef.current`。
- echoedRef 置位三路径：:200（account 切换复位 false）/ :240（首帧 courses 空置 true）/ :297（非空合并完成置 true）。首帧不置位边界：:229（已真短路）/ :234（stateData===undefined 提前 return）/ :247（pubs 空 return）/ :319（清理 effect 未回显短路）——完整。
- **target-guard-check.ts 18 断言全绿**（含「首帧未到+已回显→仍推迟」「courses 非空+全清空→放行」「无变更→返回原引用」 等关键分支）。

### 2. OBSERVE-93-01 残余面复核（第五轮）——维持
见上文 OBSERVE 延续段。7 个带文本裸按钮 + Toast Close 全部逐点位复核，清单零漂移。

### 3. F93-01 第二十三轮——通过
- Button.tsx:42 ring：`focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]` 逐字符在位。
- `git log --oneline 1351fa4..HEAD -- web/`：**空输出**；`git diff --stat 1351fa4 HEAD -- web/`：**空**（1351fa4 后全仓库 23 文件变更全为 archive/ 报告 + backend/，web/ 零代码漂移）。第二十三轮连续零漂移，COUNT=1 延续。

### 4. 无障碍纵深延续——通过
- role="alert" 三处：Dashboard:320 / Login:182 / Login:273 全部在位。
- ring 令牌族：Tabs.tsx:29 / Admin switch :583 / Login 密码切换 :173 三处同款 `focus-visible:ring-2 ring-white/60` 在位。
- 弹窗族 Esc/焦点管理纵深成果并入 OBSERVE-115-01（最小语义门完整、焦点陷阱缺失为注释声明的刻意取舍）。

### 5. OBSERVE-112-02——通过
- Dashboard.tsx:140-145 `/state` 轮询：函数式 `refetchInterval` 读 `query.state.error/data.window_closed`，非闭包捕获；3s ↔ 30s 切换、恢复成功即回 3000。logs 查询 :156-161 读闭包 state 有注释声明「/state 每 3s 刷新→重渲染→用最新闭包重调度，闭包不陈旧」成立。Select /electives :58-82 与 /state :148-155 同款函数式，且 Select 主查询通过 `queryClient.getQueryData` 缓存读 /state 规避 TDZ（:77）——三查询族全部无闭包陈旧问题。

### 6. 新契约角度（Select 选课大厅交互纵深）——通过
- 搜索/筛选为渲染期纯函数派生（filteredClasses 每次渲染独立计算），无副作用状态。
- 搜索覆盖课程名/教师名/教室名三字段（string 类型 toLowerCase 安全）。
- 「仅看有余量」豁免 max_count===0（名额未公布算有余量）；排序键 `remaining = max_count - selected_count`（非报名数）；max_count=0 映射 0 最紧张——筛选/排序/徽章/进度条四处与后端 IsClassFull（`MaxCount>0 && SelectedCount>=MaxCount`）同源，无一处分叉。
- 徽章状态机 isSelected > isFull > unannounced > remaining<=5 > 名额充足，分支完备无漏洞；unannounced 时进度条 value=0/max=1 恒空条，aria-valuenow 如实反映（:1101-1105）。
- btn_type 渲染契约（1=退选/2=报名/其他不渲染）+ can_select 双守卫 + title 悬浮提示与官网逆向一致。
- 手动操作 `actionLoading: ReadonlySet<number>` 按课程独立在飞互斥（:52），报名/退选 finally 函数式删除只清自己——并发互踩隐患已根除。

### 7. 格式卫生快扫——通过
- 「相邻 JSX 元素同行」扫描（`></*><`）零命中；无制表符、无行尾空格。
- 四个路由文件统一 UTF-8 BOM（Admin/Dashboard/Login/Select 抽查一致），快照内一致无可读性影响，维持不报。
- **契约 20 轮次前缀残留扫描：全前端零命中**（"第 N 轮"/R1xx/F9x/B9x 匹配为空）。

## 验证表

| 命令 | 结果 |
|------|------|
| `git log --oneline 1351fa4..HEAD -- web/` | 空（零漂移实证 1） |
| `git diff --stat 1351fa4 HEAD -- web/` | 空（零漂移实证 2） |
| `npm run build`（tsc -b + vite build） | 通过，1948 modules，415ms，exit 0 |
| `node --import jiti/register scripts/target-guard-check.ts` | 18 断言全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | 6 断言全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | 5 断言全绿 |
| `node scripts/audit.mjs` | 全部通过（visual 表面协调一致） |
| grep `<button` 全仓清点 | 17 处与家族册记录一致，零增零减 |
| 契约 20 扫描（第N轮/R1xx/F9x 等） | 零命中 |

## 已核无缺陷清单（关键项）

- shouldDeferSave 纯函数判据族（undefined 无条件推迟 / echoed 稳态放行 / courses 空放行）与四消费点、echoedRef 置位/复位族完整，TDD 18 断言守护。
- Button/TabsTrigger/Admin switch/Login 密码切换四处 focus-visible ring 令牌族零回归。
- Toast Close 二勘定案实证链完整（Radix 源码无注入 + lucide aria-hidden）。
- 三弹窗 Esc/autoFocus/role=dialog 最小语义门 + 各自在飞守卫（actionLoading/deleting/activating）防误关。
- 删除管理员自己退管理态三处收敛（App onDeleted :175 / onUnauthorized lostAccount :213 / onBackToStudent :312 全清 adminToken+saveAdminToken，含 logout :121）。
- 手动报名/退选 Set<number> 在飞互斥 + 双 invalidate（electives/state）刷新语义。
- copy 剪贴板三级降级链（clipboard API → execCommand 兜底 → toast 手动抄录展示完整激活码）。
- 空态/加载/错误三分支全 Tab 覆盖；失败态重试出口 Admin 五 Tab + Dashboard logs refetch 齐备。
- 日期分组链（begin_date 自带优先 → /electives 映射兜底 → "未知"恒排最后）与倒计时 begin_times 兜底跨页一致（Dashboard:191-196 / Select:767-772）。

## 结论

R115 前端审查：**零 CRITICAL/MAJOR/MINOR**，维持连续五十多轮稳定期纪律。M-1 第五十一轮闭合确认（四消费点第三参逐字符一致 + echoedRef 置位三路径完整 + 18 断言全绿 + `shouldDeferSave(` 恰 4 消费点）。OBSERVE-93-01 第五轮家族册复核：残余面清单零增零减、缺口纯 display 维度，Toast Close 二勘定案措辞确认成立（Radix 无自动注入标签实证）；651/661 引擎二选一仍为优先修复面。F93-01 第二十三轮：web/ 自 1351fa4 零代码漂移双实证。本轮新增 1 条 OBSERVE（115-01 弹窗族无焦点陷阱为注释声明取舍）。契约 20 零残留，格式卫生通过。