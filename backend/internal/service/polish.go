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
	store     store.Store
	registry  *llm.Registry
	assembler *contextengine.ContextAssembler
}

func NewPolishService(s store.Store, registry *llm.Registry) *PolishService {
	return &PolishService{
		store: s, registry: registry,
		assembler: contextengine.NewContextAssembler(s, contextengine.NewRefRegistry()),
	}
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
	snapshot, err := NewPPTMutationService(svc.store).Snapshot(ctx, project.ID)
	if err != nil {
		return PolishResult{}, err
	}
	scope, err := resolveRunScope(snapshot, params.ScopeInput)
	if err != nil {
		return PolishResult{}, model.NewAgentError("INVALID_SCOPE", "polish_instruction", err)
	}
	command := model.RunCommand{Scope: scope, Mode: params.Mode, Instruction: instruction}
	if err := command.Validate(); err != nil {
		return PolishResult{}, model.NewAgentError("INVALID_SCOPE", "polish_instruction", err)
	}
	if svc.registry == nil {
		return PolishResult{}, model.NewAgentError("MODEL_PROFILE_NOT_FOUND", "polish_instruction", nil)
	}
	profile, err := svc.registry.RoutedProfile("polish", "")
	if err != nil {
		return PolishResult{}, model.NewAgentError("MODEL_PROFILE_NOT_FOUND", "polish_instruction", nil)
	}
	pack, err := svc.assembler.AssemblePolish(ctx, contextengine.PolishContextRequest{
		ThreadID: params.ThreadID, Command: command,
	}, project)
	if err != nil {
		if errors.Is(err, model.ErrInvalidRunCommand) || errors.Is(err, contextengine.ErrRequiredMissing) {
			return PolishResult{}, model.NewAgentError("INVALID_SCOPE", "polish_instruction", err)
		}
		return PolishResult{}, err
	}
	prompt := prompts.MustLoad("command.polish")
	reference, err := contextengine.CompilePolishContext(pack)
	if err != nil {
		return PolishResult{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, polishTimeout)
	defer cancel()
	if err := commandPhase(requestCtx, 1); err != nil {
		return PolishResult{}, err
	}
	if params.Feedback != "" {
		reference += "\n\n<revision_feedback>\n" + params.Feedback + "\n</revision_feedback>"
	}
	response, err := profile.Adapter().Generate(requestCtx, llm.GenerateRequest{Messages: []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent(prompt.Body)},
		{Role: llm.RoleUser, Content: llm.TextContent(reference + "\n\n" + instruction)},
	}, Tools: []llm.ToolSchema{commandresult.Schema("polish_instruction",
		"Submit the refined instruction without executing it or changing the composer.",
		"A short summary of the wording improvements, not a claim of completed project work.",
		"The complete refined instruction as plain text, ready to send to the PPT creation Agent.", maxPolishOutputRunes)},
		MaxOutputTokens: maxPolishOutputTokens})
	if err != nil {
		if errors.Is(err, context.Canceled) && errors.Is(ctx.Err(), context.Canceled) {
			return PolishResult{}, context.Canceled
		}
		if errors.Is(err, llm.ErrUnavailable) || errors.Is(err, context.DeadlineExceeded) || errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			return PolishResult{}, model.NewAgentError("PROVIDER_UNAVAILABLE", "polish_instruction", err)
		}
		return PolishResult{}, model.NewAgentError("AGENT_FAILED", "polish_instruction", err)
	}
	if err := commandPhase(requestCtx, 2); err != nil {
		return PolishResult{}, err
	}
	result, err := commandresult.Parse(response, "polish_instruction", maxPolishOutputRunes)
	if err != nil {
		return PolishResult{}, model.NewAgentError("POLISH_OUTPUT_INVALID", "polish_instruction", err)
	}
	return PolishResult{
		ModelExecution: llm.ExecutionOf(profile.Adapter()),
		Title:          result.Title, Content: result.Content, Changed: result.Content != instruction, PromptVersion: prompt.Version,
	}, nil
}
