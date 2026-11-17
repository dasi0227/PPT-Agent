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
	// Share static guidance across modes; mode policies and tool permissions control execution.
	input.Context.Command.Mode = input.Mode
	modules := []PromptModule{
		loadPromptModule("core_agent"),
		loadPromptModule("core_references"),
		loadPromptModule("core_protocols"),
		loadPromptModule("core_contracts"),
		loadPromptModule("playbook_tool"),
		loadPromptModule("playbook_spec"),
		loadPromptModule("playbook_html"),
		loadPromptModule("rule_authoring"),
		loadPromptModule("rule_styling"),
		loadPromptModule("rule_commentating"),
	}
	// Keep the varying mode after the shared modules to preserve their common prefix.
	modules = append(modules, loadPromptModule(modePolicyID(input.Mode)))

	groups := []string{
		prompts.RenderGroup("system_prompt", modules...),
		prompts.RenderGroup("reminder", loadPromptModule("reminder_constraint_boundaries")),
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
