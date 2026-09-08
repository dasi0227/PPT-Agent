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
	if ownerInstanceID != "" && value.OwnerInstanceID != ownerInstanceID {
		return model.Run{}, ErrRunNotRunning
	}
	if !value.Status.Terminal() {
		value.Status, value.OwnerInstanceID = model.RunPaused, ""
		value.PauseReason, value.PausedAt = reason, pausedAt
		s.runs[id] = value
	}
	return value, nil
}

func (s *memStore) ClaimPausedRun(_ context.Context, id, ownerInstanceID string) (model.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.runs[id]
	if !ok || value.Status != model.RunPaused {
		return model.Run{}, ErrRunNotRunning
	}
	value.Status, value.OwnerInstanceID = model.RunRecovering, ownerInstanceID
	value.PauseReason, value.PausedAt = "", 0
	s.runs[id] = value
	return value, nil
}

func (s *memStore) ReleaseRecoveringRun(_ context.Context, id, ownerInstanceID, reason string, pausedAt int64) error {
	_, err := s.PauseRun(context.Background(), id, ownerInstanceID, reason, pausedAt)
	return err
}

func (s *memStore) CancelPausedRun(_ context.Context, id string, canceledAt int64) (model.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.runs[id]
	if !ok || value.Status != model.RunPaused {
		return model.Run{}, ErrRunNotRunning
	}
	value.Status, value.CancelRequestedAt = model.RunCanceled, canceledAt
	value.PauseReason, value.PausedAt = "", 0
	s.runs[id] = value
	return value, nil
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

func TestPauseAllAndResumeAcrossEngineRestart(t *testing.T) {
	store := newMemStore()
	first := NewEngine(store, NewLockManager(), nil, zap.NewNop())
	started := make(chan struct{})
	blocking := scriptRunner(func(ctx context.Context, _ workflow.EventEmitter, _ Checkpointer, _ Prompter) workflow.StructuredOutcome {
		close(started)
		<-ctx.Done()
		return workflow.StructuredOutcome{Status: workflow.StatusCanceled}
	})
	created, err := first.Start(context.Background(), testRun("restartable"), blocking)
	if err != nil {
		t.Fatal(err)
	}
	<-started
	waitRunStatus(t, store, created.ID, model.RunRunning)
	if err := first.PauseAll(context.Background(), "server_shutdown"); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, created.ID, model.RunPaused)
	events, err := store.EventsSince(context.Background(), created.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Type.Terminal() {
			t.Fatalf("pause must not emit a terminal event: %+v", event)
		}
	}

	second := NewEngine(store, NewLockManager(), nil, zap.NewNop())
	if err := second.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	paused, _ := store.GetRun(context.Background(), created.ID)
	resumed, err := second.Resume(context.Background(), paused, scriptRunner(func(context.Context, workflow.EventEmitter, Checkpointer, Prompter) workflow.StructuredOutcome {
		return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
	}))
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != model.RunRecovering {
		t.Fatalf("resume status=%s", resumed.Status)
	}
	waitRunStatus(t, store, created.ID, model.RunDone)
	events, err = store.EventsSince(context.Background(), created.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	resumedEvents := 0
	for _, event := range events {
		if event.Type == model.EventRunResumed {
			resumedEvents++
		}
	}
	if resumedEvents != 1 {
		t.Fatalf("run.resumed count=%d events=%+v", resumedEvents, events)
	}
}

func TestRunCanResumeAfterRepeatedProcessInterruptions(t *testing.T) {
	store := newMemStore()
	blockingExecution := func(callID string, started chan<- struct{}) scriptRunner {
		return func(ctx context.Context, emitter workflow.EventEmitter, _ Checkpointer, _ Prompter) workflow.StructuredOutcome {
			emitter.Emit(model.EventToolStarted, model.ToolStartedPayload{
				PublicEventBase: model.NewPublicEventBase("repeated-resume"),
				CallID:          callID,
				Tool:            "read_ppt",
				Display:         model.PublicDisplay{Label: "正在读取演示内容"},
			})
			close(started)
			<-ctx.Done()
			return workflow.StructuredOutcome{Status: workflow.StatusCanceled}
		}
	}

	first := NewEngine(store, NewLockManager(), nil, zap.NewNop())
	firstStarted := make(chan struct{})
	created, err := first.Start(context.Background(), testRun("repeated-resume"), blockingExecution("first-tool", firstStarted))
	if err != nil {
		t.Fatal(err)
	}
	<-firstStarted
	if err := first.PauseAll(context.Background(), "first_restart"); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, created.ID, model.RunPaused)

	second := NewEngine(store, NewLockManager(), nil, zap.NewNop())
	secondStarted := make(chan struct{})
	paused, _ := store.GetRun(context.Background(), created.ID)
	if _, err := second.Resume(context.Background(), paused, blockingExecution("first-tool", secondStarted)); err != nil {
		t.Fatal(err)
	}
	<-secondStarted
	if _, err := second.Resume(context.Background(), paused, blockingExecution("duplicate-tool", make(chan struct{}))); !errors.Is(err, ErrRunNotRunning) {
		t.Fatalf("duplicate resume must be rejected, got %v", err)
	}
	if err := second.PauseAll(context.Background(), "second_restart"); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, created.ID, model.RunPaused)

	third := NewEngine(store, NewLockManager(), nil, zap.NewNop())
	paused, _ = store.GetRun(context.Background(), created.ID)
	if _, err := third.Resume(context.Background(), paused, scriptRunner(func(_ context.Context, emitter workflow.EventEmitter, _ Checkpointer, _ Prompter) workflow.StructuredOutcome {
		emitter.Emit(model.EventMessageFinal, model.MessageFinalPayload{
			PublicEventBase: model.NewPublicEventBase(created.ID),
			MessageID:       "recovered-final",
			Text:            "恢复完成。",
		})
		return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
	})); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, created.ID, model.RunDone)

	events, err := store.EventsSince(context.Background(), created.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	resumedEvents := 0
	for _, event := range events {
		if event.Type == model.EventRunResumed {
			resumedEvents++
		}
	}
	if resumedEvents != 2 {
		t.Fatalf("run.resumed count=%d events=%+v", resumedEvents, events)
	}
	if events[len(events)-1].Type != model.EventRunCompleted {
		t.Fatalf("recovered run did not persist terminal event: %+v", events)
	}
}

