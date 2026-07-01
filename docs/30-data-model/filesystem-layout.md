---
id: DATA-FS-LAYOUT
title: 文件系统布局
status: approved
owner: backend
depends_on: [DATA-MODEL]
verifies: []
---

# 文件系统布局（work_dir）

## 原则

- **每个 PPT = 一个 Project = 一个 `work_dir`**（`PPT_WORK_ROOT/<project_id>/`）。多 PPT = 多目录，物理隔离。
- 每个 project 下可有多条 **thread**（对话线程），thread 只隔离对话历史，**共享**该 project 的产物文件。
- `state.json` 位于 project work_dir 根（参考用户偏好：work_dir 锚定 + state 在根）。
- slide 大文本（html/css/js/json）入文件系统；元数据入 SQLite。
- 公共样式层（design tokens）独立成文件，与单页 html 物理分离 → 支撑 `/page` 与 `/overview` 隔离。
- 版本快照集中存放，便于回滚与 git diff。
- 个人仓库 `_assets/` 在 `PPT_WORK_ROOT` 根、**跨所有 project 共享**（收藏的资产要能用于任意 PPT）。

## 目录结构

```
<PPT_WORK_ROOT>/
├── <project_id_A>/                  # 一个 PPT 的 work_dir（多 PPT = 多个此目录）
│   ├── state.json                   # 单一状态游标 + 项目级元信息（见下）
│   ├── project.json                 # project 概要（页顺序、主题引用）
│   ├── threads/                     # ★ 对话线程历史（一 project 多 thread）
│   │   ├── <thread_id>.jsonl        #   一条 thread 的追加式对话历史（可恢复）
│   │   └── ...
│   ├── common/
│   │   ├── tokens.css               # 公共样式层（design tokens）★ /overview 改这里
│   │   ├── base.css                 # 共享基础样式 + 16:9 舞台
│   │   └── runtime.js               # 预览运行时（切页/分步/缩放）
│   ├── slides/
│   │   ├── 000/{slide.json, index.html, slide.css?, slide.js?}
│   │   ├── 001/
│   │   └── ...
│   ├── versions/
│   │   ├── slide-000/{v0.html, v1.html}
│   │   ├── design/{v0.css, v1.css}
│   │   └── project/{v0.json}
│   └── assets/img/                  # 项目内图片等静态资源
│
├── <project_id_B>/                  # 另一个 PPT，完全独立的 work_dir
│   └── ...
│
└── _assets/                         # 个人仓库（全局，跨 project 共享；预置 + 用户新增同处）
    └── <asset_id>/
        ├── manifest.json            # 统一信封，符合 asset-manifest schema
        ├── template.html            # layout/component 的 html 载荷（可选）
        ├── style.css                # 可选
        ├── tokens.css               # theme 的 token 全集载荷（kind=theme）
        └── effect.js                # fx 的特效脚本（kind=fx）
```

## 多项目切换与隔离

- **新建 PPT**：`POST /projects` → 后端在 `PPT_WORK_ROOT/<新 id>/` 建全新 work_dir。
- **切换 PPT**：请求路径带不同 `project_id`（`/projects/{id}/...`）；work_dir 各自独立，互不可见。
- **隔离双边界**：① 文件系统各自目录；② SQLite 各表 `project_id` 外键，删项目级联清理 + 删 work_dir 目录（`DATA-MODEL-003`）。
- **thread 共享产物**：同 project 的多 thread 读写同一 `slides/`、`common/`；靠[每 project 执行锁](../20-architecture/agent-runtime.md)串行化避免打架。

## state.json 结构（单一状态游标）

```json
{
  "project_id": "uuid",
  "title": "云原生可观测性实践",
  "current_state": "editing",
  "theme": "tokyo-night",
  "slide_count": 8,
  "language": "zh",
  "updated_at": 1750000000
}
```

约束：用单一 `current_state` 表达进度，**不引入** `steps_done` 等冗余标志（避免「双重真相」）。

## 路径约定

| 文件 | SQLite 字段 | 说明 |
|---|---|---|
| `common/tokens.css` | `projects.design_path` | 公共样式层 |
| `slides/<idx>/slide.json` | `slides.json_path` | slide-json |
| `slides/<idx>/index.html` | `slides.html_path` | slide html |
| `versions/...` | `versions.snapshot_path` | 版本快照 |
| `_assets/<id>/manifest.json` | `assets.manifest_path` | 资产清单 |

| ID | 约束 |
|---|---|
| `DATA-FS-001` | `state.json` MUST 位于 work_dir 根 |
| `DATA-FS-002` | 公共样式层 MUST 独立于单页 html 文件 |
| `DATA-FS-003` | 页目录以 0 填充三位序号命名（`000`/`001`），与 `slides.idx` 对应 |
| `DATA-FS-004` | 写 `state.json` MUST 串行化（并发安全） |

## 验收标准（Given-When-Then）

- **AC-FS-002**（`DATA-FS-002`）
  - GIVEN 一个生成完成的项目
  - WHEN 执行 `/overview` 改主色
  - THEN 仅 `common/tokens.css` 变化，`slides/*/index.html` 不变

## 校验方式

```bash
# 结构校验脚本：检查 state.json 位置、页目录命名、公共层分离
node scripts/check-workdir.mjs <work_dir>
```

## 依赖

- [DATA-MODEL](data-model.md)
