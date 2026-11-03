package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
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
	st, err := sqlitestore.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := sessionstate.WithRequestScene(context.Background(), 1)
	project := model.Project{ID: "p", WorkDir: filepath.Join(root, "projects", "p", "artifacts"), CreatedAt: 1, UpdatedAt: 1}
	if err := os.MkdirAll(project.WorkDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(ctx, model.Thread{ID: "t", ProjectID: project.ID, CreatedAt: 1, UpdatedAt: 1}); err != nil {
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
				// Complete the original request without consuming the pending additions.
				if err := commitHandoffFixtureContext(ctx, st, checkpoint, r, nil); err != nil {
					return workflow.StructuredOutcome{Status: workflow.StatusFailed, Message: err.Error()}
				}
				return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
			}
			inputs, err := checkpoint.DrainInputs(ctx)
			if err == nil {
				err = commitHandoffFixtureContext(ctx, st, checkpoint, r, inputs)
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
	previous, err := svc.CreateRun(ctx, "t", model.CreateRunParams{SceneRevision: 1, ClientRequestID: "first", Command: model.RunCommand{
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

// Consume accepted instructions with the same native-context/checkpoint boundary
// as Runtime; the handoff test does not need to execute a model or any tools.
func commitHandoffFixtureContext(ctx context.Context, st *sqlitestore.Store, checkpoint run.Checkpointer, r model.Run, inputs []workflow.SteeringInput) error {
	current, err := st.GetRun(ctx, r.ID)
	if err != nil {
		return err
	}
	keys := []string{"initial:" + r.ClientRequestID}
	for _, input := range inputs {
		keys = append(keys, input.ID)
	}
	for _, key := range keys {
		if _, err := st.SaveInputDecision(ctx, current, model.InputDecision{InputID: key, Fingerprint: "handoff-fixture:" + key, Disposition: model.InputTask, ExecutionRevision: current.ExecutionRevision}); err != nil {
			return err
		}
	}
	snapshot, err := st.LoadContext(ctx, r.ThreadID)
	if err != nil {
		return err
	}
	session, err := contextengine.NewContextSession(snapshot)
	if err != nil {
		return err
	}
	messages := append(session.Messages(), llm.Message{Role: llm.RoleUser, Content: llm.TextContent(r.Command.Instruction), Metadata: &llm.MessageMetadata{Origin: "user", Kind: "instruction", RunID: r.ID}})
	for _, input := range inputs {
		messages = append(messages, llm.Message{Role: llm.RoleUser, Content: llm.TextContent(input.Content), Metadata: &llm.MessageMetadata{Origin: "user", Kind: "steering", RunID: r.ID, Key: input.ID}})
	}
	messages = append(messages, llm.Message{Role: llm.RoleAssistant, Content: llm.TextContent("done")})
	prepared, err := session.Prepare(messages, r.ID, "append")
	if err != nil {
		return err
	}
	final, err := sessionstate.NewRunUpdate(model.EventMessageFinal, model.MessageFinalPayload{PublicEventBase: model.NewPublicEventBase(r.ID), MessageID: prepared.Messages[len(prepared.Messages)-1].PersistentID, Text: "done", AffectedTargets: []model.PublicTarget{}})
	if err != nil {
		return err
	}
	return checkpoint.SaveCheckpoint(ctx, workflow.RuntimeCheckpoint{
		RunID: r.ID, LoopID: "handoff_fixture", SceneRevision: r.SceneRevision,
		Scope: r.Command.Scope, Mode: r.Command.Mode, Phase: workflow.PhaseChat,
		Boundary: "inputs_consumed", ContextRevision: snapshot.Revision,
		ContextUpdate: &prepared.Update, PublicUpdates: []sessionstate.RunUpdate{final},
		CreatedAt: time.Now().UnixMilli(),
	})
}
