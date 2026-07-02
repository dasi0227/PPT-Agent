package designsystem

import (
	"regexp"
	"strings"
)

// Check 是 lint-slide 单项检查结果（html-output-spec 校验清单一项）。
type Check struct {
	ID     string // 检查项标识
	OK     bool
	Reason string // 失败原因（OK 时为空）
}

// 编译期正则（一次编译，重复使用）。
var (
	// 十六进制颜色字面量：#rgb / #rgba / #rrggbb / #rrggbbaa（后接非 hex 边界）。
	reHexColor = regexp.MustCompile(`#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{4}|[0-9a-fA-F]{3})\b`)
	// 字面量颜色函数（rgb/rgba/hsl/hsla），均属硬编码主题色。
	reColorFunc = regexp.MustCompile(`(?i)\b(?:rgba?|hsla?)\s*\(`)
	// font-size 声明及其取值（用于判断是否走 var(--token)）。
	reFontSize = regexp.MustCompile(`(?i)font-size\s*:\s*([^;}"']+)`)
	// <img> 标签（用于校验 alt）。
	reImgTag = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	// 外部脚本 src。
	reScriptSrc = regexp.MustCompile(`(?is)<script\b[^>]*\bsrc\s*=\s*["']([^"']+)["'][^>]*>`)
	// 内联 @keyframes（动效关键帧应在公共层/fx 资产，不散落单页）。
	reKeyframes = regexp.MustCompile(`(?i)@keyframes\b`)
)

// scriptSrcAllowHosts 是允许的 CDN 增强白名单（DS-HTML-002：webfont/highlight/chart）。
var scriptSrcAllowHosts = []string{
	"fonts.googleapis.com", "fonts.gstatic.com",
	"cdn.jsdelivr.net", "cdnjs.cloudflare.com", "unpkg.com",
}

// LintSlide 按 html-output-spec 校验清单逐项检查一份 slide html，返回全部检查项结果。
// 任一项 OK=false 即不合规；write_slide 落盘前据此拒绝（ARCH-TOOLS-004）。
func LintSlide(html []byte) []Check {
	s := string(html)
	return []Check{
		checkStage(s),
		checkCommonLayer(s),
		checkNoHardcodedTheme(s),
		checkImageAlt(s),
		checkAnimationConvention(s),
		checkNoHeavyRuntime(s),
		checkPreviewable(s),
	}
}

// LintSlideResult 汇总 LintSlide 为 (ok, 首个失败原因)。供 Validator 适配。
func LintSlideResult(html []byte) (bool, string) {
	for _, c := range LintSlide(html) {
		if !c.OK {
			return false, c.ID + ": " + c.Reason
		}
	}
	return true, ""
}

// 1) 含 16:9 舞台容器与缩放逻辑。
func checkStage(s string) Check {
	if strings.Contains(s, "slide-stage") {
		return Check{ID: "stage-16-9", OK: true}
	}
	return Check{ID: "stage-16-9", OK: false, Reason: "缺少 16:9 舞台容器（class 含 slide-stage，来自 common/base.css）"}
}

// 2) 引用公共层（tokens + base）。
func checkCommonLayer(s string) Check {
	if strings.Contains(s, "tokens.css") && strings.Contains(s, "base.css") {
		return Check{ID: "common-layer", OK: true}
	}
	return Check{ID: "common-layer", OK: false, Reason: "MUST 引用公共层 common/tokens.css 与 common/base.css"}
}

// 3) 无硬编码主题色（#rrggbb / 颜色函数）/硬编码字号（font-size 必须走 var(--token)）。
func checkNoHardcodedTheme(s string) Check {
	if m := reHexColor.FindString(s); m != "" {
		return Check{ID: "no-hardcoded-theme", OK: false, Reason: "含硬编码十六进制颜色 " + m + "，应使用 var(--token)"}
	}
	if m := reColorFunc.FindString(s); m != "" {
		return Check{ID: "no-hardcoded-theme", OK: false, Reason: "含字面量颜色函数 " + strings.TrimSpace(m) + "，应使用 var(--token)"}
	}
	for _, fm := range reFontSize.FindAllStringSubmatch(s, -1) {
		val := strings.TrimSpace(fm[1])
		if !strings.Contains(val, "var(") {
			return Check{ID: "no-hardcoded-theme", OK: false, Reason: "含硬编码字号 font-size:" + val + "，应使用 var(--token)"}
		}
	}
	return Check{ID: "no-hardcoded-theme", OK: true}
}

// 4) 图片含 alt。
func checkImageAlt(s string) Check {
	for _, tag := range reImgTag.FindAllString(s, -1) {
		if !regexp.MustCompile(`(?i)\balt\s*=`).MatchString(tag) {
			return Check{ID: "img-alt", OK: false, Reason: "存在缺少 alt 的 <img>：" + truncate(tag, 60)}
		}
	}
	return Check{ID: "img-alt", OK: true}
}

// 5) 动效用约定属性（不在单页内联散落 @keyframes；关键帧属公共层/fx 资产 DS-ANIM-001/DS-HTML-007）。
func checkAnimationConvention(s string) Check {
	if reKeyframes.MatchString(s) {
		return Check{ID: "animation-convention", OK: false, Reason: "单页内联 @keyframes；动效应经 data-animate/data-fx 约定挂载，关键帧置于公共层或 fx 资产"}
	}
	return Check{ID: "animation-convention", OK: true}
}

// 6) 无被禁的重运行时依赖（白名单外的 <script src>）。
func checkNoHeavyRuntime(s string) Check {
	for _, m := range reScriptSrc.FindAllStringSubmatch(s, -1) {
		src := strings.TrimSpace(m[1])
		if isRelativeSrc(src) || hostAllowed(src) {
			continue
		}
		return Check{ID: "no-heavy-runtime", OK: false, Reason: "含白名单外的外部脚本依赖：" + src + "（仅允许本地脚本与 CDN webfont/highlight/chart）"}
	}
	return Check{ID: "no-heavy-runtime", OK: true}
}

// 7) 可被 ?preview=N 单独渲染：单页 html 应为可独立打开的完整文档。
func checkPreviewable(s string) Check {
	l := strings.ToLower(s)
	if strings.Contains(l, "<html") && strings.Contains(l, "<body") {
		return Check{ID: "previewable", OK: true}
	}
	return Check{ID: "previewable", OK: false, Reason: "单页 html 应为完整文档（含 <html> 与 <body>），以便 ?preview=N 独立渲染"}
}

func isRelativeSrc(src string) bool {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") || strings.HasPrefix(src, "//") {
		return false
	}
	return true
}

func hostAllowed(src string) bool {
	for _, h := range scriptSrcAllowHosts {
		if strings.Contains(src, h) {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
