package service

import (
	"context"
	"crypto/sha256"
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
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

const gitCommitModelTimeout = 45 * time.Second

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
	release, ok := svc.locks.TryAcquire(project.ID)
	if !ok {
		return nil, ErrRunActive
	}
	defer release()
	return svc.executeLocked(ctx, execution, project, nil)
}

// executeLocked runs while either the command or its parent Run owns the project lock.
func (svc *GitCommitService) executeLocked(ctx context.Context, execution model.CommandExecution, project model.Project, supplied *generatedCommitMessage) (any, error) {
	var profile llm.Profile
	if supplied == nil {
		var err error
		profile, err = svc.registry.RoutedProfile("commit", "")
		if err != nil {
			return nil, err
		}
		if !profile.Capabilities().ToolCalls {
			return nil, ErrGitCommitToolUnsupported
		}
	}
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
	var message generatedCommitMessage
	if supplied != nil {
		message = *supplied
	} else {
		message, err = generateGitCommitMessage(ctx, profile, changes)
		if err != nil {
			return nil, err
		}
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
	if _, err := svc.store.AppendThreadEvent(ctx, execution.ThreadID, threadjournal.Event{RunID: execution.RunID, Type: "commit.intent", CommandID: execution.CommandID, AttemptID: execution.AttemptID, Payload: raw}); err != nil {
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
	value := model.GitCommitResult{Title: message.Title, Items: message.Items, Branch: result.Branch, Hash: result.Hash, CommittedAt: result.CommittedAt, FilesChanged: result.FilesChanged, Insertions: result.Insertions, Deletions: result.Deletions}
	if supplied == nil {
		selected := llm.ExecutionOf(profile.Adapter())
		value.ModelProfile, value.FallbackUsed = selected.Profile, selected.FallbackUsed
	}
	return value, nil
}

// ExecuteInRun is called only by the workflow that already owns the project lock.
// The durable command identity survives replay of the same model tool call.
func (svc *GitCommitService) ExecuteInRun(ctx context.Context, projectID, threadID, runID, callID string, args map[string]any) (map[string]any, error) {
	message, err := validateGitCommitResponse(llm.GenerateResponse{ToolCalls: []llm.ToolCall{{Name: "git_commit", Args: args}}})
	if err != nil {
		return nil, err
	}
	current, err := svc.store.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if current.ProjectID != projectID || current.ThreadID != threadID || current.Status != model.RunRunning || current.Command.Mode != model.ModeExecute || callID == "" {
		return nil, errors.New("Git commit requires the active execute Run")
	}
	project, err := svc.store.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	identity := fmt.Sprintf("agent-commit-%x", sha256.Sum256([]byte(runID+"\x00"+callID)))
	input, err := json.Marshal(map[string]any{"title": message.Title, "items": message.Items})
	if err != nil {
		return nil, err
	}
	execution, created, err := svc.store.AcceptCommand(ctx, threadID, model.CommandRequest{
		RequestKey: identity, CommandID: identity, Kind: "commit", Source: "automatic",
		RunID: runID, ToolCallID: callID, Input: input,
	}, 0)
	if err != nil {
		return nil, err
	}
	if !created {
		if execution.Status == "completed" {
			var result map[string]any
			if err := json.Unmarshal(execution.Result, &result); err != nil {
				return nil, err
			}
			return result, nil
		}
		return nil, errors.New("previous commit attempt did not complete with a confirmed result; do not retry with a new call ID")
	}
	result, executeErr := func() (any, error) {
		execution.Status = "running"
		if err := svc.store.SaveCommandExecution(ctx, execution); err != nil {
			return nil, err
		}
		progressCtx := WithCommandProgress(ctx, func(phase int) error {
			execution.Phase = phase
			return svc.store.SaveCommandExecution(ctx, execution)
		})
		return svc.executeLocked(progressCtx, execution, project, &message)
	}()
	execution.Status = "completed"
	if executeErr == nil {
		execution.Result, executeErr = json.Marshal(result)
	}
	if executeErr != nil {
		execution.Status = "failed"
		code := "COMMIT_FAILED"
		if errors.Is(executeErr, context.Canceled) {
			execution.Status = "canceled"
		}
		var uncertain *uncertainCommitError
		if errors.As(executeErr, &uncertain) {
			execution.Status, code = "interrupted", "COMMIT_UNCERTAIN"
		}
		execution.Error, _ = json.Marshal(map[string]any{"code": code, "message": executeErr.Error(), "retryable": false})
	}
	terminalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := svc.store.SaveCommandExecution(terminalCtx, execution); err != nil {
		return nil, fmt.Errorf("commit result could not be persisted; do not repeat the commit: %w", err)
	}
	if executeErr != nil {
		return nil, executeErr
	}
	var value map[string]any
	if err := json.Unmarshal(execution.Result, &value); err != nil {
		return nil, err
	}
	return value, nil
}

type generatedCommitMessage struct {
	Title string
	Items []string
}

func generateGitCommitMessage(
	ctx context.Context,
	profile llm.Profile,
	changes gitcommit.ChangeSet,
) (generatedCommitMessage, error) {
	policy := prompts.MustLoad("command.commit").Body
	// Project titles and conversation state are not evidence of a Git change.
	// Marshal only the staged evidence, never the executor's paths or identities.
	user, err := json.Marshal(struct {
		FileStatus     string `json:"file_status"`
		LineStatistics string `json:"line_statistics"`
		StagedDiff     string `json:"staged_diff"`
		DiffTruncated  bool   `json:"diff_truncated"`
	}{changes.NameStatus, changes.NumStat, changes.Diff, changes.DiffTruncated})
	if err != nil {
		return generatedCommitMessage{}, err
	}
	tool := llm.ToolSchema(workflow.GitCommitToolSchema())
	requestCtx, cancel := context.WithTimeout(ctx, gitCommitModelTimeout)
	defer cancel()
	response, err := profile.Adapter().Generate(requestCtx, llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: llm.TextContent(policy)},
			{Role: llm.RoleUser, Content: llm.TextContent(string(user))},
		},
		Tools: []llm.ToolSchema{tool}, MaxOutputTokens: 1024,
	})
	if requestCtx.Err() != nil {
		return generatedCommitMessage{}, requestCtx.Err()
	}
	if err != nil {
		return generatedCommitMessage{}, err
	}
	message, err := validateGitCommitResponse(response)
	if err != nil {
		return generatedCommitMessage{}, err
	}
	display := model.PublicTextContext{SourceText: changes.Diff}
	message.Title = model.PublicText(message.Title, display)
	for i := range message.Items {
		message.Items[i] = model.PublicText(message.Items[i], display)
	}
	return message, nil
}

func validateGitCommitResponse(response llm.GenerateResponse) (generatedCommitMessage, error) {
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "git_commit" {
		return generatedCommitMessage{}, errors.New("model must call git_commit exactly once")
	}
	if strings.TrimSpace(response.Text()) != "" {
		return generatedCommitMessage{}, errors.New("git_commit must return its result only through the tool")
	}
	call := response.ToolCalls[0]
	if len(call.Args) != 2 {
		return generatedCommitMessage{}, errors.New("git_commit requires only title and items")
	}
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
