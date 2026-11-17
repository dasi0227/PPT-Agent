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
