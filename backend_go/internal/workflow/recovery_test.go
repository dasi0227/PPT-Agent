package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func TestCommandNodeRollbackRestoresDirectoriesModesAndLinks(t *testing.T) {
	dir := t.TempDir()
	_ = os.Mkdir(filepath.Join(dir, "old"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "old/script"), []byte("before"), 0755)
	before, err := commandexec.Snapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	session, _ := NewRunSession(dir, "rollback")
	defer session.Discard()
	session.commandBaseline = before.Hash()
	session.stageCommandNode("old/script", before, nil)
	session.stageCommandNode("old", before, nil)
	session.stageCommandNode("new", before, &commandexec.Node{Kind: "directory", Mode: 0755})
	session.stageCommandNode("new/link", before, &commandexec.Node{Kind: "symlink", Mode: 0777, Content: []byte("../old/script")})
	_, err = session.CommitOperation(context.Background(), "rollback", "", func(context.Context, CommitContext) error { return errors.New("metadata failure") })
	if err == nil {
		t.Fatal("expected metadata failure")
	}
	after, err := commandexec.Snapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if after.Hash() != before.Hash() {
		t.Fatal("rollback did not restore the original tree")
	}
}

func TestReplayedRequirementsAdvanceAuthoringViewWithoutRewritingNewerFiles(t *testing.T) {
	htmlDesign := ""
	input, state, registry := authoringBatchFixture(t, func(_ context.Context, input DomainToolInput) error {
		if input.CallID == "recovered_html" {
			htmlDesign = input.Context.Constraints.Constraints.Visual.Requirements[0]
		}
		return nil
	})
	frozen := cloneAuthoringPack(state.pack)
	versions := maps.Clone(state.seenVersions)
	receipts := newMemoryIdempotencyStore()
	input.Idempotency = receipts
	commits := 0
	input.CommitMetadata = func(ctx context.Context, c CommitContext) error {
		commits++
		return receipts.saveArtifactCommit(ctx, state.runID, c)
	}
	calls := []llm.ToolCall{
		{ID: "saved_design", Name: "edit_constraints", Args: map[string]any{"type": "visual", "requirements": []any{"B"}}},
		{ID: "saved_spec", Name: "edit_spec", Args: map[string]any{"slide_id": generationSlide, "core": "B"}},
	}
	runtime := newTestRuntime(nil)
	names := schemasByName(registry.Disclose(PhaseExecuting, model.ModeCreate, state.scope))
	savedContents := make([][]byte, len(calls))
	calls = append(calls, llm.ToolCall{ID: "recovered_html", Name: "edit_html", Args: map[string]any{
		"slide_id": generationSlide, "content": strings.Replace(generationHTML, "Original", "Recovered", 1),
	}})
	input = runtimeFixtureInput(input)
	input.publicUpdates = &publicUpdates{source: input.Emitter}
	input.Emitter = input.publicUpdates
	var contextErr error
	state.contextSession, contextErr = input.Transcript.Open(context.Background(), input.Context.Manifest.ThreadID, nil)
	if contextErr != nil {
		t.Fatal(contextErr)
	}
	if err := runtime.prepareToolBatch(context.Background(), input, state, registry, calls, ""); err != nil {
		t.Fatal(err)
	}
	// Keep one durable store across preparation, the partial execution, and
	// recovery. The third prepared call has not executed at the crash boundary.
	savedResults := runtime.Runtime.executeToolBatch(context.Background(), input, state, registry, names, calls[:2])
	for i, result := range savedResults {
		if !result.OK {
			t.Fatalf("initial write: %+v", result)
		}
		resource := resourceForTool(calls[i].Name, stringValue(calls[i].Args["slide_id"]))
		ref, _ := refForResource(state.pack, resource)
		var err error
		savedContents[i], err = os.ReadFile(filepath.Join(input.ProjectDir, ref.Path))
		if err != nil {
			t.Fatal(err)
		}
		assertSavedResourceObservation(t, calls[i].Name, result, savedContents[i])
	}
	// Newer disk requirements must neither replace replayed observations nor
	// be overwritten by replaying their already committed calls.
	design := *state.pack.Constraints.Constraints
	design.Visual.Requirements = []string{"C"}
	raw, _ := json.Marshal(design)
	writeGenerationFile(t, input.ProjectDir, ".constraints.json", raw)
	slide := spec.SlideSpec{Purpose: "content_explanation", Layout: "Title above content", Core: "B", Elements: []spec.Element{}}
	slide.Core = "C"
	raw, _ = json.Marshal(slide)
	writeGenerationFile(t, input.ProjectDir, model.SlideSpecPath(generationSlide), raw)
	recovered := batchState(frozen)
	recovered.tx, recovered.messages, recovered.seenVersions = state.tx, state.messages, versions
	recovered.contextSession = state.contextSession
	recovered.toolExecutions = state.toolExecutions
	recovered.pendingToolBatch, recovered.tools = state.pendingToolBatch, registry
	if err := runtime.resumeToolBatch(context.Background(), input, recovered); err != nil {
		t.Fatal(err)
	}
	recoveredResults := recordedToolResults(t, recovered, calls)
	for i, result := range recoveredResults {
		if !result.OK {
			t.Fatalf("recovered call: %+v", result)
		}
		if i < len(savedContents) {
			assertSavedResourceObservation(t, calls[i].Name, result, savedContents[i])
			if result.Observation != savedResults[i].Observation || !reflect.DeepEqual(result.ObservationMetadata, savedResults[i].ObservationMetadata) || !reflect.DeepEqual(result.ChangedTargets, savedResults[i].ChangedTargets) {
				t.Fatalf("replay lost saved observation, versions or changes: %+v", result)
			}
		}
	}
	if commits != 3 || htmlDesign != "B" {
		t.Fatalf("replay advanced the wrong view or wrote again: commits=%d design=%s", commits, htmlDesign)
	}
	if disk, _ := os.ReadFile(filepath.Join(input.ProjectDir, ".constraints.json")); !strings.Contains(string(disk), `"C"`) {
		t.Fatal("replay overwrote newer design")
	}
	if disk, _ := os.ReadFile(filepath.Join(input.ProjectDir, model.SlideSpecPath(generationSlide))); !strings.Contains(string(disk), `"C"`) {
		t.Fatal("replay overwrote newer spec")
	}
}

