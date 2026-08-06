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
	Phase           RuntimePhase
	Strategy        ExecutionStrategy
	ExecuteMode     ExecuteMode
	Context         contextengine.ContextPack
	ContextBriefing string
	State           string
}

func runtimeSystemPrompt(phase RuntimePhase, strategy ExecutionStrategy, state string) string {
	return buildRuntimeSystemPrompt(runtimePromptInput{
		Phase: phase, Strategy: strategy, State: state,
	})
}

func runtimeSystemPromptForRequest(req AgentRequest, state string) string {
	return buildRuntimeSystemPrompt(runtimePromptInput{
		Phase: req.Phase, Strategy: req.Strategy, ExecuteMode: req.ExecuteMode,
		Context: req.Context, ContextBriefing: req.ContextBriefing, State: state,
	})
}

func buildRuntimeSystemPrompt(input runtimePromptInput) string {
	modules := []PromptModule{
		loadPromptModule("core_runtime_policy"),
		loadPromptModule(modePolicyID(input.Strategy, input.ExecuteMode, input.Phase)),
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
	modules = append(modules, PromptModule{
		ID: "runtime_state", Version: runtimeprompts.Version, Body: input.State,
	})

	var b strings.Builder
	fmt.Fprintf(&b, "<runtime_prompt_manifest version=\"%s\" strategy=\"%s\" execute_mode=\"%s\" phase=\"%s\">\n",
		runtimeprompts.Version, input.Strategy, input.ExecuteMode, input.Phase)
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

func modePolicyID(strategy ExecutionStrategy, executeMode ExecuteMode, phase RuntimePhase) string {
	switch strategy {
	case StrategyTalk:
		return "mode_policy_talk"
	case StrategyAsk:
		return "mode_policy_ask"
	case StrategyPlan:
		return "mode_policy_plan"
	case StrategyExecute:
		if executeMode == ExecuteModePlanned || phase == PhasePlanning {
			return "mode_policy_execute_planned"
		}
		return "mode_policy_execute_direct"
	default:
		return "mode_policy_talk"
	}
}

func playbookID(pack contextengine.ContextPack) string {
	spec := pack.WorkSpec
	switch {
	case spec.Interaction.Intent == model.IntentPlan:
		return "playbook_read_only_planning"
	case spec.Interaction.Intent != model.IntentExecute:
		return "playbook_read_only_collaboration"
	case spec.Target.Artifact == model.ArtifactSpec:
		return "playbook_spec_edit"
	case spec.Target.Artifact == model.ArtifactPresentation && spec.Target.Level == model.TargetSlide:
		return "playbook_slide_presentation_edit"
	case spec.Target.Artifact == model.ArtifactPresentation && spec.Target.Level == model.TargetDeck && len(pack.Outline.Outline.SlideOrder) == 0:
		return "playbook_empty_deck_generation"
	case spec.Target.Artifact == model.ArtifactPresentation && spec.Target.Level == model.TargetDeck:
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
