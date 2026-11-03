package sqlite

import (
	"context"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"gorm.io/gorm"
)

// All lifecycle writes are current-state changes with a public commit. Expected
// scene and execution are captured before entering the write transaction.
func (s *Store) mutateRun(ctx context.Context, id, operation string, apply func(*sessionWrite, runPO) error) error {
	var expected runPO
	if err := s.reader.WithContext(ctx).Where("id = ?", id).Take(&expected).Error; err != nil {
		return mapErr(err)
	}
	if scene, ok := sessionstate.RequestScene(ctx); ok && scene != expected.SceneRevision {
		return sessionstate.ErrSceneChanged
	}
	key := fmt.Sprintf("run-state:%s:%d:%d:%s", id, expected.ExecutionRevision, expected.EntityRevision, operation)
	_, err := s.commitSession(ctx, sessionstate.Fence{ProjectID: expected.ProjectID, ThreadID: expected.ThreadID, SceneRevision: expected.SceneRevision}, sessionstate.CommitIdentity{Key: key, RequestHash: operation}, func(w *sessionWrite) error {
		var current runPO
		if err := w.tx.Where("id = ? AND thread_id = ?", id, w.head.ID).Take(&current).Error; err != nil {
			return mapErr(err)
		}
		if current.ExecutionRevision != expected.ExecutionRevision || current.SceneRevision != expected.SceneRevision {
			return run.ErrRunRevisionConflict
		}
		if owner, execution, ok := workflow.CheckpointOwnership(ctx); ok {
			sameTerminal := model.RunStatus(current.Status).Terminal() && current.OwnerInstanceID == ""
			if execution != current.ExecutionRevision || (owner != current.OwnerInstanceID && !sameTerminal) {
				return run.ErrRunRevisionConflict
			}
		}
		if err := apply(w, current); err != nil {
			return err
		}
		return w.publishRun(id)
	})
	return err
}

func (w *sessionWrite) setRunStatus(row runPO, status model.RunStatus) error {
	if model.RunStatus(row.Status).Terminal() {
		if row.Status == string(status) {
			return nil
		}
		return run.ErrRunNotRunning
	}
	updates := map[string]any{"status": string(status), "updated_at": w.now}
	if status == model.RunRunning && row.Status != string(model.RunRunning) {
		updates["active_since"] = w.now
	}
	if status.Terminal() {
		updates["owner_instance_id"] = ""
		updates["active_since"] = nil
		if err := w.cancelRunInteractions(row.ID); err != nil {
			return err
		}
		if err := w.closeConversationRound(row.ID); err != nil {
			return err
		}
	}
	if status == model.RunRunning || status == model.RunWaiting {
		updates["pause_reason"], updates["paused_at"] = "", nil
	}
	return w.tx.Table("runs").Where("id = ?", row.ID).Updates(updates).Error
}

func (s *Store) SetRunStatus(ctx context.Context, id string, status model.RunStatus) error {
	return s.mutateRun(ctx, id, "status:"+string(status), func(w *sessionWrite, row runPO) error { return w.setRunStatus(row, status) })
}

func (w *sessionWrite) pauseRun(row runPO, reason string, pausedAt int64) error {
	if model.RunStatus(row.Status).Terminal() {
		return nil
	}
	// Never turn a dispatched side effect into a retryable failure. Its original
	// operation identity survives until the recovery policy proves the outcome.
	if err := w.tx.Table("tool_executions").Where("run_id = ? AND status = 'executing'", row.ID).
		Updates(map[string]any{"status": "outcome_unknown", "entity_revision": gorm.Expr("entity_revision + 1"), "updated_seq": w.seq}).Error; err != nil {
		return err
	}
	return w.tx.Table("runs").Where("id = ?", row.ID).Updates(map[string]any{
		"checkpoint_json": gorm.Expr("CASE WHEN json_extract(checkpoint_json, '$.pending_model_call.stage') = 'dispatched' THEN json_set(checkpoint_json, '$.pending_model_call.stage', 'outcome_unknown') ELSE checkpoint_json END"),
		"active_since":    nil,
		"status":          string(model.RunPaused), "owner_instance_id": "", "pause_reason": reason, "paused_at": pausedAt, "updated_at": w.now,
	}).Error
}

func (s *Store) PauseNonTerminalRuns(ctx context.Context, reason string, pausedAt int64) ([]model.Run, error) {
	var rows []runPO
	if err := s.reader.WithContext(ctx).Where(settledHistoryProjects).Where("status IN ?", []string{"pending", "running", "waiting", "recovering"}).Order("created_at,id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.Run, 0, len(rows))
	for _, row := range rows {
		current, err := s.PauseRun(sessionstate.WithRequestScene(ctx, row.SceneRevision), row.ID, row.OwnerInstanceID, reason, pausedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, current)
	}
	return out, nil
}

func (s *Store) PauseRun(ctx context.Context, id, owner, reason string, pausedAt int64) (model.Run, error) {
	err := s.mutateRun(ctx, id, "pause:"+owner+":"+reason, func(w *sessionWrite, row runPO) error {
		if model.RunStatus(row.Status).Terminal() {
			return nil
		}
		if owner != "" && row.OwnerInstanceID != owner {
			return run.ErrRunRevisionConflict
		}
		return w.pauseRun(row, reason, pausedAt)
	})
	if err != nil {
		return model.Run{}, err
	}
	return s.GetRun(ctx, id)
}
