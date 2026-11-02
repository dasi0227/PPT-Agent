package sqlite

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
	"gorm.io/gorm"
)

// Read the resolved text and its publication receipt under the same transaction
// that appends the public event. A journal flush can fail after this commit.
func (s *Store) prepareInputReply(tx *gorm.DB, threadID string, event *model.Event, inputID string) (*model.InputDecision, bool, error) {
	var record idempotencyPO
	if err := tx.First(&record, "scope = 'input_decision' AND owner_id = ? AND key = ? AND status = 'completed'", event.RunID, inputID).Error; err != nil {
		return nil, false, err
	}
	var decision model.InputDecision
	if err := json.Unmarshal([]byte(record.ResultJSON), &decision); err != nil {
		return nil, false, err
	}
	if decision.InputID != inputID || decision.Disposition != model.InputChat {
		return nil, false, errors.New("chat reply requires a matching chat input")
	}
	if decision.ReplyEventSeq > 0 {
		saved, err := eventInTransaction(tx, threadID, decision.ReplyEventSeq)
		if err != nil {
			return nil, false, err
		}
		if saved.RunID != event.RunID || saved.Type != string(event.Type) {
			return nil, false, errors.New("chat reply was already committed as another event")
		}
		event.Payload, event.Seq, event.CreatedAt = string(saved.Payload), saved.Seq, saved.TS
		return &decision, true, nil
	}

	raw := json.RawMessage(event.Payload)
	if err := model.ValidatePublicEvent(event.Type, raw); err != nil {
		return nil, false, err
	}
	var reply model.MessageReplyPayload
	if err := json.Unmarshal(raw, &reply); err != nil {
		return nil, false, err
	}
	if reply.RunID != event.RunID || reply.Text != decision.Reply() {
		return nil, false, errors.New("chat reply does not match the resolved input")
	}
	switch event.Type {
	case model.EventMessageReply:
		if reply.InputID != inputID || reply.MessageID != "input_"+inputID {
			return nil, false, errors.New("chat reply input identity mismatch")
		}
	case model.EventMessageFinal:
		id, initial := strings.CutPrefix(inputID, "initial:")
		if !initial || reply.MessageID != "input_"+id || decision.Failure != "" {
			return nil, false, errors.New("only a successful initial chat can publish a final reply")
		}
	default:
		return nil, false, errors.New("unsupported chat reply event")
	}
	return &decision, false, nil
}

func (s *Store) appendStandaloneChatHistory(tx *gorm.DB, reply threadjournal.Event, decision model.InputDecision) error {
	var current runPO
	if err := tx.First(&current, "id = ?", reply.RunID).Error; err != nil {
		return mapErr(err)
	}
	if decision.InputID != "initial:"+current.ClientRequestID {
		return errors.New("initial chat reply does not match the accepted input")
	}
	edit, err := contextengine.StandaloneChatEdit(current.ID, reply.Seq, current.Command.Instruction, decision.ReplyText)
	if err != nil {
		return err
	}
	_, err = s.enqueueEvent(tx, current.ThreadID, edit)
	return err
}
