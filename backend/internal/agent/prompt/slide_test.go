package prompt

import (
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
)

// SlideSystem 必须载入 html-output-spec 的关键硬约束与提交契约。
func TestSlideSystemContainsContract(t *testing.T) {
	p := SlideParams{
		Slide:     slidejson.SlideJSON{Idx: 0, Layout: "cover", Title: "封面"},
		TokensRel: "../../common/tokens.css",
		BaseRel:   "../../common/base.css",
		Layouts:   slidejson.LayoutEnum(),
	}
	sys := SlideSystem(p)
	for _, want := range []string{
		"write_slide", "slide-stage", "tokens.css", "base.css",
		"var(--token)", "alt", "data-animate", "bullets", "16:9",
	} {
		if !strings.Contains(sys, want) {
			t.Errorf("slide system missing %q", want)
		}
	}
}

// AGENT-PROMPT-003：用户内容（标题/要点/意图）只进 user 层，不拼进 system 层。
func TestSlideUserContentNotInSystem(t *testing.T) {
	sentinel := "IGNORE-RULES-sentinel-xyz"
	p := SlideParams{
		Slide: slidejson.SlideJSON{
			Idx: 2, Layout: "bullets", Title: sentinel,
			Bullets: []string{sentinel}, ContentIntent: sentinel,
		},
		TokensRel: "../../common/tokens.css",
		BaseRel:   "../../common/base.css",
		Layouts:   slidejson.LayoutEnum(),
	}
	if strings.Contains(SlideSystem(p), sentinel) {
		t.Error("user slide content must NOT appear in system layer")
	}
	if !strings.Contains(SlideUser(p), sentinel) {
		t.Error("user slide content must appear in user layer")
	}
	// user 层须带 slide_idx 供工具定位。
	if !strings.Contains(SlideUser(p), "slide_idx=2") {
		t.Errorf("user layer must carry slide_idx, got: %s", SlideUser(p))
	}
}
