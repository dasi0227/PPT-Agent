package workflow

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
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
		loadPromptModule(modePolicyID(input.Mode)),
	}
	if id := playbookID(input.Context); id != "" {
		modules = append(modules, loadPromptModule(id))
	}
	switch input.Mode {
	case model.ModeChat, model.ModeGrill:
		modules = append(modules, loadPromptModule("runtime.completion"))
	case model.ModePlan:
		modules = append(modules, loadPromptModule("core.quality"))
		if module, ok := resourceContractsModule(input.Context); ok {
			modules = append(modules, module)
		}
	case model.ModeExecute:
		modules = append(modules,
			loadPromptModule("runtime.recovery"),
			loadPromptModule("runtime.completion"),
			loadPromptModule("core.quality"),
		)
		if module, ok := resourceContractsModule(input.Context); ok {
			modules = append(modules, module)
		}
	}
	if (input.Mode == model.ModePlan || input.Mode == model.ModeExecute) && input.Context.Command.Scope.AllowsHTML() {
		modules = append(modules, loadPromptModule("core.html"))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "<runtime_prompt_manifest version=\"%s\" mode=\"%s\" phase=\"%s\">\n",
		prompts.Version, input.Mode, input.Phase)
	for _, module := range modules {
		if strings.TrimSpace(module.Body) == "" {
			continue
		}
		fmt.Fprintf(&b, "<prompt_module id=\"%s\" version=\"%s\"", module.ID, module.Version)
		if module.Path != "" {
			fmt.Fprintf(&b, " path=\"%s\"", module.Path)
		}
		if module.Hash != "" {
			fmt.Fprintf(&b, " hash=\"%s\"", module.Hash)
		}
		fmt.Fprintf(&b, ">\n%s\n</prompt_module>\n", strings.TrimSpace(module.Body))
	}
	b.WriteString("</runtime_prompt_manifest>")
	return b.String()
}

func runtimeTaskStateForRequest(req AgentRequest) string {
	mode := effectivePromptMode(req.Mode, req.Context.Command.Mode)
	state := struct {
		Mode            model.RunMode      `json:"mode"`
		Phase           RunPhase           `json:"phase"`
		ContextBriefing string             `json:"context_briefing,omitempty"`
		Plan            *Plan              `json:"plan,omitempty"`
		PlanAuthority   string             `json:"plan_authority,omitempty"`
		Changes         ChangeSet          `json:"changes"`
		Evidence        []Evidence         `json:"evidence"`
		Requirements    *RequirementLedger `json:"requirements,omitempty"`
		Work            []SlideWorkItem    `json:"work_ledger,omitempty"`
	}{
		Mode: mode, Phase: req.Phase, ContextBriefing: req.ContextBriefing,
		Changes: req.Changes, Evidence: req.Evidence, Requirements: req.Requirements,
		Work: func() []SlideWorkItem {
			if req.Work == nil {
				return nil
			}
			return req.Work.Snapshot()
		}(),
	}
	if req.Plan != nil {
		state.Plan = req.Plan
		switch {
		case mode == model.ModeExecute && req.Plan.ApprovedRevision > 0 &&
			(req.Plan.Status == PlanActive || req.Plan.Status == PlanCompleted):
			state.PlanAuthority = "approved_execution_contract"
		case mode == model.ModePlan:
			state.PlanAuthority = "planning_proposal"
		default:
			state.PlanAuthority = "execution_progress_checklist"
		}
	}
	raw, _ := json.Marshal(state)
	return string(raw)
}

func loadPromptModule(id string) PromptModule {
	module := prompts.MustLoad(id)
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

func playbookID(pack contextengine.ContextPack) string {
	if pack.Command.Mode != model.ModeExecute {
		return ""
	}
	switch {
	case pack.Command.Scope.AllowsGlobal():
		return "playbook.deck"
	case pack.Command.Scope.Object == model.ScopeObjectSpec:
		return "playbook.spec"
	case pack.Command.Scope.AllowsHTML() && pack.Command.Scope.IsSinglePage():
		return "playbook.slide"
	case pack.Command.Scope.AllowsHTML():
		return "playbook.deck"
	default:
		return ""
	}
}

func resourceContractsModule(pack contextengine.ContextPack) (PromptModule, bool) {
	module := loadPromptModule("core.structure")
	contracts := map[string]any{}
	for _, name := range resourceContractNames(pack) {
		contract, err := pptschema.AgentContract(name)
		if err != nil {
			panic(err)
		}
		contracts[name] = contract
	}
	contractJSON, _ := json.Marshal(contracts)
	var err error
	module, err = module.WithBody(strings.ReplaceAll(module.Body, "{{CONTRACTS_JSON}}", string(contractJSON)))
	if err != nil {
		panic(err)
	}
	return module, true
}

func resourceContractNames(pack contextengine.ContextPack) []string {
	if pack.Command.Scope.AllowsGlobal() {
		return []string{pptschema.ManifestName, pptschema.OutlineName, pptschema.DesignName, pptschema.SlideSpecName}
	}
	if pack.Command.Scope.AllowsSpec() {
		return []string{pptschema.SlideSpecName}
	}
	return nil
}
