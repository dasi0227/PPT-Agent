package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/decision"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/tooldefinition"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	renameRequestTimeout = 20 * time.Second
	renameToolName       = "rename_thread"
)

type RenameTrigger string

const (
	RenameTriggerFirstInput RenameTrigger = "first_input"
	RenameTriggerNewInput   RenameTrigger = "new_input"
	RenameTriggerManual     RenameTrigger = "manual"
)

type renameTask struct {
	input             model.ConversationInput
	threadID          string
	projectID         string
	requestID         string
	operationVersion  int64
	trigger           RenameTrigger
	projectGeneration string
}

type pendingRename struct {
	input             model.ConversationInput
	trigger           RenameTrigger
	projectID         string
	projectGeneration string
}

type renameWorker struct {
	projectID string
	done      chan struct{}
	running   bool
	cancel    context.CancelFunc
	pending   *pendingRename
}

type directRename struct {
	cancel    context.CancelFunc
	projectID string
	done      chan struct{}
}

type NamingService struct {
	decisions          decision.Source
	store              store.Store
	provider           llm.Provider
	hub                *ThreadEventHub
	log                *zap.Logger
	rootCtx            context.Context
	cancel             context.CancelFunc
	mu                 sync.Mutex
	workers            map[string]*renameWorker
	latestInputs       map[string]pendingRename
	direct             map[string]*directRename
	projectGenerations map[string]string
	projectLocks       map[string]*sync.RWMutex
}

func NewNamingService(s store.Store, provider llm.Provider, hub *ThreadEventHub, log *zap.Logger) *NamingService {
	rootCtx, cancel := context.WithCancel(context.Background())
	return &NamingService{store: s, provider: provider, hub: hub, log: log.Named("thread-naming"), rootCtx: rootCtx, cancel: cancel, workers: map[string]*renameWorker{}, latestInputs: map[string]pendingRename{}, direct: map[string]*directRename{}, projectGenerations: map[string]string{}, projectLocks: map[string]*sync.RWMutex{}}
}

func (svc *NamingService) Close()                  { svc.cancel() }
func (svc *NamingService) Events() *ThreadEventHub { return svc.hub }

type RenameResult struct {
	Title         string                     `json:"title"`
	PendingRename *sessionstate.RenameEffect `json:"-"`
	OnCommitted   func()                     `json:"-"`
}

func (svc *NamingService) renameResult(thread model.Thread, title string, enabled *bool) RenameResult {
	return RenameResult{Title: title, PendingRename: &sessionstate.RenameEffect{Title: title, OperationVersion: thread.RenameOperationVersion, Enabled: enabled}, OnCommitted: func() {
		if current, err := svc.store.GetThread(context.Background(), thread.ID); err == nil {
			svc.hub.PublishUpdated(current)
		}
	}}
}

// ManualRename prepares the title; CompleteCommand owns the title and terminal write.
func (svc *NamingService) ManualRename(ctx context.Context, threadID, rawTitle string) (RenameResult, error) {
	title, err := ValidateThreadTitle(rawTitle)
	if err != nil {
		return RenameResult{}, err
	}
	thread, err := svc.store.UpdateThreadNamingState(ctx, threadID, nil, nil, true, time.Now().UnixMilli())
	if err != nil {
		return RenameResult{}, err
	}
	svc.invalidateWorker(threadID)
	enabled := false
	return svc.renameResult(thread, title, &enabled), nil
}

func ValidateThreadTitle(value string) (string, error) {
	title := strings.TrimSpace(value)
	if title == "" || utf8.RuneCountInString(title) > 60 {
		return "", errors.New("title length must be 1..60")
	}
	if strings.ContainsAny(title, "\r\n\t") {
		return "", errors.New("title must be single-line plain text")
	}
	for _, char := range title {
		if unicode.IsControl(char) {
			return "", errors.New("title must not contain control characters")
		}
	}
	return title, nil
}

