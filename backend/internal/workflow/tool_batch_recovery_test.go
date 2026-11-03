package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

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

type recoveryPauseRecorder struct {
	checkpointRecorder
	reason string
}

func (r *recoveryPauseRecorder) PersistenceFailed(_ context.Context, reason string) {
	r.reason = reason
}

func testToolExecution(t *testing.T, runID string, call llm.ToolCall, policy sessionstate.ToolRecoveryPolicy, status string) sessionstate.ToolExecution {
	t.Helper()
	hash, args, err := sessionstate.ToolArgumentsHash(call.Name, call.Args)
	if err != nil {
		t.Fatal(err)
	}
	return sessionstate.ToolExecution{ID: sessionstate.ToolAttemptID(runID, call.ID, 1), RunID: runID, CallID: call.ID, AttemptNo: 1, Name: call.Name, Arguments: args, ArgumentsHash: hash, Status: status, RecoveryPolicy: policy, EntityRevision: 2}
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

func TestUnknownReadOnlyToolCreatesNewAttemptAndOnePairedResult(t *testing.T) {
	pack := testPack(model.ModeCreate, model.ScopeAllPages, true, "read")
	state := batchState(pack)
	state.tools = NewToolRegistry()
	if err := state.tools.Register(timedBatchTool{name: "read_resource"}, true, CapabilityRead, RiskLow, PhaseExecuting); err != nil {
		t.Fatal(err)
	}
	call := llm.ToolCall{ID: "call", Name: "read_resource", Args: map[string]any{"id": "read"}}
	state.pendingToolBatch = &PendingToolBatch{Calls: []llm.ToolCall{call}, CallIDs: []string{call.ID}}
	state.messages = []llm.Message{{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{call}}}
	state.toolExecutions = map[string]sessionstate.ToolExecution{call.ID: testToolExecution(t, state.runID, call, sessionstate.ToolReadOnly, "outcome_unknown")}
	if err := newTestRuntime(nil).resumeToolBatch(context.Background(), RuntimeInput{ProjectDir: t.TempDir(), Context: pack}, state); err != nil {
		t.Fatal(err)
	}
	row := state.toolExecutions[call.ID]
	if row.AttemptNo != 2 || row.Status != "succeeded" || state.pendingToolBatch != nil || len(state.messages) != 2 || state.messages[1].ToolCallID != call.ID {
		t.Fatalf("unsafe retry/pairing: %+v %+v", row, state.messages)
	}
}
