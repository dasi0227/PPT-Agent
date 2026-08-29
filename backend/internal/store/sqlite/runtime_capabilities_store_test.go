package sqlite

import (
	"context"
	"sync"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func TestSaveContextIndexConcurrentRetryHasOneSnapshot(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateProject(ctx, model.Project{ID: "p", Title: "p", WorkDir: t.TempDir(), Status: "draft", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateThread(ctx, model.Thread{ID: "t", ProjectID: "p", HistoryPath: "threads/t.jsonl", Status: "active", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(ctx, model.Run{
		ID: "r", ThreadID: "t", ProjectID: "p",
		Command: model.RunCommand{Scope: model.RunScope{Artifact: model.ArtifactSpec, Level: model.ScopeDeck}, Mode: model.ModeExecute},
		Status:  model.RunRunning, CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}

	base := workflow.ContextIndex{ID: "idx", RunID: "r", PackHash: "pack", Items: []workflow.ContextIndexItem{{
		RefID: "ref", Kind: "deck", Source: "project", Hash: "hash", Summary: "summary",
	}}}
	const workers = 12
	start := make(chan struct{})
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(builtAt int64) {
			defer wg.Done()
			<-start
			index := base
			index.BuiltAt = builtAt
			index.Items = append([]workflow.ContextIndexItem(nil), base.Items...)
			index.Items[0].UpdatedAt = builtAt
			_, err := s.SaveContextIndex(ctx, index)
			errs <- err
		}(int64(i + 1))
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := s.db.Model(&contextIndexSnapshotPO{}).Where("run_id = ?", "r").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("concurrent retries created %d snapshots", count)
	}
}

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
		Requirements:      &workflow.RequirementLedger{Items: []workflow.RequirementItem{{ID: "req_01", Text: "edit", Status: workflow.RequirementPending}}},
		ContextBriefing:   "briefing",
		ContextIndexRef:   "idx",
		ActiveDurationMS:  1_234,
		WaitingDurationMS: 5_678,
	}
	if err := s.SaveCheckpoint(ctx, checkpoint); err != nil {
		t.Fatal(err)
	}
	latest, err := s.LatestCheckpoint(ctx, "r")
	if err != nil {
		t.Fatal(err)
	}
	if latest.Boundary != checkpoint.Boundary || latest.ContextBriefing != "briefing" ||
		latest.ActiveDurationMS != 1_234 || latest.WaitingDurationMS != 5_678 ||
		latest.Requirements.Items[0].ID != "req_01" {
		t.Fatalf("checkpoint=%+v", latest)
	}
	index := workflow.ContextIndex{ID: "idx", RunID: "r", PackHash: "pack", BuiltAt: 1, Items: []workflow.ContextIndexItem{{
		RefID: "ref", Kind: "slide_html", Source: "context_index", Hash: "hash", Summary: "summary", UpdatedAt: 1,
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
	retry := index
	retry.BuiltAt = 2
	retry.Items = append([]workflow.ContextIndexItem(nil), index.Items...)
	retry.Items[0].UpdatedAt = 2
	reusedID, err := s.SaveContextIndex(ctx, retry)
	if err != nil || reusedID != id {
		t.Fatalf("same logical index must be reused: id=%s err=%v", reusedID, err)
	}
	var snapshotCount int64
	if err := s.db.Model(&contextIndexSnapshotPO{}).Where("run_id = ?", "r").Count(&snapshotCount).Error; err != nil {
		t.Fatal(err)
	}
	if snapshotCount != 1 {
		t.Fatalf("same logical index created %d snapshots", snapshotCount)
	}
	changed := retry
	changed.BuiltAt = 3
	changed.Items = append([]workflow.ContextIndexItem(nil), retry.Items...)
	changed.Items[0].Summary = "changed summary"
	changed.Items[0].UpdatedAt = 3
	changedID, err := s.SaveContextIndex(ctx, changed)
	if err != nil {
		t.Fatal(err)
	}
	if changedID == id {
		t.Fatalf("changed logical index reused stale id %s", changedID)
	}
	if err := s.db.Model(&contextIndexSnapshotPO{}).Where("run_id = ?", "r").Count(&snapshotCount).Error; err != nil {
		t.Fatal(err)
	}
	if snapshotCount != 2 {
		t.Fatalf("changed logical index snapshots=%d", snapshotCount)
	}
	loadedVersion, err := s.GetContextIndex(ctx, changedID)
	if err != nil || loadedVersion.Items[0].Summary != "changed summary" {
		t.Fatalf("versioned index=%+v err=%v", loadedVersion, err)
	}
	latestVersion, err := s.LatestContextIndex(ctx, "r")
	if err != nil || latestVersion.ID != changedID {
		t.Fatalf("latest index=%+v err=%v", latestVersion, err)
	}
	if err := s.SaveSemanticReview(ctx, workflow.StoredSemanticReview{
		ID: "sem", RunID: "r", FinishCallID: "finish", Accepted: true,
		Confidence: 1, InputHash: "input", OutputJSON: `{"accepted":true}`, PromptManifestJSON: `{}`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSemanticReview(ctx, workflow.StoredSemanticReview{
		ID: "sem", RunID: "r", FinishCallID: "finish", Accepted: true,
		Confidence: 1, InputHash: "input", OutputJSON: `{"accepted":true}`, PromptManifestJSON: `{}`,
	}); err != nil {
		t.Fatalf("semantic review retry must be idempotent: %v", err)
	}
	var count int64
	if err := s.db.Raw("SELECT COUNT(*) FROM semantic_reviews WHERE run_id = ?", "r").Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("semantic review count=%d", count)
	}
}

func TestCommitPlanApprovalAtomicallyUpdatesCommandContextAndCheckpoint(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateProject(ctx, model.Project{ID: "p", Title: "p", WorkDir: t.TempDir(), Status: "draft", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateThread(ctx, model.Thread{ID: "t", ProjectID: "p", HistoryPath: "threads/t.jsonl", Status: "active", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	runModel := model.Run{ID: "plan-run", ThreadID: "t", ProjectID: "p", Command: model.RunCommand{
		Scope: model.RunScope{Artifact: model.ArtifactPPT, Level: model.ScopeDeck},
		Mode:  model.ModePlan, Instruction: "plan then execute",
	}, Status: model.RunWaiting, CreatedAt: 1, UpdatedAt: 1}
	if err := s.CreateRun(ctx, runModel); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRunContext(ctx, model.RunContext{RunID: "plan-run", ContextID: "ctx_plan", Profile: "ppt/deck", PackHash: "plan", ManifestJSON: `{"read_only":true}`, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	plan := &workflow.Plan{ID: "plan-1", Revision: 2, ApprovedRevision: 1, ApprovedContentHash: "hash", Status: workflow.PlanActive, Title: "Plan", Content: "Full plan", Steps: []workflow.PlanStep{{ID: "step-1", Title: "Do it", Status: workflow.PlanStepPending}}}
	checkpoint := workflow.RuntimeCheckpoint{
		RunID: "plan-run", LoopID: "loop-1", Boundary: "plan_updated", Phase: workflow.PhaseExecuting,
		Mode: model.ModeExecute, Plan: plan, ContextBriefing: "mode=execute", ContextIndexRef: "ctxidx_execute",
	}
	if err := s.CommitPlanApproval(ctx, "plan-run", model.ModeExecute, model.RunContext{
		ContextID: "ctx_execute", Profile: "ppt/deck", PackHash: "execute", ManifestJSON: `{"read_only":false}`,
	}, checkpoint); err != nil {
		t.Fatal(err)
	}
	storedRun, err := s.GetRun(ctx, "plan-run")
	if err != nil {
		t.Fatal(err)
	}
	storedContext, err := s.GetRunContext(ctx, "plan-run")
	if err != nil {
		t.Fatal(err)
	}
	storedCheckpoint, err := s.LatestCheckpoint(ctx, "plan-run")
	if err != nil {
		t.Fatal(err)
	}
	if storedRun.Command.Mode != model.ModeExecute || storedRun.Status != model.RunRunning || storedContext.ContextID != "ctx_execute" || storedContext.PackHash != "execute" ||
		storedCheckpoint.Mode != model.ModeExecute || storedCheckpoint.Phase != workflow.PhaseExecuting || storedCheckpoint.Plan == nil || storedCheckpoint.Plan.Content != "Full plan" {
		t.Fatalf("run=%+v context=%+v checkpoint=%+v", storedRun, storedContext, storedCheckpoint)
	}
}

func TestCommitPlanApprovalRollsBackEveryAuthorityFieldOnFailure(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateProject(ctx, model.Project{ID: "p", Title: "p", WorkDir: t.TempDir(), Status: "draft", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateThread(ctx, model.Thread{ID: "t", ProjectID: "p", HistoryPath: "threads/t.jsonl", Status: "active", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"plan-run", "other-run"} {
		if err := s.CreateRun(ctx, model.Run{ID: id, ThreadID: "t", ProjectID: "p", Command: model.RunCommand{
			Scope: model.RunScope{Artifact: model.ArtifactPPT, Level: model.ScopeDeck}, Mode: model.ModePlan, Instruction: "plan",
		}, Status: model.RunWaiting, CreatedAt: 1, UpdatedAt: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SaveRunContext(ctx, model.RunContext{RunID: "plan-run", ContextID: "ctx_plan", ManifestJSON: `{}`, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRunContext(ctx, model.RunContext{RunID: "other-run", ContextID: "ctx_taken", ManifestJSON: `{}`, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	err := s.CommitPlanApproval(ctx, "plan-run", model.ModeExecute, model.RunContext{
		ContextID: "ctx_taken", ManifestJSON: `{"read_only":false}`,
	}, workflow.RuntimeCheckpoint{RunID: "plan-run", LoopID: "loop", Mode: model.ModeExecute, Phase: workflow.PhaseExecuting})
	if err == nil {
		t.Fatal("expected unique context failure")
	}
	storedRun, _ := s.GetRun(ctx, "plan-run")
	storedContext, _ := s.GetRunContext(ctx, "plan-run")
	if storedRun.Command.Mode != model.ModePlan || storedContext.ContextID != "ctx_plan" {
		t.Fatalf("partial transition leaked: run=%+v context=%+v", storedRun, storedContext)
	}
	var checkpoints int64
	if err := s.db.Raw("SELECT COUNT(*) FROM run_checkpoints WHERE run_id = ?", "plan-run").Scan(&checkpoints).Error; err != nil {
		t.Fatal(err)
	}
	if checkpoints != 0 {
		t.Fatalf("partial checkpoint leaked: %d", checkpoints)
	}
}