func validateGeneratedThreadTitle(value string) (string, error) {
	title, err := ValidateThreadTitle(value)
	if err != nil {
		return "", err
	}
	trimmedLeft := strings.TrimLeftFunc(title, unicode.IsSpace)
	for _, prefix := range []string{"#", "- ", "* ", "+ ", "> ", "```"} {
		if strings.HasPrefix(trimmedLeft, prefix) {
			return "", errors.New("generated title must not contain Markdown formatting")
		}
	}
	if strings.Contains(title, "`") || strings.Contains(title, "**") || strings.Contains(title, "__") {
		return "", errors.New("generated title must not contain Markdown formatting")
	}
	if strings.HasSuffix(title, ".") || strings.HasSuffix(title, "。") {
		return "", errors.New("generated title must not end with a period")
	}
	pairs := [][2]string{{"\"", "\""}, {"'", "'"}, {"“", "”"}, {"‘", "’"}, {"《", "》"}}
	for _, pair := range pairs {
		if strings.HasPrefix(title, pair[0]) && strings.HasSuffix(title, pair[1]) {
			return "", errors.New("generated title must not be wrapped in quotes")
		}
	}
	return title, nil
}

func (svc *NamingService) RecordInput(current model.Run, inputID string) {
	if svc == nil || strings.TrimSpace(inputID) == "" {
		return
	}
	generation := svc.projectGeneration(current.ProjectID)
	// Registration, counters and command acceptance all belong to the naming
	// worker. Neither the input classifier nor HTTP acceptance waits for them.
	go svc.recordInput(current, inputID, generation)
}

func (svc *NamingService) recordInput(current model.Run, inputID, generation string) {
	ctx, cancel := context.WithTimeout(sessionstate.WithRequestScene(svc.rootCtx, current.SceneRevision), 3*time.Second)
	defer cancel()
	lock := svc.projectLock(current.ProjectID)
	lock.RLock()
	if svc.projectGeneration(current.ProjectID) != generation {
		lock.RUnlock()
		return
	}
	source, err := svc.store.LoadThreadConversationContextBeforeInput(ctx, current.ThreadID, current.ID, inputID)
	if err != nil || source.Input == nil {
		lock.RUnlock()
		return
	}
	thread, trigger, err := svc.store.RecordThreadNamingInput(ctx, model.ThreadNamingInput{
		SceneRevision: current.SceneRevision, ThreadID: current.ThreadID, RunID: current.ID, InputID: inputID, ExecutionRevision: current.ExecutionRevision,
	})
	lock.RUnlock()
	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, run.ErrRunNotFound) && !errors.Is(err, run.ErrRunRevisionConflict) {
			svc.log.Warn("record naming input failed", zap.String("thread_id", current.ThreadID), zap.Error(err))
		}
		return
	}
	if !trigger {
		return
	}
	kind := RenameTriggerNewInput
	if thread.RenameInputCount == 0 {
		kind = RenameTriggerFirstInput
	}
	svc.enqueueAutomatic(thread.ID, thread.ProjectID, *source.Input, kind, generation)
}

// SetAutomatic changes current naming settings; generated/manual names use commands.
func (svc *NamingService) SetAutomatic(ctx context.Context, threadID string, enabled bool) (model.Thread, error) {
	thread, err := svc.store.UpdateThreadNamingState(ctx, threadID, nil, &enabled, true, time.Now().UnixMilli())
	if err != nil {
		return thread, err
	}
	svc.invalidateWorker(threadID)
	svc.hub.PublishUpdated(thread)
	return thread, nil
}

