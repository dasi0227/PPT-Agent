package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/decision"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"go.uber.org/zap"
)

type namingDecision struct {
	probability float64
	calls       int
}

func (n *namingDecision) DecisionSnapshot() decision.Snapshot {
	return decision.Snapshot{Identity: "test", Provider: n}
}
func (n *namingDecision) Evaluate(context.Context, decision.Request) (decision.Response, error) {
	n.calls++
	return decision.Response{Model: "jev-test", Answers: map[string]decision.Answer{"rename": decision.NoulAnswer{Type: "noul", Noul: n.probability}}}, nil
}

func TestAutomaticRenameGateAvoidsGenerationButManualStillGenerates(t *testing.T) {
	f := newBriefingFixture(t)
	enabled := true
	title := "季度总结"
	thread, err := f.store.UpdateThreadNamingState(context.Background(), f.thread.ID, &title, &enabled, true, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	gate := &namingDecision{probability: 0.1}
	f.provider.Script = []llm.GenerateResponse{{ToolCalls: []llm.ToolCall{{Name: renameToolName, Args: map[string]any{"action": "keep"}}}}}
	svc := NewNamingService(f.store, f.provider, NewThreadEventHub(f.store), zap.NewNop()).WithDecisions(gate)
	defer svc.Close()
	task := renameTask{threadID: thread.ID, projectID: thread.ProjectID, operationVersion: thread.RenameOperationVersion, trigger: RenameTriggerThreshold, projectGeneration: svc.projectGeneration(thread.ProjectID)}
	raw, _ := json.Marshal(renameInput{CurrentTitle: title})
	allowed, err := svc.shouldRename(context.Background(), task, string(raw))
	if err != nil || allowed {
		t.Fatal("gate accepted unnecessary rename")
	}
	if err := svc.runTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if len(f.provider.Requests()) != 0 {
		t.Fatal("rejected automatic naming called generation model")
	}
	calls := gate.calls
	if _, err := svc.GenerateNow(context.Background(), thread.ID); err != nil {
		t.Fatal(err)
	}
	if gate.calls != calls || len(f.provider.Requests()) != 1 {
		t.Fatal("explicit naming did not bypass gate")
	}
}

func TestNormalNamingRunsInBackgroundWithoutBlockingTheNextRun(t *testing.T) {
	f := newBriefingFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	enabled := true
	if _, err := f.store.UpdateThreadNamingState(ctx, f.thread.ID, nil, &enabled, true, 1); err != nil {
		t.Fatal(err)
	}
	command := model.RunCommand{Instruction: "制作季度汇报演示", Mode: model.ModeCreate, Scope: model.NewRunScope(model.ScopeAllPages)}
	if err := f.store.CreateRun(ctx, model.Run{ID: "first", ThreadID: f.thread.ID, ProjectID: f.project.ID, ClientRequestID: "first-request", Command: command, Status: model.RunRunning}); err != nil {
		t.Fatal(err)
	}
	current, err := f.store.GetRun(ctx, "first")
	if err != nil {
		t.Fatal(err)
	}
	input := model.InputDecision{InputID: "initial:first-request", Disposition: model.InputNormal, Fingerprint: "normal", ExecutionRevision: current.ExecutionRevision}
	if _, err := f.store.SaveInputDecision(ctx, current, input); err != nil {
		t.Fatal(err)
	}
	if err := f.store.SetRunStatus(ctx, current.ID, model.RunDone); err != nil {
		t.Fatal(err)
	}
	started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	provider := namingHookProvider{Provider: f.provider, hook: func(ctx context.Context, _ llm.GenerateRequest) (llm.GenerateResponse, error) {
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
			return llm.GenerateResponse{}, ctx.Err()
		}
		return llm.GenerateResponse{ToolCalls: []llm.ToolCall{{Name: renameToolName, Args: map[string]any{"action": "keep"}}}}, nil
	}}
	naming := NewNamingService(f.store, provider, NewThreadEventHub(f.store), zap.NewNop())
	t.Cleanup(func() {
		close(release)
		naming.Close()
		stopCtx, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		_ = naming.StopDeleted(stopCtx, f.project.ID, "")
	})
	engine := run.NewEngine(f.store, f.locks, zap.NewNop())
	t.Cleanup(func() {
		stopCtx, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		_ = engine.StopDeleted(stopCtx, f.project.ID, "")
	})
	svc := NewRunServiceWithExecutionFactoryAndRegistry(f.store, engine, func(model.Run, model.CreateRunParams, model.Project) run.Execution {
		return handoffExecution(func(context.Context, workflow.EventEmitter, run.Checkpointer, run.Prompter) workflow.StructuredOutcome {
			close(finished)
			return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
		})
	}, f.registry).WithNaming(naming)
	root := filepath.Dir(filepath.Dir(model.ProjectRoot(f.project.WorkDir)))
	if _, err := svc.EnableProjectHistory(root); err != nil {
		t.Fatal(err)
	}
	svc.scheduleInputNaming(current, input)
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("normal input did not start background naming")
	}
	if _, err := svc.CreateRun(ctx, f.thread.ID, model.CreateRunParams{ClientRequestID: "next-request", Command: command}); err != nil {
		t.Fatalf("background naming blocked the next request: %v", err)
	}
	select {
	case <-finished:
	case <-ctx.Done():
		t.Fatal("main execution waited for the naming model")
	}
}
