package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type waitingResourcePrompter struct {
	requested chan model.ResourceEditApprovalRequestedPayload
	answered  chan model.ResourceEditApprovalAnswer
}

func (waitingResourcePrompter) Ask(context.Context, model.QuestionAskedPayload) (model.QuestionAnswer, string, error) {
	return model.QuestionAnswer{}, "", nil
}
func (p waitingResourcePrompter) AskResourceEditApproval(ctx context.Context, payload model.ResourceEditApprovalRequestedPayload) (model.ResourceEditApprovalAnswer, error) {
	p.requested <- payload
	select {
	case answer := <-p.answered:
		return answer, nil
	case <-ctx.Done():
		return model.ResourceEditApprovalAnswer{}, ctx.Err()
	}
}
func (waitingResourcePrompter) ResumeAfterResourceEditApproval(context.Context) {}

func TestResourceApprovalBlocksCommitAndPublishesEditedDraft(t *testing.T) {
	input, state, registry := authoringBatchFixture(t, nil)
	before, err := os.ReadFile(filepath.Join(input.ProjectDir, ".design.json"))
	if err != nil {
		t.Fatal(err)
	}
	prompter := waitingResourcePrompter{requested: make(chan model.ResourceEditApprovalRequestedPayload, 1), answered: make(chan model.ResourceEditApprovalAnswer, 1)}
	input.Prompter = prompter
	call := llm.ToolCall{ID: "edit_for_approval", Name: "edit_design", Args: map[string]any{"demands": []any{"Agent suggestion"}}}
	finished := make(chan []ToolResult, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		finished <- NewRuntime(nil).executeToolBatch(ctx, input, state, registry,
			schemasByName(registry.Disclose(PhaseExecuting, model.ModeExecute, state.scope)), []llm.ToolCall{call})
	}()
	var requested model.ResourceEditApprovalRequestedPayload
	select {
	case requested = <-prompter.requested:
	case <-ctx.Done():
		t.Fatal("approval was not requested")
	}
	actual, err := os.ReadFile(filepath.Join(input.ProjectDir, ".design.json"))
	if err != nil || string(actual) != string(before) {
		t.Fatalf("candidate was written before approval: %q %v", actual, err)
	}
	record, err := GetResourceEditApproval(input.ProjectDir, state.runID, requested.InteractionID)
	if err != nil {
		t.Fatal(err)
	}
	var userDraft spec.Design
	if err := json.Unmarshal(record.Draft, &userDraft); err != nil {
		t.Fatal(err)
	}
	userDraft.Demands = []string{"User choice"}
	draft, _ := json.Marshal(userDraft)
	record, err = UpdateResourceEditApproval(input.ProjectDir, state.runID, requested.InteractionID, record.Revision, draft)
	if err != nil {
		t.Fatal(err)
	}
	answer := model.ResourceEditApprovalAnswer{InteractionID: requested.InteractionID, CallID: call.ID, Revision: record.Revision, Decision: "approve"}
	if _, err := DecideResourceEditApproval(input.ProjectDir, state.runID, requested.InteractionID, answer, func() error { prompter.answered <- answer; return nil }); err != nil {
		t.Fatal(err)
	}
	var results []ToolResult
	select {
	case results = <-finished:
	case <-ctx.Done():
		t.Fatal("tool did not finish after approval")
	}
	if len(results) != 1 || !results[0].OK || len(results[0].OperationTargets) == 0 {
		t.Fatalf("approved tool result: %+v", results)
	}
	diff, _ := json.Marshal(results[0].OperationTargets)
	if !strings.Contains(string(diff), "User choice") || strings.Contains(string(diff), "Agent suggestion") {
		t.Fatalf("final diff does not reflect edited draft: %s", diff)
	}
	actual, err = os.ReadFile(filepath.Join(input.ProjectDir, ".design.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved spec.Design
	if err := json.Unmarshal(actual, &saved); err != nil || len(saved.Demands) != 1 || saved.Demands[0] != "User choice" {
		t.Fatalf("final draft was not committed: %+v %v", saved, err)
	}
}

func TestResourceApprovalRejectionLeavesSourceUntouched(t *testing.T) {
	input, state, registry := authoringBatchFixture(t, nil)
	path := filepath.Join(input.ProjectDir, ".manifest.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	prompter := waitingResourcePrompter{requested: make(chan model.ResourceEditApprovalRequestedPayload, 1), answered: make(chan model.ResourceEditApprovalAnswer, 1)}
	input.Prompter = prompter
	call := llm.ToolCall{ID: "edit_to_reject", Name: "edit_manifest", Args: map[string]any{"goal": "Agent goal"}}
	finished := make(chan []ToolResult, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		finished <- NewRuntime(nil).executeToolBatch(ctx, input, state, registry,
			schemasByName(registry.Disclose(PhaseExecuting, model.ModeExecute, state.scope)), []llm.ToolCall{call})
	}()
	var requested model.ResourceEditApprovalRequestedPayload
	select {
	case requested = <-prompter.requested:
	case <-ctx.Done():
		t.Fatal("approval was not requested")
	}
	answer := model.ResourceEditApprovalAnswer{InteractionID: requested.InteractionID, CallID: call.ID, Revision: requested.Revision, Decision: "reject", Feedback: "  保留原目标\n不要改写  "}
	if _, err := DecideResourceEditApproval(input.ProjectDir, state.runID, requested.InteractionID, answer, func() error { prompter.answered <- answer; return nil }); err != nil {
		t.Fatal(err)
	}
	select {
	case results := <-finished:
		if len(results) != 1 || results[0].Code != "RESOURCE_EDIT_REJECTED" {
			t.Fatalf("rejected tool result: %+v", results)
		}
		var observation map[string]any
		if err := json.Unmarshal([]byte(modelToolObservation(results[0])), &observation); err != nil || observation["feedback"] != answer.Feedback || results[0].OK {
			t.Fatalf("rejection feedback missing: %v %v", observation, err)
		}
	case <-ctx.Done():
		t.Fatal("tool did not finish after rejection")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("rejected edit changed source: %q %v", after, err)
	}
}

func TestResourceApprovalChangedSourceFailsWithoutOverwriting(t *testing.T) {
	input, state, registry := authoringBatchFixture(t, nil)
	path := filepath.Join(input.ProjectDir, ".manifest.json")
	prompter := waitingResourcePrompter{requested: make(chan model.ResourceEditApprovalRequestedPayload, 1), answered: make(chan model.ResourceEditApprovalAnswer, 1)}
	input.Prompter = prompter
	call := llm.ToolCall{ID: "edit_with_conflict", Name: "edit_manifest", Args: map[string]any{"goal": "Agent goal"}}
	finished := make(chan []ToolResult, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		finished <- NewRuntime(nil).executeToolBatch(ctx, input, state, registry,
			schemasByName(registry.Disclose(PhaseExecuting, model.ModeExecute, state.scope)), []llm.ToolCall{call})
	}()
	var requested model.ResourceEditApprovalRequestedPayload
	select {
	case requested = <-prompter.requested:
	case <-ctx.Done():
		t.Fatal("approval was not requested")
	}
	manifest := spec.DefaultManifest("Changed elsewhere")
	changed, _ := json.Marshal(manifest)
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	answer := model.ResourceEditApprovalAnswer{InteractionID: requested.InteractionID, CallID: call.ID, Revision: requested.Revision, Decision: "approve"}
	prompter.answered <- answer
	select {
	case results := <-finished:
		if len(results) != 1 || results[0].Code != CodeContentConflict {
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

func TestResourceApprovalDraftDoesNotWriteSourceAndChecksRevision(t *testing.T) {
	root := t.TempDir()
	base := spec.DefaultManifest("Deck")
	before, _ := json.Marshal(base)
	path := filepath.Join(root, ".manifest.json")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	proposal := base
	proposal.Goal = "Agent goal"
	proposed, _ := json.Marshal(proposal)
	record := ResourceEditApproval{RunID: "run", CallID: "call", InteractionID: resourceApprovalID("call"),
		Resource: "manifest", Revision: 1, BaseExists: true, BaseHash: hashBytes(before), Base: before,
		Proposal: proposed, Draft: proposed, Target: model.PublicTarget{Type: "deck", Part: "manifest"}, State: "pending"}
	if err := saveResourceApprovalLocked(root, record); err != nil {
		t.Fatal(err)
	}
	changed := proposal
	changed.Goal = "User goal"
	draft, _ := json.Marshal(changed)
	updated, err := UpdateResourceEditApproval(root, "run", record.InteractionID, 1, draft)
	if err != nil || updated.Revision != 2 || updated.Target.Diff == nil {
		t.Fatalf("draft update: %+v %v", updated, err)
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != string(before) {
		t.Fatalf("approval draft changed source: %q %v", actual, err)
	}
	if _, err := UpdateResourceEditApproval(root, "run", record.InteractionID, 1, draft); !errors.Is(err, ErrResourceApprovalConflict) {
		t.Fatalf("stale draft revision accepted: %v", err)
	}
	answer := model.ResourceEditApprovalAnswer{InteractionID: record.InteractionID, CallID: "call", Revision: 2, Decision: "approve"}
	submitted := 0
	for i := 0; i < 2; i++ {
		if _, err := DecideResourceEditApproval(root, "run", record.InteractionID, answer, func() error { submitted++; return nil }); err != nil {
			t.Fatal(err)
		}
	}
	if submitted != 1 {
		t.Fatalf("approval submitted %d times", submitted)
	}
	if err := FinalizeResourceEditApprovals(root, "run"); err != nil {
		t.Fatal(err)
	}
	retained, err := GetResourceEditApproval(root, "run", record.InteractionID)
	if err != nil || retained.State != "answered" || string(retained.Draft) != "null" {
		t.Fatalf("finalized approval receipt: %+v %v", retained, err)
	}
	if _, err := DecideResourceEditApproval(root, "run", record.InteractionID, answer, func() error {
		t.Fatal("finalized approval replay must not submit again")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestResourceApprovalRejectsChangedBaseline(t *testing.T) {
	root := t.TempDir()
	manifest := spec.DefaultManifest("Deck")
	base, _ := json.Marshal(manifest)
	path := filepath.Join(root, ".manifest.json")
	if err := os.WriteFile(path, base, 0o600); err != nil {
		t.Fatal(err)
	}
	record := ResourceEditApproval{RunID: "run", CallID: "call", InteractionID: resourceApprovalID("call"),
		Resource: "manifest", Revision: 1, BaseExists: true, BaseHash: hashBytes(base), Base: base,
		Proposal: base, Draft: base, Target: model.PublicTarget{Type: "deck", Part: "manifest"}, State: "pending"}
	if err := saveResourceApprovalLocked(root, record); err != nil {
		t.Fatal(err)
	}
	manifest.Goal = "Changed elsewhere"
	changed, _ := json.Marshal(manifest)
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := DecideResourceEditApproval(root, "run", record.InteractionID,
		model.ResourceEditApprovalAnswer{InteractionID: record.InteractionID, CallID: "call", Revision: 1, Decision: "approve"},
		func() error { t.Fatal("stale approval must not reach runtime"); return nil }); !errors.Is(err, ErrResourceApprovalConflict) {
		t.Fatalf("stale baseline accepted: %v", err)
	}
	if err := FinalizeResourceEditApprovals(root, "run"); err != nil {
		t.Fatal(err)
	}
	if _, err := GetResourceEditApproval(root, "run", record.InteractionID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending draft survived terminal cleanup: %v", err)
	}
}
