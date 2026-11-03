package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

func TestContinuationClaimsOriginalRunOnce(t *testing.T) {
	for _, status := range []model.RunStatus{model.RunFailed, model.RunCanceled, model.RunPaused} {
		t.Run(string(status), func(t *testing.T) {
			s, cp := checkpointFixture(t)
			ctx := context.Background()
			cp.Boundary, cp.ContinuationAllowed = "terminal", true
			if status == model.RunPaused {
				cp.Boundary, cp.ContinuationAllowed = "before_model", false
			}
			if err := s.SaveCheckpoint(ctx, cp); err != nil {
				t.Fatal(err)
			}
			if status == model.RunPaused {
				if _, err := s.PauseRun(ctx, cp.RunID, cp.OwnerInstanceID, "restart", 1); err != nil {
					t.Fatal(err)
				}
			} else if err := s.SetRunStatus(ctx, cp.RunID, status); err != nil {
				t.Fatal(err)
			}
			original, err := s.GetRun(ctx, cp.RunID)
			if err != nil || !original.CanContinue {
				t.Fatalf("not continuable: %+v %v", original, err)
			}
			action := currentRunAction(t, s, cp.RunID, "resume")
			if err := s.db.Exec("CREATE TRIGGER fail_resume_batch BEFORE INSERT ON thread_changes BEGIN SELECT RAISE(ABORT,'injected resume failure'); END").Error; err != nil {
				t.Fatal(err)
			}
			if _, _, err := s.ClaimRunForResume(ctx, original, "next-owner", action); err == nil {
				t.Fatal("claimed without batch")
			}
			unchanged, err := s.GetRun(ctx, cp.RunID)
			if err != nil || unchanged.ExecutionRevision != original.ExecutionRevision || unchanged.Status != original.Status {
				t.Fatalf("partial claim: %+v %v", unchanged, err)
			}
			if err := s.db.Exec("DROP TRIGGER fail_resume_batch").Error; err != nil {
				t.Fatal(err)
			}
			resumed, claimed, err := s.ClaimRunForResume(ctx, original, "next-owner", action)
			if err != nil || !claimed || resumed.Current.ID != original.ID || resumed.Current.Status != model.RunRecovering || resumed.Current.ExecutionRevision != original.ExecutionRevision+1 {
				t.Fatalf("claim: %+v %v %v", resumed, claimed, err)
			}
			repeated, claimedAgain, err := s.ClaimRunForResume(ctx, original, "duplicate-owner", action)
			if err != nil || claimedAgain || repeated.Receipt.CommitID != resumed.Receipt.CommitID || repeated.Current.OwnerInstanceID != "next-owner" {
				t.Fatalf("replay reclaimed worker: %+v %v %v", repeated, claimedAgain, err)
			}
			if _, found, err := s.RunResumeReceipt(ctx, cp.RunID, action); err != nil || !found {
				t.Fatalf("receipt lookup: %v %v", found, err)
			}
			changed := action
			changed.EntityRevision++
			if _, _, err := s.RunResumeReceipt(ctx, cp.RunID, changed); !errors.Is(err, sessionstate.ErrIdempotencyConflict) {
				t.Fatalf("key reused with another version: %v", err)
			}
			stale := action
			stale.RequestKey = "another-resume"
			if _, _, err := s.ClaimRunForResume(ctx, original, "duplicate-owner", stale); !errors.Is(err, sessionstate.ErrVersionConflict) {
				t.Fatalf("stale new action: %v", err)
			}
			if err := s.SetRunStatus(ctx, cp.RunID, model.RunFailed); err != nil {
				t.Fatal(err)
			}
			failed, err := s.GetRun(ctx, cp.RunID)
			if err != nil || failed.CanContinue {
				t.Fatalf("old terminal checkpoint reused: %+v %v", failed, err)
			}
		})
	}
}

func TestContinuationRequiresRecoveryPointAndHonorsStop(t *testing.T) {
	for _, cancelPending := range []bool{false, true} {
		s, cp := checkpointFixture(t)
		ctx := context.Background()
		if cancelPending {
			if err := s.SaveCheckpoint(ctx, cp); err != nil {
				t.Fatal(err)
			}
			if _, err := s.RequestRunCancel(ctx, cp.RunID, model.RunCancelUserRequested, currentRunAction(t, s, cp.RunID, "cancel")); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.PauseRun(ctx, cp.RunID, cp.OwnerInstanceID, "restart", 1); err != nil {
			t.Fatal(err)
		}
		if _, err := resumeRunForTest(t, s, cp.RunID, "owner2", "resume"); !errors.Is(err, run.ErrRunNotContinuable) {
			t.Fatalf("unsafe resume (cancel=%v): %v", cancelPending, err)
		}
	}
}
