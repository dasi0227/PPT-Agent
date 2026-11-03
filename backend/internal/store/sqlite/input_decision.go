package sqlite

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"gorm.io/gorm"
)

func inputDecision(input inputPO, key string) (model.InputDecision, error) {
	var decision model.InputDecision
	if input.DecisionJSON == nil {
		return decision, run.ErrRunNotFound
	}
	if err := json.Unmarshal([]byte(*input.DecisionJSON), &decision); err != nil {
		return decision, err
	}
	// A transferred input retains its accepted identity and decision; its role in
	// the new execution can change from steering to the primary input.
	decision.InputID = key
	decision.ReplyMessageID = textOrEmpty(input.ReplyMessageID)
	return decision, nil
}

func (s *Store) GetInputDecision(ctx context.Context, threadID, runID, key string) (model.InputDecision, error) {
	var result model.InputDecision
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := readSessionHead(tx, threadID); err != nil {
			return err
		}
		input, err := findRunInput(tx, threadID, runID, key)
		if err != nil {
			return err
		}
		result, err = inputDecision(input, key)
		return err
	})
	return result, err
}

// Classification is stored on its accepted input. Paused startup checks can
// publish decisions without gaining permission to resume the worker.
func (s *Store) SaveInputDecision(ctx context.Context, current model.Run, result model.InputDecision) (model.InputDecision, error) {
	if result.InputID == "" || result.Fingerprint == "" || result.ExecutionRevision != current.ExecutionRevision {
		return result, sessionstate.ErrInvalidState
	}
	switch result.Disposition {
	case model.InputTask, model.InputChat, model.InputIrrelevant, model.InputDisclosure, model.InputBypass, model.InputDetectionFailed:
	default:
		return result, sessionstate.ErrInvalidState
	}
	identity := sessionstate.CommitIdentity{Key: fmt.Sprintf("input-decision:%s:%s:%d:%s", current.ID, result.InputID, current.ExecutionRevision, result.Fingerprint), RequestHash: result.Fingerprint}
	receipt, err := s.commitSession(ctx, sessionstate.Fence{ProjectID: current.ProjectID, ThreadID: current.ThreadID, SceneRevision: current.SceneRevision}, identity, func(w *sessionWrite) error {
		var row runPO
		if err := w.tx.Where("id = ? AND thread_id = ?", current.ID, current.ThreadID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		if row.SceneRevision != current.SceneRevision || row.ExecutionRevision != current.ExecutionRevision || row.OwnerInstanceID != current.OwnerInstanceID ||
			model.RunStatus(row.Status).Terminal() || (row.Status == string(model.RunPaused) && current.Status != model.RunPaused) {
			return run.ErrRunRevisionConflict
		}
		input, err := findRunInput(w.tx, current.ThreadID, current.ID, result.InputID)
		if err != nil {
			return err
		}
		if input.DecisionJSON != nil {
			previous, err := inputDecision(input, result.InputID)
			if err != nil {
				return err
			}
			if previous.Finalized() || (previous.Fingerprint == result.Fingerprint && previous.Identity == result.Identity) {
				w.result, err = json.Marshal(previous)
				return err
			}
		}
		result.ReplyMessageID = ""
		raw, err := json.Marshal(result)
		if err != nil {
			return err
		}
		input.Disposition, input.DecisionJSON, input.UpdatedAt = result.Disposition, stringPointer(string(raw)), w.now
		if err := w.tx.Save(&input).Error; err != nil {
			return err
		}
		if err := w.tx.Exec(`UPDATE conversation_rounds SET has_task_input = EXISTS (
            SELECT 1 FROM conversation_messages AS m JOIN thread_inputs AS i ON i.id = m.input_id
            WHERE m.round_id = conversation_rounds.id AND i.disposition = 'task')
            WHERE thread_id = ? AND id = (SELECT round_id FROM conversation_messages WHERE id = ?)`, input.ThreadID, input.MessageID).Error; err != nil {
			return err
		}
		if result.StopsRun() {
			if err := w.tx.Table("runs").Where("id = ?", current.ID).Updates(map[string]any{
				"continuation_allowed": 0,
				"checkpoint_json":      gorm.Expr("CASE WHEN checkpoint_json IS NULL THEN NULL ELSE json_set(checkpoint_json, '$.continuation_allowed', json('false')) END"),
			}).Error; err != nil {
				return err
			}
			if err := w.publishRun(current.ID); err != nil {
				return err
			}
		}
		w.result = raw
		return nil
	})
	if err != nil {
		return model.InputDecision{}, err
	}
	var saved model.InputDecision
	if err := json.Unmarshal(receipt.Result, &saved); err != nil {
		return saved, err
	}
	return saved, nil
}

func (s *Store) PausedInputRuns(ctx context.Context) ([]model.Run, error) {
	var rows []runPO
	if err := s.reader.WithContext(ctx).Where("status = ?", string(model.RunPaused)).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.Run, len(rows))
	for i, row := range rows {
		out[i] = row.toModel()
	}
	return out, nil
}

func (s *Store) GuardSteeringInputs(ctx context.Context, runID string) ([]model.SteeringMessage, error) {
	return readSteering(s.reader.WithContext(ctx).Where("steering_inbox.run_id = ? AND steering_inbox.input_id <> (SELECT input_id FROM runs WHERE id = ?)", runID, runID))
}

func inputStopExists(tx *gorm.DB, runID string) (bool, error) {
	var count int64
	err := stoppingInputs(tx, runID).Count(&count).Error
	return count > 0, err
}
func stoppingInputs(tx *gorm.DB, runID string) *gorm.DB {
	return tx.Session(&gorm.Session{NewDB: true}).Table("thread_inputs AS stopped_input").Where("stopped_input.run_id = ?", runID).
		Where("(disposition IN ? OR (disposition = ? AND id <> (SELECT input_id FROM runs WHERE id = ?)))", []string{model.InputDisclosure, model.InputBypass, model.InputDetectionFailed}, model.InputIrrelevant, runID)
}
