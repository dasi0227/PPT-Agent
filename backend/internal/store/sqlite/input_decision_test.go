package sqlite

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"gorm.io/gorm"
)

func TestConversationBeforeInputValidatesAcceptanceAndExcludesFuture(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := sessionstate.WithRequestScene(context.Background(), 1)
	steeringForTest(t, s, "r", "earlier", "继续")
	steeringForTest(t, s, "r", "thanks", "你好")
	conversationMessageForTest(t, s, "r", "你好，我是 Dasi", true)
	accepted := steeringForTest(t, s, "r", "current", "继续")
	steeringForTest(t, s, "r", "future", "未来要求")
	conversationMessageForTest(t, s, "r", "未来最终回复", true)
	got, err := s.LoadThreadConversationContextBeforeInput(ctx, "t", "r", "current")
	if err != nil || len(got.Rounds) != 1 || got.Input == nil || got.Input.AcceptedSeq != accepted.AcceptedSeq || got.Input.Text != "继续" {
		t.Fatalf("input cutoff lost: %+v %v", got, err)
	}
	current, err := s.GetRun(ctx, "r")
	if err != nil {
		t.Fatal(err)
	}
	want := []model.ConversationMessage{{Role: "user", Text: current.Command.Instruction}, {Role: "user", Text: "继续"}, {Role: "user", Text: "你好"}, {Role: "assistant", Text: "你好，我是 Dasi"}}
	if !reflect.DeepEqual(got.Rounds[0].Messages, want) {
		t.Fatalf("future content crossed accepted cutoff: %+v", got)
	}
	for _, id := range []string{"unknown", "initial:earlier"} {
		if _, err := s.LoadThreadConversationContextBeforeInput(ctx, "t", "r", id); err == nil {
			t.Fatalf("unaccepted input %s", id)
		}
	}
	before, err := s.LoadThreadConversationContextBeforeInput(ctx, "t", "r", "initial:"+current.ClientRequestID)
	if err != nil || len(before.Rounds) != 0 {
		t.Fatalf("initial input saw itself: %+v %v", before, err)
	}
}

func TestChatCompletionDurablySuppressesOnlyAutomaticSuggestions(t *testing.T) {
	s, cp := checkpointFixture(t)
	payload := model.NewRunTerminalPayload(cp.RunID, 0, nil, nil)
	payload.SuppressSuggestions = true
	runUpdateForTest(t, s, cp.RunID, model.EventMessageFinal, model.MessageFinalPayload{PublicEventBase: model.NewPublicEventBase(cp.RunID), MessageID: "final-chat", Text: "chat", AffectedTargets: []model.PublicTarget{}})
	runUpdateForTest(t, s, cp.RunID, model.EventRunCompleted, payload)
	var thread threadPO
	if err := s.db.First(&thread, "id = ?", "t").Error; err != nil {
		t.Fatal(err)
	}
	thread.AutoSuggestEnabled = 1
	automatic := model.CommandExecution{Kind: "suggest", Source: "automatic", RunID: cp.RunID}
	if err := s.db.Transaction(func(tx *gorm.DB) error { return prepareSuggestCommand(tx, &automatic, thread) }); !errors.Is(err, context.Canceled) {
		t.Fatalf("chat reply triggered automatic generation: %v", err)
	}
	manual := model.CommandExecution{Kind: "suggest", Source: "manual", RunID: cp.RunID}
	if err := s.db.Transaction(func(tx *gorm.DB) error { return prepareSuggestCommand(tx, &manual, thread) }); err != nil {
		t.Fatalf("manual behavior changed: %v", err)
	}
}

func TestInputStopFencesLateResultsAndBothContinuationClaims(t *testing.T) {
	for _, disposition := range []string{model.InputDisclosure, model.InputBypass, model.InputIrrelevant, model.InputDetectionFailed} {
		for _, paused := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/paused=%t", disposition, paused), func(t *testing.T) {
				s, cp := checkpointFixture(t)
				ctx := sessionstate.WithRequestScene(context.Background(), 1)
				cp.Boundary, cp.ContinuationAllowed = "terminal", true
				if err := s.SaveCheckpoint(ctx, cp); err != nil {
					t.Fatal(err)
				}
				current, err := s.GetRun(ctx, cp.RunID)
				if err != nil {
					t.Fatal(err)
				}
				result := model.InputDecision{InputID: "initial:" + current.ClientRequestID, Fingerprint: "material", Identity: "rules-model", ExecutionRevision: current.ExecutionRevision, Disposition: disposition}
				if disposition == model.InputIrrelevant {
					result.InputID = "steering-input"
					steeringForTest(t, s, current.ID, result.InputID, "不可执行的追加输入")
				}
				late := current
				late.ExecutionRevision++
				lateResult := result
				lateResult.ExecutionRevision = late.ExecutionRevision
				if _, err := s.SaveInputDecision(ctx, late, lateResult); !errors.Is(err, run.ErrRunRevisionConflict) {
					t.Fatalf("late result accepted: %v", err)
				}
				if _, err := s.SaveInputDecision(ctx, current, result); err != nil {
					t.Fatal(err)
				}
				passed := result
				passed.Disposition = model.InputTask
				if actual, err := s.SaveInputDecision(ctx, current, passed); err != nil || !actual.StopsRun() {
					t.Fatalf("stop was overwritten by another result: %+v %v", actual, err)
				}
				if paused {
					if _, err := s.PauseRun(ctx, current.ID, current.OwnerInstanceID, "shutdown", 1); err != nil {
						t.Fatal(err)
					}
					if _, err := resumeRunForTest(t, s, current.ID, "new-owner", "resume"); !errors.Is(err, run.ErrRunNotContinuable) {
						t.Fatalf("paused stop resumed: %v", err)
					}
				} else {
					if err := s.SetRunStatus(ctx, current.ID, model.RunCanceled); err != nil {
						t.Fatal(err)
					}
					current, err = s.GetRun(ctx, current.ID)
					if err != nil || current.CanContinue {
						t.Fatalf("stopped run continuable: %+v %v", current, err)
					}
					if _, err := resumeRunForTest(t, s, current.ID, "new-owner", "resume"); !errors.Is(err, run.ErrRunNotContinuable) {
						t.Fatalf("terminal stop resumed: %v", err)
					}
				}
			})
		}
	}
}

