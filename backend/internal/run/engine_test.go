package run

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type memStore struct {
	mu       sync.Mutex
	runs     map[string]model.Run
	events   map[string][]model.Event
	steering map[string]model.SteeringMessage
}

func newMemStore() *memStore {
	return &memStore{runs: map[string]model.Run{}, events: map[string][]model.Event{}, steering: map[string]model.SteeringMessage{}}
}

func (s *memStore) RequestRunCancel(_ context.Context, id string, at int64) (model.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.runs[id]
	if !ok {
		return model.Run{}, ErrRunNotFound
	}
	if value.Status.Terminal() || value.CancelRequestedAt != 0 {
		return value, nil
	}
	value.CancelRequestedAt = at
	s.runs[id] = value
	return value, nil
}

func (s *memStore) CreateSteering(_ context.Context, message model.SteeringMessage) (model.SteeringMessage, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := message.ThreadID + ":" + message.ClientMessageID
	if existing, ok := s.steering[key]; ok {
		return existing, false, nil
	}
	s.steering[key] = message
	return message, true, nil
}

func (s *memStore) ListPendingSteering(_ context.Context, runID string) ([]model.SteeringMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []model.SteeringMessage{}
	for _, message := range s.steering {
		if message.RunID == runID && message.Status == model.SteeringAccepted {
			out = append(out, message)
		}
	}
	return out, nil
}

