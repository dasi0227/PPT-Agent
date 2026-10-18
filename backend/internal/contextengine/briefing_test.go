package contextengine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/testsupport"
)

func TestBriefingPreservesDiscussionAndReferences(t *testing.T) {
	project := fixture(t)
	proposal := "第二种方案：\n" + strings.Repeat("保留关键结论与参考资料。", 8000) + "\n仅改结论页，不修改其他页面。"
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("<compact_summary>早期讨论：面向董事会，重点是投资决策。</compact_summary>"), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "summary"}},
		{Role: llm.RoleUser, Content: llm.TextContent("需要改进结论页，请先讨论方案。")},
		{Role: llm.RoleAssistant, Content: llm.TextContent(proposal)},
	}
	for i := 0; i < 4; i++ {
		messages = append(messages,
			llm.Message{Role: llm.RoleUser, Content: llm.TextContent("保持原有的数据与品牌色。")},
			llm.Message{Role: llm.RoleAssistant, Content: llm.TextContent("可以保持，只调整信息层级。")},
		)
	}
	selection := model.DOMSelection{SelectionID: "sel_1", SlideID: "sli_aaaaaa", Comment: "放大结论标题",
		DOMTargets: []model.DOMTarget{{TextSummary: "投资结论", OuterHTML: "PRIVATE_LARGE_HTML"}},
	}
	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: []llm.ContentPart{
		{Type: "text", Text: "就用第二种方案，但先出原型。"},
		{Type: "text", Text: `<image_attachment>{"attachment_id":"att_1","purpose":"参考配色"}</image_attachment>`},
		{Type: "image", ImageRef: "project:p1/att_1"},
		{Type: "text", Text: "<user_annotation>" + string(stableJSON(selection)) + "</user_annotation>"},
	}})
	if err := NewJournalTranscriptStore(testsupport.NewJournal(project.WorkDir)).Replace(project.WorkDir, "t1", messages); err != nil {
		t.Fatal(err)
	}
	pack, err := NewContextAssembler().AssembleBriefing(context.Background(), BriefingContextRequest{ThreadID: "t1", Kind: model.BriefingHandoff}, project)
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
	for _, want := range []string{"投资决策", "先出原型", "att_1", "参考配色", "sel_1", "放大结论标题", "第 1 页"} {
		if !strings.Contains(compiled, want) {
			t.Errorf("briefing lost %q", want)
		}
	}
	if strings.Contains(compiled, "PRIVATE_LARGE_HTML") {
		t.Fatal("historical DOM snapshot leaked into briefing")
	}
}

func TestBriefingSeparatesProposalsFromExecutionEvidence(t *testing.T) {
	project := fixture(t)
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("先确认方案")},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "plan", Name: "create_plan", Args: map[string]any{"content": "只修改结论页"}}}},
		{Role: llm.RoleTool, ToolCallID: "plan", Content: llm.TextContent("Plan saved; awaiting approval")},
		{Role: llm.RoleUser, Content: llm.TextContent("The user approved the plan. Approval is complete and the runtime is now in create mode."), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "approval"}},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "question", Name: "ask_user", Args: map[string]any{"question": "是否保留数据？"}}}},
		{Role: llm.RoleTool, ToolCallID: "question", Content: llm.TextContent("用户选择：保留数据")},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "read", Name: "read_resource"}}},
		{Role: llm.RoleTool, ToolCallID: "read", Content: llm.TextContent("PRIVATE_READ_SNAPSHOT")},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "edit", Name: "run_command", Args: map[string]any{"command": "edit conclusion", "content": "PRIVATE_WRITE_PAYLOAD"}}}},
		{Role: llm.RoleTool, ToolCallID: "edit", Content: llm.TextContent("write failed: permission denied")},
		{Role: llm.RoleUser, Content: llm.TextContent("Completion check rejected this reply. INTERNAL_GUIDANCE"), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "runtime_instruction"}},
	}
	if err := NewJournalTranscriptStore(testsupport.NewJournal(project.WorkDir)).Replace(project.WorkDir, "t1", messages); err != nil {
		t.Fatal(err)
	}
	pack, err := NewContextAssembler().AssembleBriefing(context.Background(), BriefingContextRequest{ThreadID: "t1", Kind: model.BriefingHandoff}, project)
	if err != nil {
		t.Fatal(err)
	}
	compiled, _ := CompileBriefingContext(pack)
	for _, want := range []string{"只修改结论页", "awaiting approval", "historical_approval", "保留数据"} {
		if !strings.Contains(compiled, want) {
			t.Errorf("handoff lost discussion %q", want)
		}
	}
	for _, excluded := range []string{"PRIVATE_READ_SNAPSHOT", "PRIVATE_WRITE_PAYLOAD", "INTERNAL_GUIDANCE"} {
		if strings.Contains(compiled, excluded) {
			t.Errorf("handoff included irrelevant payload %q", excluded)
		}
	}
	if len(pack.ExecutionEvidence) != 1 || pack.ExecutionEvidence[0].Result != "write failed: permission denied" {
		t.Fatalf("handoff lost actual failure evidence: %+v", pack.ExecutionEvidence)
	}
}

func TestBriefingReportsUnreadableHistory(t *testing.T) {
	project := fixture(t)
	transcript := NewJournalTranscriptStore(testsupport.NewJournal(project.WorkDir))
	if err := transcript.Replace(project.WorkDir, "t1", []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("initial")}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(model.ProjectRoot(project.WorkDir), model.ThreadJournalPath("t1"))
	if err := os.WriteFile(path, []byte("broken history"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := NewContextAssembler().AssembleBriefing(context.Background(), BriefingContextRequest{ThreadID: "t1", Kind: model.BriefingHandoff}, project)
	if err == nil {
		t.Fatal("unreadable history silently became an empty discussion")
	}
}