func (svc *NamingService) enqueueAutomatic(threadID, projectID string, input model.ConversationInput, trigger RenameTrigger, generation string) {
	svc.mu.Lock()
	if svc.projectGenerations[projectID] != generation {
		svc.mu.Unlock()
		return
	}
	if svc.direct[threadID] != nil {
		svc.mu.Unlock()
		return
	}
	if latest, ok := svc.latestInputs[threadID]; ok && latest.projectGeneration == generation && latest.input.AcceptedSeq >= input.AcceptedSeq {
		svc.mu.Unlock()
		return
	}
	svc.latestInputs[threadID] = pendingRename{input: input, trigger: trigger, projectID: projectID, projectGeneration: generation}
	worker := svc.workers[threadID]
	if worker == nil {
		worker = &renameWorker{projectID: projectID, done: make(chan struct{})}
		svc.workers[threadID] = worker
	}
	if worker.running {
		worker.pending = &pendingRename{input: input, trigger: trigger, projectID: projectID, projectGeneration: generation}
		if worker.cancel != nil {
			worker.cancel()
		}
		svc.mu.Unlock()
		return
	}
	worker.running = true
	svc.mu.Unlock()
	svc.launchAutomatic(threadID, projectID, input, trigger, generation)
}

func (svc *NamingService) launchAutomatic(threadID, projectID string, input model.ConversationInput, trigger RenameTrigger, generation string) {
	// Finish a rejected worker only after releasing the project read lock:
	// draining its pending task may acquire that lock again while rollback waits.
	started := func() bool {
		projectLock := svc.projectLock(projectID)
		projectLock.RLock()
		defer projectLock.RUnlock()
		if svc.projectGeneration(projectID) != generation {
			return false
		}
		if !svc.renameInputCurrent(threadID, generation, input) {
			return false
		}
		thread, err := svc.store.BeginThreadRenameRequest(svc.rootCtx, threadID, 0, true, time.Now().UnixMilli())
		if err != nil {
			return false
		}
		return svc.launchTask(renameTask{
			input: input, threadID: thread.ID, projectID: thread.ProjectID, requestID: model.MustShortID("rename"),
			operationVersion: thread.RenameOperationVersion, trigger: trigger, projectGeneration: generation,
		})
	}()
	if !started {
		svc.finishWorker(threadID)
	}
}

func (svc *NamingService) launchTask(task renameTask) bool {
	requestCtx, cancel := context.WithCancel(svc.rootCtx)
	input, _ := json.Marshal(map[string]any{"mode": "automatic", "trigger": task.trigger})
	scene, err := svc.store.ThreadSceneRevision(requestCtx, task.threadID)
	if err != nil {
		cancel()
		return false
	}
	mutation, created, err := svc.store.AcceptCommand(requestCtx, task.threadID, model.CommandRequest{RunID: task.input.RunID, RequestKey: task.requestID, Kind: "rename", Input: input, Source: "automatic"}, scene)
	accepted := mutation.Command
	if err != nil || !created {
		if created {
			svc.interruptAutomatic(requestCtx, accepted)
		}
		cancel()
		return false
	}
	accepted.Status = "running"
	if err := svc.store.SaveCommandExecution(requestCtx, accepted); err != nil {
		svc.interruptAutomatic(requestCtx, accepted)
		cancel()
		return false
	}
	requestCtx = sessionstate.WithRequestScene(requestCtx, accepted.SceneRevision)
	requestCtx = WithCommandProgress(requestCtx, func(phase int) error {
		accepted.Phase = phase
		return svc.saveAutomaticCommand(requestCtx, accepted)
	})
	svc.mu.Lock()
	if latest, ok := svc.latestInputs[task.threadID]; ok && latest.projectGeneration == task.projectGeneration && latest.input.AcceptedSeq > task.input.AcceptedSeq {
		svc.mu.Unlock()
		cancel()
		svc.interruptAutomatic(context.WithoutCancel(requestCtx), accepted)
		return false
	}
	if worker := svc.workers[task.threadID]; worker != nil {
		worker.cancel = cancel
	}
	svc.mu.Unlock()
	go func() {
		defer cancel()
		title, err := svc.runTask(requestCtx, task)
		accepted.Status = "completed"
		if err != nil {
			accepted.Status = "failed"
			if errors.Is(err, context.Canceled) {
				accepted.Status = "canceled"
			}
			accepted.Error, _ = json.Marshal(map[string]any{"code": "RENAME_FAILED", "message": err.Error()})
		} else {
			accepted.Result, _ = json.Marshal(map[string]any{"title": title})
		}
		writeCtx, writeCancel := context.WithTimeout(context.WithoutCancel(requestCtx), 5*time.Second)
		defer writeCancel()
		var saveErr error
		if accepted.Status == "completed" {
			lock := svc.projectLock(task.projectID)
			lock.RLock()
			completed, err := svc.store.CompleteCommand(writeCtx, accepted, sessionstate.CommandEffects{Rename: &sessionstate.RenameEffect{Title: title, OperationVersion: task.operationVersion, Automatic: true}})
			saveErr = err
			if err == nil && completed.Status == "completed" {
				if thread, err := svc.store.GetThread(writeCtx, task.threadID); err == nil {
					svc.hub.PublishUpdated(thread)
				}
			}
			lock.RUnlock()
		} else {
			saveErr = svc.saveAutomaticCommand(writeCtx, accepted)
		}
		if saveErr != nil && !errors.Is(saveErr, store.ErrCommandConflict) && !errors.Is(saveErr, run.ErrRunNotFound) {
			svc.log.Warn("persist automatic naming result", zap.Error(saveErr))
		}
		svc.finishWorker(task.threadID)
	}()
	return true
}

