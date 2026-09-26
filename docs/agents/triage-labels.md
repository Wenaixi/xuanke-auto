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

Edit the right-hand column to match whatever vocabulary you actually use.

## 本仓库已确认采用默认五标签

2026-09-26 主人确认：保留上表默认命名，标签名与角色名一致。首次使用 `triage` 贴标签前，先确认这五个标签在 GitHub 仓库中已存在（`gh label list`），缺失则 `gh label create` 补齐——`gh issue edit --add-label` 不会自动创建标签，写不存在的标签会静默失败或报错。
