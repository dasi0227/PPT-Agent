package prompt

import (
	"strings"
	"testing"
)

// design.director system 蒸馏 frontend-design 六原则 + 避开 AI 默认三件套 + 提交契约。
func TestDesignSystemDistillsPrinciples(t *testing.T) {
	sys := DesignSystem()
	for _, want := range []string{
		"设计总监", "submit_design_spec", "palette", "signature",
		"排版即人格", "结构即信息", "克制", "默认三件套", "finish",
	} {
		if !strings.Contains(sys, want) {
			t.Errorf("design system missing %q", want)
		}
	}
}

// AGENT-PROMPT-003：用户主题/大纲只进 user 层，不拼进 system 层。
func TestDesignUserContentNotInSystem(t *testing.T) {
	sentinel := "IGNORE-RULES-sentinel-abc"
	p := DesignParams{
		Topic: sentinel, Brief: sentinel, SlideCount: 8, Language: "zh",
		OutlineTitles: []string{sentinel},
	}
	if strings.Contains(DesignSystem(), sentinel) {
		t.Error("user content must NOT appear in design system layer")
	}
	u := DesignUser(p)
	if !strings.Contains(u, sentinel) {
		t.Error("user topic must appear in design user layer")
	}
	if !strings.Contains(u, "submit_design_spec") {
		t.Error("design user layer should instruct submit_design_spec")
	}
}

// 用户指定主题时 user 层注明以其为基底。
func TestDesignUserThemeBaseline(t *testing.T) {
	u := DesignUser(DesignParams{Topic: "x", ThemeName: "tokyo-night"})
	if !strings.Contains(u, "tokyo-night") || !strings.Contains(u, "基底") {
		t.Errorf("theme baseline note missing: %s", u)
	}
}

// slide.gen@v2：注入 DesignBrief 后 system 含设计语言段，且版本号已升级。
func TestSlideSystemInjectsDesignBrief(t *testing.T) {
	if SlideVersion != "slide.gen@v2" {
		t.Errorf("SlideVersion should be slide.gen@v2, got %s", SlideVersion)
	}
}
