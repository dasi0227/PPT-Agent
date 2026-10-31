package sqlite

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SaveInputDecision fences late evaluator results by the worker's actual lease.
// The existing idempotency table owns the durable private result, not the UI.
func (s *Store) SaveInputDecision(ctx context.Context, current model.Run, result model.InputDecision) (model.InputDecision, error) {
	saved := result
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row runPO
		if err := tx.First(&row, "id = ?", current.ID).Error; err != nil {
			return mapErr(err)
		}
		if row.ExecutionRevision != current.ExecutionRevision || row.OwnerInstanceID != current.OwnerInstanceID ||
			model.RunStatus(row.Status).Terminal() || (row.Status == string(model.RunPaused) && current.Status != model.RunPaused) {
			return run.ErrRunRevisionConflict
		}
		var existing idempotencyPO
		if err := tx.First(&existing, "scope = 'input_decision' AND owner_id = ? AND key = ?", current.ID, result.InputID).Error; err == nil {
			var previous model.InputDecision
			if err := json.Unmarshal([]byte(existing.ResultJSON), &previous); err != nil {
				return err
			}
			if previous.StopsRun() || previous.ReplyEventSeq > 0 {
				saved = previous
				return nil
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// Reply selection is owned by the event transaction, never a classifier.
		result.ReplyText, result.ReplyEventSeq = "", 0
		saved = result
		raw, err := json.Marshal(result)
		if err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "scope"}, {Name: "owner_id"}, {Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"request_hash", "status", "result_json"}),
		}).Create(&idempotencyPO{Scope: "input_decision", OwnerID: current.ID, Key: result.InputID,
			RequestHash: result.Fingerprint, Status: "completed", ResultJSON: string(raw)}).Error
	})
	return saved, err
}

func (s *Store) PausedInputRuns(ctx context.Context) ([]model.Run, error) {
	var rows []runPO
	if err := s.db.WithContext(ctx).Where("status = ?", string(model.RunPaused)).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.Run, len(rows))
	for i, row := range rows {
		out[i] = row.toModel()
	}
	return out, nil
}

func (s *Store) GuardSteeringInputs(ctx context.Context, runID string) ([]model.SteeringMessage, error) {
	var rows []steeringPO
	if err := s.db.WithContext(ctx).Where("run_id = ?", runID).Order("input_event_seq ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.SteeringMessage, len(rows))
	for i, row := range rows {
		out[i] = row.toModel()
	}
	return out, nil
}

// A persisted rejection/failure can never be made continuable by a later
// checkpoint or by a direct resume endpoint call.
func inputStopExists(tx *gorm.DB, runID string) (bool, error) {
	var count int64
	err := tx.Table("idempotency_records").Where("scope = 'input_decision' AND owner_id = ? AND status = 'completed' AND json_extract(result_json, '$.disposition') IN ?", runID,
		[]string{model.InputDisclosure, model.InputBypass, model.InputIrrelevant, model.InputDetectionFailed}).Count(&count).Error
	return count > 0, err
}

func withoutInputStop(tx *gorm.DB, runID string) *gorm.DB {
	return tx.Where("NOT EXISTS (SELECT 1 FROM idempotency_records WHERE scope = 'input_decision' AND owner_id = ? AND status = 'completed' AND json_extract(result_json, '$.disposition') IN ?)", runID,
		[]string{model.InputDisclosure, model.InputBypass, model.InputIrrelevant, model.InputDetectionFailed})
}
