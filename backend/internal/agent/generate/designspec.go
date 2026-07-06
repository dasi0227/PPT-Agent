// designspec.go 定义 v2 设计总监节点产出的 design_spec 结构、字段校验，
// 以及由 design_spec 生成公共层 tokens.css 的确定性逻辑（V2-AGENT-PIPELINE §3）。
package generate

import (
	"fmt"
	"strings"
)

// DesignSpec 是设计总监的结构化产物（落盘 design/design-spec.json）。
type DesignSpec struct {
	Subject   DesignSubject   `json:"subject"`
	Palette   []DesignColor   `json:"palette"`
	Type      DesignType      `json:"type"`
	Layout    DesignLayout    `json:"layout"`
	Signature string          `json:"signature"`
	Motion    DesignMotion    `json:"motion,omitempty"`
}

type DesignSubject struct {
	Topic    string `json:"topic"`
	Audience string `json:"audience"`
	Job      string `json:"job"`
}

type DesignColor struct {
	Name string `json:"name"`
	Hex  string `json:"hex"`
	Role string `json:"role"`
}

type DesignType struct {
	Display DesignFont `json:"display"`
	Body    DesignFont `json:"body"`
	Utility DesignFont `json:"utility,omitempty"`
}

type DesignFont struct {
	Family  string `json:"family"`
	Weights []int  `json:"weights,omitempty"`
	Usage   string `json:"usage,omitempty"`
}

type DesignLayout struct {
	Grid    string `json:"grid,omitempty"`
	Rhythm  string `json:"rhythm,omitempty"`
	Concept string `json:"concept,omitempty"`
}

type DesignMotion struct {
	Policy string `json:"policy,omitempty"`
}

// Validate 校验 design_spec 字段完整性（V2-AGENT-PIPELINE §3.3，AC-V2-PIPE-002）。
// 返回可操作错误信息（供工具回灌给 LLM 修正）；nil 表示合格。
func (s DesignSpec) Validate() error {
	// palette 至少 3 个命名色，每个含 name/hex/role。
	if len(s.Palette) < 3 {
		return fmt.Errorf("palette 至少需要 3 个命名色（含背景与主强调），当前 %d 个", len(s.Palette))
	}
	for i, c := range s.Palette {
		if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.Role) == "" {
			return fmt.Errorf("palette[%d] 缺少 name 或 role", i)
		}
		if !isHexColor(c.Hex) {
			return fmt.Errorf("palette[%d] (%s) 的 hex 非法：%q（需形如 #RGB 或 #RRGGBB）", i, c.Name, c.Hex)
		}
	}
	// type 至少含 display 与 body，各含 family/weights/usage。
	if err := validateFont("display", s.Type.Display); err != nil {
		return err
	}
	if err := validateFont("body", s.Type.Body); err != nil {
		return err
	}
	// signature 必须非空、可实现为 HTML/CSS/SVG 的具体描述。
	if len(strings.TrimSpace(s.Signature)) < 4 {
		return fmt.Errorf("signature 不能为空，且需是一句具体、可实现为 HTML/CSS/SVG 的描述")
	}
	return nil
}

func validateFont(role string, f DesignFont) error {
	if strings.TrimSpace(f.Family) == "" {
		return fmt.Errorf("type.%s.family 不能为空", role)
	}
	if len(f.Weights) == 0 {
		return fmt.Errorf("type.%s.weights 至少给一个字重", role)
	}
	if strings.TrimSpace(f.Usage) == "" {
		return fmt.Errorf("type.%s.usage 需说明使用场景", role)
	}
	return nil
}

var hexRe = struct{ short, long func(string) bool }{
	short: func(s string) bool { return matchHex(s, 3) },
	long:  func(s string) bool { return matchHex(s, 6) },
}

func isHexColor(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 4 {
		return hexRe.short(s)
	}
	if len(s) == 7 {
		return hexRe.long(s)
	}
	return false
}

