package workflow

import (
	"fmt"
	"sort"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type SlideWorkStatus string

const (
	SlideWorkPending SlideWorkStatus = "pending"
	SlideWorkRunning SlideWorkStatus = "running"
	SlideWorkDone    SlideWorkStatus = "done"
	SlideWorkFailed  SlideWorkStatus = "failed"
)

type SlideWorkItem struct {
	SlideID    string          `json:"slide_id"`
	Status     SlideWorkStatus `json:"status"`
	PlanStepID string          `json:"plan_step_id,omitempty"`
	LastError  string          `json:"last_error,omitempty"`
}

type WorkLedger struct {
	mu    sync.Mutex               `json:"-"`
	Items map[string]SlideWorkItem `json:"items"`
}

func NewWorkLedger() *WorkLedger { return &WorkLedger{Items: map[string]SlideWorkItem{}} }

func (w *WorkLedger) SyncPlan(plan *Plan, scope model.RunScope) error {
	if w == nil || plan == nil {
		return nil
	}
	for _, step := range plan.Steps {
		for _, slideID := range step.TargetSlideIDs {
			if !scope.ContainsSlide(slideID) {
				return fmt.Errorf("%w: plan target %s is outside active scope", ErrPlanInvalid, slideID)
			}
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, step := range plan.Steps {
		for _, slideID := range step.TargetSlideIDs {
			current, exists := w.Items[slideID]
			if exists && current.Status == SlideWorkDone {
				continue
			}
			status := SlideWorkPending
			if step.Status == PlanStepInProgress {
				status = SlideWorkRunning
			}
			if step.Status == PlanStepCompleted {
				status = SlideWorkDone
			}
			if step.Status == PlanStepFailed {
				status = SlideWorkFailed
			}
			w.Items[slideID] = SlideWorkItem{SlideID: slideID, Status: status, PlanStepID: step.ID, LastError: current.LastError}
		}
	}
	return nil
}

func (w *WorkLedger) MarkRunning(slideIDs []string, scope model.RunScope) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, id := range slideIDs {
		if !scope.ContainsSlide(id) {
			continue
		}
		item := w.Items[id]
		item.SlideID, item.Status, item.LastError = id, SlideWorkRunning, ""
		w.Items[id] = item
	}
}

func (w *WorkLedger) Complete(slideIDs []string, ok bool, message string) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, id := range slideIDs {
		item, exists := w.Items[id]
		if !exists {
			continue
		}
		if ok {
			item.Status, item.LastError = SlideWorkDone, ""
		} else {
			item.Status, item.LastError = SlideWorkFailed, message
		}
		w.Items[id] = item
	}
}

func (w *WorkLedger) HasBlockingItems() bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, item := range w.Items {
		if item.Status != SlideWorkDone {
			return true
		}
	}
	return false
}

func (w *WorkLedger) Snapshot() []SlideWorkItem {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]SlideWorkItem, 0, len(w.Items))
	for _, item := range w.Items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SlideID < out[j].SlideID })
	return out
}
