# Runtime Prompt Contract Refactor Design

> 日期：2026-08-23  
> 状态：已确认，实施中  
> 范围：Runtime Prompt 分层、按模式与 Scope 装配、工具披露、Plan 完成协议和重复规则治理。

## 1. 背景与结论

现有 Runtime Prompt 已按 core、mode、playbook、guide、rubric 和 resource contract 拆分，但实际装配仍把全部质量、修复和资源模块发送给所有模式，并把用户目标、需求、批准计划和运行状态拼入 system Prompt。部分模式规则还与工具披露和 Completion Gate 不一致。

本期采用“共享可信核心 + 模式策略 + 场景 Playbook + 相关资源契约 + 动态任务上下文”的结构。不会新增独立 `tool_use_policy`，也不会为不同模型供应商复制 PPT 业务 Prompt。

## 2. 可信边界与消息分层

- system Prompt 只包含由应用维护的静态规则：Runtime 核心、用户输出规则、当前模式策略、当前场景 Playbook，以及按需选择的修复、完成、质量和资源契约模块。
- 用户指令只出现于 user 消息。项目上下文、Requirement Ledger、Approved Plan、Context Briefing 和 Runtime State 作为带来源说明的结构化动态数据放在同一 user 层。
- 动态数据不能改变模式、Scope、工具披露、系统策略或运行时权限。Approved Plan 是 Execute 的任务合同，但不能授予额外工具或扩大 Scope。
- Context Briefing 只保留派生的工作集和下一步关注点，不复制用户原始 Objective 或 Requirement 文本。

## 3. 模式与完成协议

- Talk：只读分析，可以通过 `finish` 交付答案。
- Ask：只读协作，可以提问，并在回答完整后通过 `finish` 交付。
- Plan：只读规划；新提案必须使用 `create_plan`，修订必须使用 `update_plan`。Plan 阶段不披露 `finish`，Completion Gate 也不接受 Plan 的 finish。
- Execute：唯一写入模式；可选维护执行计划，通过 `finish` 进入 Completion Gate。
- 模型没有产生工具调用时，Runtime 给出的继续指令必须按模式生成，不能要求 Plan 调用不可见的 `finish`。

## 4. 条件 Prompt 装配

所有模式加载：

1. `core_runtime_policy`
2. `user_facing_output`
3. 当前 mode policy
4. 当前 task playbook

额外装配：

- Talk / Ask：`finish_contract`。
- Plan：`ppt_quality_rubric` 和相关资源契约；不加载 completion repair 或 finish contract。
- Execute：`completion_repair_guide`、`finish_contract`、`ppt_quality_rubric` 和相关资源契约。

资源契约按 Scope 裁剪：单页 Spec 只注入 Slide Spec；Deck 或整套 PPT 工作注入 Outline、Design 和 Slide Spec。Talk / Ask 不注入写入 Schema。

## 5. 工具披露

- Talk / Ask / Plan 只披露 `read_ppt` 与 `search_refs` 等只读事实工具；不披露渲染和写入工具。
- Execute + Spec 不披露 `render_slide`。
- Execute + PPT 披露读取、检索、写入、精确编辑和渲染工具。
- `read_ppt`、`write_ppt`、`edit_ppt` 的 resource 参数 Schema 根据读写权限和当前 Scope 裁剪；单页 Scope 的 `slide_id` 固定为当前稳定 ID。
- 工具描述和参数 Schema 是工具职责与调用形态的唯一权威来源。

## 6. Prompt 职责归口

- core：ReAct 循环、Observation 真值、只使用已披露工具和控制动作隔离。
- mode：当前模式的授权、交互和终止协议。
- playbook：当前任务的资源顺序与场景策略，不重复全局权限和完成规则。
- resource contracts：资源所有权和当前相关 JSON 模型契约。
- quality rubric：叙事、视觉、HTML、可访问性和渲染质量。
- completion repair guide：Completion Gate 错误后的修复映射。
- finish contract：通过 `finish` 交付的完整性和用户表达。
- user-facing output：内部术语隔离。

原 `ppt_business_policy` 中的内容分别归入上述权威模块后删除，不保留第二份定义。

## 7. Provider 策略

OpenAI、Anthropic、DeepSeek 共用一套 PPT 业务 Prompt。Provider Adapter 只处理传输层或已由评测证明的模型差异，例如 reasoning continuation、严格结构化输出、图片 detail 或并行调用能力；不得复制 mode、playbook、quality 或 resource contract。

## 8. 非目标

- 不增加 `read_ref`，不修改 `read_ppt` 的读取粒度。
- 不向视觉 Reviewer 增加截图或页面正文。
- 不实现新的 Context Compactor。
- 不把语义或视觉 Reviewer 改为 Runtime 强制步骤。
- 不修改 Requirement Ledger 的满足算法。
- 不新增工具，不修改 PPT 数据结构。

## 9. 验收标准

- system Prompt 不含用户原始指令、Requirement Ledger、Approved Plan、Context Briefing 或 Runtime State。
- 动态任务上下文在 user 层结构化注入，并明确其不可信边界。
- Plan 不披露或接受 `finish`，新提案只能进入现有 create-plan / approval 流程。
- 不同 mode/scope 的 Prompt 模块和工具 Schema 只包含相关内容。
- 每条跨场景不变量只有一个 Prompt 权威归口，`ppt_business_policy` 被移除。
- Runtime Prompt 仍为 Provider 无关的单一业务来源。
- Context Engine、Workflow 和 Schema 相关测试通过。
