package workflow

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type CheckpointMessage struct {
	Role       string `json:"role"`
	Summary    string `json:"summary,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	ToolName   string `json:"tool_name,omitempty"`
	Hash       string `json:"hash"`
}

type CheckpointToolResult struct {
	CallID          string           `json:"call_id"`
	Tool            string           `json:"tool"`
	OK              bool             `json:"ok"`
	Code            string           `json:"code,omitempty"`
	Summary         string           `json:"summary"`
	ChangedTargets  []ChangedTarget  `json:"changed_targets,omitempty"`
	EvidenceIDs     []string         `json:"evidence_ids,omitempty"`
	LoadedResources []LoadedResource `json:"loaded_resources,omitempty"`
}

type ProviderContinuationSnapshot struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Hash     string `json:"hash"`
}

type CheckpointStore interface {
	SaveCheckpoint(context.Context, RuntimeCheckpoint) error
	LatestCheckpoint(context.Context, string) (RuntimeCheckpoint, error)
	ListCheckpoints(context.Context, string, int) ([]RuntimeCheckpoint, error)
}

type checkpointBoundary string

const (
	checkpointRuntimeInitialized      checkpointBoundary = "runtime_initialized"
	checkpointPlanUpdated             checkpointBoundary = "plan_updated"
	checkpointBeforeAskUser           checkpointBoundary = "before_ask_user"
	checkpointAfterUserAnswer         checkpointBoundary = "after_user_answer"
	checkpointBeforeCommandPermission checkpointBoundary = "before_command_permission"
	checkpointAfterCommandPermission  checkpointBoundary = "after_command_permission"
	checkpointAfterReview             checkpointBoundary = "after_review"
	checkpointAfterWrite              checkpointBoundary = "after_write"
	checkpointAfterRender             checkpointBoundary = "after_render"
	checkpointGateRejected            checkpointBoundary = "completion_gate_rejected"
	checkpointBeforeCommit            checkpointBoundary = "before_commit"
	checkpointAfterCommit             checkpointBoundary = "after_commit"
	checkpointTerminal                checkpointBoundary = "terminal"
	checkpointPeriodic                checkpointBoundary = "periodic"
)

func (r *Runtime) saveCheckpoint(ctx context.Context, input RuntimeInput, state *RunState, boundary checkpointBoundary, questionID string) error {
	if input.Checkpoint == nil || state == nil {
		return nil
	}
	cp := r.checkpointForBoundary(state, boundary, questionID)
	if err := input.Checkpoint.SaveCheckpoint(ctx, cp); err != nil {
		return err
	}
	recordTrace(input.Trace, state.runID, "checkpoint.saved", map[string]any{
		"loop_id": state.loopID, "phase": state.phase, "boundary": boundary,
		"turns": state.turns, "tool_calls": state.toolCalls,
	})
	return nil
}

func (r *Runtime) checkpointForBoundary(state *RunState, boundary checkpointBoundary, questionID string) RuntimeCheckpoint {
	cp := state.checkpoint(questionID, r.clockNow())
	cp.Boundary = string(boundary)
	cp.ContextBriefing = state.contextBriefing
	cp.ContextIndexRef = state.contextIndexRef
	cp.LatestToolResults = append([]CheckpointToolResult{}, state.latestToolResults...)
	cp.ActiveSkills = append([]model.RunSkill{}, state.activeSkills.Skills...)
	cp.MessageSummary = summarizeCheckpointMessages(state.messages)
	cp.ProviderContinuation = safeContinuationSnapshot(state.continuation)
	if cp.CreatedAt == 0 {
		cp.CreatedAt = time.Now().UnixNano()
	}
	return cp
}

func (r *Runtime) maybePeriodicCheckpoint(ctx context.Context, input RuntimeInput, state *RunState) error {
	if state == nil || input.Checkpoint == nil {
		return nil
	}
	now := time.Now()
	if state.lastCheckpointTurn == 0 ||
		state.turns-state.lastCheckpointTurn >= 2 ||
		state.toolCalls-state.lastCheckpointToolCalls >= 5 ||
		now.Sub(state.lastCheckpointAt) >= 30*time.Second {
		state.lastCheckpointTurn = state.turns
		state.lastCheckpointToolCalls = state.toolCalls
		state.lastCheckpointAt = now
		return r.saveCheckpoint(ctx, input, state, checkpointPeriodic, "")
	}
	return nil
}

func summarizeCheckpointMessages(messages []llm.Message) []CheckpointMessage {
	out := make([]CheckpointMessage, 0, len(messages))
	for _, message := range messages {
		summary := strings.TrimSpace(message.Text())
		if len([]rune(summary)) > 400 {
			summary = string([]rune(summary)[:400])
		}
		item := CheckpointMessage{
			Role:       string(message.Role),
			Summary:    summary,
			ToolCallID: message.ToolCallID,
			Hash:       hashCheckpointValue(message),
		}
		if len(message.ToolCalls) > 0 {
			item.ToolName = message.ToolCalls[0].Name
		}
		out = append(out, item)
	}
	return out
}

func safeContinuationSnapshot(value *llm.ProviderContinuation) *ProviderContinuationSnapshot {
	if value == nil {
		return nil
	}
	return &ProviderContinuationSnapshot{
		Provider: value.Provider,
		Model:    value.Model,
		Hash:     hashBytes(value.Opaque),
	}
}

func checkpointToolResult(call llm.ToolCall, result ToolResult) CheckpointToolResult {
	ids := make([]string, 0, len(result.Evidence))
	for _, evidence := range result.Evidence {
		if evidence.ID != "" {
			ids = append(ids, evidence.ID)
		}
	}
	return CheckpointToolResult{
		CallID: call.ID, Tool: call.Name, OK: result.OK, Code: result.Code,
		Summary: result.Summary, ChangedTargets: append([]ChangedTarget{}, result.ChangedTargets...),
		EvidenceIDs: ids, LoadedResources: append([]LoadedResource{}, result.LoadedResources...),
	}
}

func hashCheckpointValue(value any) string {
	raw, _ := json.Marshal(value)
	return hashBytes(raw)
}
