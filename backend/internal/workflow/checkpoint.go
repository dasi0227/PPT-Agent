package workflow

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

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
	checkpointBeforeReview            checkpointBoundary = "before_review"
	checkpointAfterReview             checkpointBoundary = "after_review"
	checkpointAfterWrite              checkpointBoundary = "after_write"
	checkpointAfterRender             checkpointBoundary = "after_render"
	checkpointGateRejected            checkpointBoundary = "completion_gate_rejected"
	checkpointAfterCommit             checkpointBoundary = "after_commit"
	checkpointTerminal                checkpointBoundary = "terminal"
	checkpointPeriodic                checkpointBoundary = "periodic"
)

func (r *Runtime) saveCheckpoint(ctx context.Context, input RuntimeInput, state *RunState, boundary checkpointBoundary) error {
	if input.Checkpoint == nil {
		return nil
	}
	return r.commitCheckpoint(ctx, input, state, boundary, input.Checkpoint.SaveCheckpoint)
}

// Special mode/scope transitions use the same context preparation and
// commit-before-publish rule as ordinary runtime checkpoints.
func (r *Runtime) commitCheckpoint(ctx context.Context, input RuntimeInput, state *RunState, boundary checkpointBoundary, write func(context.Context, RuntimeCheckpoint) error) error {
	if err := toolExecutionStopError(input, state); err != nil {
		return err
	}
	if state == nil {
		return nil
	}
	cp := r.checkpointForBoundary(state, boundary)
	if input.publicUpdates != nil {
		cp.PublicUpdates = input.publicUpdates.snapshot()
	}
	var prepared contextengine.PreparedContext
	if state.contextSession != nil {
		var err error
		prepared, err = state.contextSession.Prepare(state.messages, state.runID, "")
		if err != nil {
			return classifyPersistenceError(err, "context_prepare")
		}
		cp.ContextUpdate = &prepared.Update
		cp.CommitReceipt = &sessionstate.Receipt{}
		if cp.PendingToolBatch != nil {
			pending := *cp.PendingToolBatch
			if len(pending.CallIDs) == 0 {
				return sessionstate.ErrInvalidState
			}
			for _, message := range prepared.Messages {
				if message.Role == llm.RoleAssistant && len(message.ToolCalls) > 0 && message.ToolCalls[0].ID == pending.CallIDs[0] {
					pending.AssistantMessageID = message.PersistentID
				}
			}
			cp.PendingToolBatch = &pending
		}
		for i := range cp.ToolChanges {
			if !cp.ToolChanges[i].Terminal() {
				continue
			}
			for _, message := range prepared.Messages {
				if message.Role == llm.RoleTool && message.ToolCallID == cp.ToolChanges[i].CallID {
					cp.ToolChanges[i].ResultMessageID = message.PersistentID
				}
			}
		}
		for i := range cp.PublicUpdates {
			fact := &cp.PublicUpdates[i]
			if fact.Type != model.EventMessageFinal {
				continue
			}
			var final model.MessageFinalPayload
			if err := json.Unmarshal(fact.Payload, &final); err != nil {
				return err
			}
			if len(prepared.Messages) == 0 {
				return sessionstate.ErrInvalidState
			}
			message := prepared.Messages[len(prepared.Messages)-1]
			if message.Role != llm.RoleAssistant || message.Text() != final.Text {
				return sessionstate.ErrInvalidState
			}
			final.MessageID = message.PersistentID
			fact.Payload, err = json.Marshal(final)
			if err != nil {
				return err
			}
		}
	}
	if err := write(ctx, cp); err != nil {
		return stopForPersistenceFailure(ctx, input, state, err, "checkpoint_save")
	}
	if input.publicUpdates != nil {
		input.publicUpdates.committed(len(cp.PublicUpdates))
	}
	if state.contextSession != nil {
		if err := state.contextSession.Publish(prepared, *cp.CommitReceipt); err != nil {
			return stopForPersistenceFailure(ctx, input, state, err, "context_publish")
		}
		state.messages = prepared.Messages
	}
	state.interactionApplications = nil
	state.pendingToolBatch = cp.PendingToolBatch
	if state.toolExecutions == nil {
		state.toolExecutions = map[string]sessionstate.ToolExecution{}
	}
	for _, execution := range cp.ToolChanges {
		execution.EntityRevision++
		state.toolExecutions[execution.CallID] = execution
	}
	state.toolChanges = nil
	recordTrace(input.Trace, state.runID, "checkpoint.saved", map[string]any{
		"loop_id": state.loopID, "phase": state.phase, "boundary": boundary,
		"turns": state.turns, "tool_calls": state.toolCalls,
	})
	return nil
}

func (r *Runtime) checkpointForBoundary(state *RunState, boundary checkpointBoundary) RuntimeCheckpoint {
	cp := state.checkpoint(r.clockNow())
	cp.ToolChanges = append([]sessionstate.ToolExecution(nil), state.toolChanges...)
	cp.InteractionApplications = append([]model.InteractionApplication(nil), state.interactionApplications...)
	if state.contextSession != nil {
		cp.ContextRevision = state.contextSession.Snapshot.Revision
		cp.SceneRevision = state.contextSession.Snapshot.SceneRevision
	}
	if cognitive, ok := r.Agent.(CognitiveAgent); ok {
		if route, ok := cognitive.Provider.(*llm.RoutedProvider); ok {
			snapshot := route.State()
			cp.ModelRoute = &snapshot
		}
	}
	cp.Boundary = string(boundary)
	if boundary == checkpointTerminal {
		cp.ActiveClockRunning = false
	}
	cp.ActiveSkills, cp.ActiveComponents = state.activeSkills.Snapshot()
	cp.DOMSelections = append([]model.DOMSelection{}, state.pack.Command.DOMSelections...)
	if cp.CreatedAt == 0 {
		cp.CreatedAt = time.Now().UnixMilli()
	}
	return cp
}

// Only typed answers accepted by the prompter reach this boundary. The durable
// store still validates them against the request and the accepted answer.
func (state *RunState) applyInteraction(id string, answer any) {
	raw, _ := json.Marshal(answer)
	next := append([]model.InteractionApplication(nil), state.interactionApplications...)
	for i := range next {
		if next[i].ID == id {
			next[i].Answer = raw
			state.interactionApplications = next
			return
		}
	}
	state.interactionApplications = append(next, model.InteractionApplication{ID: id, Answer: raw})
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
		return r.saveCheckpoint(ctx, input, state, checkpointPeriodic)
	}
	return nil
}

func hashCheckpointValue(value any) string {
	raw, _ := json.Marshal(value)
	return hashBytes(raw)
}

// Keep the last committed position when a write fails. A second terminal write
// could otherwise consume answers or replace the checkpoint that must recover.
func stopForPersistenceFailure(ctx context.Context, input RuntimeInput, state *RunState, err error, operation string) error {
	failure := classifyPersistenceError(err, operation)
	if input.Checkpoint == nil {
		return failure
	}
	if state.persistenceErr == nil {
		state.persistenceErr = failure
	}
	if pauser, ok := input.Checkpoint.(interface{ PersistenceFailed(context.Context, string) }); ok {
		pauser.PersistenceFailed(ctx, "checkpoint_write_failed")
	}
	return failure
}
