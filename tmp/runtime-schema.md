# Runtime Schema 清单

本文档描述 Agent Runtime 使用的结构化协议，不包含 `outline.json`、`design.json`、`spec.json`、`materialization.json` 等项目资源 Schema。

## 输入和路由

### WorkSpec

功能：

定义一次 Run 要解决什么问题、操作什么产物、作用于整份还是单页，以及采用什么交互方式。

意义：

它是 Runtime 的任务真相。Context、资源范围、工具权限和完成检查都从 WorkSpec 派生，避免各模块分别猜测用户意图。

结构：

```json
{
  "target": {
    "artifact": "presentation",
    "level": "slide",
    "slide_id": "sli_8n4wcp"
  },
  "interaction": {
    "intent": "execute"
  },
  "instruction": "把当前页改成数据对比页，并完成渲染验证",
  "options": {
    "language": "zh-CN",
    "theme_id": "tokyo-night",
    "desired_slide_count": 12
  }
}
```

### RunTarget

功能：

描述 Run 操作的产物类型和目标层级。

意义：

显式区分语义蓝图与最终演示产物，并禁止使用 `current` 等隐式目标。

结构：

```json
{
  "artifact": "spec",
  "level": "deck",
  "slide_id": ""
}
```

### RunInteraction

功能：

声明本次 Run 的交互意图。

意义：

它决定 Runtime 是只读沟通、提问、规划，还是允许真实写入。

结构：

```json
{
  "intent": "plan"
}
```

### Scope

功能：

将 WorkSpec 的目标转换成工具读写边界。

意义：

阻止 Agent 越权修改未声明的页面或产物。

结构：

```json
{
  "target": {
    "artifact": "presentation",
    "level": "slide",
    "slide_id": "sli_8n4wcp"
  }
}
```

## 上下文系统

### ContextPack

功能：

向 Agent 提供一次推理所需的完整、预算化上下文。

意义：

它把项目状态、目标资源、相关页面、设计、资产、记忆和版本信息组合成单一输入，避免 Agent 自行扫描整个项目。

结构：

```json
{
  "schema_version": "1.0",
  "profile": "presentation/slide",
  "work_spec": {},
  "project": {
    "id": "pro_k7m2qx",
    "title": "季度业务复盘"
  },
  "outline": {},
  "target": {},
  "related_slides": [],
  "design": {},
  "slide_html": {
    "summaries": {}
  },
  "assets": [],
  "memory": {},
  "recent_turns": [],
  "revisions": {},
  "manifest": {}
}
```

### ContextManifest

功能：

记录 ContextPack 的构建过程、预算使用、来源和裁剪结果。

意义：

为上下文可复现、调试和审计提供依据。

结构：

```json
{
  "context_id": "ctx_91bc2a",
  "run_id": "run_01",
  "thread_id": "thread_01",
  "project_id": "pro_k7m2qx",
  "profile": "presentation/slide",
  "read_only": false,
  "estimated_tokens": 12400,
  "budget_tokens": 20000,
  "output_reserve": 8000,
  "pack_hash": "sha256:91bc...",
  "segments": [],
  "refs": [],
  "dropped": [],
  "warnings": []
}
```

### ContextSegment

功能：

描述 ContextPack 中一个独立上下文片段。

意义：

让每段内容的来源、版本、优先级、成本和选择原因显式化。

结构：

```json
{
  "id": "segment_target",
  "kind": "target_artifact",
  "source_ref": "slide:sli_8n4wcp:spec",
  "revision": 7,
  "content_hash": "sha256:4a75...",
  "estimated_tokens": 840,
  "priority": 100,
  "selection_reason": "declared run target",
  "detail_level": "full",
  "required": true
}
```

### ContextIndex

功能：

建立当前 Run 可检索的上下文索引。

意义：

支持 Agent 按需检索，而不是把所有项目内容一次性塞进 Prompt。

结构：

```json
{
  "id": "ctxidx_91bc2a",
  "run_id": "run_01",
  "thread_id": "thread_01",
  "project_id": "pro_k7m2qx",
  "pack_hash": "sha256:91bc...",
  "built_at": 1786201200000000000,
  "items": []
}
```

### ContextIndexItem

功能：

描述一项可被检索的上下文内容。

意义：

统一管理检索摘要、关键词、向量、作用域、成本和新鲜度。

结构：

