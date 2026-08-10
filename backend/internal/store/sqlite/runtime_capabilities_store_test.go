package sqlite

import (
	"context"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func TestRuntimeCapabilityStoresRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateProject(ctx, model.Project{ID: "p", Title: "p", WorkDir: t.TempDir(), Status: "draft", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateThread(ctx, model.Thread{ID: "t", ProjectID: "p", HistoryPath: "threads/t.jsonl", Status: "active", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	runModel := model.Run{ID: "r", ThreadID: "t", ProjectID: "p", Command: model.RunCommand{
		Scope: model.RunScope{Artifact: model.ArtifactSpec, Level: model.ScopeSlide, SlideID: "s1"},
		Mode:  model.ModeExecute, Instruction: "edit",
	}, Status: model.RunRunning, CreatedAt: 1, UpdatedAt: 1}
	if err := s.CreateRun(ctx, runModel); err != nil {
		t.Fatal(err)
	}
	checkpoint := workflow.RuntimeCheckpoint{
		RunID: "r", LoopID: "loop", Boundary: "runtime_initialized", Phase: workflow.PhaseExecuting,
		Requirements:    &workflow.RequirementLedger{Items: []workflow.RequirementItem{{ID: "req_01", Text: "edit", Status: workflow.RequirementPending}}},
		ContextBriefing: "briefing", ContextIndexRef: "idx",
	}
	if err := s.SaveCheckpoint(ctx, checkpoint); err != nil {
		t.Fatal(err)
	}
	latest, err := s.LatestCheckpoint(ctx, "r")
	if err != nil {
		t.Fatal(err)
	}
	if latest.Boundary != checkpoint.Boundary || latest.ContextBriefing != "briefing" ||
		latest.Requirements.Items[0].ID != "req_01" {
		t.Fatalf("checkpoint=%+v", latest)
	}
	index := workflow.ContextIndex{ID: "idx", RunID: "r", PackHash: "pack", Items: []workflow.ContextIndexItem{{
		RefID: "ref", Kind: "slide_html", Source: "context_index", Hash: "hash", Summary: "summary",
	}}}
	id, err := s.SaveContextIndex(ctx, index)
	if err != nil {
		t.Fatal(err)
	}
	loadedIndex, err := s.LatestContextIndex(ctx, "r")
	if err != nil {
		t.Fatal(err)
	}
	if id != "idx" || loadedIndex.Items[0].RefID != "ref" {
		t.Fatalf("index id=%s loaded=%+v", id, loadedIndex)
	}
	if err := s.SaveSemanticReview(ctx, workflow.StoredSemanticReview{
		ID: "sem", RunID: "r", FinishCallID: "finish", Accepted: true,
		Confidence: 1, InputHash: "input", OutputJSON: `{"accepted":true}`, PromptManifestJSON: `{}`,
	}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := s.db.Raw("SELECT COUNT(*) FROM semantic_reviews WHERE run_id = ?", "r").Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("semantic review count=%d", count)
	}
}
