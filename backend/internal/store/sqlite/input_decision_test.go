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
	for _, paused := range []bool{false, true} {
		t.Run(fmt.Sprint(paused), func(t *testing.T) {
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
			result := model.InputDecision{InputID: "initial:request", Fingerprint: "material", Identity: "rules-model", ExecutionRevision: current.ExecutionRevision, Disposition: model.InputDetectionFailed}
			late := current
			late.ExecutionRevision++
			if _, err := s.SaveInputDecision(ctx, late, result); !errors.Is(err, run.ErrRunRevisionConflict) {
				t.Fatalf("late result accepted: %v", err)
			}
			if _, err := s.SaveInputDecision(ctx, current, result); err != nil {
				t.Fatal(err)
			}
			passed := result
			passed.Disposition = model.InputRequest
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
