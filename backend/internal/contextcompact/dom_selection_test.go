package contextcompact

import (
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

func TestCompactDOMSelectionsDropsHeavyFields(t *testing.T) {
	messages := []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentPart{
		{Type: "text", Text: "回答"},
		{Type: "text", Text: `<selected_dom>{"selection_id":"sel_one","marker_no":1,"comment":"缩小","status":"active","slide_id":"sli_one","html_hash":"sha256:first","dom_targets":[{"tag":"div","text_summary":"Title","outer_html":"<div>secret-heavy</div>","candidate_selectors":["#title"],"computed_style":{"display":"block"},"box_model":{"content":{}}}]}</selected_dom>`},
		{Type: "text", Text: `<selected_dom>{"selection_id":"sel_three","marker_no":3,"comment":"复述第三处","status":"active","slide_id":"sli_one","dom_targets":[{"tag":"p","text_summary":"第三处原文","outer_html":"<p>secret-heavy</p>"}]}</selected_dom>`},
	}}}
	got := compactDOMSelections(llm.NormalizeHistory(messages))[0].Text()
	if !strings.HasPrefix(got, "回答\n\n<selected_dom_reference>") || !strings.Contains(got, "缩小") || !strings.Contains(got, "Title") || !strings.Contains(got, "复述第三处") || !strings.Contains(got, "第三处原文") || strings.Count(got, "<selected_dom_reference>") != 2 {
		t.Fatalf("light reference missing: %s", got)
	}
	for _, forbidden := range []string{"secret-heavy", "candidate_selectors", "computed_style", "box_model"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("heavy field %q retained: %s", forbidden, got)
		}
	}
	if !strings.Contains(messages[0].Text(), "secret-heavy") {
		t.Fatal("compaction mutated the original selection snapshot")
	}
}
