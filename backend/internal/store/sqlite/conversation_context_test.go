package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

func TestConversationPreservesAllRoundsAndUnfinishedInput(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := sessionstate.WithRequestScene(context.Background(), 1)
	for i := 0; i < 7; i++ {
		id := "r"
		if i > 0 {
			id = fmt.Sprintf("run-%d", i)
			if err := s.CreateRun(ctx, model.Run{ID: id, ProjectID: "p", ThreadID: "t", SceneRevision: 1, ClientRequestID: id, OwnerInstanceID: "owner", Status: model.RunRunning, Command: model.RunCommand{Instruction: "请求" + id, Mode: model.ModePlan, Scope: model.NewRunScope(model.ScopeAllPages)}}); err != nil {
				t.Fatal(err)
			}
		}
		for j := 0; j < 3; j++ {
			steeringForTest(t, s, id, fmt.Sprintf("%s-add-%d", id, j), fmt.Sprintf("补充%d", j))
		}
		conversationMessageForTest(t, s, id, "收到补充", true)
		runUpdateForTest(t, s, id, model.EventMessageFinal, model.MessageFinalPayload{PublicEventBase: model.NewPublicEventBase(id), MessageID: "final-" + id, Text: "完成" + id, AffectedTargets: []model.PublicTarget{}})
		runUpdateForTest(t, s, id, model.EventRunCompleted, model.NewRunTerminalPayload(id, 0, nil, nil))
	}
	if err := s.CreateRun(ctx, model.Run{ID: "unfinished", ProjectID: "p", ThreadID: "t", SceneRevision: 1, ClientRequestID: "unfinished", OwnerInstanceID: "owner", Status: model.RunRunning, Command: model.RunCommand{Instruction: "尚无最终回复", Mode: model.ModePlan, Scope: model.NewRunScope(model.ScopeAllPages)}}); err != nil {
		t.Fatal(err)
	}
	source, err := s.LoadThreadConversationContext(ctx, "t")
	if err != nil || len(source.Rounds) != 8 {
		t.Fatalf("lost rounds: %+v %v", source, err)
	}
	for i, round := range source.Rounds[:7] {
		if len(round.Messages) != 6 || round.Messages[4].Text != "收到补充" || !strings.HasPrefix(round.Messages[5].Text, "完成") {
			t.Fatalf("round %d reordered: %+v", i, round)
		}
	}
	if last := source.Rounds[7].Messages; len(last) != 1 || last[0].Text != "尚无最终回复" {
		t.Fatalf("unfinished input lost: %+v", last)
	}
	recent, err := s.LoadRecentThreadConversationContext(ctx, "t")
	if err != nil || len(recent.Rounds) == 0 || recent.Rounds[len(recent.Rounds)-1].Messages[0].Text != "尚无最终回复" {
		t.Fatalf("recent window lost unfinished prefix: %+v %v", recent, err)
	}
}

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

func TestManualClearIsIdleOnlyAndPersistsWithoutARun(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := sessionstate.WithRequestScene(context.Background(), 1)
	runUpdateForTest(t, s, "r", model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: model.NewPublicEventBase("r"), Plan: &model.PublicPlan{Steps: []model.PublicPlanStep{{Title: "Build", Status: "processing"}}}})
	before, err := s.LatestThreadPlan(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClearThreadPlan(ctx, "t", before.Seq, "clear"); !errors.Is(err, model.ErrThreadPlanBusy) {
		t.Fatalf("running clear: %v", err)
	}
	if _, err := s.PauseRun(ctx, "r", "owner", "interrupted", 2); err != nil {
		t.Fatal(err)
	}
	receipt, err := s.ClearThreadPlan(ctx, "t", before.Seq, "clear")
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := s.ClearThreadPlan(ctx, "t", before.Seq, "clear")
	if err != nil || repeated.CommitID != receipt.CommitID {
		t.Fatalf("retry changed receipt: %+v %v", repeated, err)
	}
	snapshot, err := s.LatestThreadPlan(ctx, "t")
	if err != nil || snapshot.Seq != before.Seq+1 || snapshot.Plan != nil || snapshot.RunID != "" {
		t.Fatalf("snapshot=%+v err=%v", snapshot, err)
	}
}
