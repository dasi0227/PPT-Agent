package config

import (
	"regexp"
	"strings"
)

var modelProviderRules = []struct {
	provider string
	pattern  *regexp.Regexp
}{
	{"deepseek", regexp.MustCompile(`^deepseek($|[-_.:]|[0-9])`)},
	{"kimi", regexp.MustCompile(`^(kimi|moonshot)($|[-_.:]|[0-9])`)},
	{"mimo", regexp.MustCompile(`^mimo($|[-_.:]|[0-9])`)},
	{"openai", regexp.MustCompile(`^((chat)?gpt($|[-_.:]|[0-9])|o[134]($|[-_.:]))`)},
	{"anthropic", regexp.MustCompile(`^claude($|[-_.:]|[0-9])`)},
	{"gemini", regexp.MustCompile(`^gemini($|[-_.:]|[0-9])`)},
	{"qwen", regexp.MustCompile(`^qwen($|[-_.:]|[0-9])`)},
	{"minimax", regexp.MustCompile(`^minimax($|[-_.:]|[0-9])`)},
	{"zai", regexp.MustCompile(`^(glm|zai)($|[-_.:]|[0-9])`)},
}

// InferModelProvider is display metadata only. Never infer a connection or
// credentials from it: gateways can serve several brands at the same address.
func InferModelProvider(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	// Gateways often namespace model IDs, e.g. deepseek/deepseek-chat.
	if i := strings.LastIndex(model, "/"); i >= 0 {
		model = model[i+1:]
	}
	for _, rule := range modelProviderRules {
		if rule.pattern.MatchString(model) {
			return rule.provider
		}
	}
	return "custom"
}
