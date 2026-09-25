package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

var commandIdentityPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_:-]{0,127}$`)

type CommandStore interface {
	AcceptCommand(context.Context, string, model.CommandRequest, int64) (model.CommandExecution, bool, error)
	GetCommand(context.Context, string) (model.CommandExecution, error)
	SaveCommandExecution(context.Context, model.CommandExecution) error
	RequestCommandCancel(context.Context, string, string, string) (model.CommandExecution, error)
}
type CommandExecutor func(context.Context, model.CommandExecution) (any, error)
type commandWorker struct {
	attempt string
	cancel  context.CancelFunc
	done    chan struct{}
}
type CommandService struct {
	store   CommandStore
	execute CommandExecutor
	mu      sync.Mutex
	workers map[string]*commandWorker
}

func NewCommandService(s CommandStore, execute CommandExecutor) *CommandService {
	return &CommandService{store: s, execute: execute, workers: map[string]*commandWorker{}}
}
func (s *CommandService) Accept(ctx context.Context, threadID string, request model.CommandRequest, scene int64) (model.CommandExecution, error) {
	if !clientIdentityPattern.MatchString(request.RequestKey) || !json.Valid(request.Input) || (request.CommandID != "" && !commandIdentityPattern.MatchString(request.CommandID)) || (request.BaseAttemptID != "" && !clientIdentityPattern.MatchString(request.BaseAttemptID)) {
		return model.CommandExecution{}, model.NewAgentError("BAD_REQUEST", "command", nil)
	}
	var input map[string]json.RawMessage
	if json.Unmarshal(request.Input, &input) != nil || input == nil {
		return model.CommandExecution{}, model.NewAgentError("BAD_REQUEST", "command", nil)
	}
	if request.Kind == "rename" {
		var mode, title string
		if json.Unmarshal(input["mode"], &mode) != nil || (mode != "manual" && mode != "automatic") {
			return model.CommandExecution{}, model.NewAgentError("BAD_REQUEST", "command", nil)
		}
		if mode == "manual" {
			if json.Unmarshal(input["title"], &title) != nil {
				return model.CommandExecution{}, model.NewAgentError("BAD_REQUEST", "command", nil)
			}
			if _, err := ValidateThreadTitle(title); err != nil {
				return model.CommandExecution{}, model.NewAgentError("BAD_REQUEST", "command", err)
			}
		}
	}
	switch request.Kind {
	case "rename", "polish", "kickoff", "handoff", "compact", "commit":
	default:
		return model.CommandExecution{}, model.NewAgentError("BAD_REQUEST", "command", nil)
	}
	accepted, created, err := s.store.AcceptCommand(ctx, threadID, request, scene)
	if err != nil {
		if created {
			s.interruptAccepted(ctx, accepted, "接受事件未完成投递，请恢复存储后重试")
		}
		return accepted, err
	}
	if !created {
		return accepted, nil
	}
	if err := run.CommitStartBarrier(ctx); err != nil {
		s.interruptAccepted(ctx, accepted, "项目保存失败，命令尚未开始")
		return accepted, err
	}
	workerCtx, cancel := context.WithCancel(context.Background())
	worker := &commandWorker{attempt: accepted.AttemptID, cancel: cancel, done: make(chan struct{})}
	s.mu.Lock()
	s.workers[accepted.CommandID] = worker
	s.mu.Unlock()
	go s.run(workerCtx, accepted, worker)
	return accepted, nil
}
func (s *CommandService) run(ctx context.Context, execution model.CommandExecution, worker *commandWorker) {
	defer func() {
		worker.cancel()
		close(worker.done)
		s.mu.Lock()
		if s.workers[execution.CommandID] == worker {
			delete(s.workers, execution.CommandID)
		}
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
		result, err = s.execute(ctx, execution)
	}()
	execution.Status = "completed"
	if err != nil {
		execution.Status = "failed"
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
	// An undelivered terminal event remains in the outbox for recovery.
	if saveErr := s.store.SaveCommandExecution(terminalCtx, execution); saveErr != nil && !errors.Is(saveErr, store.ErrCommandConflict) {
		slog.Error("persist command terminal state", "command_id", execution.CommandID, "attempt_id", execution.AttemptID, "error", saveErr)
	}
}
func (s *CommandService) Get(ctx context.Context, id string) (model.CommandExecution, error) {
	return s.store.GetCommand(ctx, id)
}
func (s *CommandService) Cancel(ctx context.Context, id, attempt, key string) (model.CommandExecution, error) {
	if !clientIdentityPattern.MatchString(key) {
		return model.CommandExecution{}, model.NewAgentError("BAD_REQUEST", "cancel_command", nil)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	execution, err := s.store.RequestCommandCancel(ctx, id, attempt, key)
	if err != nil {
		return execution, err
	}
	if worker := s.workers[id]; worker != nil && worker.attempt == attempt {
		worker.cancel()
	}
	return execution, nil
}

func (s *CommandService) interruptAccepted(ctx context.Context, accepted model.CommandExecution, message string) {
	accepted.Status = "interrupted"
	accepted.Error, _ = json.Marshal(map[string]any{"code": "COMMAND_NOT_STARTED", "message": message, "retryable": true})
	recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.store.SaveCommandExecution(recoveryCtx, accepted); err != nil {
		slog.Error("persist accepted command interruption", "command_id", accepted.CommandID, "error", err)
	}
}
