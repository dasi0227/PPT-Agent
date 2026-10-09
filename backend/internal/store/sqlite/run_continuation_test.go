package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"go.uber.org/zap"
)

func TestContinuationClaimsOriginalRunOnce(t *testing.T) {
	for _, status := range []model.RunStatus{model.RunFailed, model.RunCanceled} {
		t.Run(string(status), func(t *testing.T) {
			s, cp := checkpointFixture(t)
			ctx := context.Background()
			cp.Boundary = "terminal"
			cp.ContinuationAllowed = true
			if err := s.SaveCheckpoint(ctx, cp); err != nil {
				t.Fatal(err)
			}
			if err := s.SetRunStatus(ctx, cp.RunID, status); err != nil {
				t.Fatal(err)
			}
			original, err := s.GetRun(ctx, cp.RunID)
			if err != nil || !original.CanContinue {
				t.Fatalf("not continuable: %+v %v", original, err)
			}
			resumed, err := s.ClaimContinuableRun(ctx, original, "next-owner")
			if err != nil {
				t.Fatal(err)
			}
			if resumed.ID != original.ID || resumed.Status != model.RunRecovering || resumed.CancelRequestedAt != 0 || resumed.ExecutionRevision != original.ExecutionRevision+1 {
				t.Fatalf("bad continuation: %+v", resumed)
			}
			if _, err := s.ClaimContinuableRun(ctx, original, "duplicate"); !errors.Is(err, run.ErrRunNotContinuable) {
				t.Fatalf("duplicate accepted: %v", err)
			}
			// A new execution that crashes before writing a checkpoint cannot reuse the old terminal checkpoint.
			if err := s.SetRunStatus(ctx, cp.RunID, model.RunFailed); err != nil {
				t.Fatal(err)
			}
			failed, err := s.GetRun(ctx, cp.RunID)
			if err != nil || failed.CanContinue {
				t.Fatalf("stale recovery point exposed: %+v %v", failed, err)
			}
		})
	}
}
func TestContinuationRejectsSupersededProjectAndRuntimeErrors(t *testing.T) {
	for _, kind := range []string{"new_run", "runtime_error", "superseded", "no_checkpoint"} {
		t.Run(kind, func(t *testing.T) {
			s, cp := checkpointFixture(t)
			ctx := context.Background()
			cp.Boundary = "terminal"
			cp.ContinuationAllowed = true
			if kind != "no_checkpoint" {
				if err := s.SaveCheckpoint(ctx, cp); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "runtime_error" || kind == "superseded" {
				event := model.EventRunError
				payload := model.NewRunTerminalPayload(cp.RunID, 1, nil, model.NewAgentError("INTERNAL", "test", nil).Public())
				if kind == "superseded" {
					event = model.EventRunCanceled
					payload.Reason = model.RunCancelSuperseded
				}
				raw, _ := json.Marshal(payload)
				if err := s.AppendEvent(ctx, &model.Event{RunID: cp.RunID, Type: event, Payload: string(raw)}); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := s.SetRunStatus(ctx, cp.RunID, model.RunFailed); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "new_run" {
				if err := s.CreateRun(ctx, model.Run{ID: "new", ProjectID: "p", ThreadID: "t", Status: model.RunRunning, Command: model.RunCommand{Scope: cp.Scope, Mode: cp.Mode}, CreatedAt: 1}); err != nil {
					t.Fatal(err)
				}
			}
			r, err := s.GetRun(ctx, cp.RunID)
			if err != nil || r.CanContinue {
				t.Fatalf("unsafe continuation: %+v %v", r, err)
			}
			if _, err := s.ClaimContinuableRun(ctx, r, "owner2"); !errors.Is(err, run.ErrRunNotContinuable) {
				t.Fatalf("unsafe claim: %v", err)
			}
		})
	}
}

func TestEngineContinuesTerminalHistoryOnSameRun(t *testing.T) {
	for _, tc := range []struct {
		terminal model.EventType
		code     string
	}{
		{model.EventRunFailed, workflow.CodeBudgetExceeded},
		{model.EventRunFailed, "PROVIDER_UNAVAILABLE"},
		{model.EventRunCanceled, ""},
	} {
		t.Run(string(tc.terminal)+tc.code, func(t *testing.T) {
			terminal := tc.terminal
			s, cp := checkpointFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cp.Boundary = "terminal"
			cp.ContinuationAllowed = true
			if err := s.SaveCheckpoint(ctx, cp); err != nil {
				t.Fatal(err)
			}
			bus := run.NewBus(cp.RunID, s)
			base := model.NewPublicEventBase(cp.RunID)
			if err := bus.Emit(ctx, model.EventRunStarted, model.RunStartedPayload{PublicEventBase: base, Mode: cp.Mode, Scope: cp.Scope, UserInput: "continue"}); err != nil {
				t.Fatal(err)
			}
			var publicErr *model.PublicError
			if terminal == model.EventRunFailed {
				publicErr = model.NewAgentError(tc.code, "provider_request", nil).Public()
			}
			if err := bus.Emit(ctx, terminal, model.NewRunTerminalPayload(cp.RunID, 10, nil, publicErr)); err != nil {
				t.Fatal(err)
			}
			original, err := s.GetRun(ctx, cp.RunID)
			if err != nil || !original.CanContinue {
				t.Fatalf("terminal persistence revoked continuation: run=%+v err=%v", original, err)
			}
			engine := run.NewEngine(s, run.NewLockManager(), zap.NewNop())
			execution := terminalTestExecution(func(_ context.Context, em workflow.EventEmitter, _ run.Checkpointer, _ run.Prompter) workflow.StructuredOutcome {
				em.Emit(model.EventMessageFinal, model.MessageFinalPayload{PublicEventBase: base, MessageID: "final", Text: "已完成", AffectedTargets: []model.PublicTarget{}, SuggestedNextInputs: []string{}})
				em.Emit(model.EventRunCompleted, model.NewRunTerminalPayload(cp.RunID, 11, nil, nil))
				return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
			})
			if _, err := engine.Resume(ctx, original, execution); err != nil {
				t.Fatal(err)
			}
			events := readRunEventsUntilStopped(t, ctx, s, cp.RunID, nil)
			counts := map[model.EventType]int{}
			for _, event := range events {
				counts[event.Type]++
			}
			if ctx.Err() != nil {
				t.Fatal("continued run did not finish", ctx.Err())
			}
			if counts[model.EventRunStarted] != 1 || counts[terminal] != 1 || counts[model.EventRunResumed] != 1 || counts[model.EventMessageFinal] != 1 || counts[model.EventRunCompleted] != 1 {
				t.Fatalf("broken continuation history: %+v", counts)
			}
			finished, err := s.GetRun(ctx, cp.RunID)
			if err != nil || finished.Status != model.RunDone || finished.CanContinue {
				t.Fatalf("bad finished run %+v %v", finished, err)
			}
		})
	}
}

func TestContinuationRepublishesPendingQuestionAndAcceptsAnswer(t *testing.T) {
	s, cp := checkpointFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cp.Boundary = "terminal"
	cp.ContinuationAllowed = true
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatal(err)
	}
	base := model.NewPublicEventBase(cp.RunID)
	question := model.QuestionAskedPayload{PublicEventBase: base, QuestionID: "q", Questions: []model.QuestionField{{ID: "topic", Question: "选择主题", Reason: "需要确认演示主题", Options: []model.QuestionOption{{ID: "skill", Label: "Skill", Description: "采用 Skill 主题"}}}}}
	bus := run.NewBus(cp.RunID, s)
	if err := bus.Emit(ctx, model.EventRunStarted, model.RunStartedPayload{PublicEventBase: base, Mode: cp.Mode, Scope: cp.Scope, UserInput: "continue"}); err != nil {
		t.Fatal(err)
	}
	if err := bus.Emit(ctx, model.EventQuestionAsked, question); err != nil {
		t.Fatal(err)
	}
	if err := bus.Emit(ctx, model.EventRunCanceled, model.NewRunTerminalPayload(cp.RunID, 1, nil, nil)); err != nil {
		t.Fatal(err)
	}
	original, err := s.GetRun(ctx, cp.RunID)
	if err != nil {
		t.Fatal(err)
	}
	engine := run.NewEngine(s, run.NewLockManager(), zap.NewNop())
	execution := terminalTestExecution(func(worker context.Context, _ workflow.EventEmitter, _ run.Checkpointer, p run.Prompter) workflow.StructuredOutcome {
		answer, _, err := p.Ask(worker, question)
		if err != nil || len(answer.Answers) != 1 {
			return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: "BAD_ANSWER"}
		}
		return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
	})
	if _, err := engine.Resume(ctx, original, execution); err != nil {
		t.Fatal(err)
	}
	resumed, asked, answered := false, 0, 0
	readRunEventsUntilStopped(t, ctx, s, cp.RunID, func(event model.Event) {
		if event.Type == model.EventRunResumed {
			resumed = true
		}
		if event.Type == model.EventQuestionAsked {
			asked++
			if resumed {
				if err := engine.InjectInput(ctx, cp.RunID, `{"answers":[{"question_id":"topic","selected_option_id":"skill"}]}`, "q"); err != nil {
					t.Fatal(err)
				}
			}
		}
		if event.Type == model.EventQuestionAnswered {
			answered++
		}
	})
	if ctx.Err() != nil || asked != 2 || answered != 1 {
		t.Fatalf("pending controls not restored: asked=%d answered=%d err=%v", asked, answered, ctx.Err())
	}
	final, err := s.GetRun(ctx, cp.RunID)
	if err != nil || final.Status != model.RunDone {
		t.Fatalf("continuation did not complete: %+v %v", final, err)
	}
}
