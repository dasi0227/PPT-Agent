package workflow

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

var (
	ErrToolNotDisclosed = errors.New("TOOL_NOT_DISCLOSED")
	ErrCapabilityDenied = errors.New("CAPABILITY_DENIED")
)

type Risk string

const (
	RiskRead        Risk = "read"
	RiskWrite       Risk = "write"
	RiskDestructive Risk = "destructive"
	RiskControl     Risk = "control"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(context.Context, ToolInput) ToolResult
}

type ToolInput struct {
	Args        map[string]any
	State       WorkflowState
	Step        WorkflowStep
	Transaction *Transaction
}

type ToolResult struct {
	OK        bool          `json:"ok"`
	Summary   string        `json:"summary"`
	Artifacts []ArtifactRef `json:"artifacts"`
	Issues    []Issue       `json:"issues"`
	Retryable bool          `json:"retryable"`
}

func SuccessfulToolResult(summary string, artifacts ...ArtifactRef) ToolResult {
	return ToolResult{OK: true, Summary: summary, Artifacts: artifacts, Issues: []Issue{}}
}

type ToolDescriptor struct {
	Name         string              `json:"name"`
	Capabilities []Capability        `json:"capabilities"`
	Risk         Risk                `json:"risk"`
	Stages       []Stage             `json:"stages"`
	Strategies   []ExecutionStrategy `json:"strategies,omitempty"`
	Mutates      []ArtifactKind      `json:"mutates"`
	Tool         Tool                `json:"-"`
}

type ToolRegistry struct {
	tools map[string]ToolDescriptor
	order []string
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: map[string]ToolDescriptor{}, order: []string{}}
}

func (r *ToolRegistry) Register(desc ToolDescriptor) error {
	if desc.Tool == nil || desc.Name == "" || desc.Tool.Name() != desc.Name {
		return fmt.Errorf("invalid tool descriptor")
	}
	if _, exists := r.tools[desc.Name]; exists {
		return fmt.Errorf("duplicate tool %s", desc.Name)
	}
	desc.Capabilities = append([]Capability{}, desc.Capabilities...)
	desc.Stages = append([]Stage{}, desc.Stages...)
	desc.Strategies = append([]ExecutionStrategy{}, desc.Strategies...)
	desc.Mutates = append([]ArtifactKind{}, desc.Mutates...)
	r.tools[desc.Name] = desc
	r.order = append(r.order, desc.Name)
	return nil
}

func (r *ToolRegistry) Descriptor(name string) (ToolDescriptor, bool) {
	desc, ok := r.tools[name]
	return desc, ok
}

type RiskPolicy struct {
	AllowDestructive bool
}

type ToolSelection struct {
	Descriptors []ToolDescriptor
	byName      map[string]ToolDescriptor
	Step        WorkflowStep
	State       WorkflowState
}

func (s ToolSelection) Schemas() []ToolSchema {
	out := make([]ToolSchema, 0, len(s.Descriptors))
	for _, desc := range s.Descriptors {
		out = append(out, ToolSchema{
			Name: desc.Name, Description: desc.Tool.Description(), Parameters: desc.Tool.Parameters(),
		})
	}
	return out
}

type ToolSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

func (s ToolSelection) Execute(ctx context.Context, name string, args map[string]any, tx *Transaction) ToolResult {
	desc, ok := s.byName[name]
	if !ok {
		return deniedToolResult(ErrToolNotDisclosed, s.Step)
	}
	if err := validateDescriptor(desc, s.State, s.Step); err != nil {
		return deniedToolResult(err, s.Step)
	}
	return desc.Tool.Execute(ctx, ToolInput{Args: args, State: s.State, Step: s.Step, Transaction: tx})
}

func deniedToolResult(err error, step WorkflowStep) ToolResult {
	target := ArtifactRef{}
	if len(step.Targets) > 0 {
		target = step.Targets[0]
	}
	return ToolResult{
		OK: false, Summary: err.Error(), Artifacts: []ArtifactRef{}, Retryable: false,
		Issues: []Issue{{
			Code: err.Error(), Severity: SeverityFatal, Artifact: target,
			Evidence: "execution-time tool policy rejected the call", Verifier: "tool_policy",
		}},
	}
}

