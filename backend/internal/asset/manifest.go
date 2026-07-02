// Package asset 实现个人仓库的统一资产协议（ADR-0009 / ASSET-PROTOCOL）：
// 统一信封 + 4 类（layout/component/theme/fx）类型化载荷。
// 负责 manifest 结构、schema 校验（AC-ASSET-001）、theme token 全集校验（AC-ASSET-002）与载入。
package asset

// Kind 是资产类型（4 类）。
type Kind string

const (
	KindLayout    Kind = "layout"
	KindComponent Kind = "component"
	KindTheme     Kind = "theme"
	KindFx        Kind = "fx"
)

// Source 区分出厂预置与用户新增（ASSET-007：同协议同表，仅 source 不同）。
type Source string

const (
	SourcePreset Source = "preset"
	SourceUser   Source = "user"
)

// Mount 是挂载约定（layout/component/fx 用；theme 不需要）。
type Mount struct {
	Target   string `json:"target,omitempty"`
	Position string `json:"position"` // append|prepend|replace|wrap
}

// Param 是一个可参数化字段（component/fx 用），MUST 含 type 与 default（ASSET-006）。
type Param struct {
	Type        string `json:"type"` // string|number|boolean|color|enum
	Default     any    `json:"default"`
	Enum        []any  `json:"enum,omitempty"`
	Description string `json:"description,omitempty"`
}

// Payload 是载荷文件清单（相对资产目录）。按 kind 取用不同字段。
type Payload struct {
	HTML   string `json:"html,omitempty"`   // layout/component
	CSS    string `json:"css,omitempty"`    // layout/component
	JS     string `json:"js,omitempty"`     // fx
	Tokens string `json:"tokens,omitempty"` // theme
}

// Requires 声明依赖（可选）。
type Requires struct {
	Tokens []string `json:"tokens,omitempty"`
	CDN    []string `json:"cdn,omitempty"`
}

// Manifest 是统一信封 + 类型化载荷，对应 asset-manifest.schema.json。
type Manifest struct {
	Name        string           `json:"name"`
	Version     string           `json:"version"`
	Kind        Kind             `json:"kind"`
	Source      Source           `json:"source,omitempty"`
	Description string           `json:"description"`
	Tags        []string         `json:"tags,omitempty"`
	Preview     string           `json:"preview,omitempty"`
	Mount       *Mount           `json:"mount,omitempty"`
	Params      map[string]Param `json:"params,omitempty"`
	Assets      Payload          `json:"assets"`
	Requires    *Requires        `json:"requires,omitempty"`
	Cleanup     bool             `json:"cleanup,omitempty"`
}
