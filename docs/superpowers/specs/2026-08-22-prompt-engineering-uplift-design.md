# Prompt 工程改进设计（本期）

> 历史状态：本文提出的增量方案已被 `2026-08-23-runtime-prompt-contract-refactor-design.md` 取代。当前实现不新增独立 `tool_use_policy`，以可信消息分层、条件装配和唯一规则归口为准。

适用范围：`backend/prompts/runtime/**` 的系统提示词模块，以及与其耦合的 `prompt_modules.go`、`registry.go`。

本期已落地：内部术语治理的 Layer 1（前端语义化映射）、Layer 2（Prompt 输出契约）、Layer 3（后端 sanitize 兜底）。本文档在此基础上，依据 DeepSeek Harness 与 Anthropic（Claude Code）公开工程实践，对现有 prompt 结构做一次缺口评估，给出本期可一并设计的增量。

---

## 一、结论：现有结构评估

现有模块（装配顺序见 `prompt_modules.go`）：
`core_runtime_policy` → `user_facing_output` → `mode(plan/execute/talk/ask)` → `playbook_*` → `completion_repair_guide` → `finish_contract` → `ppt_quality_rubric` → `ppt_business_policy` → `resource_contracts` → 动态（context_briefing / approved_plan / runtime_state）。

已覆盖良好（无需改动）：
- 身份与运行策略（`core_runtime_policy`）——单 ReAct 循环、工具披露、模式读写约束，与 DeepSeek 的 identity/persona 分层一致。
- 契约化闭环（`finish_contract` + `completion_repair_guide` + `resource_contracts`）——与 DeepSeek「不变量接入顶层 gate 并给出违例样例」思路高度吻合。
- 静态前缀在前、动态上下文在后，符合 prefix-cache 与「避免 lost in the middle」。

调研判定：结构成熟度属业界中上。以下为常见但当前缺失/偏弱的要素。

---

## 二、缺口与增量设计

按优先级排列。每条给出「指令 —— 理由」，并标注落点模块。

### P0-1 工具使用与错误措辞规范（新增 `tool_use_policy`）

现状：工具选择与错误处理散落在 `core_runtime_policy`、`execute.md`、`completion_repair_guide` 中，无独立分区。

设计（新增 `core/tool_use_policy.md`，装配位置紧跟 `resource_contracts`）：
- 工具选择判据 —— 每个工具「目的唯一」，`read_ppt` 只读、`edit_ppt` 用于唯一锚点小改、`write_ppt` 用于整体重建；理由：避免模型在重叠能力间摇摆（Anthropic writing-tools-for-agents）。
- 错误自纠流程 —— 工具返回 repairable 错误时，先读取当前真值再重试，不要盲目重发相同参数；理由：多数「参数无效」源于对当前状态的过期假设（DeepSeek tool-catalog）。
- 内部标识不外泄 —— 工具名、错误码、资源键仅用于内部推理，面向用户改用产品语言（复用 `user_facing_output` law，此处只做交叉引用，避免双写）。

非目标：不重复列举各工具签名（已在 `resource_contracts`）。

### P0-2 生成后自查（增强 `execute.md` + 复用 `ppt_quality_rubric`）

现状：质量保证依赖 `completion_repair_guide` 的「被动兜底」（finish 被拒后才修）。

设计（在 `execute.md` 的 Completion 段前新增 self-check 指令）：
- 在 finish 之前，对照 `ppt_quality_rubric` 做一次显式核对，列出发现的差异并就地修复，而非等待 Completion Gate 拒绝；理由：Anthropic 称主动自查为「最高杠杆项」，可显著降低返工轮次。
- 自查聚焦根因而非症状（如溢出应改版式约束，而非只缩字号）。

注意：不引入新工具，不改变 Completion Gate 语义，仅前移一次自查动作。

### P1-1 上下文预算与检索策略（新增 `context_policy`）

现状：无独立的 context engineering 分区；`search_refs` 的使用时机分散。

