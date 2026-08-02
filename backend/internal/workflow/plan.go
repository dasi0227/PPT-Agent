package workflow

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type PlanStepStatus string

const (
	PlanStepPending    PlanStepStatus = "pending"
	PlanStepInProgress PlanStepStatus = "in_progress"
	PlanStepCompleted  PlanStepStatus = "completed"
	PlanStepFailed     PlanStepStatus = "failed"
)

type PlanStep struct {
	ID     string         `json:"id"`
	Title  string         `json:"title"`
	Status PlanStepStatus `json:"status"`
}

type PlanRevision struct {
	Revision    int        `json:"revision"`
	Explanation string     `json:"explanation,omitempty"`
	Steps       []PlanStep `json:"steps"`
	UpdatedAt   int64      `json:"updated_at"`
}

type Plan struct {
	ID           string         `json:"plan_id"`
	Revision     int            `json:"revision"`
	Explanation  string         `json:"explanation,omitempty"`
	Steps        []PlanStep     `json:"steps"`
	CreatedAt    int64          `json:"created_at"`
	UpdatedAt    int64          `json:"updated_at"`
	CreatedByRun string         `json:"created_by_run"`
	History      []PlanRevision `json:"history"`
}

type PlanUpdate struct {
	Explanation string     `json:"explanation,omitempty"`
	Steps       []PlanStep `json:"steps"`
}

var ErrPlanInvalid = errors.New("PLAN_INVALID")

func ApplyPlanUpdate(current *Plan, update PlanUpdate, runID string, now time.Time) (Plan, bool, error) {
	if len(update.Steps) == 0 {
		return Plan{}, false, fmt.Errorf("%w: at least one step is required", ErrPlanInvalid)
	}
	seen := map[string]bool{}
	inProgress := 0
	for _, step := range update.Steps {
		if strings.TrimSpace(step.ID) == "" || strings.TrimSpace(step.Title) == "" || seen[step.ID] {
			return Plan{}, false, fmt.Errorf("%w: step ids and titles must be non-empty and unique", ErrPlanInvalid)
		}
		seen[step.ID] = true
		switch step.Status {
		case PlanStepPending, PlanStepInProgress, PlanStepCompleted, PlanStepFailed:
		default:
			return Plan{}, false, fmt.Errorf("%w: invalid status %q", ErrPlanInvalid, step.Status)
		}
		if step.Status == PlanStepInProgress {
			inProgress++
		}
	}
	if inProgress > 1 {
		return Plan{}, false, fmt.Errorf("%w: at most one step may be in_progress", ErrPlanInvalid)
	}
	created := current == nil || current.ID == ""
	if !created {
		nextByID := map[string]PlanStep{}
		for _, step := range update.Steps {
			nextByID[step.ID] = step
		}
		for _, old := range current.Steps {
			if old.Status != PlanStepCompleted {
				continue
			}
			next, ok := nextByID[old.ID]
			if !ok || next.Status != PlanStepCompleted {
				return Plan{}, false, fmt.Errorf("%w: completed step %q cannot be deleted or regressed", ErrPlanInvalid, old.ID)
			}
		}
	}
	ts := now.Unix()
	next := Plan{
		ID: "plan_" + uuid.NewString(), Revision: 1, Explanation: update.Explanation,
		Steps: clonePlanSteps(update.Steps), CreatedAt: ts, UpdatedAt: ts, CreatedByRun: runID,
		History: []PlanRevision{},
	}
	if !created {
		next = *current
		next.Revision++
		next.Explanation = update.Explanation
		next.Steps = clonePlanSteps(update.Steps)
		next.UpdatedAt = ts
		next.History = append([]PlanRevision{}, current.History...)
	}
	next.History = append(next.History, PlanRevision{
		Revision: next.Revision, Explanation: next.Explanation,
		Steps: clonePlanSteps(next.Steps), UpdatedAt: ts,
	})
	return next, created, nil
}

func clonePlanSteps(in []PlanStep) []PlanStep {
	return append([]PlanStep{}, in...)
}

func (p Plan) HasBlockingSteps() bool {
	for _, step := range p.Steps {
		if step.Status != PlanStepCompleted {
			return true
		}
	}
	return false
}
