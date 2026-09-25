package run

import (
	"context"
	"sync"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestBusRestoreContinuesPersistedSequenceWithoutSecondRunStarted(t *testing.T) {
	store := &memStore2{}
	first := NewBus("resume-run", "", store)
	base := model.NewPublicEventBase("resume-run")
	if err := first.Emit(context.Background(), model.EventRunStarted, model.RunStartedPayload{
		PublicEventBase: base,
		Scope:           model.NewRunScope(model.ScopeAllPages),
		Mode:            model.ModePlan, UserInput: "plan",
	}); err != nil {
		t.Fatal(err)
	}
	if err := first.Emit(context.Background(), model.EventPlanUpdated, model.PlanUpdatedPayload{
		PublicEventBase: base,
		Plan:            model.PublicPlan{PlanID: "p1", Title: "计划", Content: "完整计划", Status: "awaiting_approval", Steps: []model.PublicPlanStep{{ID: "s1", Title: "执行", Status: "pending"}}},
	}); err != nil {
		t.Fatal(err)
	}
	restored := NewBus("resume-run", "", store)
	if err := restored.Restore(store.ev); err != nil {
		t.Fatal(err)
	}
	if err := restored.Emit(context.Background(), model.EventPlanApprovalRequested, model.PlanApprovalRequestedPayload{
		PublicEventBase: base, InteractionID: "i1",
		Plan: model.PublicPlan{PlanID: "p1", Title: "计划", Content: "完整计划", Status: "awaiting_approval", Steps: []model.PublicPlanStep{{ID: "s1", Title: "执行", Status: "pending"}}},
	}); err != nil {
		t.Fatal(err)
	}
	if got := store.ev[len(store.ev)-1]; got.Seq != 3 || got.Type != model.EventPlanApprovalRequested {
		t.Fatalf("restored event=%+v", got)
	}
}

func TestBusRestoredApprovalPublicationsAreIdempotent(t *testing.T) {
	ctx := context.Background()
	store := &memStore2{}
	bus := NewBus("approved-run", "thread", store)
	base := func() model.PublicEventBase { return model.NewPublicEventBase("approved-run") }
	plan := model.PublicPlan{PlanID: "plan", Title: "已批准", Content: "计划正文", Status: "active", Steps: []model.PublicPlanStep{{ID: "step", Title: "执行", Status: "pending"}}}
	previous := model.NewRunScope(model.ScopeCustomPages, "sli_one")
	approved := model.NewRunScope(model.ScopeCustomPages, "sli_one", "sli_two")
	for _, event := range []struct {
		kind    model.EventType
		payload any
	}{
		{model.EventRunStarted, model.RunStartedPayload{PublicEventBase: base(), Scope: previous, Mode: model.ModePlan, UserInput: "规划"}},
		{model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: base(), Plan: plan, InteractionID: "plan-approval"}},
		{model.EventRunModeChanged, model.RunModeChangedPayload{PublicEventBase: base(), PreviousMode: model.ModePlan, Mode: model.ModeExecute}},
		{model.EventScopeUpdated, model.ScopeUpdatedPayload{PublicEventBase: base(), PreviousScope: previous, Scope: approved, Cause: "user_approved_expansion", InteractionID: "scope-approval"}},
	} {
		if err := bus.Emit(ctx, event.kind, event.payload); err != nil {
			t.Fatal(err)
		}
	}
	restored := NewBus("approved-run", "thread", store)
	if err := restored.Restore(store.ev); err != nil {
		t.Fatal(err)
	}
	for _, event := range []struct {
		kind    model.EventType
		payload any
	}{
		{model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: base(), Plan: plan, InteractionID: "plan-approval"}},
		{model.EventRunModeChanged, model.RunModeChangedPayload{PublicEventBase: base(), PreviousMode: model.ModePlan, Mode: model.ModeExecute}},
		{model.EventScopeUpdated, model.ScopeUpdatedPayload{PublicEventBase: base(), PreviousScope: previous, Scope: approved, Cause: "user_approved_expansion", InteractionID: "scope-approval"}},
	} {
		if err := restored.Emit(ctx, event.kind, event.payload); err != nil {
			t.Fatal(err)
		}
	}
	if len(store.ev) != 4 {
		t.Fatalf("restored transition was appended twice: %+v", store.ev)
	}
	// Later plan edits are independent of the approval's publication identity.
	if err := restored.Emit(ctx, model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: base(), Plan: plan}); err != nil {
		t.Fatal(err)
	}
	if len(store.ev) != 5 {
		t.Fatal("unrelated plan update was suppressed")
	}
}

type memStore2 struct {
	mu sync.Mutex
	ev []model.Event
}

