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
   └── (N) Run        (一次 Agent 执行，关联 project/deck/slide)

Plugin (N)            (个人仓库，全局，不强绑 project)
```

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

### Run
| 字段 | 类型 | 说明 |
|---|---|---|
| id | TEXT | 主键 |
| project_id | TEXT | 外键 |
| kind | TEXT | `outline`\|`generate`\|`edit`\|`command` |
| scope | TEXT | `page`\|`overview`\|`deck` |
| page_index | INTEGER NULL | 针对页时的页序 |
| mode | TEXT | `normal`\|`talk`\|`ask` |
| status | TEXT | `pending`\|`running`\|`waiting`\|`done`\|`failed`\|`canceled` |
| created_at / updated_at | INTEGER | |

### Plugin
| 字段 | 类型 | 说明 |
|---|---|---|
| id | TEXT | 主键 |
| name | TEXT | 插件名 |
| kind | TEXT | `style`\|`fx` |
| manifest_path | TEXT | manifest.json 相对路径 |
| dir | TEXT | 插件资源目录 |
| created_at | INTEGER | |

`DATA-PLUGIN-001`：Plugin 的 manifest MUST 符合 [plugin-manifest.schema.json](../70-plugins/plugin-manifest.schema.json)。

## 数据约束汇总

| ID | 约束 |
|---|---|
| `DATA-MODEL-001` | 元数据入 SQLite，slide 大文本入 fs；二者通过路径字段关联 |
| `DATA-SLIDE-001` | slide-json 符合 slide-json schema |
| `DATA-PLUGIN-001` | plugin manifest 符合 manifest schema |
| `DATA-VERSION-001` | 任意可编辑产物变更 MUST 产生新版本，支持回滚 |
| `DATA-MODEL-002` | 外键关系 MUST 在删除时级联或受保护（不留孤儿记录） |

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
# JSON Schema 校验：见 slide-json.schema.json / plugin-manifest.schema.json
```

## 依赖

- [ARCH-SYSTEM](../20-architecture/system-overview.md)、[ADR-0002](../90-decisions/0002-persistence-sqlite-fs.md)
