package llm

import (
	"reflect"
	"testing"
)

func TestNormalizeHistoryRemovesObsoleteRenderArgumentsWithoutMutatingInput(t *testing.T) {
	messages := []Message{
		{Role: RoleUser, Content: TextContent("Keep visual_review in this quoted document.")},
		{Role: RoleAssistant, ToolCalls: []ToolCall{
			{ID: "render", Name: "render_slide", Args: map[string]any{"slide_id": "sli_one", "visual_review": true}},
			{ID: "other", Name: "other_tool", Args: map[string]any{"visual_review": true}},
		}},
		{Role: RoleTool, ToolCallID: "render", Content: TextContent("layout passed")},
	}
	got := NormalizeHistory(messages)
	if !reflect.DeepEqual(got[1].ToolCalls[0], ToolCall{ID: "render", Name: "render_slide", Args: map[string]any{"slide_id": "sli_one"}}) {
		t.Fatalf("obsolete render contract replayed: %+v", got[1].ToolCalls[0])
	}
	if !reflect.DeepEqual(got[0], messages[0]) || !reflect.DeepEqual(got[1].ToolCalls[1], messages[1].ToolCalls[1]) || !reflect.DeepEqual(got[2], messages[2]) {
		t.Fatal("unrelated history or tool result was changed")
	}
	if messages[1].ToolCalls[0].Args["visual_review"] != true {
		t.Fatal("normalization mutated the original call")
	}
	if !reflect.DeepEqual(NormalizeHistory(got), got) {
		t.Fatal("normalization is not idempotent")
	}
}
