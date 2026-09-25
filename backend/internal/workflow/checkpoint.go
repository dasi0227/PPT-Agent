package workflow

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

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
	checkpointBeforeScopeExpansion    checkpointBoundary = "before_scope_expansion"
	checkpointAfterScopeExpansion     checkpointBoundary = "after_scope_expansion"
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
	if cognitive, ok := r.Agent.(CognitiveAgent); ok {
		if route, ok := cognitive.Provider.(*llm.RoutedProvider); ok {
			snapshot := route.State()
			cp.ModelRoute = &snapshot
		}
	}
	cp.Boundary = string(boundary)
	cp.ContextIndexRef = state.contextIndexRef
	cp.ActiveSkills, cp.ActiveComponents = state.activeSkills.Snapshot()
	cp.DOMSelections = append([]model.DOMSelection{}, state.pack.Command.DOMSelections...)
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

func hashCheckpointValue(value any) string {
	raw, _ := json.Marshal(value)
	return hashBytes(raw)
}
