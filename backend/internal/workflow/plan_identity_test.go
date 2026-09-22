package workflow

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPlanApprovalIdentitySurvivesRecoveryAndChangesWithProposal(t *testing.T) {
	var update PlanUpdate
	if err := json.Unmarshal([]byte(`{"title":"Plan","content":"Do the work","steps":[{"title":"Build"}]}`), &update); err != nil {
		t.Fatal(err)
	}
	first, _, err := ApplyPlanUpdate(nil, update, "run", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	var restored Plan
	raw, _ := json.Marshal(first)
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if first.ApprovalID == "" || restored.ApprovalID != first.ApprovalID {
		t.Fatal("recovery changed approval identity")
	}
	next, _, err := ApplyPlanUpdate(&restored, update, "run", time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	if next.ID != first.ID || next.ApprovalID == first.ApprovalID {
		t.Fatal("replacement proposal did not get a new approval identity")
	}
}
