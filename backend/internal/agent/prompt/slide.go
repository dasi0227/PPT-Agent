package prompt

import (
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
)

// SlideVersion 标记 slide.gen 模板版本，便于生成质量回归追溯（AGENT-PROMPT-004）。
const SlideVersion = "slide.gen@v1"

// SlideParams 是 slide.gen 的参数。用户内容只进 user 层（AGENT-PROMPT-003 防注入）。
type SlideParams struct {
	Slide     slidejson.SlideJSON // 本页 slide-json（内容与意图）
	Theme     string              // 所选主题 id（仅作提示；换肤靠公共层 tokens.css）
	TokensRel string              // 公共层 tokens.css 相对本页 index.html 的路径
	BaseRel   string              // 公共层 base.css 相对路径
	Layouts   []string            // 合法 layout 枚举（权威同源）
}

// SlideSystem 组装 slide.gen 的 system 层：base + 产出契约 + html-output-spec 硬约束。
// 不拼接用户可控内容到 system 层（防注入）。
func SlideSystem(p SlideParams) string {
	var b strings.Builder
	b.WriteString(systemBase)
	b.WriteString("\n\n## 任务：把一页 slide-json 渲染为合规 slide html\n")
	b.WriteString("你要为**单独一页**生成完整、可独立预览的 HTML，套用设计系统（主题 token / 版式 / 图表 / 动效）。\n\n")

	b.WriteString("## 提交方式\n")
	b.WriteString("通过调用 `write_slide` 工具整页写入（参数 slide_idx + html）。系统会在落盘前隐式校验 html-output-spec，")
	b.WriteString("不合规会返回可操作错误；请据错误修正后重新提交。成功后调用 `finish` 结束。\n\n")

	b.WriteString("## 产出硬约束（html-output-spec，必须全部满足）\n")
	fmt.Fprintf(&b, "1. 固定 16:9 舞台：根结构使用 `<div class=\"slide-scaler\"><section class=\"slide-stage\">…</section></div>`（来自 base.css）。\n")
	fmt.Fprintf(&b, "2. 引用公共样式层：`<link rel=\"stylesheet\" href=\"%s\">` 与 `<link rel=\"stylesheet\" href=\"%s\">`（顺序：tokens 在前，base 在后）。\n", p.TokensRel, p.BaseRel)
	b.WriteString("3. 禁止硬编码主题色（`#rrggbb`、`rgb()/rgba()/hsl()`）与硬编码字号；一切主题相关视觉值用 `var(--token)`。\n")
	b.WriteString("4. 每个 `<img>` MUST 含 `alt`。\n")
	b.WriteString("5. 动效只用约定属性挂载（`data-animate=\"<name>\"` / `data-fx=\"<name>\"`），不要内联 `@keyframes` 或散落脚本。\n")
	b.WriteString("6. 零运行时强依赖：单页可独立渲染；仅允许 CDN webfont/highlight.js/chart.js 等可选增强（白名单）。\n")
	b.WriteString("7. 完整文档：包含 `<html>`、`<head>`、`<body>`，可被 `?preview=N` 独立渲染。\n\n")

	b.WriteString("## 版式\n")
	b.WriteString("- 合法 layout 枚举：\n  ")
	b.WriteString(strings.Join(p.Layouts, ", "))
	b.WriteString("\n- 若本页 layout 不在上表或不适用，MUST 回退到 `bullets` 的安全结构（SPEC-GEN-003）。\n")
	b.WriteString("- 可用 token（示例）：`--color-bg`、`--color-fg`、`--color-primary`、`--color-accent`、`--color-muted`、`--font-sans`、`--text-title`、`--text-body`、`--space-4`、`--radius-md`、`--shadow-card`、`--stage-w`、`--stage-h`。\n")
	b.WriteString("- 图表配色 MUST 走 token（如 `fill: var(--color-primary)`），以随主题换肤（DS-CHARTS-001）。\n\n")

	b.WriteString("## 语言与排版\n")
	b.WriteString("- 中英文一等公民：字体走 `var(--font-sans)`（已含中文回退）。\n")
	b.WriteString("- 语义标签、合理标题层级、足够对比度。\n")
	return b.String()
}

// SlideUser 组装 user 层：仅承载本页 slide-json 的内容意图（意图层）。
func SlideUser(p SlideParams) string {
	s := p.Slide
	var b strings.Builder
	fmt.Fprintf(&b, "请为第 %d 页生成 slide html。\n", s.Idx)
	fmt.Fprintf(&b, "- layout：%s\n", s.Layout)
	fmt.Fprintf(&b, "- 标题：%s\n", s.Title)
	if strings.TrimSpace(s.Subtitle) != "" {
		fmt.Fprintf(&b, "- 副标题：%s\n", s.Subtitle)
	}
	if len(s.Bullets) > 0 {
		b.WriteString("- 要点：\n")
		for _, bl := range s.Bullets {
			fmt.Fprintf(&b, "  - %s\n", bl)
		}
	}
	if strings.TrimSpace(s.ContentIntent) != "" {
		fmt.Fprintf(&b, "- 内容意图：%s\n", s.ContentIntent)
	}
	if s.ChartIntent != nil {
		fmt.Fprintf(&b, "- 图表意图：type=%s", s.ChartIntent.Type)
		if strings.TrimSpace(s.ChartIntent.DataHint) != "" {
			fmt.Fprintf(&b, "，数据提示=%s", s.ChartIntent.DataHint)
		}
		b.WriteString("（配色走 token）\n")
	}
	if s.Steps > 1 {
		fmt.Fprintf(&b, "- 分步：%d 步（用 data-step 标记分步元素）\n", s.Steps)
	}
	fmt.Fprintf(&b, "\n调用 write_slide 提交，slide_idx=%d。", s.Idx)
	return b.String()
}
