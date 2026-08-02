package designsystem

import (
	"regexp"
)

// layoutRowRe 匹配 layouts.md 版式清单表格首列的 `layout` 值（形如 `| `cover` | ...`）。
// 该表是 layout 目录的权威登记（DS-LAYOUTS）。
var layoutRowRe = regexp.MustCompile("(?m)^\\|\\s*`([a-z0-9][a-z0-9-]*)`\\s*\\|")

// ParseLayoutCatalog 从 layouts.md 文本解析版式目录（清单表格首列）。
// 供设计系统一致性校验：目录表定义可供 canonical Spec 使用的 layout archetype。
func ParseLayoutCatalog(md []byte) []string {
	var out []string
	for _, m := range layoutRowRe.FindAllSubmatch(md, -1) {
		out = append(out, string(m[1]))
	}
	return out
}
