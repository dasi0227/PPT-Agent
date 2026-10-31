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
		loadPromptModule("core_agent"),
		loadPromptModule("core_references"),
		loadPromptModule("core_protocols"),
		loadPromptModule("core_contracts"),
	}
	modules = append(modules, loadPromptModule(modePolicyID(input.Mode)))
	for _, id := range playbookIDs(input.Mode) {
		modules = append(modules, loadPromptModule(id))
	}
	modules = append(modules, loadPromptModule("rule_authoring"), loadPromptModule("rule_styling"))

	groups := []string{prompts.RenderGroup("system_prompt", modules...)}
	if ids := additionIDs(input.Mode); len(ids) > 0 {
		additions := make([]PromptModule, 0, len(ids))
		for _, id := range ids {
			additions = append(additions, loadPromptModule(id))
		}
		groups = append(groups, prompts.RenderGroup("fallible_point", additions...))
	}
	return strings.Join(groups, "\n\n")
}

func loadPromptModule(id string) PromptModule {
	return prompts.MustLoad(id)
}

func modePolicyID(mode model.RunMode) string {
	switch mode {
	case model.ModeChat, model.ModeAsk, model.ModePlan, model.ModeCreate:
		return "mode_" + string(mode)
	default:
		panic(fmt.Sprintf("invalid prompt mode %q", mode))
	}
}

func playbookIDs(mode model.RunMode) []string {
	if mode != model.ModeCreate {
		return []string{"playbook_tool"}
	}
	// All create runs share the same capabilities. Scope changes page targets,
	// never the static authoring guidance.
	return []string{"playbook_tool", "playbook_spec", "playbook_html"}
}

func additionIDs(mode model.RunMode) []string {
	if mode == model.ModePlan || mode == model.ModeCreate {
		return []string{"requirements_prohibitions_boundaries"}
	}
	return nil
}
