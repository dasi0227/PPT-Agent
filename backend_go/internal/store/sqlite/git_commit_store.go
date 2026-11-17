package sqlite

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/dasi0227/PPT-Agent/backend/internal/gitcommit"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
)

func (s *Store) HasActiveGitCommit(ctx context.Context, projectID string) (bool, error) {
	var n int64
	err := s.reader.WithContext(ctx).Model(&commandExecutionPO{}).Where("project_id = ? AND kind = ? AND status IN ?", projectID, "commit", []string{"accepted", "running", "cancel_requested"}).Count(&n).Error
	return n > 0, err
}
func (s *Store) ListActiveCommitCommands(ctx context.Context) ([]model.CommandExecution, error) {
	var rows []commandExecutionPO
	if err := s.reader.WithContext(ctx).Where(settledHistoryProjects).Where("kind = ? AND status IN ?", "commit", []string{"accepted", "running", "cancel_requested"}).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.CommandExecution, 0, len(rows))
	for _, row := range rows {
		command, err := row.toModel()
		if err != nil {
			return nil, err
		}
		out = append(out, command)
	}
	return out, nil
}

// The operation is durable before Git can change HEAD. It is private recovery
// data on this exact attempt, never inferred from the public timeline.
func (s *Store) PrepareGitCommit(ctx context.Context, execution model.CommandExecution, intent gitcommit.CommitIntent) error {
	if execution.Kind != "commit" || intent.AttemptID != execution.AttemptID || intent.Tree == "" || intent.MessageDigest == "" {
		return sessionstate.ErrInvalidState
	}
	raw, err := json.Marshal(intent)
	if err != nil {
		return err
	}
	hash, err := commandHash(intent)
	if err != nil {
		return err
	}
	_, err = s.commitSession(ctx, sessionstate.Fence{ProjectID: execution.ProjectID, ThreadID: execution.ThreadID, SceneRevision: execution.SceneRevision}, sessionstate.CommitIdentity{Key: "git-intent:" + execution.AttemptID, RequestHash: hash}, func(w *sessionWrite) error {
		var row commandExecutionPO
		if err := w.tx.Where("id = ? AND command_id = ?", execution.AttemptID, execution.CommandID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		if row.Kind != "commit" || row.OwnerInstanceID != execution.OwnerInstanceID || row.ExecutionRevision != execution.ExecutionRevision || row.SceneRevision != execution.SceneRevision {
			return store.ErrCommandConflict
		}
		if row.Status == "cancel_requested" {
			return context.Canceled
		}
		if row.Status != "running" || row.OperationJSON != nil {
			return store.ErrCommandConflict
		}
		if err := w.tx.Model(&row).Update("operation_json", string(raw)).Error; err != nil {
			return err
		}
		current, err := row.toModel()
		if err != nil {
			return err
		}
		return w.writeCommand(row, current)
	})
	if err != nil {
		return err
	}
	// An old receipt acknowledges preparation; it does not grant a revoked
	// worker permission to dispatch Git after recovery or cancellation.
	return s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := readSessionHead(tx, execution.ThreadID)
		if err != nil {
			return err
		}
		if head.SceneRevision != execution.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		var row commandExecutionPO
		if err := tx.Where("id = ?", execution.AttemptID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		if row.OwnerInstanceID != execution.OwnerInstanceID || row.ExecutionRevision != execution.ExecutionRevision {
			return store.ErrCommandConflict
		}
		if row.Status == "cancel_requested" {
			return context.Canceled
		}
		if row.Status != "running" {
			return store.ErrCommandConflict
		}
		return nil
	})
}

// Startup claims the interrupted attempt before reconciling external effects,
// so a result from its previous process cannot overwrite the recovery result.
func (s *Store) ClaimCommitRecovery(ctx context.Context, execution model.CommandExecution) (model.CommandExecution, error) {
	if execution.Kind != "commit" {
		return model.CommandExecution{}, sessionstate.ErrInvalidState
	}
	_, err := s.commitSession(ctx, sessionstate.Fence{ProjectID: execution.ProjectID, ThreadID: execution.ThreadID, SceneRevision: execution.SceneRevision}, sessionstate.CommitIdentity{
		Key:         "recover-git:" + execution.AttemptID + ":" + s.instanceID,
		RequestHash: strconv.FormatInt(execution.ExecutionRevision, 10),
	}, func(w *sessionWrite) error {
		var row commandExecutionPO
		if err := w.tx.Where("id = ? AND command_id = ?", execution.AttemptID, execution.CommandID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		if row.Kind != "commit" || !activeCommandStatus(row.Status) || row.ExecutionRevision != execution.ExecutionRevision || row.OwnerInstanceID != execution.OwnerInstanceID || row.SceneRevision != execution.SceneRevision {
			return store.ErrCommandConflict
		}
		current, err := row.toModel()
		if err != nil {
			return err
		}
		current.ExecutionRevision++
		current.OwnerInstanceID = s.instanceID
		return w.writeCommand(row, current)
	})
	if err != nil {
		return model.CommandExecution{}, err
	}
	var row commandExecutionPO
	if err := s.reader.WithContext(ctx).Where("id = ?", execution.AttemptID).Take(&row).Error; err != nil {
		return model.CommandExecution{}, mapErr(err)
	}
	if row.OwnerInstanceID != s.instanceID {
		return model.CommandExecution{}, store.ErrCommandConflict
	}
	return row.toModel()
}