```json
{
  "ref_id": "ref_slide_market",
  "kind": "slide_spec",
  "source": "slides/sli_8n4wcp/spec.json",
  "target": {
    "type": "slide",
    "slide_id": "sli_8n4wcp",
    "part": "spec"
  },
  "revision": 7,
  "hash": "sha256:4a75...",
  "summary": "企业客户贡献主要增长",
  "keywords": ["企业客户", "增长"],
  "embedding": [0.12, -0.31],
  "token_cost": {
    "summary": 40,
    "full": 480
  },
  "available_levels": ["summary", "full"],
  "scope": {
    "target": {
      "artifact": "presentation",
      "level": "deck"
    }
  },
  "freshness": "current",
  "updated_at": 1786201200000000000
}
```

### RetrievalResult

功能：

返回一次主动上下文检索的结果。

意义：

让检索结果受 Scope 和 token budget 控制，并说明每项内容为什么被选中。

结构：

```json
{
  "query": "企业客户增长证据",
  "results": [
    {
      "ref_id": "ref_slide_market",
      "kind": "slide_spec",
      "source": "slides/sli_8n4wcp/spec.json",
      "target": {
        "type": "slide",
        "slide_id": "sli_8n4wcp",
        "part": "spec"
      },
      "revision": 7,
      "hash": "sha256:4a75...",
      "score": 0.92,
      "selection_reason": "keyword and embedding match",
      "detail_available": true,
      "detail_level": "summary",
      "snippet": "企业客户贡献主要增长",
      "freshness": "current",
      "estimated_tokens": 40
    }
  ],
  "estimated_tokens": 40,
  "remaining_budget": 1760,
  "index_ref": "ctxidx_91bc2a"
}
```

### ThreadMemory

功能：

保存同一 Thread 跨 Run 可复用的稳定信息。

意义：

避免每次执行都重新询问用户偏好、确认过的决策和已知事实。

结构：

```json
{
  "schema_version": "1.0",
  "revision": 4,
  "user_preferences": [],
  "confirmed_decisions": [],
  "brand_constraints": [],
  "content_facts": [],
  "open_questions": [],
  "recent_changes": [
    {
      "key": "run:run_01",
      "value": "将约束字段提升为顶层 requirements 和 prohibitions",
      "source_run_id": "run_01",
      "recorded_at": 1786201200
    }
  ]
}
```

## 规划和需求跟踪

### Plan

功能：

描述复杂任务的可追踪执行计划。

意义：

让多步骤执行具备明确状态、修订历史和完成约束。

结构：

```json
{
  "plan_id": "plan_01",
  "revision": 2,
  "explanation": "先更新契约，再迁移数据，最后验证",
  "steps": [],
  "created_at": 1786200000,
  "updated_at": 1786201200,
  "created_by_run": "run_01",
  "history": []
}
```

### PlanStep

功能：

描述计划中的一个执行步骤。

意义：

提供最小可跟踪工作单元，并限制同一时间最多一个步骤处于执行中。

结构：

```json
{
  "id": "step_schema",
  "title": "更新 Runtime Schema",
  "status": "in_progress"
}
```

### RequirementLedger

功能：

将用户指令拆解为完成门可以检查的要求集合。

意义：

防止 Runtime 只完成部分要求就提前结束。

结构：

```json
{
  "items": [
    {
      "id": "req_01",
      "text": "更新 Schema",
      "status": "satisfied",
      "evidence": ["changed:deck:outline"]
    }
  ]
}
```

### RequirementItem

功能：

表示一项运行时要求及其满足状态。

意义：

把自然语言要求与执行证据关联起来。

结构：

```json
{
  "id": "req_02",
  "text": "运行后端和前端测试",
  "status": "in_progress",
  "evidence": ["tool:test"]
}
```

## 工具和变更

### ToolSchema

功能：

描述动态披露给模型的一个工具。

意义：

让模型只看到当前策略、阶段和权限允许使用的能力。

结构：

```json
{
  "name": "write_ppt",
  "description": "Write one disclosed PPT resource",
  "parameters": {
    "type": "object",
    "properties": {
      "resource": {
        "type": "object"
      },
      "content": {
        "type": "string"
      }
    },
    "required": ["resource", "content"]
  }
}
```

### ToolResult

功能：

统一表示工具执行结果。

意义：

为 Agent observation、Runtime 错误处理、变更跟踪和前端事件提供统一输入。

结构：

