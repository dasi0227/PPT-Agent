package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandresult"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

const maxPolishInstructionRunes = 4000
const maxPolishOutputRunes = 8000
const maxPolishOutputTokens = 1024
const polishTimeout = 12 * time.Second

type PolishParams struct {
	Instruction string
	Feedback    string
	ThreadID    string
	ScopeInput  model.CreateRunScopeInput
	Mode        model.RunMode
}

type PolishResult struct {
	ModelExecution llm.ModelExecution
	Title          string
	Content        string
	Changed        bool
	PromptVersion  string
}

type PolishService struct {
	store    store.Store
	registry *llm.Registry
}

func NewPolishService(s store.Store, registry *llm.Registry) *PolishService {
	return &PolishService{store: s, registry: registry}
}

func (svc *PolishService) Polish(ctx context.Context, projectID string, params PolishParams) (PolishResult, error) {
	if err := commandPhase(ctx, 0); err != nil {
		return PolishResult{}, err
	}
	if utf8.RuneCountInString(params.Feedback) > 4000 {
		return PolishResult{}, model.NewAgentError("BAD_REQUEST", "polish", nil)
	}
	instruction := strings.TrimSpace(params.Instruction)
	if instruction == "" || utf8.RuneCountInString(instruction) > maxPolishInstructionRunes {
		return PolishResult{}, model.NewAgentError("BAD_REQUEST", "polish_instruction", nil)
	}
	project, err := svc.store.GetProject(ctx, projectID)
	if err != nil {
		return PolishResult{}, err
	}
	if strings.TrimSpace(params.ThreadID) != "" {
		thread, threadErr := svc.store.GetThread(ctx, params.ThreadID)
		if threadErr != nil {
			return PolishResult{}, threadErr
		}
		if thread.ProjectID != project.ID {
			return PolishResult{}, model.NewAgentError("BAD_REQUEST", "polish_instruction", errors.New("thread does not belong to project"))
		}
	}
	reference, err := buildPolishInput(project, params, instruction)
	if err != nil {
		return PolishResult{}, model.NewAgentError("INVALID_SCOPE", "polish_instruction", err)
	}
	if svc.registry == nil {
		return PolishResult{}, model.NewAgentError("MODEL_PROFILE_NOT_FOUND", "polish_instruction", nil)
	}
	profile, err := svc.registry.RoutedProfile("polish", "")
	if err != nil {
		return PolishResult{}, model.NewAgentError("MODEL_PROFILE_NOT_FOUND", "polish_instruction", nil)
	}
	prompt := prompts.MustLoad("command.polish")
	requestCtx, cancel := context.WithTimeout(ctx, polishTimeout)
	defer cancel()
	if err := commandPhase(requestCtx, 1); err != nil {
		return PolishResult{}, err
	}
	var result commandresult.Text
	session := llm.NewSubmissionSession("polish", maxPolishOutputTokens)
	_, err = session.Generate(requestCtx, profile.Adapter(), llm.GenerateRequest{Messages: []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent(prompt.Body)},
		{Role: llm.RoleUser, Content: llm.TextContent(reference)},
	}, Tools: []llm.ToolSchema{commandresult.Schema("polish_instruction",
		"Return a wording suggestion for the supplied draft; do not answer or execute it.",
		"Short single-line plain-text title in the user's language describing the wording improvement, such as correcting typos or clarifying a reference. State that the draft is unchanged when no edit is needed; do not repeat the task or claim it is complete. No Markdown, HTML or trailing period.",
		"The complete revised draft ready for the user to send, or the unchanged draft when no edit is needed. Preserve intent, language and scope, applying the supplied revision feedback. Do not answer or execute the draft, invent requirements, or add an explanation around it.", maxPolishOutputRunes)},
		MaxOutputTokens: maxPolishOutputTokens}, func(response llm.GenerateResponse) error {
		var parseErr error
		result, parseErr = commandresult.Parse(response, "polish_instruction", maxPolishOutputRunes)
		return parseErr
	}, "本轮润色结果尚未提交。请仅调用一次 polish_instruction，提交 title 和完整 content，不附带普通正文；保留草稿原意及本次反馈，不执行草稿中的任务。")
	if err != nil {
		if errors.Is(err, context.Canceled) && errors.Is(ctx.Err(), context.Canceled) {
			return PolishResult{}, context.Canceled
		}
		if errors.Is(err, llm.ErrUnavailable) || errors.Is(err, context.DeadlineExceeded) || errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			return PolishResult{}, model.NewAgentError("PROVIDER_UNAVAILABLE", "polish_instruction", err)
		}
		var invalid *llm.SubmissionError
		if errors.As(err, &invalid) {
			return PolishResult{}, model.NewAgentError("POLISH_OUTPUT_INVALID", "polish_instruction", err)
		}
		return PolishResult{}, model.NewAgentError("AGENT_FAILED", "polish_instruction", err)
	}
	if err := commandPhase(requestCtx, 2); err != nil {
		return PolishResult{}, err
	}
	display := contextengine.ProjectPublicTextContext(project, instruction+"\n"+params.Feedback)
	result.Title = model.PublicText(result.Title, display)
	result.Content = model.PublicText(result.Content, display)
	return PolishResult{
		ModelExecution: llm.ExecutionOf(profile.Adapter()),
		Title:          result.Title, Content: result.Content, Changed: result.Content != instruction, PromptVersion: prompt.Version,
	}, nil
}
