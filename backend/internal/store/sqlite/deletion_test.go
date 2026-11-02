package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
)

func TestDeletionAcceptsWithoutWaitingForJournalDelivery(t *testing.T) {
	for _, kind := range []string{"thread", "project"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := checkpointFixture(t)
			unlock := s.lockThreadDelivery("t")
			defer unlock()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			var err error
			if kind == "thread" {
				err = s.DeleteThread(ctx, "t")
			} else {
				err = s.DeleteProject(ctx, "p")
			}
			if err != nil {
				t.Fatalf("active run or held journal lock blocked deletion: %v", err)
			}
			if _, err := s.GetRun(ctx, "r"); err != nil {
				t.Fatalf("undo window lost the retained run: %v", err)
			}
			if kind == "project" {
				_, err = s.GetProject(ctx, "p")
			} else {
				_, err = s.GetThread(ctx, "t")
			}
			if !errors.Is(err, run.ErrRunNotFound) {
				t.Fatalf("pending target remains visible: %v", err)
			}
			pending, err := s.PendingDeletions(ctx)
			if err != nil || len(pending) != 1 || pending[0].Kind != kind {
				t.Fatalf("cleanup receipt not durable: %+v %v", pending, err)
			}
		})
	}
}

func TestDeletedThreadCannotReturnThroughProjectRestore(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	if err := s.CreateThread(ctx, model.Thread{ID: "keep", ProjectID: "p", Title: "Keep"}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.CaptureProject(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteThread(ctx, "t"); err != nil {
		t.Fatal(err)
	}
	target := expireDeletion(t, s, "thread", "t")
	if claimed, err := s.FinalizeDeletion(ctx, target); err != nil || !claimed {
		t.Fatalf("finalize: %v %v", claimed, err)
	}
	if err := s.RestoreProject(ctx, "p", snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetThread(ctx, "t"); !errors.Is(err, run.ErrRunNotFound) {
		t.Fatalf("snapshot revived deleted thread: %v", err)
	}
	if _, err := s.GetThread(ctx, "keep"); err != nil {
		t.Fatalf("deleting one thread affected another: %v", err)
	}
	if err := s.CreateThread(ctx, model.Thread{ID: "t", ProjectID: "p"}); !errors.Is(err, run.ErrRunNotFound) {
		t.Fatalf("deleted identity was reused: %v", err)
	}
	if pending, err := s.PendingDeletions(ctx); err != nil || len(pending) != 1 {
		t.Fatalf("restore lost deletion receipt: %+v %v", pending, err)
	}
}

func TestDeletedProjectCannotReturnThroughRestoreOrRecreation(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	snapshot, err := s.CaptureProject(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProject(ctx, "p"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProject(ctx, "p"); err != nil {
		t.Fatalf("repeat deletion failed: %v", err)
	}
	if err := s.RestoreProject(ctx, "p", snapshot); !errors.Is(err, run.ErrRunNotFound) {
		t.Fatalf("snapshot revived deleted project: %v", err)
	}
	if err := s.CreateProject(ctx, model.Project{ID: "p", Title: "revived", Theme: "default"}); !errors.Is(err, run.ErrRunNotFound) {
		t.Fatalf("deleted identity was reused: %v", err)
	}
}

// Move only the persisted deadline; expiry tests need no wall-clock sleeps.
func expireDeletion(t *testing.T, s *Store, kind, id string) store.Deletion {
	t.Helper()
	if err := s.db.Model(&idempotencyPO{}).Where("scope = ? AND owner_id = ? AND key = ?", deletionScope, id, kind).
		Update("result_json", gorm.Expr("json_set(result_json, '$.undo_until', ?)", time.Now().Add(-time.Second).UnixMilli())).Error; err != nil {
		t.Fatal(err)
	}
	target, err := s.GetDeletion(context.Background(), kind, id)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestDeletionUndoRetainsRowsAndSurvivesStoreRestart(t *testing.T) {
	for _, kind := range []string{"project", "thread"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := checkpointFixture(t)
			ctx := context.Background()
			id := "p"
			var err error
			if kind == "project" {
				err = s.DeleteProject(ctx, id)
			} else {
				id = "t"
				err = s.DeleteThread(ctx, id)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := s.GetDeletion(ctx, kind, id)
			if err != nil {
				t.Fatal(err)
			}
			restarted, err := NewStore(s.db)
			if err != nil {
				t.Fatal(err)
			}
			target, err := restarted.GetDeletion(ctx, kind, id)
			if err != nil || target.Token != before.Token || target.UndoUntil != before.UndoUntil {
				t.Fatalf("restart changed undo authority: %+v %v", target, err)
			}
			if claimed, err := restarted.FinalizeDeletion(ctx, target); err != nil || claimed {
				t.Fatalf("early cleanup: %v %v", claimed, err)
			}
			if err := restarted.RequestDeletionRestore(ctx, target); err != nil {
				t.Fatal(err)
			}
			expireDeletion(t, restarted, kind, id)
			if claimed, err := restarted.FinalizeDeletion(ctx, target); err != nil || claimed {
				t.Fatalf("expiry defeated accepted undo: %v %v", claimed, err)
			}
			if err := restarted.QuiesceDeletion(ctx, target); err != nil {
				t.Fatal(err)
			}
			if err := restarted.FinishDeletionRestore(ctx, target); err != nil {
				t.Fatal(err)
			}
			if _, err := restarted.GetProject(ctx, "p"); err != nil {
				t.Fatal(err)
			}
			if _, err := restarted.GetThread(ctx, "t"); err != nil {
				t.Fatal(err)
			}
			restored, err := restarted.GetRun(ctx, "r")
			if err != nil || restored.Status != model.RunCanceled {
				t.Fatalf("undo resumed or lost work: %+v %v", restored, err)
			}
		})
	}
}

func TestDeletionExpiryAndStaleTokenCannotRestoreLaterDeletion(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	if err := s.DeleteThread(ctx, "t"); err != nil {
		t.Fatal(err)
	}
	first, _ := s.GetDeletion(ctx, "thread", "t")
	if err := s.RequestDeletionRestore(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishDeletionRestore(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteThread(ctx, "t"); err != nil {
		t.Fatal(err)
	}
	second, _ := s.GetDeletion(ctx, "thread", "t")
	if second.Token == first.Token {
		t.Fatal("reused deletion token")
	}
	if err := s.RequestDeletionRestore(ctx, first); !errors.Is(err, store.ErrDeletionExpired) {
		t.Fatalf("stale undo accepted: %v", err)
	}
	if claimed, err := s.FinalizeDeletion(ctx, first); err != nil || claimed {
		t.Fatalf("stale cleanup accepted: %v %v", claimed, err)
	}
	second = expireDeletion(t, s, "thread", "t")
	if err := s.RequestDeletionRestore(ctx, second); !errors.Is(err, store.ErrDeletionExpired) {
		t.Fatalf("expired undo accepted: %v", err)
	}
	if claimed, err := s.FinalizeDeletion(ctx, second); err != nil || !claimed {
		t.Fatalf("expired deletion not finalized: %v %v", claimed, err)
	}
	if _, err := s.GetRun(ctx, "r"); !errors.Is(err, run.ErrRunNotFound) {
		t.Fatalf("finalized run retained: %v", err)
	}
}

func TestPendingThreadDeletionBlocksHistoryReplacementAndParentUndoPreservesIt(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	snapshot, err := s.CaptureProject(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteThread(ctx, "t"); err != nil {
		t.Fatal(err)
	}
	thread, _ := s.GetDeletion(ctx, "thread", "t")
	if err := s.RestoreProject(ctx, "p", snapshot); !errors.Is(err, store.ErrDeletionPending) {
		t.Fatalf("history replaced retained rows: %v", err)
	}
	if err := s.DeleteProject(ctx, "p"); err != nil {
		t.Fatal(err)
	}
	project, _ := s.GetDeletion(ctx, "project", "p")
	if err := s.RequestDeletionRestore(ctx, project); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishDeletionRestore(ctx, project); err != nil {
		t.Fatal(err)
	}
	current, err := s.GetDeletion(ctx, "thread", "t")
	a, _ := json.Marshal(thread)
	b, _ := json.Marshal(current)
	if err != nil || string(a) != string(b) {
		t.Fatalf("parent undo changed independent deletion: %+v %v", current, err)
	}
	if _, err := s.GetThread(ctx, "t"); !errors.Is(err, run.ErrRunNotFound) {
		t.Fatalf("parent undo revived hidden thread: %v", err)
	}
}
