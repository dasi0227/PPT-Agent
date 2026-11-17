package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

var commandIdentityPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_:-]{0,127}$`)

type CommandStore interface {
	CompleteCommand(context.Context, model.CommandExecution, sessionstate.CommandEffects) (model.CommandExecution, error)
	AcceptCommand(context.Context, string, model.CommandRequest, int64) (sessionstate.CommandMutation, bool, error)
	GetCommand(context.Context, string) (model.CommandExecution, error)
	SaveCommandExecution(context.Context, model.CommandExecution) error
	RequestCommandCancel(context.Context, string, string, string) (sessionstate.CommandMutation, error)
}
type CommandExecutor func(context.Context, model.CommandExecution) (any, error)
type commandWorker struct {
	execution   model.CommandExecution
	cancel      context.CancelFunc
	done        chan struct{}
	terminalErr error
}
type CommandService struct {
	store    CommandStore
	execute  CommandExecutor
	mu       sync.Mutex
	workers  map[string]*commandWorker
	stopping bool
}

func NewCommandService(s CommandStore, execute CommandExecutor) *CommandService {
	return &CommandService{store: s, execute: execute, workers: map[string]*commandWorker{}}
}
func (s *CommandService) Accept(ctx context.Context, threadID string, request model.CommandRequest, scene int64) (sessionstate.CommandMutation, error) {
	if !clientIdentityPattern.MatchString(request.RequestKey) || !json.Valid(request.Input) || (request.CommandID != "" && !commandIdentityPattern.MatchString(request.CommandID)) || (request.BaseAttemptID != "" && !clientIdentityPattern.MatchString(request.BaseAttemptID)) {
		return sessionstate.CommandMutation{}, model.NewAgentError("BAD_REQUEST", "command", nil)
	}
	var input map[string]json.RawMessage
	if json.Unmarshal(request.Input, &input) != nil || input == nil {
		return sessionstate.CommandMutation{}, model.NewAgentError("BAD_REQUEST", "command", nil)
	}
	if request.Kind == "rename" {
		var mode, title string
		if json.Unmarshal(input["mode"], &mode) != nil || (mode != "manual" && mode != "automatic") {
			return sessionstate.CommandMutation{}, model.NewAgentError("BAD_REQUEST", "command", nil)
		}
		if mode == "manual" {
			if json.Unmarshal(input["title"], &title) != nil {
				return sessionstate.CommandMutation{}, model.NewAgentError("BAD_REQUEST", "command", nil)
			}
			if _, err := ValidateThreadTitle(title); err != nil {
				return sessionstate.CommandMutation{}, model.NewAgentError("BAD_REQUEST", "command", err)
			}
		}
	}
	switch request.Kind {
	case "rename", "polish", "handoff", "compact", "commit", "suggest":
	default:
		return sessionstate.CommandMutation{}, model.NewAgentError("BAD_REQUEST", "command", nil)
	}
	// Keep acceptance and worker registration together so deletion cannot miss
	// a command accepted immediately before its target rows were removed.
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopping {
		return sessionstate.CommandMutation{}, run.ErrEngineStopping
	}
	mutation, created, err := s.store.AcceptCommand(ctx, threadID, request, scene)
	accepted := mutation.Command
	if err != nil {
		if created {
			s.interruptAccepted(ctx, accepted, "接受事件未完成投递，请恢复存储后重试")
		}
		return mutation, err
	}
	if !created {
		return mutation, nil
	}
	if err := run.CommitStartBarrier(ctx); err != nil {
		s.interruptAccepted(ctx, accepted, "项目保存失败，命令尚未开始")
		return mutation, err
	}
	workerCtx, cancel := context.WithCancel(sessionstate.WithRequestScene(context.Background(), accepted.SceneRevision))
	worker := &commandWorker{execution: accepted, cancel: cancel, done: make(chan struct{})}
	s.workers[accepted.CommandID] = worker
	go s.run(workerCtx, accepted, worker)
	return mutation, nil
}

// Shutdown stops independent commands (including manual compaction and Git)
// and waits for their terminal receipts before the database is closed.
func (s *CommandService) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.stopping = true
	finished := make([]*commandWorker, 0, len(s.workers))
	for _, worker := range s.workers {
		worker.cancel()
		finished = append(finished, worker)
	}
	s.mu.Unlock()
	var failures []error
	for _, worker := range finished {
		select {
		case <-worker.done:
			failures = append(failures, worker.terminalErr)
		case <-ctx.Done():
			return errors.Join(append(failures, ctx.Err())...)
		}
	}
	return errors.Join(failures...)
}

func (s *CommandService) StopDeleted(ctx context.Context, projectID, threadID string) error {
	s.mu.Lock()
	var done []<-chan struct{}
	for _, worker := range s.workers {
		if worker.execution.ProjectID == projectID && (threadID == "" || worker.execution.ThreadID == threadID) {
			worker.cancel()
			done = append(done, worker.done)
		}
	}
	s.mu.Unlock()
	for _, finished := range done {
		select {
		case <-finished:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
func (s *CommandService) run(ctx context.Context, execution model.CommandExecution, worker *commandWorker) {
	var terminalErr error
	defer func() {
		worker.cancel()
		s.mu.Lock()
		worker.terminalErr = terminalErr
		if terminalErr == nil && s.workers[execution.CommandID] == worker {
			delete(s.workers, execution.CommandID)
		}
		close(worker.done)
		s.mu.Unlock()
	}()
	var result any
	var err error
	func() {
		defer func() {
			if recover() != nil {
				err = errors.New("command execution panicked")
			}
		}()
		execution.Status = "running"
		if err = s.store.SaveCommandExecution(ctx, execution); err != nil {
			return
		}
		ctx = WithCommandProgress(ctx, func(phase int) error { execution.Phase = phase; return s.store.SaveCommandExecution(ctx, execution) })
		result, err = s.execute(llm.WithSubmissionIdentity(ctx, execution.CommandID, execution.AttemptID), execution)
	}()
	execution.Status = "completed"
	if err != nil {
		execution.Status = "failed"
		if execution.Kind == "suggest" && execution.Source == "automatic" && !errors.Is(err, context.Canceled) {
			slog.Warn("automatic suggestions failed", "thread_id", execution.ThreadID, "error", err)
		}
		if errors.Is(err, context.Canceled) {
			execution.Status = "canceled"
		}
		code, retryable := "COMMAND_FAILED", true
		var uncertain *uncertainCommitError
		if errors.As(err, &uncertain) {
			code, retryable = "COMMIT_UNCERTAIN", false
			execution.Status = "interrupted"
		}
		execution.Error, _ = json.Marshal(map[string]any{"code": code, "message": err.Error(), "retryable": retryable})
	} else {
		execution.Result, err = json.Marshal(result)
		if err != nil {
			execution.Status = "failed"
			execution.Error = json.RawMessage(`{"code":"INVALID_COMMAND_RESULT"}`)
		}
	}
	terminalCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	effects := sessionstate.CommandEffects{}
	var onCommitted func()
	if execution.Status == "completed" {
		switch prepared := result.(type) {
		case CompactContextResult:
			effects.Context, effects.Compaction = prepared.PendingContext, &prepared.Compaction
		case BriefingResult:
			effects.Briefing = prepared.PendingVersion
		case RenameResult:
			effects.Rename = prepared.PendingRename
			onCommitted = prepared.OnCommitted
		}
	}
	committed, saveErr := s.persistTerminal(terminalCtx, execution, effects)
	if commandFenceLost(saveErr) {
		return
	}
	if saveErr != nil && execution.Kind != "commit" {
		// The candidate was not committed. Record a visible recovery state without
		// applying any of its business effects or repeating the executor.
		interrupted := execution
		interrupted.Status, interrupted.Result = "interrupted", nil
		interrupted.Error = json.RawMessage(`{"code":"COMMAND_RESULT_UNCOMMITTED","retryable":true}`)
		recoveryCtx, recoveryCancel := context.WithTimeout(context.Background(), 5*time.Second)
		var recoveryErr error
		committed, recoveryErr = s.persistTerminal(recoveryCtx, interrupted, sessionstate.CommandEffects{})
		recoveryCancel()
		if commandFenceLost(recoveryErr) {
			return
		}
		if recoveryErr == nil {
			saveErr = nil
		} else {
			saveErr = errors.Join(saveErr, recoveryErr)
		}
	}
	if saveErr != nil {
		// In particular, keep Git active: its durable intent must be reconciled by
		// startup recovery before another command can perform an external effect.
		terminalErr = fmt.Errorf("command %s attempt %s terminal state is unconfirmed: %w", execution.CommandID, execution.AttemptID, saveErr)
		slog.Error("persist command terminal state", "command_id", execution.CommandID, "attempt_id", execution.AttemptID, "error", terminalErr)
		return
	}
	if committed.Status == "completed" && execution.Status == "completed" && bytes.Equal(committed.Result, execution.Result) && onCommitted != nil {
		onCommitted()
	}
}

// Only the durable write is retried. Even an unknown commit outcome must never
// send the command through the model or Git executor a second time.
func (s *CommandService) persistTerminal(ctx context.Context, execution model.CommandExecution, effects sessionstate.CommandEffects) (model.CommandExecution, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(time.Duration(attempt) * 100 * time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return model.CommandExecution{}, errors.Join(lastErr, ctx.Err())
			}
		}
		if execution.Status == "completed" {
			committed, err := s.store.CompleteCommand(ctx, execution, effects)
			if err == nil {
				return committed, nil
			}
			lastErr = err
		} else {
			lastErr = s.store.SaveCommandExecution(ctx, execution)
			if lastErr == nil {
				return execution, nil
			}
		}
		current, err := s.store.GetCommand(ctx, execution.CommandID)
		if err == nil {
			if !sameCommandExecution(current, execution) {
				return model.CommandExecution{}, store.ErrCommandConflict
			}
			if !commandActive(current.Status) {
				return current, nil
			}
		} else if commandFenceLost(err) {
			return model.CommandExecution{}, err
		}
		if commandFenceLost(lastErr) || ctx.Err() != nil {
			return model.CommandExecution{}, lastErr
		}
	}
	return model.CommandExecution{}, lastErr
}

func sameCommandExecution(left, right model.CommandExecution) bool {
	return left.CommandID == right.CommandID && left.AttemptID == right.AttemptID &&
		left.ProjectID == right.ProjectID && left.ThreadID == right.ThreadID &&
		left.SceneRevision == right.SceneRevision && left.OwnerInstanceID == right.OwnerInstanceID &&
		left.ExecutionRevision == right.ExecutionRevision
}

func commandActive(status string) bool {
	return status == "accepted" || status == "running" || status == "cancel_requested"
}

func commandFenceLost(err error) bool {
	return errors.Is(err, store.ErrCommandConflict) || errors.Is(err, sessionstate.ErrSceneChanged) || errors.Is(err, run.ErrRunNotFound)
}

func (s *CommandService) Get(ctx context.Context, id string) (model.CommandExecution, error) {
	execution, err := s.store.GetCommand(ctx, id)
	if err != nil {
		return execution, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if worker := s.workers[id]; worker != nil && worker.terminalErr != nil {
		if sameCommandExecution(execution, worker.execution) && commandActive(execution.Status) {
			failure := model.NewAgentError(model.CodePersistenceFailed, "command", worker.terminalErr)
			failure.SafeMessage = "命令结果未能保存，请恢复存储后重启服务确认结果。"
			failure.Reason = failure.SafeMessage
			return execution, failure
		}
		delete(s.workers, id)
	}
	return execution, nil
}

// Session reads also surface an unconfirmed terminal write after a page reload,
// when the original command request is no longer being observed.
func (s *CommandService) CheckThreadPersistence(ctx context.Context, threadID string, scene int64) error {
	s.mu.Lock()
	var ids []string
	for id, worker := range s.workers {
		if worker.terminalErr != nil && worker.execution.ThreadID == threadID && worker.execution.SceneRevision == scene {
			ids = append(ids, id)
		}
	}
	s.mu.Unlock()
	for _, id := range ids {
		if _, err := s.Get(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
func (s *CommandService) Cancel(ctx context.Context, id, attempt, key string) (sessionstate.CommandMutation, error) {
	if !clientIdentityPattern.MatchString(key) {
		return sessionstate.CommandMutation{}, model.NewAgentError("BAD_REQUEST", "cancel_command", nil)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	execution, err := s.store.RequestCommandCancel(ctx, id, attempt, key)
	if err != nil {
		return execution, err
	}
	if worker := s.workers[id]; worker != nil && worker.execution.AttemptID == attempt {
		worker.cancel()
	}
	return execution, nil
}

func (s *CommandService) interruptAccepted(ctx context.Context, accepted model.CommandExecution, message string) {
	accepted.Status = "interrupted"
	accepted.Error, _ = json.Marshal(map[string]any{"code": "COMMAND_NOT_STARTED", "message": message, "retryable": true})
	recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if _, err := s.persistTerminal(recoveryCtx, accepted, sessionstate.CommandEffects{}); err != nil && !commandFenceLost(err) {
		// Accept holds mu. There is no executor to wait for, but failed writes
		// still need the same read-side error and shutdown accounting as workers.
		worker := &commandWorker{execution: accepted, cancel: func() {}, done: make(chan struct{}), terminalErr: err}
		close(worker.done)
		s.workers[accepted.CommandID] = worker
		slog.Error("persist accepted command interruption", "command_id", accepted.CommandID, "error", err)
	}
}
