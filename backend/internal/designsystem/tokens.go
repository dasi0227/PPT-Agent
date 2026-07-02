// Package designsystem 实现设计系统的机器校验：
//   - 必需 design token 全集校验（lint-tokens，DS-TOKENS-002 / ASSET-002）
//   - slide html 产出规范校验（lint-slide，DS-HTML-OUTPUT 校验清单）
//
// 供生成期 write_slide 的隐式校验（ARCH-TOOLS-004）与测试使用。纯 Go，无外部运行时。
package designsystem

import (
	"regexp"
)

// requiredTokens 是「必需 token 清单」（校验基线）。
// 权威源：docs/60-design-system/design-tokens.md「必需 token 清单」小节。
// 变更 MUST 同步更新该文档与本列表（DS-TOKENS-004）。
var requiredTokens = []string{
	"--color-bg", "--color-fg", "--color-primary", "--color-accent", "--color-muted",
	"--font-sans", "--text-title", "--text-body", "--space-4", "--radius-md",
	"--shadow-card", "--stage-w", "--stage-h",
}

// RequiredTokens 返回必需 token 清单副本。
func RequiredTokens() []string {
	return append([]string(nil), requiredTokens...)
}

// tokenDefRe 判定某 token 是否被定义（形如 `--color-bg:` 的声明）。
func tokenDefRe(name string) *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta(name) + `\s*:`)
}

// LintTokens 校验一段 tokens.css 是否提供必需 token 全集（DS-TOKENS-002 / ASSET-002）。
// 返回缺失的 token 名列表；空表示齐备。theme 资产用它保证 token 不残缺。
func LintTokens(css []byte) []string {
	var missing []string
	for _, tk := range requiredTokens {
		if !tokenDefRe(tk).Match(css) {
			missing = append(missing, tk)
		}
	}
	return missing
}
