package run

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

// Custom scheduler executions save the waiting boundary before the prompter
// reads it, just as Runtime checkpoints do in production.
func (s *memStore) saveInteraction(runID, kind, id, callID string, request any) error {
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	typeOf := model.EventQuestionAsked
	if kind == "command" {
		typeOf = model.EventCommandPermissionRequested
	}
	update, err := sessionstate.NewRunUpdate(typeOf, request)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.interactions == nil {
		s.interactions = map[string]model.Interaction{}
	}
	s.interactions[id] = model.Interaction{ID: id, RunID: runID, Kind: kind, CallID: callID, State: "pending", Request: raw, RequestRevision: 1}
	current := s.runs[runID]
	current.Status = model.RunWaiting
	s.runs[runID] = current
	facts := s.events[runID]
	s.events[runID] = append(facts, model.Event{RunID: runID, Seq: int64(len(facts) + 1), Type: update.Type, Payload: string(update.Payload)})
	return nil
}

func (s *memStore) Interaction(_ context.Context, runID, kind, id string) (model.Interaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.interactions[id]
	if !ok || row.RunID != runID || row.Kind != kind {
		return model.Interaction{}, ErrRunNotFound
	}
	return row, nil
}

func (s *memStore) AcceptInteractionAnswer(ctx context.Context, runID, kind, id string, raw json.RawMessage) (sessionstate.Receipt, error) {
	row, err := s.Interaction(ctx, runID, kind, id)
	if err != nil {
		return sessionstate.Receipt{}, err
	}
	answer, err := model.NormalizeInteractionAnswer(row, raw)
	if err != nil {
		return sessionstate.Receipt{}, ErrReplyMismatch
	}
	s.mu.Lock()
	if current := s.interactions[id]; current.State != "pending" {
		s.mu.Unlock()
		if bytes.Equal(current.Answer, answer) {
			return sessionstate.Receipt{}, nil
		}
		return sessionstate.Receipt{}, ErrReplyMismatch
	}
	row.State, row.Answer = "answered", answer
	s.interactions[id] = row
	s.mu.Unlock()
	var payload any
	typeOf := model.EventQuestionAnswered
	if kind == "question" {
		var reply model.QuestionReply
		_ = json.Unmarshal(answer, &reply)
		payload = model.QuestionAnsweredPayload{PublicEventBase: model.NewPublicEventBase(runID), QuestionID: id, Answer: reply.Answer}
	} else {
		var reply model.CommandPermissionAnswer
		_ = json.Unmarshal(answer, &reply)
		typeOf = model.EventCommandPermissionAnswered
		payload = model.CommandPermissionAnsweredPayload{PublicEventBase: model.NewPublicEventBase(runID), InteractionID: id, CallID: reply.CallID, CommandHash: reply.CommandHash, Decision: reply.Decision, Feedback: reply.Feedback}
	}
	update, err := sessionstate.NewRunUpdate(typeOf, payload)
	if err != nil {
		return sessionstate.Receipt{}, err
	}
	return s.ApplyRunUpdate(ctx, runID, update)
}

func emitTestFinal(emitter workflow.EventEmitter, runID string) {
	emitter.Emit(model.EventMessageFinal, model.MessageFinalPayload{PublicEventBase: model.NewPublicEventBase(runID), MessageID: "final-" + runID, Text: "done", AffectedTargets: []model.PublicTarget{}})
}
