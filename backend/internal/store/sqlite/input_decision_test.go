package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
	"gorm.io/gorm"
)

func TestConversationBeforeInputValidatesAcceptanceAndExcludesFuture(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	appendEvent := func(kind, runID string, payload any) {
		t.Helper()
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.AppendThreadEvent(ctx, "t", threadjournal.Event{Type: kind, RunID: runID, Payload: raw}); err != nil {
			t.Fatal(err)
		}
	}
	appendEvent("steering.accepted", "r", model.SteeringMessage{ClientMessageID: "earlier", Content: "继续", DOMSelections: []model.DOMSelection{{SlideID: "slide"}}})
	appendEvent("steering.accepted", "r", model.SteeringMessage{ClientMessageID: "thanks", Content: "你好"})
	appendEvent(string(model.EventMessageReply), "r", model.MessageReplyPayload{InputID: "thanks", Text: "你好，我是 Dasi"})
	appendEvent("steering.accepted", "r", model.SteeringMessage{ClientMessageID: "current", Content: "继续"})
	appendEvent("steering.accepted", "r", model.SteeringMessage{ClientMessageID: "future", Content: "未来要求"})
	appendEvent(string(model.EventMessageFinal), "r", model.MessageFinalPayload{Text: "未来最终回复"})
	// A future handoff must not hide the earlier input in the cutoff view.
	appendEvent("run.accepted", "new", runAcceptance{ClientRequestID: "transfer", SourceRunID: "r", SourceMessageIDs: []string{"earlier"}, Command: model.RunCommand{Instruction: "继续"}})
	got, err := s.LoadThreadConversationContextBeforeInput(ctx, "t", "r", "current")
	if err != nil || len(got.Rounds) != 1 {
		t.Fatalf("history=%+v %v", got, err)
	}
	if got.Input == nil || got.Input.RunID != "r" || got.Input.InputID != "current" || got.Input.Text != "继续" || got.Input.EventSeq <= 0 || got.Input.HasExplicitReferences {
		t.Fatalf("accepted input metadata was lost: %+v", got.Input)
	}
	// The fixture's accepted original requirement is retained verbatim.
	current, err := s.GetRun(ctx, "r")
	if err != nil {
		t.Fatal(err)
	}
	want := []model.ConversationMessage{{Role: "user", Text: current.Command.Instruction}, {Role: "user", Text: "继续"}, {Role: "user", Text: "你好"}, {Role: "assistant", Text: "你好，我是 Dasi"}}
	if !reflect.DeepEqual(got.Rounds[0].Messages, want) {
		t.Fatalf("cutoff lost unfinished/identical inputs or included future events: %+v", got)
	}
	for _, inputID := range []string{"unknown", "initial:earlier"} {
		if _, err := s.LoadThreadConversationContextBeforeInput(ctx, "t", "r", inputID); err == nil {
			t.Fatalf("unaccepted input %s was accepted", inputID)
		}
	}
	beforeInitial, err := s.LoadThreadConversationContextBeforeInput(ctx, "t", "r", "initial:"+current.ClientRequestID)
	if err != nil || len(beforeInitial.Rounds) != 0 {
		t.Fatalf("initial input leaked into its own history: %+v %v", beforeInitial, err)
	}
	transferred, err := s.LoadThreadConversationContextBeforeInput(ctx, "t", "new", "initial:transfer")
	if err != nil || transferred.Input == nil || transferred.Input.RunID != "r" || transferred.Input.InputID != "earlier" || !transferred.Input.HasExplicitReferences || len(transferred.Rounds) != 1 || len(transferred.Rounds[0].Messages) != 1 {
		t.Fatalf("handoff did not retain the original input boundary: %+v %v", transferred, err)
	}
}

func TestChatCompletionDurablySuppressesOnlyAutomaticSuggestions(t *testing.T) {
	s, cp := checkpointFixture(t)
	ctx := context.Background()
	payload := model.NewRunTerminalPayload(cp.RunID, 0, nil, nil)
	payload.SuppressSuggestions = true
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEvent(ctx, &model.Event{RunID: cp.RunID, Type: model.EventRunCompleted, Payload: string(raw)}); err != nil {
		t.Fatal(err)
	}
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
				ctx := context.Background()
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
				}
				late := current
				late.ExecutionRevision++
				if _, err := s.SaveInputDecision(ctx, late, result); !errors.Is(err, run.ErrRunRevisionConflict) {
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
					if _, err := s.ClaimPausedRun(ctx, current.ID, "new-owner"); !errors.Is(err, run.ErrRunNotContinuable) {
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
					if _, err := s.ClaimContinuableRun(ctx, current, "new-owner"); !errors.Is(err, run.ErrRunNotContinuable) {
						t.Fatalf("terminal stop resumed: %v", err)
					}
				}
			})
		}
	}
}

func TestNamingCountsOnlyTaskInputsButSharesConversationContext(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
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
		if _, trigger, err := s.RecordThreadNamingInput(ctx, model.ThreadNamingInput{ThreadID: "t", RunID: current.ID, InputID: decision.InputID, ExecutionRevision: current.ExecutionRevision}); err != nil || trigger {
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
		thread, trigger, err := s.RecordThreadNamingInput(ctx, model.ThreadNamingInput{ThreadID: "t", RunID: current.ID, InputID: initialID, ExecutionRevision: current.ExecutionRevision})
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
	thread, trigger, err := s.RecordThreadNamingInput(ctx, model.ThreadNamingInput{ThreadID: "t", RunID: current.ID, InputID: input.ClientMessageID, ExecutionRevision: current.ExecutionRevision})
	if err != nil || !trigger || thread.RenameInputCount != 1 {
		t.Fatalf("task steering was not counted once: %+v %v", thread, err)
	}
	plan := model.PublicPlan{Title: "保留给后续建议", Status: "active"}
	raw, err := json.Marshal(model.PlanUpdatedPayload{Plan: plan})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendThreadEvent(ctx, "t", threadjournal.Event{RunID: current.ID, Type: string(model.EventPlanUpdated), Payload: raw}); err != nil {
		t.Fatal(err)
	}
	source, err := s.LoadThreadConversationContext(ctx, "t")
	if err != nil || len(source.Rounds) != 1 || len(source.Rounds[0].Messages) != len(excluded)+2 || !reflect.DeepEqual(source.Plan, &plan) {
		t.Fatalf("shared context filtered user inputs or lost its separate plan: %+v %v", source, err)
	}
	task, err := s.LoadTaskConversationBeforeInput(ctx, "t", current.ID, input.ClientMessageID)
	if err != nil || len(task.Rounds) != 1 || len(task.Rounds[0].Messages) != 2 ||
		task.Rounds[0].Messages[0].Text != current.Command.Instruction || task.Rounds[0].Messages[1].Text != "排除:detection_failed" {
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
	late := model.ThreadNamingInput{ThreadID: "t", RunID: current.ID, InputID: input.ClientMessageID, ExecutionRevision: current.ExecutionRevision + 1}
	if _, _, err := s.RecordThreadNamingInput(ctx, late); !errors.Is(err, run.ErrRunRevisionConflict) {
		t.Fatalf("stale execution registered naming: %v", err)
	}
}
