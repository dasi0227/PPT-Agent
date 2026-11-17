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
