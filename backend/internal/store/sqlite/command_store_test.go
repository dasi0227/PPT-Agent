package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

func TestCommandRetryKeepsSuccessfulResultAndRejectsLateAttempt(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	request := model.CommandRequest{RequestKey: "first", Kind: "polish", Input: json.RawMessage(`{"instruction":"original"}`)}
	first, created, err := s.AcceptCommand(ctx, "t", request, 1)
	if err != nil || !created {
		t.Fatalf("accept: %v", err)
	}
	replay, created, err := s.AcceptCommand(ctx, "t", request, 1)
	if err != nil || created || replay.Command.AttemptID != first.Command.AttemptID || replay.Receipt.CommitID != first.Receipt.CommitID {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	changed := request
	changed.Input = json.RawMessage(`{"instruction":"different"}`)
	if _, _, err := s.AcceptCommand(ctx, "t", changed, 1); !errors.Is(err, sessionstate.ErrIdempotencyConflict) {
		t.Fatalf("key reuse: %v", err)
	}
	execution := first.Command
	execution.Status = "completed"
	execution.Result = json.RawMessage(`{"content":"success"}`)
	completed, err := s.CompleteCommand(ctx, execution, sessionstate.CommandEffects{})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestKey, request.CommandID, request.BaseAttemptID, request.ExpectedRevision = "retry", completed.CommandID, completed.AttemptID, completed.EntityRevision
	next, _, err := s.AcceptCommand(ctx, "t", request, 1)
	if err != nil {
		t.Fatal(err)
	}
	replay, created, err = s.AcceptCommand(ctx, "t", model.CommandRequest{RequestKey: "first", Kind: "polish", Input: json.RawMessage(`{"instruction":"original"}`)}, 1)
	if err != nil || created || replay.Command.AttemptID != completed.AttemptID || replay.Command.Status != "completed" {
		t.Fatalf("old receipt switched attempt: %+v %v", replay, err)
	}
	late := execution
	late.Status = "running"
	late.Result = nil
	if err := s.SaveCommandExecution(ctx, late); !errors.Is(err, store.ErrCommandConflict) {
		t.Fatalf("late attempt: %v", err)
	}
	failed := next.Command
	failed.Status = "failed"
	failed.Error = json.RawMessage(`{"message":"failure"}`)
	if err := s.SaveCommandExecution(ctx, failed); err != nil {
		t.Fatal(err)
	}
	result, err := s.GetCommand(ctx, completed.CommandID)
	if err != nil || result.LatestSuccess == nil || string(result.LatestSuccess.Result) != string(completed.Result) {
		t.Fatalf("lost success: %+v %v", result, err)
	}
}

func TestRestartInterruptsCommandWithoutExecutingAgain(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	accepted, _, err := s.AcceptCommand(ctx, "t", model.CommandRequest{RequestKey: "start", Kind: "polish", Input: json.RawMessage(`{}`)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.interruptCommands(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetCommand(ctx, accepted.Command.CommandID)
	if err != nil || got.Status != "interrupted" || len(got.Error) == 0 {
		t.Fatalf("interrupted projection: %+v %v", got, err)
	}
	if err := s.SaveCommandExecution(ctx, accepted.Command); !errors.Is(err, store.ErrCommandConflict) {
		t.Fatalf("old worker: %v", err)
	}
}

func TestCommandDatabaseFailureDoesNotPublishPartialStatus(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	accepted, _, err := s.AcceptCommand(ctx, "t", model.CommandRequest{RequestKey: "start", Kind: "polish", Input: json.RawMessage(`{}`)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	before, err := readSessionHead(s.reader, "t")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.db.Exec(`CREATE TRIGGER fail_command BEFORE UPDATE ON command_executions BEGIN SELECT RAISE(ABORT,'failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	command := accepted.Command
	command.Status = "completed"
	command.Result = json.RawMessage(`{"content":"result"}`)
	if _, err := s.CompleteCommand(ctx, command, sessionstate.CommandEffects{}); err == nil {
		t.Fatal("expected persistence failure")
	}
	got, err := s.GetCommand(ctx, command.CommandID)
	if err != nil || got.Status != "accepted" {
		t.Fatalf("partial command: %+v %v", got, err)
	}
	after, err := readSessionHead(s.reader, "t")
	if err != nil || after.LastSeq != before.LastSeq {
		t.Fatalf("rolled-back terminal published: %+v %v", after, err)
	}
}

func TestCommandTerminalReceiptDoesNotAcknowledgeRevokedExecution(t *testing.T) {
	for _, status := range []string{"completed", "failed"} {
		t.Run(status, func(t *testing.T) {
			s, _ := checkpointFixture(t)
			ctx := context.Background()
			accepted, _, err := s.AcceptCommand(ctx, "t", model.CommandRequest{RequestKey: "terminal-fence", Kind: "polish", Input: json.RawMessage(`{}`)}, 1)
			if err != nil {
				t.Fatal(err)
			}
			command := accepted.Command
			command.Status = status
			command.Result = json.RawMessage(`{"content":"prepared"}`)
			commit := func(execution model.CommandExecution) error {
				if status == "completed" {
					_, err := s.CompleteCommand(ctx, execution, sessionstate.CommandEffects{})
					return err
				}
				return s.SaveCommandExecution(ctx, execution)
			}
			if err := commit(command); err != nil {
				t.Fatal(err)
			}
			if err := commit(command); err != nil {
				t.Fatalf("same execution did not reuse terminal receipt: %v", err)
			}
			// Model a recovery claim after an unknown write outcome. Old receipts
			// must not grant the previous worker this new execution generation.
			recovered := command
			recovered.OwnerInstanceID = "recovery-owner"
			recovered.ExecutionRevision++
			if err := s.db.Model(&commandExecutionPO{}).Where("id = ?", command.AttemptID).Updates(map[string]any{"status": "running", "owner_instance_id": recovered.OwnerInstanceID, "execution_revision": recovered.ExecutionRevision}).Error; err != nil {
				t.Fatal(err)
			}
			if err := commit(command); !errors.Is(err, store.ErrCommandConflict) {
				t.Fatalf("old receipt acknowledged a revoked execution: %v", err)
			}
			if err := commit(recovered); err != nil {
				t.Fatalf("recovery generation could not commit: %v", err)
			}
			current, err := s.GetCommand(ctx, command.CommandID)
			if err != nil || current.OwnerInstanceID != recovered.OwnerInstanceID || current.ExecutionRevision != recovered.ExecutionRevision || current.Status != status {
				t.Fatalf("wrong terminal execution: %+v %v", current, err)
			}
		})
	}
}

func TestAutomaticCommandRetainsOriginatingRunInDirectProjection(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	accepted, _, err := s.AcceptCommand(ctx, "t", model.CommandRequest{RunID: "r", Source: "automatic", RequestKey: "auto-name", Kind: "rename", Input: json.RawMessage(`{"mode":"automatic"}`)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	thread, err := s.GetThread(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	command := accepted.Command
	command.Status = "completed"
	command.Result = json.RawMessage(`{"title":"名称"}`)
	if _, err := s.CompleteCommand(ctx, command, sessionstate.CommandEffects{Rename: &sessionstate.RenameEffect{Title: "名称", OperationVersion: thread.RenameOperationVersion}}); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.GetCommand(ctx, command.CommandID)
	if err != nil || loaded.RunID != "r" || loaded.Source != "automatic" {
		t.Fatalf("run ownership lost: %+v %v", loaded, err)
	}
	changes, err := s.SessionChanges(ctx, sessionstate.Cursor{ProjectID: "p", ThreadID: "t", SceneRevision: 1}, 100)
	if err != nil {
		t.Fatal(err)
	}
	var projected model.CommandExecution
	for _, batch := range changes.Batches {
		for _, change := range batch.Changes {
			if change.Type == "command.upsert" {
				if err := json.Unmarshal(change.Entity, &projected); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if projected.CommandID != command.CommandID || projected.RunID != "r" || projected.Status != "completed" {
		t.Fatalf("public projection lost origin: %+v %v", projected, err)
	}
	var count int64
	if err := s.reader.Model(&timelineItemPO{}).Where("id = ?", "command:"+command.CommandID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("metadata-only command entered visible history: %d %v", count, err)
	}
}
