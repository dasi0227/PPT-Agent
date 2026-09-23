package contextengine

import (
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestPublicSourceTextKeepsCommentsNotSnapshotMetadata(t *testing.T) {
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentPart{
			{Type: "text", Text: "回答"},
			{Type: "text", Text: `<selected_dom>{"comment":"解释 read_ppt","slide_id":"sli_internal","dom_targets":[{"text_summary":"mutate_ppt"}]}</selected_dom>`},
			{Type: "text", Text: `<selected_dom_reference>{"comment":"再解释 render_slide","slide_id":"sli_other"}</selected_dom_reference>`},
			{Type: "text", Text: `<image_attachment>{"name":"ask_user.png"}</image_attachment>`},
			{Type: "image", ImageRef: "project:pro_internal/attachment:att_one/original"},
		}},
		{Role: llm.RoleUser, Content: llm.TextContent("runtime metadata"), Metadata: &llm.MessageMetadata{Origin: "runtime"}},
		{Role: llm.RoleAssistant, Content: llm.TextContent("assistant text")},
	}
	source := PublicSourceText(messages)
	if source != "回答\n解释 read_ppt\n再解释 render_slide" {
		t.Fatalf("unexpected user-authored source: %q", source)
	}
	got := model.PublicText("read_ppt render_slide mutate_ppt sli_internal", model.PublicTextContext{SourceText: source, Pages: map[string]string{"sli_internal": "第 1 页"}})
	if got != "read_ppt render_slide 修改演示内容 第 1 页" {
		t.Fatalf("snapshot metadata leaked into display exemptions: %q", got)
	}
}