```json
{
  "ok": true,
  "summary": "outline updated",
  "data": {},
  "changed_targets": [
    {
      "type": "deck",
      "part": "outline",
      "revision": 8,
      "hash": "sha256:4a75...",
      "fields": ["requirements", "prohibitions"],
      "insertions": 4,
      "deletions": 8
    }
  ],
  "issues": [],
  "retryable": false
}
```

### Resource

功能：

以领域语义定位一个模型可见资源。

意义：

避免向模型暴露磁盘路径、数据库 ID 和内部 Artifact 命名。

结构：

```json
{
  "type": "slide",
  "slide_id": "sli_8n4wcp",
  "part": "html"
}
```

### ArtifactRef

功能：

在 Runtime 内部精确定位一个真实产物。

意义：

连接领域资源、文件路径、版本快照和变更记录。

结构：

```json
{
  "kind": "slide_spec",
  "id": "sli_8n4wcp",
  "path": "slides/sli_8n4wcp/spec.json",
  "project_id": "pro_k7m2qx"
}
```

### ChangeSet

功能：

汇总一次 Run 创建、更新和删除的全部产物。

意义：

它是完成检查、提交、恢复和最终结果的共同变更来源。

结构：

```json
{
  "created": [],
  "updated": [],
  "deleted": [],
  "warnings": []
}
```

### ArtifactChange

功能：

描述单个产物在 Run 中发生的变化。

意义：

通过前后 hash 支持冲突检测、恢复和审计。

结构：

```json
{
  "artifact": {
    "kind": "outline",
    "id": "pro_k7m2qx",
    "path": "outline.json",
    "project_id": "pro_k7m2qx"
  },
  "before_hash": "sha256:1111...",
  "after_hash": "sha256:2222...",
  "source": "write_ppt",
  "tentative": true,
  "insertions": 4,
  "deletions": 8
}
```

## 证据和完成门

### Evidence

功能：

记录某个资源已经通过某类验证。

意义：

让 Runtime 依据可验证证据完成任务，而不是相信 Agent 的文字声明。

结构：

```json
{
  "id": "evi_01",
  "kind": "render",
  "target": {
    "type": "slide",
    "slide_id": "sli_8n4wcp",
    "part": "html"
  },
  "source_hash": "4a75...",
  "produced_at": 1786201200,
  "fresh": true,
  "data": {
    "overflow": false
  }
}
```

### MaterializationProof

功能：

在 Runtime 内部证明一次成功渲染使用了哪些精确输入。

意义：

防止旧渲染证据被用于提交已经变化的 HTML、Outline、Spec 或 Design。

结构：

```json
{
  "slide_id": "sli_8n4wcp",
  "html_revision": 3,
  "source_outline_revision": 5,
  "source_spec_revision": 7,
  "source_design_revision": 2,
  "artifact_hash": "4a75...",
  "source_hash": "sha256:91bc..."
}
```

### CompletionResult

功能：

表示 Completion Gate 是否接受本次完成请求。

意义：

集中判断计划、需求、变更、证据和渲染是否形成闭环。

结构：

```json
{
  "accepted": false,
  "issues": [
    {
      "code": "EVIDENCE_HTML_MISSING",
      "summary": "HTML evidence is missing",
      "required_actions": []
    }
  ]
}
```

### CompletionIssue

功能：

描述一个阻止 Runtime 完成的结构化问题。

意义：

向 Agent 提供明确、可执行的修复动作，而不是模糊地要求重试。

结构：

```json
{
  "code": "ASYNC_SPEC_HTML",
  "summary": "Spec changes are not materialized in HTML",
  "required_actions": [
    {
      "tool": "edit_ppt",
      "target": {
        "type": "slide",
        "slide_id": "sli_8n4wcp",
        "part": "html"
      }
    },
    {
      "tool": "render_slide",
      "target": {
        "type": "slide",
        "slide_id": "sli_8n4wcp",
        "part": "html"
      }
    }
  ]
}
```

### SemanticReviewInput

功能：

向独立语义审查模型提供完成候选及其上下文。

意义：

补充规则型 Completion Gate 无法可靠判断的语义完整性和质量问题。

结构：

```json
{
  "run_id": "run_01",
  "finish_call_id": "call_finish_01",
  "work_spec": {},
  "requirement_ledger": {},
  "plan": {},
  "changes": {},
  "gate_result": {},
  "evidence": [],
  "latest_issues": [],
  "context_briefing": "Current task and constraints",
  "retrieved_context": [],
  "candidate_message": "已完成修改并通过验证",
  "focus": "completion",
  "rubric": "Review whether all explicit requirements are satisfied"
}
```

