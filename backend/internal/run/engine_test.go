package run

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type memStore struct {
	mu     sync.Mutex
	runs   map[string]model.Run
	events map[string][]model.Event
}

func newMemStore() *memStore {
	return &memStore{runs: map[string]model.Run{}, events: map[string][]model.Event{}}
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
		emitter.Emit(model.EventRunStarted, map[string]any{"user_input": "explain"})
		emitter.Emit(model.EventStrategySelected, map[string]any{"strategy": workflow.StrategyRespond})
		outcome := workflow.StructuredOutcome{Status: workflow.StatusCompleted, Strategy: workflow.StrategyRespond}
		emitter.Emit(model.EventRunCompleted, workflow.TerminalEvent{Outcome: outcome})
		return outcome
	})
	run, err := engine.Start(context.Background(), model.Run{ID: "r1", ThreadID: "t1", ProjectID: "p1"}, execution)
	if err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, run.ID, model.RunDone)
	events, err := store.EventsSince(context.Background(), run.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
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
	if len(got) != 2 || got[0].Type != model.EventStrategySelected || got[1].Type != model.EventRunCompleted {
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
		return workflow.StructuredOutcome{Status: workflow.StatusCanceled, Code: workflow.CodeWorkflowCanceled}
	})
	if _, err := engine.Start(context.Background(), model.Run{ID: "cancel", ProjectID: "p1"}, execution); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := engine.Cancel(context.Background(), "cancel"); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, "cancel", model.RunCanceled)
	events, _ := store.EventsSince(context.Background(), "cancel", 0)
	if len(events) != 1 || events[0].Type != model.EventRunCanceled {
		t.Fatalf("events=%+v", events)
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
