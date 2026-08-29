package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

func TestRunPauseLifecycleIsDurableAndAtomic(t *testing.T) {
	store := newTestStore(t)
	seedProject(t, store)
	ctx := context.Background()
	if err := store.CreateThread(ctx, model.Thread{
		ID: "thread", ProjectID: "p1", HistoryPath: "threads/thread.jsonl",
		Status: "active", CreatedAt: 1, UpdatedAt: 1,
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
