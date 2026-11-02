package workflow

import (
	"encoding/json"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func pendingProposal(callID, content, title string) *PendingPlan {
	proposal := model.PlanProposal{Content: content, Steps: []string{title}}
	return &PendingPlan{Call: llm.ToolCall{ID: callID, Name: "create_plan", Args: map[string]any{"content": content, "steps": proposal.Steps}},
		Request: model.PlanApprovalRequestedPayload{InteractionID: "approval-" + callID, Plan: proposal}}
}

func TestPendingProposalSurvivesRecoveryIndependentlyOfExecutionSteps(t *testing.T) {
	pending := pendingProposal("call", "Do the work", "Build")
	checklist := proposalChecklist(pending.Request.Plan)
	checklist.Steps[0].Title = "Changed"
	raw, err := json.Marshal(pending)
	if err != nil {
		t.Fatal(err)
	}
	var restored PendingPlan
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Request.InteractionID != pending.Request.InteractionID || restored.Request.Plan.Steps[0] != "Build" {
		t.Fatal(restored.Request)
	}
}
