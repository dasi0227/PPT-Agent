package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

var ErrToolOutcomeUnknown = errors.New("tool outcome is unknown")

// Durable batch intent. Recovery consumes receipts without re-executing calls.
type PendingToolBatch struct {
	CallIDs            []string       `json:"call_ids"`
	AssistantMessageID string         `json:"assistant_message_id"`
	Calls              []llm.ToolCall `json:"-"`
	AssistantText      string         `json:"-"`
}

func (r *Runtime) resumeToolBatch(ctx context.Context, input RuntimeInput, state *RunState) error {
	pending := state.pendingToolBatch
	if pending == nil {
		return nil
	}
	for _, call := range pending.Calls {
		row, ok := state.toolExecutions[call.ID]
		if !ok {
			return sessionstate.ErrInvalidState
		}
		if row.Terminal() || row.Status == "prepared" {
			continue
		}
		if row.Status == "executing" {
			row.Status = "outcome_unknown"
			candidate := *state
			candidate.toolChanges = []sessionstate.ToolExecution{row}
			if err := r.commitToolState(ctx, input, state, candidate, "tool_interrupted"); err != nil {
				return err
			}
			row = state.toolExecutions[call.ID]
		}
		if row.Status != "outcome_unknown" {
			return sessionstate.ErrInvalidState
		}
		if row.RecoveryPolicy == sessionstate.ToolReconcile && input.Idempotency != nil {
			receipt, err := input.Idempotency.GetIdempotency(ctx, "artifact_commit", state.runID, row.OperationID)
			if err != nil && !errors.Is(err, model.ErrRunNotFound) {
				return err
			}
			if err == nil {
				if receipt.Status != "completed" || !validToolReceipt(receipt.ResultJSON) {
					return sessionstate.ErrInvalidState
				}
				var stored persistedToolResult
				if err := json.Unmarshal([]byte(receipt.ResultJSON), &stored); err != nil {
					return err
				}
				result := stored.restoreResult()
				projector := ToolPublicProjector{ProjectDir: input.ProjectDir, TextContext: state.publicTextContext()}
				if input.Emitter != nil {
					if event, ok := projector.Completed(state.runID, call.ID, call.Name, call.Args, result); ok {
						input.Emitter.Emit(model.EventToolCompleted, event)
					}
				}
				if err := r.persistToolCall(ctx, input, state, call, result); err != nil {
					return err
				}
				continue
			}
		}
		if row.RecoveryPolicy == sessionstate.ToolReadOnly {
			row.AttemptNo++
			row.ID = sessionstate.ToolAttemptID(state.runID, call.ID, row.AttemptNo)
			row.Status, row.EntityRevision = "prepared", 0
			candidate := *state
			candidate.toolChanges = []sessionstate.ToolExecution{row}
			if err := r.commitToolState(ctx, input, state, candidate, "tool_retry_prepared"); err != nil {
				return err
			}
			continue
		}
		if approval := state.pendingCommand; approval != nil && approval.CallID == call.ID && approval.Prepared != nil {
			continue
		}
		if approval := state.pendingResourceApproval; approval != nil && approval.CallID == call.ID && approval.Candidate != nil {
			continue
		}
		return stopForUnknownTool(ctx, input, state, call.ID)
	}
	allTerminal := true
	for _, id := range pending.CallIDs {
		if !state.toolExecutions[id].Terminal() {
			allTerminal = false
		}
	}
	if !allTerminal && (state.pendingCommand != nil || state.pendingResourceApproval != nil) {
		return nil
	}
	results := r.executeToolBatch(ctx, input, state, state.tools, schemasByName(state.tools.Disclose(state.phase, state.mode, state.scope)), pending.Calls)
	if err := toolExecutionStopError(input, state); err != nil {
		return err
	}
	if err := r.finishToolBatch(ctx, input, state); err != nil {
		return err
	}
	state.rememberResourceVersions()
	recordToolFailures(state, results)
	if failure := fatalRuntimeToolError(results); failure != nil {
		return failure
	}
	return nil
}

// Do not turn a missing receipt into a canceled model observation. The durable
// pending batch remains the recovery position until its effects are reconciled.
func stopForUnknownTool(ctx context.Context, input RuntimeInput, state *RunState, callID string) error {
	err := fmt.Errorf("%w: %s", ErrToolOutcomeUnknown, callID)
	return stopForToolRecovery(ctx, input, state, err)
}

func stopForToolRecovery(ctx context.Context, input RuntimeInput, state *RunState, err error) error {
	state.toolRecoveryErr = err
	if pauser, ok := input.Checkpoint.(interface {
		PersistenceFailed(context.Context, string)
	}); ok {
		pauser.PersistenceFailed(context.WithoutCancel(ctx), "tool_outcome_unknown")
	}
	return err
}

func toolExecutionStopError(input RuntimeInput, state *RunState) error {
	if state == nil {
		return executionStopError(input)
	}
	if state.persistenceErr != nil {
		return state.persistenceErr
	}
	if state.toolRecoveryErr != nil {
		return state.toolRecoveryErr
	}
	return executionStopError(input)
}