func TestNamingCountsOnlyTaskInputsButSharesConversationContext(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := sessionstate.WithRequestScene(context.Background(), 1)
	enabled := true
	if _, err := s.UpdateThreadNamingState(ctx, "t", nil, &enabled, true, 1); err != nil {
		t.Fatal(err)
	}
	current, err := s.GetRun(ctx, "r")
	if err != nil {
		t.Fatal(err)
	}
	excluded := []string{model.InputChat, model.InputIrrelevant, model.InputDisclosure, model.InputBypass, model.InputDetectionFailed}
	for _, disposition := range excluded {
		input := model.SteeringMessage{RunID: current.ID, ThreadID: "t", ClientMessageID: disposition, RequestHash: disposition, Content: "排除:" + disposition, Scope: current.Command.Scope}
		if _, _, err := s.CreateSteering(ctx, input); err != nil {
			t.Fatal(err)
		}
		decision := model.InputDecision{InputID: input.ClientMessageID, Fingerprint: disposition, Disposition: disposition, ExecutionRevision: current.ExecutionRevision}
		if _, err := s.SaveInputDecision(ctx, current, decision); err != nil {
			t.Fatal(err)
		}
		if _, trigger, err := s.RecordThreadNamingInput(ctx, model.ThreadNamingInput{SceneRevision: current.SceneRevision, ThreadID: "t", RunID: current.ID, InputID: decision.InputID, ExecutionRevision: current.ExecutionRevision}); err != nil || trigger {
			t.Fatalf("%s triggered naming: %v", disposition, err)
		}
	}
	thread, err := s.GetThread(ctx, "t")
	if err != nil || thread.RenameFirstInputSeen || thread.RenameInputCount != 0 {
		t.Fatalf("non-task inputs consumed naming eligibility: %+v %v", thread, err)
	}
	initialID := "initial:" + current.ClientRequestID
	if _, err := s.SaveInputDecision(ctx, current, model.InputDecision{InputID: initialID, Fingerprint: "task", Disposition: model.InputTask, ExecutionRevision: current.ExecutionRevision}); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		thread, trigger, err := s.RecordThreadNamingInput(ctx, model.ThreadNamingInput{SceneRevision: current.SceneRevision, ThreadID: "t", RunID: current.ID, InputID: initialID, ExecutionRevision: current.ExecutionRevision})
		if err != nil || trigger != (attempt == 0) || !thread.RenameFirstInputSeen || thread.RenameInputCount != 0 {
			t.Fatalf("initial task input was lost or counted twice: %+v %v", thread, err)
		}
	}
	input := model.SteeringMessage{RunID: current.ID, ThreadID: "t", ClientMessageID: "task-extra", RequestHash: "task-extra", Content: "修改演示的图表", Scope: current.Command.Scope}
	if _, _, err := s.CreateSteering(ctx, input); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveInputDecision(ctx, current, model.InputDecision{InputID: input.ClientMessageID, Fingerprint: "extra", Disposition: model.InputTask, ExecutionRevision: current.ExecutionRevision}); err != nil {
		t.Fatal(err)
	}
	thread, trigger, err := s.RecordThreadNamingInput(ctx, model.ThreadNamingInput{SceneRevision: current.SceneRevision, ThreadID: "t", RunID: current.ID, InputID: input.ClientMessageID, ExecutionRevision: current.ExecutionRevision})
	if err != nil || !trigger || thread.RenameInputCount != 1 {
		t.Fatalf("task steering was not counted once: %+v %v", thread, err)
	}
	plan := model.PublicPlan{Steps: []model.PublicPlanStep{{Title: "保留给后续建议", Status: "processing"}}}
	runUpdateForTest(t, s, current.ID, model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: model.NewPublicEventBase(current.ID), Plan: &plan})
	source, err := s.LoadThreadConversationContext(ctx, "t")
	if err != nil || len(source.Rounds) != 1 || len(source.Rounds[0].Messages) != len(excluded)+2 || !reflect.DeepEqual(source.Plan, &plan) {
		t.Fatalf("shared context filtered user inputs or lost its separate plan: %+v %v", source, err)
	}
	task, err := s.LoadTaskConversationBeforeInput(ctx, "t", current.ID, input.ClientMessageID)
	if err != nil || len(task.Rounds) != 1 || len(task.Rounds[0].Messages) != 1 ||
		task.Rounds[0].Messages[0].Text != current.Command.Instruction {
		t.Fatalf("unexpected task-only history: %+v %v", task, err)
	}
	pending, err := s.ListPendingSteering(ctx, current.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range pending {
		if item.ClientMessageID == model.InputChat {
			t.Fatal("silently consumed chat is still pending")
		}
	}
	late := model.ThreadNamingInput{SceneRevision: current.SceneRevision, ThreadID: "t", RunID: current.ID, InputID: input.ClientMessageID, ExecutionRevision: current.ExecutionRevision + 1}
	if _, _, err := s.RecordThreadNamingInput(ctx, late); !errors.Is(err, run.ErrRunRevisionConflict) {
		t.Fatalf("stale execution registered naming: %v", err)
	}
}
