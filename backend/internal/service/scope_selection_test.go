package service

import (
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func scopeSnapshot() spec.ProjectContentSnapshot {
	return spec.ProjectContentSnapshot{Outline: spec.Outline{Sections: []spec.Section{
		{ID: "sec_a", Title: "A", Slides: []spec.SlideNode{{SlideID: "sli_1"}}, Subsections: []spec.Subsection{{ID: "sub_a", Slides: []spec.SlideNode{{SlideID: "sli_2"}}}}},
		{ID: "sec_b", Title: "B", Slides: []spec.SlideNode{{SlideID: "sli_3"}}},
	}}}
}

func TestResolveRunScopeNormalizesSelectionSources(t *testing.T) {
	snapshot := scopeSnapshot()
	tests := []struct {
		name  string
		input model.CreateRunScopeInput
		want  model.RunScope
	}{
		{
			name: "current page",
			input: model.CreateRunScopeInput{Object: model.ScopeObjectHTML, Selection: model.ScopeSelectionInput{
				Kind: model.ScopeCurrentPage, CurrentSlideID: "sli_2",
			}},
			want: model.NewRunScope(model.ScopeObjectHTML, model.ScopeCurrentPage, "sli_2"),
		},
		{
			name: "custom pages follow outline order and deduplicate",
			input: model.CreateRunScopeInput{Object: model.ScopeObjectPresentation, Selection: model.ScopeSelectionInput{
				Kind: model.ScopeCustomPages, SlideIDs: []string{"sli_3", "sli_1", "sli_3"},
			}},
			want: model.NewRunScope(model.ScopeObjectPresentation, model.ScopeCustomPages, "sli_1", "sli_3"),
		},
		{
			name: "section includes subsection pages",
			input: model.CreateRunScopeInput{Object: model.ScopeObjectSpec, Selection: model.ScopeSelectionInput{
				Kind: model.ScopeCustomSections, SectionIDs: []string{"sec_a"},
			}},
			want: model.RunScope{Object: model.ScopeObjectSpec, SlideIDs: []string{"sli_1", "sli_2"}, Source: model.ScopeSource{
				Kind: model.ScopeCustomSections, SectionIDs: []string{"sec_a"},
			}, Revision: 1},
		},
		{
			name:  "global forces all pages",
			input: model.CreateRunScopeInput{Object: model.ScopeObjectGlobal, Selection: model.ScopeSelectionInput{Kind: model.ScopeCurrentPage, CurrentSlideID: "sli_1"}},
			want:  model.NewRunScope(model.ScopeObjectGlobal, model.ScopeAllPages, "sli_1", "sli_2", "sli_3"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveRunScope(snapshot, test.input)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(test.want) {
				t.Fatalf("scope=%+v want %+v", got, test.want)
			}
		})
	}
}

func TestResolveRunScopeRejectsUnknownAndEmptySelections(t *testing.T) {
	snapshot := scopeSnapshot()
	for _, input := range []model.CreateRunScopeInput{
		{Object: model.ScopeObjectPresentation, Selection: model.ScopeSelectionInput{Kind: model.ScopeCurrentPage, CurrentSlideID: "sli_missing"}},
		{Object: model.ScopeObjectPresentation, Selection: model.ScopeSelectionInput{Kind: model.ScopeCustomPages}},
		{Object: model.ScopeObjectPresentation, Selection: model.ScopeSelectionInput{Kind: model.ScopeCustomSections, SectionIDs: []string{"sec_missing"}}},
	} {
		if _, err := resolveRunScope(snapshot, input); err == nil {
			t.Fatalf("invalid scope accepted: %+v", input)
		}
	}
}
