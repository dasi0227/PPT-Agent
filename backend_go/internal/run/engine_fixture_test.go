package run

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type memStore struct {
	interactions map[string]model.Interaction
	mu           sync.Mutex
	runs         map[string]model.Run
	events       map[string][]model.Event
}

func newMemStore() *memStore {
	return &memStore{runs: map[string]model.Run{}, events: map[string][]model.Event{}}
}

func (s *memStore) RequestRunCancel(_ context.Context, id string, reason model.RunCancelReason, action sessionstate.RunAction) (sessionstate.RunMutation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.runs[id]
	if !ok {
		return sessionstate.RunMutation{}, ErrRunNotFound
	}
	if !value.Status.Terminal() && value.CancelRequestedAt == 0 {
		value.CancelRequestedAt = time.Now().UnixMilli()
		value.CancelRequestID, value.CancelReason = action.RequestKey, reason
	}
	if value.Status == model.RunPaused {
		payload := model.NewRunTerminalPayload(id, 0, nil, nil)
		payload.Reason = reason
		raw, _ := json.Marshal(payload)
		s.events[id] = append(s.events[id], model.Event{RunID: id, Seq: int64(len(s.events[id]) + 1), Type: model.EventRunCanceled, Payload: string(raw)})
		value.Status = model.RunCanceled
		value.PauseReason, value.PausedAt = "", 0
	}
	s.runs[id] = value
	return sessionstate.RunMutation{Current: value, Run: sessionstate.RunView{ID: id, Status: string(value.Status)}}, nil
}

func (s *memStore) CreateSteering(context.Context, model.SteeringMessage) (model.SteeringMessage, bool, error) {
	return model.SteeringMessage{}, false, errors.New("unexpected steering in lifecycle fixture")
}

func (s *memStore) ListPendingSteering(context.Context, string) ([]model.SteeringMessage, error) {
	return nil, nil
}

func (s *memStore) MarkSteering(context.Context, string, []string, model.SteeringStatus, int64, string) error {
	return errors.New("unexpected steering in lifecycle fixture")
}

func (s *memStore) CreateRun(_ context.Context, run model.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = run
	return nil
}

func (s *memStore) GetRun(_ context.Context, id string) (model.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[id]
	if !ok {
		return model.Run{}, ErrRunNotFound
	}
	return run, nil
}

func (s *memStore) SetRunStatus(_ context.Context, id string, status model.RunStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[id]
	if !ok {
		return ErrRunNotFound
	}
	run.Status = status
	s.runs[id] = run
	return nil
}

func (s *memStore) PauseNonTerminalRuns(_ context.Context, reason string, pausedAt int64) ([]model.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var paused []model.Run
	for id, value := range s.runs {
		if value.Status == model.RunPending || value.Status == model.RunRunning || value.Status == model.RunWaiting || value.Status == model.RunRecovering {
			value.Status, value.OwnerInstanceID = model.RunPaused, ""
			value.PauseReason, value.PausedAt = reason, pausedAt
			s.runs[id] = value
			paused = append(paused, value)
		}
	}
	return paused, nil
}

func (s *memStore) PauseRun(_ context.Context, id, ownerInstanceID, reason string, pausedAt int64) (model.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.runs[id]
	if !ok {
		return model.Run{}, ErrRunNotFound
	}
	if !value.Status.Terminal() && ownerInstanceID != "" && value.OwnerInstanceID != ownerInstanceID {
		return model.Run{}, ErrRunNotRunning
	}
	if !value.Status.Terminal() {
		value.Status, value.OwnerInstanceID = model.RunPaused, ""
		value.PauseReason, value.PausedAt = reason, pausedAt
		s.runs[id] = value
	}
	return value, nil
}

func (s *memStore) RunResumeReceipt(context.Context, string, sessionstate.RunAction) (sessionstate.RunMutation, bool, error) {
	return sessionstate.RunMutation{}, false, nil
}

