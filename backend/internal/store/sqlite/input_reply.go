package sqlite

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type inputReplyChoice struct {
	Text string `json:"text"`
}

// SQLite's single writer orders this selection with the actual reply event.
// Classification completion order does not advance a category's last choice.
func (s *Store) prepareInputReply(tx *gorm.DB, threadID string, event *model.Event, inputID string) (*model.InputDecision, bool, error) {
	var record idempotencyPO
	if err := tx.First(&record, "scope = 'input_decision' AND owner_id = ? AND key = ? AND status = 'completed'", event.RunID, inputID).Error; err != nil {
		return nil, false, err
	}
	var decision model.InputDecision
	if err := json.Unmarshal([]byte(record.ResultJSON), &decision); err != nil {
		return nil, false, err
	}
	choices := decision.ReplyCandidates()
	if decision.InputID != inputID || len(choices) == 0 {
		return nil, false, errors.New("input has no selectable fixed reply")
	}
	if decision.ReplyEventSeq > 0 {
		// A commit may have succeeded before journal delivery failed. Reuse the
		// committed event as well as its text, without advancing category state.
		saved, err := eventInTransaction(tx, threadID, decision.ReplyEventSeq)
		if err != nil {
			return nil, false, err
		}
		if saved.RunID != event.RunID || saved.Type != string(event.Type) {
			return nil, false, errors.New("fixed reply was already committed as another event")
		}
		event.Payload, event.Seq, event.CreatedAt = string(saved.Payload), saved.Seq, saved.TS
		return &decision, true, nil
	}

	var previous idempotencyPO
	err := tx.First(&previous, "scope = 'input_reply_choice' AND owner_id = ? AND key = ?", threadID, decision.Disposition).Error
	if err == nil {
		var last inputReplyChoice
		if err := json.Unmarshal([]byte(previous.ResultJSON), &last); err != nil {
			return nil, false, err
		}
		remaining := make([]string, 0, len(choices))
		for _, choice := range choices {
			if choice != last.Text {
				remaining = append(remaining, choice)
			}
		}
		choices = remaining
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	decision.ReplyText = choices[rand.IntN(len(choices))]

	var payload map[string]any
	if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
		return nil, false, err
	}
	if payload["run_id"] != event.RunID {
		return nil, false, errors.New("fixed reply run identity mismatch")
	}
	plainID := strings.TrimPrefix(inputID, "initial:")
	switch event.Type {
	case model.EventMessageReply:
		if decision.Disposition == model.InputIrrelevant || payload["input_id"] != plainID || payload["message_id"] != "input_"+plainID {
			return nil, false, errors.New("fixed reply input identity mismatch")
		}
		payload["text"] = decision.ReplyText
	case model.EventMessageFinal:
		if decision.Disposition == model.InputIrrelevant || !strings.HasPrefix(inputID, "initial:") || payload["message_id"] != "input_"+plainID {
			return nil, false, errors.New("fixed final reply must belong to the initial input")
		}
		payload["text"] = decision.ReplyText
	case model.EventRunCanceled:
		reply, ok := payload["reply"].(map[string]any)
		if !ok || decision.Disposition != model.InputIrrelevant || payload["reason"] != string(model.RunCancelInputRejected) || reply["input_id"] != plainID {
			return nil, false, errors.New("fixed terminal reply must reject the classified input")
		}
		reply["text"] = decision.ReplyText
	default:
		return nil, false, errors.New("unsupported fixed reply event")
	}
	if err := model.ValidatePublicEvent(event.Type, payload); err != nil {
		return nil, false, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, false, err
	}
	event.Payload = string(raw)
	last, err := json.Marshal(inputReplyChoice{Text: decision.ReplyText})
	if err != nil {
		return nil, false, err
	}
	err = tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "scope"}, {Name: "owner_id"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"result_json"}),
	}).Create(&idempotencyPO{Scope: "input_reply_choice", OwnerID: threadID, Key: decision.Disposition,
		RequestHash: "fixed_reply", Status: "completed", ResultJSON: string(last)}).Error
	return &decision, false, err
}
