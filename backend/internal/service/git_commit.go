package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/gitcommit"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
)

const (
	gitCommitModelAttempts = 3
	gitCommitModelTimeout  = 45 * time.Second
)

type GitCommitService struct {
	store    store.Store
	registry *llm.Registry
	locks    *run.LockManager
	git      *gitcommit.Executor
}

func NewGitCommitService(s store.Store, registry *llm.Registry, locks *run.LockManager) *GitCommitService {
	return &GitCommitService{store: s, registry: registry, locks: locks, git: gitcommit.NewExecutor()}
}

type uncertainCommitError struct{ cause error }

func (e *uncertainCommitError) Error() string {
	return "Git 提交结果无法确认，请检查仓库后再决定后续操作"
}
func (svc *GitCommitService) Initialize(ctx context.Context) error {
	commands, err := svc.store.ListActiveCommitCommands(ctx)
	if err != nil {
		return err
	}
	for _, command := range commands {
		project, err := svc.store.GetProject(ctx, command.ProjectID)
		if err != nil {
			return err
		}
		events, err := svc.store.ThreadEvents(ctx, command.ThreadID, 0)
		if err != nil {
			return err
		}
		var intent *gitcommit.CommitIntent
		for _, event := range events {
			if event.Type == "commit.intent" && event.AttemptID == command.AttemptID {
				var value gitcommit.CommitIntent
				if err := json.Unmarshal(event.Payload, &value); err != nil {
					return err
				}
				intent = &value
			}
		}
		command.Status = "interrupted"
		command.Error = json.RawMessage(`{"code":"COMMIT_INTERRUPTED","message":"服务已重启，请重试。","retryable":true}`)
		if intent != nil {
			result, confirmed, err := svc.git.Reconcile(ctx, project.WorkDir, *intent)
			if err == nil && confirmed {
				command.Status = "completed"
				command.Error = nil
				command.Result, _ = json.Marshal(model.GitCommitResult{Title: intent.Message.Title, Items: intent.Message.Items, Branch: result.Branch, Hash: result.Hash, FilesChanged: result.FilesChanged, Insertions: result.Insertions, Deletions: result.Deletions, CommittedAt: result.CommittedAt})
			} else {
				command.Error = json.RawMessage(`{"code":"COMMIT_UNCERTAIN","message":"提交结果无法确认，请检查仓库。","retryable":false}`)
			}
		}
		if err := svc.store.SaveCommandExecution(ctx, command); err != nil {
			return err
		}
	}
	return nil
}
func (svc *GitCommitService) ExecuteCommand(ctx context.Context, execution model.CommandExecution) (any, error) {
	project, err := svc.store.GetProject(ctx, execution.ProjectID)
	if err != nil {
		return nil, err
	}
	profile, err := svc.registry.RoutedProfile("commit", "")
	if err != nil {
		return nil, err
	}
	if !profile.Capabilities().ToolCalls {
		return nil, ErrGitCommitToolUnsupported
	}
	release, ok := svc.locks.TryAcquire(project.ID)
	if !ok {
		return nil, ErrRunActive
	}
	defer release()
	if err := svc.git.Bootstrap(ctx, project.WorkDir); err != nil {
		return nil, err
	}
	if err := commandPhase(ctx, 0); err != nil {
		return nil, err
	}
	changes, cleanup, err := svc.git.StageAll(ctx, project.WorkDir, execution.AttemptID)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if changes.FilesChanged == 0 {
		return map[string]any{"empty": true}, nil
	}
	if err := commandPhase(ctx, 1); err != nil {
		return nil, err
	}
	message, err := generateGitCommitMessage(ctx, profile, project.Title, changes)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := commandPhase(ctx, 2); err != nil {
		return nil, err
	}
	input := gitcommit.Message{Title: message.Title, Items: message.Items}
	intent, err := svc.git.PrepareIntent(ctx, project.WorkDir, execution.AttemptID, changes, input)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(intent)
	if err != nil {
		return nil, err
	}
	if _, err := svc.store.AppendThreadEvent(ctx, execution.ThreadID, threadjournal.Event{Type: "commit.intent", CommandID: execution.CommandID, AttemptID: execution.AttemptID, Payload: raw}); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// After the durable intent, cancellation cannot hide a completed Git effect.
	commitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	result, err := svc.git.Commit(commitCtx, project.WorkDir, changes, input)
	if err != nil {
		if recovered, ok, reconcileErr := svc.git.Reconcile(commitCtx, project.WorkDir, intent); reconcileErr == nil && ok {
			result = recovered
			err = nil
		}
	}
	if err != nil {
		return nil, &uncertainCommitError{cause: err}
	}
	selected := llm.ExecutionOf(profile.Adapter())
	return model.GitCommitResult{ModelProfile: selected.Profile, FallbackUsed: selected.FallbackUsed, Title: message.Title, Items: message.Items, Branch: result.Branch, Hash: result.Hash, CommittedAt: result.CommittedAt, FilesChanged: result.FilesChanged, Insertions: result.Insertions, Deletions: result.Deletions}, nil
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
	policy := prompts.PublicPolicy("command.commit")
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
	requestCtx, cancel := context.WithTimeout(ctx, gitCommitModelTimeout)
	defer cancel()
	var lastErr error
	for attempt := 0; attempt < gitCommitModelAttempts; attempt++ {
		if err := requestCtx.Err(); err != nil {
			return generatedCommitMessage{}, err
		}
		response, err := profile.Adapter().Generate(requestCtx, llm.GenerateRequest{
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: llm.TextContent(policy)},
				{Role: llm.RoleUser, Content: llm.TextContent(user)},
			},
			Tools: []llm.ToolSchema{tool}, MaxOutputTokens: 1024,
		})
		if err != nil {
			lastErr = err
			if errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
				return generatedCommitMessage{}, context.DeadlineExceeded
			}
			return generatedCommitMessage{}, err
		}
		if err := requestCtx.Err(); err != nil {
			return generatedCommitMessage{}, err
		}
		message, validateErr := validateGitCommitResponse(response)
		if validateErr == nil {
			message.Title = model.PublicText(message.Title)
			for i := range message.Items {
				message.Items[i] = model.PublicText(message.Items[i])
			}
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

// Cancel acknowledges only after execution has stopped. Once the atomic commit
// boundary has begun, return its actual outcome instead of claiming cancellation.