func TestUnknownArtifactCommitPausesWithoutPublishingOrRepeatingTool(t *testing.T) {
	executed := 0
	input, state, registry := authoringBatchFixture(t, func(context.Context, DomainToolInput) error { executed++; return nil })
	checkpoint := &recoveryPauseRecorder{}
	events := &eventRecorder{}
	input.Checkpoint, input.Emitter = checkpoint, events
	input.CommitMetadata = func(context.Context, CommitContext) error {
		return errors.Join(pptmutation.ErrRecoveryRequired, context.Canceled)
	}
	calls := []llm.ToolCall{
		{ID: "unknown", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "content": strings.Replace(generationHTML, "Original", "Unconfirmed", 1)}},
		{ID: "next", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "content": strings.Replace(generationHTML, "Original", "Repeated", 1)}},
	}
	before := len(state.messages)
	runtime := newTestRuntime(nil)
	runtime.executeToolBatch(context.Background(), input, state, registry, schemasByName(registry.Disclose(PhaseExecuting, model.ModeCreate, state.scope)), calls)
	if !errors.Is(state.toolRecoveryErr, pptmutation.ErrRecoveryRequired) || checkpoint.reason != "tool_outcome_unknown" || executed != 1 {
		t.Fatalf("unknown commit did not stop: %v reason=%q executions=%d", state.toolRecoveryErr, checkpoint.reason, executed)
	}
	if state.pendingToolBatch == nil || len(state.messages) != before+1 || state.toolExecutions["unknown"].Terminal() || state.toolExecutions["next"].Status != "prepared" {
		t.Fatalf("unknown outcome consumed its pending batch: %+v %+v", state.pendingToolBatch, state.toolExecutions)
	}
	for _, event := range events.events {
		if event.kind == model.EventToolCompleted {
			t.Fatal("unknown commit published a fabricated tool outcome")
		}
	}
	if raw, err := os.ReadFile(filepath.Join(input.ProjectDir, model.SlideHTMLPath(generationSlide))); err != nil || !strings.Contains(string(raw), "Unconfirmed") {
		t.Fatalf("unknown commit lost its postimage: %q %v", raw, err)
	}
	checkpoints := len(checkpoint.checkpoints)
	runtime.fail(input, state, model.CodeInternal, state.toolRecoveryErr)
	if len(checkpoint.checkpoints) != checkpoints {
		t.Fatal("failure overwrote the recovery checkpoint")
	}
}

