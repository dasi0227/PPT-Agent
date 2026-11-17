package run

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"go.uber.org/zap"
)

func TestPauseAllAndResumeAcrossEngineRestart(t *testing.T) {
	store := newMemStore()
	first := NewEngine(store, NewLockManager(), zap.NewNop())
	started := make(chan struct{})
	pausedAtCancellation := make(chan bool, 1)
	blocking := scriptRunner(func(ctx context.Context, _ workflow.EventEmitter, _ Checkpointer, _ Prompter) workflow.StructuredOutcome {
		close(started)
		<-ctx.Done()
		current, err := store.GetRun(context.Background(), "restartable")
		pausedAtCancellation <- err == nil && current.Status == model.RunPaused
		return workflow.StructuredOutcome{Status: workflow.StatusCanceled}
	})
	created, err := first.Start(context.Background(), testRun("restartable"), blocking)
	if err != nil {
		t.Fatal(err)
	}
	<-started
	waitRunStatus(t, store, created.ID, model.RunRunning)
	if err := first.PauseAll(context.Background(), "server_shutdown"); err != nil {
		t.Fatal(err)
	}
	if !<-pausedAtCancellation {
		t.Fatal("execution was canceled before its pause was durable")
	}
	waitRunStatus(t, store, created.ID, model.RunPaused)
	events, err := store.EventsSince(context.Background(), created.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Type.Terminal() {
			t.Fatalf("pause must not emit a terminal event: %+v", event)
		}
	}

	second := NewEngine(store, NewLockManager(), zap.NewNop())
	if err := second.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	paused, _ := store.GetRun(context.Background(), created.ID)
	resumed, err := second.Resume(context.Background(), paused, scriptRunner(func(_ context.Context, emitter workflow.EventEmitter, _ Checkpointer, _ Prompter) workflow.StructuredOutcome {
		emitTestFinal(emitter, "restartable")
		return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
	}), sessionstate.RunAction{SceneRevision: 1, EntityRevision: 1, RequestKey: "resume"})
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Current.Status != model.RunRecovering {
		t.Fatalf("resume status=%s", resumed.Current.Status)
	}
	waitRunStatus(t, store, created.ID, model.RunDone)
	events, err = store.EventsSince(context.Background(), created.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	resumedEvents := 0
	for _, event := range events {
		if event.Type == model.EventRunResumed {
			resumedEvents++
		}
	}
	if resumedEvents != 0 {
		t.Fatalf("run.resumed count=%d events=%+v", resumedEvents, events)
	}
}

func TestSchedulerPersistsCanonicalEventsAndSingleTerminal(t *testing.T) {
	store := newMemStore()
	engine := NewEngine(store, NewLockManager(), zap.NewNop())
	execution := scriptRunner(func(_ context.Context, emitter workflow.EventEmitter, _ Checkpointer, _ Prompter) workflow.StructuredOutcome {
		emitter.Emit(model.EventMessageThinking, model.MessageThinkingPayload{
			PublicEventBase: model.NewPublicEventBase("r1"), MessageID: "m1", Text: "先读取当前内容。",
		})
		emitter.Emit(model.EventMessageFinal, model.MessageFinalPayload{
			PublicEventBase: model.NewPublicEventBase("r1"), MessageID: "m2", Text: "分析完成。",
			AffectedTargets: []model.PublicTarget{},
		})
		emitter.Emit(model.EventRunCompleted, model.NewRunTerminalPayload("r1", 10, nil, nil))
		outcome := workflow.StructuredOutcome{Status: workflow.StatusCompleted}
		return outcome
	})
	run, err := engine.Start(context.Background(), testRun("r1"), execution)
	if err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, run.ID, model.RunDone)
	events, err := store.EventsSince(context.Background(), run.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("events=%+v", events)
	}
	for index, event := range events {
		if event.Seq != int64(index+1) {
			t.Fatalf("non-contiguous seq: %+v", events)
		}
	}
	terminals := 0
	for _, event := range events {
		if event.Type.Terminal() {
			terminals++
		}
	}
	if terminals != 1 {
		t.Fatalf("terminal count=%d", terminals)
	}
}

