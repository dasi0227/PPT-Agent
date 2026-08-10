package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

var (
	ErrToolNotDisclosed = errors.New("RESOURCE_NOT_DISCLOSED")
	ErrCapabilityDenied = errors.New("CAPABILITY_DENIED")
	ErrTargetOutOfScope = errors.New("TARGET_OUT_OF_SCOPE")
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

const (
	CodeResourceInvalid         = "RESOURCE_INVALID"
	CodeResourceNotFound        = "RESOURCE_NOT_FOUND"
	CodeResourceNotDisclosed    = "RESOURCE_NOT_DISCLOSED"
	CodeTargetOutOfScope        = "TARGET_OUT_OF_SCOPE"
	CodeContentTooLarge         = "CONTENT_TOO_LARGE"
	CodeContentInvalid          = "CONTENT_INVALID"
	CodeTargetNotFound          = CodeResourceNotFound
	CodeTargetAlreadyExists     = "TARGET_ALREADY_EXISTS"
	CodeEditAnchorNotFound      = "EDIT_ANCHOR_NOT_FOUND"
	CodeEditAnchorAmbiguous     = "EDIT_ANCHOR_AMBIGUOUS"
	CodeModelInvalid            = CodeContentInvalid
	CodeContextBudget           = "CONTEXT_BUDGET_EXCEEDED"
	CodeRenderFailed            = "RENDER_FAILED"
	CodeRenderWorkerUnavailable = "RENDER_WORKER_UNAVAILABLE"
	CodeRunSessionRequired      = "RUN_SESSION_MISSING"
)

type ToolSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type DomainTool interface {
	Schema() ToolSchema
	Execute(context.Context, DomainToolInput) ToolResult
}

type DomainToolInput struct {
	Args       map[string]any
	CallID     string
	Context    contextengine.ContextPack
	ProjectDir string
	RunID      string
	Session    *RunSession
	Scope      model.RunScope
	Phase      RunPhase
	Mode       model.RunMode
}

// ChangedTarget is deliberately domain-shaped. Model-visible results never
// expose artifact paths, runtime paths, database keys, or session details.
type ChangedTarget struct {
	Type       string   `json:"type"`
	SlideID    string   `json:"slide_id,omitempty"`
	Part       string   `json:"part"`
	Revision   int      `json:"revision,omitempty"`
	Hash       string   `json:"hash"`
	Fields     []string `json:"fields,omitempty"`
	Insertions int      `json:"insertions,omitempty"`
	Deletions  int      `json:"deletions,omitempty"`
}

func (c ChangedTarget) Target() Resource {
	return Resource{Type: c.Type, SlideID: c.SlideID, Part: c.Part}
}

type ToolResult struct {
	OK               bool              `json:"ok"`
	Summary          string            `json:"summary"`
	Data             map[string]any    `json:"data,omitempty"`
	ChangedTargets   []ChangedTarget   `json:"changed_targets"`
	Issues           []Issue           `json:"issues"`
	Retryable        bool              `json:"retryable"`
	Code             string            `json:"code,omitempty"`
	Observation      string            `json:"-"`
	ObservationParts []llm.ContentPart `json:"-"`
	// Evidence and invalidation are runtime-internal. They are recorded in the
	// Evidence Ledger and SSE but are not duplicated in model observations.
	Evidence           []Evidence `json:"-"`
	InvalidatedTargets []Resource `json:"-"`
}

func SuccessfulToolResult(summary string) ToolResult {
	return ToolResult{
		OK: true, Summary: summary, Data: map[string]any{}, ChangedTargets: []ChangedTarget{},
		Evidence: []Evidence{}, InvalidatedTargets: []Resource{}, Issues: []Issue{},
	}
}

type ToolDescriptor struct {
	Tool       DomainTool
	ReadOnly   bool
	Capability string
	Risk       RiskLevel
	Phases     []RunPhase
}

type ToolRegistry struct {
	tools map[string]ToolDescriptor
	order []string
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: map[string]ToolDescriptor{}, order: []string{}}
}

func (r *ToolRegistry) RegisterDomainTool(tool DomainTool, readOnly bool, phases ...RunPhase) error {
	capability := "read"
	risk := RiskLow
	if !readOnly {
		capability, risk = "write", RiskMedium
	}
	return r.Register(tool, readOnly, capability, risk, phases...)
}

func (r *ToolRegistry) Register(tool DomainTool, readOnly bool, capability string, risk RiskLevel, phases ...RunPhase) error {
	if tool == nil || tool.Schema().Name == "" {
		return errors.New("invalid domain tool")
	}
	name := tool.Schema().Name
	if _, ok := r.tools[name]; ok {
		return fmt.Errorf("duplicate tool %s", name)
	}
	if len(phases) == 0 {
		phases = []RunPhase{PhaseChat, PhasePlanning, PhaseExecuting}
	}
	r.tools[name] = ToolDescriptor{
		Tool: tool, ReadOnly: readOnly, Capability: capability, Risk: risk,
		Phases: append([]RunPhase{}, phases...),
	}
	r.order = append(r.order, name)
	return nil
}

func (r *ToolRegistry) Descriptor(name string) (ToolDescriptor, bool) {
	desc, ok := r.tools[name]
	return desc, ok
}

type DomainToolProvider interface {
	RegisterDomainTools(*ToolRegistry) error
}

