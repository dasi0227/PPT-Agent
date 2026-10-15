package workflow

import (
	"context"
	"encoding/json"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Durable batch intent. Recovery consumes receipts without re-executing calls.
type PendingToolBatch struct {
	Calls         []llm.ToolCall `json:"calls"`
	AssistantText string         `json:"assistant_text"`
}

func (r *Runtime) resumeToolBatch(ctx context.Context, input RuntimeInput, state *RunState) error {
	pending := state.pendingToolBatch
	if pending == nil || state.pendingCommand != nil {
		return nil
	}
	observed := map[string]bool{}
	for _, m := range state.messages {
		if m.Role == llm.RoleTool {
			observed[m.ToolCallID] = true
		}
	}
	allObserved := true
	for _, c := range pending.Calls {
		if !observed[c.ID] {
			allObserved = false
		}
	}
	if allObserved {
		state.pendingToolBatch = nil
		return nil
	}
	results := make([]ToolResult, len(pending.Calls))
	for i, call := range pending.Calls {
		results[i] = failedToolResult(CodeCanceled, "This call was interrupted and has not been replayed. Saved changes from other calls remain intact.")
		if input.Idempotency == nil {
			continue
		}
		receipt, err := input.Idempotency.GetIdempotency(ctx, "tool_call", state.runID, call.ID)
		if err != nil || receipt.Status != "completed" {
			continue
		}
		var stored persistedToolResult
		if json.Unmarshal([]byte(receipt.ResultJSON), &stored) != nil {
			continue
		}
		results[i] = stored.restoreResult()
		state.restoreResourceActivation(stored)
	}
	refreshRuntimePack(input.ProjectDir, state, []ChangedTarget{{Type: "deck", Part: "manifest"}, {Type: "deck", Part: "outline"}, {Type: "deck", Part: "design"}})
	r.invalidatePreviews(input, state)
	for i := range results {
		r.refreshPreviewResult(ctx, input, state, &results[i])
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// A crash can happen after commit and before the tool terminal is published.
	// Republish the saved projection by call ID; never diff the current files.
	if input.Emitter != nil {
		projector := ToolPublicProjector{ProjectDir: input.ProjectDir, TextContext: state.publicTextContext()}
		for i, call := range pending.Calls {
			if results[i].OK && results[i].Applied && results[i].OperationTargets != nil {
				if event, ok := projector.Completed(state.runID, call.ID, call.Name, call.Args, results[i]); ok {
					input.Emitter.Emit(model.EventToolCompleted, event)
				}
			}
		}
	}
	// Rebuild the interrupted batch as a single observation group. Persisted
	// receipts, never model arguments, determine which calls succeeded.
	filtered := state.messages[:0]
	callIDs := map[string]bool{}
	for _, c := range pending.Calls {
		callIDs[c.ID] = true
	}
	for _, m := range state.messages {
		drop := m.Role == llm.RoleTool && callIDs[m.ToolCallID]
		if m.Role == llm.RoleAssistant {
			for _, c := range m.ToolCalls {
				if callIDs[c.ID] {
					drop = true
				}
			}
		}
		if !drop {
			filtered = append(filtered, m)
		}
	}
	state.messages = appendBatchObservations(filtered, pending.Calls, pending.AssistantText, results)
	state.rememberResourceVersions()
	state.pendingToolBatch = nil
	return r.saveCheckpoint(ctx, input, state, checkpointBoundary("tool_batch_recovered"))
}

func toolBatchMayWrite(registry *ToolRegistry, calls []llm.ToolCall) bool {
	for _, call := range calls {
		desc, ok := registry.Descriptor(call.Name)
		if ok && (!desc.ReadOnly || desc.Dynamic) {
			return true
		}
	}
	return false
}
