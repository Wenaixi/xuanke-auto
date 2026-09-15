# 第 31 轮全模块审查记录（2026-09-16）

> 审查范围：backend 全部模块（scheduler 1767 行 / handler 1187 行 / manager 278 行 / client 697 行 /
> store 467 行 / session 231 行 / runtime 65 行 / config 160 行 / secure 87 行 / db 80 行 / main 194 行 /
> embed 52 行 / 全部测试 5000+ 行）+ web 全部模块。两个只读子代理并行产出发现，主 gate 逐条核实
> （读源码 + 推演真实触发路径）。
> 本轮后端 1 项 MINOR 确认修复（B31-01），前端 4 项确认修复（Select-31-01 MAJOR + Select-31-04 /
> App-31-02 / Admin-31-03 MINOR），另按主人指示剥离全仓库注释轮次前缀 140 处并写入 CLAUDE.md。
> 说明：review31-backend 报告两次截断，主 gate 发消息索要完整正文均成功送达。

## 后端（1 项，确认修复）

### B31-01（MINOR）DELETE /api/admin/accounts 无 body 被 requireJSONBody 403 拒——与 codes DELETE 不对称
**缺陷**（review31-backend）：B7-M8/M9 只给 `codes` 的 DELETE 放行空 body（标准 REST 客户端
DELETE 默认无 body、无 Content-Type → 被 403 拒），而账号删除（`DELETE /api/admin/accounts`）
仍强制 `requireJSONBody`。curl/Postman/脚本用 DELETE 删账号必踩同一可用性坑。
**触发条件**：`curl -X DELETE http://host:3091/api/admin/accounts -H 'Authorization: Bearer <admin>'`
（无 body）→ 403 "仅接受 JSON 提交"。
**修复**：去掉 requireJSONBody 门（与 codes DELETE 对称），空 body 由 handler 解码失败返回明确
业务错误；前端始终带 JSON body 不受影响。**TDD**：`TestAdminDeleteAccountNoBodyOK` 红灯
（403 仅接受 JSON 提交）→ 绿灯（code=1 请求体解析失败 + 带 body 正常删除）。提交 `d55a87e`。

## 前端（4 项，确认修复）

### Select-31-01（MAJOR）M30-03 回显修复不完整——updater 守卫整体跳过合并，旧目标仍被防抖 PUT 覆盖删除
**缺陷**（review31-frontend）：第 30 轮 M30-03 把回显跳过条件从 `rev>0` 改为 `echoedRef` 只合并
一次 + updater 内"selected 已有内容即返回"守卫——但守卫是**整体 `return prev`，没有任何补进
逻辑**，注释声称"未涉及旧目标补进"实际从未实现。触发链：进页后课程列表（/electives 内存快照）
先渲染，/state 首次加载慢于 electives（或首帧失败 retry 拉长到秒级）→ 用户先手选课程 C →
stateData 到达 → updater 看到 selected 已有 [C] → 整体 return prev → 后端旧目标 [A,B] 永不进
selected → 防抖 PUT 只含 [C] 整包覆盖删掉 [A,B]（"添加一门"变"替换全部"，M30-03 声称修复的
缺陷原样复发）。且第 200 行 `echoedRef.current = true` **无条件执行**——合并被跳过也置位，
永不重试。
**修复**：updater 改为按 publish_id 真正合并——`prev` 中**无条目**的 publish_id（用户未触碰的
发布）把后端 courses 对应目标补进；已触碰发布（key 存在）保留用户现状（含用户主动清空过的
空数组，清空语义绝不复活）；无任何新发布可补时保持现状。回显 effect 依赖补 `rev` 使全清空
判据新鲜。提交 `d55a87e`。

### Select-31-04（MINOR）全清空被未置位回显撤销——与 31-01 同源统一修复
**缺陷**（review31-frontend）：旧守卫 `Object.values(prev).some(arr => arr.length>0)` 对
"选 A,B→保存成功→全清空（selected 空对象）"场景不拦截合并——全清空后首次非空 courses 响应
会把 [A,B] 合并回来并重新写回后端，清空被静默撤销（第 4 轮"清空后轮询旧 courses 再次回填撤销
清空"要保护的语义被新判据穿透）。
**修复**：updater 顶部加 `if (rev > 0 && !anyHas) return prev`——rev>0 且无任何条目 = 用户已明确
全清空，绝不合并回显。与 31-01 同源一次修改同时落地。提交 `d55a87e`。

### App-31-02（MINOR）自定义 adminName 刷新后丢失——管理员被判定成普通学生掉回看板
**缺陷**（review31-frontend）：`adminName` 内存态只在登录响应时 set（App.tsx），刷新后恒回默认
"admin"。后端 `XUANKE_ADMIN_NAME` 自定义名（如 root）时，管理员刷新 → adminName=admin →
`current === adminName` 判据失效 → `inAdmin || current === adminName` 恒 false → 渲染落学生
Dashboard；管理员被当普通学生，且学生看板用 admin 名查 /state 会撞不存在的客户端。
**修复**：`loadAdminName()` 读 localStorage `xk_admin_name`（try/catch 降级默认 admin），
`useState(loadAdminName)` 初始化；login 响应 adminName 时 `saveAdminName` 持久化。
与 xk_sessions 同款快照降级语义。提交 `d55a87e`。

