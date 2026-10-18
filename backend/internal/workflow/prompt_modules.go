package workflow

import (
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
)

type PromptModule = prompts.Module

type runtimePromptInput struct {
	Phase   RunPhase
	Mode    model.RunMode
	Context contextengine.ContextPack
}

func effectivePromptMode(mode, commandMode model.RunMode) model.RunMode {
	if mode != "" {
		return mode
	}
	if commandMode != "" {
		return commandMode
	}
	return model.ModeChat
}

func runtimeSystemPromptForRequest(req AgentRequest) string {
	req.Mode = effectivePromptMode(req.Mode, req.Context.Command.Mode)
	return buildRuntimeSystemPrompt(runtimePromptInput{
		Phase: req.Phase, Mode: req.Mode, Context: req.Context,
	})
}

func buildRuntimeSystemPrompt(input runtimePromptInput) string {
	input.Mode = effectivePromptMode(input.Mode, input.Context.Command.Mode)
	// Use one effective mode for all static guidance.
	input.Context.Command.Mode = input.Mode
	modules := []PromptModule{
		loadPromptModule("core.agent"),
		loadPromptModule("core.references"),
		loadPromptModule("core.protocols"),
	}
	if input.Mode == model.ModeCreate {
		modules = append(modules, loadPromptModule("core.contracts"))
	}
	modules = append(modules, loadPromptModule(modePolicyID(input.Mode)))
	for _, id := range playbookIDs(input.Mode) {
		modules = append(modules, loadPromptModule(id))
	}
	if input.Mode == model.ModeCreate {
		modules = append(modules, loadPromptModule("rule.authoring"), loadPromptModule("rule.styling"))
	}

	var b strings.Builder
	for _, module := range modules {
		if rendered := module.SystemPrompt(); rendered != "" {
			b.WriteString(rendered)
			b.WriteByte('\n')
		}
	}
	return strings.TrimSpace(b.String())
}

func loadPromptModule(id string) PromptModule {
	return prompts.MustLoad(id)
}

func modePolicyID(mode model.RunMode) string {
	switch mode {
	case model.ModeChat, model.ModeAsk, model.ModePlan, model.ModeCreate:
		return "mode." + string(mode)
	default:
		panic(fmt.Sprintf("invalid prompt mode %q", mode))
	}
}

func playbookIDs(mode model.RunMode) []string {
	if mode != model.ModeCreate {
		return []string{"playbook.tool"}
	}
	// All create runs share the same capabilities. Scope changes page targets,
	// never the static authoring guidance.
	return []string{"playbook.tool", "playbook.spec", "playbook.html"}
}