func TestUnknownToolOutcomeKeepsPendingBatchAndDoesNotFabricateObservation(t *testing.T) {
	for _, status := range []string{"outcome_unknown", "executing"} {
		t.Run(status, func(t *testing.T) {
			call := llm.ToolCall{ID: "call", Name: "edit_html"}
			pending := &PendingToolBatch{Calls: []llm.ToolCall{call}, CallIDs: []string{call.ID}}
			state := &RunState{runID: "run", pendingToolBatch: pending, ledger: NewEvidenceLedger(), activeSkills: &ActiveSkillSet{},
				messages:       []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("input")}, {Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{call}}},
				toolExecutions: map[string]sessionstate.ToolExecution{call.ID: testToolExecution(t, "run", call, sessionstate.ToolUnknown, status)}}
			checkpoint := &recoveryPauseRecorder{}
			events := &eventRecorder{}
			input := RuntimeInput{Checkpoint: checkpoint, Emitter: events}
			err := newTestRuntime(nil).resumeToolBatch(context.Background(), input, state)
			if !errors.Is(err, ErrToolOutcomeUnknown) || checkpoint.reason != "tool_outcome_unknown" {
				t.Fatalf("unknown outcome did not pause: %v %q", err, checkpoint.reason)
			}
			if state.pendingToolBatch == nil || len(state.messages) != 2 || len(events.events) != 0 {
				t.Fatal("unknown tool became a fabricated observation")
			}
			before := len(checkpoint.checkpoints)
			newTestRuntime(nil).fail(input, state, model.CodeInternal, err)
			if len(checkpoint.checkpoints) != before {
				t.Fatal("failure overwrote the recoverable checkpoint")
			}
		})
	}
}

func TestAuthoringCancellationRetainsIndependentCommitAndReceipt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input, state, registry := authoringBatchFixture(t, func(ctx context.Context, input DomainToolInput) error {
		if input.CallID == "waiting" {
			<-ctx.Done()
			return ctx.Err()
		}
		return nil
	})
	receipts := newMemoryIdempotencyStore()
	input.Idempotency = receipts
	input.CommitMetadata = func(ctx context.Context, c CommitContext) error {
		if err := receipts.saveArtifactCommit(ctx, state.runID, c); err != nil {
			return err
		}
		cancel()
		return nil
	}
	calls := []llm.ToolCall{
		{ID: "waiting", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "content": strings.Replace(generationHTML, "Original", "A", 1)}},
		{ID: "committed", Name: "edit_html", Args: map[string]any{"slide_id": otherBatchSlide, "content": strings.Replace(generationHTML, "Original", "B", 1)}},
	}
	results := newTestRuntime(nil).executeToolBatch(ctx, input, state, registry, schemasByName(registry.Disclose(PhaseExecuting, model.ModeCreate, state.scope)), calls)
	if results[0].Code != CodeCanceled || !results[1].OK {
		t.Fatalf("cancellation crossed commit boundary: %+v", results)
	}
	a, _ := os.ReadFile(filepath.Join(input.ProjectDir, model.SlideHTMLPath(generationSlide)))
	b, _ := os.ReadFile(filepath.Join(input.ProjectDir, model.SlideHTMLPath(otherBatchSlide)))
	if string(a) != generationHTML || string(b) != calls[1].Args["content"] {
		t.Fatal("cancellation discarded independent persisted HTML")
	}
	receipt, err := receipts.GetIdempotency(context.Background(), "artifact_commit", state.runID, "committed")
	if err != nil || receipt.Status != "completed" || receipt.ResultJSON == "" {
		t.Fatalf("committed receipt lost: %+v %v", receipt, err)
	}
}
