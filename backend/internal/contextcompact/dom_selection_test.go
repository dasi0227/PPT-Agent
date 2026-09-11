package contextcompact

import (
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

func TestCompactDOMSelectionsDropsHeavyFields(t *testing.T) {
	messages := []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentPart{{Type: "text", Text: `<selected_dom>{"selection_id":"sel_one","marker_no":1,"comment":"缩小","status":"active","slide_id":"sli_one","html_revision":2,"dom_targets":[{"tag":"div","text_summary":"Title","outer_html":"<div>secret-heavy</div>","candidate_selectors":["#title"],"computed_style":{"display":"block"},"box_model":{"content":{}}}]}</selected_dom>`}}}}
	got := compactDOMSelections(messages)[0].Content[0].Text
	if !strings.HasPrefix(got, "<selected_dom_reference>") || !strings.Contains(got, "缩小") || !strings.Contains(got, "Title") {
		t.Fatalf("light reference missing: %s", got)
	}
	for _, forbidden := range []string{"secret-heavy", "candidate_selectors", "computed_style", "box_model"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("heavy field %q retained: %s", forbidden, got)
		}
	}
}