func (svc *NamingService) saveAutomaticCommand(ctx context.Context, execution model.CommandExecution) error {
	lock := svc.projectLock(execution.ProjectID)
	lock.RLock()
	defer lock.RUnlock()
	return svc.store.SaveCommandExecution(ctx, execution)
}

func (svc *NamingService) interruptAutomatic(ctx context.Context, execution model.CommandExecution) {
	execution.Status = "interrupted"
	execution.Error = json.RawMessage(`{"code":"COMMAND_NOT_STARTED","retryable":true}`)
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := svc.store.SaveCommandExecution(writeCtx, execution); err != nil {
		svc.log.Warn("persist automatic naming interruption", zap.Error(err))
	}
}

func (svc *NamingService) finishWorker(threadID string) {
	svc.mu.Lock()
	worker := svc.workers[threadID]
	if worker == nil {
		svc.mu.Unlock()
		return
	}
	worker.cancel = nil
	pending := worker.pending
	worker.pending = nil
	if pending == nil {
		worker.running = false
		delete(svc.workers, threadID)
		close(worker.done)
		svc.mu.Unlock()
		return
	}
	svc.mu.Unlock()
	svc.launchAutomatic(threadID, pending.projectID, pending.input, pending.trigger, pending.projectGeneration)
}

func (svc *NamingService) invalidateWorker(threadID string) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if command := svc.direct[threadID]; command != nil {
		command.cancel()
	}
	if worker := svc.workers[threadID]; worker != nil {
		if worker.cancel != nil {
			worker.cancel()
		}
		worker.pending = nil
	}
}

func (svc *NamingService) CancelProject(ctx context.Context, projectID string) {
	svc.bumpProjectGeneration(projectID)
	threads, _ := svc.store.ListThreads(ctx, projectID)
	for _, thread := range threads {
		svc.invalidateWorker(thread.ID)
	}
}

