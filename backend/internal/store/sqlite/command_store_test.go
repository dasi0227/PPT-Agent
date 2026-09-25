package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

func TestCommandRetryKeepsSuccessfulResultAndRejectsLateAttempt(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	request := model.CommandRequest{RequestKey: "first", Kind: "polish", Input: json.RawMessage(`{"instruction":"original"}`)}
	first, created, err := s.AcceptCommand(ctx, "t", request, 0)
	if err != nil || !created {
		t.Fatalf("accept: %v", err)
	}
	replay, created, err := s.AcceptCommand(ctx, "t", request, 0)
	if err != nil || created || replay.AttemptID != first.AttemptID {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	changed := request
	changed.Input = json.RawMessage(`{"instruction":"different"}`)
	if _, _, err := s.AcceptCommand(ctx, "t", changed, 0); err == nil {
		t.Fatal("request key reused with another body")
	}
	first.Status = "completed"
	first.Result = json.RawMessage(`{"content":"success"}`)
	if err := s.SaveCommandExecution(ctx, first); err != nil {
		t.Fatal(err)
	}
	request.RequestKey = "retry"
	request.CommandID = first.CommandID
	request.BaseAttemptID = first.AttemptID
	next, _, err := s.AcceptCommand(ctx, "t", request, 0)
	if err != nil {
		t.Fatal(err)
	}
	replay, created, err = s.AcceptCommand(ctx, "t", model.CommandRequest{RequestKey: "first", Kind: "polish", Input: json.RawMessage(`{"instruction":"original"}`)}, 0)
	if err != nil || created || replay.AttemptID != first.AttemptID || replay.Status != "completed" {
		t.Fatalf("old receipt returned a newer attempt: %+v %v", replay, err)
	}
	if err := s.SaveCommandExecution(ctx, first); !errors.Is(err, store.ErrCommandConflict) {
		t.Fatalf("late attempt: %v", err)
	}
	next.Status = "failed"
	next.Error = json.RawMessage(`{"message":"failure"}`)
	if err := s.SaveCommandExecution(ctx, next); err != nil {
		t.Fatal(err)
	}
	result, err := s.GetCommand(ctx, first.CommandID)
	if err != nil || result.LatestSuccess == nil || string(result.LatestSuccess.Result) != string(first.Result) {
		t.Fatalf("lost successful revision: %+v %v", result, err)
	}
}
func TestRestartInterruptsCommandWithoutExecutingAgain(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	command, _, err := s.AcceptCommand(ctx, "t", model.CommandRequest{RequestKey: "start", Kind: "polish", Input: json.RawMessage(`{}`)}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.interruptCommands(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetCommand(ctx, command.CommandID)
	if err != nil || got.Status != "interrupted" || len(got.Error) == 0 {
		t.Fatalf("interrupted projection: %+v %v", got, err)
	}
	if err := s.SaveCommandExecution(ctx, command); !errors.Is(err, store.ErrCommandConflict) {
		t.Fatalf("old worker accepted: %v", err)
	}
}
func TestCommandDatabaseFailureDoesNotPublishPartialStatus(t *testing.T) {
	s, _ := checkpointFixture(t)
	ctx := context.Background()
	command, _, err := s.AcceptCommand(ctx, "t", model.CommandRequest{RequestKey: "start", Kind: "polish", Input: json.RawMessage(`{}`)}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.db.Exec(`CREATE TRIGGER fail_command BEFORE UPDATE ON command_executions BEGIN SELECT RAISE(ABORT,'failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	command.Status = "completed"
	command.Result = json.RawMessage(`{"content":"result"}`)
	if err := s.SaveCommandExecution(ctx, command); err == nil {
		t.Fatal("expected persistence failure")
	}
	got, err := s.GetCommand(ctx, command.CommandID)
	if err != nil || got.Status != "accepted" {
		t.Fatalf("partial status: %+v %v", got, err)
	}
	events, err := s.ThreadEvents(ctx, "t", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Type == "command.completed" {
			t.Fatal("published rolled-back terminal result")
		}
	}
}
