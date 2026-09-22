package contextengine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestTranscriptLoadsExistingScreenshotHistoryAsTextAndKeepsUploads(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "artifacts")
	path := filepath.Join(model.ProjectRoot(dir), TranscriptPath("thread"))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	raw := `{"role":"tool","tool_call_id":"render","type":"read_file","content":[{"Type":"text","Text":"layout checked"},{"Type":"image","ImageRef":"run:old/screenshot:shot_one"}]}` + "\n" +
		`{"role":"user","type":"chat_history","content":[{"Type":"image","ImageRef":"project:pro_one/attachment:att_one/original"}]}` + "\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewFSTranscriptStore()
	messages, err := store.Load(dir, "thread")
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0].Text() != "layout checked" || len(messages[0].Content) != 1 || len(messages[1].Content) != 1 || messages[1].Content[0].Type != "image" {
		t.Fatalf("history or attachment lost: %+v", messages)
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
