package model

import "testing"

func TestAdaptiveRuntimeTerminalEvents(t *testing.T) {
	for _, evt := range []EventType{
		EventStrategySelected, EventPlanCreated, EventVerificationCompleted,
		EventArtifactCommitted, EventStatusSummary,
	} {
		if evt.Terminal() {
			t.Errorf("%s must be non-terminal", evt)
		}
	}
	for _, evt := range []EventType{EventRunCompleted, EventRunFailed, EventRunCanceled} {
		if !evt.Terminal() {
			t.Errorf("%s must be terminal", evt)
		}
	}
}
