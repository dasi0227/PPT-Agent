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

func TestIrrelevantSteeringWaitsAtBoundaryAndCannotReuseInitialDecision(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	s := newMemStore()
	e := NewEngine(s, NewLockManager(), zap.NewNop())
	ready, work, checking, verdict, boundary := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	e.WithInputClassifier(func(ctx context.Context, current model.Run, id, text string) (model.InputDecision, error) {
		if text == "task" {
			return model.InputDecision{InputID: id, Disposition: model.InputNormal}, nil
		}
		close(checking)
		select {
		case <-verdict:
			return model.InputDecision{InputID: id, Disposition: model.InputIrrelevant}, nil
		case <-ctx.Done():
			return model.InputDecision{}, ctx.Err()
		}
	})
	_, err := e.Start(ctx, model.Run{ID: "r", ProjectID: "p", ThreadID: "t", ClientRequestID: "same", Command: model.RunCommand{Mode: model.ModeExecute, Instruction: "task", Scope: model.NewRunScope(model.ScopeAllPages)}}, scriptRunner(func(ctx context.Context, _ workflow.EventEmitter, cp Checkpointer, _ Prompter) workflow.StructuredOutcome {
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
	if _, err := e.SteerWithReferences(ctx, "r", "r", "same", "different", "independent NBA news", nil, nil, nil, model.RunScope{}); err != nil {
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
	if terminal.Reply.Text != "我专注于演示文稿的规划、制作与修改，暂不处理与此无关的独立请求。如果这项需求用于演示文稿，请说明它与当前内容的关系。当前任务已停止。" {
		t.Fatalf("wrong irrelevant reply: %q", terminal.Reply.Text)
	}
}
