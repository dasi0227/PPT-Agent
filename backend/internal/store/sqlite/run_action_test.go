package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

func currentRunAction(t *testing.T, s *Store, id, key string) sessionstate.RunAction {
	t.Helper()
	r, err := s.GetRun(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return sessionstate.RunAction{SceneRevision: r.SceneRevision, EntityRevision: r.EntityRevision, RequestKey: key}
}

func resumeRunForTest(t *testing.T, s *Store, id, owner, key string) (sessionstate.RunMutation, error) {
	t.Helper()
	current, err := s.GetRun(context.Background(), id)
	if err != nil {
		return sessionstate.RunMutation{}, err
	}
	result, _, err := s.ClaimRunForResume(context.Background(), current, owner, sessionstate.RunAction{SceneRevision: current.SceneRevision, EntityRevision: current.EntityRevision, RequestKey: key})
	return result, err
}

func TestCancelRunCommitsIntentAndReceiptAtomically(t *testing.T) {
	s, cp := checkpointFixture(t)
	ctx := context.Background()
	action := currentRunAction(t, s, cp.RunID, "cancel")
	before, err := readSessionHead(s.reader, "t")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.db.Exec("CREATE TRIGGER fail_cancel_batch BEFORE INSERT ON thread_changes BEGIN SELECT RAISE(ABORT,'injected batch failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.RequestRunCancel(ctx, cp.RunID, model.RunCancelUserRequested, action); err == nil {
		t.Fatal("cancellation committed without its public batch")
	}
	r, err := s.GetRun(ctx, cp.RunID)
	if err != nil || r.CancelRequestedAt != 0 || r.EntityRevision != action.EntityRevision {
		t.Fatalf("partial cancellation: %+v %v", r, err)
	}
	if err := s.db.Exec("DROP TRIGGER fail_cancel_batch").Error; err != nil {
		t.Fatal(err)
	}
	accepted, err := s.RequestRunCancel(ctx, cp.RunID, model.RunCancelUserRequested, action)
	if err != nil || accepted.Current.CancelRequestedAt == 0 || accepted.Current.CancelRequestID != action.RequestKey || accepted.Current.CancelReason != model.RunCancelUserRequested {
		t.Fatalf("intent not durable: %+v %v", accepted, err)
	}
	if accepted.Receipt.Seq != before.LastSeq+1 || accepted.Run.EntityRevision != action.EntityRevision+1 {
		t.Fatalf("cancellation did not publish one commit: %+v", accepted)
	}
	replayed, err := s.RequestRunCancel(ctx, cp.RunID, model.RunCancelUserRequested, action)
	if err != nil || replayed.Receipt.CommitID != accepted.Receipt.CommitID || replayed.Run.EntityRevision != accepted.Run.EntityRevision {
		t.Fatalf("retry did not return original receipt: %+v %v", replayed, err)
	}
	if _, err := s.RequestRunCancel(ctx, cp.RunID, model.RunCancelSuperseded, action); !errors.Is(err, sessionstate.ErrIdempotencyConflict) {
		t.Fatalf("same key changed reason: %v", err)
	}
	stale := action
	stale.RequestKey = "new-stale-action"
	if _, err := s.RequestRunCancel(ctx, cp.RunID, model.RunCancelUserRequested, stale); !errors.Is(err, sessionstate.ErrVersionConflict) {
		t.Fatalf("stale action accepted: %v", err)
	}
	stale.SceneRevision++
	if _, err := s.RequestRunCancel(ctx, cp.RunID, model.RunCancelUserRequested, stale); !errors.Is(err, sessionstate.ErrSceneChanged) {
		t.Fatalf("different scene accepted: %v", err)
	}
}

func TestPausedCancellationRetainsFirstReasonAndRealToolOutcome(t *testing.T) {
	s, cp := checkpointFixture(t)
	ctx := context.Background()
	first, err := s.RequestRunCancel(ctx, cp.RunID, model.RunCancelUserRequested, currentRunAction(t, s, cp.RunID, "first"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PauseRun(ctx, cp.RunID, cp.OwnerInstanceID, "server_restarted", 100); err != nil {
		t.Fatal(err)
	}
	canceled, err := s.RequestRunCancel(ctx, cp.RunID, model.RunCancelSuperseded, currentRunAction(t, s, cp.RunID, "second"))
	if err != nil || canceled.Current.Status != model.RunCanceled || canceled.Current.CancelReason != model.RunCancelUserRequested || canceled.Current.CancelRequestedAt != first.Current.CancelRequestedAt {
		t.Fatalf("first intent was overwritten: %+v %v", canceled, err)
	}
	var terminal model.RunTerminalPayload
	if err := json.Unmarshal(canceled.Run.Terminal, &terminal); err != nil || terminal.Reason != model.RunCancelUserRequested {
		t.Fatalf("terminal lost durable reason: %+v %v", terminal, err)
	}
	var tools int64
	if err := s.reader.Table("tool_executions").Where("run_id = ?", cp.RunID).Count(&tools).Error; err != nil || tools != 0 {
		t.Fatalf("cancel fabricated tool results: %d %v", tools, err)
	}
	if active, err := s.HasActiveRun(ctx, "p"); err != nil || active {
		t.Fatalf("paused cancel retained project lock: %v %v", active, err)
	}
}
