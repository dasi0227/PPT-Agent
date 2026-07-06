package generate

import (
	"context"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
)

// AC-V2-CTR-002：整套生成事件流出现恰好一个 plan，其后 plan.update 的 step_id 均命中该 plan。
func TestPlanEmittedOnceAndStepsHit(t *testing.T) {
	store, dir, slides := setupGen(t, 7)
	em := &pageEmitter{}
	out := newGenRunner(store, dir, "tokyo-night", "gen1", nil).Run(context.Background(), em, nil, nil)
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("want finished, got %s (%s)", out.Status, out.Message)
	}

	if len(em.plans) != 1 {
		t.Fatalf("V2-PLAN-001: want exactly 1 plan event, got %d", len(em.plans))
	}
	plan := em.plans[0]
	// 每页一条 + validate + deliver。
	if len(plan.Steps) != len(slides)+2 {
		t.Fatalf("want %d steps, got %d", len(slides)+2, len(plan.Steps))
	}

	known := map[string]bool{}
	for _, s := range plan.Steps {
		known[s.ID] = true
		if s.Status != planStatusPending {
			t.Errorf("initial step %s status=%q, want pending", s.ID, s.Status)
		}
	}

	// V2-SSE-002：每个 plan.update.step_id 必须命中已发 plan 的 step，且 plan.id 一致。
	if len(em.updates) == 0 {
		t.Fatal("expected plan.update events")
	}
	for _, u := range em.updates {
		if u.ID != plan.ID {
			t.Errorf("update plan id=%q, want %q", u.ID, plan.ID)
		}
		if !known[u.StepID] {
			t.Errorf("V2-SSE-002: update step_id %q not in plan", u.StepID)
		}
	}
}

// AC-V2-PIPE-003：第 k 页开始/完成分别收到 in_progress / completed，且 in_progress 先于 completed。
func TestPlanUpdatePerPageLifecycle(t *testing.T) {
	store, dir, slides := setupGen(t, 3)
	em := &pageEmitter{}
	if out := newGenRunner(store, dir, "tokyo-night", "gen1", nil).Run(context.Background(), em, nil, nil); out.Status != harness.OutcomeFinished {
		t.Fatalf("want finished, got %s (%s)", out.Status, out.Message)
	}

	for _, sl := range slides {
		stepID := pageStepID(sl.Idx)
		inProgAt, doneAt := -1, -1
		for i, u := range em.updates {
			if u.StepID != stepID {
				continue
			}
			if u.Status == planStatusInProgress && inProgAt == -1 {
				inProgAt = i
			}
			if u.Status == planStatusCompleted {
				doneAt = i
			}
		}
		if inProgAt == -1 || doneAt == -1 {
			t.Fatalf("step %s missing in_progress(%d)/completed(%d)", stepID, inProgAt, doneAt)
		}
		if inProgAt >= doneAt {
			t.Errorf("step %s: in_progress(%d) must precede completed(%d)", stepID, inProgAt, doneAt)
		}
	}

	// 校验/交付步收尾为 completed。
	for _, want := range []string{planStepValidate, planStepDeliver} {
		found := false
		for _, u := range em.updates {
			if u.StepID == want && u.Status == planStatusCompleted {
				found = true
			}
		}
		if !found {
			t.Errorf("closing step %s not completed", want)
		}
	}
}

// 单页重生成走精简路径：不发跨页 plan（V2-AGENT-PIPELINE §7 / AC-V2-PIPE-005）。
func TestSinglePageRegenEmitsNoPlan(t *testing.T) {
	store, dir, _ := setupGen(t, 5)
	if out := newGenRunner(store, dir, "tokyo-night", "gen1", nil).Run(context.Background(), &pageEmitter{}, nil, nil); out.Status != harness.OutcomeFinished {
		t.Fatalf("initial gen failed: %s", out.Message)
	}
	two := 2
	em := &pageEmitter{}
	if out := newGenRunner(store, dir, "tokyo-night", "gen2", &two).Run(context.Background(), em, nil, nil); out.Status != harness.OutcomeFinished {
		t.Fatalf("regen failed: %s", out.Message)
	}
	if len(em.plans) != 0 {
		t.Errorf("single-page regen must not emit plan, got %d", len(em.plans))
	}
	if len(em.updates) != 0 {
		t.Errorf("single-page regen must not emit plan.update, got %d", len(em.updates))
	}
}

// buildPlan 纯函数：step id 命名与 pageStepID 一致，末两步为 validate/deliver。
func TestBuildPlanDeterministic(t *testing.T) {
	_, _, slides := setupGen(t, 4)
	plan := buildPlan("r9", slides)
	if plan.ID != "plan_r9" {
		t.Errorf("plan id=%q, want plan_r9", plan.ID)
	}
	for i, sl := range slides {
		if plan.Steps[i].ID != pageStepID(sl.Idx) {
			t.Errorf("step %d id=%q, want %q", i, plan.Steps[i].ID, pageStepID(sl.Idx))
		}
	}
	n := len(plan.Steps)
	if plan.Steps[n-2].ID != planStepValidate || plan.Steps[n-1].ID != planStepDeliver {
		t.Errorf("last two steps must be validate/deliver, got %q/%q", plan.Steps[n-2].ID, plan.Steps[n-1].ID)
	}
}
