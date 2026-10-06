package workflow

import (
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
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
	// Use one effective mode for both the policy and the task playbook.
	input.Context.Command.Mode = input.Mode
	modules := []PromptModule{
		loadPromptModule("core.agent"),
		loadPromptModule("core.output"),
		loadPromptModule("core.reference"),
		loadPromptModule("core.quality"),
		loadPromptModule(modePolicyID(input.Mode)),
	}
	switch input.Mode {
	case model.ModeChat, model.ModeGrill:
		modules = append(modules, loadPromptModule("workflow.completion"))
	case model.ModeExecute:
		modules = append(modules,
			loadPromptModule("workflow.execution"),
			loadPromptModule("workflow.recovery"),
			loadPromptModule("workflow.completion"),
		)
	}
	for _, id := range playbookIDs(input.Mode) {
		modules = append(modules, loadPromptModule(id))
	}
	if input.Mode == model.ModePlan || input.Mode == model.ModeExecute {
		modules = append(modules, loadPromptModule("playbook.html"))
	}

	modules = append(modules, loadPromptModule("playbook.runtime"))

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
	module := prompts.MustLoad(id)
	if id == "playbook.html" {
		body := module.Body
		for _, name := range []string{"cover", "content", "chart"} {
			example, err := runtimeassets.Example(name)
			if err != nil {
				panic(err)
			}
			body += "\n\nShared composition reference (" + name + "):\n```html\n" + string(example) + "\n```"
		}
		module, _ = module.WithBody(body)
	}
	return module
}

func modePolicyID(mode model.RunMode) string {
	switch mode {
	case model.ModeChat, model.ModeGrill, model.ModePlan, model.ModeExecute:
		return "mode." + string(mode)
	default:
		panic(fmt.Sprintf("invalid prompt mode %q", mode))
	}
}

func playbookIDs(mode model.RunMode) []string {
	if mode != model.ModeExecute {
		return nil
	}
	// All execute runs share the same capabilities. Scope changes page targets,
	// never the static policy; each playbook owns a different authoring decision.
	return []string{"playbook.init", "playbook.improve"}
}
