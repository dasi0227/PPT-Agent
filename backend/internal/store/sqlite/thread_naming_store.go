package sqlite

import (
	"context"
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Store) RecordThreadNamingInput(ctx context.Context, input model.ThreadNamingInput) (model.Thread, bool, error) {
	var thread model.Thread
	shouldTrigger := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&threadNamingInputPO{
			ThreadID: input.ThreadID, InputID: input.InputID, Content: input.Content, AcceptedAt: input.AcceptedAt,
		})
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
				shouldTrigger = po.RenameInputCount >= 5
			}
		}
		if err := tx.Model(&threadPO{}).Where("id = ?", input.ThreadID).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.First(&po, "id = ?", input.ThreadID).Error; err != nil {
			return mapErr(err)
		}
		thread = po.toModel()
		return nil
	})
	return thread, shouldTrigger, err
}

func (s *Store) BeginThreadRenameRequest(ctx context.Context, id string, expectedOperationVersion int64, resetInputCount bool, updatedAt int64) (model.Thread, error) {
	var out model.Thread
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
			"updated_at":               updatedAt,
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
		return nil
	})
	return out, err
}

func (s *Store) StartThreadExplicitRenameRequest(ctx context.Context, id string, operationVersion int64, updatedAt int64) (model.Thread, error) {
	var out model.Thread
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var po threadPO
		if err := tx.First(&po, "id = ?", id).Error; err != nil {
			return mapErr(err)
		}
		if po.AutoRenameEnabled == 0 || po.RenameFirstInputSeen == 0 || po.RenameOperationVersion != operationVersion {
			return store.ErrNamingOperationConflict
		}
		if err := tx.Model(&threadPO{}).Where("id = ?", id).Updates(map[string]any{
			"rename_input_count": 0, "updated_at": updatedAt,
		}).Error; err != nil {
			return err
		}
		if err := tx.First(&po, "id = ?", id).Error; err != nil {
			return mapErr(err)
		}
		out = po.toModel()
		return nil
	})
	return out, err
}

func (s *Store) ApplyThreadRenameResult(ctx context.Context, id, title string, operationVersion int64, updatedAt int64) (model.Thread, bool, error) {
	var out model.Thread
	applied := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var po threadPO
		if err := tx.First(&po, "id = ?", id).Error; err != nil {
			return mapErr(err)
		}
		if po.AutoRenameEnabled == 0 || po.RenameOperationVersion != operationVersion {
			out = po.toModel()
			return nil
		}
		applied = true
		if po.Title == title {
			out = po.toModel()
			return nil
		}
		if err := tx.Model(&threadPO{}).Where("id = ?", id).Updates(map[string]any{
			"title": title, "naming_revision": gorm.Expr("naming_revision + 1"), "updated_at": updatedAt,
		}).Error; err != nil {
			return err
		}
		if err := tx.First(&po, "id = ?", id).Error; err != nil {
			return mapErr(err)
		}
		out = po.toModel()
		return nil
	})
	return out, applied, err
}

func (s *Store) UpdateThreadNamingState(ctx context.Context, id string, title *string, enabled *bool, resetInputCount bool, updatedAt int64) (model.Thread, error) {
	var out model.Thread
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var po threadPO
		if err := tx.First(&po, "id = ?", id).Error; err != nil {
			return mapErr(err)
		}
		updates := map[string]any{
			"rename_operation_version": gorm.Expr("rename_operation_version + 1"),
			"updated_at":               updatedAt,
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
		return nil
	})
	return out, err
}

func (s *Store) GetThreadNamingOperation(ctx context.Context, threadID, operationID string) (model.ThreadNamingOperation, error) {
	var po threadNamingOperationPO
	if err := s.db.WithContext(ctx).First(&po, "thread_id = ? AND operation_id = ?", threadID, operationID).Error; err != nil {
		return model.ThreadNamingOperation{}, mapErr(err)
	}
	return po.toModel(), nil
}

