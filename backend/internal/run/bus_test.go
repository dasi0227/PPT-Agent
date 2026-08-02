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

func TestBusPersistsSafePublicHistoryButExcludesProgress(t *testing.T) {
	writer, dir := newFSWriterForTest(t)
	store := &memStore2{}
	bus := NewBus("r1", "t1", store, writer)
	base := func() model.PublicEventBase { return model.NewPublicEventBase("r1") }
	events := []struct {
		kind    model.EventType
		payload any
	}{
		{model.EventRunStarted, model.RunStartedPayload{
			PublicEventBase: base(), Target: model.RunTarget{Artifact: model.ArtifactPresentation, Level: model.TargetDeck},
			Interaction: model.RunInteraction{Intent: model.IntentExecute}, UserInput: "change title",
		}},
		{model.EventRunProgress, model.RunProgressPayload{PublicEventBase: base(), Stage: "thinking", Text: "正在分析"}},
		{model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: base(), Plan: model.PublicPlan{
			PlanID: "p1", Revision: 1, Steps: []model.PublicPlanStep{{ID: "s1", Title: "生成", Status: "in_progress"}},
		}}},
		{model.EventMessageReasoning, model.MessageReasoningPayload{PublicEventBase: base(), MessageID: "m1", Text: "先确认全局设计。"}},
		{model.EventToolStarted, model.ToolStartedPayload{
			PublicEventBase: base(), CallID: "c1", Tool: "read_ppt", Display: model.PublicDisplay{Label: "读取 PPT"},
		}},
		{model.EventToolCompleted, model.ToolCompletedPayload{
			PublicEventBase: base(), CallID: "c1", Tool: "read_ppt", Status: "completed", Display: model.PublicDisplay{Label: "已读取 PPT"},
		}},
		{model.EventQuestionAsked, model.QuestionAskedPayload{
			PublicEventBase: base(), QuestionID: "q1", Prompt: "选择风格", Selection: "single",
			Options: []model.QuestionOption{{ID: "tech", Label: "科技"}}, AllowCustom: true,
		}},
		{model.EventQuestionAnswered, model.QuestionAnsweredPayload{
			PublicEventBase: base(), QuestionID: "q1",
			Answer: model.QuestionAnswer{SelectedOptionIDs: []string{"tech"}}, DisplayText: "科技",
		}},
		{model.EventMessageFinal, model.MessageFinalPayload{PublicEventBase: base(), MessageID: "m2", Text: "已完成。"}},
		{model.EventRunFinished, model.RunFinishedPayload{PublicEventBase: base(), Status: "completed", DurationMS: 10}},
	}
	for _, event := range events {
		if err := bus.Emit(context.Background(), event.kind, event.payload); err != nil {
			t.Fatalf("%s: %v", event.kind, err)
		}
	}
	if len(store.ev) != len(events) {
		t.Fatalf("event store=%+v", store.ev)
	}
	for index, event := range store.ev {
		if event.Seq != int64(index+1) {
			t.Fatalf("non-contiguous seq: %+v", store.ev)
		}
	}
	entries := readHistoryLines(t, dir)
	if len(entries) != len(events)-1 {
		t.Fatalf("history=%+v", entries)
	}
	for _, entry := range entries {
		if entry.Type == string(model.EventRunProgress) {
			t.Fatal("run.progress entered thread history")
		}
	}
	if entries[0].Type != "user_turn" || entries[len(entries)-1].Type != string(model.EventRunFinished) {
		t.Fatalf("history mapping=%+v", entries)
	}
}

func TestBusEnforcesPublicSequenceInvariants(t *testing.T) {
	ctx := context.Background()
	base := func() model.PublicEventBase { return model.NewPublicEventBase("r1") }
	store := &memStore2{}
	bus := NewBus("r1", "", store, nil)
	if err := bus.Emit(ctx, model.EventRunProgress, model.RunProgressPayload{PublicEventBase: base(), Stage: "thinking", Text: "x"}); err == nil {
		t.Fatal("accepted an event before run.started")
	}
	if err := bus.Emit(ctx, model.EventRunStarted, model.RunStartedPayload{
		PublicEventBase: base(), Target: model.RunTarget{Artifact: model.ArtifactPresentation, Level: model.TargetDeck},
		Interaction: model.RunInteraction{Intent: model.IntentExecute}, UserInput: "go",
	}); err != nil {
		t.Fatal(err)
	}
	if err := bus.Emit(ctx, model.EventPlanUpdated, model.PlanUpdatedPayload{
		PublicEventBase: base(), Plan: model.PublicPlan{
			PlanID: "p1", Revision: 1,
			Steps: []model.PublicPlanStep{{ID: "s1", Title: "完成", Status: "completed"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := bus.Emit(ctx, model.EventPlanUpdated, model.PlanUpdatedPayload{
		PublicEventBase: base(), Plan: model.PublicPlan{
			PlanID: "p1", Revision: 2,
			Steps: []model.PublicPlanStep{{ID: "s1", Title: "完成", Status: "pending"}},
		},
	}); err == nil {
		t.Fatal("accepted a regressed completed plan step")
	}
	if err := bus.Emit(ctx, model.EventToolCompleted, model.ToolCompletedPayload{
		PublicEventBase: base(), CallID: "missing", Tool: "read_ppt", Status: "completed",
		Display: model.PublicDisplay{Label: "完成"},
	}); err == nil {
		t.Fatal("accepted unmatched tool.completed")
	}
	if err := bus.Emit(ctx, model.EventToolStarted, model.ToolStartedPayload{
		PublicEventBase: base(), CallID: "mismatch", Tool: "read_ppt",
		Display: model.PublicDisplay{Label: "读取"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := bus.Emit(ctx, model.EventToolCompleted, model.ToolCompletedPayload{
		PublicEventBase: base(), CallID: "mismatch", Tool: "write_ppt", Status: "completed",
		Display: model.PublicDisplay{Label: "完成"},
	}); err == nil {
		t.Fatal("accepted a tool.completed with a different tool name")
	}
	if err := bus.Emit(ctx, model.EventQuestionAnswered, model.QuestionAnsweredPayload{
		PublicEventBase: base(), QuestionID: "missing",
		Answer: model.QuestionAnswer{CustomText: "x"}, DisplayText: "x",
	}); err == nil {
		t.Fatal("accepted unmatched question.answered")
	}
	if err := bus.Emit(ctx, model.EventRunFinished, model.RunFinishedPayload{
		PublicEventBase: base(), Status: "completed", DurationMS: 1,
	}); err == nil {
		t.Fatal("accepted completed terminal without message.final")
	}
	if err := bus.Emit(ctx, model.EventMessageFinal, model.MessageFinalPayload{
		PublicEventBase: base(), MessageID: "m1", Text: "done",
	}); err != nil {
		t.Fatal(err)
	}
	if err := bus.Emit(ctx, model.EventRunFinished, model.RunFinishedPayload{
		PublicEventBase: base(), Status: "completed", DurationMS: 1,
	}); err != nil {
		t.Fatal(err)
	}
	_ = bus.Emit(ctx, model.EventMessageReasoning, model.MessageReasoningPayload{
		PublicEventBase: base(), MessageID: "late", Text: "late",
	})
	if len(store.ev) != 5 {
		t.Fatalf("terminal guard events=%+v", store.ev)
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
