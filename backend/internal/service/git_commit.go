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

	"github.com/dasi0227/PPT-Agent/backend/internal/gitcommit"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

const (
	gitCommitModelAttempts = 3
	gitCommitModelTimeout  = 45 * time.Second
)

type GitCommitParams struct {
	ThreadID        string
	Model           string
	ClientRequestID string
}

type gitCommitBus struct {
	mu          sync.Mutex
	seq         int64
	subscribers map[int]chan model.GitCommitEvent
	nextID      int
	closed      bool
}

func newGitCommitBus() *gitCommitBus {
	return &gitCommitBus{subscribers: map[int]chan model.GitCommitEvent{}}
}

func (b *gitCommitBus) subscribe() (<-chan model.GitCommitEvent, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	channel := make(chan model.GitCommitEvent, 16)
	if b.closed {
		close(channel)
		return channel, func() {}
	}
	id := b.nextID
	b.nextID++
	b.subscribers[id] = channel
	return channel, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if current, ok := b.subscribers[id]; ok {
			delete(b.subscribers, id)
			close(current)
		}
	}
}

func (b *gitCommitBus) publish(event model.GitCommitEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq = event.Seq
	for _, channel := range b.subscribers {
		channel <- event
	}
}

func (b *gitCommitBus) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for id, channel := range b.subscribers {
		delete(b.subscribers, id)
		close(channel)
	}
}

type GitCommitService struct {
	store    store.Store
	registry *llm.Registry
	locks    *run.LockManager
	git      *gitcommit.Executor

	mu    sync.Mutex
	buses map[string]*gitCommitBus
}

func NewGitCommitService(s store.Store, registry *llm.Registry, locks *run.LockManager) *GitCommitService {
	return &GitCommitService{
		store: s, registry: registry, locks: locks, git: gitcommit.NewExecutor(),
		buses: map[string]*gitCommitBus{},
	}
}

func (svc *GitCommitService) Initialize(ctx context.Context) error {
	operations, err := svc.store.ListActiveGitCommits(ctx)
	if err != nil {
		return err
	}
	for i := range operations {
		operation := operations[i]
		events, eventsErr := svc.store.GitCommitEventsSince(ctx, operation.ID, 0)
		if eventsErr != nil {
			return eventsErr
		}
		bus := newGitCommitBus()
		if len(events) > 0 {
			bus.seq = events[len(events)-1].Seq
		}
		svc.fail(ctx, &operation, bus, "COMMIT_INTERRUPTED", true, errors.New("Git commit interrupted by service restart"))
		bus.close()
	}
	return nil
}

func (svc *GitCommitService) Start(ctx context.Context, projectID string, params GitCommitParams) (model.GitCommitOperation, error) {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(params.ThreadID) == "" ||
		strings.TrimSpace(params.Model) == "" || strings.TrimSpace(params.ClientRequestID) == "" {
		return model.GitCommitOperation{}, ErrGitCommitInvalid
	}
	if existing, err := svc.store.GetGitCommitOperationByRequest(ctx, params.ThreadID, params.ClientRequestID); err == nil {
		return existing, nil
	}
	project, err := svc.store.GetProject(ctx, projectID)
	if err != nil {
		return model.GitCommitOperation{}, err
	}
	thread, err := svc.store.GetThread(ctx, params.ThreadID)
	if err != nil {
		return model.GitCommitOperation{}, err
	}
	if thread.ProjectID != project.ID {
		return model.GitCommitOperation{}, ErrGitCommitInvalid
	}
	profile, err := svc.registry.Resolve(params.Model)
	if err != nil {
		return model.GitCommitOperation{}, model.NewAgentError("MODEL_PROFILE_NOT_FOUND", "git_commit", err)
	}
	if !profile.Capabilities().ToolCalls {
		return model.GitCommitOperation{}, ErrGitCommitToolUnsupported
	}
	activeRun, err := svc.store.HasActiveRun(ctx, project.ID)
	if err != nil {
		return model.GitCommitOperation{}, err
	}
	if activeRun {
		return model.GitCommitOperation{}, ErrRunActive
	}
	activeCommit, err := svc.store.HasActiveGitCommit(ctx, project.ID)
	if err != nil {
		return model.GitCommitOperation{}, err
	}
	if activeCommit {
		return model.GitCommitOperation{}, ErrGitCommitActive
	}
	release, acquired := svc.locks.TryAcquire(project.ID)
	if !acquired {
		return model.GitCommitOperation{}, ErrRunActive
	}
	now := time.Now().Unix()
	operation := model.GitCommitOperation{
		ID: model.MustShortID("gco"), ProjectID: project.ID, ThreadID: thread.ID,
		ClientRequestID: params.ClientRequestID, ModelProfile: profile.Name(),
		Status: model.GitCommitAccepted, CreatedAt: now, UpdatedAt: now,
	}
	if err := svc.store.CreateGitCommitOperation(ctx, operation); err != nil {
		release()
		if existing, getErr := svc.store.GetGitCommitOperationByRequest(ctx, params.ThreadID, params.ClientRequestID); getErr == nil {
			return existing, nil
		}
		if errors.Is(err, store.ErrRunActive) {
			return model.GitCommitOperation{}, ErrRunActive
		}
		return model.GitCommitOperation{}, err
	}
	bus := newGitCommitBus()
	svc.mu.Lock()
	svc.buses[operation.ID] = bus
	svc.mu.Unlock()
	go svc.execute(context.Background(), project, operation, profile, bus, release)
	return operation, nil
}