func (svc *NamingService) StopDeleted(ctx context.Context, projectID, threadID string) error {
	// Task registration also holds this project lock. After deletion, a later
	// registration fails its database lookup instead of escaping cancellation.
	lock := svc.projectLock(projectID)
	lock.Lock()
	svc.mu.Lock()
	var done []<-chan struct{}
	for id, latest := range svc.latestInputs {
		if latest.projectID == projectID && (threadID == "" || id == threadID) {
			delete(svc.latestInputs, id)
		}
	}
	for id, worker := range svc.workers {
		if worker.projectID == projectID && (threadID == "" || id == threadID) {
			worker.pending = nil
			if worker.cancel != nil {
				worker.cancel()
			}
			done = append(done, worker.done)
		}
	}
	for id, worker := range svc.direct {
		if worker.projectID == projectID && (threadID == "" || id == threadID) {
			worker.cancel()
			done = append(done, worker.done)
		}
	}
	svc.mu.Unlock()
	lock.Unlock()
	for _, finished := range done {
		select {
		case <-finished:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (svc *NamingService) BeginProjectReset(ctx context.Context, projectID string) func() {
	lock := svc.projectLock(projectID)
	lock.Lock()
	svc.CancelProject(ctx, projectID)
	return lock.Unlock
}

func (svc *NamingService) projectGeneration(projectID string) string {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	generation := svc.projectGenerations[projectID]
	if generation == "" {
		generation = uuid.NewString()
		svc.projectGenerations[projectID] = generation
	}
	return generation
}

func (svc *NamingService) bumpProjectGeneration(projectID string) {
	svc.mu.Lock()
	svc.projectGenerations[projectID] = uuid.NewString()
	svc.mu.Unlock()
}

func (svc *NamingService) projectLock(projectID string) *sync.RWMutex {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	lock := svc.projectLocks[projectID]
	if lock == nil {
		lock = &sync.RWMutex{}
		svc.projectLocks[projectID] = lock
	}
	return lock
}

func (svc *NamingService) runTask(parent context.Context, task renameTask) (string, error) {
	ctx, cancel := context.WithTimeout(parent, renameRequestTimeout)
	defer cancel()
	started := time.Now()
	stage := "input_preparation"
	outcome := "kept"
	preparedTitle := ""
	provider := svc.provider
	var captureErr error
	if factory, ok := provider.(interface{ Capture() (llm.Provider, error) }); ok {
		provider, captureErr = factory.Capture()
	}
	defer func() {
		execution := llm.ModelExecution{}
		if provider != nil {
			execution = llm.ExecutionOf(provider)
		}
		termination := outcome
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			termination = "timed_out"
		}
		svc.log.Info("rename request completed", zap.String("thread_id", task.threadID), zap.String("request_id", task.requestID), zap.String("outcome", outcome), zap.String("termination", termination), zap.String("stage", stage), zap.Int64("elapsed_ms", time.Since(started).Milliseconds()), zap.Any("execution", execution))
	}()
	var contextValue renameInput
	err := commandPhase(ctx, 0)
	if err == nil {
		contextValue, err = svc.renameContext(ctx, task.threadID)
	}
	if captureErr != nil {
		err = captureErr
	}
	if err == nil && task.trigger != RenameTriggerManual {
		stage = "naming_decision"
		proceed, gateErr := svc.shouldRename(ctx, task, contextValue)
		if gateErr != nil {
			outcome = "failed"
			if errors.Is(gateErr, context.Canceled) {
				outcome = "superseded"
			}
			return "", gateErr
		}
		if !proceed {
			return contextValue.CurrentTitle, nil
		}
	}
	if err == nil {
		err = svc.renameTaskCurrent(ctx, task)
	}
	if err == nil {
		var action, title string
		stage = "submission"
		err = commandPhase(ctx, 1)
		if err == nil && provider == nil {
			err = errors.New("rename provider unavailable")
		}
		if err == nil {
			action, title, err = svc.generateRename(ctx, task, provider, contextValue)
		}
		if err == nil {
			err = svc.renameTaskCurrent(ctx, task)
		}
		if err == nil {
			err = commandPhase(ctx, 2)
		}
		if err == nil {
			stage = "application"
			display := model.PublicTextContext{HiddenValues: []string{task.projectID, task.threadID}}
			if project, projectErr := svc.store.GetProject(ctx, task.projectID); projectErr == nil {
				display = contextengine.ProjectPublicTextContext(project, "")
				display.HiddenValues = append(display.HiddenValues, task.threadID)
			}
			for _, activity := range contextValue.Activity {
				if activity.Role == "user" {
					display.SourceText += activity.Text + "\n"
				}
			}
			title = model.PublicText(title, display)
			if err == nil {
				projectLock := svc.projectLock(task.projectID)
				projectLock.RLock()
				err = svc.renameTaskCurrent(ctx, task)
				if err == nil {
					current, getErr := svc.store.GetThread(ctx, task.threadID)
					err = getErr
					if err == nil {
						preparedTitle = current.Title
						if action != "keep" {
							preparedTitle = title
						}
						if preparedTitle != current.Title {
							outcome = "prepared"
						}
					}
				}
				projectLock.RUnlock()
			}
		}
	}
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) || errors.Is(err, store.ErrNamingOperationConflict) || errors.Is(err, run.ErrRunNotFound) {
			outcome = "superseded"
		} else {
			outcome = "failed"
			svc.log.Warn("rename request failed", zap.String("thread_id", task.threadID), zap.String("request_id", task.requestID), zap.String("stage", stage), zap.Any("error_diagnostic", llm.ProviderFailureDiagnostic(err)))
		}
	}
	if err != nil {
		return "", err
	}
	if outcome == "superseded" {
		return "", context.Canceled
	}
	return preparedTitle, nil
}

