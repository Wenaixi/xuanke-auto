# 至道选课自动化 · 第三方声明

本仓库用到以下项目（许可证已逐一确认可开源）：

## Go 后端（backend/）

| 项目 | 用途 | 许可证 |
| --- | --- | --- |
| [modernc.org/sqlite](https://modernc.org/sqlite) | 纯 Go SQLite 驱动 | BSD-3-Clause |
| [yangbin1322/go-ddddocr](https://github.com/yangbin1322/go-ddddocr) | 原生 ddddocr 识别引擎绑定 | Apache-2.0 |
| [nfnt/resize](https://github.com/nfnt/resize) | 图像缩放 | ISC |
| [getlantern/systray](https://github.com/getlantern/systray) | 系统托盘 | MIT |
| [lxn/walk](https://github.com/lxn/walk) 与 [lxn/win](https://github.com/lxn/win) | Windows 原生 GUI 与 Win32 API | BSD-3-Clause |
| [skratchdot/open-golang](https://github.com/skratchdot/open-golang) | 默认浏览器打开链接 | MIT |
| [google/uuid](https://github.com/google/uuid) | UUID 生成 | BSD-3-Clause |
| [stretchr/testify](https://github.com/stretchr/testify) | 测试断言 | MIT |
| 其他 Go 依赖（含 modernc 系 cc/v4、libc 等） | 见 `backend/go.sum` | 各依赖 LICENSE |

## 前端（web/）

React 19、Vite、TypeScript、Tailwind CSS 4、Radix Primitives、TanStack Query、lucide-react、oxlint 等。各包许可证见 `web/package-lock.json` 对应 LICENSE；构建依赖上游数据，建议定期 `npm audit`。

## 符号与内置资产

- `backend/internal/zhidao/assets/common_old.onnx`、`charsets_old.json`：ddddocr 识别模型与字符集，来自 [ddddocr](https://github.com/sml2h3/ddddocr)，遵循其许可。
- `web/public/bg.jpg`：界面背景图（作者自备图片）。

## 完整第三方声明

所有第三方代码、模型、资产的完整版权与许可声明，请在各上游仓库查询。发布产物（单 exe）内嵌上述全部内容，分发时请一并保留本文件。