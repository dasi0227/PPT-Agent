package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	renameRequestTimeout = 20 * time.Second
	renameInputBudget    = 4000
	renameToolName       = "rename_thread"
)

type RenameTrigger string

const (
	RenameTriggerFirstInput RenameTrigger = "first_input"
	RenameTriggerThreshold  RenameTrigger = "input_threshold"
	RenameTriggerManual     RenameTrigger = "manual"
)

type renameTask struct {
	threadID          string
	projectID         string
	requestID         string
	operationVersion  int64
	trigger           RenameTrigger
	projectGeneration string
}

type pendingRename struct {
	trigger           RenameTrigger
	projectID         string
	projectGeneration string
}

type renameWorker struct {
	running bool
	cancel  context.CancelFunc
	pending *pendingRename
}

type directRename struct {
	cancel context.CancelFunc
}

type NamingService struct {
	store              store.Store
	provider           llm.Provider
	hub                *ThreadEventHub
	log                *zap.Logger
	rootCtx            context.Context
	cancel             context.CancelFunc
	mu                 sync.Mutex
	workers            map[string]*renameWorker
	direct             map[string]*directRename
	projectGenerations map[string]string
	projectLocks       map[string]*sync.RWMutex
}

func NewNamingService(s store.Store, provider llm.Provider, hub *ThreadEventHub, log *zap.Logger) *NamingService {
	rootCtx, cancel := context.WithCancel(context.Background())
	return &NamingService{store: s, provider: provider, hub: hub, log: log.Named("thread-naming"), rootCtx: rootCtx, cancel: cancel, workers: map[string]*renameWorker{}, direct: map[string]*directRename{}, projectGenerations: map[string]string{}, projectLocks: map[string]*sync.RWMutex{}}
}

func (svc *NamingService) Close()                  { svc.cancel() }
func (svc *NamingService) Events() *ThreadEventHub { return svc.hub }

func (svc *NamingService) ManualRename(ctx context.Context, threadID, rawTitle string) (model.Thread, error) {
	title, err := ValidateThreadTitle(rawTitle)
	if err != nil {
		return model.Thread{}, err
	}
	enabled := false
	thread, err := svc.store.UpdateThreadNamingState(ctx, threadID, &title, &enabled, true, time.Now().Unix())
	if err != nil {
		return model.Thread{}, err
	}
	svc.invalidateWorker(threadID)
	svc.hub.PublishUpdated(thread)
	return thread, nil
}

