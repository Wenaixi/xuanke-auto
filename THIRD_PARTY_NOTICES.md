# 自动选课 · 第三方声明

本仓库使用以下第三方项目，许可证已逐一核对。

## Go 后端（backend/）

| 项目 | 用途 | 许可证 |
| --- | --- | --- |
| [modernc.org/sqlite](https://modernc.org/sqlite) | 纯 Go SQLite 驱动 | BSD-3-Clause |
| [yangbin1322/go-ddddocr](https://github.com/yangbin1322/go-ddddocr) | 原生 ddddocr 识别引擎绑定 | Apache-2.0 |
| [yalue/onnxruntime_go](https://github.com/yalue/onnxruntime_go) | ONNX Runtime 的 Go 绑定 | MIT |
| [nfnt/resize](https://github.com/nfnt/resize) | 图像缩放 | ISC |
| [getlantern/systray](https://github.com/getlantern/systray) | 系统托盘 | MIT |
| [lxn/walk](https://github.com/lxn/walk) 与 [lxn/win](https://github.com/lxn/win) | Windows 原生 GUI 与 Win32 API | BSD-3-Clause |
| [skratchdot/open-golang](https://github.com/skratchdot/open-golang) | 用默认浏览器打开链接 | MIT |
| [google/uuid](https://github.com/google/uuid) | UUID 生成 | BSD-3-Clause |
| [stretchr/testify](https://github.com/stretchr/testify) | 测试断言 | MIT |
| 其他 Go 依赖（含 modernc 系 cc/v4、libc 等） | 见 `backend/go.sum` | 各依赖 LICENSE |

## 前端（web/）

React 19、Vite、TypeScript、Tailwind CSS 4、Radix Primitives、TanStack Query、lucide-react、oxlint 等。各包许可证见 `web/package-lock.json` 及对应包内 LICENSE，构建依赖上游数据，建议定期 `npm audit`。

### 内嵌字体

界面字体随包分发，均为 **SIL Open Font License 1.1**：

| 包 / 文件 | 字体 | 用途 | 版权 |
| --- | --- | --- | --- |
| `@fontsource-variable/geist` | Geist Variable | 全站界面正文（可变无衬线） | The Geist Project Authors（https://github.com/vercel/geist-font） |
| `web/public/fonts/jf-openhuninn-title.woff2` | jf open 粉圓 的**派生子集** | 仅左上角标题 | Copyright (c) 2010 MOTOYA CO.,LTD.；2011-2016 The Varela Round Project Authors；2020-2024 justfont Co., LTD.（https://github.com/justfont/open-huninn-font） |
| `@fontsource-variable/geist-mono` | Geist Mono Variable | 等宽数字（倒计时、课程 ID、统计表格） | The Geist Project Authors（https://github.com/vercel/geist-font） |

- 标题子集由 `jf-openhuninn-2.1.ttf` 用 fontTools 按标题实际用字切出（11KB）。该字体的许可声明了
  **Reserved Font Names**（`open huninn` 等），故派生子集**不使用保留名**：内部字体名已改为
  `JF Open Huninn Title`，CSS family 名为 `Title Round`。许可证全文随包放在
  `web/public/fonts/jf-openhuninn-OFL.txt`（会随 `public/` 一起进入 dist、打进 exe/APK）。
- 粉圓子集不含部分简体字（实测缺「选课择统」），这些字由字体栈第二顺位 Geist Variable 承接，不会出现豆腐块。
- **Geist 不含汉字**：中文由系统中文字体按 `PingFang SC`（macOS/iOS）→ `Microsoft YaHei UI`（Windows）→ `Noto Sans CJK SC`（Linux）顺位兜底，各平台观感会略有差异。

## 模型与运行时

- `backend/internal/upstream/assets/common_old.onnx`、`charsets_old.json`：ddddocr 识别模型与字符集，来自 [ddddocr](https://github.com/sml2h3/ddddocr)，遵循其许可。
- Microsoft ONNX Runtime 运行时（`onnxruntime` dll/so/dylib）：版本锁定官方 v1.25.0，与 `go.sum` 中的 `onnxruntime_go` 对齐。**不入 git**，由 `backend/scripts/fetch-onnxruntime.sh` 在构建时下载，许可与版权见[上游仓库](https://github.com/microsoft/onnxruntime)。

## 完整第三方声明

上述第三方代码、模型与资产的完整版权与许可声明，请在各上游仓库查询。发布产物（单 exe、APK）内嵌了上述全部内容，分发时请一并保留本文件。