func (s *memStore) ClaimRunForResume(_ context.Context, expected model.Run, ownerInstanceID string, _ sessionstate.RunAction) (sessionstate.RunMutation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.runs[expected.ID]
	if !ok || value.Status != model.RunPaused {
		return sessionstate.RunMutation{}, false, ErrRunNotRunning
	}
	value.Status, value.OwnerInstanceID = model.RunRecovering, ownerInstanceID
	value.ExecutionRevision++
	value.PauseReason, value.PausedAt = "", 0
	s.runs[expected.ID] = value
	return sessionstate.RunMutation{Current: value}, true, nil
}

// Scheduler tests record typed committed facts. Database transaction, pairing
// and idempotency invariants are exercised by SQLite tests, not replayed here.

func (s *memStore) ApplyRunUpdate(_ context.Context, runID string, update sessionstate.RunUpdate) (sessionstate.Receipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.runs[runID]
	if !ok {
		return sessionstate.Receipt{}, ErrRunNotFound
	}
	if err := model.ValidatePublicEvent(update.Type, update.Payload); err != nil {
		return sessionstate.Receipt{}, err
	}
	facts := s.events[runID]
	if update.Type.Terminal() && value.Status.Terminal() {
		return sessionstate.Receipt{}, nil
	}
	var identity struct {
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(update.Payload, &identity); err != nil {
		return sessionstate.Receipt{}, err
	}
	switch update.Type {
	case model.EventMessageFinal:
		value.FinalMessageID = identity.MessageID
	case model.EventRunCompleted:
		value.Status = model.RunDone
	case model.EventRunFailed, model.EventRunError:
		value.Status = model.RunFailed
	case model.EventRunCanceled:
		value.Status = model.RunCanceled
	}
	if value.Status.Terminal() {
		value.OwnerInstanceID = ""
	}
	s.runs[runID] = value
	s.events[runID] = append(facts, model.Event{RunID: runID, Seq: int64(len(facts) + 1), Type: update.Type, Payload: string(update.Payload)})
	return sessionstate.Receipt{}, nil
}

func (s *memStore) EventsSince(_ context.Context, runID string, after int64) ([]model.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []model.Event{}
	for _, event := range s.events[runID] {
		if event.Seq > after {
			out = append(out, event)
		}
	}
	return out, nil
}

func (s *memStore) status(id string) model.RunStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runs[id].Status
}

type scriptRunner func(context.Context, workflow.EventEmitter, Checkpointer, Prompter) workflow.StructuredOutcome

func (execution scriptRunner) Run(ctx context.Context, emitter workflow.EventEmitter, checkpoint Checkpointer, prompter Prompter) workflow.StructuredOutcome {
	return execution(ctx, emitter, checkpoint, prompter)
}

func testRun(id string) model.Run {
	return model.Run{
		ID: id, ThreadID: "t1", ProjectID: "p1",
		Command: model.RunCommand{
			Instruction: "test",
			Scope:       model.NewRunScope(model.ScopeAllPages),
			Mode:        model.ModeCreate,
		},
	}
}

func waitRunStatus(t *testing.T, store *memStore, id string, want model.RunStatus) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if store.status(id) == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("status=%s want=%s", store.status(id), want)
}

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

func (s *memStore) AcceptInteractionAnswer(context.Context, string, string, string, json.RawMessage) (sessionstate.Receipt, error) {
	return sessionstate.Receipt{}, errors.New("unexpected answer in cancellation fixture")
}

func emitTestFinal(emitter workflow.EventEmitter, runID string) {
	emitter.Emit(model.EventMessageFinal, model.MessageFinalPayload{PublicEventBase: model.NewPublicEventBase(runID), MessageID: "final-" + runID, Text: "done", AffectedTargets: []model.PublicTarget{}})
}

type rejectionAgent struct {
	calls    []llm.ToolCall
	requests []workflow.AgentRequest
}

func (a *rejectionAgent) Next(_ context.Context, request workflow.AgentRequest) (workflow.AgentResponse, error) {
	a.requests = append(a.requests, request)
	if len(a.requests) == 1 {
		return workflow.AgentResponse{ToolCalls: a.calls}, nil
	}
	return workflow.AgentResponse{Text: "检查完成"}, nil
}

// Exercise the real Runtime -> emitter -> Bus path, not an event-only recorder.
