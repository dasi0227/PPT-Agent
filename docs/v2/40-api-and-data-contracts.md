---
id: V2-CONTRACTS
title: Artifact Target 与 Blueprint 数据契约
status: implemented
owner: shared
depends_on: [V2-INDEX, API-SSE, API-REST, DATA-MODEL]
verifies: [R0]
---

# Artifact Target 与 Blueprint 数据契约

R0 以“目标产物 + 目标层级 + 交互意图”取代旧的 action/scope/mode 公共协议。仓库仍复用既有 Run 外壳、SSE、HTML 生成器和安全预览运行时；`render` 继续是预览 HTTP 能力，不是 Agent target。

## 1. 创建 Run

`POST /api/v1/threads/{thread_id}/runs`

```json
{
  "target": {
    "artifact": "presentation",
    "level": "slide",
    "slide_id": "stable-slide-id"
  },
  "interaction": {
    "intent": "apply",
    "clarification": "when_blocked"
  },
  "instruction": "把这一页改成对比结构",
  "options": {
    "language": "zh-CN",
    "theme_id": "swiss-modern",
    "desired_slide_count": 8
  }
}
```

枚举：

- `target.artifact`: `blueprint | presentation`
- `target.level`: `slide | deck`
- `interaction.intent`: `apply | consult`
- `interaction.clarification`: `when_blocked | before_apply | never`

约束：

- `slide` 必须带属于当前 thread/project 的稳定 `slide_id`。
- `deck` 不得带 `slide_id`。
- `current` 只存在于前端临时选择；发送前解析成稳定 ID。
- project 从 thread 反查，请求体不接受 `project_id`。
- `consult` 只有只读 capability，不提交文件或 revision。
- 非法组合返回结构化 `422 INVALID_TARGET`；不存在的页返回 `400 SLIDE_NOT_FOUND`。

响应只返回标准化 target/interaction，不返回旧 action/scope/mode：

```json
{
  "id": "run-id",
  "thread_id": "thread-id",
  "project_id": "project-id",
  "status": "pending",
  "events_url": "/api/v1/runs/run-id/events",
  "target": {"artifact": "presentation", "level": "slide", "slide_id": "stable-slide-id"},
  "interaction": {"intent": "apply", "clarification": "when_blocked"}
}
```

## 2. 四种执行路径

| target | Runner 语义 |
|---|---|
| `blueprint/deck` | 创建或修改目标、受众、叙事、目录与稳定页序 |
| `blueprint/slide` | 修改单页 role、key message、语义内容和 visual intent |
| `presentation/deck` | 物化整份 HTML，或修改已有整份 presentation |
| `presentation/slide` | 物化尚未生成的单页，或修改已有单页 HTML |

`materialize/revise` 是 Runner 根据 HTML 与 revision 状态推导的内部 operation，不是公共 kind。

## 3. Blueprint 文件布局

```text
project/
├── deck.json
├── design/
│   └── design-spec.json
└── slides/
    └── <stable-slide-id>/
        ├── slide.json
        └── index.html
```

- `deck.json` 保存 project 目标、受众、核心论点、叙事、section/subsection 与 `outline_order`。
- `design/design-spec.json` 保存画布、色板、字体、间距、圆角、阴影、layout system、signature 与 motion。
- 每页 `slide.json` 保存 role、title、key message、语义内容、visual intent 与 speaker notes。
- schema 版本为 `2.0`；写入采用临时文件、fsync、原子替换。

Blueprint API：

- `GET /projects/{id}/blueprint`
- `PATCH /projects/{id}/blueprint`
- `PATCH /projects/{id}/blueprint/design`
- `GET /slides/{id}/blueprint`
- `PATCH /slides/{id}/blueprint`

PATCH 必须携带 `expected_revision`；冲突返回 `409 BLUEPRINT_REVISION_CONFLICT`。

## 4. Revision 与物化状态

SQLite 保存 deck/design/per-slide blueprint revision，以及 presentation 的 source deck/blueprint/design revision。前端消费的派生状态：

- `not_materialized`
- `fresh`
- `blueprint_stale`
- `design_stale`
- `unknown`

成功写入后才增加 revision。失败、取消和 consult 不提交 revision；单页 Blueprint 修改只使对应页 stale；deck 或 design 更新通过 source revision 使相关页 stale。

## 5. 事件与 history

Run 在解析 thread/project 并验证 WorkSpec 后，必须先完成 Context Engineering v1
组装与 manifest 持久化，再进入 Runner。SSE 与 history replay 增加：

```json
{
  "type": "context.assembled",
  "context_id": "ctx_opaque",
  "profile": "presentation/slide",
  "estimated_tokens": 6200,
  "budget_tokens": 12000,
  "segments": 9,
  "refs": 3,
  "warnings": [],
  "read_only": false
}
```

该事件只公开状态与 warning，不公开 ContextPack、完整 HTML、Thread Memory 或 ref 正文。
`consult` 使用同一 target profile，但 `read_only=true`。manifest 存入 `run_contexts`；
大型正文仅能用当前 Run manifest 中的 opaque `ref_id` 通过 `read_context_ref` 展开。

`run.started` 携带 `target`、`interaction` 和 `user_input`。`done.result` 至少携带：

```json
{
  "target": {"artifact": "presentation", "level": "slide", "slide_id": "stable-slide-id"},
  "interaction": {"intent": "apply", "clarification": "when_blocked"},
  "operation": "materialize-or-revise",
  "affected_slide_ids": ["stable-slide-id"],
  "materialization_status": "fresh",
  "revisions": {"deck": 3, "design": 2}
}
```

上述字段进入 `history.jsonl`，刷新后的 replay 可恢复用户目标和结果摘要。现有 `plan/plan.update`、SSE 续传和终态唯一性保持不变。

## 6. 开发期迁移策略

本项目处于开发阶段，R0 直接覆盖旧公共协议，不提供旧 kind/scope/mode 请求兼容。开发数据库需要按新 migration 重新创建。

旧 project 文件仍采用幂等内容迁移，以保护测试样例和本地作品：

1. 保留稳定 slide ID。
2. 旧 `slide.json` 备份为 `slide.legacy.json`。
3. 生成 `deck.json`、v2 per-slide blueprint 与 `design/design-spec.json`。
4. 重复打开不重复建页、不改 ID、不无意义增加 revision。

## 7. 验证

```bash
cd backend
go test ./...
go vet ./...

cd ../frontend
pnpm test -- --run
pnpm tsc
pnpm build
pnpm lint
```
