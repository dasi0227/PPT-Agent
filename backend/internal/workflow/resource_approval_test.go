package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func TestResourceApprovalChangedSourceFailsWithoutOverwriting(t *testing.T) {
	input, state, registry := authoringBatchFixture(t, nil)
	path := filepath.Join(input.ProjectDir, ".meta.json")
	prompter := waitingResourcePrompter{approvals: input.Checkpoint.(*resourceApprovalFixture), requested: make(chan model.ResourceEditApprovalRequestedPayload, 1), answered: make(chan model.ResourceEditApprovalAnswer, 1)}
	input.Prompter = prompter
	call := llm.ToolCall{ID: "edit_with_conflict", Name: "edit_meta", Args: map[string]any{"goal": "Agent goal"}}
	finished := make(chan []ToolResult, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		finished <- newTestRuntime(nil).executeToolBatch(ctx, input, state, registry,
			schemasByName(registry.Disclose(PhaseExecuting, model.ModeCreate, state.scope)), []llm.ToolCall{call})
	}()
	var requested model.ResourceEditApprovalRequestedPayload
	select {
	case requested = <-prompter.requested:
	case <-ctx.Done():
		t.Fatal("approval was not requested")
	}
	manifest := spec.DefaultMeta("Changed elsewhere")
	changed, _ := json.Marshal(manifest)
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	answer := model.ResourceEditApprovalAnswer{InteractionID: requested.InteractionID, CallID: call.ID, Revision: requested.Revision, Decision: "approve"}
	prompter.answered <- answer
	select {
	case results := <-finished:
		if len(results) != 1 || results[0].Code != model.CodeContentConflict {
			t.Fatalf("changed baseline result: %+v", results)
		}
	case <-ctx.Done():
		t.Fatal("tool did not finish after baseline conflict")
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != string(changed) {
		t.Fatalf("changed source was overwritten: %q %v", actual, err)
	}
}
