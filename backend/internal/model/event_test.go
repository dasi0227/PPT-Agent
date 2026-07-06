package model

import "testing"

// plan/plan.update 是非终态事件（V2-SSE-004：不改变"终态恰好一个 done/error"）。
func TestPlanEventsNonTerminal(t *testing.T) {
	for _, evt := range []EventType{EventPlan, EventPlanUpdate} {
		if evt.Terminal() {
			t.Errorf("%s must be non-terminal", evt)
		}
	}
	for _, evt := range []EventType{EventDone, EventError} {
		if !evt.Terminal() {
			t.Errorf("%s must be terminal", evt)
		}
	}
}
