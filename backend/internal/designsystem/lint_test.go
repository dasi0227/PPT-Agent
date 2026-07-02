package designsystem_test

import (
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
)

// validSlide 是一份合规的最小 slide html（供 lint 正例）。
const validSlide = `<!doctype html>
<html lang="zh">
<head>
  <meta charset="utf-8">
  <link rel="stylesheet" href="../../common/tokens.css">
  <link rel="stylesheet" href="../../common/base.css">
</head>
<body>
  <div class="slide-scaler">
    <section class="slide-stage" data-preview>
      <div class="slide-content">
        <h1 class="slide-title">标题</h1>
        <p class="slide-body" data-animate="fade-in">正文</p>
        <img src="assets/img/x.png" alt="示意图">
      </div>
    </section>
  </div>
</body>
</html>`

func lintOK(t *testing.T, html string) {
	t.Helper()
	ok, reason := designsystem.LintSlideResult([]byte(html))
	if !ok {
		t.Fatalf("expected lint pass, got: %s", reason)
	}
}

func lintFail(t *testing.T, html, wantCheckID string) {
	t.Helper()
	for _, c := range designsystem.LintSlide([]byte(html)) {
		if c.ID == wantCheckID {
			if c.OK {
				t.Fatalf("expected check %q to fail", wantCheckID)
			}
			return
		}
	}
	t.Fatalf("check %q not found", wantCheckID)
}

// AC-HTML-001：合规 slide 全部检查项通过。
func TestLintSlideAcceptsValid(t *testing.T) {
	lintOK(t, validSlide)
}

// AC-GEN-002 / AC-TOKENS-001：硬编码十六进制主题色 → 失败。
func TestLintSlideRejectsHexColor(t *testing.T) {
	bad := strings.Replace(validSlide, `class="slide-title"`, `class="slide-title" style="color:#ff0066"`, 1)
	lintFail(t, bad, "no-hardcoded-theme")
}

// AC-TOKENS-001：硬编码字号 → 失败。
func TestLintSlideRejectsHardcodedFontSize(t *testing.T) {
	bad := strings.Replace(validSlide, `class="slide-body"`, `class="slide-body" style="font-size:32px"`, 1)
	lintFail(t, bad, "no-hardcoded-theme")
}

// 走 var(--token) 的字号应通过。
func TestLintSlideAllowsTokenFontSize(t *testing.T) {
	ok := strings.Replace(validSlide, `class="slide-body"`, `class="slide-body" style="font-size:var(--text-body)"`, 1)
	lintOK(t, ok)
}

// DS-HTML-001：缺 16:9 舞台 → 失败。
func TestLintSlideRejectsNoStage(t *testing.T) {
	bad := strings.Replace(validSlide, "slide-stage", "some-box", 1)
	lintFail(t, bad, "stage-16-9")
}

// DS-HTML-004：未引用公共层 → 失败。
func TestLintSlideRejectsNoCommonLayer(t *testing.T) {
	bad := strings.Replace(validSlide, `<link rel="stylesheet" href="../../common/base.css">`, "", 1)
	lintFail(t, bad, "common-layer")
}

// DS-HTML-005：img 缺 alt → 失败。
func TestLintSlideRejectsImgWithoutAlt(t *testing.T) {
	bad := strings.Replace(validSlide, `<img src="assets/img/x.png" alt="示意图">`, `<img src="assets/img/x.png">`, 1)
	lintFail(t, bad, "img-alt")
}

// DS-HTML-002：白名单外重运行时脚本 → 失败。
func TestLintSlideRejectsHeavyRuntime(t *testing.T) {
	bad := strings.Replace(validSlide, "</body>", `<script src="https://evil.example.com/heavy.js"></script></body>`, 1)
	lintFail(t, bad, "no-heavy-runtime")
}

// DS-HTML-002：白名单内 CDN（chart.js via jsdelivr）应通过。
func TestLintSlideAllowsWhitelistedCDN(t *testing.T) {
	ok := strings.Replace(validSlide, "</body>", `<script src="https://cdn.jsdelivr.net/npm/chart.js"></script></body>`, 1)
	lintOK(t, ok)
}

// DS-HTML-007：单页内联 @keyframes → 失败。
func TestLintSlideRejectsInlineKeyframes(t *testing.T) {
	bad := strings.Replace(validSlide, "</head>", `<style>@keyframes x{from{opacity:0}to{opacity:1}}</style></head>`, 1)
	lintFail(t, bad, "animation-convention")
}
