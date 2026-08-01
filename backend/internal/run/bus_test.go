package run

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type memStore2 struct {
	mu sync.Mutex
	ev []model.Event
}

func (s *memStore2) AppendEvent(_ context.Context, event model.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ev = append(s.ev, event)
	return nil
}
func (s *memStore2) EventsSince(context.Context, string, int64) ([]model.Event, error) {
	return nil, nil
}
func (s *memStore2) CreateRun(context.Context, model.Run) error { return nil }
func (s *memStore2) SetRunStatus(context.Context, string, model.RunStatus) error {
	return nil
}
func (s *memStore2) GetRun(context.Context, string) (model.Run, error) { return model.Run{}, nil }

func TestBusHistorySupportsPlanlessAndPlannedStrategies(t *testing.T) {
	writer, dir := newFSWriterForTest(t)
	bus := NewBus("r1", "t1", &memStore2{}, writer)
	events := []struct {
		kind    model.EventType
		payload any
	}{
		{model.EventRunStarted, map[string]any{"user_input": "change title"}},
		{model.EventContextAssembled, map[string]any{"context_id": "ctx"}},
		{model.EventStrategySelected, map[string]any{"strategy": workflow.StrategyDirectAction}},
		{model.EventStatusSummary, map[string]any{"summary": "updated"}},
		{model.EventRunCompleted, workflow.TerminalEvent{Outcome: workflow.StructuredOutcome{
			Status: workflow.StatusCompleted, Strategy: workflow.StrategyDirectAction,
		}}},
	}
	for _, event := range events {
		if err := bus.Emit(context.Background(), event.kind, event.payload); err != nil {
			t.Fatal(err)
		}
	}
	entries := readHistoryLines(t, dir)
	if len(entries) != len(events) {
		t.Fatalf("entries=%+v", entries)
	}
	if entries[0].Type != "user_turn" || entries[2].Type != string(model.EventStrategySelected) ||
		entries[3].Type != "markdown" || entries[4].Type != "final_result" {
		t.Fatalf("history mapping=%+v", entries)
	}
}

func TestBusTerminalGuard(t *testing.T) {
	store := &memStore2{}
	bus := NewBus("r1", "", store, nil)
	_ = bus.Emit(context.Background(), model.EventRunFailed, workflow.TerminalEvent{})
	_ = bus.Emit(context.Background(), model.EventRunCompleted, workflow.TerminalEvent{})
	if len(store.ev) != 1 || store.ev[0].Type != model.EventRunFailed {
		t.Fatalf("events=%+v", store.ev)
	}
}

func newFSWriterForTest(t *testing.T) (*FSHistoryWriter, string) {
	t.Helper()
	dir := t.TempDir()
	locator := &fakeLoc{
		proj: model.Project{ID: "p1", WorkDir: dir},
		thr:  model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl"},
	}
	return NewFSHistoryWriter(locator), dir
}

func readHistoryLines(t *testing.T, dir string) []HistoryEntry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "threads", "t1.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	out := make([]HistoryEntry, 0, len(lines))
	for _, line := range lines {
		var entry HistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		out = append(out, entry)
	}
	return out
}
