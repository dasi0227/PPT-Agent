package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
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
	if err := s.CreateRun(ctx, model.Run{ID: "r", ProjectID: "p", ThreadID: "t", Command: model.RunCommand{Scope: scope, Mode: model.ModePlan, Instruction: "original input"}, OwnerInstanceID: "owner", Status: model.RunRunning, CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	return s, workflow.RuntimeCheckpoint{RunID: "r", LoopID: "loop", OwnerInstanceID: "owner", ExecutionRevision: 1, Scope: scope, Mode: model.ModePlan, Phase: workflow.PhasePlanning}
}
func TestCheckpointRejectsLateOwnerAndRevision(t *testing.T) {
	s, cp := checkpointFixture(t)
	ctx := context.Background()
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCheckpoint(ctx, cp); !errors.Is(err, run.ErrRunRevisionConflict) {
		t.Fatalf("stale checkpoint accepted: %v", err)
	}
	latest, err := s.LatestCheckpoint(ctx, "r")
	if err != nil {
		t.Fatal(err)
	}
	latest.OwnerInstanceID = "old-owner"
	if err := s.SaveCheckpoint(ctx, latest); !errors.Is(err, run.ErrRunRevisionConflict) {
		t.Fatalf("old owner accepted: %v", err)
	}
	current, err := s.GetRun(ctx, "r")
	if err != nil || current.Command.Instruction != "original input" || current.CheckpointRevision != 1 {
		t.Fatalf("run=%+v err=%v", current, err)
	}
}
func TestPlanApprovalAndCheckpointRollbackTogether(t *testing.T) {
	s, cp := checkpointFixture(t)
	ctx := context.Background()
	if err := s.db.Exec(`CREATE TRIGGER fail_checkpoint BEFORE UPDATE OF checkpoint_json ON runs BEGIN SELECT RAISE(ABORT,'disk failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	cp.Mode = model.ModeExecute
	if err := s.CommitPlanApproval(ctx, "r", model.ModeExecute, cp); err == nil {
		t.Fatal("expected failure")
	}
	current, err := s.GetRun(ctx, "r")
	if err != nil || current.Command.Mode != model.ModePlan || current.CheckpointRevision != 0 {
		t.Fatalf("partial approval: %+v %v", current, err)
	}
}
func TestAnswerSurvivesLostAcknowledgmentAndRejectsConflict(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	answer := []byte(`{"decision":"approve"}`)
	if err := s.AcceptInteractionAnswer(ctx, "r", "plan", "approval", answer); err != nil {
		t.Fatal(err)
	}
	if err := s.AcceptInteractionAnswer(ctx, "r", "plan", "approval", answer); err != nil {
		t.Fatal(err)
	}
	if err := s.AcceptInteractionAnswer(ctx, "r", "plan", "approval", []byte(`{"decision":"cancel"}`)); !errors.Is(err, run.ErrReplyMismatch) {
		t.Fatalf("conflicting answer: %v", err)
	}
	stored, err := s.InteractionAnswer(ctx, "r", "plan", "approval")
	if err != nil || string(stored) != string(answer) {
		t.Fatalf("answer=%s err=%v", stored, err)
	}
	events, err := s.ThreadEvents(ctx, "t", 0)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range events {
		if e.Type == "interaction.answer_accepted" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("accepted %d copies", count)
	}
}

func TestSteeringAndWorkerShareCheckpointAuthority(t *testing.T) {
	s, cp := checkpointFixture(t)
	ctx := workflow.WithCheckpointLease(context.Background(), cp.OwnerInstanceID, cp.ExecutionRevision, 0)
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
	if err != nil || latest.Scope.Revision != scope.Revision || latest.CheckpointRevision != 3 {
		t.Fatalf("scope or checkpoint regressed: %+v %v", latest, err)
	}
	pending, err := s.ListPendingSteering(ctx, "r")
	if err != nil || len(pending) != 1 {
		t.Fatalf("accepted input lost: %+v %v", pending, err)
	}
}
