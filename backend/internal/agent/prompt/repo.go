package prompt

import (
	"fmt"
	"strings"
)

// RepoVersion 标记 repo prompt 模板版本（AGENT-PROMPT-004）。
const RepoVersion = "repo.nl@v1"

// RepoParams 是 /repo prompt 的参数。用户指令只进 user 层（AGENT-PROMPT-003 防注入）。
type RepoParams struct {
	Instruction string // 用户自然语言资产操作指令
}

// RepoSystem 组装 /repo system 层：base + 资产操作契约 + 「只碰资产不碰页」约束。
func RepoSystem(p RepoParams) string {
	var b strings.Builder
	b.WriteString(systemBase)
	b.WriteString("\n\n## 任务：只修改个人仓库中的资产（layout/component/theme/fx）\n")
	b.WriteString("本次操作严格限定在**仓库资产本身**，MUST NOT 触碰任何 PPT 项目的页或公共样式层。\n\n")

	b.WriteString("## 工具\n")
	b.WriteString("- `search_assets`：按 kind + 关键词找到要操作的资产，拿到 asset_id。\n")
	b.WriteString("- `read_asset`：读该资产的 manifest 与载荷，确认锚点上下文。\n")
	b.WriteString("- `create_asset`：新增资产，提交 manifest 与 payload；系统强制 source=user，校验后落库并产版本。\n")
	b.WriteString("- `patch_asset`：锚定替换某载荷文件（html/css/js/tokens）。old_text 必须在该文件内唯一。\n")
	b.WriteString("- `delete_asset`：删除 user 资产；preset 出厂资产禁止删除。\n")
	b.WriteString("- `validate_asset`：校验资产协议合规（theme 另校 token 全集）。\n")
	b.WriteString("完成后调用 `finish`。\n\n")

	b.WriteString("## 硬约束\n")
	b.WriteString("- 只做用户要求的最小改动；不顺手重写整份载荷。\n")
	b.WriteString("- 修改 theme 的 tokens 时 MUST 保持必需 token 全集，不残缺。\n")
	b.WriteString("- 你没有修改 PPT 页或公共层的工具——这是设计使然，不要尝试。\n")
	return b.String()
}

// RepoUser 组装 user 层：承载用户资产操作指令。
func RepoUser(p RepoParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "资产操作指令：%s\n", strings.TrimSpace(p.Instruction))
	return b.String()
}
