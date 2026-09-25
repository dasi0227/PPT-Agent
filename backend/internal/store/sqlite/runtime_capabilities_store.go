package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"gorm.io/gorm"
)

func readCheckpoint(tx *gorm.DB, runID string) (workflow.RuntimeCheckpoint, error) {
	var row struct {
		CheckpointJSON     *string
		CheckpointRevision int64
		OwnerInstanceID    string
		ExecutionRevision  int64
	}
	if err := tx.Table("runs").Select("checkpoint_json,checkpoint_revision,owner_instance_id,execution_revision").Where("id = ?", runID).Take(&row).Error; err != nil {
		return workflow.RuntimeCheckpoint{}, mapErr(err)
	}
	if row.CheckpointJSON == nil {
		return workflow.RuntimeCheckpoint{}, run.ErrRunNotFound
	}
	var cp workflow.RuntimeCheckpoint
	if err := json.Unmarshal([]byte(*row.CheckpointJSON), &cp); err != nil {
		return cp, err
	}
	cp.CheckpointRevision = row.CheckpointRevision
	cp.OwnerInstanceID = row.OwnerInstanceID
	cp.ExecutionRevision = row.ExecutionRevision
	return cp, nil
}

// writeCheckpoint requires the worker's expected ownership and versions. Scope
// changes and the new checkpoint share this single conditional write.
func writeCheckpoint(tx *gorm.DB, cp workflow.RuntimeCheckpoint, expectedScope int64, updates map[string]any) error {
	if cp.RunID == "" || cp.LoopID == "" || cp.ExecutionRevision < 1 {
		return errors.New("checkpoint requires execution identity")
	}
	if updates == nil {
		updates = map[string]any{}
	}
	cp.CreatedAt = time.Now().UnixMilli()
	cp.CheckpointRevision++
	// Artifact recovery belongs to the mutation journal, indexes are rebuilt.
	raw, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	updates["checkpoint_json"] = string(raw)
	updates["checkpoint_revision"] = cp.CheckpointRevision
	updates["updated_at"] = nowUnix()
	result := tx.Table("runs").Where("id = ? AND owner_instance_id = ? AND execution_revision = ? AND scope_revision = ? AND checkpoint_revision = ? AND status IN ?", cp.RunID, cp.OwnerInstanceID, cp.ExecutionRevision, expectedScope, cp.CheckpointRevision-1, []string{"pending", "running", "waiting", "recovering"}).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return run.ErrRunRevisionConflict
	}
	return nil
}
func (s *Store) SaveCheckpoint(ctx context.Context, cp workflow.RuntimeCheckpoint) error {
	return workflow.WithCheckpointWrite(ctx, cp, func(cp workflow.RuntimeCheckpoint) error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return writeCheckpoint(tx, cp, cp.Scope.Revision, nil) })
	})
}
func (s *Store) LatestCheckpoint(ctx context.Context, runID string) (workflow.RuntimeCheckpoint, error) {
	return readCheckpoint(s.db.WithContext(ctx), runID)
}

func (s *Store) CommitPlanApproval(ctx context.Context, runID string, mode model.RunMode, cp workflow.RuntimeCheckpoint) error {
	if runID == "" || mode != model.ModeExecute || cp.RunID != runID || cp.Mode != mode {
		return errors.New("invalid plan approval transition")
	}
	return workflow.WithCheckpointWrite(ctx, cp, func(cp workflow.RuntimeCheckpoint) error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var row struct{ Mode string }
			if err := tx.Table("runs").Select("mode").Where("id = ?", runID).Take(&row).Error; err != nil {
				return mapErr(err)
			}
			if row.Mode != string(model.ModePlan) {
				return run.ErrRunRevisionConflict
			}
			return writeCheckpoint(tx, cp, cp.Scope.Revision, map[string]any{"mode": string(mode), "status": string(model.RunRunning)})
		})
	})
}
func (s *Store) CommitScopeExpansion(ctx context.Context, runID string, scope model.RunScope, cp workflow.RuntimeCheckpoint) error {
	if runID == "" || cp.RunID != runID || cp.Scope.Revision != scope.Revision {
		return errors.New("invalid scope expansion transition")
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(scope)
	if err != nil {
		return err
	}
	return workflow.WithCheckpointWrite(ctx, cp, func(cp workflow.RuntimeCheckpoint) error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return writeCheckpoint(tx, cp, scope.Revision-1, map[string]any{"scope_json": string(raw), "scope_revision": scope.Revision, "status": string(model.RunRunning)})
		})
	})
}
func (s *Store) SaveSemanticReview(ctx context.Context, review workflow.StoredSemanticReview) error {
	var row struct{ ThreadID string }
	if err := s.db.WithContext(ctx).Table("runs").Select("thread_id").Where("id = ?", review.RunID).Take(&row).Error; err != nil {
		return mapErr(err)
	}
	raw, err := json.Marshal(review)
	if err != nil {
		return err
	}
	_, err = s.AppendThreadEvent(ctx, row.ThreadID, threadjournal.Event{Type: "diagnostic.semantic_review", RunID: review.RunID, Payload: raw})
	return err
}
