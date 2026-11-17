package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

func (r *Runtime) commitToolState(ctx context.Context, input RuntimeInput, state *RunState, candidate RunState, boundary string) error {
	candidate.toolExecutions = maps.Clone(state.toolExecutions)
	if candidate.toolExecutions == nil {
		candidate.toolExecutions = map[string]sessionstate.ToolExecution{}
	}
	if input.Checkpoint != nil {
		if err := r.saveCheckpoint(ctx, input, &candidate, checkpointBoundary(boundary)); err != nil {
			state.persistenceErr = candidate.persistenceErr
			return err
		}
	} else {
		for _, row := range candidate.toolChanges {
			row.EntityRevision++
			candidate.toolExecutions[row.CallID] = row
		}
		candidate.toolChanges = nil
	}
	*state = candidate
	return nil
}

func (r *Runtime) prepareToolBatch(ctx context.Context, input RuntimeInput, state *RunState, registry *ToolRegistry, calls []llm.ToolCall, text string) error {
	if state.pendingToolBatch != nil {
		return sessionstate.ErrInvalidState
	}
	if input.Checkpoint != nil && (state.contextSession == nil || input.ToolExecutions == nil) {
		return sessionstate.ErrInvalidState
	}
	candidate := *state
	candidate.toolCalls += len(calls)
	pending := &PendingToolBatch{Calls: make([]llm.ToolCall, len(calls)), AssistantText: text, CallIDs: make([]string, len(calls))}
	seen := map[string]bool{}
	for i, call := range calls {
		if call.ID == "" || seen[call.ID] {
			return sessionstate.ErrInvalidState
		}
		seen[call.ID] = true
		policy := sessionstate.ToolUnknown
		if desc, ok := registry.Descriptor(call.Name); ok {
			policy = desc.RecoveryPolicy
			if args, converted, err := prepareToolArguments(desc.Tool.Schema(), call.Args); err == nil {
				call.Args = args
				traceArgumentConversion(input.Trace, state.runID, call, converted)
			}
		}
		if _, ok := state.toolExecutions[call.ID]; ok {
			return sessionstate.ErrIdempotencyConflict
		}
		if input.ToolExecutions != nil {
			if _, err := input.ToolExecutions.GetToolExecution(ctx, state.runID, call.ID); err == nil {
				return sessionstate.ErrIdempotencyConflict
			} else if !errors.Is(err, model.ErrRunNotFound) {
				return err
			}
		}
		hash, raw, err := sessionstate.ToolArgumentsHash(call.Name, call.Args)
		if err != nil {
			return err
		}
		row := sessionstate.ToolExecution{ID: sessionstate.ToolAttemptID(state.runID, call.ID, 1), RunID: state.runID, CallID: call.ID, AttemptNo: 1, Name: call.Name, Arguments: raw, ArgumentsHash: hash, Status: "prepared", RecoveryPolicy: policy}
		if policy == sessionstate.ToolReconcile {
			row.OperationID = call.ID
		}
		candidate.toolChanges = append(candidate.toolChanges, row)
		pending.Calls[i], pending.CallIDs[i] = call, call.ID
		calls[i] = call
	}
	candidate.pendingToolBatch = pending
	candidate.messages = append(slices.Clone(state.messages), llm.Message{Role: llm.RoleAssistant, Content: llm.TextContent(text), ToolCalls: pending.Calls, Replay: state.pendingReplay})
	candidate.pendingReplay = nil
	return r.commitToolState(ctx, input, state, candidate, "tool_batch_prepared")
}

// Command preflight can execute in an isolated workspace. Its durable dispatch
// is separate from the public start, which still follows the existing policy.
func (r *Runtime) dispatchToolCall(ctx context.Context, input RuntimeInput, state *RunState, call llm.ToolCall, publishStart bool) error {
	if err := toolExecutionStopError(input, state); err != nil {
		return err
	}
	row, ok := state.toolExecutions[call.ID]
	if !ok {
		return sessionstate.ErrInvalidState
	}
	if row.Terminal() {
		return sessionstate.ErrVersionConflict
	}
	if row.Status == "executing" && !publishStart {
		return nil
	}
	if row.Status != "prepared" && row.Status != "outcome_unknown" && row.Status != "executing" {
		return sessionstate.ErrVersionConflict
	}
	changes := []sessionstate.ToolExecution(nil)
	if row.Status != "executing" {
		row.Status = "executing"
		changes = []sessionstate.ToolExecution{row}
	}
	if publishStart && input.Emitter != nil {
		projector := ToolPublicProjector{ProjectDir: input.ProjectDir, TextContext: state.publicTextContext()}
		if event, ok := projector.Started(state.runID, call.ID, call.Name, call.Args, nil); ok {
			input.Emitter.Emit(model.EventToolStarted, event)
		}
	}
	candidate := *state
	candidate.toolChanges = changes
	return r.commitToolState(ctx, input, state, candidate, "tool_dispatched")
}

