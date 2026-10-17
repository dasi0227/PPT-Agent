package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SaveInputDecision fences late evaluator results by the worker's actual lease.
// The existing idempotency table owns the durable private result, not the UI.
func (s *Store) SaveInputDecision(ctx context.Context, current model.Run, result model.InputDecision) (model.InputDecision, error) {
	raw, err := json.Marshal(result)
	if err != nil {
		return result, err
	}
	saved := result
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
			if previous.StopsRun() {
				saved = previous
				return nil
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
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

// LoadIntentHistory reads original journal text. It never uses model context
// compaction, tool records, progress, or ask_user answers as dialogue history.
func (s *Store) LoadIntentHistory(ctx context.Context, threadID, runID, inputID string) ([]model.DialogueRound, error) {
	initial := strings.HasPrefix(inputID, "initial:")
	inputID = strings.TrimPrefix(inputID, "initial:")
	events, err := s.ThreadEvents(ctx, threadID, 0)
	if err != nil {
		return nil, err
	}
	type userInput struct{ id, text string }
	pending := map[string][]userInput{}
	rounds := []model.DialogueRound{}
	appendRound := func(runID, inputID, reply string) {
		inputs := []string{}
		remaining := []userInput{}
		for _, item := range pending[runID] {
			if inputID == "" || item.id == inputID {
				inputs = append(inputs, item.text)
			} else {
				remaining = append(remaining, item)
			}
		}
		pending[runID] = remaining
		if len(inputs) > 0 && reply != "" {
			rounds = append(rounds, model.DialogueRound{UserInputs: inputs, FinalReply: reply})
		}
	}
	found := false
	for _, event := range events {
		switch event.Type {
		case "run.accepted":
			var p runAcceptance
			if err := json.Unmarshal(event.Payload, &p); err != nil {
				return nil, err
			}
			if initial && event.RunID == runID && p.ClientRequestID == inputID {
				found = true
				break
			}
			pending[event.RunID] = append(pending[event.RunID], userInput{p.ClientRequestID, p.Command.Instruction})
		case "steering.accepted":
			var p model.SteeringMessage
			if err := json.Unmarshal(event.Payload, &p); err != nil {
				return nil, err
			}
			if !initial && event.RunID == runID && p.ClientMessageID == inputID {
				found = true
				break
			}
			// Handoff may repeat the original input under the new run.
			seen := false
			for _, item := range pending[event.RunID] {
				if item.id == p.ClientMessageID {
					seen = true
				}
			}
			if !seen {
				pending[event.RunID] = append(pending[event.RunID], userInput{p.ClientMessageID, p.Content})
			}
		case string(model.EventMessageFinal):
			var p model.MessageFinalPayload
			if err := json.Unmarshal(event.Payload, &p); err != nil {
				return nil, err
			}
			appendRound(event.RunID, "", p.Text)
		case string(model.EventMessageReply):
			var p model.MessageReplyPayload
			if err := json.Unmarshal(event.Payload, &p); err != nil {
				return nil, err
			}
			appendRound(event.RunID, p.InputID, p.Text)
		case string(model.EventRunCanceled), string(model.EventRunFailed):
			var p model.RunTerminalPayload
			if err := json.Unmarshal(event.Payload, &p); err != nil {
				return nil, err
			}
			if p.Reply != nil {
				appendRound(event.RunID, "", p.Reply.Text)
			}
		}
		if found {
			break
		}
	}
	if !found {
		return nil, errors.New("classification input is not durably accepted")
	}
	if len(rounds) > 4 {
		rounds = rounds[len(rounds)-4:]
	}
	return rounds, nil
}

// A persisted rejection/failure can never be made continuable by a later
// checkpoint or by a direct resume endpoint call.
func inputStopExists(tx *gorm.DB, runID string) (bool, error) {
	var count int64
	err := tx.Table("idempotency_records").Where("scope = 'input_decision' AND owner_id = ? AND status = 'completed' AND json_extract(result_json, '$.disposition') IN ?", runID,
		[]string{model.InputInternalInstructions, model.InputRuleBypass, model.InputDetectionFailed}).Count(&count).Error
	return count > 0, err
}

func withoutInputStop(tx *gorm.DB, runID string) *gorm.DB {
	return tx.Where("NOT EXISTS (SELECT 1 FROM idempotency_records WHERE scope = 'input_decision' AND owner_id = ? AND status = 'completed' AND json_extract(result_json, '$.disposition') IN ?)", runID,
		[]string{model.InputInternalInstructions, model.InputRuleBypass, model.InputDetectionFailed})
}
