package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func completedSteeringFixture(t *testing.T) (*Store, model.Run, []model.SteeringMessage) {
	t.Helper()
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateProject(ctx, model.Project{ID: "p", WorkDir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateThread(ctx, model.Thread{ID: "t", ProjectID: "p"}); err != nil {
		t.Fatal(err)
	}
	previous := model.Run{ID: "old", ThreadID: "t", ProjectID: "p", Status: model.RunRunning,
		Command: model.RunCommand{Instruction: "original", Mode: model.ModeChat, Scope: model.NewRunScope(model.ScopeAllPages)}}
	if err := s.CreateRun(ctx, previous); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two", "three"} {
		if _, _, err := s.CreateSteering(ctx, model.SteeringMessage{RunID: previous.ID, ThreadID: previous.ThreadID,
			ClientMessageID: id, RequestHash: id, Content: id, Scope: model.NewRunScope(model.ScopeAllPages)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetRunStatus(ctx, previous.ID, model.RunDone); err != nil {
		t.Fatal(err)
	}
	previous.Status = model.RunDone
	messages, err := s.ListPendingSteering(ctx, previous.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, previous, messages
}

func TestSteeringHandoffAcceptsNewRunAndMovesInboxAtomically(t *testing.T) {
	s, previous, messages := completedSteeringFixture(t)
	ctx := run.WithSteeringHandoff(context.Background(), previous.ID, messages)
	next := previous
	next.ID, next.Status, next.ClientRequestID, next.RequestHash = "new", model.RunPending, "queued_old", "hash"
	next.Command.Instruction = messages[0].Content
	if err := s.CreateRun(ctx, next); err != nil {
		t.Fatal(err)
	}
	oldPending, err := s.ListPendingSteering(ctx, previous.ID)
	if err != nil || len(oldPending) != 0 {
		t.Fatalf("old inbox not consumed: %+v %v", oldPending, err)
	}
	newPending, err := s.ListPendingSteering(ctx, next.ID)
	if err != nil || len(newPending) != 2 || newPending[0].Content != "two" || newPending[1].Content != "three" {
		t.Fatalf("new inbox lost ordering: %+v %v", newPending, err)
	}
	if !newPending[0].Scope.Equal(next.Command.Scope) || !newPending[1].Scope.Equal(next.Command.Scope) {
		t.Fatalf("new initial scope authority: %+v", newPending)
	}
	checkpoint := workflow.RuntimeCheckpoint{RunID: next.ID, LoopID: "new-loop", ExecutionRevision: 1, Scope: next.Command.Scope}
	if err := s.SaveCheckpoint(context.Background(), checkpoint); err != nil {
		t.Fatalf("initial checkpoint failed after handoff: %v", err)
	}
	checkpoint, err = s.LatestCheckpoint(context.Background(), next.ID)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint.Scope = newPending[1].Scope
	if err := s.SaveCheckpoint(context.Background(), checkpoint); err != nil {
		t.Fatalf("consuming initial queued scope caused revision conflict: %v", err)
	}
	var first steeringPO
	if err := s.db.First(&first, "thread_id = ? AND client_message_id = ?", "t", "one").Error; err != nil {
		t.Fatal(err)
	}
	if message := first.toModel(); message.RunID != next.ID || message.Status != model.SteeringInjected {
		t.Fatalf("primary message not owned by new Run: %+v", message)
	}
	if value, err := s.GetRun(context.Background(), previous.ID); err != nil || value.Status != model.RunDone {
		t.Fatalf("original Run changed outcome: %+v %v", value, err)
	}
	if recoverable, err := s.CompletedRunsWithPendingSteering(context.Background()); err != nil || len(recoverable) != 0 {
		t.Fatalf("handoff can be repeated after restart: %+v %v", recoverable, err)
	}
}

func TestSteeringHandoffRollsBackNewRunOnInboxMismatch(t *testing.T) {
	s, previous, messages := completedSteeringFixture(t)
	next := previous
	next.ID, next.Status, next.ClientRequestID, next.RequestHash = "new", model.RunPending, "queued_old", "hash"
	ctx := run.WithSteeringHandoff(context.Background(), previous.ID, messages[:1])
	if err := s.CreateRun(ctx, next); !errors.Is(err, run.ErrRunRevisionConflict) {
		t.Fatalf("expected inbox conflict: %v", err)
	}
	if _, err := s.GetRun(context.Background(), next.ID); !errors.Is(err, run.ErrRunNotFound) {
		t.Fatalf("partial Run committed: %v", err)
	}
	pending, err := s.ListPendingSteering(context.Background(), previous.ID)
	if err != nil || len(pending) != 3 {
		t.Fatalf("failed start lost inputs: %+v %v", pending, err)
	}
	if _, err := s.GetIdempotency(context.Background(), "create_run", "t", "queued_old"); !errors.Is(err, run.ErrRunNotFound) {
		t.Fatalf("failed handoff left receipt: %v", err)
	}
}