func (svc *GitCommitService) Get(ctx context.Context, operationID string) (model.GitCommitOperation, error) {
	return svc.store.GetGitCommitOperation(ctx, operationID)
}

func (svc *GitCommitService) Subscribe(ctx context.Context, operationID string, afterSeq int64) (<-chan model.GitCommitEvent, func(), error) {
	operation, err := svc.store.GetGitCommitOperation(ctx, operationID)
	if err != nil {
		return nil, nil, err
	}
	svc.mu.Lock()
	bus := svc.buses[operationID]
	svc.mu.Unlock()
	var live <-chan model.GitCommitEvent
	stopLive := func() {}
	if bus != nil && !operation.Status.Terminal() {
		live, stopLive = bus.subscribe()
	}
	history, err := svc.store.GitCommitEventsSince(ctx, operationID, afterSeq)
	if err != nil {
		stopLive()
		return nil, nil, err
	}
	out := make(chan model.GitCommitEvent, len(history)+16)
	stop := make(chan struct{})
	go func() {
		defer close(out)
		last := afterSeq
		for _, event := range history {
			if event.Seq > last {
				out <- event
				last = event.Seq
			}
		}
		if live == nil {
			return
		}
		for {
			select {
			case event, ok := <-live:
				if !ok {
					return
				}
				if event.Seq > last {
					out <- event
					last = event.Seq
				}
			case <-stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, func() {
		select {
		case <-stop:
		default:
			close(stop)
		}
		stopLive()
	}, nil
}

func (svc *GitCommitService) execute(
	ctx context.Context,
	project model.Project,
	operation model.GitCommitOperation,
	profile llm.Profile,
	bus *gitCommitBus,
	release func(),
) {
	defer release()
	defer func() {
		bus.close()
		svc.mu.Lock()
		delete(svc.buses, operation.ID)
		svc.mu.Unlock()
	}()
	if err := svc.git.Bootstrap(ctx, project.WorkDir); err != nil {
		svc.fail(ctx, &operation, bus, "GIT_UNAVAILABLE", false, err)
		return
	}
	if err := svc.emitProgress(ctx, &operation, bus, model.GitCommitStaging); err != nil {
		svc.fail(ctx, &operation, bus, "GIT_STAGE_FAILED", true, err)
		return
	}
	changes, cleanup, err := svc.git.StageAll(ctx, project.WorkDir, operation.ID)
	if err != nil {
		svc.fail(ctx, &operation, bus, "GIT_STAGE_FAILED", true, err)
		return
	}
	defer cleanup()
	if changes.FilesChanged == 0 {
		payload := model.GitCommitEmptyPayload{GitCommitEventBase: model.NewGitCommitEventBase(operation.ID, project.ID, operation.ThreadID)}
		_ = svc.emit(ctx, &operation, bus, model.EventGitCommitEmpty, payload, model.GitCommitEmpty, "", "", "")
		return
	}
	if err := svc.emitProgress(ctx, &operation, bus, model.GitCommitAnalyzing); err != nil {
		svc.fail(ctx, &operation, bus, "GIT_STAGE_FAILED", true, err)
		return
	}
	message, err := generateGitCommitMessage(ctx, profile, project.Title, changes)
	if err != nil {
		code := "COMMIT_MESSAGE_INVALID"
		if errors.Is(err, context.DeadlineExceeded) {
			code = "PROVIDER_UNAVAILABLE"
		}
		svc.fail(ctx, &operation, bus, code, true, err)
		return
	}
	if err := svc.emitProgress(ctx, &operation, bus, model.GitCommitCommitting); err != nil {
		svc.fail(ctx, &operation, bus, "GIT_COMMIT_FAILED", true, err)
		return
	}
	gitResult, err := svc.git.Commit(ctx, project.WorkDir, changes, gitcommit.Message{
		Title: message.Title, Items: message.Items,
	})
	if err != nil {
		svc.fail(ctx, &operation, bus, "GIT_COMMIT_FAILED", true, err)
		return
	}
	result := model.GitCommitResult{
		Title: message.Title, Items: message.Items,
		Branch: gitResult.Branch, Hash: gitResult.Hash, CommittedAt: gitResult.CommittedAt,
		FilesChanged: gitResult.FilesChanged, Insertions: gitResult.Insertions, Deletions: gitResult.Deletions,
	}
	payload := model.GitCommitCompletedPayload{
		GitCommitEventBase: model.NewGitCommitEventBase(operation.ID, project.ID, operation.ThreadID),
		Commit:             result,
	}
	raw, _ := json.Marshal(result)
	_ = svc.emit(ctx, &operation, bus, model.EventGitCommitCompleted, payload, model.GitCommitCompleted, "", string(raw), "")
}

func (svc *GitCommitService) emitProgress(
	ctx context.Context,
	operation *model.GitCommitOperation,
	bus *gitCommitBus,
	phase model.GitCommitPhase,
) error {
	payload := model.GitCommitProgressPayload{
		GitCommitEventBase: model.NewGitCommitEventBase(operation.ID, operation.ProjectID, operation.ThreadID),
		Phase:              phase,
	}
	return svc.emit(ctx, operation, bus, model.EventGitCommitProgress, payload, model.GitCommitRunning, phase, "", "")
}

func (svc *GitCommitService) fail(
	ctx context.Context,
	operation *model.GitCommitOperation,
	bus *gitCommitBus,
	code string,
	retryable bool,
	cause error,
) {
	publicError := model.GitCommitPublicError{
		Code: code, Message: "提交失败，请重新尝试或手动提交", Retryable: retryable,
	}
	payload := model.GitCommitFailedPayload{
		GitCommitEventBase: model.NewGitCommitEventBase(operation.ID, operation.ProjectID, operation.ThreadID),
		Error:              publicError,
	}
	raw, _ := json.Marshal(publicError)
	_ = svc.emit(ctx, operation, bus, model.EventGitCommitFailed, payload, model.GitCommitFailed, "", "", string(raw))
	_ = cause
}

func (svc *GitCommitService) emit(
	ctx context.Context,
	operation *model.GitCommitOperation,
	bus *gitCommitBus,
	eventType model.GitCommitEventType,
	payload any,
	status model.GitCommitStatus,
	phase model.GitCommitPhase,
	resultJSON string,
	errorJSON string,
) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	bus.mu.Lock()
	seq := bus.seq + 1
	bus.mu.Unlock()
	now := time.Now().Unix()
	event := model.GitCommitEvent{
		OperationID: operation.ID, Seq: seq, Type: eventType, Payload: string(raw), CreatedAt: now,
	}
	if err := svc.store.AppendGitCommitEvent(ctx, event); err != nil {
		return err
	}
	operation.Status, operation.Phase = status, phase
	operation.ResultJSON, operation.ErrorJSON, operation.UpdatedAt = resultJSON, errorJSON, now
	if err := svc.store.UpdateGitCommitOperation(ctx, *operation); err != nil {
		return err
	}
	bus.publish(event)
	return nil
}

type generatedCommitMessage struct {
	Title string
	Items []string
}

func generateGitCommitMessage(
	ctx context.Context,
	profile llm.Profile,
	projectTitle string,
	changes gitcommit.ChangeSet,
) (generatedCommitMessage, error) {
	prompt := prompts.MustLoad("command.commit")
	user := fmt.Sprintf(
		"Project: %s\n\nFile status:\n%s\n\nLine statistics:\n%s\n\nStaged diff:\n%s",
		projectTitle, changes.NameStatus, changes.NumStat, changes.Diff,
	)
	tool := llm.ToolSchema{
		Name: "git_commit", Description: "Generate the commit title and summary items.",
		Parameters: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []string{"title", "items"},
			"properties": map[string]any{
				"title": map[string]any{"type": "string", "minLength": 1, "maxLength": 72},
				"items": map[string]any{
					"type": "array", "minItems": 1, "maxItems": 6,
					"items": map[string]any{"type": "string", "minLength": 1, "maxLength": 160},
				},
			},
		},
	}
	var lastErr error
	for attempt := 0; attempt < gitCommitModelAttempts; attempt++ {
		requestCtx, cancel := context.WithTimeout(ctx, gitCommitModelTimeout)
		response, err := profile.Adapter().Generate(requestCtx, llm.GenerateRequest{
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: llm.TextContent(prompt.Body)},
				{Role: llm.RoleUser, Content: llm.TextContent(user)},
			},
			Tools: []llm.ToolSchema{tool}, Reasoning: llm.ReasoningDisabled, MaxOutputTokens: 1024,
		})
		cancel()
		if err != nil {
			lastErr = err
			if errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
				return generatedCommitMessage{}, context.DeadlineExceeded
			}
			continue
		}
		message, validateErr := validateGitCommitResponse(response)
		if validateErr == nil {
			return message, nil
		}
		lastErr = validateErr
	}
	return generatedCommitMessage{}, lastErr
}

