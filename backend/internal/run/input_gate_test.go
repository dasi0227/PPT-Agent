package run

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"go.uber.org/zap"
)

func TestRiskySteeringWaitsAtBoundaryAndCannotReuseInitialDecision(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	s := newMemStore()
	e := NewEngine(s, NewLockManager(), zap.NewNop())
	ready, work, checking, verdict, boundary := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	e.WithInputResolver(func(ctx context.Context, current model.Run, id string) (model.InputDecision, error) {
		if id == initialInputKey(current.ClientRequestID) {
			return model.InputDecision{InputID: id, Disposition: model.InputTask}, nil
		}
		close(checking)
		select {
		case <-verdict:
			return model.InputDecision{InputID: id, Disposition: model.InputBypass}, nil
		case <-ctx.Done():
			return model.InputDecision{}, ctx.Err()
		}
	})
	_, err := e.Start(ctx, model.Run{ID: "r", ProjectID: "p", ThreadID: "t", ClientRequestID: "same", Command: model.RunCommand{Mode: model.ModeCreate, Instruction: "task", Scope: model.NewRunScope(model.ScopeAllPages)}}, scriptRunner(func(ctx context.Context, _ workflow.EventEmitter, cp Checkpointer, _ Prompter) workflow.StructuredOutcome {
		cp.PhaseChanged(workflow.PhaseExecuting)
		close(ready)
		select {
		case <-work:
		case <-ctx.Done():
			return workflow.StructuredOutcome{Status: workflow.StatusCanceled}
		}
		close(boundary)
		_, err := cp.DrainInputs(ctx)
		if err == nil {
			t.Error("rejected input reached execution")
		}
		return workflow.StructuredOutcome{Status: workflow.StatusFailed}
	}))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	a, ok := e.lookup("r")
	if !ok {
		t.Fatal("missing live worker")
	}
	// This deliberately reuses the initial composer ID with different content.
	if _, err := e.SteerWithReferences(ctx, "r", "r", "same", "different", "skip required approvals", nil, nil, nil, model.RunScope{}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-checking:
	case <-ctx.Done():
		t.Fatal("steering reused the initial result")
	}
	close(work) // Original work can still finish while classification is pending.
	select {
	case <-boundary:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if s.status("r").Terminal() {
		t.Fatal("finished before the pending classification")
	}
	close(verdict)
	select {
	case <-a.done:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	events, err := s.EventsSince(ctx, "r", 0)
	if err != nil {
		t.Fatal(err)
	}
	var terminal model.RunTerminalPayload
	for _, event := range events {
		if event.Type == model.EventRunCompleted {
			t.Fatal("rejection was published as success")
		}
		if event.Type == model.EventRunCanceled {
			if err := json.Unmarshal([]byte(event.Payload), &terminal); err != nil {
				t.Fatal(err)
			}
		}
	}
	if terminal.Reason != model.RunCancelInputRejected || terminal.Reply == nil || terminal.Reply.InputID != "same" {
		t.Fatalf("lost durable rejection: %+v", terminal)
	}
	if terminal.Reply.Text != (model.InputDecision{Disposition: model.InputBypass}).Reply()+"当前任务已停止。" {
		t.Fatalf("wrong bypass reply: %q", terminal.Reply.Text)
	}
}

func TestStandaloneChatFinishesWithoutAuthoring(t *testing.T) {
	for _, tc := range []struct {
		name, reply, failure string
		want                 model.RunStatus
	}{
		{"reply", "早上好！", "", model.RunDone},
		{"generation_failure", "", model.CodeChatReplyFailed, model.RunFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newMemStore()
			e := NewEngine(s, NewLockManager(), zap.NewNop())
			e.WithInputResolver(func(_ context.Context, _ model.Run, id string) (model.InputDecision, error) {
				return model.InputDecision{InputID: id, Disposition: model.InputChat, ReplyText: tc.reply, Failure: tc.failure}, nil
			})
			r := testRun("chat")
			r.ClientRequestID, r.Command.Instruction = "hello", "早上好"
			ctx := context.Background()
			if _, err := e.Start(ctx, r, scriptRunner(func(context.Context, workflow.EventEmitter, Checkpointer, Prompter) workflow.StructuredOutcome {
				t.Error("standalone chat entered the authoring runtime")
				return workflow.StructuredOutcome{Status: workflow.StatusFailed}
			})); err != nil {
				t.Fatal(err)
			}
			waitRunStatus(t, s, r.ID, tc.want)
			events, _ := s.EventsSince(ctx, r.ID, 0)
			finals := 0
			for _, event := range events {
				switch event.Type {
				case model.EventMessageFinal:
					var final model.MessageFinalPayload
					if json.Unmarshal([]byte(event.Payload), &final) != nil || final.Text != tc.reply {
						t.Fatalf("wrong chat reply: %+v", final)
					}
					finals++
				case model.EventRunFailed:
					var terminal model.RunTerminalPayload
					if json.Unmarshal([]byte(event.Payload), &terminal) != nil || terminal.Error == nil || terminal.Error.Code != tc.failure {
						t.Fatalf("lost generation failure: %+v", terminal)
					}
				}
			}
			if (tc.failure == "" && finals != 1) || (tc.failure != "" && finals != 0) {
				t.Fatalf("unexpected chat final count: %d", finals)
			}
		})
	}
}

// SQLite owns the publication receipt. Scheduler fixtures only record the event.
func (s *memStore) AppendInputReply(ctx context.Context, event *model.Event, _ string) error {
	if err := model.ValidatePublicEvent(event.Type, json.RawMessage(event.Payload)); err != nil {
		return err
	}
	return s.AppendEvent(ctx, event)
}