func (s *memStore2) AppendEvent(_ context.Context, event *model.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ev = append(s.ev, *event)
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
func (s *memStore2) RequestRunCancel(context.Context, string, int64) (model.Run, error) {
	return model.Run{}, nil
}
func (s *memStore2) CreateSteering(context.Context, model.SteeringMessage) (model.SteeringMessage, bool, error) {
	return model.SteeringMessage{}, true, nil
}
func (s *memStore2) ListPendingSteering(context.Context, string) ([]model.SteeringMessage, error) {
	return nil, nil
}
func (s *memStore2) MarkSteering(context.Context, string, []string, model.SteeringStatus, int64, string) error {
	return nil
}

func TestBusPersistsPublicEventsInOrder(t *testing.T) {
	store := &memStore2{}
	bus := NewBus("r1", "t1", store)
	base := func() model.PublicEventBase { return model.NewPublicEventBase("r1") }
	events := []struct {
		kind    model.EventType
		payload any
	}{
		{model.EventRunStarted, model.RunStartedPayload{
			PublicEventBase: base(), Scope: model.NewRunScope(model.ScopeAllPages),
			Mode: model.ModeExecute, UserInput: "change title",
		}},
		{model.EventRunProgress, model.RunProgressPayload{PublicEventBase: base(), Activity: model.ActivityRunAnalyzing}},
		{model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: base(), Plan: model.PublicPlan{
			PlanID: "p1", Title: "计划", Content: "完整计划", Status: "active",
			Steps: []model.PublicPlanStep{{ID: "s1", Title: "生成", Status: "in_progress"}},
		}}},
		{model.EventMessageReasoning, model.MessageReasoningPayload{PublicEventBase: base(), MessageID: "m1", Text: "先确认视觉要求。"}},
		{model.EventToolStarted, model.ToolStartedPayload{
			PublicEventBase: base(), CallID: "c1", Tool: "read_ppt", Display: model.PublicDisplay{Label: "读取 PPT"},
		}},
		{model.EventToolCompleted, model.ToolCompletedPayload{
			PublicEventBase: base(), CallID: "c1", Tool: "read_ppt", Status: "completed", Display: model.PublicDisplay{Label: "已读取 PPT"},
		}},
		{model.EventQuestionAsked, model.QuestionAskedPayload{
			PublicEventBase: base(), QuestionID: "q1", Questions: []model.QuestionField{{
				ID: "style", Title: "选择风格",
				Options: []model.QuestionOption{{ID: "tech", Label: "科技"}}, AllowCustom: true,
			}},
		}},
		{model.EventQuestionAnswered, model.QuestionAnsweredPayload{
			PublicEventBase: base(), QuestionID: "q1",
			Answer: model.QuestionAnswer{Answers: []model.QuestionFieldAnswer{{QuestionID: "style", SelectedOptionID: "tech"}}}, DisplayText: "科技",
		}},
		{model.EventMessageFinal, model.MessageFinalPayload{PublicEventBase: base(), MessageID: "m2", Text: "已完成。", AffectedTargets: []model.PublicTarget{}, SuggestedNextInputs: []string{}}},
		{model.EventRunCompleted, model.NewRunTerminalPayloadFromBase(base(), 10, nil, nil)},
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

}

func TestBusEnforcesPublicSequenceInvariants(t *testing.T) {
	ctx := context.Background()
	base := func() model.PublicEventBase { return model.NewPublicEventBase("r1") }
	store := &memStore2{}
	bus := NewBus("r1", "", store)
	if err := bus.Emit(ctx, model.EventRunProgress, model.RunProgressPayload{PublicEventBase: base(), Activity: model.ActivityRunAnalyzing}); err == nil {
		t.Fatal("accepted an event before run.started")
	}
	if err := bus.Emit(ctx, model.EventRunStarted, model.RunStartedPayload{
		PublicEventBase: base(), Scope: model.NewRunScope(model.ScopeAllPages),
		Mode: model.ModeExecute, UserInput: "go",
	}); err != nil {
		t.Fatal(err)
	}
	if err := bus.Emit(ctx, model.EventPlanUpdated, model.PlanUpdatedPayload{
		PublicEventBase: base(), Plan: model.PublicPlan{
			PlanID: "p1", Title: "计划", Content: "完整计划", Status: "completed",
			Steps: []model.PublicPlanStep{{ID: "s1", Title: "完成", Status: "completed"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := bus.Emit(ctx, model.EventPlanUpdated, model.PlanUpdatedPayload{
		PublicEventBase: base(), Plan: model.PublicPlan{
			PlanID: "p1", Title: "计划", Content: "完整计划", Status: "active",
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
		PublicEventBase: base(), CallID: "mismatch", Tool: "mutate_ppt", Status: "completed",
		Display: model.PublicDisplay{Label: "完成"},
	}); err == nil {
		t.Fatal("accepted a tool.completed with a different tool name")
	}
	if err := bus.Emit(ctx, model.EventToolCompleted, model.ToolCompletedPayload{
		PublicEventBase: base(), CallID: "mismatch", Tool: "read_ppt", Status: "completed",
		Display: model.PublicDisplay{Label: "完成"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := bus.Emit(ctx, model.EventQuestionAnswered, model.QuestionAnsweredPayload{
		PublicEventBase: base(), QuestionID: "missing",
		Answer: model.QuestionAnswer{Answers: []model.QuestionFieldAnswer{{QuestionID: "detail", CustomText: "x"}}}, DisplayText: "x",
	}); err == nil {
		t.Fatal("accepted unmatched question.answered")
	}
	if err := bus.Emit(ctx, model.EventRunCompleted, model.NewRunTerminalPayloadFromBase(base(), 1, nil, nil)); err == nil {
		t.Fatal("accepted completed terminal without message.final")
	}
	if err := bus.Emit(ctx, model.EventMessageFinal, model.MessageFinalPayload{
		PublicEventBase: base(), MessageID: "m1", Text: "done",
		AffectedTargets: []model.PublicTarget{}, SuggestedNextInputs: []string{},
	}); err != nil {
		t.Fatal(err)
	}
	if err := bus.Emit(ctx, model.EventRunCompleted, model.NewRunTerminalPayloadFromBase(base(), 1, nil, nil)); err != nil {
		t.Fatal(err)
	}
	_ = bus.Emit(ctx, model.EventMessageReasoning, model.MessageReasoningPayload{
		PublicEventBase: base(), MessageID: "late", Text: "late",
	})
	if len(store.ev) != 6 {
		t.Fatalf("terminal guard events=%+v", store.ev)
	}
}
