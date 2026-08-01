package workflow

import (
	"strings"
	"unicode"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type ExecutionStrategy string

const (
	StrategyRespond         ExecutionStrategy = "respond"
	StrategyDirectAction    ExecutionStrategy = "direct_action"
	StrategyCompactWorkflow ExecutionStrategy = "compact_workflow"
	StrategyFullPEV         ExecutionStrategy = "full_pev"
)

type RiskLevel string

const (
	RiskLevelLow    RiskLevel = "low"
	RiskLevelMedium RiskLevel = "medium"
	RiskLevelHigh   RiskLevel = "high"
)

type ComplexityLevel string

const (
	ComplexityLow    ComplexityLevel = "low"
	ComplexityMedium ComplexityLevel = "medium"
	ComplexityHigh   ComplexityLevel = "high"
)

type DecisionSignal struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type StrategyDecision struct {
	Strategy   ExecutionStrategy `json:"strategy"`
	Reason     string            `json:"reason"`
	Risk       RiskLevel         `json:"risk"`
	Complexity ComplexityLevel   `json:"complexity"`
	Signals    []DecisionSignal  `json:"signals"`
}

type ExecutionStrategyRouter struct{}

var (
	fullPEVTerms = []string{
		"新增", "添加页面", "删除", "移除页面", "重排", "重新排序", "调整顺序", "批量", "多页",
		"所有页面", "整份", "整套", "全局", "整体", "章节", "section", "subsection",
		"add slide", "delete slide", "remove slide", "reorder", "batch", "multiple slides",
		"all slides", "global", "whole deck",
	}
	structureTerms = []string{
		"结构", "布局", "重建", "重新生成", "竞品对比", "流程图", "图表", "挂载", "组件", "资产",
		"structure", "layout", "rebuild", "regenerate", "comparison", "mount", "component", "asset",
	}
	blueprintFieldTerms = []string{
		"标题", "title", "key_message", "key message", "核心句", "要点", "summary", "role",
	}
	presentationPatchTerms = []string{
		"替换文本", "修改文本", "文案", "字号", "颜色", "css token", "token", "dom", "锚点",
		"replace text", "change text", "font size", "color",
	}
)

func (ExecutionStrategyRouter) Decide(pack contextengine.ContextPack) StrategyDecision {
	spec := pack.WorkSpec
	signals := []DecisionSignal{
		{Name: "target", Value: string(spec.Target.Artifact) + "/" + string(spec.Target.Level)},
		{Name: "intent", Value: string(spec.Interaction.Intent)},
	}
	if spec.Interaction.Intent == model.IntentConsult {
		return StrategyDecision{
			Strategy: StrategyRespond, Reason: "consult intent is read-only",
			Risk: RiskLevelLow, Complexity: ComplexityLow, Signals: signals,
		}
	}
	instruction := normalizeInstruction(spec.Instruction)
	if term := firstTerm(instruction, fullPEVTerms); term != "" {
		signals = append(signals, DecisionSignal{Name: "high_impact_term", Value: term})
		return StrategyDecision{
			Strategy: StrategyFullPEV, Reason: "instruction requests multi-artifact or high-impact change",
			Risk: RiskLevelHigh, Complexity: ComplexityHigh, Signals: signals,
		}
	}
	if spec.Target.Level == model.TargetDeck {
		signals = append(signals, DecisionSignal{Name: "deck_scope", Value: "true"})
		return StrategyDecision{
			Strategy: StrategyFullPEV, Reason: "deck-level work requires cross-artifact planning and verification",
			Risk: RiskLevelHigh, Complexity: ComplexityHigh, Signals: signals,
		}
	}
	if spec.Target.Artifact == model.ArtifactBlueprint {
		if term := firstTerm(instruction, structureTerms); term != "" {
			signals = append(signals, DecisionSignal{Name: "structure_term", Value: term})
			return StrategyDecision{
				Strategy: StrategyCompactWorkflow, Reason: "single-slide blueprint structure change needs coordinated steps",
				Risk: RiskLevelMedium, Complexity: ComplexityMedium, Signals: signals,
			}
		}
		if term := firstTerm(instruction, blueprintFieldTerms); term != "" || instructionIsShort(instruction) {
			signals = append(signals, DecisionSignal{Name: "single_field_signal", Value: firstValue(term, "short explicit instruction")})
			return StrategyDecision{
				Strategy: StrategyDirectAction, Reason: "single blueprint slide field update",
				Risk: RiskLevelLow, Complexity: ComplexityLow, Signals: signals,
			}
		}
		return StrategyDecision{
			Strategy: StrategyCompactWorkflow, Reason: "single blueprint slide requires more than one semantic decision",
			Risk: RiskLevelMedium, Complexity: ComplexityMedium, Signals: signals,
		}
	}
	state := model.MaterializationUnknown
	if pack.Target.Materialization != nil {
		state = model.MaterializationState(pack.Target.Materialization.State)
	}
	signals = append(signals, DecisionSignal{Name: "materialization_state", Value: string(state)})
	switch state {
	case model.MaterializationNotMaterialized:
		return StrategyDecision{
			Strategy: StrategyCompactWorkflow, Reason: "first materialization requires build and verification steps",
			Risk: RiskLevelMedium, Complexity: ComplexityMedium, Signals: signals,
		}
	case model.MaterializationBlueprintStale, model.MaterializationDesignStale, model.MaterializationUnknown:
		return StrategyDecision{
			Strategy: StrategyCompactWorkflow, Reason: "stale or unknown presentation must be rebuilt from canonical inputs",
			Risk: RiskLevelMedium, Complexity: ComplexityMedium, Signals: signals,
		}
	}
	if term := firstTerm(instruction, structureTerms); term != "" {
		signals = append(signals, DecisionSignal{Name: "structure_or_asset_term", Value: term})
		return StrategyDecision{
			Strategy: StrategyCompactWorkflow, Reason: "single-slide structure or asset work needs a lightweight workflow",
			Risk: RiskLevelMedium, Complexity: ComplexityMedium, Signals: signals,
		}
	}
	if term := firstTerm(instruction, presentationPatchTerms); term != "" || instructionIsShort(instruction) {
		signals = append(signals, DecisionSignal{Name: "anchored_patch_signal", Value: firstValue(term, "short explicit instruction")})
		return StrategyDecision{
			Strategy: StrategyDirectAction, Reason: "explicit local presentation patch",
			Risk: RiskLevelLow, Complexity: ComplexityLow, Signals: signals,
		}
	}
	return StrategyDecision{
		Strategy: StrategyCompactWorkflow, Reason: "ambiguous single-slide change needs lightweight planning",
		Risk: RiskLevelMedium, Complexity: ComplexityMedium,
		Signals: append(signals, DecisionSignal{Name: "semantic_ambiguity", Value: "true"}),
	}
}

func StrategyCapabilities(strategy ExecutionStrategy) map[Capability]bool {
	switch strategy {
	case StrategyRespond:
		return capabilitySet(
			CapabilityReadContext, CapabilityReadBlueprint, CapabilityReadDesignSpec,
			CapabilityReadPresentation, CapabilityReadAssets, CapabilitySearchAssets, CapabilityControl,
		)
	case StrategyDirectAction:
		return capabilitySet(
			CapabilityReadContext, CapabilityReadBlueprint, CapabilityWriteBlueprint,
			CapabilityReadDesignSpec, CapabilityWriteDesignSpec,
			CapabilityReadPresentation, CapabilityWritePresentation,
			CapabilityValidateBlueprint, CapabilityValidatePresentation, CapabilityControl,
		)
	case StrategyCompactWorkflow:
		return presentationCapabilitiesWithBlueprint()
	case StrategyFullPEV:
		return presentationCapabilitiesWithBlueprint()
	default:
		return map[Capability]bool{}
	}
}

func presentationCapabilitiesWithBlueprint() map[Capability]bool {
	return capabilitySet(
		CapabilityReadContext, CapabilityReadBlueprint, CapabilityWriteBlueprint,
		CapabilityReadDesignSpec, CapabilityWriteDesignSpec,
		CapabilityReadPresentation, CapabilityWritePresentation,
		CapabilitySearchAssets, CapabilityReadAssets, CapabilityMountAssets,
		CapabilityRenderPreview, CapabilityValidateBlueprint,
		CapabilityValidatePresentation, CapabilityControl,
	)
}

func normalizeInstruction(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func firstTerm(instruction string, terms []string) string {
	for _, term := range terms {
		if strings.Contains(instruction, strings.ToLower(term)) {
			return term
		}
	}
	return ""
}

func instructionIsShort(instruction string) bool {
	count := 0
	for _, r := range instruction {
		if !unicode.IsSpace(r) {
			count++
		}
	}
	return count > 0 && count <= 32
}
