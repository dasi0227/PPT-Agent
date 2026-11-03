package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func checkpointFixture(t *testing.T) (*Store, workflow.RuntimeCheckpoint) {
	t.Helper()
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateProject(ctx, model.Project{ID: "p", Title: "project", Theme: "theme", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateThread(ctx, model.Thread{ID: "t", ProjectID: "p", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	scope := model.NewRunScope(model.ScopeAllPages)
	if err := s.CreateRun(ctx, model.Run{ID: "r", ProjectID: "p", ThreadID: "t", SceneRevision: 1, ClientRequestID: "fixture-input", ExecutionRevision: 1, Command: model.RunCommand{Scope: scope, Mode: model.ModePlan, Instruction: "original input"}, OwnerInstanceID: "owner", Status: model.RunRunning, CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	return s, workflow.RuntimeCheckpoint{RunID: "r", SceneRevision: 1, LoopID: "loop", OwnerInstanceID: "owner", ExecutionRevision: 1, Scope: scope, Mode: model.ModePlan, Phase: workflow.PhasePlanning}
}
func TestCheckpointRejectsLateOwnerAndRevision(t *testing.T) {
	s, cp := checkpointFixture(t)
	ctx := context.Background()
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatalf("exact checkpoint retry rejected: %v", err)
	}
	latest, err := s.LatestCheckpoint(ctx, "r")
	if err != nil {
		t.Fatal(err)
	}
	latest.OwnerInstanceID = "old-owner"
	if err := s.SaveCheckpoint(ctx, latest); !errors.Is(err, sessionstate.ErrVersionConflict) {
		t.Fatalf("old owner accepted: %v", err)
	}
	current, err := s.GetRun(ctx, "r")
	if err != nil || current.Command.Instruction != "original input" || current.CheckpointRevision != 1 {
		t.Fatalf("run=%+v err=%v", current, err)
	}
}

func TestCheckpointKeepsCreatedPagesAndApprovedScopeConsistent(t *testing.T) {
	s, cp := checkpointFixture(t)
	ctx := context.Background()
	cp.Scope.SlideIDs = []string{"sli_created"}
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatal(err)
	}
	current, err := s.GetRun(ctx, cp.RunID)
	if err != nil || !current.Command.Scope.Equal(cp.Scope) {
		t.Fatalf("created-page authority lost: run=%+v err=%v", current, err)
	}
	latest, err := s.LatestCheckpoint(ctx, cp.RunID)
	if err != nil {
		t.Fatal(err)
	}
	approved := model.NewRunScope(model.ScopeAllPages, "sli_created", "sli_added")
	approved.Revision = cp.Scope.Revision + 1
	latest.Scope = approved
	mismatched := approved
	mismatched.SlideIDs = []string{"sli_different"}
	if err := s.CommitScopeExpansion(ctx, cp.RunID, mismatched, latest); err == nil {
		t.Fatal("accepted different checkpoint and run permissions with the same revision")
	}
	if err := s.CommitScopeExpansion(ctx, cp.RunID, approved, latest); err != nil {
		t.Fatal(err)
	}
	current, err = s.GetRun(ctx, cp.RunID)
	if err != nil || !current.Command.Scope.Equal(approved) {
		t.Fatalf("run=%+v err=%v", current, err)
	}
	latest, err = s.LatestCheckpoint(ctx, cp.RunID)
	if err != nil || !latest.Scope.Equal(approved) {
		t.Fatalf("checkpoint=%+v err=%v", latest, err)
	}
}
func TestPlanApprovalAndCheckpointRollbackTogether(t *testing.T) {
	s, cp := checkpointFixture(t)
	ctx := context.Background()
	if err := s.db.Exec(`CREATE TRIGGER fail_checkpoint BEFORE UPDATE OF checkpoint_json ON runs BEGIN SELECT RAISE(ABORT,'disk failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	cp.Mode = model.ModeCreate
	if err := s.CommitPlanApproval(ctx, "r", model.ModeCreate, cp); err == nil {
		t.Fatal("expected failure")
	}
	current, err := s.GetRun(ctx, "r")
	if err != nil || current.Command.Mode != model.ModePlan || current.CheckpointRevision != 0 {
		t.Fatalf("partial approval: %+v %v", current, err)
	}
}
func TestAnswerSurvivesLostAcknowledgmentAndRejectsConflict(t *testing.T) {
	s, cp := checkpointFixture(t)
	cp.Phase = workflow.PhaseWaitingInput
	cp.PendingPlanCall = &workflow.PendingPlan{Call: llm.ToolCall{ID: "plan-call", Name: "plan"}, Request: model.PlanApprovalRequestedPayload{PublicEventBase: model.NewPublicEventBase("r"), InteractionID: "approval", Plan: model.PlanProposal{Content: "制作演示文稿", Steps: []string{"准备页面"}}}}
	if err := s.SaveCheckpoint(context.Background(), cp); err != nil {
		t.Fatal(err)
	}
	ctx := sessionstate.WithInteractionRevision(sessionstate.WithRequestScene(context.Background(), 1), 1)
	answer := []byte(`{"interaction_id":"approval","decision":"approve"}`)
	receipt, err := s.AcceptInteractionAnswer(ctx, "r", "plan", "approval", answer)
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := s.AcceptInteractionAnswer(ctx, "r", "plan", "approval", answer)
	if err != nil || repeat.CommitID != receipt.CommitID {
		t.Fatalf("answer duplicated: %+v %v", repeat, err)
	}
	if _, err := s.AcceptInteractionAnswer(ctx, "r", "plan", "approval", []byte(`{"interaction_id":"approval","decision":"refuse"}`)); !errors.Is(err, sessionstate.ErrIdempotencyConflict) {
		t.Fatalf("conflicting answer: %v", err)
	}
	stored, err := s.Interaction(ctx, "r", "plan", "approval")
	if err != nil || stored.State != "answered" || len(stored.Answer) == 0 {
		t.Fatalf("answer lost: %+v %v", stored, err)
	}
	var count int64
	if err := s.reader.Table("run_interactions").Where("id='approval' AND state='answered'").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("answer count %d %v", count, err)
	}
}

func TestSteeringAndWorkerShareCheckpointAuthority(t *testing.T) {
	s, cp := checkpointFixture(t)
	ctx := workflow.WithCheckpointLease(sessionstate.WithRequestScene(context.Background(), cp.SceneRevision), cp.OwnerInstanceID, cp.ExecutionRevision, 0)
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatal(err)
	}
	scope := cp.Scope
	scope.Revision++
	scope.SlideIDs = []string{"additional"}
	if _, _, err := s.CreateSteering(ctx, model.SteeringMessage{RunID: "r", ThreadID: "t", ClientMessageID: "steer", RequestHash: "hash", Content: "include another page", Scope: scope}); err != nil {
		t.Fatal(err)
	}
	// A model turn may finish with a snapshot made before the input arrived.
	cp.Boundary = "model_finished"
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatalf("accepted steering invalidated worker: %v", err)
	}
	latest, err := s.LatestCheckpoint(ctx, "r")
	if err != nil || !latest.Scope.Equal(cp.Scope) || latest.CheckpointRevision != 2 {
		t.Fatalf("unchecked steering changed worker authority: %+v %v", latest, err)
	}
	pending, err := s.ListPendingSteering(ctx, "r")
	if err != nil || len(pending) != 1 || !pending[0].Scope.Equal(scope) {
		t.Fatalf("accepted input lost: %+v %v", pending, err)
	}
}

func TestAutoCompactionRollsBackContextCheckpointAndPublicResultTogether(t *testing.T) {
	s, cp := checkpointFixture(t)
	cp.Boundary = "context_compacted"
	cp.ContextUpdate = &sessionstate.ContextUpdate{ExpectedRevision: 0, Reason: "compact", MessageIDs: []string{"summary"}, NewMessages: []sessionstate.Message{{ID: "summary", RunID: "r", RoundID: "r", Kind: "summary", Native: llm.Message{Role: llm.RoleUser, Content: llm.TextContent("compacted context")}}}}
	compaction := model.ContextCompaction{ID: "cmp", ThreadID: "t", ProjectID: "p", RunID: "r", Trigger: model.ContextCompactionAuto, Title: "收敛自动压缩结果", Content: "summary", MaxTokens: 65536, CreatedAt: 1}
	fact, err := sessionstate.NewRunUpdate(model.EventContextCompacted, model.ContextCompactedPayload{PublicEventBase: model.NewPublicEventBase("r"), Compaction: compaction})
	if err != nil {
		t.Fatal(err)
	}
	cp.PublicUpdates = []sessionstate.RunUpdate{fact}
	if err := s.db.Exec(`CREATE TRIGGER fail_compaction BEFORE INSERT ON context_compactions BEGIN SELECT RAISE(ABORT,'failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCheckpoint(context.Background(), cp); err == nil {
		t.Fatal("expected rollback")
	}
	before, err := s.LoadContext(context.Background(), "t")
	if err != nil || before.Revision != 0 {
		t.Fatalf("context escaped rollback: %+v %v", before, err)
	}
	current, err := s.GetRun(context.Background(), "r")
	if err != nil || current.CheckpointRevision != 0 {
		t.Fatalf("checkpoint escaped rollback: %+v %v", current, err)
	}
	var count int64
	if err := s.db.Table("timeline_items").Where("id = 'compaction:cmp'").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("public result escaped rollback: %d %v", count, err)
	}
	if err := s.db.Exec(`DROP TRIGGER fail_compaction`).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCheckpoint(context.Background(), cp); err != nil {
		t.Fatal(err)
	}
	after, err := s.LoadContext(context.Background(), "t")
	if err != nil || after.Revision != 1 || len(after.Messages) != 1 {
		t.Fatalf("committed context: %+v %v", after, err)
	}
	if err := s.db.Table("timeline_items").Where("id = 'compaction:cmp'").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("public result: %d %v", count, err)
	}
}
