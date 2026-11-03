package sqlite

import (
	"context"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Store) RecordThreadNamingInput(ctx context.Context, input model.ThreadNamingInput) (model.Thread, bool, error) {
	thread, err := s.GetThread(ctx, input.ThreadID)
	if err != nil {
		return model.Thread{}, false, err
	}
	shouldTrigger := false
	_, err = s.commitSession(ctx, sessionstate.Fence{ProjectID: thread.ProjectID, ThreadID: input.ThreadID, SceneRevision: input.SceneRevision},
		sessionstate.CommitIdentity{Key: fmt.Sprintf("naming-input:%s:%s:%d", input.RunID, input.InputID, input.ExecutionRevision), RequestHash: input.InputID}, func(w *sessionWrite) error {
			tx := w.tx
			var current runPO
			if err := tx.First(&current, "id = ? AND thread_id = ?", input.RunID, input.ThreadID).Error; err != nil {
				return mapErr(err)
			}
			if current.ExecutionRevision != input.ExecutionRevision || current.SceneRevision != input.SceneRevision {
				return run.ErrRunRevisionConflict
			}
			accepted, err := findRunInput(tx, input.ThreadID, input.RunID, input.InputID)
			if err != nil {
				return err
			}
			if accepted.Disposition != model.InputTask {
				return nil
			}
			key := accepted.ID
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&idempotencyPO{Scope: "naming_input", OwnerID: input.ThreadID, Key: key, RequestHash: key, Status: "completed", ResultJSON: "{}"})
			if result.Error != nil {
				return result.Error
			}
			var po threadPO
			if err := tx.First(&po, "id = ?", input.ThreadID).Error; err != nil {
				return mapErr(err)
			}
			if result.RowsAffected == 0 {
				thread = po.toModel()
				return nil
			}
			updates := map[string]any{"rename_first_input_seen": 1}
			if po.AutoRenameEnabled != 0 {
				if po.RenameFirstInputSeen == 0 {
					shouldTrigger = true
				} else {
					po.RenameInputCount++
					updates["rename_input_count"] = po.RenameInputCount
					shouldTrigger = true
				}
			}
			if err := tx.Model(&threadPO{}).Where("id = ?", input.ThreadID).Updates(updates).Error; err != nil {
				return err
			}
			if err := tx.First(&po, "id = ?", input.ThreadID).Error; err != nil {
				return mapErr(err)
			}
			thread = po.toModel()
			return w.publishThread()
		})
	if err != nil {
		return model.Thread{}, false, err
	}
	thread, err = s.GetThread(ctx, input.ThreadID)
	return thread, shouldTrigger, err
}

func (s *Store) BeginThreadRenameRequest(ctx context.Context, id string, expectedOperationVersion int64, resetInputCount bool, updatedAt int64) (model.Thread, error) {
	var out model.Thread
	head, err := readSessionHead(s.reader.WithContext(ctx), id)
	if err != nil {
		return out, err
	}
	scene := head.SceneRevision
	if requested, ok := sessionstate.RequestScene(ctx); ok {
		scene = requested
	}
	_, err = s.commitSession(ctx, sessionstate.Fence{ProjectID: head.ProjectID, ThreadID: id, SceneRevision: scene}, sessionstate.CommitIdentity{Key: model.MustShortID("naming"), RequestHash: "naming-state"}, func(w *sessionWrite) error {
		tx := w.tx
		var po threadPO
		if err := tx.First(&po, "id = ?", id).Error; err != nil {
			return mapErr(err)
		}
		if po.AutoRenameEnabled == 0 || po.RenameFirstInputSeen == 0 {
			return run.ErrRunNotRunning
		}
		if expectedOperationVersion > 0 && po.RenameOperationVersion != expectedOperationVersion {
			return store.ErrNamingOperationConflict
		}
		updates := map[string]any{
			"rename_operation_version": gorm.Expr("rename_operation_version + 1"),
			"updated_at":               w.now,
		}
		if resetInputCount {
			updates["rename_input_count"] = 0
		}
		if err := tx.Model(&threadPO{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.First(&po, "id = ?", id).Error; err != nil {
			return mapErr(err)
		}
		out = po.toModel()
		return w.publishThread()
	})
	return out, err
}

func (w *sessionWrite) completeRename(effect sessionstate.RenameEffect) (bool, error) {
	var po threadPO
	if err := w.tx.Where("id = ?", w.head.ID).Take(&po).Error; err != nil {
		return false, mapErr(err)
	}
	if effect.OperationVersion != po.RenameOperationVersion || (effect.Automatic && po.AutoRenameEnabled == 0) {
		return false, nil
	}
	updates := map[string]any{"updated_at": w.now}
	changed := po.Title != effect.Title
	updates["title"] = effect.Title
	if effect.Enabled != nil {
		updates["auto_rename_enabled"] = boolInt(*effect.Enabled)
		updates["rename_input_count"] = 0
		changed = changed || po.AutoRenameEnabled != boolInt(*effect.Enabled)
	}
	if changed {
		updates["naming_revision"] = gorm.Expr("naming_revision + 1")
	}
	if err := w.tx.Table("threads").Where("id = ?", w.head.ID).Updates(updates).Error; err != nil {
		return false, err
	}
	return true, w.publishThread()
}

func (s *Store) UpdateThreadNamingState(ctx context.Context, id string, title *string, enabled *bool, resetInputCount bool, updatedAt int64) (model.Thread, error) {
	var out model.Thread
	head, err := readSessionHead(s.reader.WithContext(ctx), id)
	if err != nil {
		return out, err
	}
	scene := head.SceneRevision
	if requested, ok := sessionstate.RequestScene(ctx); ok {
		scene = requested
	}
	_, err = s.commitSession(ctx, sessionstate.Fence{ProjectID: head.ProjectID, ThreadID: id, SceneRevision: scene}, sessionstate.CommitIdentity{Key: model.MustShortID("naming"), RequestHash: "naming-state"}, func(w *sessionWrite) error {
		tx := w.tx
		var po threadPO
		if err := tx.First(&po, "id = ?", id).Error; err != nil {
			return mapErr(err)
		}
		updates := map[string]any{
			"rename_operation_version": gorm.Expr("rename_operation_version + 1"),
			"updated_at":               w.now,
		}
		publicChanged := false
		if title != nil && po.Title != *title {
			updates["title"] = *title
			publicChanged = true
		}
		if enabled != nil {
			value := 0
			if *enabled {
				value = 1
			}
			if po.AutoRenameEnabled != value {
				updates["auto_rename_enabled"] = value
				publicChanged = true
			}
			if resetInputCount {
				updates["rename_input_count"] = 0
			}
		}
		if publicChanged {
			updates["naming_revision"] = gorm.Expr("naming_revision + 1")
		}
		if err := tx.Model(&threadPO{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.First(&po, "id = ?", id).Error; err != nil {
			return mapErr(err)
		}
		out = po.toModel()
		return w.publishThread()
	})
	return out, err
}
