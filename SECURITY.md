# 安全策略

本项目是一个跑在**使用者自己机器**上的本地单用户选课工具：Go 后端（监听本机端口）+ 内嵌前端，数据（教务凭据、目标课程、日志）全部落在本地 `data/` 目录。下面的威胁模型与取舍以这个形态为前提。

## 支持的版本

| 版本 | 支持状态 |
| --- | --- |
| `master` 分支最新提交 | 支持：安全修复直接进 `master` |
| 最新 Release 标签的产物 | 支持：修复随下一个 Release 发布 |
| 更早的历史版本与产物 | 不支持：本项目不做长期维护分支，也不回补历史版本；遇到问题请先升级到最新产物 |

升级方式就是下载最新产物覆盖。Android 有一个例外：v0.2.15 与 v0.2.16 的 CI 曾现场生成签名密钥，两版证书指纹不同，装过这两版任意一个的设备**必须先卸载重装一次**（清 App 数据），此后的版本才会正常覆盖升级。

## 报告漏洞

**请走私密渠道，不要开公开 issue、PR 或讨论。** 可用渠道（任选其一，优先第一个）：

- 邮箱 **wenxiloveyou@gmail.com**（标题建议以 `[security]` 开头）
- [GitHub Security Advisory 私密上报](https://github.com/Wenaixi/xuanke-auto/security/advisories/new)（仓库 Security 页 → Report a vulnerability；若该入口不可用，请走邮箱）

以下为补充说明：


报告里请包含：

- 漏洞类型与影响范围（能到哪一步：读到数据 / 改配置 / 执行操作 / 只是信息泄露）
- 复现步骤，含版本（tag）与平台（Windows / Linux / macOS / Android）
- 最小复现或 PoC；涉及平台接口的，说明是猜测还是有实测/抓包依据
- 建议的修复方向（可选）

我们会尽快确认并修复；修复发布前不公开细节。若你不想在公开历史里留名，请在报告里说明。**请勿在报告里附带他人的真实账号、学号、手机号或 token**——需要样本时用你自己的测试账号。

### 不在范围内

- 需要攻击者已经能以同一 OS 用户身份读写本机文件的问题（本地工具的固有前提，见下表）
- 教务平台自身（zhidao.fj.cn）的漏洞——那是上游，请按其渠道上报
- 平台风控被触发、账号被限制这类使用后果
- 依赖库自身的已知漏洞（可提，但请同时给出受影响版本与升级建议）

## 威胁模型与已知取舍

| 威胁 | 现状 | 依据 |
| --- | --- | --- |
| 同网段其他主机访问管理面 | 默认只监听 `127.0.0.1`；`XUANKE_LISTEN_HOST` 可改成内网 IP / 域名 / `0.0.0.0`，改成非回环即等于把管理面开放给该网络 | `backend/internal/config/config.go`（`envOr("XUANKE_LISTEN_HOST", "127.0.0.1")`）、`backend/server.go` |
| 后台误改监听地址把服务改死 | 改地址走 `Deps.Rebind`：**先绑新地址成功、再关旧监听**；失败整体拒绝（HTTP 400），不落库不生效 | `backend/internal/api/handler.go`（`validateListen`、`Rebind`）、`backend/server.go`、`backend/listen_host_test.go` |
| 同设备其他 App 访问 Android 壳的管理面 | **已知且接受**：APK 内置固定管理账密（`admin` / `admin123`）并在启动时写进 `<filesDir>/data/.env`，服务监听本机 3091；同设备任意 App 都能连上并登录管理页 | `backend/platform_android.go`（`XuankeStart` → `config.SetPlatformProfileForPlatform`）、`backend/internal/config/config.go` |
| 本机同权限攻击者读数据库 | 凭据以密文落库，但主密钥就在同目录（`data/.master_key`）或环境变量里；**只能防「数据库被单独拷走」，防不了能读该 OS 用户文件的攻击者** | `backend/internal/secure/crypto.go`、`backend/internal/store/store.go`、`backend/internal/runtime/config.go` |
| 伪造 `X-Forwarded-For` 刷爆他人限流桶 | 默认关闭；仅当 `XUANKE_TRUSTED_PROXY=on` **且** 请求确实来自回环地址时才取 XFF 最右一个非空值 | `backend/internal/api/handler.go`（`clientIP`） |
| 抓取本机明文流量拿到会话令牌 | 无内置 TLS：`net.Listen` + `srv.Serve`，全仓没有 TLS 配置；会话 token 由平台机制以 URL 参数承载。面向公网使用依赖使用者自行前置 HTTPS 反代 | `backend/server.go` |
| 日志泄露 token / 请求 URL | token 只打印前 8 位（长度不足 8 位的一律输出 `***`）；密码从不进日志；网络层错误上抛前剥掉 `*url.Error` 里的完整 URL（那里带着 `?idToken=`） | `backend/internal/scheduler/scheduler.go`（`maskedToken`）、`backend/internal/accounts/manager.go`（`tokenShort`）、`backend/internal/upstream/client.go`（`sanitizeError`） |
| 管理口令被在线爆破或侧信道枚举 | 管理员入口必须是「管理员名 + 管理口令」双条件，口令比对用 `subtle.ConstantTimeCompare`；口令错误分支固定延迟后再响应，抹平「账号是管理员名」的时延特征 | `backend/internal/api/handler.go`（`handleLogin`） |
| 激活票据被重放/穷举 | 登录成功但未激活的账号拿到一次性票据，TTL 5 分钟，**先消费再校验激活码**——输错一次即销毁票据，同一票据无法在窗口内穷举不同码 | `backend/internal/session/store.go`（`CreateTicket`、`ConsumeTicket`） |
| 登录/激活接口被刷 | 按 IP 分桶限流（5 次/分、突发 5），登录与激活各自独立桶，超限返回真实 HTTP 429 | `backend/internal/api/handler.go`（`loginLimiter`）、`backend/internal/api/router.go` |

## 已实现的安全机制

- **凭据加密**：密码与 `vision_key` 走 AES-256-GCM（随机 nonce 前置，hex 输出）。账号密码落在 `credentials.password_enc` 列；运行期配置里的 `vision_key` 落 `settings` 表并带 `enc:` 前缀——**没有该前缀的旧明文一律拒绝加载**。主密钥取自 `XUANKE_MASTER_KEY`（64 位 hex）或 `data/.master_key`（32 字节，0600）；文件损坏、长度不符一律拒绝启动（`log.Fatalf`），绝不带伤运行。账号密码密文解不开时不会拒绝启动，而是记为「自动重登将无保存账密」并写日志。
- **管理口令缺失拒绝启动**：`XUANKE_ADMIN_TOKEN` 必须来自环境变量或 `data/.env`；桌面版首次运行会生成 24 位十六进制随机口令写进 `.env`（`crypto/rand` 不可用时直接 panic 拒绝启动，不接受可预测兜底）。
- **票据单次防重放**：见上表。
- **限流与风控**：见上表；网络或风控失败分级退避，不盲目高频重试上游。
- **未注册 API 一律 404**：SPA 兜底在路由前把所有 `/api` 前缀（含精确 `/api`）拒为 404，避免未知端点被 200 + `index.html` 掩盖。
- **数据目录整体忽略**：`data/`（db + `.env` + `.master_key`）已在 `.gitignore` 忽略，随部署一起备份迁移，禁止提交。

## 敏感信息红线（贡献者必读）

本项目对接真实教务选课系统，以下内容禁止进入任何 commit：

- 教务账号、学籍号、手机号等个人标识
- 密码、`idToken`、会话 Cookie 的真实值
- `XUANKE_ADMIN_TOKEN`、`SF_API_KEY`、`XUANKE_MASTER_KEY` 的真实值
- `data/.env`、`data/xuanke.db`、`data/.master_key`（已被 `.gitignore` 忽略）
- 真实抓包文件（HAR，含账号级会话数据）
- 各平台 onnxruntime 库（16MB+，构建时经 `fetch-onnxruntime.sh` 下载，不入 git；Android 版经 Maven Central `onnxruntime-android` aar 提供）
- Android `libxuanke.so`（Go c-shared 交叉编译产物，CI 构建时生成，不入 git）

> 已入库的敏感历史如被发现，请**走私密渠道**报告并配合重写历史清理（本项目已做过一次全历史脱敏）。
> 新增配置项时先判断是否含敏感值：含则必须走 `data/.env`（不入库）并在日志/回显脱敏；`.env.example` 模板只写占位值。

## 使用者注意事项

| 事项 | 说明 |
| --- | --- |
| 不要把 3091 直接暴露到公网 | 服务无内置 TLS、无访问控制层。确需外网访问请前置反向代理 + HTTPS + 访问限制，并在确认代理与后端同机（`RemoteAddr` 为回环）后才打开 `XUANKE_TRUSTED_PROXY=on` |
| 管理口令要够强 | 它就是管理后台的唯一门槛（激活码、系统配置、账号管理、操作日志全在一个口令后面）。改成弱口令等于把管理面交出去 |
| 激活码按一次性凭证分发 | 别贴到公开群里；一码一用、激活一次永久免激活 |
| 备份 `data/` 等于备份密钥 | 换机器请把 `data/` 一起迁移；`.master_key` 丢失后已保存的教务密码不可解密，只会留下「凭据解密失败」日志，需要重新登录 |
| Android 包只装在自己设备上 | 同设备任意 App 都能连 `127.0.0.1:3091` 并用内置口令登管理页；不要在装了来源不明 App 的设备上使用，也不要 root 后当多用户环境用 |
| 不要用他人的教务凭据 | 这是给本人选课用的工具，代他人登录会同时波及双方账号 |
| 服务器部署注意文件权限 | `data/.master_key` 与数据库对运行账号可读即可，别给无关用户读权限 |
| 只从本仓库 Releases 下载产物 | 产物带 `checksums.txt`（sha256），APK 另有固定签名指纹；第三方转发的包无法保证未被改动 |

## 自动化抢课的合规边界

本项目面向"校内学生自主选课"场景，使用者请遵守所在院校的选课规则与平台使用条款。请勿将本项目用于：绕过平台风控、批量占位影响他人正常选课、牟利或任何违反校规/法规的行为。本项目不提供任何规避平台安全措施的机制（验证码识别仅用于替代手动输入，不绕过登录鉴权）。
