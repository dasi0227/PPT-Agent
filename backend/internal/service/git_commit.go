package service

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/dasi0227/PPT-Agent/backend/internal/tooldefinition"
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

func commitPersistenceError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return model.NewAgentError(model.CodeRunCanceled, "git_commit", err)
	}
	return model.AsAgentError(err, model.CodePersistenceFailed, "git_commit")
}

func commitExecutionError(err error) error {
	if errors.Is(err, context.Canceled) {
		return model.NewAgentError(model.CodeRunCanceled, "git_commit", err)
	}
	return model.AsAgentError(err, "GIT_COMMIT_FAILED", "git_commit")
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
	profile, err := svc.registry.RoutedProfile("commit", "")
	if err != nil {
		return nil, err
	}
	if !profile.Capabilities().ToolCalls {
		return nil, ErrGitCommitToolUnsupported
	}
	if err := svc.git.Bootstrap(ctx, project.WorkDir); err != nil {
		return nil, commitExecutionError(err)
	}
	if err := commandPhase(ctx, 0); err != nil {
		return nil, commitPersistenceError(err)
	}
	changes, cleanup, err := svc.git.StageAll(ctx, project.WorkDir, execution.AttemptID)
	if err != nil {
		return nil, commitExecutionError(err)
	}
	defer cleanup()
	if changes.FilesChanged == 0 {
		return map[string]any{"empty": true}, nil
	}
	if err := commandPhase(ctx, 1); err != nil {
		return nil, commitPersistenceError(err)
	}
	message, err := generateGitCommitMessage(ctx, profile, changes)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := commandPhase(ctx, 2); err != nil {
		return nil, commitPersistenceError(err)
	}
	input := gitcommit.Message{Title: message.Title, Items: message.Items}
	intent, err := svc.git.PrepareIntent(ctx, project.WorkDir, execution.AttemptID, changes, input)
	if err != nil {
		return nil, commitExecutionError(err)
	}
	raw, err := json.Marshal(intent)
	if err != nil {
		return nil, model.NewAgentError(model.CodeInternal, "git_commit", err)
	}
	if _, err := svc.store.AppendThreadEvent(ctx, execution.ThreadID, threadjournal.Event{Type: "commit.intent", CommandID: execution.CommandID, AttemptID: execution.AttemptID, Payload: raw}); err != nil {
		return nil, commitPersistenceError(err)
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
		return nil, model.NewAgentError("GIT_COMMIT_FAILED", "git_commit", &uncertainCommitError{cause: err})
	}
	value := model.GitCommitResult{Title: message.Title, Items: message.Items, Branch: result.Branch, Hash: result.Hash, CommittedAt: result.CommittedAt, FilesChanged: result.FilesChanged, Insertions: result.Insertions, Deletions: result.Deletions}
	selected := llm.ExecutionOf(profile.Adapter())
	value.ModelProfile, value.FallbackUsed = selected.Profile, selected.FallbackUsed
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
	}{changes.NameStatus, changes.NumStat, changes.Diff})
	if err != nil {
		return generatedCommitMessage{}, err
	}
	tool := tooldefinition.Get("git_commit")
	requestCtx, cancel := context.WithTimeout(ctx, gitCommitModelTimeout)
	defer cancel()
	var message generatedCommitMessage
	session := llm.NewSubmissionSession("commit")
	_, err = session.Generate(requestCtx, profile.Adapter(), llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: llm.TextContent(policy)},
			{Role: llm.RoleUser, Content: llm.TextContent(string(user))},
		},
		Tools: []llm.ToolSchema{tool},
	}, func(response llm.GenerateResponse) error {
		var parseErr error
		message, parseErr = validateGitCommitResponse(response)
		return parseErr
	}, "本轮提交信息尚未接受。请仅调用一次 git_commit，提交合法 title 和 1–6 项 items，不附带正文；仅概括所给 diff。被拒绝的调用不会执行 Git，只需修正提交信息。")
	if requestCtx.Err() != nil {
		return generatedCommitMessage{}, requestCtx.Err()
	}
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
	if err := llm.ValidateSubmissionEnvelope(response, "git_commit", true); err != nil {
		return generatedCommitMessage{}, err
	}
	call := response.ToolCalls[0]
	if len(call.Args) != 2 {
		return generatedCommitMessage{}, llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/", "git_commit requires only title and items. Supply title as single-line plain text (1–72 characters) and items as 1–6 non-empty plain-text strings (each at most 160 characters); no other fields.")
	}
	title, ok := call.Args["title"].(string)
	if !ok {
		return generatedCommitMessage{}, llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/title", "commit title is required. Supply title as single-line plain text (1–72 characters) and items as 1–6 non-empty plain-text strings (each at most 160 characters); no other fields.")
	}
	title = strings.TrimSpace(title)
	if !validCommitText(title, 72) || strings.Contains(title, "\n") {
		return generatedCommitMessage{}, llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/title", "commit title is invalid. Supply title as single-line plain text (1–72 characters) and items as 1–6 non-empty plain-text strings (each at most 160 characters); no other fields.")
	}
	rawItems, ok := call.Args["items"].([]any)
	if !ok {
		if stringsItems, stringsOK := call.Args["items"].([]string); stringsOK {
			rawItems = make([]any, len(stringsItems))
			for i := range stringsItems {
				rawItems[i] = stringsItems[i]
			}
		} else {
			return generatedCommitMessage{}, llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/items", "commit items are required. Supply title as single-line plain text (1–72 characters) and items as 1–6 non-empty plain-text strings (each at most 160 characters); no other fields.")
		}
	}
	if len(rawItems) < 1 || len(rawItems) > 6 {
		return generatedCommitMessage{}, llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/items", "commit items count is invalid. Supply title as single-line plain text (1–72 characters) and items as 1–6 non-empty plain-text strings (each at most 160 characters); no other fields.")
	}
	seen := map[string]bool{}
	items := make([]string, 0, len(rawItems))
	for _, raw := range rawItems {
		item, ok := raw.(string)
		if !ok {
			return generatedCommitMessage{}, llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/items", "commit item must be text. Supply title as single-line plain text (1–72 characters) and items as 1–6 non-empty plain-text strings (each at most 160 characters); no other fields.")
		}
		item = strings.TrimSpace(strings.TrimPrefix(item, "-"))
		if !validCommitText(item, 160) {
			return generatedCommitMessage{}, llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/items", "commit item is invalid. Supply title as single-line plain text (1–72 characters) and items as 1–6 non-empty plain-text strings (each at most 160 characters); no other fields.")
		}
		if !seen[item] {
			seen[item] = true
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return generatedCommitMessage{}, llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/items", "commit items are empty. Supply title as single-line plain text (1–72 characters) and items as 1–6 non-empty plain-text strings (each at most 160 characters); no other fields.")
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
