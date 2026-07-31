package run

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// memStore2 是 bus 测试独立的最小 Store 实现（避免与 engine_test 共享状态）。
type memStore2 struct {
	mu sync.Mutex
	ev []model.Event
}

func (s *memStore2) AppendEvent(_ context.Context, e model.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ev = append(s.ev, e)
	return nil
}
func (s *memStore2) EventsSince(_ context.Context, _ string, _ int64) ([]model.Event, error) {
	return nil, nil
}
func (s *memStore2) CreateRun(_ context.Context, _ model.Run) error                    { return nil }
func (s *memStore2) SetRunStatus(_ context.Context, _ string, _ model.RunStatus) error { return nil }
func (s *memStore2) GetRun(_ context.Context, _ string) (model.Run, error)             { return model.Run{}, nil }

func newFSWriterForTest(t *testing.T) (*FSHistoryWriter, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "threads"), 0o755); err != nil {
		t.Fatal(err)
	}
	loc := &fakeLoc{
		proj: model.Project{ID: "p1", WorkDir: dir},
		thr:  model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl"},
	}
	return NewFSHistoryWriter(loc), dir
}

func readHistoryLines(t *testing.T, dir string) []HistoryEntry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "threads/t1.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	out := make([]HistoryEntry, 0, len(lines))
	for _, l := range lines {
		var e HistoryEntry
		if err := json.Unmarshal([]byte(l), &e); err != nil {
			t.Fatalf("invalid jsonl line %q: %v", l, err)
		}
		out = append(out, e)
	}
	return out
}

// thought 不持久化（不保存 chain-of-thought）；可见用户与结果事件必须落盘。
func TestBusAppendsWhitelistedEventsToHistory(t *testing.T) {
	hw, dir := newFSWriterForTest(t)
	b := NewBus("r1", "t1", &memStore2{}, hw)

	ctx := context.Background()
	if err := b.Emit(ctx, model.EventRunStarted, harness.RunStartedPayload{RunID: "r1", UserInput: "hi"}); err != nil {
		t.Fatal(err)
	}
	_ = b.Emit(ctx, model.EventThought, harness.ThoughtPayload{Text: "thinking"}) // 不落盘
	_ = b.Emit(ctx, model.EventProgress, harness.ProgressPayload{Stage: "turn"})  // 不落
	_ = b.Emit(ctx, model.EventInfo, harness.InfoPayload{Text: "info line"})
	_ = b.Emit(ctx, model.EventDone, harness.DonePayload{Result: map[string]any{"ok": true}})

	entries := readHistoryLines(t, dir)
	if len(entries) != 3 { // run.started, info, done
		t.Fatalf("expected 3 lines, got %d: %+v", len(entries), entries)
	}
	if entries[0].Turn != "user" || entries[0].Type != "user_turn" || entries[0].Data["text"] != "hi" {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
	if entries[0].RunID != "r1" || entries[0].Seq != 1 {
		t.Fatalf("expected run_id=r1 seq=1, got %+v", entries[0])
	}
	if entries[1].Turn != "agent" || entries[1].Type != "markdown" {
		t.Fatalf("expected info->markdown agent turn, got %+v", entries[1])
	}
	if entries[2].Type != "final_result" || entries[2].Turn != "agent" {
		t.Fatalf("unexpected done entry: %+v", entries[2])
	}
}

// run.started 里 user_input 为空时不落盘（避免污染没有指令的 run，如 lock 失败提前发终态的场景）。
func TestBusSkipsRunStartedWithoutUserInput(t *testing.T) {
	hw, dir := newFSWriterForTest(t)
	b := NewBus("r1", "t1", &memStore2{}, hw)
	_ = b.Emit(context.Background(), model.EventRunStarted, harness.RunStartedPayload{RunID: "r1"})
	entries := readHistoryLines(t, dir)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for empty user_input, got %+v", entries)
	}
}

// hw=nil 时 Bus 正常工作、事件照常持久化到 store（保底：老测试不受影响）。
func TestBusNilHistoryWriterIsNoop(t *testing.T) {
	st := &memStore2{}
	b := NewBus("r1", "t1", st, nil)
	if err := b.Emit(context.Background(), model.EventRunStarted, harness.RunStartedPayload{RunID: "r1", UserInput: "hi"}); err != nil {
		t.Fatal(err)
	}
	if len(st.ev) != 1 {
		t.Fatalf("expected store event when hw=nil, got %d", len(st.ev))
	}
}

// error 事件应落盘（终态一次性）。
func TestBusAppendsErrorEvent(t *testing.T) {
	hw, dir := newFSWriterForTest(t)
	b := NewBus("r1", "t1", &memStore2{}, hw)
	_ = b.Emit(context.Background(), model.EventError, harness.ErrorPayload{Code: "BOOM", Message: "explode"})
	entries := readHistoryLines(t, dir)
	if len(entries) != 1 || entries[0].Type != "error" || entries[0].Data["code"] != "BOOM" {
		t.Fatalf("unexpected error entries: %+v", entries)
	}
}

func TestBusReplaysContextAssembledWithoutSensitiveContent(t *testing.T) {
	hw, dir := newFSWriterForTest(t)
	b := NewBus("r1", "t1", &memStore2{}, hw)
	_ = b.Emit(context.Background(), model.EventContextAssembled, harness.ContextAssembledPayload{
		ContextID: "ctx_1", Profile: "presentation/slide", EstimatedTokens: 1200,
		BudgetTokens: 8000, Segments: 7, Refs: 1, Warnings: []string{"html downgraded"},
	})
	entries := readHistoryLines(t, dir)
	if len(entries) != 1 || entries[0].Type != "context_assembled" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	if _, leaked := entries[0].Data["content"]; leaked {
		t.Fatal("context payload leaked full content into history")
	}
}
