package contextengine

import (
	"encoding/json"
	"strings"
	"testing"

	pptspec "github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func TestModelSectionsExcludeRoutingAndRetainPageIdentity(t *testing.T) {
	pack := ContextPack{Project: ProjectContext{ID: "private-project", Title: "演示"}, Outline: OutlineContext{Outline: pptspec.Outline{
		Sections: []pptspec.Section{{ID: "sec_a", Slides: []pptspec.SlideNode{{SlideID: "sli_a", Title: "市场"}}}},
	}}}
	raw, _ := json.Marshal(ModelSections(pack))
	for _, forbidden := range []string{"private-project", "project_id", "created_at", "available_context_refs"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("leaked %s: %s", forbidden, raw)
		}
	}
	for _, want := range []string{"sli_a", "市场", `"ordinal":1`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("missing %s: %s", want, raw)
		}
	}
	if pack.Project.ID != "private-project" {
		t.Fatal("projection changed runtime state")
	}
}