func renameThreadToolSchema() llm.ToolSchema {
	return tooldefinition.Get("rename_thread")
}

func parseRenameResponse(response llm.GenerateResponse) (string, string, error) {
	if err := llm.ValidateSubmissionEnvelope(response, renameToolName, true); err != nil {
		return "", "", err
	}
	args := response.ToolCalls[0].Args
	action, ok := args["action"].(string)
	if !ok {
		return "", "", llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/action", "rename action is required")
	}
	switch action {
	case "keep":
		if len(args) != 1 {
			return "", "", llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/title", "keep must not include a title")
		}
		return action, "", nil
	case "rename":
		if len(args) != 2 {
			return "", "", llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/", "rename requires only action and title")
		}
		title, ok := args["title"].(string)
		if !ok {
			return "", "", llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/title", "rename title is required")
		}
		clean, err := validateGeneratedThreadTitle(title)
		if err != nil {
			return "", "", llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/title", err.Error())
		}
		return action, clean, nil
	default:
		return "", "", llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/action", "unsupported rename action")
	}
}

func (svc *NamingService) renameContext(ctx context.Context, threadID string) (renameInput, error) {
	thread, err := svc.store.GetThread(ctx, threadID)
	if err != nil {
		return renameInput{}, err
	}
	source, err := svc.store.LoadRecentThreadConversationContext(ctx, threadID)
	if err != nil {
		return renameInput{}, err
	}
	return newRenameInput(thread.Title, source), nil
}

// GenerateNow is the explicit user command. It uses the current snapshot even
// before the first accepted conversation input; its command owns cancellation.
func (svc *NamingService) GenerateNow(parent context.Context, threadID string) (RenameResult, error) {
	if svc.provider == nil {
		return RenameResult{}, model.NewAgentError("MODEL_PROFILE_NOT_FOUND", "rename", nil)
	}
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(svc.rootCtx, cancel)
	defer func() { stop(); cancel() }()
	existing, err := svc.store.GetThread(ctx, threadID)
	if err != nil {
		return RenameResult{}, err
	}
	lock := svc.projectLock(existing.ProjectID)
	lock.RLock()
	enabled := true
	thread, err := svc.store.UpdateThreadNamingState(ctx, threadID, nil, nil, true, time.Now().UnixMilli())
	if err != nil {
		lock.RUnlock()
		return RenameResult{}, err
	}
	svc.invalidateWorker(threadID)
	command := &directRename{cancel: cancel, projectID: thread.ProjectID, done: make(chan struct{})}
	svc.mu.Lock()
	svc.direct[threadID] = command
	svc.mu.Unlock()
	generation := svc.projectGeneration(thread.ProjectID)
	lock.RUnlock()
	defer func() {
		defer close(command.done)
		svc.mu.Lock()
		if svc.direct[threadID] == command {
			delete(svc.direct, threadID)
		}
		svc.mu.Unlock()
	}()
	task := renameTask{threadID: thread.ID, projectID: thread.ProjectID, requestID: model.MustShortID("rename"), operationVersion: thread.RenameOperationVersion, trigger: RenameTriggerManual, projectGeneration: generation}
	title, err := svc.runTask(ctx, task)
	if err != nil {
		return RenameResult{}, err
	}
	return svc.renameResult(thread, title, &enabled), nil
}

