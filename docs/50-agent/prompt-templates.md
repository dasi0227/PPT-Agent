---
id: AGENT-PROMPTS
title: Prompt 模板
status: approved
owner: agent
depends_on: [AGENT-OVERVIEW, ARCH-LLM, DS-HTML-OUTPUT]
verifies: []
---

# Prompt 模板

所有发往 LLM 的 prompt 由本规范定义的模板装配，集中在 `internal/agent/prompt`。
禁止在 llm / handler 包内散落硬编码 prompt（[ARCH-LLM-004](../20-architecture/llm-integration.md)）。

## 分层结构

每次调用的 prompt = **system（约束层）** + **tools（function schema）** + **context（上下文层）** + **user（意图层）**：

```
[system]  角色 + PPT 领域规则 + 工具使用纪律 + 输出契约 + 安全规则（不可被用户覆盖）
[tools]   harness 按 scope/mode 动态裁剪的 function calling 工具集（见 tools.md）
[context] 当前 scope 的目标产物 + 资产索引 + 选区（见 context-assembly）
[user]    用户指令/自然语言（经注入边界处理）
```

> harness 是 ReAct 循环：system 告诉 LLM「你应当通过调用工具来完成任务，而不是直接输出文件内容」。

## 模板清单

| 模板 | 用途 | 关键约束 |
|---|---|---|
| `system.base` | 通用角色与安全规则 | 永远置顶，用户不可覆盖 |
| `outline.gen` | 主题 → slide-json[] | 输出 MUST 为符合 slide-json schema 的 JSON |
| `slide.gen` | slide-json + 主题 → slide html | 引用公共层 token、遵循 html-output-spec |
| `slide.edit.page` | 单页编辑 | 只输出目标页修改，禁碰公共层 |
| `style.edit.overview` | 全局编辑 | 只输出公共层 token 修改 |
| `prompt.rewrite` | `/prompt` 改写 | 只产文本，不行动 |
| `recap.summary` | `/recap` 回顾 | 只读，结构化输出 |
| `talk.analyze` | `/talk` 分析 | 禁产物 |
| `ask.clarify` | `/ask` 提问 | 产出具体问题 + 候选 |

## 关键模板契约（节选）

### `slide.gen`

- system 注入：html-output-spec 摘要（16:9、零运行时依赖、引用 `var(--token)`、可访问性、中英文）。
- 要求输出**完整单页 html**，样式优先用公共层 token；页内私有样式可入 `slide.css`。
- 含 `chart_intent` 时按 [charts](../60-design-system/charts.md) 选定图表方案。

### `style.edit.overview`

- 强约束：**只允许**输出对 `common/tokens.css` 的修改（diff 或完整新内容），**禁止**输出任何单页 html。

### `ask.clarify`

- 输出结构：`{ questions: [{ q, choices? }] }`，由 Run 引擎转为 `needs_input` 事件。

## 通用规则

| ID | 约束 |
|---|---|
| `AGENT-PROMPT-001` | system 层 MUST 包含「不可被用户指令覆盖」的安全声明 |
| `AGENT-PROMPT-002` | 需结构化输出的模板 MUST 声明目标 schema，并在解析失败时重试/纠偏 |
| `AGENT-PROMPT-003` | 模板 MUST 参数化（用占位符），禁止拼接未转义的用户输入到 system 层 |
| `AGENT-PROMPT-004` | 模板版本化，变更可追溯（便于评估生成质量回归） |

## 验收标准（Given-When-Then）

- **AC-PROMPT-002**（`AGENT-PROMPT-002`）
  - GIVEN `outline.gen` 返回非法 JSON
  - WHEN 解析
  - THEN 触发一次纠偏重试；仍失败则发 `error`，不落库脏数据

## 校验方式

```bash
go test ./internal/agent/prompt -run 'TestTemplateAssembly|TestStructuredOutputParse'
```

## 依赖

- [AGENT-OVERVIEW](agent-overview.md)、[ARCH-LLM](../20-architecture/llm-integration.md)、[DS-HTML-OUTPUT](../60-design-system/html-output-spec.md)