func AllowsWrite(scope model.RunScope, target Resource) bool {
	if scope.Artifact == model.ArtifactSpec && target.Type == "slide" && target.Part == "html" {
		return false
	}
	if scope.Level == model.ScopeDeck {
		return true
	}
	return target.Type == "slide" && target.SlideID == scope.SlideID
}

func AllowsRead(scope model.RunScope, target Resource) bool {
	if target.Type == "deck" {
		return true
	}
	if scope.Level == model.ScopeDeck {
		return true
	}
	return target.SlideID == scope.SlideID &&
		(scope.Artifact == model.ArtifactPPT || target.Part != "html")
}

func AllowsArtifact(scope model.RunScope, ref ArtifactRef) bool {
	return AllowsWrite(scope, resourceForArtifact(ref))
}

func (r *ToolRegistry) Disclose(phase RunPhase, mode model.RunMode) []ToolSchema {
	out := []ToolSchema{}
	for _, name := range r.order {
		desc := r.tools[name]
		if !containsPhase(desc.Phases, phase) {
			continue
		}
		if mode != model.ModeExecute && !desc.ReadOnly {
			continue
		}
		if phase == PhasePlanning && !desc.ReadOnly {
			continue
		}
		out = append(out, desc.Tool.Schema())
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r *ToolRegistry) Execute(ctx context.Context, disclosed map[string]bool, name string, args map[string]any, input DomainToolInput) ToolResult {
	if err := input.Context.Command.Validate(); err != nil {
		return failedToolResult(ErrCapabilityDenied.Error(), "RunCommand is invalid: "+err.Error(), false)
	}
	if !disclosed[name] {
		return failedToolResult(ErrToolNotDisclosed.Error(), "tool was not disclosed in this turn", false)
	}
	desc, ok := r.tools[name]
	if !ok {
		return failedToolResult(ErrToolNotDisclosed.Error(), "tool is not registered", false)
	}
	if !containsPhase(desc.Phases, input.Phase) {
		return failedToolResult(ErrCapabilityDenied.Error(), "tool is not allowed in the current runtime phase", false)
	}
	if input.Mode != model.ModeExecute && !desc.ReadOnly {
		return failedToolResult(ErrCapabilityDenied.Error(), "read-only mode cannot use write capabilities", false)
	}
	if input.Phase == PhasePlanning && !desc.ReadOnly {
		return failedToolResult(ErrCapabilityDenied.Error(), "planning phase cannot use write capabilities", false)
	}
	if !executionCapabilityAllowed(desc, input) {
		return failedToolResult(ErrCapabilityDenied.Error(), "tool capability or risk is denied by the current run policy", false)
	}
	if !desc.ReadOnly && input.Session == nil {
		return failedToolResult(CodeRunSessionRequired, "write tool requires an active run session", false)
	}
	if !desc.ReadOnly {
		if target, ok := declaredTarget(args); ok && !AllowsWrite(input.Scope, target) {
			return failedToolResult(ErrTargetOutOfScope.Error(), "requested write target is outside the current run scope", false)
		}
	}
	result := desc.Tool.Execute(ctx, input)
	for _, target := range result.ChangedTargets {
		if !AllowsWrite(input.Scope, target.Target()) {
			return failedToolResult(ErrTargetOutOfScope.Error(), "tool attempted to write a target outside the current run scope", false)
		}
	}
	return result
}

func executionCapabilityAllowed(desc ToolDescriptor, input DomainToolInput) bool {
	switch desc.Risk {
	case RiskLow:
	case RiskMedium:
		if desc.ReadOnly || input.Mode != model.ModeExecute || input.Phase != PhaseExecuting {
			return false
		}
	default:
		return false
	}
	switch desc.Capability {
	case "read", "ppt.read", "ppt.render", "context.search":
		return desc.ReadOnly
	case "write", "ppt.write", "ppt.edit":
		return !desc.ReadOnly &&
			(input.Context.Command.Scope.Artifact == model.ArtifactSpec ||
				input.Context.Command.Scope.Artifact == model.ArtifactPPT)
	default:
		return false
	}
}

func declaredTarget(args map[string]any) (Resource, bool) {
	value, ok := args["resource"].(map[string]any)
	if !ok {
		return Resource{}, false
	}
	target := Resource{}
	target.Type, _ = value["type"].(string)
	target.SlideID, _ = value["slide_id"].(string)
	target.Part, _ = value["part"].(string)
	return target, target.Type != ""
}

func failedToolResult(code, summary string, retryable bool) ToolResult {
	_ = retryable
	agentErr := model.NewAgentError(code, "tool_call", errors.New(summary))
	agentErr.Details["reason"] = summary
	if agentErr.ModelMessage != "" {
		agentErr.Details["next_action"] = agentErr.ModelMessage
	}
	observation, _ := json.Marshal(agentErr.ModelObservation())
	return ToolResult{
		OK: false, Summary: summary, Data: map[string]any{}, ChangedTargets: []ChangedTarget{},
		Evidence: []Evidence{}, InvalidatedTargets: []Resource{},
		Issues:    []Issue{{Code: code, Severity: SeverityError, Summary: summary}},
		Retryable: agentErr.Retryable, Code: code, Observation: string(observation),
	}
}

func containsPhase(phases []RunPhase, phase RunPhase) bool {
	for _, value := range phases {
		if value == phase {
			return true
		}
	}
	return false
}

func schemasByName(schemas []ToolSchema) map[string]bool {
	out := make(map[string]bool, len(schemas))
	for _, schema := range schemas {
		out[schema.Name] = true
	}
	return out
}
