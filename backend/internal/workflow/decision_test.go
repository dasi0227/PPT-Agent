package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/decision"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type decisionFunc func(context.Context, decision.Request) (decision.Response, error)

func (f decisionFunc) Evaluate(ctx context.Context, req decision.Request) (decision.Response, error) {
	return f(ctx, req)
}

func TestToolDecisionIsConservativeAndSurvivesRegistryRebuild(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	calls := 0
	provider := decisionFunc(func(_ context.Context, req decision.Request) (decision.Response, error) {
		calls++
		if len(req.Questions) != 3 || req.Questions["content"] == nil || req.Questions["visual"] == nil || req.Questions["outline"] == nil || req.Questions["commit"] != nil {
			t.Fatal("capabilities were not batched")
		}
		return decision.Response{Model: "jev-test", Answers: map[string]decision.Answer{
			"content": decision.ChoiceAnswer{Type: "choice", Choice: "not_needed", Confidence: 1, Probabilities: map[string]float64{"not_needed": 1}},
			"visual":  decision.ChoiceAnswer{Type: "choice", Choice: "possible", Confidence: 1, Probabilities: map[string]float64{"possible": 1}},
			"outline": decision.ChoiceAnswer{Type: "choice", Choice: "not_needed", Confidence: 0.3, Probabilities: map[string]float64{"not_needed": 0.6}},
		}}, nil
	})
	runtime := NewRuntime(nil)
	runtime.Decisions = decision.Snapshot{Identity: "one", Provider: provider}
	input := RuntimeInput{ProjectDir: dir, Context: pack, DomainTools: DefaultDomainToolProvider{}}
	state := &RunState{pack: pack, mode: model.ModeExecute, phase: PhaseExecuting, scope: pack.Command.Scope, decisionIdentity: "one"}
	state.tools, _ = buildDomainToolRegistry(input, pack)
	if schemasByName(state.tools.Disclose(state.phase, state.mode, state.scope))["git_commit"] {
		t.Fatal("git_commit must not be available before pruning")
	}
	if err := runtime.decideTools(context.Background(), input, state); err != nil {
		t.Fatal(err)
	}
	names := schemasByName(state.tools.Disclose(state.phase, state.mode, state.scope))
	if names["edit_manifest"] || names["git_commit"] || !names["edit_design"] || !names["edit_outline"] || !names["edit_spec"] {
		t.Fatalf("incorrect pruning: %v", names)
	}
	state.tools, _ = buildDomainToolRegistry(input, pack)
	runtime.Decisions = decision.Snapshot{Identity: "changed", Provider: provider}
	if err := runtime.decideTools(context.Background(), input, state); err != nil {
		t.Fatal(err)
	}
	names = schemasByName(state.tools.Disclose(state.phase, state.mode, state.scope))
	if calls != 1 || names["edit_manifest"] || names["git_commit"] {
		t.Fatal("same run reclassified or reopened tools")
	}
	for _, name := range []string{"edit_manifest", "git_commit"} {
		result := state.tools.Execute(context.Background(), map[string]bool{name: true}, name, nil, DomainToolInput{Context: pack, Scope: state.scope, Mode: state.mode, Phase: state.phase})
		if result.OK || result.Code != ErrToolNotDisclosed.Error() {
			t.Fatalf("old disclosed name %s bypassed frozen list", name)
		}
	}
}

type batchReceiptStore struct {
	records map[string]model.IdempotencyRecord
}

func (s batchReceiptStore) AcquireIdempotency(context.Context, model.IdempotencyRecord) (model.IdempotencyRecord, bool, error) {
	panic("recovery must not acquire or replay a tool")
}
func (s batchReceiptStore) CompleteIdempotency(context.Context, string, string, string, string, string) error {
	panic("recovery must not rewrite receipts")
}
func (s batchReceiptStore) GetIdempotency(_ context.Context, scope, owner, key string) (model.IdempotencyRecord, error) {
	v, ok := s.records[scope+":"+key]
	if !ok {
		return v, errors.New("missing")
	}
	return v, nil
}

func TestPendingToolBatchRecoversReceiptsWithoutReplayingEdits(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	final := []byte(strings.Replace(generationHTML, "Original", "Committed", 1))
	writeGenerationFile(t, dir, model.SlideHTMLPath(generationSlide), final)
	saved := SuccessfulToolResult("saved")
	saved.ChangedTargets = []ChangedTarget{{Type: "slide", Part: "html", SlideID: generationSlide, Hash: hashBytes(final)}}
	raw, err := marshalPersistedToolResult(saved, nil)
	if err != nil {
		t.Fatal(err)
	}
	refused := refusedResourceEditResult()
	setApprovalFeedback(&refused, "Keep the original", "recover-check", "refused", "resource_edit_feedback")
	refusedRaw, err := marshalPersistedToolResult(refused, nil)
	if err != nil {
		t.Fatal(err)
	}
	receipts := batchReceiptStore{records: map[string]model.IdempotencyRecord{
		"tool_call:refused": {Status: "completed", ResultJSON: refusedRaw},
		"tool_call:write":   {Status: "completed", ResultJSON: raw}, "artifact_commit:write": {Status: "completed"},
	}}
	state := &RunState{runID: "recover-check", projectDir: dir, pack: pack, decisionIdentity: "old", pendingToolBatch: &PendingToolBatch{Calls: []llm.ToolCall{{ID: "write", Name: "edit_html"}, {ID: "unstarted", Name: "edit_html"}, {ID: "refused", Name: "edit_manifest"}}}}
	runtime := NewRuntime(nil)
	events := &eventRecorder{}
	if err := runtime.resumeToolBatch(context.Background(), RuntimeInput{ProjectDir: dir, Idempotency: receipts, Emitter: events}, state); err != nil {
		t.Fatal(err)
	}
	if state.pendingToolBatch != nil || len(state.messages) != 4 {
		t.Fatal("recovered batch was not observed")
	}
	var completed, interrupted map[string]any
	if err := json.Unmarshal([]byte(state.messages[1].Text()), &completed); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(state.messages[2].Text()), &interrupted); err != nil {
		t.Fatal(err)
	}
	if state.messages[1].ToolCallID != "write" || completed["status"] != "success" ||
		state.messages[2].ToolCallID != "unstarted" || interrupted["status"] != "error" || interrupted["code"] != CodeCanceled {
		t.Fatal("recovery lost a successful receipt or replayed an unstarted call")
	}
	var refusal map[string]any
	if err := json.Unmarshal([]byte(state.messages[3].Text()), &refusal); err != nil || refusal["status"] != "refuse" || refusal["decision"] != nil || refusal["content"] != nil || refusal["feedback"] != "Keep the original" {
		t.Fatalf("refusal receipt lost during recovery: %v (%v)", refusal, err)
	}
	if len(events.events) != 1 {
		t.Fatalf("refusal terminal was not recovered: %+v", events.events)
	}
	terminal := events.events[0].payload.(model.ToolCompletedPayload)
	if terminal.CallID != "refused" || terminal.Status != "refuse" || terminal.Changes != nil || terminal.Error != nil {
		t.Fatalf("recovery counted refusal as a saved edit: %+v", terminal)
	}
	current, err := os.ReadFile(filepath.Join(dir, model.SlideHTMLPath(generationSlide)))
	if err != nil || string(current) != string(final) {
		t.Fatal("batch recovery changed the saved artifact")
	}
}