func TestCancelAuthorityOverridesLateSuccessfulOutcome(t *testing.T) {
	store := newMemStore()
	engine := NewEngine(store, NewLockManager(), zap.NewNop())
	started := make(chan struct{})
	release := make(chan struct{})
	execution := scriptRunner(func(_ context.Context, _ workflow.EventEmitter, _ Checkpointer, _ Prompter) workflow.StructuredOutcome {
		close(started)
		<-release
		return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
	})
	if _, err := engine.Start(context.Background(), testRun("cancel-wins"), execution); err != nil {
		t.Fatal(err)
	}
	<-started
	if _, err := engine.RequestCancelWithReason(context.Background(), "cancel-wins", model.RunCancelUserRequested, sessionstate.RunAction{SceneRevision: 1, EntityRevision: 1, RequestKey: "cancel"}); err != nil {
		t.Fatal(err)
	}
	close(release)
	waitRunStatus(t, store, "cancel-wins", model.RunCanceled)
	events, _ := store.EventsSince(context.Background(), "cancel-wins", 0)
	terminalCount := 0
	for _, event := range events {
		if event.Type == model.EventMessageFinal {
			t.Fatalf("cancel-requested run emitted message.final: %+v", events)
		}
		if event.Type == model.EventRunCanceled {
			terminalCount++
			var payload model.RunTerminalPayload
			if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
				t.Fatalf("terminal payload=%s err=%v", event.Payload, err)
			}
		}
	}
	if terminalCount != 1 {
		t.Fatalf("terminal count=%d events=%+v", terminalCount, events)
	}
}

func TestCancelInterruptsAskUserWait(t *testing.T) {
	store := newMemStore()
	engine := NewEngine(store, NewLockManager(), zap.NewNop())
	asking := make(chan struct{})
	execution := scriptRunner(func(ctx context.Context, _ workflow.EventEmitter, _ Checkpointer, prompter Prompter) workflow.StructuredOutcome {
		close(asking)
		question := model.QuestionAskedPayload{
			PublicEventBase: model.NewPublicEventBase("cancel-question"),
			QuestionID:      "q-cancel", Questions: []model.QuestionField{{
				ID: "choice", Question: "choose", Reason: "需要确认后续内容", Options: []model.QuestionOption{{ID: "a", Label: "A", Description: "选择 A"}},
			}},
		}
		if err := store.saveInteraction("cancel-question", "question", question.QuestionID, "", question); err != nil {
			return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: "CHECKPOINT_FAILED", Message: err.Error()}
		}
		_, err := prompter.Ask(ctx, question)
		if !errors.Is(err, context.Canceled) {
			return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: "BAD_ANSWER"}
		}
		return workflow.StructuredOutcome{Status: workflow.StatusCanceled, Code: workflow.CodeCanceled}
	})
	if _, err := engine.Start(context.Background(), testRun("cancel-question"), execution); err != nil {
		t.Fatal(err)
	}
	<-asking
	waitRunStatus(t, store, "cancel-question", model.RunWaiting)
	if _, err := engine.RequestCancelWithReason(context.Background(), "cancel-question", model.RunCancelUserRequested, sessionstate.RunAction{SceneRevision: 1, EntityRevision: 1, RequestKey: "cancel"}); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, "cancel-question", model.RunCanceled)
	events, _ := store.EventsSince(context.Background(), "cancel-question", 0)
	asked, answered, canceled := 0, 0, 0
	for _, event := range events {
		switch event.Type {
		case model.EventQuestionAsked:
			asked++
		case model.EventQuestionAnswered:
			answered++
		case model.EventRunCanceled:
			canceled++
		}
	}
	if asked != 1 || answered != 0 || canceled != 1 {
		t.Fatalf("ask cancel lifecycle: asked=%d answered=%d canceled=%d events=%+v", asked, answered, canceled, events)
	}
}

func TestStartBarrierPrecedesWorkerAndRejectsFailedAcceptance(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "accepted", true: "failed"}[fail], func(t *testing.T) {
			store := newMemStore()
			engine := NewEngine(store, NewLockManager(), zap.NewNop())
			called := 0
			ctx := WithStartBarrier(context.Background(), func() error {
				called++
				if row, err := store.GetRun(context.Background(), "r1"); err != nil || row.Status != model.RunPending {
					t.Fatal("barrier ran before durable acceptance")
				}
				if fail {
					return errors.New("acceptance disk failure")
				}
				return nil
			})
			started := make(chan bool, 1)
			execution := scriptRunner(func(_ context.Context, emitter workflow.EventEmitter, _ Checkpointer, _ Prompter) workflow.StructuredOutcome {
				started <- called == 1
				emitTestFinal(emitter, "r1")
				return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
			})
			_, err := engine.Start(ctx, testRun("r1"), execution)
			if called != 1 {
				t.Fatalf("barrier calls = %d", called)
			}
			if fail {
				if err == nil {
					t.Fatal("failed acceptance started worker")
				}
				row, _ := store.GetRun(context.Background(), "r1")
				if row.Status != model.RunFailed {
					t.Fatal("orphan active run")
				}
				select {
				case <-started:
					t.Fatal("worker ran after failed acceptance")
				default:
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			waitRunStatus(t, store, "r1", model.RunDone)
			if !<-started {
				t.Fatal("worker ran before barrier")
			}
		})
	}
}
