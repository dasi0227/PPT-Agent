package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/decision"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
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
