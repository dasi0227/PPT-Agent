package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"go.uber.org/zap"
)

type terminalTestExecution func(context.Context, workflow.EventEmitter, run.Checkpointer, run.Prompter) workflow.StructuredOutcome

func (f terminalTestExecution) Run(ctx context.Context, em workflow.EventEmitter, cp run.Checkpointer, p run.Prompter) workflow.StructuredOutcome {
	return f(ctx, em, cp, p)
}

func TestEngineTerminalLifecycleWithSQLite(t *testing.T) {
	for _, tc := range []struct {
		name   string
		event  model.EventType
		status model.RunStatus
		result workflow.WorkflowStatus
	}{
		{"completed", model.EventRunCompleted, model.RunDone, workflow.StatusCompleted},
		{"failed", model.EventRunFailed, model.RunFailed, workflow.StatusFailed},
		{"cancel_waiting_question", model.EventRunCanceled, model.RunCanceled, workflow.StatusCanceled},
	} {
		for _, publish := range []bool{true, false} {
			name := tc.name + "/scheduler_terminal"
			if publish {
				name = tc.name + "/runtime_terminal"
			}
			t.Run(name, func(t *testing.T) {
				s := newJournalTestStore(t)
				engine := run.NewEngine(s, run.NewLockManager(), zap.NewNop())
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				t.Cleanup(func() {
					cleanupCtx, stop := context.WithTimeout(context.Background(), time.Second)
					defer stop()
					if err := engine.PauseAll(cleanupCtx, "test_cleanup"); err != nil {
						t.Error(err)
					}
				})
				execution := terminalTestExecution(func(worker context.Context, em workflow.EventEmitter, _ run.Checkpointer, prompter run.Prompter) workflow.StructuredOutcome {
					if tc.status == model.RunCanceled {
						_, _, err := prompter.Ask(worker, model.QuestionAskedPayload{
							PublicEventBase: model.NewPublicEventBase("r"), QuestionID: "topic",
							Questions: []model.QuestionField{{ID: "topic", Title: "这次要做什么演示文稿？", Options: []model.QuestionOption{{ID: "other", Label: "其他主题"}}}},
						})
						if err != context.Canceled {
							t.Errorf("question was not canceled: %v", err)
							return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: "UNEXPECTED_ANSWER"}
						}
					}
					if publish {
						if tc.status == model.RunDone {
							em.Emit(model.EventMessageFinal, model.MessageFinalPayload{
								PublicEventBase: model.NewPublicEventBase("r"), MessageID: "final", Text: "你好！", AffectedTargets: []model.PublicTarget{}, SuggestedNextInputs: []string{},
							})
						}
						var publicErr *model.PublicError
						if tc.status == model.RunFailed {
							publicErr = model.NewAgentError("RUN_FAILED", "run", nil).Public()
						}
						em.Emit(tc.event, model.NewRunTerminalPayload("r", 0, nil, publicErr))
					}
					return workflow.StructuredOutcome{Status: tc.result}
				})
				if _, err := engine.Start(ctx, model.Run{ID: "r", ProjectID: "p", ThreadID: "t", Command: activeRunSpec()}, execution); err != nil {
					t.Fatal(err)
				}
				events, stop, err := engine.Subscribe(ctx, "r", 0)
				if err != nil {
					t.Fatal(err)
				}
				defer stop()
				terminalCount := 0
				for event := range events {
					if event.Type == model.EventQuestionAsked {
						if _, err := engine.RequestCancel(ctx, "r"); err != nil {
							t.Fatal(err)
						}
					}
					if event.Type == tc.event {
						terminalCount++
						stored, err := s.GetRun(ctx, "r")
						if err != nil || stored.Status != tc.status {
							t.Fatalf("published terminal before state commit: %+v, %v", stored, err)
						}
					}
				}
				if ctx.Err() != nil {
					t.Fatal("run did not finish", ctx.Err())
				}
				if terminalCount != 1 {
					t.Fatalf("terminal events=%d", terminalCount)
				}
				if err := s.CreateRun(ctx, model.Run{ID: "next", ThreadID: "t", ProjectID: "p", Command: activeRunSpec(), Status: model.RunPending}); err != nil {
					t.Fatalf("project still locked: %v", err)
				}
			})
		}
	}
}

func TestSchedulerTerminalWriteFailureLeavesRecoverableRun(t *testing.T) {
	s := newJournalTestStore(t)
	if err := s.db.Exec(`CREATE TRIGGER fail_terminal BEFORE INSERT ON thread_event_outbox
WHEN json_extract(NEW.event_json, '$.type') = 'run.completed'
BEGIN SELECT RAISE(ABORT,'injected journal failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	engine := run.NewEngine(s, run.NewLockManager(), zap.NewNop())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	execution := terminalTestExecution(func(context.Context, workflow.EventEmitter, run.Checkpointer, run.Prompter) workflow.StructuredOutcome {
		return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
	})
	if _, err := engine.Start(ctx, model.Run{ID: "r", ThreadID: "t", ProjectID: "p", Command: activeRunSpec()}, execution); err != nil {
		t.Fatal(err)
	}
	events, stop, err := engine.Subscribe(ctx, "r", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	for event := range events {
		if event.Type == model.EventRunCompleted {
			t.Fatal("published an uncommitted completion")
		}
	}
	if ctx.Err() != nil {
		t.Fatal(ctx.Err())
	}
	stored, err := s.GetRun(ctx, "r")
	if err != nil || stored.Status != model.RunPaused || stored.PauseReason != "journal_write_failed" {
		t.Fatalf("failed finalization left an unrecoverable run: %+v, %v", stored, err)
	}
	if _, err := engine.RequestCancel(ctx, "r"); err != nil {
		t.Fatal("cannot cancel paused run", err)
	}
	if active, err := s.HasActiveRun(ctx, "p"); err != nil || active {
		t.Fatalf("cancel did not release project: %v, %v", active, err)
	}
}
