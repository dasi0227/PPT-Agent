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

func (r *Runtime) commitModelPhase(ctx context.Context, input RuntimeInput, state *RunState, pending sessionstate.PendingModelCall, snapshot *sessionstate.ModelRequestSnapshot, response json.RawMessage) error {
	candidate := *state
	candidate.pendingModelCall = &pending
	if err := r.commitCheckpoint(ctx, input, &candidate, checkpointBoundary("model_"+pending.Stage), func(ctx context.Context, cp RuntimeCheckpoint) error {
		cp.NewModelRequest, cp.ModelResponseJSON = snapshot, response
		return input.Checkpoint.SaveCheckpoint(ctx, cp)
	}); err != nil {
		state.persistenceErr = candidate.persistenceErr
		return err
	}
	*state = candidate
	if snapshot != nil {
		saved := *snapshot
		state.modelRequestSnapshot = &saved
	}
	if len(response) != 0 {
		if state.modelRequestSnapshot == nil {
			return sessionstate.ErrInvalidState
		}
		saved := *state.modelRequestSnapshot
		saved.ResponseJSON = append(json.RawMessage(nil), response...)
		state.modelRequestSnapshot = &saved
	}
	return nil
}

// Commit the frozen normalized request before calling a provider. A complete
// response is saved before Runtime can interpret it or dispatch any tool.
func (r *Runtime) generatePersistedModelCall(ctx context.Context, input RuntimeInput, state *RunState, provider llm.Provider, live llm.GenerateRequest) (llm.GenerateResponse, error) {
	if input.Checkpoint == nil {
		return provider.Generate(ctx, live)
	}
	if state.contextSession == nil {
		return llm.GenerateResponse{}, sessionstate.ErrInvalidState
	}
	current := state.contextSession.Snapshot
	replacePending := state.pendingModelCall == nil
	if pending := state.pendingModelCall; pending != nil {
		saved := state.modelRequestSnapshot
		if saved == nil || saved.ID != pending.ID || saved.ContextRevision != current.Revision {
			return llm.GenerateResponse{}, sessionstate.ErrInvalidState
		}
		var err error
		live, err = sessionstate.DecodeModelRequest(saved.RequestJSON, live)
		if err != nil {
			return llm.GenerateResponse{}, err
		}
		if pending.Stage == "responded" {
			response, err := sessionstate.DecodeModelResponse(saved.ResponseJSON)
			if saved.SceneRevision != current.SceneRevision || saved.Provider != provider.Name() || saved.Model != provider.Model() {
				response.Continuation = nil
			}
			return response, err
		}
		// A scene/model change invalidates opaque continuation. Save the revised
		// request under a new identity instead of mutating an immutable snapshot.
		if saved.SceneRevision != current.SceneRevision || saved.Provider != provider.Name() || saved.Model != provider.Model() {
			live.Continuation = nil
			replacePending = true
		}
	}
	if replacePending {
		raw, err := sessionstate.EncodeModelRequest(live)
		if err != nil {
			return llm.GenerateResponse{}, err
		}
		pending := sessionstate.PendingModelCall{ID: model.MustShortID("model"), ContextRevision: current.Revision, Provider: provider.Name(), Model: provider.Model(), Stage: "prepared"}
		snapshot := &sessionstate.ModelRequestSnapshot{ID: pending.ID, SceneRevision: current.SceneRevision, ContextRevision: current.Revision, Provider: pending.Provider, Model: pending.Model, RequestJSON: raw}
		if err := r.commitModelPhase(ctx, input, state, pending, snapshot, nil); err != nil {
			return llm.GenerateResponse{}, err
		}
		// Use the saved value, including native metadata, as the dispatched body.
		live, err = sessionstate.DecodeModelRequest(raw, live)
		if err != nil {
			return llm.GenerateResponse{}, err
		}
	}
	pending := *state.pendingModelCall
	pending.Stage = "dispatched"
	if err := r.commitModelPhase(ctx, input, state, pending, nil, nil); err != nil {
		return llm.GenerateResponse{}, err
	}
	response, err := provider.Generate(ctx, live)
	if errors.Is(err, llm.ErrFallbackActivated) {
		return llm.GenerateResponse{}, err
	}
	if err != nil {
		pending.Stage = "outcome_unknown"
		if saveErr := r.commitModelPhase(context.WithoutCancel(ctx), input, state, pending, nil, nil); saveErr != nil {
			return llm.GenerateResponse{}, saveErr
		}
		return llm.GenerateResponse{}, err
	}
	for i := range response.ToolCalls {
		if response.ToolCalls[i].ID == "" {
			response.ToolCalls[i].ID = fmt.Sprintf("%s-%d", pending.ID, i+1)
		}
	}
	raw, err := sessionstate.EncodeModelResponse(response)
	if err != nil {
		return llm.GenerateResponse{}, err
	}
	pending.Stage = "responded"
	if err := r.commitModelPhase(context.WithoutCancel(ctx), input, state, pending, nil, raw); err != nil {
		return llm.GenerateResponse{}, err
	}
	return response, nil
}

func asAgentResponse(response llm.GenerateResponse) AgentResponse {
	return AgentResponse{Thinking: response.Thinking, Replay: response.Replay, ToolCalls: response.ToolCalls, Text: response.Text(), Continuation: response.Continuation}
}
