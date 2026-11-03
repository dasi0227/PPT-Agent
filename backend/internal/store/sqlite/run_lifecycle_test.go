package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func TestOwnedRunTerminalStatusReleasesProject(t *testing.T) {
	for _, status := range []model.RunStatus{model.RunDone, model.RunFailed, model.RunCanceled} {
		t.Run(string(status), func(t *testing.T) {
			s, cp := checkpointFixture(t)
			ctx := workflow.WithCheckpointLease(context.Background(), cp.OwnerInstanceID, cp.ExecutionRevision, 0)
			if err := s.SetRunStatus(ctx, cp.RunID, status); err != nil {
				t.Fatal(err)
			}
			if err := s.SetRunStatus(ctx, cp.RunID, status); err != nil {
				t.Fatalf("repeating the same terminal transition: %v", err)
			}
			stored, err := s.GetRun(context.Background(), cp.RunID)
			if err != nil || stored.Status != status || stored.OwnerInstanceID != "" {
				t.Fatalf("terminal state not committed: %+v, %v", stored, err)
			}
			if active, err := s.HasActiveRun(context.Background(), "p"); err != nil || active {
				t.Fatalf("terminal run still blocks project: %v, %v", active, err)
			}
			stale := workflow.WithCheckpointLease(context.Background(), cp.OwnerInstanceID, cp.ExecutionRevision+1, 0)
			if err := s.SetRunStatus(stale, cp.RunID, status); !errors.Is(err, run.ErrRunRevisionConflict) {
				t.Fatalf("different execution accepted: %v", err)
			}
		})
	}
}

func TestTerminalUpdateCommitsRunStatusAndBatchAtomically(t *testing.T) {
	for _, tc := range []struct {
		kind   model.EventType
		status model.RunStatus
	}{{model.EventRunCompleted, model.RunDone}, {model.EventRunFailed, model.RunFailed}, {model.EventRunError, model.RunFailed}, {model.EventRunCanceled, model.RunCanceled}} {
		t.Run(string(tc.kind), func(t *testing.T) {
			s, cp := checkpointFixture(t)
			ctx := runContextForTest(t, s, cp.RunID)
			if tc.status == model.RunDone {
				runUpdateForTest(t, s, cp.RunID, model.EventMessageFinal, model.MessageFinalPayload{PublicEventBase: model.NewPublicEventBase(cp.RunID), MessageID: "final", Text: "完成", AffectedTargets: []model.PublicTarget{}})
			}
			before, err := readSessionHead(s.reader, "t")
			if err != nil {
				t.Fatal(err)
			}
			var publicErr *model.PublicError
			if tc.status == model.RunFailed {
				publicErr = model.NewAgentError("RUN_FAILED", "test", nil).Public()
			}
			fact, err := sessionstate.NewRunUpdate(tc.kind, model.NewRunTerminalPayload(cp.RunID, 0, nil, publicErr))
			if err != nil {
				t.Fatal(err)
			}
			if err := s.db.Exec("CREATE TRIGGER fail_terminal BEFORE UPDATE OF status ON runs BEGIN SELECT RAISE(ABORT,'injected state failure'); END").Error; err != nil {
				t.Fatal(err)
			}
			if _, err := s.ApplyRunUpdate(ctx, cp.RunID, fact); err == nil {
				t.Fatal("terminal survived status failure")
			}
			after, err := readSessionHead(s.reader, "t")
			if err != nil || after.LastSeq != before.LastSeq {
				t.Fatalf("failed transition published batch: %+v %v", after, err)
			}
			if err := s.db.Exec("DROP TRIGGER fail_terminal").Error; err != nil {
				t.Fatal(err)
			}
			stale := workflow.WithCheckpointLease(sessionstate.WithRequestScene(context.Background(), 1), "other-owner", cp.ExecutionRevision, 0)
			if _, err := s.ApplyRunUpdate(stale, cp.RunID, fact); !errors.Is(err, run.ErrRunRevisionConflict) {
				t.Fatalf("stale terminal accepted: %v", err)
			}
			receipt, err := s.ApplyRunUpdate(ctx, cp.RunID, fact)
			if err != nil {
				t.Fatal(err)
			}
			stored, err := s.GetRun(context.Background(), cp.RunID)
			if err != nil || stored.Status != tc.status || stored.OwnerInstanceID != "" {
				t.Fatalf("terminal state: %+v %v", stored, err)
			}
			repeated, err := s.ApplyRunUpdate(ctx, cp.RunID, fact)
			if err != nil || repeated.CommitID != receipt.CommitID {
				t.Fatalf("terminal duplicated: %+v %v", repeated, err)
			}
			if err := s.CreateRun(context.Background(), model.Run{ID: "next", ThreadID: "t", ProjectID: "p", SceneRevision: 1, ClientRequestID: "next", Command: activeRunSpec(), Status: model.RunPending}); err != nil {
				t.Fatalf("project remains locked: %v", err)
			}
		})
	}
}

func TestRunPauseLifecycleIsDurableAndAtomic(t *testing.T) {
	s, cp := checkpointFixture(t)
	ctx := sessionstate.WithRequestScene(context.Background(), cp.SceneRevision)
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatal(err)
	}
	paused, err := s.PauseNonTerminalRuns(ctx, "server_restarted", 10)
	if err != nil || len(paused) != 1 || paused[0].Status != model.RunPaused || paused[0].OwnerInstanceID != "" {
		t.Fatalf("startup pause: %+v %v", paused, err)
	}
	if active, err := s.HasActiveRun(ctx, "p"); err != nil || !active {
		t.Fatalf("paused run must keep project locked: %v %v", active, err)
	}
	claimed, err := resumeRunForTest(t, s, cp.RunID, "new-process", "resume")
	if err != nil || claimed.Current.Status != model.RunRecovering || claimed.Current.OwnerInstanceID != "new-process" {
		t.Fatalf("claim: %+v %v", claimed, err)
	}
	if _, err := resumeRunForTest(t, s, cp.RunID, "racing-process", "second"); !errors.Is(err, run.ErrRunNotContinuable) {
		t.Fatalf("duplicate claim: %v", err)
	}
	if _, err := s.PauseRun(ctx, cp.RunID, "new-process", "resume_failed", 20); err != nil {
		t.Fatal(err)
	}
	canceled, err := s.RequestRunCancel(ctx, cp.RunID, model.RunCancelUserRequested, currentRunAction(t, s, cp.RunID, "cancel"))
	if err != nil || canceled.Current.Status != model.RunCanceled || canceled.Current.CancelRequestedAt == 0 || canceled.Current.PauseReason != "" || canceled.Current.PausedAt != 0 {
		t.Fatalf("paused cancellation: %+v %v", canceled, err)
	}
	if _, err := s.GetActiveRunForThread(ctx, "t"); !errors.Is(err, run.ErrRunNotFound) {
		t.Fatalf("terminal run remained active: %v", err)
	}
}
