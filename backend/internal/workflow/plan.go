package workflow

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

var ErrPlanInvalid = errors.New("PLAN_INVALID")

type Playbook interface {
	Key() string
	AllowedKinds() map[StepKind]bool
	AllowedCapabilities() map[Capability]bool
	Fallback(contextengine.ContextPack, Operation) WorkflowPlan
	Verifiers() []string
}

type PlanModel interface {
	Plan(contextengine.ContextPack, WorkflowPlan) (WorkflowPlan, error)
}

type Planner struct {
	Model PlanModel
}

func (p Planner) Create(pack contextengine.ContextPack, book Playbook, operation Operation) (WorkflowPlan, error) {
	fallback := book.Fallback(pack, operation)
	plan := fallback
	if p.Model != nil {
		if proposed, err := p.Model.Plan(pack, fallback); err == nil {
			plan = proposed
		}
	}
	if plan.ID == "" {
		plan.ID = "plan_" + uuid.NewString()
	}
	if plan.Version == 0 {
		plan.Version = 1
	}
	if plan.Budget.MaxPlanAttempts == 0 {
		plan.Budget = DefaultBudget()
	}
	if err := ValidatePlan(plan, pack.WorkSpec, book); err != nil {
		if err := ValidatePlan(fallback, pack.WorkSpec, book); err != nil {
			return WorkflowPlan{}, err
		}
		return fallback, nil
	}
	return plan, nil
}

func ValidatePlan(plan WorkflowPlan, spec model.WorkSpec, book Playbook) error {
	if plan.Target != spec.Target {
		return fmt.Errorf("%w: plan target differs from work spec", ErrPlanInvalid)
	}
	if strings.TrimSpace(plan.Goal) == "" || len(plan.Steps) == 0 {
		return fmt.Errorf("%w: goal and steps are required", ErrPlanInvalid)
	}
	seen := map[string]WorkflowStep{}
	allowedKinds := book.AllowedKinds()
	allowedCaps := book.AllowedCapabilities()
	for _, step := range plan.Steps {
		if step.ID == "" || seen[step.ID].ID != "" {
			return fmt.Errorf("%w: duplicate or empty step id %q", ErrPlanInvalid, step.ID)
		}
		if !allowedKinds[step.Kind] {
			return fmt.Errorf("%w: step kind %s is not in playbook", ErrPlanInvalid, step.Kind)
		}
		for _, cap := range step.Capabilities {
			if !allowedCaps[cap] {
				return fmt.Errorf("%w: capability %s is not in playbook", ErrPlanInvalid, cap)
			}
		}
		for _, target := range step.Targets {
			if spec.Target.Level == model.TargetSlide && target.ID != "" && target.ID != spec.Target.SlideID && target.Kind != ArtifactDesign {
				return fmt.Errorf("%w: slide plan targets unrelated artifact %s", ErrPlanInvalid, target.Key())
			}
		}
		seen[step.ID] = step
	}
	for _, step := range plan.Steps {
		for _, dep := range step.DependsOn {
			if _, ok := seen[dep]; !ok {
				return fmt.Errorf("%w: unknown dependency %s", ErrPlanInvalid, dep)
			}
		}
	}
	visiting, visited := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("%w: dependency cycle at %s", ErrPlanInvalid, id)
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		for _, dep := range seen[id].DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[id], visited[id] = false, true
		return nil
	}
	for id := range seen {
		if err := visit(id); err != nil {
			return err
		}
	}
	if plan.Operation == OperationConsult {
		for _, step := range plan.Steps {
			for _, cap := range step.Capabilities {
				if IsWriteCapability(cap) {
					return fmt.Errorf("%w: consult step %s requests write capability", ErrPlanInvalid, step.ID)
				}
			}
		}
	}
	return nil
}

func IsWriteCapability(cap Capability) bool {
	switch cap {
	case CapabilityWriteBlueprint, CapabilityWriteDesignSpec,
		CapabilityWritePresentation, CapabilityMountAssets:
		return true
	default:
		return false
	}
}