### SemanticReviewResult

功能：

返回语义审查模型的结构化判断。

意义：

只允许受控的审查代码和具体说明进入 Runtime 决策。

结构：

```json
{
  "checks": [
    {
      "code": "REVIEW_PASS",
      "summary": "所有明确要求均已完成，变更与验证证据一致。"
    }
  ]
}
```

## 恢复和生命周期

### RuntimeCheckpoint

功能：

保存 Runtime 在关键边界上的可恢复快照。

意义：

使等待用户输入、进程中断、模型重试或服务重启后能够继续执行，而不是从头开始。

结构：

```json
{
  "run_id": "run_01",
  "loop_id": "loop_01",
  "boundary": "after_render",
  "phase": "completion_check",
  "resume_phase": "completion_check",
  "plan": {},
  "requirements": {},
  "changes": {},
  "evidence": [],
  "context_index_ref": "ctxidx_91bc2a",
  "context_briefing": "Current task summary",
  "message_summary": [],
  "latest_tool_results": [],
  "turns": 12,
  "tool_calls": 8,
  "waiting_question_id": "",
  "provider_continuation": {
    "provider": "openai",
    "model": "gpt-5",
    "hash": "4a75..."
  },
  "completion_failures": 0,
  "created_at": 1786201200000000000
}
```

### RecoverySnapshot

功能：

汇总恢复时对当前文件状态的检查结果。

意义：

判断文件是当前 Run 已写入、仍保持基线、被外部修改，还是已经丢失。

结构：

```json
{
  "checkpoint_id": "checkpoint_01",
  "run_id": "run_01",
  "results": []
}
```

### ArtifactReconciliation

功能：

描述一个产物在恢复阶段的比对结果。

意义：

防止恢复流程覆盖用户或其他进程在 Run 中断后产生的外部修改。

结构：

```json
{
  "artifact": {
    "kind": "outline",
    "id": "pro_k7m2qx",
    "path": "outline.json"
  },
  "status": "external_modified",
  "expected_hash": "sha256:1111...",
  "current_hash": "sha256:2222..."
}
```

### StructuredOutcome

功能：

表示 Runtime 执行结束后的统一结果。

意义：

向 Service 层提供与模型文字无关的机器可读终态。

结构：

```json
{
  "loop_id": "loop_01",
  "phase": "terminal",
  "status": "completed",
  "target": {
    "artifact": "presentation",
    "level": "slide",
    "slide_id": "sli_8n4wcp"
  },
  "changes": {},
  "issues": [],
  "summary": "Slide updated and rendered",
  "message": "已完成修改并通过验证"
}
```

## 公共事件和 HITL

### PublicEventBase

功能：

为所有前端 SSE 事件提供统一信封。

意义：

让前端能够按版本、Run 和时间顺序稳定消费事件。

结构：

```json
{
  "schema_version": 2,
  "run_id": "run_01",
  "occurred_at": "2026-08-09T14:30:00Z"
}
```

### RunStartedPayload

功能：

通知前端 Run 已经开始。

意义：

固定前端时间线的任务目标、交互意图和原始用户输入。

结构：

```json
{
  "schema_version": 2,
  "run_id": "run_01",
  "occurred_at": "2026-08-09T14:30:00Z",
  "target": {
    "artifact": "presentation",
    "level": "slide",
    "slide_id": "sli_8n4wcp"
  },
  "interaction": {
    "intent": "execute"
  },
  "user_input": "重做当前页"
}
```

### RunProgressPayload

功能：

通知前端 Runtime 当前执行阶段和进度。

意义：

让长任务具备可观察的阶段反馈。

结构：

```json
{
  "schema_version": 2,
  "run_id": "run_01",
  "occurred_at": "2026-08-09T14:30:05Z",
  "stage": "rendering",
  "text": "正在渲染页面",
  "target": {
    "type": "slide",
    "slide_id": "sli_8n4wcp",
    "part": "html"
  },
  "progress": {
    "current": 1,
    "total": 1,
    "unit": "slide"
  }
}
```

### RunFinishedPayload

功能：

通知前端 Run 的最终状态。

意义：

统一表达成功、失败、取消、受影响目标和错误信息。

结构：

```json
{
  "schema_version": 2,
  "run_id": "run_01",
  "occurred_at": "2026-08-09T14:31:00Z",
  "status": "completed",
  "affected_targets": [],
  "duration_ms": 60000
}
```

