package sqlite

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

func TestConversationKeepsPrivateMessagesOutAndCurrentPlanSeparate(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := sessionstate.WithRequestScene(context.Background(), 1)
	plan := model.PublicPlan{Steps: []model.PublicPlanStep{{Title: "制作图表", Status: "pending"}}}
	runUpdateForTest(t, s, "r", model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: model.NewPublicEventBase("r"), Plan: &plan})
	conversationMessageForTest(t, s, "r", "PRIVATE_TOOL_RESULT_AND_APPROVAL", false)
	conversationMessageForTest(t, s, "r", "已整理方案", true)
	source, err := s.LoadThreadConversationContext(ctx, "t")
	if err != nil || len(source.Rounds) != 1 || source.Plan == nil || source.Plan.Steps[0].Title != plan.Steps[0].Title {
		t.Fatalf("lost current conversation/plan: %+v %v", source, err)
	}
	raw, _ := json.Marshal(source.Rounds)
	if len(source.Rounds[0].Messages) != 2 || strings.Contains(string(raw), "PRIVATE_") {
		t.Fatalf("private execution data entered conversation: %s", raw)
	}
}
