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

func TestIntentHistoryUsesFourCompletedOriginalPairsBeforeInput(t *testing.T) {
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
	for i := 1; i <= 6; i++ {
		id := fmt.Sprintf("history%d", i)
		appendEvent("run.accepted", id, runAcceptance{ClientRequestID: id, Command: model.RunCommand{Instruction: id}})
		if i == 6 {
			appendEvent("steering.accepted", id, model.SteeringMessage{ClientMessageID: "extra", Content: "追加原文"})
		}
		appendEvent(string(model.EventQuestionAnswered), id, map[string]any{"answer": "排除 ask_user 回答"})
		appendEvent(string(model.EventMessageReasoning), id, map[string]any{"text": "排除思考"})
		appendEvent(string(model.EventMessageFinal), id, model.MessageFinalPayload{Text: id + "最终原文"})
	}
	appendEvent("steering.accepted", "r", model.SteeringMessage{ClientMessageID: "current", Content: "本次输入"})
	appendEvent("run.accepted", "future", runAcceptance{ClientRequestID: "future", Command: model.RunCommand{Instruction: "未来"}})
	appendEvent(string(model.EventMessageFinal), "future", model.MessageFinalPayload{Text: "不应进入当前历史"})
	got, err := s.LoadIntentHistory(ctx, "t", "r", "current")
	if err != nil {
		t.Fatal(err)
	}
	want := []model.DialogueRound{
		{UserInputs: []string{"history3"}, FinalReply: "history3最终原文"},
		{UserInputs: []string{"history4"}, FinalReply: "history4最终原文"},
		{UserInputs: []string{"history5"}, FinalReply: "history5最终原文"},
		{UserInputs: []string{"history6", "追加原文"}, FinalReply: "history6最终原文"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("history=%+v", got)
	}
	if _, err := s.LoadIntentHistory(ctx, "t", "r", "unknown"); err == nil {
		t.Fatal("unknown input was accepted")
	}
}

func TestFixedCompletionDurablySuppressesOnlyAutomaticSuggestions(t *testing.T) {
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
		t.Fatalf("fixed reply triggered automatic generation: %v", err)
	}
	manual := model.CommandExecution{Kind: "suggest", Source: "manual", RunID: cp.RunID}
	if err := s.db.Transaction(func(tx *gorm.DB) error { return prepareSuggestCommand(tx, &manual, thread) }); err != nil {
		t.Fatalf("manual behavior changed: %v", err)
	}
}

func TestInputStopFencesLateResultsAndBothContinuationClaims(t *testing.T) {
	for _, disposition := range []string{model.InputInternalInstructions, model.InputRuleBypass, model.InputIrrelevant, model.InputDetectionFailed} {
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
				result := model.InputDecision{InputID: "initial:request", Fingerprint: "material", Identity: "rules-model", ExecutionRevision: current.ExecutionRevision, Disposition: disposition}
				late := current
				late.ExecutionRevision++
				if _, err := s.SaveInputDecision(ctx, late, result); !errors.Is(err, run.ErrRunRevisionConflict) {
					t.Fatalf("late result accepted: %v", err)
				}
				if _, err := s.SaveInputDecision(ctx, current, result); err != nil {
					t.Fatal(err)
				}
				passed := result
				passed.Disposition = model.InputNormal
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

func TestNamingCountsOnlyNormalInputsAndFiltersAutomaticContext(t *testing.T) {
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
	for _, disposition := range []string{model.InputGreeting, model.InputIdentity, model.InputIrrelevant, model.InputInternalInstructions, model.InputRuleBypass, model.InputDetectionFailed} {
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
		t.Fatalf("non-normal inputs consumed naming eligibility: %+v %v", thread, err)
	}
	initialID := "initial:" + current.ClientRequestID
	if _, err := s.SaveInputDecision(ctx, current, model.InputDecision{InputID: initialID, Fingerprint: "normal", Disposition: model.InputNormal, ExecutionRevision: current.ExecutionRevision}); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		thread, trigger, err := s.RecordThreadNamingInput(ctx, model.ThreadNamingInput{ThreadID: "t", RunID: current.ID, InputID: initialID, ExecutionRevision: current.ExecutionRevision})
		if err != nil || trigger != (attempt == 0) || !thread.RenameFirstInputSeen || thread.RenameInputCount != 0 {
			t.Fatalf("initial normal input was lost or counted twice: %+v %v", thread, err)
		}
	}
	input := model.SteeringMessage{RunID: current.ID, ThreadID: "t", ClientMessageID: "normal-extra", RequestHash: "normal-extra", Content: "修改演示的图表", Scope: current.Command.Scope}
	if _, _, err := s.CreateSteering(ctx, input); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveInputDecision(ctx, current, model.InputDecision{InputID: input.ClientMessageID, Fingerprint: "extra", Disposition: model.InputNormal, ExecutionRevision: current.ExecutionRevision}); err != nil {
		t.Fatal(err)
	}
	thread, trigger, err := s.RecordThreadNamingInput(ctx, model.ThreadNamingInput{ThreadID: "t", RunID: current.ID, InputID: input.ClientMessageID, ExecutionRevision: current.ExecutionRevision})
	if err != nil || !trigger || thread.RenameInputCount != 1 {
		t.Fatalf("normal steering was not counted once: %+v %v", thread, err)
	}
	automatic, err := s.LoadAutomaticRenameContext(ctx, "t")
	if err != nil || !reflect.DeepEqual(automatic.Activity, []model.ThreadNamingActivity{{Role: "user", Text: "original input"}, {Role: "user", Text: input.Content}}) {
		t.Fatalf("automatic naming includes non-normal input: %+v %v", automatic, err)
	}
	manual, err := s.LoadThreadRenameContext(ctx, "t")
	if err != nil || len(manual.Activity) != 8 {
		t.Fatalf("manual naming context changed: %+v %v", manual, err)
	}
	late := model.ThreadNamingInput{ThreadID: "t", RunID: current.ID, InputID: input.ClientMessageID, ExecutionRevision: current.ExecutionRevision + 1}
	if _, _, err := s.RecordThreadNamingInput(ctx, late); !errors.Is(err, run.ErrRunRevisionConflict) {
		t.Fatalf("stale execution registered naming: %v", err)
	}
}
