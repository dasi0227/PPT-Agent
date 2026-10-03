package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/gitcommit"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
	"go.uber.org/zap"
)

func TestCommitGenerationUsesOnlyStagedEvidenceAndOneModelCall(t *testing.T) {
	provider := &llmtest.FakeProvider{Caps: llm.Capabilities{ToolCalls: true}, Script: []llm.GenerateResponse{{
		ToolCalls: []llm.ToolCall{{Name: "git_commit", Args: map[string]any{"title": "fix: 修正页码", "items": []any{"让页码从一开始计数"}}}},
	}}}
	changes := gitcommit.ChangeSet{
		AttemptID: "private-attempt", IndexPath: "/private/index", NameStatus: "M\tsli_one.html\n",
		NumStat: "1\t1\tsli_one.html\n", Diff: "-page = index\n+page = index + 1\n", DiffTruncated: true,
	}
	profile := llm.NewTestProfile("Commit", "https://example.invalid", provider)
	message, err := generateGitCommitMessage(context.Background(), profile, changes)
	if err != nil || message.Title != "fix: 修正页码" {
		t.Fatalf("message=%+v err=%v", message, err)
	}
	requests := provider.Requests()
	if len(requests) != 1 || len(requests[0].Messages) != 2 || len(requests[0].Tools) != 1 || requests[0].Continuation != nil {
		t.Fatalf("not an isolated call: %+v", requests)
	}
	if requests[0].Messages[0].Text() != prompts.MustLoad("command.commit").Body {
		t.Fatal("commit inherited another policy")
	}
	var input map[string]any
	if err := json.Unmarshal([]byte(requests[0].Messages[1].Text()), &input); err != nil {
		t.Fatal(err)
	}
	if len(input) != 4 || input["file_status"] != changes.NameStatus || input["line_statistics"] != changes.NumStat || input["staged_diff"] != changes.Diff || input["diff_truncated"] != true {
		t.Fatalf("staged evidence changed or unrelated context added: %v", input)
	}
	provider.Exhausted = llm.GenerateResponse{Content: llm.TextContent("invalid plain-text result")}
	if _, err := generateGitCommitMessage(context.Background(), profile, changes); err == nil || len(provider.Requests()) != 4 {
		t.Fatal("invalid output must exhaust only two correction requests")
	}
	provider.Exhausted.ToolCalls = []llm.ToolCall{{Name: "git_commit", Args: map[string]any{"title": "fix: 修正页码", "items": []any{"让页码从一开始计数"}}}}
	if _, err := generateGitCommitMessage(context.Background(), profile, changes); err == nil || len(provider.Requests()) != 5 {
		t.Fatal("prose alongside an unsafe call must fail without replay")
	}
}

func TestRenameUsesRecentChronologicalRecapAndStandalonePolicy(t *testing.T) {
	f := newBriefingFixture(t)
	ctx := context.Background()
	appendEvent := func(kind string, value any) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.AppendThreadEvent(ctx, f.thread.ID, threadjournal.Event{RunID: "run-recap", Type: kind, Payload: raw}); err != nil {
			t.Fatal(err)
		}
	}
	appendEvent("run.accepted", map[string]any{"command": map[string]any{"instruction": "STALE_FIRST_GOAL"}})
	appendEvent(string(model.EventPlanUpdated), model.PlanUpdatedPayload{Plan: model.PublicPlan{Title: "STALE_PLAN", Content: "OLD_PLAN_BODY"}})
	appendEvent("context.compacted", map[string]any{"content": "STALE_COMPACTION"})
	for i := 1; i <= 6; i++ {
		appendEvent("steering.accepted", model.SteeringMessage{Content: fmt.Sprintf("图表打磨第%d次要求", i)})
		appendEvent(string(model.EventMessageFinal), model.MessageFinalPayload{Text: fmt.Sprintf("完成第%d次调整", i)})
	}
	appendEvent(string(model.EventPlanUpdated), model.PlanUpdatedPayload{Plan: model.PublicPlan{
		PlanID: "private-plan", Title: "打磨图表", Content: "FULL_PLAN_BODY", Status: "active",
		Steps: []model.PublicPlanStep{
			{ID: "private-step-1", Title: "调整数据标签", Status: "completed", TargetSlideIDs: []string{"sli_aaaaaa"}},
			{ID: "private-step-2", Title: "统一图例", Status: "processing"},
		},
	}})
	f.provider.Script = []llm.GenerateResponse{{ToolCalls: []llm.ToolCall{{Name: "rename_thread", Args: map[string]any{"action": "keep"}}}}}
	svc := NewNamingService(f.store, f.provider, NewThreadEventHub(f.store), zap.NewNop())
	t.Cleanup(svc.Close)
	if _, err := svc.GenerateNow(ctx, f.thread.ID); err != nil {
		t.Fatal(err)
	}
	requests := f.provider.Requests()
	if len(requests) != 1 || len(requests[0].Messages) != 2 || len(requests[0].Tools) != 1 || requests[0].Continuation != nil || requests[0].Messages[0].Text() != prompts.MustLoad("command.rename").Body {
		t.Fatalf("rename inherited agent input: %+v", requests)
	}
	user := requests[0].Messages[1].Text()
	for _, forbidden := range []string{"STALE_", "FULL_PLAN_BODY", "OLD_PLAN_BODY", "private-plan", "private-step", "sli_aaaaaa", "第1次", "第2次", "完成第3次", "完成第4次"} {
		if strings.Contains(user, forbidden) {
			t.Fatalf("rename includes %q: %s", forbidden, user)
		}
	}
	var input renameInput
	if err := json.Unmarshal([]byte(user), &input); err != nil {
		t.Fatal(err)
	}
	if len(input.RecentActivity) != 6 || input.RecentActivity[0].Text != "图表打磨第3次要求" || input.RecentActivity[5].Text != "完成第6次调整" || input.Progress == nil || input.Progress.CompletedSteps != 1 || input.Progress.Steps[0].Title != "统一图例" {
		t.Fatalf("recap lost chronology or progress: %+v", input)
	}
}

func TestRenameBudgetPreservesLatestCorrection(t *testing.T) {
	source := model.ThreadRenameContextSource{Activity: []model.ThreadNamingActivity{
		{Role: "user", Text: "近期任务：" + strings.Repeat("\x01", 5000) + "最后改为投资人路演"},
		{Role: "assistant", Text: strings.Repeat("\x02", 5000)},
		{Role: "assistant", Text: strings.Repeat("\x03", 5000)},
	}}
	input, err := buildRenameInput("当前会话", source)
	if err != nil || !strings.Contains(input, "最后改为投资人路演") {
		t.Fatalf("latest user correction dropped: %s, %v", input, err)
	}
	request := llm.GenerateRequest{Messages: []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent(prompts.MustLoad("command.rename").Body)},
		{Role: llm.RoleUser, Content: llm.TextContent(input)},
	}, Tools: []llm.ToolSchema{renameThreadToolSchema()}}
	if llm.EstimateRequestTokens(request) > renameInputBudget {
		t.Fatal("rename request exceeded budget")
	}
}
