package sqlite

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func toolCheckpointFixture(t *testing.T, policy sessionstate.ToolRecoveryPolicy) (*Store, workflow.RuntimeCheckpoint) {
	t.Helper()
	s, cp := checkpointFixture(t)
	call := llm.ToolCall{ID: "call", Name: "read_resource", Args: map[string]any{"path": "a"}}
	hash, args, err := sessionstate.ToolArgumentsHash(call.Name, call.Args)
	if err != nil {
		t.Fatal(err)
	}
	cp.PendingToolBatch = &workflow.PendingToolBatch{CallIDs: []string{call.ID}, AssistantMessageID: "assistant"}
	cp.ToolChanges = []sessionstate.ToolExecution{{ID: sessionstate.ToolAttemptID(cp.RunID, call.ID, 1), RunID: cp.RunID, CallID: call.ID, AttemptNo: 1, Name: call.Name, Arguments: args, ArgumentsHash: hash, Status: "prepared", RecoveryPolicy: policy}}
	cp.ContextUpdate = &sessionstate.ContextUpdate{ExpectedRevision: 0, Reason: "append", MessageIDs: []string{"assistant"}, NewMessages: []sessionstate.Message{{ID: "assistant", RunID: cp.RunID, RoundID: cp.RunID, Kind: "assistant", Native: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{call}}}}}
	if err := s.SaveCheckpoint(context.Background(), cp); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.LatestCheckpoint(context.Background(), cp.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.ToolExecutions) != 1 || len(loaded.PendingToolBatch.Calls) != 1 || loaded.PendingToolBatch.Calls[0].Name != call.Name {
		t.Fatal("pending tool references did not hydrate")
	}
	return s, loaded
}

func TestToolResultContextCheckpointAndPublicOutcomeRollbackTogether(t *testing.T) {
	s, cp := toolCheckpointFixture(t, sessionstate.ToolReadOnly)
	ctx := context.Background()
	row := cp.ToolExecutions[0]
	row.Status = "executing"
	cp.ToolChanges = []sessionstate.ToolExecution{row}
	started, err := sessionstate.NewRunUpdate(model.EventToolStarted, model.ToolStartedPayload{PublicEventBase: model.NewPublicEventBase(cp.RunID), CallID: "call", Tool: "read_resource", Display: model.PublicDisplay{Label: "读取资料"}})
	if err != nil {
		t.Fatal(err)
	}
	cp.PublicUpdates = []sessionstate.RunUpdate{started}
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatal(err)
	}
	cp, err = s.LatestCheckpoint(ctx, cp.RunID)
	if err != nil {
		t.Fatal(err)
	}
	row = cp.ToolExecutions[0]
	row.Status = "succeeded"
	row.ResultMessageID = "result"
	row.ResultJSON = json.RawMessage(`{"result":{"ok":true,"applied":true,"summary":"read"}}`)
	cp.ToolChanges = []sessionstate.ToolExecution{row}
	cp.ContextUpdate = &sessionstate.ContextUpdate{ExpectedRevision: cp.ContextRevision, Reason: "append", MessageIDs: []string{"assistant", "result"}, NewMessages: []sessionstate.Message{{ID: "result", RunID: cp.RunID, RoundID: cp.RunID, Kind: "tool_result", Native: llm.Message{Role: llm.RoleTool, ToolCallID: "call", Content: llm.TextContent("read")}}}}
	fact, err := sessionstate.NewRunUpdate(model.EventToolCompleted, model.ToolCompletedPayload{PublicEventBase: model.NewPublicEventBase(cp.RunID), CallID: "call", Tool: "read_resource", Status: "completed", Display: model.PublicDisplay{Label: "读取资料"}})
	if err != nil {
		t.Fatal(err)
	}
	cp.PublicUpdates = []sessionstate.RunUpdate{fact}
	if err := s.db.Exec(`CREATE TRIGGER reject_tool_checkpoint BEFORE UPDATE OF checkpoint_json ON runs BEGIN SELECT RAISE(ABORT,'checkpoint failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCheckpoint(ctx, cp); err == nil {
		t.Fatal("expected checkpoint failure")
	}
	stored, err := s.GetToolExecution(ctx, cp.RunID, "call")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "executing" || stored.ResultMessageID != "" || len(stored.ResultJSON) != 0 {
		t.Fatal("tool result escaped checkpoint rollback")
	}
	var count int64
	if err := s.db.Table("conversation_messages").Where("id='result'").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("partial observation: %d %v", count, err)
	}
	if err := s.db.Exec(`DROP TRIGGER reject_tool_checkpoint`).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatalf("exact retry failed: %v", err)
	}
	stored, err = s.GetToolExecution(ctx, cp.RunID, "call")
	if err != nil || !stored.Terminal() || stored.ResultMessageID != "result" {
		t.Fatalf("missing committed observation: %+v %v", stored, err)
	}
	contextState, err := s.LoadContext(ctx, "t")
	if err != nil || len(contextState.Messages) != 2 || contextState.Messages[1].Native.ToolCallID != "call" {
		t.Fatalf("unpaired context: %+v %v", contextState, err)
	}
	latest, err := s.LatestCheckpoint(ctx, cp.RunID)
	if err != nil {
		t.Fatal(err)
	}
	latest.PendingToolBatch = nil
	if err := s.SaveCheckpoint(ctx, latest); err != nil {
		t.Fatalf("completed batch cannot be consumed: %v", err)
	}
}

func TestOnlyReadOnlyUnknownToolCanPrepareAnotherAttempt(t *testing.T) {
	for _, policy := range []sessionstate.ToolRecoveryPolicy{sessionstate.ToolReadOnly, sessionstate.ToolUnknown} {
		t.Run(string(policy), func(t *testing.T) {
			s, cp := toolCheckpointFixture(t, policy)
			ctx := sessionstate.WithRequestScene(context.Background(), 1)
			row := cp.ToolExecutions[0]
			row.Status = "executing"
			cp.ToolChanges = []sessionstate.ToolExecution{row}
			if err := s.SaveCheckpoint(ctx, cp); err != nil {
				t.Fatal(err)
			}
			if _, err := s.PauseRun(ctx, cp.RunID, cp.OwnerInstanceID, "restart", 1); err != nil {
				t.Fatal(err)
			}
			if _, err := resumeRunForTest(t, s, cp.RunID, "new-owner", "resume"); err != nil {
				t.Fatal(err)
			}
			cp, err := s.LatestCheckpoint(ctx, cp.RunID)
			if err != nil {
				t.Fatal(err)
			}
			row = cp.ToolExecutions[0]
			if row.Status != "outcome_unknown" {
				t.Fatal("dispatched tool was not isolated")
			}
			row.AttemptNo++
			row.ID = sessionstate.ToolAttemptID(cp.RunID, row.CallID, row.AttemptNo)
			row.Status = "prepared"
			row.EntityRevision = 0
			cp.ToolChanges = []sessionstate.ToolExecution{row}
			err = s.SaveCheckpoint(ctx, cp)
			if policy == sessionstate.ToolReadOnly && err != nil {
				t.Fatalf("safe attempt rejected: %v", err)
			}
			if policy == sessionstate.ToolUnknown && err == nil {
				t.Fatal("unknown side effect was blindly retried")
			}
		})
	}
}
