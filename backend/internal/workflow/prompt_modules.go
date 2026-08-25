package workflow

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	runtimeprompts "github.com/dasi0227/PPT-Agent/backend/prompts/runtime"
	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
)

type PromptModule struct {
	ID      string
	Version string
	Path    string
	Hash    string
	Body    string
}

type runtimePromptInput struct {
	Phase   RunPhase
	Mode    model.RunMode
	Context contextengine.ContextPack
}

func runtimeSystemPrompt(phase RunPhase, mode model.RunMode) string {
	return buildRuntimeSystemPrompt(runtimePromptInput{
		Phase: phase, Mode: mode,
	})
}

func runtimeSystemPromptForRequest(req AgentRequest) string {
	if req.Mode == "" {
		req.Mode = req.Context.Command.Mode
	}
	return buildRuntimeSystemPrompt(runtimePromptInput{
		Phase: req.Phase, Mode: req.Mode, Context: req.Context,
	})
}

func buildRuntimeSystemPrompt(input runtimePromptInput) string {
	modules := []PromptModule{
		loadPromptModule("core_runtime_policy"),
		loadPromptModule("user_facing_output"),
		loadPromptModule(modePolicyID(input.Mode)),
		loadPromptModule(playbookID(input.Context)),
	}
	switch input.Mode {
	case model.ModeTalk, model.ModeAsk:
		modules = append(modules, loadPromptModule("finish_contract"))
	case model.ModePlan:
		modules = append(modules, loadPromptModule("ppt_quality_rubric"))
		if module, ok := resourceContractsModule(input.Context); ok {
			modules = append(modules, module)
		}
	case model.ModeExecute:
		modules = append(modules,
			loadPromptModule("completion_repair_guide"),
			loadPromptModule("finish_contract"),
			loadPromptModule("ppt_quality_rubric"),
		)
		if module, ok := resourceContractsModule(input.Context); ok {
			modules = append(modules, module)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "<runtime_prompt_manifest version=\"%s\" mode=\"%s\" phase=\"%s\">\n",
		runtimeprompts.Version, input.Mode, input.Phase)
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
	mode := req.Mode
	if mode == "" {
		mode = req.Context.Command.Mode
	}
	state := struct {
		Mode            model.RunMode      `json:"mode"`
		Phase           RunPhase           `json:"phase"`
		ContextBriefing string             `json:"context_briefing,omitempty"`
		Plan            *Plan              `json:"plan,omitempty"`
		PlanAuthority   string             `json:"plan_authority,omitempty"`
		Changes         ChangeSet          `json:"changes"`
		Evidence        []Evidence         `json:"evidence"`
		Requirements    *RequirementLedger `json:"requirements,omitempty"`
	}{
		Mode: mode, Phase: req.Phase, ContextBriefing: req.ContextBriefing,
		Changes: req.Changes, Evidence: req.Evidence, Requirements: req.Requirements,
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
	module := runtimeprompts.MustLoad(id)
	return PromptModule{
		ID: module.ID, Version: module.Version, Path: module.Path,
		Hash: module.Hash, Body: module.Body,
	}
}

func modePolicyID(mode model.RunMode) string {
	switch mode {
	case model.ModeTalk:
		return "mode_policy_talk"
	case model.ModeAsk:
		return "mode_policy_ask"
	case model.ModePlan:
		return "mode_policy_plan"
	case model.ModeExecute:
		return "mode_policy_execute"
	default:
		return "mode_policy_talk"
	}
}

func playbookID(pack contextengine.ContextPack) string {
	command := pack.Command
	switch {
	case command.Mode == model.ModePlan:
		return "playbook_read_only_planning"
	case command.Mode != model.ModeExecute:
		return "playbook_read_only_collaboration"
	case command.Scope.Artifact == model.ArtifactSpec:
		return "playbook_spec_edit"
	case command.Scope.Artifact == model.ArtifactPPT && command.Scope.Level == model.ScopeSlide:
		return "playbook_slide_presentation_edit"
	case command.Scope.Artifact == model.ArtifactPPT && command.Scope.Level == model.ScopeDeck && len(spec.FlattenOutline(pack.Outline.Outline)) == 0:
		return "playbook_empty_deck_generation"
	case command.Scope.Artifact == model.ArtifactPPT && command.Scope.Level == model.ScopeDeck:
		return "playbook_deck_coordinated_edit"
	default:
		return "playbook_default"
	}
}

func resourceContractsModule(pack contextengine.ContextPack) (PromptModule, bool) {
	module := loadPromptModule("resource_contracts")
	contracts := map[string]any{}
	for _, name := range resourceContractNames(pack) {
		if contract, err := pptschema.AgentContract(name); err == nil {
			contracts[name] = contract
		}
	}
	if len(contracts) == 0 {
		return PromptModule{}, false
	}
	contractJSON, _ := json.Marshal(contracts)
	module.Body = strings.ReplaceAll(module.Body, "{{CONTRACTS_JSON}}", string(contractJSON))
	return module, true
}

func resourceContractNames(pack contextengine.ContextPack) []string {
	if pack.Command.Scope.Level == model.ScopeSlide {
		return []string{pptschema.SlideSpecName}
	}
	return []string{pptschema.DeckName, pptschema.OutlineName, pptschema.DesignName, pptschema.SlideSpecName}
}
