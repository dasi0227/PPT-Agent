---
id: DATA-VERSION
title: 版本与回滚模型
status: approved
owner: backend
depends_on: [DATA-MODEL, DATA-FS-LAYOUT]
verifies: []
---

# 版本与回滚模型

## 目标

任何可编辑产物（单页 slide html、公共样式层、project 结构）的每次变更都产生**可回滚快照**，
支撑 [SPEC-EDIT-005](../10-spec/feat-nl-editing.md) 与 [SPEC-GLOBAL-004](../10-spec/functional-spec.md)。

## 版本对象

| target_type | 含义 | 快照内容 |
|---|---|---|
| `slide` | 单页 html | 该页 `index.html`（必要时含 slide.css/js） |
| `design` | 公共样式层 | `common/tokens.css` |
| `project` | project 结构 | `project.json`（页顺序/主题引用） |
| `asset` | 个人仓库资产 | 该资产目录快照（manifest + 载荷） |

## 版本生成规则

| ID | 规则 |
|---|---|
| `DATA-VERSION-001` | 每次成功编辑/生成 MUST 写入新版本（version_no 递增） |
| `DATA-VERSION-002` | 版本号在 `(target_type, target_id)` 维度单调递增，不复用 |
| `DATA-VERSION-003` | 版本快照存于 `versions/`，并在 SQLite `versions` 表登记 |
| `DATA-VERSION-004` | 回滚 = 用历史快照覆盖当前 + 记录一条**新版本**（回滚本身也是一次变更） |
| `DATA-VERSION-005` | `slides.current_version` MUST 指向该 slide 当前生效版本号 |

## 回滚语义

```
slide-000:  v0 → v1 → v2(current)
回滚到 v1:
  1. 用 v1 快照覆盖 slides/000/index.html
  2. 写入 v3（内容=v1），current_version=3
  3. 关联 run_id（若由某次 run 触发）
```

> 不做「破坏性回退」（不删除 v2），保证历史完整、可再次前进。

## 与 Run 的关联

- 每个版本可携带 `run_id`，追溯「哪次执行产生了它」，便于 `/recap` 回顾。

## 验收标准（Given-When-Then）

- **AC-VERSION-004**（`DATA-VERSION-004`）
  - GIVEN slide-000 处于 v2
  - WHEN 回滚到 v1
  - THEN 文件内容 == v1，且 `versions` 表新增 v3（内容等于 v1），`current_version=3`

- **AC-VERSION-002**（`DATA-VERSION-002`）
  - GIVEN 连续 3 次编辑同一页
  - WHEN 查询版本
  - THEN version_no 为 0,1,2,3 单调递增无重复

## 校验方式

```bash
go test ./internal/service -run 'TestVersioning|TestRollback'
```

## 依赖

- [DATA-MODEL](data-model.md)、[DATA-FS-LAYOUT](filesystem-layout.md)
