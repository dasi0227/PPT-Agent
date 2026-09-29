package contextengine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/testsupport"
)

func TestTranscriptLoadsExistingScreenshotHistoryAsTextAndKeepsUploads(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "artifacts")
	path := filepath.Join(model.ProjectRoot(dir), TranscriptPath("thread"))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	store := NewJournalTranscriptStore(testsupport.NewJournal(dir))
	seed := []llm.Message{
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "render", Name: "render_slide", Args: map[string]any{"slide_id": "sli_one"}}}},
		{Role: llm.RoleTool, ToolCallID: "render", Content: []llm.ContentPart{{Type: "text", Text: "layout checked"}, {Type: "image", ImageRef: "project:pro_one/render:sli_one/shot_one"}}},
		{Role: llm.RoleUser, Content: []llm.ContentPart{{Type: "image", ImageRef: "project:pro_one/attachment:att_one/original"}}},
	}
	if err := store.Replace(dir, "thread", seed); err != nil {
		t.Fatal(err)
	}
	messages, err := store.Load(dir, "thread")
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 3 || messages[1].Text() != "layout checked" || len(messages[1].Content) != 1 || len(messages[2].Content) != 1 || messages[2].Content[0].Type != "image" {
		t.Fatalf("history or attachment lost: %+v", messages)
	}
	if len(messages[0].ToolCalls) != 1 || messages[0].ToolCalls[0].ID != messages[1].ToolCallID || messages[0].ToolCalls[0].Args["slide_id"] != "sli_one" {
		t.Fatalf("current tool call lost its result association: %+v", messages)
	}
	messages = append(messages, llm.Message{Role: llm.RoleTool, ToolCallID: "read", Content: []llm.ContentPart{{Type: "text", Text: "render path"}, {Type: "image", ImageRef: "project:pro_one/render:sli_one/shot_new"}}})
	if err := store.Replace(dir, "thread", messages); err != nil {
		t.Fatal(err)
	}
	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(persisted), "shot_new") || strings.Contains(string(persisted), "shot_one") || !strings.Contains(string(persisted), "att_one") {
		t.Fatalf("transcript persisted render pixels or dropped attachment: %s", persisted)
	}
}