func (svc *NamingService) BeginProjectSnapshot(_ context.Context, projectID string) func() {
	lock := svc.projectLock(projectID)
	lock.Lock()
	return lock.Unlock
}

func (svc *NamingService) WithDecisions(source decision.Source) *NamingService {
	svc.decisions = source
	return svc
}

func (svc *NamingService) renameTaskCurrent(ctx context.Context, task renameTask) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	thread, err := svc.store.GetThread(ctx, task.threadID)
	if err != nil {
		return err
	}
	if (task.trigger != RenameTriggerManual && !thread.AutoRenameEnabled) || thread.RenameOperationVersion != task.operationVersion || svc.projectGeneration(task.projectID) != task.projectGeneration {
		return context.Canceled
	}
	if task.trigger != RenameTriggerManual && !svc.renameInputCurrent(task.threadID, task.projectGeneration, task.input) {
		return context.Canceled
	}
	return nil
}

func (svc *NamingService) renameInputCurrent(threadID, generation string, input model.ConversationInput) bool {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	latest, ok := svc.latestInputs[threadID]
	return !ok || latest.projectGeneration != generation || latest.input.AcceptedSeq <= input.AcceptedSeq
}

func (svc *NamingService) shouldRename(ctx context.Context, task renameTask, source renameInput) (bool, error) {
	if err := svc.renameTaskCurrent(ctx, task); err != nil {
		return false, err
	}
	if strings.TrimSpace(source.CurrentTitle) == "" {
		return true, nil
	}
	if svc.decisions == nil {
		return false, nil
	}
	snapshot := svc.decisions.DecisionSnapshot()
	if snapshot.Provider == nil {
		return false, nil
	}
	history, err := svc.store.LoadThreadConversationContextBeforeInput(ctx, task.threadID, task.input.RunID, task.input.InputID)
	if err != nil || history.Input == nil || *history.Input != task.input {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		svc.log.Info("naming decision unavailable", zap.String("reason", "context_unavailable"))
		return false, nil
	}
	request, rules, err := buildRenameDecisionRequest(source.CurrentTitle, history)
	if err != nil {
		svc.log.Info("naming decision unavailable", zap.String("reason", "rules_unavailable"))
		return false, nil
	}
	if !decision.Fits(request) {
		svc.log.Info("naming decision unavailable", zap.String("reason", "capacity"))
		return false, nil
	}
	gateCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	started := time.Now()
	response, err := snapshot.Provider.Evaluate(gateCtx, request)
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		svc.log.Info("naming decision unavailable", zap.String("reason", decision.Failure(err)), zap.Duration("elapsed", time.Since(started)))
		return false, nil
	}
	answer, err := response.Noul("rename")
	if err != nil || !(answer.Noul >= 0 && answer.Noul <= 1) {
		return false, nil
	}
	svc.log.Info("naming decision", zap.String("model", response.Model), zap.String("policy", "rename-v2"), zap.String("rules_version", rules.Version), zap.String("rules_hash", rules.Hash), zap.Float64("probability", answer.Noul), zap.Int("input_tokens", response.Usage.InputTokens), zap.Duration("elapsed", time.Since(started)))
	if err := svc.renameTaskCurrent(ctx, task); err != nil {
		return false, err
	}
	// Initial conservative policy; evaluate against real Chinese naming examples.
	return answer.Noul >= 0.85, nil
}
