package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
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
			if _, err := s.GetRun(ctx, "r"); !errors.Is(err, run.ErrRunNotFound) {
				t.Fatalf("deleted run remains accessible: %v", err)
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
