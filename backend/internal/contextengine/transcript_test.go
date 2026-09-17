package contextengine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestFSTranscriptStoreRoundTripsAndClassifiesMessages(t *testing.T) {
	workDir := filepath.Join(t.TempDir(), "projects", "p1", "artifacts")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store := NewFSTranscriptStore()
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("make a deck")},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "read-1", Name: "read_ppt"}}},
		{Role: llm.RoleTool, ToolCallID: "read-1", Content: llm.TextContent("<html>")},
	}
	if err := store.Replace(workDir, "thread", messages); err != nil {
		t.Fatal(err)
	}
	entries, err := store.LoadEntries(workDir, "thread")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(messages) ||
		entries[0].Type != BucketChatHistory ||
		entries[2].Type != BucketReadFile {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	loaded, err := store.Load(workDir, "thread")
	if err != nil || loaded[2].Text() != "<html>" {
		t.Fatalf("round trip failed: messages=%+v err=%v", loaded, err)
	}
	if TranscriptPath("thread") != "threads/thread/model.jsonl" {
		t.Fatalf("unexpected transcript path: %s", TranscriptPath("thread"))
	}
	raw, err := os.ReadFile(filepath.Join(model.ProjectRoot(workDir), filepath.FromSlash(TranscriptPath("thread"))))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"layer"`) {
		t.Fatalf("transcript still persists layer labels: %s", raw)
	}
}

func TestFSTranscriptStoreRejectsLegacyContextTypes(t *testing.T) {
	workDir := filepath.Join(t.TempDir(), "projects", "p1", "artifacts")
	path := filepath.Join(model.ProjectRoot(workDir), filepath.FromSlash(TranscriptPath("thread")))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"role":"user","content":[],"type":"user_prompt"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFSTranscriptStore().LoadEntries(workDir, "thread"); err == nil {
		t.Fatal("legacy transcript context type was accepted")
	}
}