func TestSupersedingPausedRunPersistsCancellationReason(t *testing.T) {
	store := newMemStore()
	first := NewEngine(store, NewLockManager(), nil, zap.NewNop())
	started := make(chan struct{})
	created, err := first.Start(context.Background(), testRun("superseded"), scriptRunner(
		func(ctx context.Context, emitter workflow.EventEmitter, _ Checkpointer, _ Prompter) workflow.StructuredOutcome {
			emitter.Emit(model.EventToolStarted, model.ToolStartedPayload{
				PublicEventBase: model.NewPublicEventBase("superseded"),
				CallID:          "interrupted-tool",
				Tool:            "read_ppt",
				Display:         model.PublicDisplay{Label: "正在读取演示内容"},
			})
			close(started)
			<-ctx.Done()
			return workflow.StructuredOutcome{Status: workflow.StatusCanceled}
		},
	))
	if err != nil {
		t.Fatal(err)
	}
	<-started
	waitRunStatus(t, store, created.ID, model.RunRunning)
	if err := first.PauseAll(context.Background(), "server_shutdown"); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, created.ID, model.RunPaused)

	second := NewEngine(store, NewLockManager(), nil, zap.NewNop())
	if _, err := second.RequestCancelWithReason(context.Background(), created.ID, model.RunCancelSuperseded); err != nil {
		t.Fatal(err)
	}
	events, err := store.EventsSince(context.Background(), created.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Type != model.EventRunCanceled {
			continue
		}
		var payload model.RunTerminalPayload
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Reason != model.RunCancelSuperseded {
			t.Fatalf("cancel reason=%s", payload.Reason)
		}
		return
	}
	t.Fatalf("run.canceled was not emitted: %+v", events)
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
		emitter.Emit(model.EventRunCompleted, model.NewRunTerminalPayload("r1", 10, nil, nil))
		outcome := workflow.StructuredOutcome{Status: workflow.StatusCompleted}
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
		got[2].Type != model.EventRunCompleted {
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
		events[2].Type != model.EventRunCanceled {
		t.Fatalf("events=%+v", events)
	}
}

func TestSchedulerFallbackUsesRuntimeOutcomeDuration(t *testing.T) {
	store := newMemStore()
	engine := NewEngine(store, NewLockManager(), nil, zap.NewNop())
	activeDurationMS := int64(1_234)
	execution := scriptRunner(func(context.Context, workflow.EventEmitter, Checkpointer, Prompter) workflow.StructuredOutcome {
		return workflow.StructuredOutcome{Status: workflow.StatusCompleted, DurationMS: &activeDurationMS}
	})
	if _, err := engine.Start(context.Background(), testRun("fallback-duration"), execution); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, "fallback-duration", model.RunDone)
	events, _ := store.EventsSince(context.Background(), "fallback-duration", 0)
	for _, event := range events {
		if event.Type != model.EventRunCompleted {
			continue
		}
		var payload model.RunTerminalPayload
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.DurationMS != activeDurationMS {
			t.Fatalf("duration=%d want=%d", payload.DurationMS, activeDurationMS)
		}
		return
	}
	t.Fatal("fallback run.completed was not emitted")
}

