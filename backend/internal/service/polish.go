package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	polishprompts "github.com/dasi0227/PPT-Agent/backend/prompts/polish"
)

const maxPolishInstructionRunes = 4000
const maxPolishOutputRunes = 8000
const maxPolishOutputTokens = 1024
const polishTimeout = 12 * time.Second

type PolishParams struct {
	Instruction string
	ThreadID    string
	Scope       model.RunScope
	Mode        model.RunMode
	Model       string
}

type PolishResult struct {
	Instruction   string
	Changed       bool
	PromptVersion string
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
	instruction := strings.TrimSpace(params.Instruction)
	if instruction == "" || utf8.RuneCountInString(instruction) > maxPolishInstructionRunes {
		return PolishResult{}, model.NewAgentError("BAD_REQUEST", "polish_prompt", nil)
	}
	command := model.RunCommand{Scope: params.Scope, Mode: params.Mode, Instruction: instruction}
	if err := command.Validate(); err != nil {
		return PolishResult{}, model.NewAgentError("INVALID_SCOPE", "polish_prompt", err)
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
			return PolishResult{}, model.NewAgentError("BAD_REQUEST", "polish_prompt", errors.New("thread does not belong to project"))
		}
	}
	if svc.registry == nil {
		return PolishResult{}, model.NewAgentError("MODEL_PROFILE_NOT_FOUND", "polish_prompt", nil)
	}
	profile, err := svc.registry.Resolve(params.Model)
	if err != nil {
		return PolishResult{}, model.NewAgentError("MODEL_PROFILE_NOT_FOUND", "polish_prompt", nil)
	}
	pack, err := svc.assembler.AssemblePolish(ctx, contextengine.PolishContextRequest{
		ThreadID: params.ThreadID, Command: command,
	}, project)
	if err != nil {
		if errors.Is(err, model.ErrInvalidRunCommand) || errors.Is(err, contextengine.ErrRequiredMissing) {
			return PolishResult{}, model.NewAgentError("INVALID_SCOPE", "polish_prompt", err)
		}
		return PolishResult{}, err
	}
	prompt := polishprompts.Load()
	system, err := contextengine.CompilePolishContext(pack, prompt.Body)
	if err != nil {
		return PolishResult{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, polishTimeout)
	defer cancel()
	response, err := profile.Adapter().Generate(requestCtx, llm.GenerateRequest{Messages: []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent(system)},
		{Role: llm.RoleUser, Content: llm.TextContent(instruction)},
	}, Reasoning: llm.ReasoningDisabled, MaxOutputTokens: maxPolishOutputTokens})
	if err != nil {
		if errors.Is(err, context.Canceled) && errors.Is(ctx.Err(), context.Canceled) {
			return PolishResult{}, context.Canceled
		}
		if errors.Is(err, llm.ErrUnavailable) || errors.Is(err, context.DeadlineExceeded) || errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			return PolishResult{}, model.NewAgentError("PROVIDER_UNAVAILABLE", "polish_prompt", err)
		}
		return PolishResult{}, model.NewAgentError("AGENT_FAILED", "polish_prompt", err)
	}
	polished := strings.TrimSpace(response.Text())
	if polished == "" || utf8.RuneCountInString(polished) > maxPolishOutputRunes {
		return PolishResult{}, model.NewAgentError("POLISH_OUTPUT_INVALID", "polish_prompt", nil)
	}
	return PolishResult{
		Instruction: polished, Changed: polished != instruction, PromptVersion: prompt.Version,
	}, nil
}
