package prompt

import "strings"

// PromptRewriteSystem 是 /prompt 的 system 层：只改写用户输入、保持原意、不执行。
func PromptRewriteSystem() string {
	var b strings.Builder
	b.WriteString(systemBase)
	b.WriteString("\n\n## 任务：改写/优化用户的输入指令\n")
	b.WriteString("把用户的原始想法改写成更清晰、更适合驱动后续生成/编辑的一条指令。\n")
	b.WriteString("- MUST 保持用户原意，只优化表达/结构/明确度（SPEC-CMD-PROMPT-002）。\n")
	b.WriteString("- 只输出改写后的指令文本本身，便于用户直接复制为下一条输入（SPEC-CMD-PROMPT-003）。\n")
	b.WriteString("- MUST NOT 执行该指令，也不要附加解释、寒暄或额外操作（SPEC-CMD-PROMPT-004）。\n")
	return b.String()
}

// TalkSystem 是 /talk 的 system 层：只分析不动手。
func TalkSystem() string {
	var b strings.Builder
	b.WriteString(systemBase)
	b.WriteString("\n\n## 模式：只说不做（talk）\n")
	b.WriteString("只输出分析、思考、方案与取舍，帮助与用户对齐颗粒度。\n")
	b.WriteString("- MUST NOT 产生任何文件/产物变更（SPEC-CMD-TALK-001）。\n")
	b.WriteString("- 即使用户描述了可执行操作，也只给出「将如何做」的分析，不实际执行（SPEC-CMD-TALK-003）。\n")
	b.WriteString("- 用自然语言直接回答，不需要调用任何工具。\n")
	return b.String()
}

// AskDecisionSystem 是 /ask 澄清判定阶段的 system 层：判断是否需要提问。
func AskDecisionSystem() string {
	var b strings.Builder
	b.WriteString(systemBase)
	b.WriteString("\n\n## 模式：必要时提问（ask）——澄清判定\n")
	b.WriteString("先判断本次任务是否存在**影响产出的关键不确定点**：\n")
	b.WriteString("- 若存在（如缺少数据、目标歧义、多种合理理解），调用 `ask_user` 提出**具体、可回答**的问题，尽量给候选项（SPEC-CMD-ASK-001/003）。\n")
	b.WriteString("- 若没有任何不确定点（指令明确无歧义），调用 `proceed` 直接执行（SPEC-CMD-ASK-002）。\n")
	b.WriteString("- 只聚焦关键分歧，避免过度打扰（SPEC-CMD-ASK-005）。\n")
	return b.String()
}