### Admin-31-03（MINOR）激活码删除 removing 全局单值——异码并发删除互踩在飞标记
**缺陷**（review31-frontend）：`remove` 用 `useState(false)` 单值布尔，M30-05 只覆盖"同一码双击"
场景——码 A 删除在飞点码 B 覆盖标记、A finally 清 false 把 B 在飞态抹掉、用户再点 B 发第二发
后端报"不存在"假失败 toast（与选课大厅 actionLoading M29-01 单值→Set 同族，异码维度复发）。
**修复**：改 `ReadonlySet<string>` 按码独立跟踪——守卫 `removing.has(code)`、置位函数式 add、
finally 函数式 delete 只删自己的码，按钮 disabled 同步 `removing.has(c.code)`。
提交 `d55a87e`。

## 可疑待核裁决（review31-backend）
- **B31-02 setApiKey("") 清空窗口**：F26-04 已把清空挪到健康信号（configEpoch 自增 + toast）
  之后，残留窗口为"PUT 完成 → 用户此刻点进密钥框打字"的亚秒窄窗，且保存后才清空仍保证
  "留空 = 不改动 key"回显语义不被旧输入污染。**定不修，维持观察**。
- **B31-03 管理员名与学生对撞**：学生用管理员名登录会被重定向到管理口令校验属设计意图；
  改名为空时 adminName=="" 恒不命中改用默认 admin。**定不修，保留决策**。
- **reloginAt 本地钟 vs 对齐钟**：写读两端同基（同为本地钟），退避只在此链内判读，与 tick 对齐
  钟判定无交互，偏差 < 1s / 30s 退避窗口。**维持观察**。

## 已核无缺陷清单（review31-backend 详核，16 项）
1. **删除账号四段防线**：memory-first（B26-01）+ 自动链（B18-M2）+ 手动 MarkDone/RemoveDone
   （B20-01）+ 重登成功分支（B21-03）+ spawnChain 链顶前置复核（B30-01）全部在岗，对应测试绿灯。
2. **实时人数复核与删除竞态**：spawnChain 复核路径三处 doneHas 复核持锁，删号期间链顶 block。
3. **B23-03 tokenValidForLocked 与 reloginBackoff 交互**：退避期链每 tick 短路不发起，退避过才恢复。
4. **识别引擎模板透传（B29-01）**：SetRecognizer 写模板 + SetVision 保留引擎，测试齐备。
5. **WindowClosed 判据单源（B29-02）**：windowClosedLocked() 三判据统一，测试绿灯。
6. **时钟对齐链路**：lastSyncTime 只被成功推进、syncing 调用侧置位、lastSyncFailAt 30s 退避、
   连续失败 3 次复位 offset 保留 streak（B21-01 已真可达）。
7. **探测节流/单飞/probeSem**：lastProbe 只归 probe()/ProbeNow 写、probing 单飞、probeSem cap 4。
8. **快照回退链**：目标账号缺失/过期→(nil,false)；无目标账号专属帧过期→(nil,false)；
   从未有专属帧才回退全局帧。
9. **透传凭据表校验五处**：写路径/课程读/手动操作/状态读全走 accountExists。
10. **open_time 契约**：FormatOpenTime 空串零值 + B21-04 前置校验 + B11-A1 零值守卫 + B15-M2 降频。
11. **scheduler 并发锁纪律**：状态读写持 s.mu、网络调用锁外、对齐钟写入侧统一。
12. **handler API 层**：登录/激活独立限流桶、requireAdminSession + 恒定时间比对、ROLE 越权隔离、
    /api 前缀 404、content-type CSRF 门、panic recover、XFF 回环信任门。
13. **store/SQLite**：单连接、WAL、事务原子、恢复顺序 RestoreDone→RestoreTargets→RestoreRefused。
14. **session**：64 字节令牌、Admin 标志随会话、ConsumeTicket 单次防重放、RevokeAccount 全量吊销。
15. **secure**：AES-256-GCM 随机 nonce、.master_key 32 字节校验、vision_key enc: 前缀加密落库。
16. **web/embed.go**：/api 前缀在 SPA 侧也 404，B30-02 闭环。

## 回归
- backend：`go build ./... && go vet ./...` 全绿；`go test -race ./...` 全量通过
  （api 15.0s 含新增 TestAdminDeleteAccountNoBodyOK）。
- frontend：`npm run build`（tsc -b + vite）通过。
- 注释清理：全仓库剥离"（第N轮）"轮次前缀 140 处（21 文件），sed 产生的双冒号清零，
  决策历史统一移入 CLAUDE.md；build/vet/race/build 全绿证明清理零破坏。

## 观察项（本轮追加/延续）
- 观察 B31-01：scheduler 时间戳写入侧用对齐钟、lastSyncFailAt/reloginAt 用本地钟——校准失败时
  对齐钟退化到本地钟，边界同义，无实质影响。维持观察。
- 观察 B31-02：task_log 表无容量上限（无定期清理），长期运行日志行无限增长，当前量级小。维持观察。
- 观察 B31-03：emptyRunsFor 死代码 / syncFailedWindow 写而不读 / RemoveFull 死方法 /
  Decrypt Deps 字段未消费——历轮观察延续。
- 历轮观察项全表延续（tsconfig 缺 strict、reloginBackoff 死分支、RestoreRefused 注释过时、
  settings captcha_concurrency 无上限、probe per-account 近超时排队、PUT config 空变更、
  HTTP 探测单飞、handleBack 极端失败路径、Dashboard 日志 key 缺 account 维度、adminName 撞名、
  ddddocr 识别无净化、B31-02/B31-03 上述裁决等）。
