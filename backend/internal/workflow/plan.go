package workflow

import (
	"errors"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type Plan = model.PublicPlan
type PlanStep = model.PublicPlanStep

const (
	PlanStepPending    = "pending"
	PlanStepProcessing = "processing"
	PlanStepCompleted  = "completed"
)

// The argument schema validates each row. The processing limit applies to this snapshot only.
func executionPlan(steps []PlanStep, display model.PublicTextContext) (*Plan, error) {
	if steps == nil {
		return nil, nil
	}
	processing := false
	for i, step := range steps {
		if step.Status == PlanStepProcessing {
			if processing {
				return nil, errors.New("at most one step may be processing")
			}
			processing = true
		}
		steps[i].Title = model.PublicText(strings.TrimSpace(step.Title), display)
		if steps[i].Title == "" {
			return nil, errors.New("step title is required")
		}
	}
	return &Plan{Steps: steps}, nil
}

func proposalChecklist(proposal model.PlanProposal) *Plan {
	steps := make([]PlanStep, len(proposal.Steps))
	for i, title := range proposal.Steps {
		steps[i] = PlanStep{Title: title, Status: PlanStepPending}
	}
	return &Plan{Steps: steps}
}

func publishChecklist(input RuntimeInput, runID string, plan *Plan) error {
	if input.Emitter != nil {
		input.Emitter.Emit(model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: publicBase(runID), Plan: plan})
	}
	return executionStopError(input)
}
