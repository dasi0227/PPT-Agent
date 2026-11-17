package sqlite

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"gorm.io/gorm"
)

// Continuation eligibility is materialized with the checkpoint. Creating a new
// project Run invalidates the prior flags, so list reads need no old checkpoints.
func canContinueRun(tx *gorm.DB, row runPO) (bool, error) {
	if !row.continuable() {
		return false, nil
	}
	stopped, err := inputStopExists(tx, row.ID)
	if err != nil || stopped {
		return false, err
	}
	var count int64
	err = tx.Table("runs").Where("project_id = ? AND id <> ? AND status NOT IN ?", row.ProjectID, row.ID, []string{"done", "failed", "canceled"}).Count(&count).Error
	return count == 0, err
}

func (row runPO) continuable() bool {
	if row.Status == string(model.RunPaused) {
		return row.CancelRequestedAt == nil && row.CheckpointRevision > 0 && row.CheckpointPhase != string(workflow.PhaseTerminal) && row.Command.Scope.Validate() == nil
	}
	return (row.Status == string(model.RunFailed) || row.Status == string(model.RunCanceled)) && row.ContinuationAllowed != 0 &&
		row.CheckpointExecutionRevision == row.ExecutionRevision && row.CheckpointBoundary == "terminal" && row.CheckpointPhase != string(workflow.PhaseTerminal) && row.Command.Scope.Validate() == nil
}
