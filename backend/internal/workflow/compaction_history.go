package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextcompact"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

var ErrCompactionPending = errors.New("存在待回答或待恢复的交互，请先继续当前任务；本次压缩未修改历史")

// PrepareCompactionHistory reuses Runtime's receipt/batch recovery without
// executing tools, asking users, publishing events or persisting a checkpoint.
func PrepareCompactionHistory(ctx context.Context, input RuntimeInput, messages []llm.Message) ([]llm.Message, error) {
	state := &RunState{runID: input.RunID, projectDir: input.ProjectDir, pack: input.Context,
		messages: append([]llm.Message(nil), messages...), scope: input.Context.Command.Scope,
		mode: input.Context.Command.Mode, ledger: NewEvidenceLedger(), activeSkills: &ActiveSkillSet{}}
	runtime := &Runtime{}
	observed := func(id string) bool {
		for _, message := range state.messages {
			if message.Role == llm.RoleTool && message.ToolCallID == id {
				return true
			}
		}
		return false
	}
	if cp := input.ResumeCheckpoint; cp != nil {
		state.pendingReplay = cp.PendingReplay
		if cp.PendingQuestion != nil && !observed(cp.PendingQuestion.Call.ID) ||
			cp.PendingCommand != nil && !observed(cp.PendingCommand.CallID) ||
			cp.PendingResourceApproval != nil && !observed(cp.PendingResourceApproval.CallID) ||
			cp.PendingScopeExpansion != nil && !observed(cp.PendingScopeExpansion.Request.CallID) ||
			cp.PendingPlanCall != nil && cp.PendingPlanPublication == nil && !observed(cp.PendingPlanCall.Call.ID) {
			return nil, ErrCompactionPending
		}
		state.pendingPlanCall = cp.PendingPlanCall
		if cp.PendingPlanPublication != nil {
			runtime.completePlanTool(state, *cp.PendingPlanPublication)
		}
		if review := cp.PendingReview; review != nil && !observed(review.Call.ID) {
			if review.Result == nil {
				return nil, ErrCompactionPending
			}
			runtime.appendControlObservation(state, review.Call, review.AssistantText, *review.Result)
		}
		state.pendingToolBatch = cp.PendingToolBatch
		input.Checkpoint, input.Transcript, input.Emitter = nil, nil, nil
		if err := runtime.resumeToolBatch(ctx, input, state); err != nil {
			return nil, err
		}
	}
	if err := restoreHistoryReceipts(ctx, input, state); err != nil {
		return nil, err
	}
	return state.messages, contextcompact.ValidateHistory(state.messages)
}

// Unknown history is never repaired by assuming an operation did not execute.
// Only a completed receipt with a known originating Run can fill a missing result.
func restoreHistoryReceipts(ctx context.Context, input RuntimeInput, state *RunState) error {
	if contextcompact.ValidateHistory(state.messages) == nil {
		return nil
	}
	out := []llm.Message{}
	owner := ""
	for i := 0; i < len(state.messages); {
		message := state.messages[i]
		if m := message.Metadata; message.Role == llm.RoleUser && m != nil && m.Origin == "user" && m.RunID != "" {
			owner = m.RunID
		}
		if message.Role == llm.RoleTool {
			return contextcompact.ErrUnclosedHistory
		}
		out = append(out, message)
		i++
		if len(message.ToolCalls) == 0 {
			continue
		}
		seen := map[string]bool{}
		for i < len(state.messages) && state.messages[i].Role == llm.RoleTool {
			result := state.messages[i]
			seen[result.ToolCallID] = true
			out = append(out, result)
			i++
		}
		for _, call := range message.ToolCalls {
			if seen[call.ID] {
				continue
			}
			if owner == "" || input.Idempotency == nil {
				return contextcompact.ErrUnclosedHistory
			}
			receipt, err := input.Idempotency.GetIdempotency(ctx, "tool_call", owner, call.ID)
			if err != nil {
				if errors.Is(err, model.ErrRunNotFound) {
					return contextcompact.ErrUnclosedHistory
				}
				return err
			}
			if receipt.Status != "completed" {
				return contextcompact.ErrUnclosedHistory
			}
			var stored persistedToolResult
			if err := json.Unmarshal([]byte(receipt.ResultJSON), &stored); err != nil {
				return fmt.Errorf("invalid persisted tool result: %w", err)
			}
			if !validToolReceipt(receipt.ResultJSON) {
				return errors.New("completed tool receipt is incomplete")
			}
			generated := appendBatchObservations(nil, []llm.ToolCall{call}, "", []ToolResult{stored.restoreResult()})
			out = append(out, generated[1])
			state.restoreResourceActivation(stored)
		}
	}
	if err := contextcompact.ValidateHistory(out); err != nil {
		return err
	}
	state.messages = out
	state.rememberResourceVersions()
	return nil
}

func validToolReceipt(raw string) bool {
	var value struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if json.Unmarshal([]byte(raw), &value) != nil {
		return false
	}
	return value.Result["ok"] != nil && value.Result["applied"] != nil && value.Result["summary"] != nil
}