func (s *Store) CreateThreadNamingOperation(ctx context.Context, operation model.ThreadNamingOperation) (bool, error) {
	po := threadNamingOperationPO{
		ThreadID: operation.ThreadID, OperationID: operation.OperationID, RequestHash: operation.RequestHash,
		Action: operation.Action, Status: operation.Status, RequestID: operation.RequestID,
		ResultJSON: operation.ResultJSON, CreatedAt: operation.CreatedAt, UpdatedAt: operation.UpdatedAt,
	}
	result := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&po)
	return result.RowsAffected == 1, result.Error
}

func (s *Store) CompleteThreadNamingOperation(ctx context.Context, operation model.ThreadNamingOperation) error {
	result := s.db.WithContext(ctx).Model(&threadNamingOperationPO{}).
		Where("thread_id = ? AND operation_id = ? AND request_hash = ?", operation.ThreadID, operation.OperationID, operation.RequestHash).
		Updates(map[string]any{"status": operation.Status, "request_id": operation.RequestID, "result_json": operation.ResultJSON, "updated_at": operation.UpdatedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return store.ErrNamingOperationConflict
	}
	return nil
}

func (s *Store) ListThreadNamingInputs(ctx context.Context, threadID string, limit int) ([]model.ThreadNamingInput, error) {
	if limit < 1 {
		limit = 20
	}
	var rows []threadNamingInputPO
	if err := s.db.WithContext(ctx).Where("thread_id = ?", threadID).
		Order("accepted_at DESC, input_id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.ThreadNamingInput, len(rows))
	for index := range rows {
		out[len(rows)-1-index] = rows[index].toModel()
	}
	return out, nil
}

func (s *Store) LoadThreadRenameContext(ctx context.Context, threadID string) (model.ThreadRenameContextSource, error) {
	inputs, err := s.ListThreadNamingInputs(ctx, threadID, 16)
	if err != nil {
		return model.ThreadRenameContextSource{}, err
	}
	var first threadNamingInputPO
	firstResult := s.db.WithContext(ctx).Where("thread_id = ?", threadID).Order("accepted_at ASC, input_id ASC").Limit(1).Find(&first)
	if firstResult.Error != nil {
		return model.ThreadRenameContextSource{}, firstResult.Error
	}
	if firstResult.RowsAffected > 0 && (len(inputs) == 0 || inputs[0].InputID != first.InputID) {
		inputs = append([]model.ThreadNamingInput{first.toModel()}, inputs...)
	}
	var eventRows []struct {
		Type    string
		Payload string
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT e.type, e.payload
		FROM run_events e
		JOIN runs r ON r.id = e.run_id
		WHERE r.thread_id = ? AND e.type IN (?, ?)
		ORDER BY e.created_at DESC, e.seq DESC
		LIMIT 16
	`, threadID, string(model.EventMessageFinal), string(model.EventPlanUpdated)).Scan(&eventRows).Error; err != nil {
		return model.ThreadRenameContextSource{}, err
	}
	source := model.ThreadRenameContextSource{Inputs: inputs, AssistantReplies: []string{}}
	for _, row := range eventRows {
		switch row.Type {
		case string(model.EventMessageFinal):
			if len(source.AssistantReplies) >= 4 {
				continue
			}
			var payload model.MessageFinalPayload
			if json.Unmarshal([]byte(row.Payload), &payload) == nil && payload.Text != "" {
				source.AssistantReplies = append(source.AssistantReplies, payload.Text)
			}
		case string(model.EventPlanUpdated):
			if source.Plan != nil {
				continue
			}
			var payload model.PlanUpdatedPayload
			if json.Unmarshal([]byte(row.Payload), &payload) == nil {
				plan := payload.Plan
				source.Plan = &plan
			}
		}
	}
	for left, right := 0, len(source.AssistantReplies)-1; left < right; left, right = left+1, right-1 {
		source.AssistantReplies[left], source.AssistantReplies[right] = source.AssistantReplies[right], source.AssistantReplies[left]
	}
	var compaction contextCompactionPO
	result := s.db.WithContext(ctx).Where("thread_id = ?", threadID).Order("created_at DESC, id DESC").Limit(1).Find(&compaction)
	if result.Error != nil {
		return model.ThreadRenameContextSource{}, result.Error
	}
	if result.RowsAffected > 0 {
		source.ContextSummary = compaction.Summary
	}
	return source, nil
}
