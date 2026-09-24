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
			input: model.CreateRunScopeInput{Selection: model.ScopeSelectionInput{
				Kind: model.ScopeCurrentPage, CurrentSlideID: "sli_2",
			}},
			want: model.NewRunScope(model.ScopeCurrentPage, "sli_2"),
		},
		{
			name: "custom pages follow outline order and deduplicate",
			input: model.CreateRunScopeInput{Selection: model.ScopeSelectionInput{
				Kind: model.ScopeCustomPages, SlideIDs: []string{"sli_3", "sli_1", "sli_3"},
			}},
			want: model.NewRunScope(model.ScopeCustomPages, "sli_1", "sli_3"),
		},
		{
			name: "section includes subsection pages",
			input: model.CreateRunScopeInput{Selection: model.ScopeSelectionInput{
				Kind: model.ScopeCustomSections, SectionIDs: []string{"sec_a"},
			}},
			want: model.RunScope{SlideIDs: []string{"sli_1", "sli_2"}, Source: model.ScopeSource{
				Kind: model.ScopeCustomSections, SectionIDs: []string{"sec_a"},
			}, Revision: 1},
		},
		{
			name:  "all pages includes future run-created pages",
			input: model.CreateRunScopeInput{Selection: model.ScopeSelectionInput{Kind: model.ScopeAllPages}},
			want:  model.NewRunScope(model.ScopeAllPages, "sli_1", "sli_2", "sli_3"),
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
		{Selection: model.ScopeSelectionInput{Kind: model.ScopeCurrentPage, CurrentSlideID: "sli_missing"}},
		{Selection: model.ScopeSelectionInput{Kind: model.ScopeCustomPages}},
		{Selection: model.ScopeSelectionInput{Kind: model.ScopeCustomSections, SectionIDs: []string{"sec_missing"}}},
	} {
		if _, err := resolveRunScope(snapshot, input); err == nil {
			t.Fatalf("invalid scope accepted: %+v", input)
		}
	}
}

func TestMergeSelectionScopeIsMonotonicAndSingleRevision(t *testing.T) {
	base := model.NewRunScope(model.ScopeCurrentPage, "sli_1")
	selection := model.DOMSelection{SlideID: "sli_2", Status: model.DOMSelectionActive}
	got := mergeSelectionScope(base, scopeSnapshot(), []model.DOMSelection{selection, selection})
	if got.Source.Kind != model.ScopeCustomPages || got.Revision != 2 {
		t.Fatalf("scope = %+v", got)
	}
	if len(got.SlideIDs) != 2 || got.SlideIDs[0] != "sli_1" || got.SlideIDs[1] != "sli_2" {
		t.Fatalf("slide ids = %#v", got.SlideIDs)
	}
}

func TestMergeDecorationSelectionPreservesPageBoundary(t *testing.T) {
	base := model.NewRunScope(model.ScopeCurrentPage, "sli_2")
	selection := model.DOMSelection{SlideID: "sli_2", Status: model.DOMSelectionActive, DecorationTargets: []model.DecorationTarget{{Type: "page_number"}}}
	got := mergeSelectionScope(base, scopeSnapshot(), []model.DOMSelection{selection})
	if !got.Equal(base) {
		t.Fatalf("scope = %+v", got)
	}
}

func TestValidateSelectionProjectAllowsOnlyExplicitDeletedPages(t *testing.T) {
	if err := validateSelectionProject(scopeSnapshot(), []model.DOMSelection{{SlideID: "sli_missing", Status: model.DOMSelectionActive}}, nil); err == nil {
		t.Fatal("unknown active page accepted")
	}
	if err := validateSelectionProject(scopeSnapshot(), []model.DOMSelection{{SlideID: "sli_missing", Status: model.DOMSelectionPageDeleted}}, map[string]bool{"sli_missing": true}); err != nil {
		t.Fatalf("deleted page rejected: %v", err)
	}
	if err := validateSelectionProject(scopeSnapshot(), []model.DOMSelection{{SlideID: "sli_other_project", Status: model.DOMSelectionPageDeleted}}, nil); err == nil {
		t.Fatal("unproven deleted page accepted")
	}
}
