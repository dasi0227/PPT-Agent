package workflow

import (
	"encoding/json"
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

func runtimeSystemPrompt(phase RunPhase, mode model.RunMode) string {
	return buildRuntimeSystemPrompt(runtimePromptInput{
		Phase: phase, Mode: mode,
	})
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
	for _, id := range playbookIDs(input.Mode) {
		modules = append(modules, loadPromptModule(id))
	}
	switch input.Mode {
	case model.ModeChat, model.ModeGrill:
		modules = append(modules, loadPromptModule("runtime.completion"))
	case model.ModePlan:
		modules = append(modules, loadPromptModule("core.structure"))
	case model.ModeExecute:
		modules = append(modules,
			loadPromptModule("runtime.recovery"),
			loadPromptModule("runtime.completion"),
			loadPromptModule("runtime.execution"),
			loadPromptModule("core.structure"),
		)
	}
	if input.Mode == model.ModeChat || input.Mode == model.ModeGrill || input.Mode == model.ModeExecute {
		modules = append(modules, loadPromptModule("runtime.next-input-suggestions"))
	}
	if input.Mode == model.ModePlan || input.Mode == model.ModeExecute {
		modules = append(modules, loadPromptModule("core.html"))
	}

	var b strings.Builder
	for _, module := range modules {
		if strings.TrimSpace(module.Body) == "" {
			continue
		}
		fmt.Fprintf(&b, "<prompt_module id=%q>\n%s\n</prompt_module>\n", module.ID, strings.TrimSpace(module.Body))
	}
	return strings.TrimSpace(b.String())
}

func runtimeTaskStateForRequest(req AgentRequest) string {
	mode := effectivePromptMode(req.Mode, req.Context.Command.Mode)
	changes := make([]any, 0, req.Changes.Count())
	for _, change := range req.Changes.All() {
		changes = append(changes, map[string]any{"target": change.Artifact.Resource(), "artifact_hash": change.AfterHash})
	}
	evidence := []any{}
	// Only the latest evidence of each kind for a target informs the next action.
	seen := map[string]bool{}
	entries := promptEvidence(req.Evidence)
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		key := entry.Target.Key() + ":" + entry.Kind
		if seen[key] {
			continue
		}
		seen[key] = true
		evidence = append(evidence, map[string]any{"kind": entry.Kind, "target": entry.Target, "source_hash": entry.SourceHash, "fresh": entry.Fresh, "data": entry.Data})
	}
	state := map[string]any{
		"mode": mode, "phase": req.Phase, "context_briefing": req.ContextBriefing,
		"latest_rendered_images": req.RenderedImages, "plan": req.Plan, "plan_authority": nil,
		"changes": changes, "evidence": evidence, "requirements": req.Requirements, "work_ledger": nil,
	}
	if req.Work != nil {
		state["work_ledger"] = req.Work.Snapshot()
	}
	if req.Plan != nil {
		authority := "execution_progress_checklist"
		if mode == model.ModePlan {
			authority = "planning_proposal"
		} else if mode == model.ModeExecute && req.Plan.ApprovedContentHash != "" && (req.Plan.Status == PlanActive || req.Plan.Status == PlanCompleted) {
			authority = "approved_execution_contract"
		}
		state["plan_authority"] = authority
	}
	raw, _ := json.Marshal(contextengine.ModelValue(state))
	return string(raw)
}

// UI evidence retains screenshot links; the model discovers images exclusively
// through the current per-slide index, not accumulated evidence paths.
func promptEvidence(entries []Evidence) []Evidence {
	out := []Evidence{}
	for _, entry := range entries {
		if entry.Kind == "render" && !entry.Fresh {
			continue // Reference invalidation is internal; the gate requests a render when needed.
		}
		if entry.Kind != "render" && entry.Kind != "render_diagnostic" {
			out = append(out, entry)
			continue
		}
		data := make(map[string]any, len(entry.Data))
		for key, value := range entry.Data {
			if key != "screenshot_ref" && key != "screenshot_url" && key != "image_path" {
				data[key] = value
			}
		}
		entry.Data = data
		out = append(out, entry)
	}
	return out
}

func loadPromptModule(id string) PromptModule {
	module := prompts.MustLoad(id)
	if id == "core.html" {
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
	return PromptModule{
		ID: module.ID, Version: module.Version, Path: module.Path,
		Hash: module.Hash, Body: module.Body,
	}
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
	return []string{"playbook.deck", "playbook.spec", "playbook.slide"}
}
