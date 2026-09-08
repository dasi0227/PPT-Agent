package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type PlanStatus string

const (
	PlanAwaitingApproval PlanStatus = "awaiting_approval"
	PlanActive           PlanStatus = "active"
	PlanCompleted        PlanStatus = "completed"
	PlanCanceled         PlanStatus = "canceled"
)

type PlanStepStatus string

const (
	PlanStepPending    PlanStepStatus = "pending"
	PlanStepInProgress PlanStepStatus = "in_progress"
	PlanStepCompleted  PlanStepStatus = "completed"
	PlanStepFailed     PlanStepStatus = "failed"
)

type PlanStep struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	Status         PlanStepStatus `json:"status"`
	TargetSlideIDs []string       `json:"target_slide_ids,omitempty"`
}

// Plan is the only persisted plan snapshot. Events and checkpoints are the audit
// trail; deliberately do not add an in-object history copy.
type Plan struct {
	ID                  string     `json:"plan_id"`
	Revision            int        `json:"revision"`
	ApprovedRevision    int        `json:"approved_revision,omitempty"`
	ApprovedContentHash string     `json:"approved_content_hash,omitempty"`
	Status              PlanStatus `json:"status"`
	Title               string     `json:"title"`
	Content             string     `json:"content"`
	Steps               []PlanStep `json:"steps"`
	CreatedAt           int64      `json:"created_at"`
	UpdatedAt           int64      `json:"updated_at"`
	CreatedByRun        string     `json:"created_by_run"`
}

// PlanUpdate is used in planning mode. IDs, revisions and statuses are Runtime-owned.
type PlanUpdate struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Steps   []struct {
		Title          string   `json:"title"`
		TargetSlideIDs []string `json:"target_slide_ids,omitempty"`
	} `json:"steps"`
}

type PlanProgressUpdate struct {
	Updates []struct {
		StepID string         `json:"step_id"`
		Status PlanStepStatus `json:"status"`
	} `json:"updates"`
}

var ErrPlanInvalid = errors.New("PLAN_INVALID")

func newPlanSteps(source []struct {
	Title          string   `json:"title"`
	TargetSlideIDs []string `json:"target_slide_ids,omitempty"`
}) ([]PlanStep, error) {
	if len(source) == 0 {
		return nil, fmt.Errorf("%w: at least one step is required", ErrPlanInvalid)
	}
	steps := make([]PlanStep, 0, len(source))
	for _, item := range source {
		if strings.TrimSpace(item.Title) == "" {
			return nil, fmt.Errorf("%w: step title is required", ErrPlanInvalid)
		}
		seen := map[string]bool{}
		targets := make([]string, 0, len(item.TargetSlideIDs))
		for _, raw := range item.TargetSlideIDs {
			id := strings.TrimSpace(raw)
			if id == "" || seen[id] {
				return nil, fmt.Errorf("%w: target_slide_ids must be non-empty and unique", ErrPlanInvalid)
			}
			seen[id] = true
			targets = append(targets, id)
		}
		steps = append(steps, PlanStep{ID: "step_" + uuid.NewString(), Title: strings.TrimSpace(item.Title), Status: PlanStepPending, TargetSlideIDs: targets})
	}
	return steps, nil
}

func ApplyPlanUpdate(current *Plan, update PlanUpdate, runID string, now time.Time) (Plan, bool, error) {
	if strings.TrimSpace(update.Title) == "" || strings.TrimSpace(update.Content) == "" {
		return Plan{}, false, fmt.Errorf("%w: title and content are required", ErrPlanInvalid)
	}
	if current != nil && current.Status == PlanActive {
		return Plan{}, false, fmt.Errorf("%w: approved plan structure is locked", ErrPlanInvalid)
	}
	steps, err := newPlanSteps(update.Steps)
	if err != nil {
		return Plan{}, false, err
	}
	ts := now.Unix()
	if current == nil || current.ID == "" {
		return Plan{ID: "plan_" + uuid.NewString(), Revision: 1, Status: PlanAwaitingApproval, Title: strings.TrimSpace(update.Title), Content: update.Content, Steps: steps, CreatedAt: ts, UpdatedAt: ts, CreatedByRun: runID}, true, nil
	}
	next := *current
	next.Revision++
	next.Status = PlanAwaitingApproval
	next.Title, next.Content, next.Steps, next.UpdatedAt = strings.TrimSpace(update.Title), update.Content, steps, ts
	next.ApprovedRevision, next.ApprovedContentHash = 0, ""
	return next, false, nil
}

func ApplyPlanProgress(current *Plan, update PlanProgressUpdate, now time.Time) (Plan, error) {
	if current == nil || current.Status != PlanActive {
		return Plan{}, fmt.Errorf("%w: active plan is required", ErrPlanInvalid)
	}
	if len(update.Updates) == 0 {
		return Plan{}, fmt.Errorf("%w: updates are required", ErrPlanInvalid)
	}
	next := *current
	next.Steps = clonePlanSteps(current.Steps)
	seen := map[string]bool{}
	for _, patch := range update.Updates {
		if patch.StepID == "" || seen[patch.StepID] {
			return Plan{}, fmt.Errorf("%w: updates must have unique step ids", ErrPlanInvalid)
		}
		seen[patch.StepID] = true
		found := false
		for i := range next.Steps {
			if next.Steps[i].ID == patch.StepID {
				found = true
				if next.Steps[i].Status == PlanStepCompleted && patch.Status != PlanStepCompleted {
					return Plan{}, fmt.Errorf("%w: completed step cannot regress", ErrPlanInvalid)
				}
				next.Steps[i].Status = patch.Status
				break
			}
		}
		if !found {
			return Plan{}, fmt.Errorf("%w: unknown step %q", ErrPlanInvalid, patch.StepID)
		}
	}
	inProgress := 0
	for _, step := range next.Steps {
		if step.Status == PlanStepInProgress {
			inProgress++
		}
	}
	if inProgress > 1 {
		return Plan{}, fmt.Errorf("%w: at most one step may be in_progress", ErrPlanInvalid)
	}
	next.Revision++
	next.UpdatedAt = now.Unix()
	if !next.HasBlockingSteps() {
		next.Status = PlanCompleted
	}
	return next, nil
}

func (p Plan) ContentHash() string {
	h := sha256.New()
	_, _ = h.Write([]byte(p.Title))
	_, _ = h.Write([]byte("\x00"))
	_, _ = h.Write([]byte(p.Content))
	for _, step := range p.Steps {
		_, _ = h.Write([]byte("\x00" + step.ID + "\x00" + step.Title))
		for _, slideID := range step.TargetSlideIDs {
			_, _ = h.Write([]byte("\x00" + slideID))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func clonePlanSteps(in []PlanStep) []PlanStep { return append([]PlanStep{}, in...) }
func (p Plan) HasBlockingSteps() bool {
	for _, step := range p.Steps {
		if step.Status != PlanStepCompleted {
			return true
		}
	}
	return false
}
