package workflow

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
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
	Phase           RunPhase
	Mode            model.RunMode
	Context         contextengine.ContextPack
	Plan            *Plan
	ContextBriefing string
	State           string
}

func runtimeSystemPrompt(phase RunPhase, mode model.RunMode, state string) string {
	return buildRuntimeSystemPrompt(runtimePromptInput{
		Phase: phase, Mode: mode, State: state,
	})
}

func runtimeSystemPromptForRequest(req AgentRequest, state string) string {
	if req.Mode == "" {
		req.Mode = req.Context.Command.Mode
	}
	return buildRuntimeSystemPrompt(runtimePromptInput{
		Phase: req.Phase, Mode: req.Mode,
		Context: req.Context, Plan: req.Plan, ContextBriefing: req.ContextBriefing, State: state,
	})
}

func buildRuntimeSystemPrompt(input runtimePromptInput) string {
	modules := []PromptModule{
		loadPromptModule("core_runtime_policy"),
		loadPromptModule("user_facing_output"),
		loadPromptModule(modePolicyID(input.Mode)),
		loadPromptModule(playbookID(input.Context)),
		loadPromptModule("completion_repair_guide"),
		loadPromptModule("finish_contract"),
		loadPromptModule("ppt_quality_rubric"),
		loadPromptModule("ppt_business_policy"),
		resourceContractsModule(),
	}
	if strings.TrimSpace(input.ContextBriefing) != "" {
		modules = append(modules, PromptModule{
			ID: "context_briefing", Version: runtimeprompts.Version, Body: input.ContextBriefing,
		})
	}
	if input.Mode == model.ModeExecute && input.Plan != nil && input.Plan.ApprovedRevision > 0 &&
		(input.Plan.Status == PlanActive || input.Plan.Status == PlanCompleted) {
		planJSON, _ := json.Marshal(struct {
			PlanID           string     `json:"plan_id"`
			ApprovedRevision int        `json:"approved_revision"`
			Title            string     `json:"title"`
			Content          string     `json:"content"`
			Steps            []PlanStep `json:"steps"`
		}{input.Plan.ID, input.Plan.ApprovedRevision, input.Plan.Title, input.Plan.Content, input.Plan.Steps})
		modules = append(modules, PromptModule{ID: "approved_plan", Version: runtimeprompts.Version, Body: "Approved plan: this exact structure is the authoritative execution contract.\n" + string(planJSON)})
	}
	modules = append(modules, PromptModule{
		ID: "runtime_state", Version: runtimeprompts.Version, Body: input.State,
	})

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
	case command.Scope.Artifact == model.ArtifactPPT && command.Scope.Level == model.ScopeDeck && len(pack.Outline.Outline.SlideOrder) == 0:
		return "playbook_empty_deck_generation"
	case command.Scope.Artifact == model.ArtifactPPT && command.Scope.Level == model.ScopeDeck:
		return "playbook_deck_coordinated_edit"
	default:
		return "playbook_default"
	}
}

func resourceContractsModule() PromptModule {
	module := loadPromptModule("resource_contracts")
	contracts := map[string]any{}
	for _, name := range []string{pptschema.OutlineName, pptschema.DesignName, pptschema.SlideSpecName} {
		if contract, err := pptschema.AgentContract(name); err == nil {
			contracts[name] = contract
		}
	}
	contractJSON, _ := json.Marshal(contracts)
	module.Body = strings.ReplaceAll(module.Body, "{{CONTRACTS_JSON}}", string(contractJSON))
	return module
}