func validateGitCommitResponse(response llm.GenerateResponse) (generatedCommitMessage, error) {
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "git_commit" {
		return generatedCommitMessage{}, errors.New("model must call git_commit exactly once")
	}
	call := response.ToolCalls[0]
	title, ok := call.Args["title"].(string)
	if !ok {
		return generatedCommitMessage{}, errors.New("commit title is required")
	}
	title = strings.TrimSpace(title)
	if !validCommitText(title, 72) || strings.Contains(title, "\n") {
		return generatedCommitMessage{}, errors.New("commit title is invalid")
	}
	rawItems, ok := call.Args["items"].([]any)
	if !ok {
		if stringsItems, stringsOK := call.Args["items"].([]string); stringsOK {
			rawItems = make([]any, len(stringsItems))
			for i := range stringsItems {
				rawItems[i] = stringsItems[i]
			}
		} else {
			return generatedCommitMessage{}, errors.New("commit items are required")
		}
	}
	if len(rawItems) < 1 || len(rawItems) > 6 {
		return generatedCommitMessage{}, errors.New("commit items count is invalid")
	}
	seen := map[string]bool{}
	items := make([]string, 0, len(rawItems))
	for _, raw := range rawItems {
		item, ok := raw.(string)
		if !ok {
			return generatedCommitMessage{}, errors.New("commit item must be text")
		}
		item = strings.TrimSpace(strings.TrimPrefix(item, "-"))
		if !validCommitText(item, 160) {
			return generatedCommitMessage{}, errors.New("commit item is invalid")
		}
		if !seen[item] {
			seen[item] = true
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return generatedCommitMessage{}, errors.New("commit items are empty")
	}
	return generatedCommitMessage{Title: title, Items: items}, nil
}

func validCommitText(value string, maxRunes int) bool {
	if value == "" || utf8.RuneCountInString(value) > maxRunes {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
