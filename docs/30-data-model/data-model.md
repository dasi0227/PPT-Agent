---
id: DATA-MODEL
title: 数据模型总览
status: approved
owner: backend
depends_on: [ARCH-SYSTEM, ADR-0002]
verifies: []
---

# 数据模型总览

## 存储划分（混合持久化）

| 数据 | 存储 | 理由 |
|---|---|---|
| 元数据（项目/Deck/Slide 索引/版本记录/插件索引/Run 记录） | SQLite | 可查询、事务安全 |
| slide html / css / js、公共样式层、slide-json、插件资源 | 文件系统（work_dir） | 大文本、git 友好、直接可渲染 |

详见 [filesystem-layout](filesystem-layout.md) 与 [sqlite-schema.sql](sqlite-schema.sql)。

## 实体关系（ER）

```
Project (1) ──── (1) Deck (1) ──── (N) Slide
   │                 │                  │
   │                 │                  └── (N) Version  (slide 级快照)
   │                 └── (N) Version       (deck 级/公共样式层快照)
   │
   ├── (N) Thread     (对话线程，可恢复；共享本 project 产物)
   │        └── (N) Run   (一次执行/turn，跑 harness 循环)
   │
   └──（Run 也直接挂 project：project 是隔离与锁的单元）

Asset (N)             (个人仓库统一资产，全局，不强绑 project；kind=layout|component|theme|fx)
   └── (N) Version    (资产级快照，可回滚)
```

## 三层隔离模型（对齐 Codex 的 project/thread/turn）

| 层 | 隔离什么 | 载体 | 键 |
|---|---|---|---|
| Project | 一个 PPT 的产物文件 + 元信息 | `PPT_WORK_ROOT/<project_id>/` 目录 + SQLite `project_id` | `project_id` |
| Thread | **对话历史/上下文**（可恢复、可多条并行） | `threads` 表 + `threads/<thread_id>.jsonl` | `thread_id` |
| Run | 单次执行（turn）的事件流 | `runs` + `run_events`（带 `thread_id`） | `run_id` |

- **共享产物语义（Codex 一致）**：同一 project 下多个 thread **共享同一份产物文件**；thread 只隔离对话历史，不分叉文件。试验性改版靠[版本回滚](versioning.md)。
- **每 project 一把执行锁**：同 project 的 Run 串行（防 state.json/文件打架），跨 project 并行（work_dir 物理隔离）。详见 [agent-runtime](../20-architecture/agent-runtime.md)。

## 实体定义

### Project
| 字段 | 类型 | 说明 |
|---|---|---|
| id | TEXT (uuid) | 主键 |
| title | TEXT | 项目标题 |
| work_dir | TEXT | 工作目录绝对/相对路径 |
| created_at / updated_at | INTEGER (unix) | 时间戳 |

### Deck
| 字段 | 类型 | 说明 |
|---|---|---|
| id | TEXT | 主键 |
| project_id | TEXT | 外键 → Project |
| theme | TEXT | 当前主题 id（design-system） |
| status | TEXT | `draft`\|`generating`\|`ready` |
| common_style_path | TEXT | 公共样式层文件相对路径 |
| created_at / updated_at | INTEGER | |

### Slide
| 字段 | 类型 | 说明 |
|---|---|---|
| id | TEXT | 主键 |
| deck_id | TEXT | 外键 → Deck |
| idx | INTEGER | 页序（0 基） |
| layout | TEXT | 版式枚举 |
| title | TEXT | 标题 |
| json_path | TEXT | slide-json 文件相对路径 |
| html_path | TEXT | slide html 文件相对路径 |
| current_version | INTEGER | 当前版本号 |
| last_export_at | INTEGER NULL | backlog：导出时间（预留） |

`DATA-SLIDE-001`：Slide 的 slide-json MUST 符合 [slide-json.schema.json](slide-json.schema.json)。

### Version
| 字段 | 类型 | 说明 |
|---|---|---|
| id | TEXT | 主键 |
| target_type | TEXT | `slide`\|`deck`\|`common_style` |
| target_id | TEXT | 关联实体 id |
| version_no | INTEGER | 递增版本号 |
| snapshot_path | TEXT | 快照文件相对路径 |
| run_id | TEXT NULL | 产生该版本的 Run |
| created_at | INTEGER | |

详见 [versioning](versioning.md)。

