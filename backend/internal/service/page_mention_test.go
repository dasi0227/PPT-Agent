package service

import (
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func pageMentionSnapshot() spec.ProjectContentSnapshot {
	return spec.ProjectContentSnapshot{
		Outline: spec.Outline{Sections: []spec.Section{{
			ID: "sec_1",
			Slides: []spec.SlideNode{
				{SlideID: "sli_a", Title: "封面", Role: spec.SlideRoleCover},
				{SlideID: "sli_b", Title: "融资历程", Role: spec.SlideRoleContent},
			},
		}}},
		SlidesByID: map[string]spec.SlideContent{
			"sli_a": {SpecState: "pending", HTMLState: "not_materialized"},
			"sli_b": {SpecState: "ready", HTMLState: "spec_stale"},
		},
	}
}

func TestResolveMentionedPagesPreservesOrderDeduplicatesAndDropsMissing(t *testing.T) {
	pages, dropped, err := resolveMentionedPages(
		pageMentionSnapshot(),
		[]string{"sli_b", "sli_missing", "sli_b", "sli_a"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 || pages[0].SlideID != "sli_b" || pages[1].SlideID != "sli_a" {
		t.Fatalf("unexpected pages: %#v", pages)
	}
	if pages[0].Ordinal != 2 || pages[0].Title != "融资历程" ||
		pages[0].SpecState != "ready" || pages[0].HTMLState != "spec_stale" {
		t.Fatalf("page metadata was not resolved from the current snapshot: %#v", pages[0])
	}
	if len(dropped) != 1 || dropped[0] != "sli_missing" {
		t.Fatalf("unexpected dropped ids: %#v", dropped)
	}
}

func TestResolveMentionedPagesAllowsAllMissingAndRejectsInvalidSelection(t *testing.T) {
	pages, dropped, err := resolveMentionedPages(pageMentionSnapshot(), []string{"sli_missing"})
	if err != nil || len(pages) != 0 || len(dropped) != 1 {
		t.Fatalf("all-missing selection should degrade: pages=%#v dropped=%#v err=%v", pages, dropped, err)
	}

	_, _, err = resolveMentionedPages(pageMentionSnapshot(), []string{"current"})
	var agentErr *model.AgentError
	if !errors.As(err, &agentErr) || agentErr.Code != "PAGE_SELECTION_INVALID" {
		t.Fatalf("invalid id should return PAGE_SELECTION_INVALID, got %v", err)
	}

	tooMany := make([]string, model.MaxMentionedPages+1)
	for i := range tooMany {
		tooMany[i] = "sli_x"
	}
	_, _, err = resolveMentionedPages(pageMentionSnapshot(), tooMany)
	if !errors.As(err, &agentErr) || agentErr.Code != "PAGE_SELECTION_INVALID" {
		t.Fatalf("oversized selection should return PAGE_SELECTION_INVALID, got %v", err)
	}
}
