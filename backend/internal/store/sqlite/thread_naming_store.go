package sqlite

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

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
		var current runPO
		if err := tx.First(&current, "id = ? AND thread_id = ?", input.RunID, input.ThreadID).Error; err != nil {
			return mapErr(err)
		}
		if current.ExecutionRevision != input.ExecutionRevision {
			return run.ErrRunRevisionConflict
		}
		var record idempotencyPO
		if err := tx.First(&record, "scope = 'input_decision' AND owner_id = ? AND key = ? AND status = 'completed'", input.RunID, input.InputID).Error; err != nil {
			return mapErr(err)
		}
		var decision model.InputDecision
		if err := json.Unmarshal([]byte(record.ResultJSON), &decision); err != nil {
			return err
		}
		if decision.InputID != input.InputID || decision.Disposition != model.InputNormal {
			return nil
		}
		accepted, err := eventInTransaction(tx, input.ThreadID, current.StartEventSeq)
		if err != nil {
			return err
		}
		var acceptance runAcceptance
		if err := json.Unmarshal(accepted.Payload, &acceptance); err != nil {
			return err
		}
		key := namingInputOrigin(acceptance, input.InputID)
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
		return nil
	})
	return thread, shouldTrigger, err
}

// Transferred inputs keep the original identity, including across recovery.
// Initial and steering inputs may legitimately share the same composer ID.
func namingInputOrigin(accepted runAcceptance, inputID string) string {
	if strings.HasPrefix(inputID, "initial:") && accepted.SourceRunID != "" && len(accepted.SourceMessageIDs) > 0 {
		return accepted.SourceMessageIDs[0]
	}
	return inputID
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

func (s *Store) LoadAutomaticRenameContext(ctx context.Context, threadID string) (model.ThreadRenameContextSource, error) {
	source := model.ThreadRenameContextSource{}
	var records []idempotencyPO
	if err := s.db.WithContext(ctx).Where("scope = 'input_decision' AND status = 'completed' AND owner_id IN (SELECT id FROM runs WHERE thread_id = ?)", threadID).Find(&records).Error; err != nil {
		return source, err
	}
	normal := map[string]map[string]bool{}
	for _, record := range records {
		var decision model.InputDecision
		if err := json.Unmarshal([]byte(record.ResultJSON), &decision); err != nil {
			return source, err
		}
		if decision.Disposition == model.InputNormal && decision.InputID == record.Key {
			if normal[record.OwnerID] == nil {
				normal[record.OwnerID] = map[string]bool{}
			}
			normal[record.OwnerID][decision.InputID] = true
		}
	}
	return s.loadThreadRenameContext(ctx, threadID, normal)
}

func (s *Store) LoadThreadRenameContext(ctx context.Context, threadID string) (model.ThreadRenameContextSource, error) {
	return s.loadThreadRenameContext(ctx, threadID, nil)
}

func (s *Store) loadThreadRenameContext(ctx context.Context, threadID string, normal map[string]map[string]bool) (model.ThreadRenameContextSource, error) {
	source := model.ThreadRenameContextSource{}
	// Read the journal once so user inputs and replies share one chronological history.
	events, err := s.ThreadEvents(ctx, threadID, 0)
	if err != nil {
		return source, err
	}
	seenInputs := map[string]bool{}
	for index := len(events) - 1; index >= 0; index-- {
		event := events[index]
		var activity model.ThreadNamingActivity
		switch event.Type {
		case "run.accepted":
			var p runAcceptance
			if err := json.Unmarshal(event.Payload, &p); err != nil {
				return source, err
			}
			inputID := "initial:" + p.ClientRequestID
			key := namingInputOrigin(p, inputID)
			if (normal != nil && !normal[event.RunID][inputID]) || seenInputs[key] {
				continue
			}
			seenInputs[key] = true
			activity = model.ThreadNamingActivity{Role: "user", Text: p.Command.Instruction}
		case "steering.accepted":
			var p model.SteeringMessage
			if err := json.Unmarshal(event.Payload, &p); err != nil {
				return source, err
			}
			key := p.ClientMessageID
			if (normal != nil && !normal[event.RunID][p.ClientMessageID]) || seenInputs[key] {
				continue
			}
			seenInputs[key] = true
			activity = model.ThreadNamingActivity{Role: "user", Text: p.Content}
		case string(model.EventMessageFinal):
			if normal != nil && len(normal[event.RunID]) == 0 {
				continue
			}
			var p model.MessageFinalPayload
			if err := json.Unmarshal(event.Payload, &p); err != nil {
				return source, err
			}
			activity = model.ThreadNamingActivity{Role: "assistant", Text: p.Text}
		case string(model.EventPlanUpdated):
			if source.Plan != nil || (normal != nil && len(normal[event.RunID]) == 0) {
				continue
			}
			var p model.PlanUpdatedPayload
			if err := json.Unmarshal(event.Payload, &p); err != nil {
				return source, err
			}
			source.Plan = &p.Plan
		}
		if strings.TrimSpace(activity.Text) == "" {
			continue
		}
		source.Activity = append(source.Activity, activity)
	}
	slices.Reverse(source.Activity)
	return source, nil
}
