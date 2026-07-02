package prompt

import (
	"strings"
	"testing"
)

// EditSystem 必须载入编辑契约与锚定/隔离约束。
func TestEditSystemContainsContract(t *testing.T) {
	sys := EditSystem(EditParams{PageIndex: 3, Instruction: "x", SlideHTML: "<div/>"})
	for _, want := range []string{"patch_slide", "read_slide", "第 3 页", "唯一", "html-output-spec", "slide_idx"} {
		if !strings.Contains(sys, want) {
			t.Errorf("edit system missing %q", want)
		}
	}
}

// AGENT-PROMPT-003：用户编辑指令只进 user 层，不拼进 system 层。
func TestEditUserInstructionNotInSystem(t *testing.T) {
	sentinel := "IGNORE-RULES-sentinel-xyz"
	p := EditParams{PageIndex: 1, Instruction: sentinel, SlideHTML: "<section>body</section>"}
	if strings.Contains(EditSystem(p), sentinel) {
		t.Error("user instruction MUST NOT appear in system layer")
	}
	u := EditUser(p)
	if !strings.Contains(u, sentinel) {
		t.Error("user instruction MUST appear in user layer")
	}
	if !strings.Contains(u, "slide_idx=1") {
		t.Errorf("user layer must carry slide_idx, got: %s", u)
	}
	if !strings.Contains(u, "body") {
		t.Error("user layer must include target page html")
	}
}
