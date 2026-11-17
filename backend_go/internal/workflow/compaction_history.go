package workflow

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextcompact"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

var ErrCompactionPending = errors.New("存在待回答或待恢复的交互，请先继续当前任务；本次压缩未修改历史")

// PrepareCompactionHistory only reads already paired context. Pending tools
// must be recovered by Runtime, never by the context inspector or compactor.
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
		if cp.PendingModelCall != nil {
			return nil, ErrCompactionPending
		}
		state.pendingReplay = cp.PendingReplay
		if cp.PendingQuestion != nil && !observed(cp.PendingQuestion.Call.ID) ||
			cp.PendingCommand != nil && !observed(cp.PendingCommand.CallID) ||
			cp.PendingResourceApproval != nil && !observed(cp.PendingResourceApproval.CallID) ||
			cp.PendingScopeExpansion != nil && !observed(cp.PendingScopeExpansion.Request.CallID) ||
			cp.PendingPlanCall != nil && !observed(cp.PendingPlanCall.Call.ID) {
			return nil, ErrCompactionPending
		}
		state.pendingPlanCall = cp.PendingPlanCall
		if review := cp.PendingReview; review != nil && !observed(review.Call.ID) {
			if review.Result == nil {
				return nil, ErrCompactionPending
			}
			runtime.appendControlObservation(state, review.Call, review.AssistantText, *review.Result)
		}
		if cp.PendingToolBatch != nil {
			for _, id := range cp.PendingToolBatch.CallIDs {
				if !observed(id) {
					return nil, ErrCompactionPending
				}
			}
		}
	}
	if err := validateCurrentHistory(state); err != nil {
		return nil, err
	}
	return state.messages, contextcompact.ValidateHistory(state.messages)
}

// Tool outcomes and native observations are committed together. An unpaired
// history is corruption or a pending batch, never input for synthesized results.
func validateCurrentHistory(state *RunState) error {
	return contextcompact.ValidateHistory(state.messages)
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
