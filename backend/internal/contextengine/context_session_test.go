package contextengine

import (
	"reflect"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

func TestPreparedContextKeepsCandidateIdentityAcrossCommitRetry(t *testing.T) {
	snapshot := sessionstate.ContextSnapshot{ThreadID: "thread", SceneRevision: 3, Revision: 4}
	session, err := NewContextSession(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	messages := []llm.Message{
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "call", Name: "read", Args: map[string]any{"path": "page"}}}},
		{Role: llm.RoleTool, ToolCallID: "call", Content: llm.TextContent("result"), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "observation"}},
	}
	first, err := session.Prepare(messages, "run", "append")
	if err != nil {
		t.Fatal(err)
	}
	for _, retryInput := range [][]llm.Message{messages, first.Messages} {
		retry, err := session.Prepare(retryInput, "run", "append")
		if err != nil || !reflect.DeepEqual(first.Update, retry.Update) {
			t.Fatalf("same candidate changed commit payload: %+v %v", retry.Update, err)
		}
	}
	if err := session.Publish(first, sessionstate.Receipt{SceneRevision: 3, ContextRevision: 5}); err != nil {
		t.Fatal(err)
	}
	unchanged, err := session.Prepare(session.Messages(), "run", "")
	if err != nil || len(unchanged.Update.NewMessages) != 0 || !reflect.DeepEqual(unchanged.Update.MessageIDs, first.Update.MessageIDs) {
		t.Fatalf("committed bodies were rewritten: %+v %v", unchanged.Update, err)
	}
	changed := session.Messages()
	changed[1].Content = llm.TextContent("different result")
	updated, err := session.Prepare(changed, "run", "")
	if err != nil || updated.Update.MessageIDs[1] == first.Update.MessageIDs[1] {
		t.Fatal("changed native body reused an immutable identity", err)
	}
	snapshot.SceneRevision++
	other, err := NewContextSession(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := other.Prepare(messages, "run", "append")
	if err != nil || restored.Update.MessageIDs[0] == first.Update.MessageIDs[0] {
		t.Fatal("message identity crossed project scenes", err)
	}
}