type ToolSelector struct {
	Registry *ToolRegistry
}

func (s ToolSelector) Select(state WorkflowState, step WorkflowStep, policy RiskPolicy) ToolSelection {
	base := BaseCapabilities(state.WorkSpec)
	strategy := StrategyCapabilities(state.Strategy)
	stepCaps := make(map[Capability]bool, len(step.Capabilities))
	for _, cap := range step.Capabilities {
		stepCaps[cap] = true
	}
	out := ToolSelection{
		Descriptors: []ToolDescriptor{}, byName: map[string]ToolDescriptor{},
		Step: step, State: state,
	}
	if s.Registry == nil {
		return out
	}
	for _, name := range s.Registry.order {
		desc := s.Registry.tools[name]
		if !containsStage(desc.Stages, state.Stage) {
			continue
		}
		if !containsStrategy(desc.Strategies, state.Strategy) {
			continue
		}
		if desc.Risk == RiskDestructive && !policy.AllowDestructive {
			continue
		}
		if !capabilitiesAllowed(desc.Capabilities, base, strategy, stepCaps) {
			continue
		}
		if !mutationsInScope(desc, step) {
			continue
		}
		out.Descriptors = append(out.Descriptors, desc)
		out.byName[name] = desc
	}
	sort.SliceStable(out.Descriptors, func(i, j int) bool { return out.Descriptors[i].Name < out.Descriptors[j].Name })
	return out
}

func BaseCapabilities(spec model.WorkSpec) map[Capability]bool {
	allowed := map[Capability]bool{
		CapabilityReadContext: true, CapabilityReadBlueprint: true,
		CapabilityReadDesignSpec: true, CapabilityControl: true,
	}
	if spec.Target.Artifact == model.ArtifactPresentation {
		allowed[CapabilityReadPresentation] = true
		allowed[CapabilitySearchAssets] = true
		allowed[CapabilityReadAssets] = true
		allowed[CapabilityRenderPreview] = true
	}
	if spec.Interaction.Intent == model.IntentConsult {
		return allowed
	}
	if spec.Target.Artifact == model.ArtifactBlueprint {
		allowed[CapabilityWriteBlueprint] = true
	} else {
		allowed[CapabilityWritePresentation] = true
		allowed[CapabilityWriteDesignSpec] = true
		allowed[CapabilityMountAssets] = true
	}
	allowed[CapabilityValidateBlueprint] = true
	allowed[CapabilityValidatePresentation] = true
	return allowed
}

func validateDescriptor(desc ToolDescriptor, state WorkflowState, step WorkflowStep) error {
	if !containsStage(desc.Stages, state.Stage) || !containsStrategy(desc.Strategies, state.Strategy) || !mutationsInScope(desc, step) {
		return ErrCapabilityDenied
	}
	base := BaseCapabilities(state.WorkSpec)
	strategy := StrategyCapabilities(state.Strategy)
	stepCaps := map[Capability]bool{}
	for _, cap := range step.Capabilities {
		stepCaps[cap] = true
	}
	if !capabilitiesAllowed(desc.Capabilities, base, strategy, stepCaps) {
		return ErrCapabilityDenied
	}
	return nil
}

func capabilitiesAllowed(required []Capability, sets ...map[Capability]bool) bool {
	for _, cap := range required {
		for _, set := range sets {
			if !set[cap] {
				return false
			}
		}
	}
	return true
}

func containsStage(stages []Stage, stage Stage) bool {
	for _, candidate := range stages {
		if candidate == stage {
			return true
		}
	}
	return false
}

func containsStrategy(strategies []ExecutionStrategy, strategy ExecutionStrategy) bool {
	if len(strategies) == 0 {
		return true
	}
	for _, candidate := range strategies {
		if candidate == strategy {
			return true
		}
	}
	return false
}

func mutationsInScope(desc ToolDescriptor, step WorkflowStep) bool {
	if len(desc.Mutates) == 0 {
		return true
	}
	for _, mutation := range desc.Mutates {
		found := false
		for _, target := range step.Targets {
			if target.Kind == mutation {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
