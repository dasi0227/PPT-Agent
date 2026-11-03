package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"math"

	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"gorm.io/gorm"
)

const historyReceiptScope = "project_history"

type projectHistoryPO struct {
	ProjectID            string `gorm:"primaryKey"`
	SceneRevision        int64
	HistoryRevision      int64
	SwitchState          string
	SwitchOperationID    *string
	PendingOperationJSON *string
	CheckpointsJSON      string
	LatestSnapshotRef    *string
	LatestTime           *int64
	UISceneJSON          *string
}

func (projectHistoryPO) TableName() string { return "project_history_state" }

func readProjectHistory(tx *gorm.DB, id string) (sessionstate.HistoryControl, error) {
	var out sessionstate.HistoryControl
	if err := rejectDeleted(tx, "project", id); err != nil {
		return out, err
	}
	var row projectHistoryPO
	if err := tx.Where("project_id = ?", id).Take(&row).Error; err != nil {
		return out, mapErr(err)
	}
	out.State = sessionstate.HistoryState{SceneRevision: row.SceneRevision, Revision: row.HistoryRevision, Checkpoints: []sessionstate.ProjectCheckpoint{}, Latest: textOrEmpty(row.LatestSnapshotRef)}
	if row.LatestTime != nil {
		out.State.LatestTime = *row.LatestTime
	}
	if row.UISceneJSON != nil {
		out.State.Scene = json.RawMessage(*row.UISceneJSON)
	}
	if err := json.Unmarshal([]byte(row.CheckpointsJSON), &out.State.Checkpoints); err != nil {
		return out, err
	}
	out.Restoring = row.SwitchState == "restoring"
	if row.PendingOperationJSON != nil {
		if err := json.Unmarshal([]byte(*row.PendingOperationJSON), &out.Pending); err != nil {
			return out, err
		}
		if out.Pending == nil || out.Pending.ID != textOrEmpty(row.SwitchOperationID) {
			return out, sessionstate.ErrInvalidState
		}
	} else if out.Restoring || row.SwitchOperationID != nil {
		return out, sessionstate.ErrInvalidState
	}
	return out, nil
}

func (s *Store) ReadProjectHistory(ctx context.Context, id string) (sessionstate.HistoryControl, error) {
	return readProjectHistory(s.reader.WithContext(ctx), id)
}

func historyUpdates(state sessionstate.HistoryState) (map[string]any, error) {
	if state.Revision < 1 || state.SceneRevision < 1 || state.Checkpoints == nil || (len(state.Scene) > 0 && !json.Valid(state.Scene)) {
		return nil, sessionstate.ErrInvalidState
	}
	raw, err := json.Marshal(state.Checkpoints)
	if err != nil {
		return nil, err
	}
	return map[string]any{"scene_revision": state.SceneRevision, "history_revision": state.Revision,
		"checkpoints_json": string(raw), "latest_snapshot_ref": nullableID(state.Latest),
		"latest_time": state.LatestTime, "ui_scene_json": jsonColumn(state.Scene)}, nil
}

func (s *Store) SaveProjectHistory(ctx context.Context, id string, expectedScene, expectedRevision int64, next sessionstate.HistoryState) (sessionstate.HistoryState, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := readProjectHistory(tx, id)
		if err != nil {
			return err
		}
		if current.Restoring {
			return sessionstate.ErrProjectRestoring
		}
		if current.State.SceneRevision != expectedScene {
			return sessionstate.ErrSceneChanged
		}
		if current.State.Revision != expectedRevision || expectedRevision == math.MaxInt64 {
			return sessionstate.ErrVersionConflict
		}
		next.SceneRevision, next.Revision = expectedScene, expectedRevision+1
		updates, err := historyUpdates(next)
		if err != nil {
			return err
		}
		return tx.Table("project_history_state").Where("project_id = ?", id).Updates(updates).Error
	})
	return next, err
}

