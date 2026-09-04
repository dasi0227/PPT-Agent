package contextengine

import (
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

func TestFSTranscriptStoreRoundTripsAndClassifiesMessages(t *testing.T) {
	workDir := t.TempDir()
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
		entries[0].Type != BucketUserPrompt ||
		entries[2].Type != BucketReadPPT ||
		entries[2].Layer != LayerTranscript {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	loaded, err := store.Load(workDir, "thread")
	if err != nil || loaded[2].Text() != "<html>" {
		t.Fatalf("round trip failed: messages=%+v err=%v", loaded, err)
	}
	if filepath.Base(TranscriptPath("thread")) != "thread.transcript.jsonl" {
		t.Fatalf("unexpected transcript path: %s", TranscriptPath("thread"))
	}
}
