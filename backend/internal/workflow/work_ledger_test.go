package workflow

import (
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestWorkLedgerTracksOnlyExplicitPlanTargets(t *testing.T) {
	scope := model.NewRunScope(model.ScopeObjectPresentation, model.ScopeCustomPages, "sli_one", "sli_two")
	plan := &Plan{Steps: []PlanStep{{ID: "step_one", Title: "修改第一页", Status: PlanStepPending, TargetSlideIDs: []string{"sli_one"}}}}
	ledger := NewWorkLedger()
	if err := ledger.SyncPlan(plan, scope); err != nil {
		t.Fatal(err)
	}
	if len(ledger.Items) != 1 || ledger.Items["sli_one"].Status != SlideWorkPending {
		t.Fatalf("unexpected work ledger: %#v", ledger.Items)
	}
	if _, exists := ledger.Items["sli_two"]; exists {
		t.Fatal("scope-only slide must not become a work item")
	}
	ledger.MarkRunning([]string{"sli_one"}, scope)
	ledger.Complete([]string{"sli_one"}, true, "")
	if ledger.HasBlockingItems() {
		t.Fatal("completed explicit work must not block completion")
	}
}

func TestWorkLedgerRejectsPlanTargetsOutsideScope(t *testing.T) {
	scope := model.NewRunScope(model.ScopeObjectHTML, model.ScopeCurrentPage, "sli_one")
	plan := &Plan{Steps: []PlanStep{
		{ID: "step_one", Title: "范围内", Status: PlanStepPending, TargetSlideIDs: []string{"sli_one"}},
		{ID: "step_two", Title: "越界", Status: PlanStepPending, TargetSlideIDs: []string{"sli_two"}},
	}}
	ledger := NewWorkLedger()
	if err := ledger.SyncPlan(plan, scope); err == nil {
		t.Fatal("expected out-of-scope plan target to be rejected")
	}
	if len(ledger.Items) != 0 {
		t.Fatalf("rejected plan must not partially mutate the ledger: %#v", ledger.Items)
	}
}
