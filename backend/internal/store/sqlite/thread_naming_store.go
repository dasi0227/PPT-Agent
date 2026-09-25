package sqlite

import (
	"context"
	"encoding/json"
	"errors"

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
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&idempotencyPO{Scope: "naming_input", OwnerID: input.ThreadID, Key: input.InputID, RequestHash: input.InputID, Status: "completed", ResultJSON: "{}"})
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

func (s *Store) ListThreadNamingInputs(ctx context.Context, threadID string, limit int) ([]model.ThreadNamingInput, error) {
	events, err := s.ThreadEvents(ctx, threadID, 0)
	if err != nil {
		return nil, err
	}
	out := []model.ThreadNamingInput{}
	for _, e := range events {
		var input model.ThreadNamingInput
		switch e.Type {
		case "run.accepted":
			var a runAcceptance
			if err := json.Unmarshal(e.Payload, &a); err != nil {
				return nil, err
			}
			input = model.ThreadNamingInput{ThreadID: threadID, InputID: a.ClientRequestID, Content: a.Command.Instruction, AcceptedAt: e.TS}
		case "steering.accepted":
			var a model.SteeringMessage
			if err := json.Unmarshal(e.Payload, &a); err != nil {
				return nil, err
			}
			input = model.ThreadNamingInput{ThreadID: threadID, InputID: a.ClientMessageID, Content: a.Content, AcceptedAt: e.TS}
		default:
			continue
		}
		out = append(out, input)
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}
func (s *Store) LoadThreadRenameContext(ctx context.Context, threadID string) (model.ThreadRenameContextSource, error) {
	source := model.ThreadRenameContextSource{AssistantReplies: []string{}}
	inputs, err := s.ListThreadNamingInputs(ctx, threadID, 0)
	if err != nil {
		return source, err
	}
	if len(inputs) > 16 {
		inputs = append([]model.ThreadNamingInput{inputs[0]}, inputs[len(inputs)-16:]...)
	}
	source.Inputs = inputs
	events, err := s.ListThreadEvents(ctx, threadID)
	if err != nil {
		return source, err
	}
	for _, event := range events {
		switch event.Type {
		case model.EventMessageFinal:
			var p model.MessageFinalPayload
			if err := json.Unmarshal([]byte(event.Payload), &p); err != nil {
				return source, err
			}
			source.AssistantReplies = append(source.AssistantReplies, p.Text)
		case model.EventPlanUpdated:
			var p model.PlanUpdatedPayload
			if err := json.Unmarshal([]byte(event.Payload), &p); err != nil {
				return source, err
			}
			source.Plan = &p.Plan
		}
	}
	if len(source.AssistantReplies) > 4 {
		source.AssistantReplies = source.AssistantReplies[len(source.AssistantReplies)-4:]
	}
	compactions, err := s.ListThreadContextCompactions(ctx, threadID)
	if err != nil && !errors.Is(err, run.ErrRunNotFound) {
		return source, err
	}
	if len(compactions) > 0 {
		source.ContextSummary = compactions[len(compactions)-1].Content
	}
	return source, nil
}
