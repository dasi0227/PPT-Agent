package contextengine

import (
	"context"
	"github.com/dasi0227/PPT-Agent/backend/internal/testsupport"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestJournalTranscriptStoreRoundTripsAndClassifiesMessages(t *testing.T) {
	workDir := filepath.Join(t.TempDir(), "projects", "p1", "artifacts")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store := NewJournalTranscriptStore(testsupport.NewJournal(workDir))
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("make a deck")},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "read-1", Name: "read_ppt"}}},
		{Role: llm.RoleTool, ToolCallID: "read-1", Content: llm.TextContent("<html>"), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "resource", Resources: []llm.ResourceStamp{{Key: "ppt/slide:sli_a:html", Hash: "hash"}}}},
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
	if err != nil || loaded[2].Text() != "<html>" || loaded[2].Metadata == nil || loaded[2].Metadata.Resources[0].Hash != "hash" {
		t.Fatalf("round trip failed: messages=%+v err=%v", loaded, err)
	}
	if TranscriptPath("thread") != "threads/thread/thread.jsonl" {
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

func TestJournalTranscriptStoreRejectsLegacyContextTypes(t *testing.T) {
	workDir := filepath.Join(t.TempDir(), "projects", "p1", "artifacts")
	path := filepath.Join(model.ProjectRoot(workDir), filepath.FromSlash(TranscriptPath("thread")))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"role":"user","content":[],"type":"user_prompt"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewJournalTranscriptStore(testsupport.NewJournal(workDir)).LoadEntries(workDir, "thread"); err == nil {
		t.Fatal("legacy transcript context type was accepted")
	}
}

func TestCompressionPreservesLaterInputAndCanReplaceEarlierSummary(t *testing.T) {
	workDir := filepath.Join(t.TempDir(), "projects", "p1", "artifacts")
	journal := testsupport.NewJournal(workDir)
	store := NewJournalTranscriptStore(journal)
	original := []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("first")}, {Role: llm.RoleAssistant, Content: llm.TextContent("answer")}}
	if err := store.Replace(workDir, "thread", original); err != nil {
		t.Fatal(err)
	}
	later := llm.Message{Role: llm.RoleUser, Content: llm.TextContent("arrived during compression")}
	if err := store.Replace(workDir, "thread", append(append([]llm.Message{}, original...), later)); err != nil {
		t.Fatal(err)
	}
	summary := []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("first summary")}}
	if err := store.ReplaceFrom(workDir, "thread", original, summary); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(workDir, "thread")
	if err != nil || len(loaded) != 2 || loaded[1].Text() != later.Text() {
		t.Fatalf("later input lost: %+v %v", loaded, err)
	}
	second := []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("second summary")}}
	if err := store.ReplaceFrom(workDir, "thread", summary, second); err != nil {
		t.Fatal(err)
	}
	loaded, err = store.Load(workDir, "thread")
	if err != nil || len(loaded) != 2 || loaded[0].Text() != "second summary" || loaded[1].Text() != later.Text() {
		t.Fatalf("successive compression: %+v %v", loaded, err)
	}
	events, err := journal.ThreadEvents(context.Background(), "thread", 0)
	if err != nil || len(events) != 4 {
		t.Fatalf("original history was lost: %d %v", len(events), err)
	}
}
