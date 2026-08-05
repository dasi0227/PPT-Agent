package workflow

import (
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

type DecisionSignal struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type StrategyDecision struct {
	Strategy    ExecutionStrategy `json:"strategy"`
	ExecuteMode ExecuteMode       `json:"execute_mode,omitempty"`
	Reason      string            `json:"reason"`
	Confidence  float64           `json:"confidence"`
	Risk        RiskLevel         `json:"risk"`
	Signals     []DecisionSignal  `json:"signals"`
}

type StrategyRouter struct{}

func (StrategyRouter) Decide(pack contextengine.ContextPack) StrategyDecision {
	spec := pack.WorkSpec
	signals := []DecisionSignal{
		{Name: "interaction", Value: string(spec.Interaction.Intent)},
		{Name: "target", Value: string(spec.Target.Artifact) + "/" + string(spec.Target.Level)},
	}
	if spec.Interaction.Intent == model.IntentTalk {
		signals = append(signals, DecisionSignal{Name: "read_only_authorization", Value: "true"})
		return StrategyDecision{
			Strategy: StrategyTalk, Reason: "talk interaction explicitly authorizes read-only collaboration",
			Confidence: 1, Risk: RiskLow, Signals: signals,
		}
	}
	if spec.Interaction.Intent == model.IntentAsk {
		signals = append(signals, DecisionSignal{Name: "read_only_authorization", Value: "true"})
		return StrategyDecision{
			Strategy: StrategyAsk, Reason: "ask interaction authorizes read-only collaboration with user questions",
			Confidence: 1, Risk: RiskLow, Signals: signals,
		}
	}
	if spec.Interaction.Intent == model.IntentPlan {
		signals = append(signals, DecisionSignal{Name: "read_only_authorization", Value: "true"})
		return StrategyDecision{
			Strategy: StrategyPlan, Reason: "plan interaction requires a read-only plan before final explanation",
			Confidence: 1, Risk: RiskLow, Signals: signals,
		}
	}

	slideCount := len(pack.Outline.Outline.SlideOrder)
	empty := slideCount == 0
	impact := 1
	if spec.Target.Level == model.TargetDeck {
		impact = slideCount
		if impact == 0 && spec.Options.DesiredSlideCount > 0 {
			impact = spec.Options.DesiredSlideCount
		}
	}
	signals = append(signals,
		DecisionSignal{Name: "empty_project", Value: fmt.Sprint(empty)},
		DecisionSignal{Name: "estimated_target_count", Value: fmt.Sprint(impact)},
	)
	if pack.Target.Materialization != nil {
		signals = append(signals, DecisionSignal{Name: "materialization_state", Value: pack.Target.Materialization.State})
	}

	instruction := strings.ToLower(strings.Join(strings.Fields(spec.Instruction), " "))
	structural := containsAny(instruction,
		"新增页", "添加页面", "删除页", "移除页面", "重排", "调整顺序", "章节", "目录结构", "重建", "结构调整",
		"add slide", "delete slide", "remove slide", "reorder", "section", "rebuild")
	multi := containsAny(instruction,
		"多页", "所有页面", "全部页面", "整份", "整套", "全局协同", "批量",
		"multiple slides", "all slides", "whole deck", "entire deck", "batch")
	globalCoordination := containsAny(instruction,
		"全局与", "全局和", "设计语言", "统一风格", "主题并", "global and", "design language")
	ambiguous := len([]rune(strings.TrimSpace(spec.Instruction))) < 4
	signals = append(signals,
		DecisionSignal{Name: "structural_change", Value: fmt.Sprint(structural)},
		DecisionSignal{Name: "multi_target", Value: fmt.Sprint(multi || impact > 1)},
		DecisionSignal{Name: "global_coordination", Value: fmt.Sprint(globalCoordination)},
		DecisionSignal{Name: "instruction_ambiguous", Value: fmt.Sprint(ambiguous)},
	)

	switch {
	case empty && spec.Target.Level == model.TargetDeck:
		return complexDecision("empty project requires coordinated whole-deck generation", RiskHigh, .99, signals)
	case structural:
		return complexDecision("page or section structure changes require coordinated planning", RiskHigh, .98, signals)
	case multi || globalCoordination || impact > 1:
		return complexDecision("multiple targets or global coordination require a tracked plan", RiskMedium, .94, signals)
	case ambiguous:
		return complexDecision("instruction is not explicit enough for a safe direct write", RiskMedium, .78, signals)
	default:
		return StrategyDecision{
			Strategy: StrategyExecute, ExecuteMode: ExecuteModeDirect,
			Reason:     "explicit local instruction affects one declared target",
			Confidence: .92, Risk: RiskLow, Signals: signals,
		}
	}
}

func complexDecision(reason string, risk RiskLevel, confidence float64, signals []DecisionSignal) StrategyDecision {
	return StrategyDecision{
		Strategy: StrategyExecute, ExecuteMode: ExecuteModePlanned,
		Reason: reason, Confidence: confidence, Risk: risk, Signals: signals,
	}
}

func containsAny(value string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}
