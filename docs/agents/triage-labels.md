# Triage Labels

The skills speak in terms of five canonical triage roles. This file maps those roles to the actual label strings used in this repo's issue tracker.

| Label in mattpocock/skills | Label in our tracker | Meaning                                  |
| -------------------------- | -------------------- | ---------------------------------------- |
| `needs-triage`             | `needs-triage`       | Maintainer needs to evaluate this issue  |
| `needs-info`               | `needs-info`         | Waiting on reporter for more information |
| `ready-for-agent`          | `ready-for-agent`    | Fully specified, ready for an AFK agent  |
| `ready-for-human`          | `ready-for-human`    | Requires human implementation            |
| `wontfix`                  | `wontfix`            | Will not be actioned                     |

When a skill mentions a role (e.g. "apply the AFK-ready triage label"), use the corresponding label string from this table.

## 本仓库现状（2026-09-27 实测）

采用上表默认命名，标签名与角色名一致。**但 GitHub 仓库里目前只有 `wontfix` 一个角色标签**——`gh label list` 实测共 10 个标签，其余是 GitHub 自带的 `bug` / `enhancement` / `documentation` / `duplicate` / `good first issue` / `help wanted` / `invalid` / `question` / `accessibility`。另外四个角色标签还没建，首次使用 `triage` 之前必须先补：

```bash
gh label create needs-triage    --description "Maintainer needs to evaluate this issue"
gh label create needs-info      --description "Waiting on reporter for more information"
gh label create ready-for-agent --description "Fully specified, ready for an AFK agent"
gh label create ready-for-human --description "Requires human implementation"
```

`gh issue edit --add-label` 不会自动创建标签，贴一个仓库里不存在的标签会失败。

另：`triage` 技能在 `wontfix` 的 enhancement 分支会写仓库根的 `.out-of-scope/`。该目录当前不存在，也未被 `.gitignore` 忽略，首次触发时会以未跟踪文件的形式出现。
