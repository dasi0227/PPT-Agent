// Package designsystem validates the public theme token contract.
package designsystem

import (
	"regexp"
)

const DefaultTheme = "editorial-serif"

// Required theme tokens back the public roles in runtimeassets/base.css.
// Canvas geometry belongs to the runtime, not to a theme.
var requiredTokens = []string{
	"--font-sans",
	"--font-mono",
	"--text-body",
	"--text-subtitle",
	"--text-caption",
	"--weight-body",
	"--leading-body",
	"--leading-title",
	"--space-4",
	"--stage-padding-x",
	"--stage-padding-y",
	"--border-width",
	"--border-style",
	"--quote-border-width",
	"--color-bg",
	"--color-fg",
	"--color-secondary",
	"--color-caption",
	"--color-primary",
	"--color-accent",
	"--color-on-accent",
	"--color-surface",
	"--color-surface-soft",
	"--color-border",
	"--font-display",
	"--text-title",
	"--text-section",
	"--text-metric",
	"--weight-title",
	"--tracking-title",
	"--radius-md",
	"--shadow-card",
	"--chart-primary",
	"--chart-secondary",
	"--chart-accent",

	// Surface foregrounds.
	"--color-on-surface",
	"--color-on-surface-secondary",
	"--color-on-surface-caption",
	"--color-on-surface-soft",
	"--color-on-surface-soft-secondary",
	"--color-on-surface-soft-caption",
	"--color-on-accent-secondary",
	"--color-on-accent-caption",
	"--color-highlight",
	"--color-on-highlight",

	// Semantic states.
	"--color-success",
	"--color-success-surface",
	"--color-success-border",
	"--color-warning",
	"--color-warning-surface",
	"--color-warning-border",
	"--color-danger",
	"--color-danger-surface",
	"--color-danger-border",
	"--color-info",
	"--color-info-surface",
	"--color-info-border",

	// Type and spacing.
	"--weight-emphasis",
	"--leading-caption",
	"--space-1",
	"--space-2",
	"--space-3",
	"--space-6",
	"--space-8",
	"--card-padding",
	"--table-padding-x",
	"--table-padding-y",
	"--list-gap",
	"--metric-gap",
	"--quote-padding",

	// Surfaces.
	"--radius-sm",
	"--radius-lg",
	"--shadow-sm",
	"--shadow-lg",

	// Charts.
	"--chart-series-1",
	"--chart-series-2",
	"--chart-series-3",
	"--chart-series-4",
	"--chart-series-5",
	"--chart-series-6",
	"--chart-series-7",
	"--chart-series-8",
	"--chart-highlight",
	"--chart-muted",
	"--chart-positive",
	"--chart-negative",
	"--chart-line-width",
	"--chart-grid-width",
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
