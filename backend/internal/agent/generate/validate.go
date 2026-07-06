// validate.go 是 v2 生成流水线的 Stage 4（全局校验节点，V2-AGENT-PIPELINE §8.1）。
// 确定性（无 LLM）：在 v1 逐页 lint（designsystem.LintSlide）之上做跨页一致性检查，
// 不重造 lint 规则。不合格页由 runner 触发有限次修复子循环（slide.fix@v1）。
package generate

import (
	"regexp"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
)

// maxFixRounds 是单页连续修复的上限（V2-STOP-002，默认 2）。
// 超限该页标 failed 并计入 done.result.warnings，MUST NOT 阻塞整体 done。
const maxFixRounds = 2

// 交付告警的错误码（进入 done.result.warnings）。
const (
	warnFixExceeded = "FIX_EXCEEDED"
)

// Warning 是结构化交付告警的一项（Stage 5 done.result.warnings）。
type Warning struct {
	PageIndex int    `json:"page_index"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// reTokensLink / reBaseLink 匹配公共层 <link>，用于校验引用顺序（tokens 在前，base 在后）。
var (
	reTokensLink = regexp.MustCompile(`tokens\.css`)
	reBaseLink   = regexp.MustCompile(`base\.css`)
	// reFontFamily 提取 font-family 声明的取值（判断字体族是否落在 design_spec 集合内）。
	// 取值到 ; 或 } 结束（含引号，字体名常带引号）。
	reFontFamily = regexp.MustCompile(`(?i)font-family\s*:\s*([^;}]+)`)
)

// genericFontKeywords 是无需在 design_spec 声明的通用/回退字体族（不触发一致性告警）。
var genericFontKeywords = map[string]bool{
	"sans-serif": true, "serif": true, "monospace": true, "cursive": true, "fantasy": true,
	"system-ui": true, "ui-sans-serif": true, "ui-serif": true, "ui-monospace": true,
	"inherit": true, "initial": true, "unset": true,
	"pingfang sc": true, "microsoft yahei": true, "-apple-system": true, "blinkmacsystemfont": true,
}

// crossPageLint 校验单页：复用 v1 LintSlide（逐页规则）+ 跨页一致性增量。
// 返回不合规项的可读描述列表（空=合规）；供 runner 回灌修复子循环与汇总 warnings。
func crossPageLint(html []byte, spec *DesignSpec) []string {
	var issues []string

	// 1) 复用 v1 逐页 lint：16:9 舞台、公共层引用、无硬编码主题色、img alt、动效约定等。
	for _, c := range designsystem.LintSlide(html) {
		if !c.OK {
			issues = append(issues, c.ID+": "+c.Reason)
		}
	}

	s := string(html)

	// 2) 公共层引用顺序：tokens 在前、base 在后（LintSlide 只校验二者存在，此处补顺序）。
	if !tokensBeforeBase(s) {
		issues = append(issues, "common-layer-order: 公共层 <link> 顺序应为 tokens.css 在前、base.css 在后")
	}

	// 3) 设计语言一致：抽样校验字体族是否落在 design_spec 声明集合内（主色一致性由 no-hardcoded-theme 保证）。
	if spec != nil {
		issues = append(issues, fontConsistencyIssues(s, spec)...)
	}

	return issues
}

// tokensBeforeBase 当 tokens.css 与 base.css 均出现且 tokens 先于 base 时返回 true。
// 仅当二者都存在才判定顺序（缺失由 LintSlide 的 common-layer 项负责）。
func tokensBeforeBase(s string) bool {
	tIdx := reTokensLink.FindStringIndex(s)
	bIdx := reBaseLink.FindStringIndex(s)
	if tIdx == nil || bIdx == nil {
		return true
	}
	return tIdx[0] < bIdx[0]
}

// fontConsistencyIssues 检查 html 中的字面 font-family 是否落在 design_spec 声明字体集合内。
// 走 var(--token) 的声明视为合规（token 已由 design_spec 生成）。
func fontConsistencyIssues(s string, spec *DesignSpec) []string {
	allow := specFontSet(spec)
	var issues []string
	for _, m := range reFontFamily.FindAllStringSubmatch(s, -1) {
		val := strings.TrimSpace(m[1])
		if strings.Contains(val, "var(") {
			continue // token 驱动，合规
		}
		for _, fam := range strings.Split(val, ",") {
			name := normalizeFamily(fam)
			if name == "" || genericFontKeywords[name] || allow[name] {
				continue
			}
			issues = append(issues, "design-font-consistency: 字体族 \""+strings.TrimSpace(fam)+"\" 不在设计语言声明集合内，应使用 var(--font-display)/var(--font-sans)")
			break
		}
	}
	return issues
}

// specFontSet 收集 design_spec 声明的字体族（display/body/utility），用于一致性校验。
func specFontSet(spec *DesignSpec) map[string]bool {
	set := map[string]bool{}
	for _, f := range []string{spec.Type.Display.Family, spec.Type.Body.Family, spec.Type.Utility.Family} {
		if n := normalizeFamily(f); n != "" {
			set[n] = true
		}
	}
	return set
}

// normalizeFamily 去引号、去空白并小写，便于字体族名比较。
func normalizeFamily(f string) string {
	f = strings.TrimSpace(f)
	f = strings.Trim(f, `"'`)
	return strings.ToLower(strings.TrimSpace(f))
}