### PlanUpdatedPayload

功能：

向前端同步 Runtime 的最新计划。

意义：

让用户看到复杂任务的执行步骤和当前状态。

结构：

```json
{
  "schema_version": 2,
  "run_id": "run_01",
  "occurred_at": "2026-08-09T14:30:10Z",
  "plan": {
    "plan_id": "plan_01",
    "revision": 2,
    "explanation": "更新契约并迁移",
    "steps": [
      {
        "id": "step_schema",
        "title": "更新 Schema",
        "status": "completed"
      }
    ]
  }
}
```

### ToolStartedPayload

功能：

通知前端一个工具调用已经开始。

意义：

提供工具级进度和展示信息，但不泄露内部参数或敏感路径。

结构：

```json
{
  "schema_version": 2,
  "run_id": "run_01",
  "occurred_at": "2026-08-09T14:30:20Z",
  "call_id": "call_01",
  "tool": "render_slide",
  "plan_step_id": "step_render",
  "target": {
    "type": "slide",
    "slide_id": "sli_8n4wcp",
    "part": "html"
  },
  "display": {
    "label": "渲染页面",
    "detail": "企业客户增长"
  }
}
```

### ToolCompletedPayload

功能：

通知前端一个工具调用已经结束。

意义：

统一表达工具状态、预览和公开错误。

结构：

```json
{
  "schema_version": 2,
  "run_id": "run_01",
  "occurred_at": "2026-08-09T14:30:30Z",
  "call_id": "call_01",
  "tool": "render_slide",
  "status": "completed",
  "target": {
    "type": "slide",
    "slide_id": "sli_8n4wcp",
    "part": "html"
  },
  "display": {
    "label": "页面渲染完成"
  },
  "preview": {
    "slide_id": "sli_8n4wcp",
    "image_url": "/api/v1/runs/run_01/screenshots/shot_01",
    "warnings": []
  }
}
```

### QuestionAskedPayload

功能：

向用户发起单题或多题结构化问题。

意义：

让 Runtime 在信息不足时显式等待用户决定，而不是自行推断。

结构：

```json
{
  "schema_version": 2,
  "run_id": "run_01",
  "occurred_at": "2026-08-09T14:30:40Z",
  "question_id": "question_01",
  "header": "确认视觉方向",
  "prompt": "请选择整体方向",
  "selection": "single",
  "options": [
    {
      "id": "formal",
      "label": "正式商务",
      "description": "克制、清晰、适合管理层汇报"
    }
  ],
  "allow_custom": true,
  "questions": []
}
```

### QuestionAnsweredPayload

功能：

记录用户对结构化问题的回答。

意义：

为恢复执行和前端回放提供稳定答案结构。

结构：

```json
{
  "schema_version": 2,
  "run_id": "run_01",
  "occurred_at": "2026-08-09T14:31:00Z",
  "question_id": "question_01",
  "answer": {
    "selected_option_ids": ["formal"],
    "custom_text": "",
    "answers": []
  },
  "display_text": "正式商务"
}
```

### MessageReasoningPayload

功能：

向前端输出可公开的阶段性推理说明。

意义：

提供必要透明度，但不暴露模型内部隐藏推理。

结构：

```json
{
  "schema_version": 2,
  "run_id": "run_01",
  "occurred_at": "2026-08-09T14:30:15Z",
  "message_id": "message_01",
  "text": "先同步页面语义蓝图，再更新 HTML。"
}
```

### MessageMilestonePayload

功能：

通知前端一个阶段性目标已经完成。

意义：

把长任务拆成用户可感知的阶段成果。

结构：

```json
{
  "schema_version": 2,
  "run_id": "run_01",
  "occurred_at": "2026-08-09T14:30:35Z",
  "message_id": "message_02",
  "text": "页面结构与内容已更新。",
  "completed_step_ids": ["step_schema", "step_html"]
}
```

### MessageFinalPayload

功能：

向前端输出 Run 的最终用户可见回复。

意义：

将最终说明与实际受影响目标绑定。

结构：

```json
{
  "schema_version": 2,
  "run_id": "run_01",
  "occurred_at": "2026-08-09T14:31:00Z",
  "message_id": "message_final",
  "text": "已完成页面修改并通过渲染验证。",
  "affected_targets": [
    {
      "type": "slide",
      "slide_id": "sli_8n4wcp",
      "part": "html",
      "display_name": "企业客户增长"
    }
  ]
}
```
