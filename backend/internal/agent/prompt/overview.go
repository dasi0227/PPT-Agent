package prompt

import (
	"fmt"
	"strings"
)

// OverviewVersion 标记 overview prompt 模板版本（AGENT-PROMPT-004）。
const OverviewVersion = "overview.nl@v1"

// OverviewParams 是 overview prompt 的参数。用户指令只进 user 层（AGENT-PROMPT-003 防注入）。
type OverviewParams struct {
	Instruction string // 用户自然语言全局调整指令
	TokensCSS   string // 当前公共层 tokens.css（供 LLM 判断能否用 token 表达）
	PageCount   int    // 项目页数
}

// OverviewSystem 组装 overview system 层：base + 改动面最小化契约 + 两条落点（公共层优先 / 跨页子代理）。
func OverviewSystem(p OverviewParams) string {
	var b strings.Builder
	b.WriteString(systemBase)
	b.WriteString("\n\n## 任务：站在整个演示文稿层面做全局调整\n")
	b.WriteString("核心原则：**改动面最小化**。能用公共层设计 token 表达的调整，MUST 优先改公共层，不逐页改。\n\n")

	b.WriteString("## 工具与落点\n")
	b.WriteString("- `read_design`：读公共层 tokens.css，确认可改的 token。\n")
	b.WriteString("- `search_assets`：检索仓库资产；换主题或移植资产前先定位 asset_id。\n")
	b.WriteString("- `apply_theme`：把 theme 资产 token 全集写入公共层，用于整体换肤。\n")
	b.WriteString("- `patch_design`（**首选**）：锚定替换 tokens.css 里的 token（如 `--color-primary`）。一次影响所有页，只改 1 个文件。\n")
	b.WriteString("- `mount_asset`：把 layout/component/fx 资产挂载到指定 slide_idx，成功后该页产版本并保持 html-output-spec 合规。\n")
	b.WriteString("- `fanout_page_patch`（**仅当** token 无法表达时）：如「每页右下角加页脚 logo」这类逐页结构性添加，逐页派发子代理执行；每页各自版本、某页失败不影响其它页。\n")
	fmt.Fprintf(&b, "- 完成后调用 `finish` 结束。当前项目共 %d 页。\n\n", p.PageCount)

	b.WriteString("## 决策准则\n")
	b.WriteString("- 「主色/字号/间距/圆角等 token 化的全局样式」→ patch_design。\n")
	b.WriteString("- 「换某个主题」→ search_assets(kind=theme) 后 apply_theme。\n")
	b.WriteString("- 「给某一页加组件/动效」→ search_assets(kind=component/fx) 后 mount_asset。\n")
	b.WriteString("- 「每页新增/移除结构元素、逐页内容变化」→ fanout_page_patch。\n")
	b.WriteString("- 不确定能否 token 化时，先 read_design 看有没有对应 token；有则 patch_design。\n")
	return b.String()
}

// OverviewUser 组装 user 层：用户指令 + 当前公共层内容（供判断落点）。
func OverviewUser(p OverviewParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "全局调整指令：%s\n\n", strings.TrimSpace(p.Instruction))
	b.WriteString("当前公共层 tokens.css：\n")
	b.WriteString("```css\n")
	b.WriteString(strings.TrimSpace(p.TokensCSS))
	b.WriteString("\n```\n")
	return b.String()
}
