package sqlite

import (
	"context"
	"errors"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
)

func (s *Store) UpdateThreadSuggestState(ctx context.Context, id string, enabled bool) (model.Thread, error) {
	var row threadPO
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&row, "id = ?", id).Error; err != nil {
			return mapErr(err)
		}
		if row.AutoSuggestEnabled == boolInt(enabled) {
			return nil
		}
		row.AutoSuggestEnabled = boolInt(enabled)
		row.SuggestVersion++
		row.NamingRevision++
		row.UpdatedAt = time.Now().Unix()
		return tx.Model(&row).Updates(map[string]any{"auto_suggest_enabled": row.AutoSuggestEnabled, "suggest_version": row.SuggestVersion, "naming_revision": row.NamingRevision, "updated_at": row.UpdatedAt}).Error
	})
	return row.toModel(), err
}

// Bind every command to one idle turn and a monotonically increasing
// generation. New runs, refreshes and preference changes invalidate old work.
func prepareSuggestCommand(tx *gorm.DB, execution *model.CommandExecution, thread threadPO) error {
	var latest runPO
	if err := tx.Where("thread_id = ?", thread.ID).Order("start_event_seq DESC").First(&latest).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.NewAgentError("BAD_REQUEST", "suggest", errors.New("会话尚无已完成的回复"))
		}
		return err
	}
	if latest.Status != string(model.RunDone) && latest.Status != string(model.RunFailed) && latest.Status != string(model.RunCanceled) {
		return store.ErrRunActive
	}
	if execution.Source == "automatic" && (thread.AutoSuggestEnabled == 0 || execution.RunID != latest.ID || latest.Status != string(model.RunDone)) {
		return context.Canceled
	}
	if execution.Source == "automatic" {
		if suppressed, err := automaticSuggestionsSuppressed(tx, latest); err != nil {
			return err
		} else if suppressed {
			return context.Canceled
		}
	}
	execution.RunID = latest.ID
	execution.SuggestVersion = thread.SuggestVersion + 1
	return tx.Model(&threadPO{}).Where("id = ?", thread.ID).Update("suggest_version", execution.SuggestVersion).Error
}

func (s *Store) CheckSuggestCommand(ctx context.Context, execution model.CommandExecution) error {
	return checkSuggestCommand(s.db.WithContext(ctx), execution)
}

func checkSuggestCommand(tx *gorm.DB, execution model.CommandExecution) error {
	var command commandExecutionPO
	if err := tx.First(&command, "id = ?", execution.AttemptID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return context.Canceled
		}
		return err
	}
	if command.Status == "cancel_requested" || !activeCommandStatus(command.Status) || command.ExecutionRevision != execution.ExecutionRevision || command.OwnerInstanceID != execution.OwnerInstanceID {
		return context.Canceled
	}

	var thread threadPO
	if err := tx.First(&thread, "id = ?", execution.ThreadID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return context.Canceled
		}
		return err
	}
	if thread.SuggestVersion != execution.SuggestVersion || (execution.Source == "automatic" && thread.AutoSuggestEnabled == 0) {
		return context.Canceled
	}
	var latest runPO
	if err := tx.Where("thread_id = ?", execution.ThreadID).Order("start_event_seq DESC").First(&latest).Error; err != nil {
		return err
	}
	if latest.ID != execution.RunID || (latest.Status != string(model.RunDone) && latest.Status != string(model.RunFailed) && latest.Status != string(model.RunCanceled)) {
		return context.Canceled
	}
	if execution.Source == "automatic" {
		if suppressed, err := automaticSuggestionsSuppressed(tx, latest); err != nil {
			return err
		} else if suppressed {
			return context.Canceled
		}
	}
	return nil
}

func automaticSuggestionsSuppressed(tx *gorm.DB, current runPO) (bool, error) {
	// This projection commits atomically with the fixed reply's terminal event.
	var count int64
	err := tx.Table("idempotency_records").Where("scope = 'automatic_suggestions' AND owner_id = ? AND key = 'suppressed' AND status = 'completed'", current.ID).Count(&count).Error
	return count > 0, err
}
