# 至道选课自动化 · 第三方声明

本仓库用到以下项目（许可证已逐一确认可开源）：

| 项目 | 用途 | 许可证 |
| --- | --- | --- |
| [modernc.org/sqlite](https://modernc.org/sqlite) | 纯 Go SQLite 驱动 | BSD-3-Clause |
| [yangbin1322/go-ddddocr](https://github.com/yangbin1322/go-ddddocr) | 原生 ddddocr 识别引擎绑定 | Apache-2.0 |
| [nfnt/resize](https://github.com/nfnt/resize) | 图像缩放 | ISC |
| 其他 Go 依赖 | 见 `backend/go.sum` | 各依赖 LICENSE |

前端依赖（React / Vite / Tailwind / Radix 等）的许可证见 `web/package-lock.json` 各包对应 LICENSE。

## 符号与内置资产

- `backend/internal/zhidao/assets/common_old.onnx`、`charsets_old.json`：ddddocr 识别模型与字符集，来自 [ddddocr](https://github.com/sml2h3/ddddocr)，遵循其许可。
- `backend/web/dist/bg.jpg`：界面背景图（作者自备图片）。

## 完整第三方声明

所有第三方代码、模型、资产的完整版权与许可声明，请在各上游仓库查询。