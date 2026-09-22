package contextengine

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestBriefingPreservesDiscussionAndReferences(t *testing.T) {
	project, store := fixture(t)
	proposal := "第二种方案：\n" + strings.Repeat("保留关键结论与参考资料。", 80) + "\n仅改结论页，不修改其他页面。"
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("<context_summary>早期讨论：面向董事会，重点是投资决策。</context_summary>")},
		{Role: llm.RoleUser, Content: llm.TextContent("需要改进结论页，请先讨论方案。")},
		{Role: llm.RoleAssistant, Content: llm.TextContent(proposal)},
	}
	for i := 0; i < 4; i++ {
		messages = append(messages,
			llm.Message{Role: llm.RoleUser, Content: llm.TextContent("保持原有的数据与品牌色。")},
			llm.Message{Role: llm.RoleAssistant, Content: llm.TextContent("可以保持，只调整信息层级。")},
		)
	}
	selection := model.DOMSelection{SelectionID: "sel_1", SlideID: "sli_aaaaaa", Comment: "放大结论标题", HTMLRevision: 2,
		DOMTargets: []model.DOMTarget{{TextSummary: "投资结论", OuterHTML: "PRIVATE_LARGE_HTML"}},
	}
	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: []llm.ContentPart{
		{Type: "text", Text: "就用第二种方案，但先出原型。"},
		{Type: "text", Text: `<image_attachment>{"attachment_id":"att_1","purpose":"参考配色"}</image_attachment>`},
		{Type: "image", ImageRef: "project:p1/att_1"},
		{Type: "text", Text: "<selected_dom>" + string(stableJSON(selection)) + "</selected_dom>"},
	}})
	if err := NewFSTranscriptStore().Replace(project.WorkDir, "t1", messages); err != nil {
		t.Fatal(err)
	}
	pack, err := testAssembler(store, nil).AssembleBriefing(context.Background(), BriefingContextRequest{ThreadID: "t1", Kind: model.BriefingKickoff}, project)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Conversation[1].Role != "assistant" || pack.Conversation[1].Text != proposal {
		t.Fatal("early multiline proposal was truncated or lost")
	}
	compiled, err := CompileBriefingContext(pack)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"投资决策", "先出原型", "att_1", "参考配色", "sel_1", "放大结论标题", "slides/sli_aaaaaa/spec.json"} {
		if !strings.Contains(compiled, want) {
			t.Errorf("briefing lost %q", want)
		}
	}
	if strings.Contains(compiled, "PRIVATE_LARGE_HTML") {
		t.Fatal("historical DOM snapshot leaked into briefing")
	}
}

func TestBriefingSeparatesProposalsFromExecutionEvidence(t *testing.T) {
	project, store := fixture(t)
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("先确认方案")},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "plan", Name: "create_plan", Args: map[string]any{"content": "只修改结论页"}}}},
		{Role: llm.RoleTool, ToolCallID: "plan", Content: llm.TextContent("Plan saved; awaiting approval")},
		{Role: llm.RoleUser, Content: llm.TextContent("The user approved the plan. Approval is complete and the runtime is now in execute mode.")},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "question", Name: "ask_user", Args: map[string]any{"question": "是否保留数据？"}}}},
		{Role: llm.RoleTool, ToolCallID: "question", Content: llm.TextContent("用户选择：保留数据")},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "read", Name: "read_ppt"}}},
		{Role: llm.RoleTool, ToolCallID: "read", Content: llm.TextContent("PRIVATE_READ_SNAPSHOT")},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "edit", Name: "run_command", Args: map[string]any{"command": "edit conclusion", "content": "PRIVATE_WRITE_PAYLOAD"}}}},
		{Role: llm.RoleTool, ToolCallID: "edit", Content: llm.TextContent("write failed: permission denied")},
		{Role: llm.RoleUser, Content: llm.TextContent("Ordinary assistant text is not a completion signal. INTERNAL_GUIDANCE")},
	}
	if err := NewFSTranscriptStore().Replace(project.WorkDir, "t1", messages); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []model.BriefingKind{model.BriefingKickoff, model.BriefingHandoff} {
		pack, err := testAssembler(store, nil).AssembleBriefing(context.Background(), BriefingContextRequest{ThreadID: "t1", Kind: kind}, project)
		if err != nil {
			t.Fatal(err)
		}
		compiled, _ := CompileBriefingContext(pack)
		for _, want := range []string{"只修改结论页", "awaiting approval", "historical_approval", "保留数据"} {
			if !strings.Contains(compiled, want) {
				t.Errorf("%s lost discussion %q", kind, want)
			}
		}
		for _, excluded := range []string{"PRIVATE_READ_SNAPSHOT", "PRIVATE_WRITE_PAYLOAD", "INTERNAL_GUIDANCE"} {
			if strings.Contains(compiled, excluded) {
				t.Errorf("%s included irrelevant payload %q", kind, excluded)
			}
		}
		if kind == model.BriefingKickoff && len(pack.ExecutionEvidence) != 0 {
			t.Fatal("kickoff received execution logs")
		}
		if kind == model.BriefingHandoff && (len(pack.ExecutionEvidence) != 1 || pack.ExecutionEvidence[0].Result != "write failed: permission denied") {
			t.Fatalf("handoff lost actual failure evidence: %+v", pack.ExecutionEvidence)
		}
	}
}

