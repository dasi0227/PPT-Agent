package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
)

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