func matchHex(s string, n int) bool {
	if s[0] != '#' || len(s) != n+1 {
		return false
	}
	for _, c := range s[1:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// pickColor 从 palette 里按角色关键词选色；命中返回 hex，未命中返回 fallback。
func pickColor(palette []DesignColor, fallback string, keywords ...string) string {
	for _, c := range palette {
		for _, kw := range keywords {
			if strings.Contains(c.Role, kw) || strings.Contains(c.Name, kw) {
				return c.Hex
			}
		}
	}
	return fallback
}

// fontFamilyCSS 组装带中文回退的 font-family 值。
func fontFamilyCSS(families ...string) string {
	seen := map[string]bool{}
	var parts []string
	for _, f := range families {
		f = strings.TrimSpace(f)
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		parts = append(parts, quoteFamily(f))
	}
	// 中文与系统回退。
	parts = append(parts, `"PingFang SC"`, `"Microsoft YaHei"`, "system-ui", "sans-serif")
	return strings.Join(parts, ", ")
}

func quoteFamily(f string) string {
	if strings.ContainsAny(f, " ") {
		return `"` + f + `"`
	}
	return f
}

// tokensFromSpec 由 design_spec 生成项目专属 tokens.css（自由创作场景）。
// 保证覆盖 designsystem.RequiredTokens() 全集，随后可通过 LintTokens 校验。
func tokensFromSpec(spec DesignSpec) []byte {
	bg := pickColor(spec.Palette, spec.Palette[len(spec.Palette)-1].Hex, "背景", "底", "浅底", "paper")
	fg := pickColor(spec.Palette, "#12161C", "正文", "文字", "前景", "ink")
	primary := pickColor(spec.Palette, spec.Palette[0].Hex, "主强调", "强调", "primary", "signal")
	accent := pickColor(spec.Palette, primary, "次强调", "告警", "accent", "amber")
	muted := pickColor(spec.Palette, "#8A8F98", "辅助", "次要", "muted", "grey", "gray")

	displayFamily := fontFamilyCSS(spec.Type.Display.Family, spec.Type.Body.Family)
	bodyFamily := fontFamilyCSS(spec.Type.Body.Family)

	var b strings.Builder
	b.WriteString("/* generated from design-spec.json (design.director@v1) */\n")
	b.WriteString(":root {\n")
	fmt.Fprintf(&b, "  --color-bg: %s;\n", bg)
	fmt.Fprintf(&b, "  --color-fg: %s;\n", fg)
	fmt.Fprintf(&b, "  --color-primary: %s;\n", primary)
	fmt.Fprintf(&b, "  --color-accent: %s;\n", accent)
	fmt.Fprintf(&b, "  --color-muted: %s;\n", muted)
	fmt.Fprintf(&b, "  --font-sans: %s;\n", bodyFamily)
	fmt.Fprintf(&b, "  --font-display: %s;\n", displayFamily)
	b.WriteString("  --text-title: clamp(2rem, 4vw, 3.25rem);\n")
	b.WriteString("  --text-body: clamp(1rem, 1.4vw, 1.25rem);\n")
	b.WriteString("  --space-4: 1rem;\n")
	b.WriteString("  --radius-md: 0.75rem;\n")
	b.WriteString("  --shadow-card: 0 8px 24px rgba(0,0,0,0.12);\n")
	b.WriteString("  --stage-w: 1280px;\n")
	b.WriteString("  --stage-h: 720px;\n")
	b.WriteString("}\n")
	return []byte(b.String())
}

// tokensWithSpecOverlay 在已选主题 tokens 基底上叠加 design_spec 的字体角色（不覆盖主色，V2 §6）。
// 仅当基底缺 --font-display 时补充，保证 signature/排版可用又不喧宾夺主。
func tokensWithSpecOverlay(base []byte, spec DesignSpec) []byte {
	css := string(base)
	if strings.Contains(css, "--font-display") {
		return base
	}
	overlay := fmt.Sprintf("\n/* design.director@v1 overlay: display font only, base palette preserved */\n:root { --font-display: %s; }\n",
		fontFamilyCSS(spec.Type.Display.Family, spec.Type.Body.Family))
	return []byte(css + overlay)
}
