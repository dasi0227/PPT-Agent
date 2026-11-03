package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

type terminalFaultStore struct {
	CommandStore
	complete func(context.Context, model.CommandExecution, sessionstate.CommandEffects) (model.CommandExecution, error)
	save     func(context.Context, model.CommandExecution) error
	get      func(context.Context, string) (model.CommandExecution, error)
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

func (s *terminalFaultStore) GetCommand(ctx context.Context, id string) (model.CommandExecution, error) {
	if s.get != nil {
		return s.get(ctx, id)
	}
	return s.CommandStore.GetCommand(ctx, id)
}

func runTerminalCommand(t *testing.T, f briefingFixture, commandStore CommandStore, kind string, execute CommandExecutor) (*CommandService, model.CommandExecution) {
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

func TestCommandTerminalRetriesCandidateWithoutExecutingAgain(t *testing.T) {
	for _, mode := range []string{"write_failed", "response_lost"} {
		t.Run(mode, func(t *testing.T) {
			f := newBriefingFixture(t)
			thread, err := f.store.GetThread(context.Background(), f.thread.ID)
			if err != nil {
				t.Fatal(err)
			}
			fault := &terminalFaultStore{CommandStore: f.store}
			writes, executions, notifications := 0, 0, 0
			fault.complete = func(ctx context.Context, command model.CommandExecution, effects sessionstate.CommandEffects) (model.CommandExecution, error) {
				writes++
				if mode == "write_failed" && writes < 3 {
					return model.CommandExecution{}, errors.New("temporary write failure")
				}
				committed, err := f.store.CompleteCommand(ctx, command, effects)
				if mode == "response_lost" && err == nil {
					return model.CommandExecution{}, errors.New("commit response lost")
				}
				return committed, err
			}
			svc, accepted := runTerminalCommand(t, f, fault, "rename", func(context.Context, model.CommandExecution) (any, error) {
				executions++
				return RenameResult{Title: "确认后的标题", PendingRename: &sessionstate.RenameEffect{Title: "确认后的标题", OperationVersion: thread.RenameOperationVersion}, OnCommitted: func() { notifications++ }}, nil
			})
			got, err := svc.Get(context.Background(), accepted.CommandID)
			if err != nil || got.Status != "completed" || executions != 1 || notifications != 1 {
				t.Fatalf("command=%+v executions=%d notifications=%d error=%v", got, executions, notifications, err)
			}
			wantWrites := map[string]int{"write_failed": 3, "response_lost": 1}[mode]
			if writes != wantWrites {
				t.Fatalf("terminal writes=%d, want %d", writes, wantWrites)
			}
			thread, err = f.store.GetThread(context.Background(), f.thread.ID)
			if err != nil || thread.Title != "确认后的标题" {
				t.Fatalf("candidate not committed: %+v %v", thread, err)
			}
		})
	}
}

func TestCommandTerminalFailureInterruptsWithoutApplyingCandidate(t *testing.T) {
	f := newBriefingFixture(t)
	thread, err := f.store.GetThread(context.Background(), f.thread.ID)
	if err != nil {
		t.Fatal(err)
	}
	fault := &terminalFaultStore{CommandStore: f.store}
	writes, executions, notifications := 0, 0, 0
	fault.complete = func(context.Context, model.CommandExecution, sessionstate.CommandEffects) (model.CommandExecution, error) {
		writes++
		return model.CommandExecution{}, errors.New("candidate transaction failed")
	}
	svc, accepted := runTerminalCommand(t, f, fault, "rename", func(context.Context, model.CommandExecution) (any, error) {
		executions++
		return RenameResult{Title: "不得应用", PendingRename: &sessionstate.RenameEffect{Title: "不得应用", OperationVersion: thread.RenameOperationVersion}, OnCommitted: func() { notifications++ }}, nil
	})
	got, err := svc.Get(context.Background(), accepted.CommandID)
	if err != nil || got.Status != "interrupted" || executions != 1 || writes != 3 || notifications != 0 || len(got.Result) != 0 {
		t.Fatalf("command=%+v writes=%d executions=%d notifications=%d error=%v", got, writes, executions, notifications, err)
	}
	var failure struct{ Code string }
	if json.Unmarshal(got.Error, &failure) != nil || failure.Code != "COMMAND_RESULT_UNCOMMITTED" {
		t.Fatalf("missing recovery reason: %s", got.Error)
	}
	current, err := f.store.GetThread(context.Background(), f.thread.ID)
	if err != nil || current.Title != thread.Title {
		t.Fatalf("uncommitted candidate changed title: %+v %v", current, err)
	}
	if err := svc.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCommandStartBarrierFailureRemainsVisibleWhenInterruptionCannotPersist(t *testing.T) {
	f := newBriefingFixture(t)
	writeFailure := errors.New("interruption storage unavailable")
	fault := &terminalFaultStore{CommandStore: f.store, save: func(context.Context, model.CommandExecution) error {
		return writeFailure
	}}
	executions := 0
	svc := NewCommandService(fault, func(context.Context, model.CommandExecution) (any, error) {
		executions++
		return nil, nil
	})
	barrierFailure := errors.New("project barrier failed")
	ctx := run.WithStartBarrier(context.Background(), func() error { return barrierFailure })
	request := model.CommandRequest{RequestKey: "failed-start-barrier", Kind: "polish", Input: json.RawMessage(`{}`)}
	accepted, err := svc.Accept(ctx, f.thread.ID, request, 1)
	if !errors.Is(err, barrierFailure) || executions != 0 {
		t.Fatalf("barrier failure executed command: %+v %v executions=%d", accepted, err, executions)
	}
	var failure *model.AgentError
	current, err := svc.Get(context.Background(), accepted.Command.CommandID)
	if !errors.As(err, &failure) || failure.Code != model.CodePersistenceFailed || current.Status != "accepted" {
		t.Fatalf("failed interruption was hidden: %+v %v", current, err)
	}
	if err := svc.CheckThreadPersistence(context.Background(), f.thread.ID, 1); !errors.As(err, &failure) || failure.Code != model.CodePersistenceFailed {
		t.Fatalf("reload hid failed acceptance interruption: %v", err)
	}
	replayed, err := svc.Accept(context.Background(), f.thread.ID, request, 1)
	if err != nil || replayed.Command.AttemptID != accepted.Command.AttemptID || executions != 0 {
		t.Fatalf("acceptance retry executed again: %+v %v executions=%d", replayed, err, executions)
	}
	if err := svc.Shutdown(context.Background()); !errors.Is(err, writeFailure) {
		t.Fatalf("shutdown hid failed interruption: %v", err)
	}
}

func TestCommandTerminalStorageFailureRemainsVisibleDuringCancelAndShutdown(t *testing.T) {
	f := newBriefingFixture(t)
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

func TestGitTerminalStorageFailureKeepsIntentBlockedUntilReconciled(t *testing.T) {
	f := newBriefingFixture(t)
	f.provider.Caps = llm.Capabilities{ToolCalls: true}
	f.provider.Script = []llm.GenerateResponse{{ToolCalls: []llm.ToolCall{{ID: "commit-once", Name: "git_commit", Args: map[string]any{"title": "feat: 保存内容", "items": []any{"保存演示页面"}}}}}}
	git := NewGitCommitService(f.store, f.registry, f.locks)
	fault := &terminalFaultStore{CommandStore: f.store, complete: func(context.Context, model.CommandExecution, sessionstate.CommandEffects) (model.CommandExecution, error) {
		return model.CommandExecution{}, errors.New("terminal storage unavailable")
	}}
	svc, accepted := runTerminalCommand(t, f, fault, "commit", git.ExecuteCommand)
	current, err := svc.Get(context.Background(), accepted.CommandID)
	var failure *model.AgentError
	if !errors.As(err, &failure) || failure.Code != model.CodePersistenceFailed || current.Status != "running" || len(current.Operation) == 0 || len(f.provider.Requests()) != 1 {
		t.Fatalf("Git outcome lost: %+v %v requests=%d", current, err, len(f.provider.Requests()))
	}
	if _, _, err := f.store.AcceptCommand(context.Background(), f.thread.ID, model.CommandRequest{RequestKey: "duplicate-effect", Kind: "commit", Input: json.RawMessage(`{}`)}, 1); err == nil {
		t.Fatalf("unconfirmed Git effect released project: %v", err)
	}
	if blocked, err := f.store.HasActiveGitCommit(context.Background(), f.project.ID); err != nil || !blocked {
		t.Fatalf("unconfirmed Git intent stopped blocking: blocked=%v error=%v", blocked, err)
	}
	if err := git.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	current, err = svc.Get(context.Background(), accepted.CommandID)
	if err != nil || current.Status != "completed" || len(f.provider.Requests()) != 1 {
		t.Fatalf("recovery repeated Git generation: %+v %v requests=%d", current, err, len(f.provider.Requests()))
	}
}

func TestCommandTerminalReconciliationRejectsOtherExecution(t *testing.T) {
	for name, change := range map[string]func(*model.CommandExecution){
		"attempt":    func(command *model.CommandExecution) { command.AttemptID = "other-attempt" },
		"scene":      func(command *model.CommandExecution) { command.SceneRevision++ },
		"owner":      func(command *model.CommandExecution) { command.OwnerInstanceID = "other-owner" },
		"generation": func(command *model.CommandExecution) { command.ExecutionRevision++ },
	} {
		t.Run(name, func(t *testing.T) {
			f := newBriefingFixture(t)
			fault := &terminalFaultStore{CommandStore: f.store}
			writes, terminalWrites := 0, 0
			fault.complete = func(context.Context, model.CommandExecution, sessionstate.CommandEffects) (model.CommandExecution, error) {
				writes++
				return model.CommandExecution{}, errors.New("response outcome unknown")
			}
			fault.save = func(ctx context.Context, command model.CommandExecution) error {
				if !commandActive(command.Status) {
					terminalWrites++
				}
				return f.store.SaveCommandExecution(ctx, command)
			}
			fault.get = func(ctx context.Context, id string) (model.CommandExecution, error) {
				current, err := f.store.GetCommand(ctx, id)
				change(&current)
				current.Status = "completed"
				return current, err
			}
			svc, _ := runTerminalCommand(t, f, fault, "polish", func(context.Context, model.CommandExecution) (any, error) {
				return map[string]string{"content": "once"}, nil
			})
			if writes != 1 || terminalWrites != 0 || len(svc.workers) != 0 {
				t.Fatalf("stale execution retried or overwrote recovery: writes=%d terminalWrites=%d workers=%d", writes, terminalWrites, len(svc.workers))
			}
		})
	}
}