func ValidateThreadTitle(value string) (string, error) {
	title := strings.TrimSpace(value)
	if title == "" || utf8.RuneCountInString(title) > 60 {
		return "", errors.New("title length must be 1..60")
	}
	if strings.ContainsAny(title, "\r\n\t<>") {
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

func (svc *NamingService) RecordInput(ctx context.Context, threadID, inputID, content string) {
	if svc == nil || strings.TrimSpace(inputID) == "" {
		return
	}
	thread, trigger, err := svc.store.RecordThreadNamingInput(ctx, model.ThreadNamingInput{
		ThreadID: threadID, InputID: inputID, Content: strings.TrimSpace(content), AcceptedAt: time.Now().UnixNano(),
	})
	if err != nil {
		svc.log.Warn("record naming input failed", zap.String("thread_id", threadID), zap.Error(err))
		return
	}
	if !trigger {
		return
	}
	kind := RenameTriggerThreshold
	if thread.RenameInputCount == 0 {
		kind = RenameTriggerFirstInput
	}
	svc.enqueueAutomatic(threadID, thread.ProjectID, kind, svc.projectGeneration(thread.ProjectID))
}

// SetAutomatic changes current naming settings; generated/manual names use commands.
func (svc *NamingService) SetAutomatic(ctx context.Context, threadID string, enabled bool) (model.Thread, error) {
	thread, err := svc.store.UpdateThreadNamingState(ctx, threadID, nil, &enabled, true, time.Now().Unix())
	if err != nil {
		return thread, err
	}
	svc.invalidateWorker(threadID)
	svc.hub.PublishUpdated(thread)
	return thread, nil
}

func (svc *NamingService) enqueueAutomatic(threadID, projectID string, trigger RenameTrigger, generation string) {
	svc.mu.Lock()
	if svc.direct[threadID] != nil {
		svc.mu.Unlock()
		return
	}
	worker := svc.workers[threadID]
	if worker == nil {
		worker = &renameWorker{}
		svc.workers[threadID] = worker
	}
	if worker.running {
		if worker.pending == nil {
			worker.pending = &pendingRename{trigger: trigger, projectID: projectID, projectGeneration: generation}
		}
		svc.mu.Unlock()
		return
	}
	worker.running = true
	svc.mu.Unlock()
	svc.launchAutomatic(threadID, projectID, trigger, generation)
}

func (svc *NamingService) launchAutomatic(threadID, projectID string, trigger RenameTrigger, generation string) {
	// Finish a rejected worker only after releasing the project read lock:
	// draining its pending task may acquire that lock again while rollback waits.
	started := func() bool {
		projectLock := svc.projectLock(projectID)
		projectLock.RLock()
		defer projectLock.RUnlock()
		if svc.projectGeneration(projectID) != generation {
			return false
		}
		thread, err := svc.store.BeginThreadRenameRequest(svc.rootCtx, threadID, 0, true, time.Now().Unix())
		if err != nil {
			return false
		}
		return svc.launchTask(renameTask{
			threadID: thread.ID, projectID: thread.ProjectID, requestID: model.MustShortID("rename"),
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
	accepted, created, err := svc.store.AcceptCommand(requestCtx, task.threadID, model.CommandRequest{RequestKey: task.requestID, Kind: "rename", Input: input, Source: "automatic"}, 0)
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
	requestCtx = WithCommandProgress(requestCtx, func(phase int) error {
		accepted.Phase = phase
		return svc.store.SaveCommandExecution(requestCtx, accepted)
	})
	svc.mu.Lock()
	if worker := svc.workers[task.threadID]; worker != nil {
		worker.cancel = cancel
	}
	svc.mu.Unlock()
	go func() {
		defer cancel()
		err := svc.runTask(requestCtx, task)
		accepted.Status = "completed"
		if err != nil {
			accepted.Status = "failed"
			if errors.Is(err, context.Canceled) {
				accepted.Status = "canceled"
			}
			accepted.Error, _ = json.Marshal(map[string]any{"code": "RENAME_FAILED", "message": err.Error()})
		} else {
			thread, getErr := svc.store.GetThread(context.WithoutCancel(requestCtx), task.threadID)
			if getErr != nil {
				accepted.Status = "failed"
			} else {
				accepted.Result, _ = json.Marshal(map[string]any{"title": thread.Title})
			}
		}
		if err := svc.store.SaveCommandExecution(context.WithoutCancel(requestCtx), accepted); err != nil {
			svc.log.Warn("persist automatic naming result", zap.Error(err))
		}
		svc.finishWorker(task.threadID)
	}()
	return true
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
		svc.mu.Unlock()
		return
	}
	svc.mu.Unlock()
	svc.launchAutomatic(threadID, pending.projectID, pending.trigger, pending.projectGeneration)
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

func (svc *NamingService) InvalidateThread(threadID string) { svc.invalidateWorker(threadID) }

func (svc *NamingService) InvalidateProject(ctx context.Context, projectID string) {
	svc.bumpProjectGeneration(projectID)
	threads, _ := svc.store.ListThreads(ctx, projectID)
	for _, thread := range threads {
		svc.invalidateWorker(thread.ID)
	}
	svc.hub.ResetProject(ctx, projectID)
}

func (svc *NamingService) CancelProject(ctx context.Context, projectID string) {
	svc.bumpProjectGeneration(projectID)
	threads, _ := svc.store.ListThreads(ctx, projectID)
	for _, thread := range threads {
		svc.invalidateWorker(thread.ID)
	}
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

func (svc *NamingService) runTask(parent context.Context, task renameTask) error {
	ctx, cancel := context.WithTimeout(parent, renameRequestTimeout)
	defer cancel()
	outcome := "kept"
	provider := svc.provider
	var captureErr error
	if factory, ok := provider.(interface{ Capture() (llm.Provider, error) }); ok {
		provider, captureErr = factory.Capture()
	}
	var contextValue string
	err := commandPhase(ctx, 0)
	if err == nil {
		contextValue, err = svc.renameContext(ctx, task.threadID, task.trigger)
	}
	if captureErr != nil {
		err = captureErr
	}
	if err == nil {
		var response llm.GenerateResponse
		err = commandPhase(ctx, 1)
		if err == nil && provider == nil {
			err = errors.New("rename provider unavailable")
		}
		if err == nil {
			response, err = provider.Generate(ctx, llm.GenerateRequest{
				Messages: []llm.Message{
					{Role: llm.RoleSystem, Content: llm.TextContent(prompts.PublicPolicy("command.rename"))},
					{Role: llm.RoleUser, Content: llm.TextContent("<rename_context>\n" + contextValue + "\n</rename_context>")},
				},
				Tools: []llm.ToolSchema{renameThreadToolSchema()}, MaxOutputTokens: 128,
			})
		}
		if err == nil {
			err = commandPhase(ctx, 2)
		}
		if err == nil {
			var action, title string
			action, title, err = parseRenameResponse(response)
			display := model.PublicTextContext{HiddenValues: []string{task.projectID, task.threadID}}
			if project, projectErr := svc.store.GetProject(ctx, task.projectID); projectErr == nil {
				display = contextengine.ProjectPublicTextContext(project, "")
				display.HiddenValues = append(display.HiddenValues, task.threadID)
			}
			var source struct {
				First  string   `json:"first_user_request"`
				Recent []string `json:"recent_user_inputs"`
			}
			_ = json.Unmarshal([]byte(contextValue), &source)
			display.SourceText = source.First + "\n" + strings.Join(source.Recent, "\n")
			title = model.PublicText(title, display)
			if err == nil {
				projectLock := svc.projectLock(task.projectID)
				projectLock.RLock()
				if svc.projectGeneration(task.projectID) != task.projectGeneration {
					outcome = "superseded"
				} else {
					var current model.Thread
					current, err = svc.store.GetThread(ctx, task.threadID)
					if err == nil && (!current.AutoRenameEnabled || current.RenameOperationVersion != task.operationVersion) {
						outcome = "superseded"
					} else if err == nil && action == "keep" {
						outcome = "kept"
					} else if err == nil && current.Title == title {
						outcome = "kept"
					} else if err == nil {
						var thread model.Thread
						var applied bool
						thread, applied, err = svc.store.ApplyThreadRenameResult(ctx, task.threadID, title, task.operationVersion, time.Now().Unix())
						if err == nil && applied {
							outcome = "renamed"
							svc.hub.PublishUpdated(thread)
						} else if err == nil {
							outcome = "superseded"
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
			svc.log.Warn("rename request failed", zap.String("thread_id", task.threadID), zap.String("request_id", task.requestID), zap.Error(err))
		}
	}
	if err != nil {
		return err
	}
	if outcome == "superseded" {
		return context.Canceled
	}
	return nil
}

func renameThreadToolSchema() llm.ToolSchema {
	return llm.ToolSchema{
		Name: renameToolName, Description: "Rename the conversation or keep its current title.",
		Parameters: map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"action"},
			"properties": map[string]any{
				"action": map[string]any{"type": "string", "enum": []string{"rename", "keep"}},
				"title":  map[string]any{"type": "string", "minLength": 1, "maxLength": 60},
			},
		},
	}
}

func parseRenameResponse(response llm.GenerateResponse) (string, string, error) {
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != renameToolName {
		return "", "", errors.New("model must call rename_thread exactly once")
	}
	args := response.ToolCalls[0].Args
	action, ok := args["action"].(string)
	if !ok {
		return "", "", errors.New("rename action is required")
	}
	switch action {
	case "keep":
		if len(args) != 1 {
			return "", "", errors.New("keep must not include a title")
		}
		return action, "", nil
	case "rename":
		if len(args) != 2 {
			return "", "", errors.New("rename requires only action and title")
		}
		title, ok := args["title"].(string)
		if !ok {
			return "", "", errors.New("rename title is required")
		}
		clean, err := validateGeneratedThreadTitle(title)
		return action, clean, err
	default:
		return "", "", errors.New("unsupported rename action")
	}
}

func (svc *NamingService) renameContext(ctx context.Context, threadID string, trigger RenameTrigger) (string, error) {
	thread, err := svc.store.GetThread(ctx, threadID)
	if err != nil {
		return "", err
	}
	source, err := svc.store.LoadThreadRenameContext(ctx, threadID)
	if err != nil {
		return "", err
	}
	inputs := make([]string, 0, len(source.Inputs))
	for _, input := range source.Inputs {
		if value := strings.TrimSpace(input.Content); value != "" {
			inputs = append(inputs, value)
		}
	}
	first := ""
	if len(inputs) > 0 {
		first = inputs[0]
	}
	value := map[string]any{
		"current_title": thread.Title, "trigger": trigger, "first_user_request": first,
		"recent_user_inputs": inputs, "recent_assistant_replies": source.AssistantReplies,
	}
	if source.Plan != nil {
		value["plan"] = contextengine.ModelValue(source.Plan)
	}
	if strings.TrimSpace(source.ContextSummary) != "" {
		value["context_summary"] = source.ContextSummary
	}
	return fitRenameContext(value)
}

func fitRenameContext(value map[string]any) (string, error) {
	systemTokens := contextengine.EstimateTextTokens(prompts.PublicPolicy("command.rename"))
	toolTokens := contextengine.EstimateValueTokens([]llm.ToolSchema{renameThreadToolSchema()})
	budget := renameInputBudget - systemTokens - toolTokens - 32
	if budget < 256 {
		return "", errors.New("rename prompt exceeds input budget")
	}
	for attempts := 0; attempts < 64; attempts++ {
		raw, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		if contextengine.EstimateTextTokens(string(raw)) <= budget {
			return string(raw), nil
		}
		if replies, ok := value["recent_assistant_replies"].([]string); ok && len(replies) > 0 {
			value["recent_assistant_replies"] = replies[1:]
			continue
		}
		if _, ok := value["context_summary"]; ok {
			delete(value, "context_summary")
			continue
		}
		if _, ok := value["plan"]; ok {
			delete(value, "plan")
			continue
		}
		if inputs, ok := value["recent_user_inputs"].([]string); ok && len(inputs) > 2 {
			value["recent_user_inputs"] = inputs[1:]
			continue
		}
		for _, key := range []string{"first_user_request", "recent_user_inputs"} {
			switch current := value[key].(type) {
			case string:
				value[key] = truncateRenameText(current)
			case []string:
				for index := range current {
					current[index] = truncateRenameText(current[index])
				}
				value[key] = current
			}
		}
	}
	return "", fmt.Errorf("rename context cannot fit input budget")
}

func truncateRenameText(value string) string {
	runes := []rune(value)
	if len(runes) <= 256 {
		return value
	}
	return string(runes[:240]) + "…[truncated]"
}

// GenerateNow is the explicit user command. It uses the current snapshot even
// before the first accepted conversation input; its command owns cancellation.
func (svc *NamingService) GenerateNow(parent context.Context, threadID string) (model.Thread, error) {
	if svc.provider == nil {
		return model.Thread{}, model.NewAgentError("MODEL_PROFILE_NOT_FOUND", "rename", nil)
	}
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(svc.rootCtx, cancel)
	defer func() { stop(); cancel() }()
	existing, err := svc.store.GetThread(ctx, threadID)
	if err != nil {
		return model.Thread{}, err
	}
	lock := svc.projectLock(existing.ProjectID)
	lock.RLock()
	enabled := true
	thread, err := svc.store.UpdateThreadNamingState(ctx, threadID, nil, &enabled, true, time.Now().Unix())
	if err != nil {
		lock.RUnlock()
		return model.Thread{}, err
	}
	svc.invalidateWorker(threadID)
	command := &directRename{cancel: cancel}
	svc.mu.Lock()
	svc.direct[threadID] = command
	svc.mu.Unlock()
	generation := svc.projectGeneration(thread.ProjectID)
	lock.RUnlock()
	defer func() {
		svc.mu.Lock()
		if svc.direct[threadID] == command {
			delete(svc.direct, threadID)
		}
		svc.mu.Unlock()
	}()
	svc.hub.PublishUpdated(thread)
	task := renameTask{threadID: thread.ID, projectID: thread.ProjectID, requestID: model.MustShortID("rename"), operationVersion: thread.RenameOperationVersion, trigger: RenameTriggerManual, projectGeneration: generation}
	if err := svc.runTask(ctx, task); err != nil {
		return model.Thread{}, err
	}
	return svc.store.GetThread(context.WithoutCancel(ctx), threadID)
}

func (svc *NamingService) BeginProjectSnapshot(_ context.Context, projectID string) func() {
	lock := svc.projectLock(projectID)
	lock.Lock()
	return lock.Unlock
}