设计（新增 `core/context_policy.md`）：
- Just-in-time 检索 —— 需要时用 `search_refs`/`read_ppt` 拉取，不预载全量；传页 ID/结构位置而非整份内容进入推理；理由：token 是有限注意力预算，context rot 随长度上升（Anthropic effective-context-engineering）。
- 大产物落盘不入正文 —— HTML/JSON 产物通过资源读写，不在推理里全文复述。
- 最小高信号集 —— 只读执行当前改动所必需的资源（与 `execute.md` 的「Read only the resources needed」呼应，此处上升为通用律，`execute.md` 改为引用而非重复表述）。

### P1-2 输出契约的正反例（增强 `user_facing_output`）

现状：`user_facing_output.md` 已含 2 组 ✅/❌ 示例，质量良好。

设计（小幅增强）：
- 再补 1 组「计划标题/步骤标题」的正反例（因 `plan.md` 已约束但缺示例）；理由：Anthropic 建议用少量 canonical 示例表达期望，胜过堆砌规则。
- 保持「精选示例、不堆 edge case」，总量控制，避免 prompt 膨胀。

### P2-1 停止与升级条件（增强 `finish_contract`）

现状：`finish_contract` 偏「如何完成」，`completion_repair_guide` 覆盖部分终止码，但缺「何时应主动停下求助」的正向定义。

设计（在 `finish_contract` 增补 stop/escalate 段）：
- 明确「应停并求助」的场景：关键决策信息不足、越权目标、连续修复预算耗尽（对齐已有 `CONSECUTIVE_TOOL_ERRORS`/`COMPLETION_*_REPEAT` 终止码）。
- 用 `ask_user` 而非猜测继续；理由：DeepSeek 的 cancel cause 枚举与「缺信息即问」是清晰的终止契约模板。

### P2-2 措辞与去冗余风格律（并入 `core_runtime_policy`）

设计（在 `core_runtime_policy` 末尾补一条写作律，1-3 句）：
- 用直接、具体措辞，不用比喻；一处事实只有一个归属（one home per fact），避免跨模块双写；理由：DeepSeek AGENTS 写作规范，可减少提示词自相矛盾与冗余（与用户「去冗余、消除双写」的文档偏好一致）。

---

## 三、本期建议范围（收敛）

考虑本期已完成 Layer 1/2/3，建议本期只增量落地「低风险、与治理主题强相关」的三项，其余转下期：

| 项 | 本期 | 理由 |
|---|---|---|
| P0-1 tool_use_policy | ✅ 建议做 | 与内部术语治理同源，收拢工具/错误措辞 |
| P0-2 execute 自查前移 | ✅ 建议做 | 纯 prompt 改动，零架构风险，收益高 |
| P1-2 输出契约正反例 | ✅ 建议做 | 直接强化本期 Layer 2 效果 |
| P1-1 context_policy | ⏭ 下期 | 涉及检索策略，需与 context engine 一并评估 |
| P2-1 stop/escalate | ⏭ 下期 | 需与 Completion Gate 终止码统一梳理 |
| P2-2 写作风格律 | ⏭ 下期 | 收益低，可随下次 core 修订顺带 |

## 四、验收标准

- 新增/修改的 prompt 模块通过 `registry.go` 注册并在 `prompt_modules.go` 装配，`go test ./backend/internal/workflow/` 全绿。
- 不引入新工具、不改 Completion Gate 语义、不改资源契约对象形态。
- 无跨模块双写：新律以「交叉引用」方式指向 `user_facing_output` / `resource_contracts`，不复制其正文。
- 面向用户文本零内部术语（Layer 2 契约 + Layer 3 兜底双保险）。

## 五、非目标

- 不重构 prompt 装配机制（不引入 DeepSeek 式插件化 PromptSection order）。
- 不引入子代理（当前为单 ReAct 循环，`subagent_policy` 暂不适用）。
- 不改动前端。

## 来源

- Anthropic: effective-context-engineering-for-ai-agents / writing-tools-for-agents / building-effective-agents / claude-code best-practices
- DeepSeek Harness: docs/subsystems/system-prompt.md、core.md、tool-catalog.md、AGENTS.md（v0.1 预览，以架构/契约描述为主）
