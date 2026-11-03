package sqlite

import (
	"encoding/json"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"gorm.io/gorm"
)

type inputPO struct {
	ID               string `gorm:"primaryKey"`
	ThreadID         string
	ClientMessageID  string
	MessageID        string
	RequestHash      string
	RunID            *string
	SourceRunID      *string
	AcceptedSeq      int64
	HistoryBeforeSeq int64
	Disposition      string
	DecisionJSON     *string
	DeliveryState    string
	ReplyMessageID   *string
	ReferenceJSON    string
	CreatedAt        int64
	UpdatedAt        int64
}

func (inputPO) TableName() string { return "thread_inputs" }

func findRunInput(tx *gorm.DB, threadID, runID, key string) (inputPO, error) {
	var input inputPO
	query := tx.Where("thread_id = ?", threadID)
	if initial, ok := strings.CutPrefix(key, "initial:"); ok {
		query = query.Where("id = (SELECT input_id FROM runs WHERE id = ? AND thread_id = ? AND client_request_id = ?)", runID, threadID, initial)
	} else {
		query = query.Where("client_message_id = ? AND run_id = ?", key, runID)
	}
	err := query.Take(&input).Error
	return input, mapErr(err)
}

func (w *sessionWrite) acceptInput(id, clientID, runID, requestHash string, command model.RunCommand) (inputPO, error) {
	input := inputPO{ID: id, ThreadID: w.head.ID, ClientMessageID: clientID, MessageID: model.MustShortID("msg"), RequestHash: requestHash, RunID: stringPointer(runID), AcceptedSeq: w.seq, HistoryBeforeSeq: w.seq, Disposition: "pending", DeliveryState: "accepted", CreatedAt: w.now, UpdatedAt: w.now}
	raw, err := json.Marshal(command)
	if err != nil {
		return input, err
	}
	input.ReferenceJSON = string(raw)
	public, err := json.Marshal(map[string]any{"text": command.Instruction})
	if err != nil {
		return input, err
	}
	roundID, err := w.openConversationRound(runID)
	if err != nil {
		return input, err
	}
	message := sessionstate.Message{ID: input.MessageID, RunID: runID, InputID: id, RoundID: roundID, Kind: "instruction", Native: llm.Message{Role: llm.RoleUser, Content: llm.TextContent(command.Instruction), Metadata: &llm.MessageMetadata{Origin: "user", Kind: "instruction", RunID: runID, Key: clientID}}, Public: public}
	if err := w.insertMessages([]sessionstate.Message{message}); err != nil {
		return input, err
	}
	if err := w.tx.Create(&input).Error; err != nil {
		return input, err
	}
	return input, w.projectInput(input, command, "run:"+runID)
}

func (w *sessionWrite) projectInput(input inputPO, command model.RunCommand, groupID string) error {
	value := map[string]any{
		"type": "user_turn", "text": command.Instruction, "runId": textOrEmpty(input.RunID),
		"scope": command.Scope, "mode": command.Mode, "skills": command.PublicSkills(), "components": command.PublicComponents(),
		"deliveryStatus": input.DeliveryState, "clientMessageId": strings.TrimPrefix(input.ClientMessageID, "initial:"),
		"steering":      !strings.HasPrefix(input.ClientMessageID, "initial:"),
		"domSelections": model.PublicDOMSelections(command.DOMSelections), "referenceOrder": command.ReferenceOrder,
		"attachments": command.Attachments,
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return w.putPublicItem(sessionstate.TimelineItem{ID: "input:" + input.ID, ThreadID: w.head.ID, GroupID: groupID, Kind: "user_turn", MessageID: input.MessageID}, raw)
}

func (w *sessionWrite) moveInput(input inputPO, command model.RunCommand, groupID string) error {
	var item timelineItemPO
	if err := w.tx.Where("id = ? AND thread_id = ?", "input:"+input.ID, w.head.ID).Take(&item).Error; err != nil {
		return err
	}
	if item.GroupID != groupID {
		var position int64
		if err := w.tx.Table("timeline_items").Select("COALESCE(MAX(position),0)+1").Where("group_id = ?", groupID).Scan(&position).Error; err != nil {
			return err
		}
		if err := w.tx.Model(&timelineItemPO{}).Where("id = ?", item.ID).Updates(map[string]any{"group_id": groupID, "position": position}).Error; err != nil {
			return err
		}
		for _, change := range []struct {
			ID    string
			Delta int
		}{{item.GroupID, -1}, {groupID, 1}} {
			if err := w.tx.Model(&timelineGroupPO{}).Where("id = ?", change.ID).Updates(map[string]any{"item_count": gorm.Expr("item_count + ?", change.Delta), "entity_revision": gorm.Expr("entity_revision + 1"), "updated_seq": w.seq}).Error; err != nil {
				return err
			}
			var group timelineGroupPO
			if err := w.tx.Where("id = ?", change.ID).Take(&group).Error; err != nil {
				return err
			}
			if err := w.upsert("timeline.group", group.ID, group.view()); err != nil {
				return err
			}
		}
	}
	return w.projectInput(input, command, groupID)
}
