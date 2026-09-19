# round40 backend 修复报告

日期：2026-09-20

范围：仅 backend/ 目录（web/ 由并行代理处理，未触碰）。

## B40-01（MINOR）：requireJSONBody 的 CSRF-403 仍走 writeJSON 恒 HTTP 200

- 位置：`backend/internal/api/router.go` `requireJSONBody` 拒绝分支
- 修复：拒绝分支改 `writeJSONStatus(w, http.StatusForbidden, 403, nil, "仅接受 JSON 提交")`，与登录/激活两处 CSRF 门同款（真实 HTTP 403）
- 测试名：`TestRequireJSONBodyRejectsFormContentType`（handler_test.go）
  - 修复前红：`http=200`（body code=403 但 HTTP 恒 200）
  - 修复后绿：HTTP 403 + body code=403 + 文案含 "JSON"
- commit：`4c876fb`（fix(api): requireJSONBody 拒绝分支写真实 HTTP 403）
- 说明：B39-02 把 4 类基础设施错误改真实状态码，但 requireJSONBody 是改漏的最后一处；HTTP 层状态分裂会让安全扫描/反代无法识别被 CSRF 拒的副作用请求。

## B40-02（MINOR）：config.OpenTime 死配置字段 + 硬编码 2026 日期残留

- 位置：`backend/internal/config/config.go`
- 修复：删除 `Config.OpenTime` 字段、`envOr("XUANKE_OPEN_TIME", "2026-09-13 09:00:00")` 默认值、`ensureEnvFile` 模板中的 XUANKE_OPEN_TIME 示例行；移除处加注释"开放时间唯一事实源 = 平台 beginTimes 自动识别（scheduler 层），配置层不再注入"
- 全仓库消费点核验：`cfg.OpenTime` 在 main.go 从未消费（`scheduler.New(accts, st, time.Time{}, ...)` 传零值），scheduler 侧 `openTimeForLocked` 只回退遗留 openTime 字段（生产恒零值，保留字段仅为测试兼容）；README.md:36 的 XUANKE_OPEN_TIME 文档行未动（pre-existing 未提交文档改动，属他人范围）
- 测试名：`TestConfigDoesNotInjectOpenTime`（env_test.go）
  - 修复前红：`Config 不得再含 OpenTime 字段`（reflect.FieldByName 命中）
  - 修复后绿：结构体无该字段 + 预置 `XUANKE_OPEN_TIME=2099-01-01 00:00:00` 环境变量亦不注入
- commit：`9980a47`（fix(config): 移除死配置 OpenTime 字段与硬编码 2026 默认值）
- 说明：死配置 + 过期日期 + 环境变量读取并存，运维按旧文档配置会产生"识别槽为空时把 2026-09-13 当开窗点"的误导。

## 全量回归

```
cd backend
go build ./...        # 通过
go vet ./...          # 通过
go test -race -count=1 ./...  # 9 包全绿
```
- accounts / api / config / db / runtime / scheduler / secure / session / store / zhidao 全 ok，cmd/probe、web 无测试文件

## 契约核对

- 零吞错落库点：本轮两处修改均不涉及落库路径
- 注释均为简体中文、无轮次前缀标签
- 每处修复独立 commit，未 push
