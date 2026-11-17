package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

// The resolved answer, immutable public message, input receipt and display item
// share one transaction. No journal position is needed to recover the reply.
func (s *Store) CommitInputReply(ctx context.Context, runID, inputID string, update sessionstate.RunUpdate) (sessionstate.Receipt, error) {
	if inputID == "" {
		return sessionstate.Receipt{}, sessionstate.ErrInvalidState
	}
	scene, ok := sessionstate.RequestScene(ctx)
	if !ok {
		return sessionstate.Receipt{}, sessionstate.ErrSceneChanged
	}
	var current runPO
	if err := s.reader.WithContext(ctx).Where("id = ?", runID).Take(&current).Error; err != nil {
		return sessionstate.Receipt{}, mapErr(err)
	}
	if err := model.ValidatePublicEvent(update.Type, json.RawMessage(update.Payload)); err != nil {
		return sessionstate.Receipt{}, err
	}
	var requested model.MessageReplyPayload
	if err := json.Unmarshal([]byte(update.Payload), &requested); err != nil {
		return sessionstate.Receipt{}, err
	}
	hash, err := commandHash(struct {
		Type      model.EventType
		InputID   string
		MessageID string
		Text      string
	}{update.Type, inputID, requested.MessageID, requested.Text})
	if err != nil {
		return sessionstate.Receipt{}, err
	}
	return s.commitSession(ctx, sessionstate.Fence{ProjectID: current.ProjectID, ThreadID: current.ThreadID, SceneRevision: scene}, sessionstate.CommitIdentity{Key: "input-reply:" + runID + ":" + inputID, RequestHash: hash}, func(w *sessionWrite) error {
		if err := w.tx.Where("id = ?", runID).Take(&current).Error; err != nil {
			return mapErr(err)
		}
		if current.SceneRevision != scene || model.RunStatus(current.Status).Terminal() {
			return run.ErrRunRevisionConflict
		}
		if owner, execution, ok := workflow.CheckpointOwnership(ctx); ok && (current.OwnerInstanceID != owner || current.ExecutionRevision != execution) {
			return run.ErrRunRevisionConflict
		}
		input, err := findRunInput(w.tx, current.ThreadID, current.ID, inputID)
		if err != nil {
			return err
		}
		decision, err := inputDecision(input, inputID)
		if err != nil {
			return err
		}
		if !decision.RepliesDirectly() {
			return errors.New("direct reply requires a resolved input")
		}
		if input.ReplyMessageID != nil {
			return sessionstate.ErrIdempotencyConflict
		}
		var reply model.MessageReplyPayload
		if err := json.Unmarshal([]byte(update.Payload), &reply); err != nil {
			return err
		}
		if reply.RunID != runID || reply.Text != decision.Reply() {
			return errors.New("reply does not match its resolved input")
		}
		kind := "message"
		switch update.Type {
		case model.EventMessageReply:
			if reply.InputID != inputID || reply.MessageID != "input_"+inputID {
				return sessionstate.ErrInvalidState
			}
		case model.EventMessageFinal:
			initial, ok := strings.CutPrefix(inputID, "initial:")
			if !ok || reply.MessageID != "input_"+initial || decision.Failure != "" {
				return sessionstate.ErrInvalidState
			}
			kind = "final"
		default:
			return sessionstate.ErrInvalidState
		}
		var origin struct{ RoundID string }
		if err := w.tx.Table("conversation_messages").Select("round_id").Where("id = ? AND thread_id = ?", input.MessageID, w.head.ID).Take(&origin).Error; err != nil {
			return err
		}
		messageID := model.MustShortID("msg")
		raw := json.RawMessage(update.Payload)
		message := sessionstate.Message{ID: messageID, RunID: runID, InputID: input.ID, RoundID: origin.RoundID, Kind: "direct_reply", Public: raw,
			Native: llm.Message{Role: llm.RoleAssistant, Content: llm.TextContent(reply.Text), Metadata: &llm.MessageMetadata{Origin: "user", Kind: "direct_reply", RunID: runID, Key: inputID}}}
		if err := w.insertMessages([]sessionstate.Message{message}); err != nil {
			return err
		}
		decision.ReplyMessageID = messageID
		decisionJSON, err := json.Marshal(decision)
		if err != nil {
			return err
		}
		if err := w.tx.Model(&input).Updates(map[string]any{"reply_message_id": messageID, "decision_json": string(decisionJSON), "updated_at": w.now}).Error; err != nil {
			return err
		}
		if err := w.putPublicItem(sessionstate.TimelineItem{ID: "reply:" + input.ID, ThreadID: w.head.ID, GroupID: "run:" + runID, Kind: kind, MessageID: messageID}, raw); err != nil {
			return err
		}
		if update.Type == model.EventMessageFinal {
			if current.FinalMessageID != nil {
				return sessionstate.ErrVersionConflict
			}
			if err := w.tx.Table("runs").Where("id = ?", runID).Update("final_message_id", messageID).Error; err != nil {
				return err
			}
			if err := w.closeConversationRound(runID); err != nil {
				return err
			}
		}
		w.result, err = json.Marshal(map[string]string{"message_id": messageID, "input_id": input.ID})
		return err
	})
}
