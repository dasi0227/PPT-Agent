package prompt

import (
	"fmt"
	"strings"
)

// OutlineVersion 标记模板版本，便于生成质量回归追溯（AGENT-PROMPT-004）。
const OutlineVersion = "outline.gen@v1"

// OutlineParams 是 outline.gen 的参数（占位符），用户文本只进 user 层（AGENT-PROMPT-003）。
type OutlineParams struct {
	Topic      string
	Brief      string
	SlideCount int      // 0 表示未指定，由模型在 5–20 中自定
	Language   string   // zh | en，空默认 zh
	Layouts    []string // 合法 layout 枚举（来自 schema，权威同源）
	MinBody    int      // 中间页数下限（未指定页数时）
	MaxBody    int      // 中间页数上限
}

// OutlineSystem 组装 outline.gen 的 system 层：base + 产出契约 + 约束。
// 不拼接用户输入到 system 层（防注入）。
func OutlineSystem(p OutlineParams) string {
	var b strings.Builder
	b.WriteString(systemBase)
	b.WriteString("\n\n## 任务：生成演示文稿大纲（主题 → slide-json[]）\n")
	b.WriteString("你要产出一份结构化大纲：每页用 slide-json 描述其**内容与意图**，")
	b.WriteString("**不要**生成任何 HTML/CSS（那是后续阶段的工作）。\n\n")

	b.WriteString("## 提交方式\n")
	b.WriteString("通过调用 `submit_outline` 工具提交整份大纲（参数 slides 为 slide-json 数组）。\n")
	b.WriteString("提交成功后调用 `finish` 结束。若工具返回校验错误，请据错误修正后重新提交。\n\n")

	b.WriteString("## 每页 slide-json 字段\n")
	b.WriteString("- `layout`（必填）：从以下枚举中取值：\n  ")
	b.WriteString(strings.Join(p.Layouts, ", "))
	b.WriteString("\n- `title`（必填）：页标题\n")
	b.WriteString("- `bullets`（要点数组）或 `content_intent`（内容意图描述）：至少提供其一\n")
	b.WriteString("- 可选：`subtitle`、`chart_intent`（数据型页）、`notes`\n")
	b.WriteString("- 不要提供 `id` 与 `idx`，由系统按顺序回填\n\n")

	b.WriteString("## 结构约束\n")
	b.WriteString("- 首页 `layout` 必须为 `cover`\n")
	b.WriteString("- 末页 `layout` 必须为 `thanks` 或 `cta`\n")
	if p.SlideCount > 0 {
		fmt.Fprintf(&b, "- 总页数必须恰为 %d 页（含首尾）\n", p.SlideCount)
	} else {
		fmt.Fprintf(&b, "- 中间正文页（不含首尾）数量应在 %d–%d 页之间\n", p.MinBody, p.MaxBody)
	}

	lang := p.Language
	if lang == "" {
		lang = "zh"
	}
	fmt.Fprintf(&b, "- 输出语言：%s\n", lang)
	return b.String()
}

// OutlineUser 组装 user 层：仅承载用户主题/简报（意图层）。
func OutlineUser(p OutlineParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "主题：%s\n", p.Topic)
	if strings.TrimSpace(p.Brief) != "" {
		fmt.Fprintf(&b, "补充说明：%s\n", p.Brief)
	}
	if p.SlideCount > 0 {
		fmt.Fprintf(&b, "期望页数：%d\n", p.SlideCount)
	}
	return b.String()
}

// OutlineEditVersion 标记大纲编辑模板版本。
const OutlineEditVersion = "outline.edit@v1"

// OutlineEditParams 是大纲编辑（已有大纲 → 结构/内容调整）的参数。
type OutlineEditParams struct {
	Instruction string   // 用户自然语言指令（意图层）
	Language    string   // zh | en
	Layouts     []string // 合法 layout 枚举
	Slides      string   // 当前大纲摘要（id | idx | layout | title 多行）
}

// OutlineEditSystem 组装大纲编辑 system 层：说明可用工具与安全约束（不拼用户输入）。
func OutlineEditSystem(p OutlineEditParams) string {
	var b strings.Builder
	b.WriteString(systemBase)
	b.WriteString("\n\n## 任务：编辑现有大纲（结构与内容调整）\n")
	b.WriteString("演示文稿已有一份大纲（slide-json）。请根据用户指令调整**内容与结构**，")
	b.WriteString("**不要**生成任何 HTML/CSS。\n\n")

	b.WriteString("## 可用工具\n")
	b.WriteString("- `patch_outline_slide(slide_id, ...)`：改某页 title/subtitle/bullets/content_intent/layout\n")
	b.WriteString("- `add_outline_slide(after_slide_id?, layout?)`：在某页后插入新页\n")
	b.WriteString("- `delete_outline_slide(slide_id)`：删除某页（不可逆，系统会请用户二次确认）\n")
	b.WriteString("- `reorder_outline_slides(ordered_ids)`：按完整 id 顺序重排全部页\n")
	b.WriteString("- 完成后调用 `finish`\n\n")

	b.WriteString("## 约束\n")
	b.WriteString("- 只能引用下方大纲中真实存在的 slide_id\n")
	b.WriteString("- 删除操作不可逆，请仅在用户明确要求删页时调用 delete_outline_slide\n")
	b.WriteString("- reorder 时 ordered_ids 必须覆盖全部现有页\n")
	b.WriteString("- 可用 layout 枚举：\n  ")
	b.WriteString(strings.Join(p.Layouts, ", "))
	b.WriteString("\n")

	lang := p.Language
	if lang == "" {
		lang = "zh"
	}
	fmt.Fprintf(&b, "- 输出语言：%s\n", lang)
	return b.String()
}

// OutlineEditUser 组装 user 层：当前大纲摘要 + 用户指令。
func OutlineEditUser(p OutlineEditParams) string {
	var b strings.Builder
	b.WriteString("## 当前大纲（slide_id | idx | layout | title）\n")
	b.WriteString(p.Slides)
	b.WriteString("\n## 编辑指令\n")
	b.WriteString(p.Instruction)
	b.WriteString("\n")
	return b.String()
}
