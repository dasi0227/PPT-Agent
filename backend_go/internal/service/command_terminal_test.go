package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

type terminalFaultStore struct {
	CommandStore
	complete func(context.Context, model.CommandExecution, sessionstate.CommandEffects) (model.CommandExecution, error)
	save     func(context.Context, model.CommandExecution) error
}

func (s *terminalFaultStore) CompleteCommand(ctx context.Context, command model.CommandExecution, effects sessionstate.CommandEffects) (model.CommandExecution, error) {
	if s.complete != nil {
		return s.complete(ctx, command, effects)
	}
	return s.CommandStore.CompleteCommand(ctx, command, effects)
}

func (s *terminalFaultStore) SaveCommandExecution(ctx context.Context, command model.CommandExecution) error {
	if s.save != nil {
		return s.save(ctx, command)
	}
	return s.CommandStore.SaveCommandExecution(ctx, command)
}

func runTerminalCommand(t *testing.T, f serviceFixture, commandStore CommandStore, kind string, execute CommandExecutor) (*CommandService, model.CommandExecution) {
	t.Helper()
	accepted, _, err := f.store.AcceptCommand(context.Background(), f.thread.ID, model.CommandRequest{RequestKey: "terminal-fault", Kind: kind, Input: json.RawMessage(`{}`)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewCommandService(commandStore, execute)
	ctx, cancel := context.WithCancel(sessionstate.WithRequestScene(context.Background(), 1))
	worker := &commandWorker{execution: accepted.Command, cancel: cancel, done: make(chan struct{})}
	svc.workers[accepted.Command.CommandID] = worker
	svc.run(ctx, accepted.Command, worker)
	return svc, accepted.Command
}

func TestCommandTerminalStorageFailureRemainsVisibleDuringCancelAndShutdown(t *testing.T) {
	f := newServiceFixture(t)
	fault := &terminalFaultStore{CommandStore: f.store}
	writeFailure := errors.New("storage unavailable")
	fault.complete = func(context.Context, model.CommandExecution, sessionstate.CommandEffects) (model.CommandExecution, error) {
		return model.CommandExecution{}, writeFailure
	}
	fault.save = func(ctx context.Context, command model.CommandExecution) error {
		if !commandActive(command.Status) {
			return writeFailure
		}
		return f.store.SaveCommandExecution(ctx, command)
	}
	executions := 0
	svc, accepted := runTerminalCommand(t, f, fault, "polish", func(context.Context, model.CommandExecution) (any, error) {
		executions++
		return map[string]string{"content": "prepared once"}, nil
	})
	current, err := svc.Get(context.Background(), accepted.CommandID)
	var failure *model.AgentError
	if !errors.As(err, &failure) || failure.Code != model.CodePersistenceFailed || current.Status != "running" || executions != 1 {
		t.Fatalf("unconfirmed terminal was hidden: %+v %v executions=%d", current, err, executions)
	}
	if err := svc.CheckThreadPersistence(context.Background(), f.thread.ID, 1); !errors.As(err, &failure) || failure.Code != model.CodePersistenceFailed {
		t.Fatalf("reload hid persistence failure: %v", err)
	}
	if err := svc.CheckThreadPersistence(context.Background(), f.thread.ID, 2); err != nil {
		t.Fatalf("failure leaked into another scene: %v", err)
	}
	cancelCtx := sessionstate.WithEntityRevision(sessionstate.WithRequestScene(context.Background(), 1), current.EntityRevision)
	if _, err := svc.Cancel(cancelCtx, accepted.CommandID, accepted.AttemptID, "cancel-unconfirmed"); err != nil {
		t.Fatal(err)
	}
	current, err = svc.Get(context.Background(), accepted.CommandID)
	if !errors.As(err, &failure) || current.Status != "cancel_requested" {
		t.Fatalf("cancel released an unconfirmed result: %+v %v", current, err)
	}
	if err := svc.StopDeleted(context.Background(), f.project.ID, f.thread.ID); err != nil {
		t.Fatalf("completed executor blocked deletion: %v", err)
	}
	if err := svc.Shutdown(context.Background()); !errors.Is(err, writeFailure) {
		t.Fatalf("shutdown hid terminal write failure: %v", err)
	}
	// A later durable recovery result clears the local failure, rather than
	// keeping the command unreadable for the lifetime of the process.
	current.Status, current.Error = "interrupted", json.RawMessage(`{"code":"RECOVERED"}`)
	if err := f.store.SaveCommandExecution(context.Background(), current); err != nil {
		t.Fatal(err)
	}
	if current, err = svc.Get(context.Background(), accepted.CommandID); err != nil || current.Status != "interrupted" {
		t.Fatalf("recovery did not clear local failure: %+v %v", current, err)
	}
	if err := svc.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}
