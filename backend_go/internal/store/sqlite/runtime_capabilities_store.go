package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
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
	if err := hydrateCheckpointModelCall(tx, &cp); err != nil {
		return cp, err
	}
	if err := hydrateCheckpointTools(tx, &cp); err != nil {
		return cp, err
	}
	if err := hydrateCheckpointInteractions(tx, &cp); err != nil {
		return cp, err
	}
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
	if cp.CreatedAt <= 0 {
		cp.CreatedAt = time.Now().UnixMilli()
	}
	cp.CheckpointRevision++
	// Artifact recovery belongs to the mutation journal, indexes are rebuilt.
	raw, err := json.Marshal(checkpointReferences(cp))
	if err != nil {
		return err
	}
	updates["checkpoint_json"] = string(raw)
	updates["active_duration_ms"] = cp.ActiveDurationMS
	updates["active_since"] = nil
	if cp.ActiveClockRunning && cp.Phase != workflow.PhaseTerminal {
		updates["active_since"] = cp.CreatedAt
	}
	updates["continuation_allowed"] = boolInt(cp.ContinuationAllowed)
	// Page creation can extend the effective IDs without a new approval revision.
	// Keep the run projection and its checkpoint in the same atomic write.
	scopeRaw, err := json.Marshal(cp.Scope)
	if err != nil {
		return err
	}
	updates["scope_json"] = string(scopeRaw)
	updates["checkpoint_revision"] = cp.CheckpointRevision
	updates["checkpoint_execution_revision"] = cp.ExecutionRevision
	updates["checkpoint_boundary"] = cp.Boundary
	updates["checkpoint_phase"] = string(cp.Phase)
	updates["updated_at"] = time.Now().UnixMilli()
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
		var row struct {
			ThreadID  string
			ProjectID string
		}
		if err := s.reader.WithContext(ctx).Table("runs").Select("thread_id,project_id").Where("id = ?", cp.RunID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		var candidate *model.ResourceEditCandidate
		if cp.PendingResourceApproval != nil {
			candidate = cp.PendingResourceApproval.Candidate
		}
		hash, err := commandHash(struct {
			Checkpoint        workflow.RuntimeCheckpoint
			Update            *sessionstate.ContextUpdate
			ScopeTransition   bool
			ModeTransition    bool
			Interactions      []model.InteractionApplication
			ResourceCandidate *model.ResourceEditCandidate
			PublicUpdates     []sessionstate.RunUpdate
			ModelRequest      *sessionstate.ModelRequestSnapshot
			ModelResponse     json.RawMessage
			Tools             []sessionstate.ToolExecution
		}{cp, cp.ContextUpdate, cp.ScopeTransition, cp.ModeTransition, cp.InteractionApplications, candidate, cp.PublicUpdates, cp.NewModelRequest, cp.ModelResponseJSON, cp.ToolChanges})
		if err != nil {
			return err
		}
		receipt, err := s.CommitRuntimeState(ctx,
			sessionstate.Fence{ProjectID: row.ProjectID, ThreadID: row.ThreadID, SceneRevision: cp.SceneRevision, RunID: cp.RunID, OwnerInstanceID: cp.OwnerInstanceID, ExecutionRevision: cp.ExecutionRevision},
			sessionstate.CommitIdentity{Key: "checkpoint:" + cp.RunID + ":" + fmt.Sprint(cp.ExecutionRevision) + ":" + fmt.Sprint(cp.CheckpointRevision), RequestHash: hash}, cp.ContextUpdate, cp)
		if err == nil && cp.CommitReceipt != nil {
			*cp.CommitReceipt = receipt
		}
		return err
	})
}
func (s *Store) LatestCheckpoint(ctx context.Context, runID string) (workflow.RuntimeCheckpoint, error) {
	var cp workflow.RuntimeCheckpoint
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct{ ThreadID string }
		if err := tx.Table("runs").Select("thread_id").Where("id = ?", runID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		head, err := readSessionHead(tx, row.ThreadID)
		if err != nil {
			return err
		}
		if scene, ok := sessionstate.RequestScene(ctx); ok && scene != head.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		cp, err = readCheckpoint(tx, runID)
		return err
	})
	return cp, err
}

func (s *Store) CommitPlanApproval(ctx context.Context, runID string, mode model.RunMode, cp workflow.RuntimeCheckpoint) error {
	if runID == "" || mode != model.ModeCreate || cp.RunID != runID || cp.Mode != mode {
		return errors.New("invalid plan approval transition")
	}
	cp.ModeTransition = true
	return s.SaveCheckpoint(ctx, cp)
}
func (s *Store) CommitScopeExpansion(ctx context.Context, runID string, scope model.RunScope, cp workflow.RuntimeCheckpoint) error {
	if runID == "" || cp.RunID != runID || !cp.Scope.Equal(scope) {
		return errors.New("invalid scope expansion transition")
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	cp.ScopeTransition = true
	return s.SaveCheckpoint(ctx, cp)
}
