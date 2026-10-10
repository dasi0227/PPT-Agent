package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type handoffExecution func(context.Context, workflow.EventEmitter, run.Checkpointer, run.Prompter) workflow.StructuredOutcome

func (execute handoffExecution) Run(ctx context.Context, emitter workflow.EventEmitter, checkpoint run.Checkpointer, prompter run.Prompter) workflow.StructuredOutcome {
	return execute(ctx, emitter, checkpoint, prompter)
}

func TestCompletedRunStartsPendingMessagesInNewRun(t *testing.T) {
	root := t.TempDir()
	db, cleanup, err := sqlitestore.Open(&config.Config{WorkRoot: root, DBPath: filepath.Join(root, "run.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	project := model.Project{ID: "p", WorkDir: filepath.Join(root, "projects", "p", "artifacts")}
	if err := os.MkdirAll(project.WorkDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(ctx, model.Thread{ID: "t", ProjectID: project.ID}); err != nil {
		t.Fatal(err)
	}
	engine := run.NewEngine(st, run.NewLockManager(), zap.NewNop())
	ready, finish := make(chan struct{}), make(chan struct{})
	type observedRun struct {
		run    model.Run
		inputs []workflow.SteeringInput
		err    error
	}
	followUp := make(chan observedRun, 1)
	svc := NewRunServiceWithExecutionFactory(st, engine, func(r model.Run, _ model.CreateRunParams, _ model.Project) run.Execution {
		return handoffExecution(func(ctx context.Context, _ workflow.EventEmitter, checkpoint run.Checkpointer, _ run.Prompter) workflow.StructuredOutcome {
			if r.Command.Instruction == "original" {
				close(ready)
				select {
				case <-finish:
				case <-ctx.Done():
					return workflow.StructuredOutcome{Status: workflow.StatusCanceled}
				}
				// Simulate a model returning finish_task without another loop drain.
				return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
			}
			inputs, err := checkpoint.DrainInputs(ctx)
			if err == nil {
				ids := make([]string, len(inputs))
				for index, input := range inputs {
					ids[index] = input.ID
				}
				err = checkpoint.MarkInputsInjected(ctx, ids)
			}
			followUp <- observedRun{run: r, inputs: inputs, err: err}
			return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
		})
	})
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = engine.PauseAll(shutdownCtx, "test_cleanup")
	})
	previous, err := svc.CreateRun(ctx, "t", model.CreateRunParams{ClientRequestID: "first", Command: model.RunCommand{
		Instruction: "original", Mode: model.ModeChat, Scope: model.NewRunScope(model.ScopeAllPages)}})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("old run did not start")
	}
	for _, content := range []string{"one", "two", "three"} {
		if _, err := engine.SteerWithReferences(ctx, previous.ID, previous.ID, content, content, content, nil, nil, nil, model.RunScope{}); err != nil {
			t.Fatal(err)
		}
	}
	close(finish)
	select {
	case observed := <-followUp:
		if observed.err != nil || observed.run.ID == previous.ID || observed.run.Command.Instruction != "one" {
			t.Fatalf("pending input did not become a new Run: %+v", observed)
		}
		if len(observed.inputs) != 2 || observed.inputs[0].Content != "two" || observed.inputs[1].Content != "three" {
			t.Fatalf("new Run lost queued messages/order: %+v", observed.inputs)
		}
		old, err := st.GetRun(ctx, previous.ID)
		if err != nil || old.Status != model.RunDone {
			t.Fatalf("old Run did not complete normally: %+v %v", old, err)
		}
		pending, err := st.ListPendingSteering(ctx, previous.ID)
		if err != nil || len(pending) != 0 {
			t.Fatalf("messages remain in old Run: %+v %v", pending, err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("pending messages never started in new Run")
	}
}