func TestSchedulerRecoversRuntimePanicAsRunError(t *testing.T) {
	store := newMemStore()
	engine := NewEngine(store, NewLockManager(), nil, zap.NewNop())
	execution := scriptRunner(func(context.Context, workflow.EventEmitter, Checkpointer, Prompter) workflow.StructuredOutcome {
		panic("broken runtime")
	})
	if _, err := engine.Start(context.Background(), testRun("panic"), execution); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, "panic", model.RunFailed)
	events, _ := store.EventsSince(context.Background(), "panic", 0)
	for _, event := range events {
		if event.Type != model.EventRunError {
			continue
		}
		var payload model.RunTerminalPayload
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Error == nil || payload.Error.Code != "INTERNAL" || payload.TraceID != "panic" {
			t.Fatalf("run.error payload=%+v", payload)
		}
		return
	}
	t.Fatalf("run.error was not emitted: %+v", events)
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
		if event.Type == model.EventRunCanceled {
			terminalCount++
			var payload model.RunTerminalPayload
			if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
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
			QuestionID:      "q-cancel", Questions: []model.QuestionField{{
				ID: "choice", Title: "choose", Options: []model.QuestionOption{{ID: "a", Label: "A"}},
			}},
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
		case model.EventRunCanceled:
			canceled++
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
			QuestionID:      "q1", Questions: []model.QuestionField{{
				ID: "direction", Title: "选择方向",
				Options: []model.QuestionOption{{ID: "tech", Label: "克制科技"}}, AllowCustom: true,
			}},
		})
		if err != nil || len(answer.Answers) != 1 || display != "Q：选择方向\nA：克制科技" {
			return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: "BAD_ANSWER", Message: "answer failed"}
		}
		emitter.Emit(model.EventMessageFinal, model.MessageFinalPayload{
			PublicEventBase: model.NewPublicEventBase("question"), MessageID: "m1", Text: "已继续完成。",
		})
		emitter.Emit(model.EventRunCompleted, model.NewRunTerminalPayload("question", 10, nil, nil))
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
	if err := engine.InjectInput(context.Background(), "question", `{"answers":[{"question_id":"direction","selected_option_id":"missing"}]}`, "q1"); !errors.Is(err, ErrReplyMismatch) {
		t.Fatalf("invalid answer err=%v", err)
	}
	if err := engine.InjectInput(context.Background(), "question", `{"answers":[{"question_id":"direction","selected_option_id":"tech"}]}`, "q1"); err != nil {
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

func TestSchedulerCommandPermissionAuthority(t *testing.T) {
	store := newMemStore()
	engine := NewEngine(store, NewLockManager(), nil, zap.NewNop())
	asking := make(chan struct{})
	execution := scriptRunner(func(ctx context.Context, emitter workflow.EventEmitter, _ Checkpointer, prompter Prompter) workflow.StructuredOutcome {
		commandPrompter, ok := prompter.(interface {
			AskCommandPermission(context.Context, model.CommandPermissionRequestedPayload) (model.CommandPermissionAnswer, error)
		})
		if !ok {
			return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: "PROMPTER_MISSING"}
		}
		close(asking)
		answer, err := commandPrompter.AskCommandPermission(ctx, model.CommandPermissionRequestedPayload{
			PublicEventBase: model.NewPublicEventBase("command-permission"),
			InteractionID:   "cmdperm_call-1",
			CallID:          "call-1",
			Command:         "cat .env",
			CommandHash:     "hash-1",
			ReasonCode:      "COMMAND_SENSITIVE_READ",
			Reason:          "sensitive file",
		})
		if err != nil || answer.Decision != "allow_once" {
			return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: "BAD_PERMISSION"}
		}
		emitter.Emit(model.EventMessageFinal, model.MessageFinalPayload{
			PublicEventBase: model.NewPublicEventBase("command-permission"), MessageID: "m1", Text: "done",
		})
		emitter.Emit(model.EventRunCompleted, model.NewRunTerminalPayload("command-permission", 10, nil, nil))
		return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
	})
	if _, err := engine.Start(context.Background(), testRun("command-permission"), execution); err != nil {
		t.Fatal(err)
	}
	<-asking
	waitRunStatus(t, store, "command-permission", model.RunWaiting)
	mismatch := model.CommandPermissionAnswer{
		InteractionID: "cmdperm_call-1", CallID: "call-1",
		CommandHash: "stale-hash", Decision: "allow_once",
	}
	if err := engine.SubmitCommandPermission(context.Background(), "command-permission", mismatch); !errors.Is(err, ErrReplyMismatch) {
		t.Fatalf("stale command permission err=%v", err)
	}
	answer := mismatch
	answer.CommandHash = "hash-1"
	if err := engine.SubmitCommandPermission(context.Background(), "command-permission", answer); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, "command-permission", model.RunDone)
	events, _ := store.EventsSince(context.Background(), "command-permission", 0)
	requestedIndex, answeredIndex := -1, -1
	for index, event := range events {
		switch event.Type {
		case model.EventCommandPermissionRequested:
			requestedIndex = index
		case model.EventCommandPermissionAnswered:
			answeredIndex = index
		}
	}
	if requestedIndex < 0 || answeredIndex <= requestedIndex {
		t.Fatalf("command permission events are missing or unordered: %+v", events)
	}
}

func testRun(id string) model.Run {
	return model.Run{
		ID: id, ThreadID: "t1", ProjectID: "p1",
		Command: model.RunCommand{
			Instruction: "test",
			Scope:       model.NewRunScope(model.ScopeObjectPresentation, model.ScopeAllPages),
			Mode:        model.ModeExecute,
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