func (s *memStore) MarkSteering(_ context.Context, runID string, ids []string, status model.SteeringStatus, at int64, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, message := range s.steering {
		for _, id := range ids {
			if message.RunID == runID && message.ClientMessageID == id {
				message.Status, message.InjectedAt, message.RejectionCode = status, at, code
				s.steering[key] = message
			}
		}
	}
	return nil
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

func (s *memStore) AppendEvent(_ context.Context, event model.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[event.RunID] = append(s.events[event.RunID], event)
	return nil
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

func TestSchedulerPersistsCanonicalEventsAndSingleTerminal(t *testing.T) {
	store := newMemStore()
	engine := NewEngine(store, NewLockManager(), nil, zap.NewNop())
	execution := scriptRunner(func(_ context.Context, emitter workflow.EventEmitter, _ Checkpointer, _ Prompter) workflow.StructuredOutcome {
		emitter.Emit(model.EventMessageReasoning, model.MessageReasoningPayload{
			PublicEventBase: model.NewPublicEventBase("r1"), MessageID: "m1", Text: "先读取当前内容。",
		})
		emitter.Emit(model.EventMessageFinal, model.MessageFinalPayload{
			PublicEventBase: model.NewPublicEventBase("r1"), MessageID: "m2", Text: "分析完成。",
		})
		emitter.Emit(model.EventRunFinished, model.RunFinishedPayload{
			PublicEventBase: model.NewPublicEventBase("r1"), Status: "completed", DurationMS: 10,
		})
		outcome := workflow.StructuredOutcome{Status: workflow.StatusCompleted, Strategy: workflow.StrategyChat}
		return outcome
	})
	run, err := engine.Start(context.Background(), testRun("r1"), execution)
	if err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, run.ID, model.RunDone)
	events, err := store.EventsSince(context.Background(), run.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("events=%+v", events)
	}
	for index, event := range events {
		if event.Seq != int64(index+1) {
			t.Fatalf("non-contiguous seq: %+v", events)
		}
	}
	terminals := 0
	for _, event := range events {
		if event.Type.Terminal() {
			terminals++
		}
	}
	if terminals != 1 {
		t.Fatalf("terminal count=%d", terminals)
	}
	replayed, stop, err := engine.Subscribe(context.Background(), run.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	var got []model.Event
	for event := range replayed {
		got = append(got, event)
	}
	if len(got) != 3 ||
		got[0].Type != model.EventMessageReasoning ||
		got[1].Type != model.EventMessageFinal ||
		got[2].Type != model.EventRunFinished {
		t.Fatalf("replay=%+v", got)
	}
}

func TestSchedulerCancellationProducesCanonicalTerminal(t *testing.T) {
	store := newMemStore()
	engine := NewEngine(store, NewLockManager(), nil, zap.NewNop())
	started := make(chan struct{})
	execution := scriptRunner(func(ctx context.Context, _ workflow.EventEmitter, _ Checkpointer, _ Prompter) workflow.StructuredOutcome {
		close(started)
		<-ctx.Done()
		return workflow.StructuredOutcome{Status: workflow.StatusCanceled, Code: workflow.CodeCanceled}
	})
	if _, err := engine.Start(context.Background(), testRun("cancel"), execution); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := engine.Cancel(context.Background(), "cancel"); err != nil {
		t.Fatal(err)
	}
	if err := engine.Cancel(context.Background(), "cancel"); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, "cancel", model.RunCanceled)
	events, _ := store.EventsSince(context.Background(), "cancel", 0)
	if len(events) != 3 ||
		events[0].Type != model.EventRunStarted ||
		events[1].Type != model.EventRunProgress ||
		events[2].Type != model.EventRunFinished {
		t.Fatalf("events=%+v", events)
	}
}

func TestCancelAuthorityOverridesLateSuccessfulOutcome(t *testing.T) {
	store := newMemStore()
	engine := NewEngine(store, NewLockManager(), nil, zap.NewNop())
	started := make(chan struct{})
	release := make(chan struct{})
	execution := scriptRunner(func(_ context.Context, _ workflow.EventEmitter, _ Checkpointer, _ Prompter) workflow.StructuredOutcome {
		close(started)
		<-release
		return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
	})
	if _, err := engine.Start(context.Background(), testRun("cancel-wins"), execution); err != nil {
		t.Fatal(err)
	}
	<-started
	if _, err := engine.RequestCancel(context.Background(), "cancel-wins"); err != nil {
		t.Fatal(err)
	}
	close(release)
	waitRunStatus(t, store, "cancel-wins", model.RunCanceled)
	events, _ := store.EventsSince(context.Background(), "cancel-wins", 0)
	terminalCount := 0
	for _, event := range events {
		if event.Type == model.EventMessageFinal {
			t.Fatalf("cancel-requested run emitted message.final: %+v", events)
		}
		if event.Type == model.EventRunFinished {
			terminalCount++
			var payload model.RunFinishedPayload
			if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil || payload.Status != "canceled" {
				t.Fatalf("terminal payload=%s err=%v", event.Payload, err)
			}
		}
	}
	if terminalCount != 1 {
		t.Fatalf("terminal count=%d events=%+v", terminalCount, events)
	}
}

func TestCancelInterruptsAskUserWait(t *testing.T) {
	store := newMemStore()
	engine := NewEngine(store, NewLockManager(), nil, zap.NewNop())
	asking := make(chan struct{})
	execution := scriptRunner(func(ctx context.Context, _ workflow.EventEmitter, _ Checkpointer, prompter Prompter) workflow.StructuredOutcome {
		close(asking)
		_, _, err := prompter.Ask(ctx, model.QuestionAskedPayload{
			PublicEventBase: model.NewPublicEventBase("cancel-question"),
			QuestionID:      "q-cancel", Prompt: "choose", Selection: "single",
			Options: []model.QuestionOption{{ID: "a", Label: "A"}},
		})
		if !errors.Is(err, context.Canceled) {
			return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: "BAD_ANSWER"}
		}
		return workflow.StructuredOutcome{Status: workflow.StatusCanceled, Code: workflow.CodeCanceled}
	})
	if _, err := engine.Start(context.Background(), testRun("cancel-question"), execution); err != nil {
		t.Fatal(err)
	}
	<-asking
	waitRunStatus(t, store, "cancel-question", model.RunWaiting)
	if _, err := engine.RequestCancel(context.Background(), "cancel-question"); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, "cancel-question", model.RunCanceled)
	events, _ := store.EventsSince(context.Background(), "cancel-question", 0)
	asked, answered, canceled := 0, 0, 0
	for _, event := range events {
		switch event.Type {
		case model.EventQuestionAsked:
			asked++
		case model.EventQuestionAnswered:
			answered++
		case model.EventRunFinished:
			var payload model.RunFinishedPayload
			_ = json.Unmarshal([]byte(event.Payload), &payload)
			if payload.Status == "canceled" {
				canceled++
			}
		}
	}
	if asked != 1 || answered != 0 || canceled != 1 {
		t.Fatalf("ask cancel lifecycle: asked=%d answered=%d canceled=%d events=%+v", asked, answered, canceled, events)
	}
}

