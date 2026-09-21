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
	"github.com/dasi0227/PPT-Agent/backend/internal/idempotency"
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

type NamingOperationResponse struct {
	OperationID string
	RequestID   string
	Status      string
	Thread      model.Thread
	StreamEpoch string
}

type renameTask struct {
	threadID          string
	projectID         string
	requestID         string
	operationID       string
	operationVersion  int64
	trigger           RenameTrigger
	explicit          bool
	projectGeneration string
}

type pendingRename struct {
	automatic         bool
	trigger           RenameTrigger
	projectID         string
	projectGeneration string
	task              renameTask
}

type renameWorker struct {
	running bool
	cancel  context.CancelFunc
	pending *pendingRename
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
	projectGenerations map[string]string
	projectLocks       map[string]*sync.RWMutex
}

func NewNamingService(s store.Store, provider llm.Provider, hub *ThreadEventHub, log *zap.Logger) *NamingService {
	rootCtx, cancel := context.WithCancel(context.Background())
	return &NamingService{store: s, provider: provider, hub: hub, log: log.Named("thread-naming"), rootCtx: rootCtx, cancel: cancel, workers: map[string]*renameWorker{}, projectGenerations: map[string]string{}, projectLocks: map[string]*sync.RWMutex{}}
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

func (svc *NamingService) Operate(ctx context.Context, threadID, operationID, action, rawTitle string) (NamingOperationResponse, error) {
	if svc == nil || !clientIdentityPattern.MatchString(operationID) {
		return NamingOperationResponse{}, errors.New("invalid operation_id")
	}
	if action != "generate" && action != "manual" && action != "enable" && action != "disable" {
		return NamingOperationResponse{}, errors.New("invalid naming action")
	}
	if action != "manual" && strings.TrimSpace(rawTitle) != "" {
		return NamingOperationResponse{}, errors.New("title is only valid for manual naming")
	}
	if _, err := svc.store.GetThread(ctx, threadID); err != nil {
		return NamingOperationResponse{}, err
	}
	var title string
	var err error
	if action == "manual" {
		title, err = ValidateThreadTitle(rawTitle)
		if err != nil {
			return NamingOperationResponse{}, err
		}
	}
	hash, err := idempotency.CanonicalHash(map[string]string{"action": action, "title": title})
	if err != nil {
		return NamingOperationResponse{}, err
	}
	if existing, getErr := svc.store.GetThreadNamingOperation(ctx, threadID, operationID); getErr == nil {
		return svc.awaitOperation(ctx, existing, hash)
	} else if !errors.Is(getErr, run.ErrRunNotFound) {
		return NamingOperationResponse{}, getErr
	}
	now := time.Now().Unix()
	created, err := svc.store.CreateThreadNamingOperation(ctx, model.ThreadNamingOperation{
		ThreadID: threadID, OperationID: operationID, RequestHash: hash, Action: action,
		Status: "in_progress", CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return NamingOperationResponse{}, err
	}
	if !created {
		existing, getErr := svc.store.GetThreadNamingOperation(ctx, threadID, operationID)
		if getErr != nil {
			return NamingOperationResponse{}, getErr
		}
		return svc.awaitOperation(ctx, existing, hash)
	}

	response, task, operationErr := svc.applyOperation(ctx, threadID, operationID, action, title)
	status := "completed"
	if operationErr != nil {
		status = "failed"
	}
	if response.Status == "accepted" {
		status = "accepted"
	}
	raw, _ := json.Marshal(response)
	completeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	completeErr := svc.store.CompleteThreadNamingOperation(completeCtx, model.ThreadNamingOperation{
		ThreadID: threadID, OperationID: operationID, RequestHash: hash,
		Status: status, RequestID: response.RequestID, ResultJSON: string(raw), UpdatedAt: time.Now().Unix(),
	})
	cancel()
	if operationErr != nil {
		return NamingOperationResponse{}, operationErr
	}
	if completeErr != nil {
		return NamingOperationResponse{}, completeErr
	}
	if task != nil {
		svc.enqueueExplicit(*task)
	}
	return response, nil
}

func (svc *NamingService) awaitOperation(ctx context.Context, operation model.ThreadNamingOperation, hash string) (NamingOperationResponse, error) {
	if operation.RequestHash != hash {
		return NamingOperationResponse{}, store.ErrNamingOperationConflict
	}
	for operation.Status == "in_progress" {
		select {
		case <-ctx.Done():
			return NamingOperationResponse{}, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
		next, err := svc.store.GetThreadNamingOperation(ctx, operation.ThreadID, operation.OperationID)
		if err != nil {
			return NamingOperationResponse{}, err
		}
		operation = next
	}
	return svc.replayOperation(operation, hash)
}

func (svc *NamingService) replayOperation(operation model.ThreadNamingOperation, hash string) (NamingOperationResponse, error) {
	if operation.RequestHash != hash {
		return NamingOperationResponse{}, store.ErrNamingOperationConflict
	}
	if operation.Status == "in_progress" || operation.ResultJSON == "" {
		return NamingOperationResponse{}, store.ErrNamingOperationConflict
	}
	if operation.Status == "failed" {
		return NamingOperationResponse{}, errors.New("naming operation failed")
	}
	var response NamingOperationResponse
	if err := json.Unmarshal([]byte(operation.ResultJSON), &response); err != nil {
		return NamingOperationResponse{}, err
	}
	return response, nil
}

func (svc *NamingService) applyOperation(ctx context.Context, threadID, operationID, action, title string) (NamingOperationResponse, *renameTask, error) {
	now := time.Now().Unix()
	var titleValue *string
	var enabled *bool
	switch action {
	case "manual":
		titleValue = &title
		value := false
		enabled = &value
	case "enable":
		value := true
		enabled = &value
	case "disable":
		value := false
		enabled = &value
	case "generate":
		value := true
		enabled = &value
	}
	thread, err := svc.store.UpdateThreadNamingState(ctx, threadID, titleValue, enabled, action != "generate", now)
	if err != nil {
		return NamingOperationResponse{}, nil, err
	}
	svc.invalidateWorker(threadID)
	svc.hub.PublishUpdated(thread)
	response := NamingOperationResponse{OperationID: operationID, Status: "completed", Thread: thread, StreamEpoch: svc.hub.Epoch(thread.ProjectID)}
	if action != "generate" || !thread.RenameFirstInputSeen {
		if action == "generate" {
			response.Status = "waiting_for_input"
		}
		return response, nil, nil
	}
	requestID := model.MustShortID("rename")
	response.RequestID = requestID
	response.Status = "accepted"
	task := &renameTask{
		threadID: thread.ID, projectID: thread.ProjectID, requestID: requestID,
		operationID: operationID, operationVersion: thread.RenameOperationVersion,
		trigger: RenameTriggerManual, explicit: true, projectGeneration: svc.projectGeneration(thread.ProjectID),
	}
	return response, task, nil
}

func (svc *NamingService) enqueueAutomatic(threadID, projectID string, trigger RenameTrigger, generation string) {
	svc.mu.Lock()
	worker := svc.workers[threadID]
	if worker == nil {
		worker = &renameWorker{}
		svc.workers[threadID] = worker
	}
	if worker.running {
		if worker.pending == nil || !worker.pending.task.explicit {
			worker.pending = &pendingRename{automatic: true, trigger: trigger, projectID: projectID, projectGeneration: generation}
		}
		svc.mu.Unlock()
		return
	}
	worker.running = true
	svc.mu.Unlock()
	svc.launchAutomatic(threadID, projectID, trigger, generation)
}

func (svc *NamingService) launchAutomatic(threadID, projectID string, trigger RenameTrigger, generation string) {
	projectLock := svc.projectLock(projectID)
	projectLock.RLock()
	defer projectLock.RUnlock()
	if svc.projectGeneration(projectID) != generation {
		svc.finishWorker(threadID)
		return
	}
	thread, err := svc.store.BeginThreadRenameRequest(svc.rootCtx, threadID, 0, true, time.Now().Unix())
	if err != nil {
		svc.finishWorker(threadID)
		return
	}
	task := renameTask{
		threadID: thread.ID, projectID: thread.ProjectID, requestID: model.MustShortID("rename"),
		operationVersion: thread.RenameOperationVersion, trigger: trigger, projectGeneration: generation,
	}
	svc.launchTask(task)
}

func (svc *NamingService) launchExplicit(task renameTask) {
	projectLock := svc.projectLock(task.projectID)
	projectLock.RLock()
	if svc.projectGeneration(task.projectID) != task.projectGeneration {
		projectLock.RUnlock()
		svc.completeTaskOperation(task, "superseded")
		svc.hub.PublishResult(task.threadID, task.projectID, task.operationID, task.requestID, "superseded", "")
		svc.finishWorker(task.threadID)
		return
	}
	_, err := svc.store.StartThreadExplicitRenameRequest(svc.rootCtx, task.threadID, task.operationVersion, time.Now().Unix())
	projectLock.RUnlock()
	if err != nil {
		svc.completeTaskOperation(task, "superseded")
		svc.hub.PublishResult(task.threadID, task.projectID, task.operationID, task.requestID, "superseded", "")
		svc.finishWorker(task.threadID)
		return
	}
	svc.launchTask(task)
}

func (svc *NamingService) enqueueExplicit(task renameTask) {
	svc.mu.Lock()
	worker := svc.workers[task.threadID]
	if worker == nil {
		worker = &renameWorker{}
		svc.workers[task.threadID] = worker
	}
	if worker.running {
		if worker.cancel != nil {
			worker.cancel()
		}
		worker.pending = &pendingRename{task: task}
		svc.mu.Unlock()
		return
	}
	worker.running = true
	svc.mu.Unlock()
	svc.launchExplicit(task)
}

func (svc *NamingService) launchTask(task renameTask) {
	requestCtx, cancel := context.WithCancel(svc.rootCtx)
	svc.mu.Lock()
	if worker := svc.workers[task.threadID]; worker != nil {
		worker.cancel = cancel
	}
	svc.mu.Unlock()
	go func() {
		defer cancel()
		svc.runTask(requestCtx, task)
		svc.finishWorker(task.threadID)
	}()
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
	if pending.automatic {
		svc.launchAutomatic(threadID, pending.projectID, pending.trigger, pending.projectGeneration)
		return
	}
	svc.launchExplicit(pending.task)
}

func (svc *NamingService) invalidateWorker(threadID string) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
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

func (svc *NamingService) runTask(parent context.Context, task renameTask) {
	ctx, cancel := context.WithTimeout(parent, renameRequestTimeout)
	defer cancel()
	outcome := "kept"
	safeError := ""
	provider := svc.provider
	var captureErr error
	if factory, ok := provider.(interface{ Capture() (llm.Provider, error) }); ok {
		provider, captureErr = factory.Capture()
	}
	contextValue, err := svc.renameContext(ctx, task.threadID, task.trigger)
	if captureErr != nil {
		err = captureErr
	}
	if err == nil {
		var response llm.GenerateResponse
		response, err = provider.Generate(ctx, llm.GenerateRequest{
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: llm.TextContent(prompts.MustLoad("command.rename").Body)},
				{Role: llm.RoleUser, Content: llm.TextContent("<rename_context>\n" + contextValue + "\n</rename_context>")},
			},
			Tools: []llm.ToolSchema{renameThreadToolSchema()}, MaxOutputTokens: 128,
		})
		if err == nil {
			var action, title string
			action, title, err = parseRenameResponse(response)
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
			safeError = "自动命名失败，请稍后重试"
			svc.log.Warn("rename request failed", zap.String("thread_id", task.threadID), zap.String("request_id", task.requestID), zap.Error(err))
		}
	}
	svc.completeTaskOperation(task, outcome)
	if provider != nil {
		svc.hub.PublishResult(task.threadID, task.projectID, task.operationID, task.requestID, outcome, safeError, llm.ExecutionOf(provider))
	} else {
		svc.hub.PublishResult(task.threadID, task.projectID, task.operationID, task.requestID, outcome, safeError)
	}
}

func (svc *NamingService) completeTaskOperation(task renameTask, outcome string) {
	if !task.explicit {
		return
	}
	projectLock := svc.projectLock(task.projectID)
	projectLock.RLock()
	defer projectLock.RUnlock()
	if svc.projectGeneration(task.projectID) != task.projectGeneration {
		return
	}
	operation, err := svc.store.GetThreadNamingOperation(context.Background(), task.threadID, task.operationID)
	if err != nil {
		return
	}
	operation.Status = "completed"
	if outcome == "failed" {
		operation.Status = "failed"
	}
	operation.RequestID = task.requestID
	var response NamingOperationResponse
	if json.Unmarshal([]byte(operation.ResultJSON), &response) == nil {
		response.Status = "completed"
		response.RequestID = task.requestID
		response.StreamEpoch = svc.hub.Epoch(task.projectID)
		if thread, getErr := svc.store.GetThread(context.Background(), task.threadID); getErr == nil {
			response.Thread = thread
		}
		if raw, marshalErr := json.Marshal(response); marshalErr == nil {
			operation.ResultJSON = string(raw)
		}
	}
	operation.UpdatedAt = time.Now().Unix()
	_ = svc.store.CompleteThreadNamingOperation(context.Background(), operation)
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
		value["plan"] = source.Plan
	}
	if strings.TrimSpace(source.ContextSummary) != "" {
		value["context_summary"] = source.ContextSummary
	}
	return fitRenameContext(value)
}

func fitRenameContext(value map[string]any) (string, error) {
	systemTokens := contextengine.EstimateTextTokens(prompts.MustLoad("command.rename").Body)
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
