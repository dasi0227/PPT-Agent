package contextengine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/testsupport"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
)

func TestJournalTranscriptStoreRoundTripsAndClassifiesMessages(t *testing.T) {
	workDir := filepath.Join(t.TempDir(), "projects", "p1", "artifacts")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store := NewJournalTranscriptStore(testsupport.NewJournal(workDir))
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("make a deck")},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "read-1", Name: "read_resource"}},
			Replay: &llm.AssistantReplay{Source: "source", Blocks: []json.RawMessage{
				json.RawMessage(`{"type":"thinking","thinking":"inspect","signature":"signed"}`),
				json.RawMessage(`{"type":"tool_use","id":"read-1","name":"read_resource","input":{}}`),
			}}},

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
		entries[0].Role != llm.RoleUser ||
		entries[2].Role != llm.RoleTool {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	loaded, err := store.Load(workDir, "thread")
	if err != nil || len(loaded) != 3 || !reflect.DeepEqual(loaded[1].Replay, messages[1].Replay) || loaded[2].Text() != "<html>" || loaded[2].Metadata == nil || loaded[2].Metadata.Resources[0].Hash != "hash" {
		t.Fatalf("round trip failed: messages=%+v err=%v", loaded, err)
	}
	if model.ThreadJournalPath("thread") != "threads/thread/thread.jsonl" {
		t.Fatalf("unexpected transcript path: %s", model.ThreadJournalPath("thread"))
	}
	raw, err := os.ReadFile(filepath.Join(model.ProjectRoot(workDir), filepath.FromSlash(model.ThreadJournalPath("thread"))))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"layer"`) {
		t.Fatalf("transcript still persists layer labels: %s", raw)
	}
}

func TestJournalTranscriptStoreRejectsLegacyContextTypes(t *testing.T) {
	workDir := filepath.Join(t.TempDir(), "projects", "p1", "artifacts")
	path := filepath.Join(model.ProjectRoot(workDir), filepath.FromSlash(model.ThreadJournalPath("thread")))
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
	if err := store.ReplaceFromContext(context.Background(), workDir, "thread", original, summary); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(workDir, "thread")
	if err != nil || len(loaded) != 2 || loaded[1].Text() != later.Text() {
		t.Fatalf("later input lost: %+v %v", loaded, err)
	}
	second := []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("second summary")}}
	if err := store.ReplaceFromContext(context.Background(), workDir, "thread", summary, second); err != nil {
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

func TestTaskTranscriptExcludesDirectExchangeThroughCompaction(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "projects", "p1", "artifacts")
	ctx := context.Background()
	journal := testsupport.NewJournal(dir)
	store := NewJournalTranscriptStore(journal)
	appendEvent := func(runID, kind string, payload any) {
		t.Helper()
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := journal.AppendThreadEvent(ctx, "thread", threadjournal.Event{RunID: runID, Type: kind, Payload: raw}); err != nil {
			t.Fatal(err)
		}
	}
	instruction := func(id, runID, text string) projectedEntry {
		return projectedEntry{ID: id, Entry: TranscriptEntry{Role: llm.RoleUser, Content: llm.TextContent(text),
			Metadata: &llm.MessageMetadata{Origin: "user", Kind: "instruction", RunID: runID}}}
	}
	task := instruction("task-input", "authoring", "修改第 3 页")
	final := projectedEntry{ID: "task-final", Entry: TranscriptEntry{Role: llm.RoleAssistant, Content: llm.TextContent("第 3 页已修改")}}
	appendEvent("authoring", "model.edit", modelEdit{Entries: []projectedEntry{task, final}})
	appendEvent("chat", string(model.EventMessageFinal), model.MessageFinalPayload{MessageID: "input_aside", Text: "不用谢"})
	appendEvent("chat", "model.edit", modelEdit{Watermark: 2, Entries: []projectedEntry{
		instruction("aside", "chat", "谢谢"), {ID: "aside-reply", Entry: TranscriptEntry{Role: llm.RoleAssistant, Content: llm.TextContent("不用谢"),
			Replay: &llm.AssistantReplay{Source: "side", Blocks: []json.RawMessage{json.RawMessage(`{"type":"thinking","thinking":"side thought"}`)}}}},
	}})
	mixedSummary := projectedEntry{ID: "mixed", Entry: TranscriptEntry{Role: llm.RoleUser,
		Content: llm.TextContent("第 3 页已修改。用户说谢谢。"), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "summary"}}}
	appendEvent("", "model.edit", modelEdit{Watermark: 3, Remove: []string{"task-input", "task-final", "aside", "aside-reply"}, Entries: []projectedEntry{mixedSummary}})
	messages, err := store.Load(dir, "thread")
	if err != nil || !reflect.DeepEqual(messages, []llm.Message{task.Entry.Message(), final.Entry.Message()}) {
		t.Fatalf("mixed summary did not resolve to its task source: %+v %v", messages, err)
	}
	clean := mixedSummary.Entry.Message()
	clean.Content = llm.TextContent("第 3 页已修改")
	if err := store.ReplaceFromContext(ctx, dir, "thread", messages, []llm.Message{clean}); err != nil {
		t.Fatal(err)
	}
	messages, err = store.Load(dir, "thread")
	if err != nil || !reflect.DeepEqual(messages, []llm.Message{clean}) {
		t.Fatalf("task-only compaction changed on replay: %+v %v", messages, err)
	}
}