func readHistoryReceipt(tx *gorm.DB, id, key, hash string) (*sessionstate.HistoryState, error) {
	var row idempotencyPO
	err := tx.Where("scope = ? AND owner_id = ? AND key = ?", historyReceiptScope, id, key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.RequestHash != hash {
		return nil, sessionstate.ErrIdempotencyConflict
	}
	if row.Status != "completed" {
		return nil, sessionstate.ErrVersionConflict
	}
	var state sessionstate.HistoryState
	if err := json.Unmarshal([]byte(row.ResultJSON), &state); err != nil {
		return nil, err
	}
	return &state, nil
}
func (s *Store) ProjectHistoryReceipt(ctx context.Context, id, key, hash string) (*sessionstate.HistoryState, error) {
	if err := rejectDeleted(s.reader.WithContext(ctx), "project", id); err != nil {
		return nil, err
	}
	return readHistoryReceipt(s.reader.WithContext(ctx), id, key, hash)
}

func projectThreadIDs(tx *gorm.DB, id string) ([]string, error) {
	var ids []string
	err := tx.Table("threads").Where("project_id = ?", id).Pluck("id", &ids).Error
	return ids, err
}
func (s *Store) notifySessions(ids []string) {
	for _, id := range ids {
		s.notifySession(id)
	}
}

func (s *Store) BeginHistoryOperation(ctx context.Context, id string, operation sessionstate.HistoryOperation) error {
	if operation.ID == "" || operation.RequestHash == "" || len(operation.Before) != 64 || (operation.Kind != "switch" && operation.Kind != "mutation") {
		return sessionstate.ErrInvalidState
	}
	var ids []string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := readProjectHistory(tx, id)
		if err != nil {
			return err
		}
		if current.Pending != nil {
			if current.Pending.ID == operation.ID && current.Pending.RequestHash == operation.RequestHash {
				return nil
			}
			return sessionstate.ErrProjectRestoring
		}
		if receipt, err := readHistoryReceipt(tx, id, operation.ID, operation.RequestHash); err != nil || receipt != nil {
			if err != nil {
				return err
			}
			return sessionstate.ErrVersionConflict
		}
		if current.State.SceneRevision != operation.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		if current.State.Revision != operation.HistoryRevision {
			return sessionstate.ErrVersionConflict
		}
		if err := rejectPendingDeletion(tx, id); err != nil {
			return err
		}
		var count int64
		if err := tx.Table("runs").Where("project_id = ? AND status NOT IN ?", id, []string{"done", "failed", "canceled"}).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return sessionstate.ErrVersionConflict
		}
		if err := tx.Table("command_executions").Where("project_id = ? AND status IN ? AND NOT (kind = 'rename' AND source = 'automatic')", id, []string{"accepted", "running", "cancel_requested"}).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return sessionstate.ErrVersionConflict
		}
		raw, err := json.Marshal(operation)
		if err != nil {
			return err
		}
		status := "ready"
		if operation.Kind == "switch" {
			status = "restoring"
		}
		if err := tx.Table("project_history_state").Where("project_id = ?", id).Updates(map[string]any{"switch_state": status, "switch_operation_id": operation.ID, "pending_operation_json": string(raw)}).Error; err != nil {
			return err
		}
		ids, err = projectThreadIDs(tx, id)
		return err
	})
	if err == nil && operation.Kind == "switch" {
		s.notifySessions(ids)
	}
	return err
}

// A failed/unfinished mutation must stop serving the scene before its files are
// rolled back. The preimage record remains pinned until the restore commits.
func (s *Store) IsolateHistoryOperation(ctx context.Context, id, operationID string) error {
	var ids []string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := readProjectHistory(tx, id)
		if err != nil {
			return err
		}
		if current.Pending == nil || current.Pending.ID != operationID {
			return sessionstate.ErrVersionConflict
		}
		if err := tx.Table("project_history_state").Where("project_id = ?", id).Update("switch_state", "restoring").Error; err != nil {
			return err
		}
		ids, err = projectThreadIDs(tx, id)
		return err
	})
	if err == nil {
		s.notifySessions(ids)
	}
	return err
}

func writeHistoryReceipt(tx *gorm.DB, id string, operation sessionstate.HistoryOperation, state sessionstate.HistoryState, success bool) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	status := "failed"
	if success {
		status = "completed"
	}
	return tx.Create(&idempotencyPO{Scope: historyReceiptScope, OwnerID: id, Key: operation.ID, RequestHash: operation.RequestHash, Status: status, ResultJSON: string(raw)}).Error
}

func (s *Store) FinishHistoryMutation(ctx context.Context, id, operationID string) (sessionstate.HistoryState, error) {
	var next sessionstate.HistoryState
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := readProjectHistory(tx, id)
		if err != nil {
			return err
		}
		if current.Restoring || current.Pending == nil || current.Pending.ID != operationID || current.Pending.Kind != "mutation" {
			return sessionstate.ErrVersionConflict
		}
		if current.State.Revision == math.MaxInt64 {
			return sessionstate.ErrVersionConflict
		}
		next = current.State
		next.Revision++
		next.Latest, next.LatestTime = "", 0
		updates, err := historyUpdates(next)
		if err != nil {
			return err
		}
		updates["switch_state"], updates["switch_operation_id"], updates["pending_operation_json"] = "ready", nil, nil
		updates["last_operation_id"], updates["last_command"] = operationID, "mutation"
		if err := tx.Table("project_history_state").Where("project_id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		return writeHistoryReceipt(tx, id, *current.Pending, next, true)
	})
	return next, err
}

func (s *Store) PendingHistoryProjects(ctx context.Context) ([]string, error) {
	var ids []string
	err := s.reader.WithContext(ctx).Table("project_history_state").Where("pending_operation_json IS NOT NULL").Pluck("project_id", &ids).Error
	return ids, err
}
