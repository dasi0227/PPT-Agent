package contextengine

import (
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

func TestNativeContextKeepsScreenshotResultsAndUploads(t *testing.T) {
	seed := []llm.Message{
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "render", Name: "render_slide", Args: map[string]any{"slide_id": "sli_one"}}}},
		{Role: llm.RoleTool, ToolCallID: "render", Content: []llm.ContentPart{{Type: "text", Text: "layout checked"}, {Type: "image", ImageRef: "project:pro_one/render:sli_one/shot_one"}}},
		{Role: llm.RoleUser, Content: []llm.ContentPart{{Type: "image", ImageRef: "project:pro_one/attachment:att_one/original"}}},
	}
	messages := make([]llm.Message, len(seed))
	for i, message := range seed {
		raw, err := sessionstate.EncodeMessage(message)
		if err != nil {
			t.Fatal(err)
		}
		messages[i], err = sessionstate.DecodeMessage(raw)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(messages) != 3 || messages[1].Text() != "layout checked" || len(messages[1].Content) != 2 || messages[1].Content[1].Type != "image" || len(messages[2].Content) != 1 || messages[2].Content[0].Type != "image" {
		t.Fatalf("history or attachment lost: %+v", messages)
	}
	if len(messages[0].ToolCalls) != 1 || messages[0].ToolCalls[0].ID != messages[1].ToolCallID || messages[0].ToolCalls[0].Args["slide_id"] != "sli_one" {
		t.Fatalf("current tool call lost its result association: %+v", messages)
	}
}