func TestBriefingBudgetProtectsDiscussion(t *testing.T) {
	estimator := StableTokenEstimator{}
	discussion := []BriefingTurn{
		{Role: "user", Text: "希望结论更清楚", Exchange: 1},
		{Role: "assistant", Text: "第二种方案是保持数据，放大结论。", Exchange: 1},
		{Role: "user", Text: "就按第二种方案", Exchange: 2},
	}
	pack := BriefingContext{Kind: model.BriefingKickoff, Conversation: append([]BriefingTurn(nil), discussion...),
		Resources: []BriefingResource{{Ref: "design.json", Content: strings.Repeat("无关设计细节", 4000)}},
	}
	if err := trimBriefingContext(&pack, estimator, 1200); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pack.Conversation, discussion) || pack.Resources[0].Ref != "design.json" || estimator.Estimate(pack) > 1200 {
		t.Fatal("resource contents displaced discussion or lost its reference")
	}

	pack = BriefingContext{Kind: model.BriefingKickoff, HistorySummary: "旧结论已确认，最新用户修订优先。"}
	for i := 1; i <= 8; i++ {
		pack.Conversation = append(pack.Conversation, BriefingTurn{Role: "user", Text: strings.Repeat("长讨论上下文。", 800), Exchange: i})
	}
	pack.Conversation = append(pack.Conversation,
		BriefingTurn{Role: "assistant", Text: "最新方案：仅调整结论页", Exchange: 8},
		BriefingTurn{Role: "user", Text: "按这个来，先做原型。", Exchange: 9},
	)
	if err := trimBriefingContext(&pack, estimator, 2000); err != nil {
		t.Fatal(err)
	}
	if estimator.Estimate(pack) > 2000 || pack.HistorySummary == "" || pack.Conversation[0].Exchange != 1 {
		t.Fatal("context exceeded its budget or lost its opening/summary")
	}
	for _, turn := range pack.Conversation {
		if !utf8.ValidString(turn.Text) {
			t.Fatal("budget truncation broke Unicode")
		}
	}
	compiled, _ := CompileBriefingContext(pack)
	for _, want := range []string{"最新方案：仅调整结论页", "先做原型", "omitted"} {
		if !strings.Contains(compiled, want) {
			t.Errorf("trimmed discussion lost %q", want)
		}
	}
}

func TestBriefingReportsUnreadableHistory(t *testing.T) {
	project, store := fixture(t)
	transcript := NewFSTranscriptStore()
	if err := transcript.Create(project.WorkDir, "t1"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(model.ProjectRoot(project.WorkDir), TranscriptPath("t1"))
	if err := os.WriteFile(path, []byte("broken history"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := testAssembler(store, nil).AssembleBriefing(context.Background(), BriefingContextRequest{ThreadID: "t1", Kind: model.BriefingKickoff}, project)
	if err == nil {
		t.Fatal("unreadable history silently became an empty discussion")
	}
}