func TestSteeringStateMachineAndIdempotency(t *testing.T) {
	store := newMemStore()
	engine := NewEngine(store, NewLockManager(), nil, zap.NewNop())
	ready := make(chan Checkpointer, 1)
	execution := scriptRunner(func(ctx context.Context, _ workflow.EventEmitter, checkpoint Checkpointer, _ Prompter) workflow.StructuredOutcome {
		ready <- checkpoint
		<-ctx.Done()
		return workflow.StructuredOutcome{Status: workflow.StatusCanceled, Code: workflow.CodeCanceled}
	})
	if _, err := engine.Start(context.Background(), testRun("steering"), execution); err != nil {
		t.Fatal(err)
	}

	// The active handle exists while the run is still pending or entering running.
	first, err := engine.Steer(context.Background(), "steering", "steering", "msg-1", "hash-1", "dark")
	if err != nil || first.Status != model.SteeringAccepted {
		t.Fatalf("pending steering rejected: message=%+v err=%v", first, err)
	}
	replay, err := engine.Steer(context.Background(), "steering", "steering", "msg-1", "hash-1", "dark")
	if err != nil || replay.AcceptedAt != first.AcceptedAt {
		t.Fatalf("same steering request did not replay first result: first=%+v replay=%+v err=%v", first, replay, err)
	}
	_, err = engine.Steer(context.Background(), "steering", "steering", "msg-1", "hash-other", "light")
	assertAgentErrorCode(t, err, "IDEMPOTENCY_KEY_REUSED")

	checkpoint := <-ready
	checkpoint.PhaseChanged(workflow.PhaseExecuting)
	if _, err := engine.Steer(context.Background(), "steering", "steering", "msg-2", "hash-2", "compact"); err != nil {
		t.Fatalf("running steering rejected: %v", err)
	}
	checkpoint.PhaseChanged(workflow.PhaseWaitingInput)
	_, err = engine.Steer(context.Background(), "steering", "steering", "msg-wait", "hash-wait", "answer-like")
	assertAgentErrorCode(t, err, "RUN_WAITING_FOR_ANSWER")
	checkpoint.PhaseChanged(workflow.PhaseCompletionCheck)
	_, err = engine.Steer(context.Background(), "steering", "steering", "msg-complete", "hash-complete", "late")
	assertAgentErrorCode(t, err, "RUN_NOT_STEERABLE")

	if _, err := engine.RequestCancel(context.Background(), "steering"); err != nil {
		t.Fatal(err)
	}
	_, err = engine.Steer(context.Background(), "steering", "steering", "msg-cancel", "hash-cancel", "too late")
	assertAgentErrorCode(t, err, "RUN_CANCELING")
	waitRunStatus(t, store, "steering", model.RunCanceled)
	_, err = engine.Steer(context.Background(), "steering", "steering", "msg-terminal", "hash-terminal", "next")
	assertAgentErrorCode(t, err, "RUN_NOT_STEERABLE")
}

func assertAgentErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	var agentErr *model.AgentError
	if !errors.As(err, &agentErr) || agentErr.Code != code {
		t.Fatalf("got error %v, want AgentError %s", err, code)
	}
}

func TestSchedulerQuestionAskedAnsweredAuthority(t *testing.T) {
	store := newMemStore()
	engine := NewEngine(store, NewLockManager(), nil, zap.NewNop())
	asking := make(chan struct{})
	execution := scriptRunner(func(ctx context.Context, emitter workflow.EventEmitter, _ Checkpointer, prompter Prompter) workflow.StructuredOutcome {
		close(asking)
		answer, display, err := prompter.Ask(ctx, model.QuestionAskedPayload{
			PublicEventBase: model.NewPublicEventBase("question"),
			QuestionID:      "q1", Prompt: "选择方向", Selection: "single",
			Options:     []model.QuestionOption{{ID: "tech", Label: "克制科技"}},
			AllowCustom: true,
		})
		if err != nil || len(answer.SelectedOptionIDs) != 1 || display != "克制科技" {
			return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: "BAD_ANSWER", Message: "answer failed"}
		}
		emitter.Emit(model.EventMessageFinal, model.MessageFinalPayload{
			PublicEventBase: model.NewPublicEventBase("question"), MessageID: "m1", Text: "已继续完成。",
		})
		emitter.Emit(model.EventRunFinished, model.RunFinishedPayload{
			PublicEventBase: model.NewPublicEventBase("question"), Status: "completed", DurationMS: 10,
		})
		return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
	})
	if _, err := engine.Start(context.Background(), testRun("question"), execution); err != nil {
		t.Fatal(err)
	}
	<-asking
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if store.status("question") == model.RunWaiting {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err := engine.InjectInput(context.Background(), "question", `{"selected_option_ids":["missing"],"custom_text":""}`, "q1"); !errors.Is(err, ErrReplyMismatch) {
		t.Fatalf("invalid answer err=%v", err)
	}
	if err := engine.InjectInput(context.Background(), "question", `{"selected_option_ids":["tech"],"custom_text":""}`, "q1"); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, "question", model.RunDone)
	events, _ := store.EventsSince(context.Background(), "question", 0)
	var asked, answered int
	for _, event := range events {
		if event.Type == model.EventQuestionAsked {
			asked++
		}
		if event.Type == model.EventQuestionAnswered {
			answered++
		}
	}
	if asked != 1 || answered != 1 {
		t.Fatalf("events=%+v", events)
	}
}

func testRun(id string) model.Run {
	return model.Run{
		ID: id, ThreadID: "t1", ProjectID: "p1",
		WorkSpec: model.WorkSpec{
			Instruction: "test",
			Target:      model.RunTarget{Artifact: model.ArtifactPresentation, Level: model.TargetDeck},
			Interaction: model.RunInteraction{Intent: model.IntentExecute},
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