### Thread（对话线程）
一个 project 下可有多条 thread，各自独立可恢复的对话历史，但**共享 project 产物**。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | TEXT | 主键 |
| project_id | TEXT | 外键 → Project |
| title | TEXT | 线程标题（可由首条消息生成） |
| history_path | TEXT | 对话历史文件相对路径（`threads/<id>.jsonl`） |
| status | TEXT | `active`\|`archived` |
| created_at / updated_at | INTEGER | |

`DATA-THREAD-001`：同 project 的多个 thread 共享产物文件；thread 仅隔离对话历史。
`DATA-THREAD-002`：thread 历史以追加式 jsonl 持久化，支持恢复（关闭再打开接着聊）。

### Run
| 字段 | 类型 | 说明 |
|---|---|---|
| id | TEXT | 主键 |
| thread_id | TEXT NULL | 外键 → Thread（挂在某对话线程下；repo 类快操作可无 thread） |
| project_id | TEXT NULL | 外键（冗余便于按项目查询/加锁；repo scope 的 run 可无项目） |
| kind | TEXT | `outline`\|`generate`\|`edit`\|`command` |
| scope | TEXT | `current`\|`page`\|`overview`\|`repo` |
| page_index | INTEGER NULL | 针对页时的页序 |
| mode | TEXT | `normal`\|`talk`\|`ask` |
| command | TEXT NULL | 显式指令名（prompt/recap/talk/ask 等） |
| status | TEXT | `pending`\|`running`\|`waiting`\|`done`\|`failed`\|`canceled` |
| created_at / updated_at | INTEGER | |

### Asset（个人仓库统一资产）
四类资产共享统一信封，载荷存文件系统。预置与用户新增同表，`source` 区分。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | TEXT | 主键 |
| name | TEXT | 资产名 |
| kind | TEXT | `layout`\|`component`\|`theme`\|`fx` |
| version | TEXT | 语义化版本 |
| source | TEXT | `preset`（出厂预置）\|`user`（用户新增） |
| description | TEXT | 一句话用途（供 AI 检索） |
| tags | TEXT | JSON 数组文本 |
| manifest_path | TEXT | manifest.json 相对路径 |
| dir | TEXT | 资产资源目录 |
| created_at / updated_at | INTEGER | |

`DATA-ASSET-001`：Asset 的 manifest MUST 符合 [asset-manifest.schema.json](../60-design-system/asset-manifest.schema.json)。
`DATA-ASSET-002`：`kind=theme` 的资产 MUST 提供「必需 token 全集」（见 [design-tokens](../60-design-system/design-tokens.md)）。

## 数据约束汇总

| ID | 约束 |
|---|---|
| `DATA-MODEL-001` | 元数据入 SQLite，slide 大文本入 fs；二者通过路径字段关联 |
| `DATA-SLIDE-001` | slide-json 符合 slide-json schema |
| `DATA-ASSET-001` | asset manifest 符合 asset-manifest schema |
| `DATA-ASSET-002` | theme 资产提供必需 token 全集 |
| `DATA-THREAD-001` | 同 project 多 thread 共享产物，仅隔离对话历史 |
| `DATA-VERSION-001` | 任意可编辑产物变更 MUST 产生新版本，支持回滚（含 slide/deck/common_style/asset） |
| `DATA-MODEL-002` | 外键关系 MUST 在删除时级联或受保护（不留孤儿记录） |
| `DATA-MODEL-003` | 删除 project MUST 级联删除其 thread/run/deck/slide/version 与 work_dir 目录 |

## 验收标准（Given-When-Then）

- **AC-DATA-001**（`DATA-MODEL-001`）
  - GIVEN [sqlite-schema.sql](sqlite-schema.sql)
  - WHEN 在 SQLite 执行
  - THEN 建表成功且外键约束生效

- **AC-DATA-SLIDE-001**（`DATA-SLIDE-001`）
  - GIVEN 任一 slide-json 文件
  - WHEN 用 slide-json schema 校验
  - THEN 通过校验

## 校验方式

```bash
sqlite3 :memory: < docs/30-data-model/sqlite-schema.sql
# JSON Schema 校验：见 slide-json.schema.json / asset-manifest.schema.json
```

## 依赖

- [ARCH-SYSTEM](../20-architecture/system-overview.md)、[ADR-0002](../90-decisions/0002-persistence-sqlite-fs.md)
