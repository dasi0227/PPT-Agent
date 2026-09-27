package sqlite

import (
	"context"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"gorm.io/gorm"
)

// Eligibility is re-evaluated against current project history, never trusted from the UI.
func canContinueRun(tx *gorm.DB, row runPO) (bool, error) {
	if row.Status != string(model.RunFailed) && row.Status != string(model.RunCanceled) {
		return false, nil
	}
	var matching int64
	if err := tx.Table("runs").Where("id = ? AND json_extract(checkpoint_json, '$.execution_revision') = execution_revision", row.ID).Count(&matching).Error; err != nil {
		return false, err
	}
	if matching != 1 {
		return false, nil
	}
	cp, err := readCheckpoint(tx, row.ID)
	if errors.Is(err, run.ErrRunNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if cp.Plan != nil && cp.Plan.Status == workflow.PlanCanceled {
		return false, nil
	}
	if !cp.ContinuationAllowed || cp.Boundary != "terminal" || cp.Phase == workflow.PhaseTerminal || cp.Scope.Validate() != nil {
		return false, nil
	}
	var count int64
	// rowid orders accepted runs even when their second-resolution timestamps tie.
	err = tx.Table("runs").Where("project_id = ? AND (rowid > (SELECT rowid FROM runs WHERE id = ?) OR (id <> ? AND status IN ?))", row.ProjectID, row.ID, row.ID, []string{"pending", "running", "waiting", "paused", "recovering"}).Count(&count).Error
	return count == 0, err
}

func (s *Store) ClaimContinuableRun(ctx context.Context, expected model.Run, owner string) (model.Run, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row runPO
		if err := tx.First(&row, "id = ?", expected.ID).Error; err != nil {
			return mapErr(err)
		}
		eligible, err := canContinueRun(tx, row)
		if err != nil {
			return err
		}
		if !eligible || row.ExecutionRevision != expected.ExecutionRevision || row.CheckpointRevision != expected.CheckpointRevision {
			return run.ErrRunNotContinuable
		}
		result := tx.Model(&runPO{}).Where("id = ? AND status = ? AND execution_revision = ? AND checkpoint_revision = ?", row.ID, row.Status, row.ExecutionRevision, row.CheckpointRevision).Updates(map[string]any{
			"status": string(model.RunRecovering), "owner_instance_id": owner, "execution_revision": gorm.Expr("execution_revision + 1"),
			"cancel_requested_at": nil, "pause_reason": "", "paused_at": nil, "updated_at": nowUnix(),
		})
		if result.Error != nil {
			return mapProjectWriteErr(result.Error)
		}
		if result.RowsAffected != 1 {
			return run.ErrRunNotContinuable
		}
		return interruptRunIdempotency(tx, []string{row.ID}, nowUnix())
	})
	if err != nil {
		return model.Run{}, err
	}
	return s.GetRun(ctx, expected.ID)
}
