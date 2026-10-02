# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- **`CONTEXT.md`** at the repo root: 本仓库的领域词汇表（单上下文）。文档、issue、ADR、命名一律用它定义的词。
- **`docs/adr/`**: read ADRs that touch the area you're about to work in. 现有 `0001`（前端产物 embed 进单二进制）、`0002`（按教务账号隔离会话与课程快照）、`0003`（开窗时间由平台 beginTimes 自动识别，不可配置）、`0004`（站点差异全部收进平台档案，引擎只保留流程）。
- 本仓库**没有** `CONTEXT-MAP.md`：只有一个上下文，不存在 `src/<context>/docs/adr/`。

这两个落点都已经建立（2026-09-27）。维护方式：新术语写进 `CONTEXT.md`；新的架构决策按 `docs/adr/NNNN-短横线标题.md` 取最大编号 +1，四段 `Status` / `Context` / `Decision` / `Consequences`，结论必须能指回核对的源文件路径。写入者仍是 `/domain-modeling`（经 `/grill-with-docs` 与 `/improve-codebase-architecture` 到达）。

## File structure

本仓库属于单上下文形态（下表第一段），`CONTEXT.md` 在根目录，决策在 `docs/adr/`。

Single-context repo (this repo):

```
/
├── CONTEXT.md
├── docs/adr/
│   ├── 0001-single-binary-embed-web.md
│   ├── 0002-per-account-isolation.md
│   ├── 0003-open-time-auto-detection.md
│   └── 0004-platform-profiles.md
└── backend/, web/, build/            ← 本仓库没有 src/，代码在 backend/ 与 web/
```

Multi-context repo (presence of `CONTEXT-MAP.md` at the root) — 本仓库不适用，仅作形态参考：

```
/
├── CONTEXT-MAP.md
├── docs/adr/                          ← system-wide decisions
└── src/
    ├── ordering/
    │   ├── CONTEXT.md
    │   └── docs/adr/                  ← context-specific decisions
    └── billing/
        ├── CONTEXT.md
        └── docs/adr/
```

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in `CONTEXT.md`. Don't drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal: either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/domain-modeling`).

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than silently overriding:

> _Contradicts ADR-0007 (event-sourced orders), but worth reopening because…_