func storedExecutionResult(row sessionstate.ToolExecution, state *RunState) (ToolResult, error) {
	if !row.Terminal() || !validToolReceipt(string(row.ResultJSON)) {
		return ToolResult{}, sessionstate.ErrInvalidState
	}
	var stored persistedToolResult
	if err := json.Unmarshal(row.ResultJSON, &stored); err != nil {
		return ToolResult{}, err
	}
	state.restoreResourceActivation(stored)
	return stored.restoreResult(), nil
}

// The tool result row, its native observation, public outcome and checkpoint
// are candidates for one commit. A failed commit publishes none of them.
func (r *Runtime) persistToolCall(ctx context.Context, input RuntimeInput, state *RunState, call llm.ToolCall, result ToolResult) error {
	if err := toolExecutionStopError(input, state); err != nil {
		return err
	}
	row, ok := state.toolExecutions[call.ID]
	if !ok {
		return sessionstate.ErrInvalidState
	}
	if row.Terminal() {
		return nil
	}
	result = bindToolObservation(result, call)
	raw, err := marshalPersistedToolResult(result, state.activeSkills)
	if err != nil {
		return err
	}
	row.ResultJSON = json.RawMessage(raw)
	row.Status = "failed"
	if result.OK {
		row.Status = "succeeded"
	} else if result.Code == CodeCanceled {
		row.Status = "canceled"
	}
	candidate := *state
	candidate.toolChanges = []sessionstate.ToolExecution{row}
	candidate.messages = slices.Clone(state.messages)
	observation := appendToolObservation(nil, call, "", result)[1]
	pending := state.pendingToolBatch
	if pending == nil {
		return sessionstate.ErrInvalidState
	}
	// Preserve immutable assistant/native IDs and insert results in call order.
	index := -1
	for i, m := range candidate.messages {
		if m.Role == llm.RoleAssistant && len(m.ToolCalls) > 0 && m.ToolCalls[0].ID == pending.CallIDs[0] {
			index = i + 1
		}
	}
	if index < 0 {
		return sessionstate.ErrInvalidState
	}
	for i := index; i < len(candidate.messages) && candidate.messages[i].Role == llm.RoleTool; i++ {
		if candidate.messages[i].ToolCallID == call.ID {
			return sessionstate.ErrInvalidState
		}
	}
	order := slices.Index(pending.CallIDs, call.ID)
	if order < 0 {
		return sessionstate.ErrInvalidState
	}
	for index < len(candidate.messages) && candidate.messages[index].Role == llm.RoleTool && slices.Index(pending.CallIDs, candidate.messages[index].ToolCallID) < order {
		index++
	}
	candidate.messages = slices.Insert(candidate.messages, index, observation)
	if approval := candidate.pendingResourceApproval; approval != nil && approval.CallID == call.ID && approval.Answer != nil {
		candidate.applyInteraction(approval.InteractionID, *approval.Answer)
	}
	return r.commitToolState(ctx, input, state, candidate, "tool_result")
}

func (r *Runtime) acquireDispatchedToolCall(ctx context.Context, input RuntimeInput, state *RunState, call llm.ToolCall, publishStart bool) (ToolResult, bool) {
	row, ok := state.toolExecutions[call.ID]
	if !ok {
		return internalToolFailure(sessionstate.ErrInvalidState), false
	}
	hash, _, err := sessionstate.ToolArgumentsHash(call.Name, call.Args)
	if err != nil || hash != row.ArgumentsHash {
		return internalToolFailure(sessionstate.ErrIdempotencyConflict), false
	}
	if row.Terminal() {
		result, err := storedExecutionResult(row, state)
		if err != nil {
			return internalToolFailure(err), false
		}
		return result, false
	}
	if err := r.dispatchToolCall(ctx, input, state, call, publishStart); err != nil {
		return persistenceToolFailure(err), false
	}
	return ToolResult{}, true
}

func (r *Runtime) finishToolBatch(ctx context.Context, input RuntimeInput, state *RunState) error {
	if err := toolExecutionStopError(input, state); err != nil {
		return err
	}
	if state.pendingToolBatch == nil {
		return sessionstate.ErrInvalidState
	}
	for _, id := range state.pendingToolBatch.CallIDs {
		if !state.toolExecutions[id].Terminal() {
			return fmt.Errorf("%w: unfinished tool %s", sessionstate.ErrInvalidState, id)
		}
	}
	candidate := *state
	candidate.pendingToolBatch = nil
	candidate.pendingCommand = nil
	candidate.pendingResourceApproval = nil
	return r.commitToolState(ctx, input, state, candidate, "tool_batch_observed")
}
