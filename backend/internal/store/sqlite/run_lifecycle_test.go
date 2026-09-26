package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
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

func TestTerminalEventCommitsRunStatusAtomically(t *testing.T) {
	for _, tc := range []struct {
		event  model.EventType
		status model.RunStatus
	}{
		{model.EventRunCompleted, model.RunDone},
		{model.EventRunFailed, model.RunFailed},
		{model.EventRunError, model.RunFailed},
		{model.EventRunCanceled, model.RunCanceled},
	} {
		t.Run(string(tc.event), func(t *testing.T) {
			s, cp := checkpointFixture(t)
			ctx := workflow.WithCheckpointLease(context.Background(), cp.OwnerInstanceID, cp.ExecutionRevision, 0)
			// A failure after enqueueing must roll back both state and history.
			if err := s.db.Exec("CREATE TRIGGER fail_terminal BEFORE UPDATE OF status ON runs BEGIN SELECT RAISE(ABORT,'injected state failure'); END").Error; err != nil {
				t.Fatal(err)
			}
			event := model.Event{RunID: cp.RunID, Type: tc.event, Payload: `{}`}
			if err := s.AppendEvent(ctx, &event); err == nil {
				t.Fatal("terminal event succeeded without committing the status")
			}
			assertNoTerminalEvent := func() {
				t.Helper()
				events, err := s.EventsSince(context.Background(), cp.RunID, 0)
				if err != nil {
					t.Fatal(err)
				}
				for _, e := range events {
					if e.Type == tc.event {
						t.Fatal("failed transition published a terminal event")
					}
				}
			}
			assertNoTerminalEvent()
			if err := s.db.Exec("DROP TRIGGER fail_terminal").Error; err != nil {
				t.Fatal(err)
			}
			stale := workflow.WithCheckpointLease(context.Background(), "other-owner", cp.ExecutionRevision, 0)
			if err := s.AppendEvent(stale, &event); !errors.Is(err, run.ErrRunRevisionConflict) {
				t.Fatalf("stale worker published a terminal event: %v", err)
			}
			assertNoTerminalEvent()
			if err := s.AppendEvent(ctx, &event); err != nil {
				t.Fatal(err)
			}
			stored, err := s.GetRun(context.Background(), cp.RunID)
			if err != nil || stored.Status != tc.status || stored.OwnerInstanceID != "" {
				t.Fatalf("event and state disagree: %+v, %v", stored, err)
			}
			if err := s.CreateRun(context.Background(), model.Run{ID: "next", ThreadID: "t", ProjectID: "p", Command: activeRunSpec(), Status: model.RunPending}); err != nil {
				t.Fatalf("new run rejected after terminal event: %v", err)
			}
		})
	}
}

func TestRunPauseLifecycleIsDurableAndAtomic(t *testing.T) {
	store := newTestStore(t)
	seedProject(t, store)
	ctx := context.Background()
	if err := store.CreateThread(ctx, model.Thread{
		ID: "thread", ProjectID: "p1", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRun(ctx, model.Run{
		ID: "run", ThreadID: "thread", ProjectID: "p1", Command: activeRunSpec(),
		Status: model.RunRunning, OwnerInstanceID: "old-process", CreatedAt: 2, UpdatedAt: 2,
	}); err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"tool_call", "commit"} {
		if _, created, err := store.AcquireIdempotency(ctx, model.IdempotencyRecord{
			Scope: scope, OwnerID: "run", Key: scope + "-key", RequestHash: "stable-hash",
		}); err != nil || !created {
			t.Fatalf("seed %s idempotency: created=%v err=%v", scope, created, err)
		}
	}

	paused, err := store.PauseNonTerminalRuns(ctx, "server_restarted", 10)
	if err != nil || len(paused) != 1 {
		t.Fatalf("pause startup: runs=%+v err=%v", paused, err)
	}
	stored, err := store.GetActiveRunForThread(ctx, "thread")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.RunPaused || stored.OwnerInstanceID != "" || stored.PauseReason != "server_restarted" || stored.PausedAt != 10 {
		t.Fatalf("unexpected paused row: %+v", stored)
	}
	if active, err := store.HasActiveRun(ctx, "p1"); err != nil || !active {
		t.Fatalf("paused run must keep project locked: active=%v err=%v", active, err)
	}
	assertInterruptedIdempotencyReclaimed(t, store, ctx)

	claimed, err := store.ClaimPausedRun(ctx, "run", "new-process")
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != model.RunRecovering || claimed.OwnerInstanceID != "new-process" || claimed.PausedAt != 0 {
		t.Fatalf("unexpected claimed row: %+v", claimed)
	}
	if _, err := store.ClaimPausedRun(ctx, "run", "racing-process"); !errors.Is(err, run.ErrRunNotRunning) {
		t.Fatalf("second claim must lose atomically, got %v", err)
	}
	if err := store.ReleaseRecoveringRun(ctx, "run", "new-process", "resume_failed", 20); err != nil {
		t.Fatal(err)
	}
	assertInterruptedIdempotencyReclaimed(t, store, ctx)
	canceled, err := store.CancelPausedRun(ctx, "run", 30)
	if err != nil {
		t.Fatal(err)
	}
	if canceled.Status != model.RunCanceled || canceled.CancelRequestedAt != 30 || canceled.PauseReason != "" || canceled.PausedAt != 0 {
		t.Fatalf("unexpected canceled row: %+v", canceled)
	}
	if _, err := store.GetActiveRunForThread(ctx, "thread"); !errors.Is(err, run.ErrRunNotFound) {
		t.Fatalf("terminal run must not be discoverable as active, got %v", err)
	}
}

func assertInterruptedIdempotencyReclaimed(t *testing.T, store *Store, ctx context.Context) {
	t.Helper()
	for _, scope := range []string{"tool_call", "commit"} {
		record, reclaimed, err := store.AcquireIdempotency(ctx, model.IdempotencyRecord{
			Scope: scope, OwnerID: "run", Key: scope + "-key", RequestHash: "stable-hash",
		})
		if err != nil || !reclaimed || record.Status != "in_progress" {
			t.Fatalf("reclaim %s idempotency: record=%+v reclaimed=%v err=%v", scope, record, reclaimed, err)
		}
	}
}
