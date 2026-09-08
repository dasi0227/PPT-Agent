package workflow

import (
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func scopeExpansionPack() contextengine.ContextPack {
	return contextengine.ContextPack{Outline: contextengine.OutlineContext{Summaries: []contextengine.SlideSummary{
		{ID: "sli_one", Ordinal: 1}, {ID: "sli_two", Ordinal: 2}, {ID: "sli_three", Ordinal: 3},
	}}}
}

func TestProposeScopeExpansionMergesPagesAndObjectAsCartesianScope(t *testing.T) {
	current := model.NewRunScope(model.ScopeObjectHTML, model.ScopeCustomPages, "sli_one")
	next, addition, err := proposeScopeExpansion(current, scopeExpansionPack(), []string{"sli_three"}, model.ScopeObjectSpec)
	if err != nil {
		t.Fatal(err)
	}
	if next.Object != model.ScopeObjectPresentation || next.Revision != 2 {
		t.Fatalf("unexpected object/revision: %#v", next)
	}
	if len(next.SlideIDs) != 2 || next.SlideIDs[0] != "sli_one" || next.SlideIDs[1] != "sli_three" {
		t.Fatalf("unexpected slides: %#v", next.SlideIDs)
	}
	if len(addition.SlideIDs) != 1 || addition.SlideIDs[0] != "sli_three" {
		t.Fatalf("unexpected addition: %#v", addition)
	}
}

func TestProposeScopeExpansionNoopsForContainedPrivileges(t *testing.T) {
	current := model.NewRunScope(model.ScopeObjectPresentation, model.ScopeCustomPages, "sli_one")
	next, _, err := proposeScopeExpansion(current, scopeExpansionPack(), []string{"sli_one"}, model.ScopeObjectHTML)
	if err != nil {
		t.Fatal(err)
	}
	if !next.Equal(current) {
		t.Fatalf("contained request changed scope: %#v", next)
	}
}

func TestAdjustedScopeCannotShrinkCurrentPrivileges(t *testing.T) {
	current := model.NewRunScope(model.ScopeObjectPresentation, model.ScopeCustomPages, "sli_one", "sli_two")
	candidate := model.NewRunScope(model.ScopeObjectHTML, model.ScopeCustomPages, "sli_one")
	if scopeContains(candidate, current) {
		t.Fatal("shrinking pages or object capabilities must be rejected")
	}
}
