package prompt

import (
	"fmt"
	"strings"
)

// EditVersion 标记 edit prompt 模板版本（AGENT-PROMPT-004）。
const EditVersion = "edit.nl@v1"

// EditParams 是 edit prompt 的参数。用户编辑指令只进 user 层（AGENT-PROMPT-003 防注入）。
type EditParams struct {
	PageIndex   int    // 锁定编辑的页序（0 基）
	Instruction string // 用户自然语言编辑指令
	SlideHTML   string // 目标页当前 html（page scope 只注入目标页，AGENT-CTX-001）
	SlideJSON   string // 目标页 slide-json（内容与意图，可空）
}

// EditSystem 组装编辑 system 层：base + 编辑契约 + 锚定 patch 约束 + html-output-spec 保持。
// 不拼接用户可控内容到 system 层（防注入）。
func EditSystem(p EditParams) string {
	var b strings.Builder
	b.WriteString(systemBase)
	b.WriteString("\n\n## 任务：按自然语言指令精确编辑单页 slide html\n")
	fmt.Fprintf(&b, "本次编辑严格锁定在第 %d 页，MUST NOT 触碰其它页或公共样式层。\n\n", p.PageIndex)

	b.WriteString("## 编辑方式（锚定替换）\n")
	b.WriteString("通过 `patch_slide` 工具做**锚定文本替换**：提供 old_text（该页内**唯一**出现的原文片段）与 new_text。\n")
	b.WriteString("- 每个 old_text MUST 在该页内唯一，否则整体失败——请补足周边上下文使其唯一后重试。\n")
	b.WriteString("- 需要确认最新内容时先调用 `read_slide`；落盘前系统会隐式校验 html-output-spec。\n")
	fmt.Fprintf(&b, "- 调用 patch_slide 时 slide_idx MUST 为 %d。\n", p.PageIndex)
	b.WriteString("完成后调用 `finish` 结束。若工具返回错误，请据错误修正后重试。\n\n")

	b.WriteString("## 硬约束（保持产出合规）\n")
	b.WriteString("- 编辑后该页 MUST 仍满足 html-output-spec：固定 16:9 舞台、引用公共层、无硬编码主题色/字号（用 `var(--token)`）、图片含 alt、动效用约定属性。\n")
	b.WriteString("- 只做用户要求的最小改动，不顺手重写整页。\n")
	b.WriteString("- 不新增白名单外的重运行时依赖。\n")
	return b.String()
}

// EditUser 组装 user 层：承载用户编辑指令 + 目标页当前内容（意图与上下文）。
func EditUser(p EditParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "编辑指令：%s\n\n", strings.TrimSpace(p.Instruction))
	fmt.Fprintf(&b, "目标页：第 %d 页（slide_idx=%d）\n\n", p.PageIndex, p.PageIndex)
	if strings.TrimSpace(p.SlideJSON) != "" {
		b.WriteString("当前页 slide-json（内容与意图）：\n")
		b.WriteString("```json\n")
		b.WriteString(strings.TrimSpace(p.SlideJSON))
		b.WriteString("\n```\n\n")
	}
	b.WriteString("当前页 html：\n")
	b.WriteString("```html\n")
	b.WriteString(strings.TrimSpace(p.SlideHTML))
	b.WriteString("\n```\n")
	return b.String()
}
