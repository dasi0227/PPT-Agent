package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

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

// ToolCapability is an internal Runtime authorization class. It is not part
// of the model-visible function contract: a model can call only the tool
// schemas Runtime discloses for the current turn.
//
// Keep the capability vocabulary closed. A descriptor that cannot be mapped
// to this policy is a server configuration error, not something an Agent can
// repair by trying another call.
type ToolCapability string

const (
	CapabilityRead               ToolCapability = "read"
	CapabilityWrite              ToolCapability = "write"
	CapabilityPPTRead            ToolCapability = "ppt.read"
	CapabilityPPTMutate          ToolCapability = "ppt.mutate"
	CapabilityPPTRender          ToolCapability = "ppt.render"
	CapabilityProjectCommandRead ToolCapability = "project.command.read"
	CapabilityProjectCommandEdit ToolCapability = "project.command.edit"
)

type capabilityPolicy struct {
	ReadOnly bool
	Risk     RiskLevel
}

var capabilityPolicies = map[ToolCapability]capabilityPolicy{
	CapabilityRead:               {ReadOnly: true, Risk: RiskLow},
	CapabilityWrite:              {ReadOnly: false, Risk: RiskMedium},
	CapabilityPPTRead:            {ReadOnly: true, Risk: RiskLow},
	CapabilityPPTMutate:          {ReadOnly: false, Risk: RiskMedium},
	CapabilityPPTRender:          {ReadOnly: true, Risk: RiskLow},
	CapabilityProjectCommandRead: {ReadOnly: true, Risk: RiskLow},
	CapabilityProjectCommandEdit: {ReadOnly: false, Risk: RiskHigh},
}

const (
	CodeResourceInvalid         = "RESOURCE_INVALID"
	CodeResourceNotFound        = "RESOURCE_NOT_FOUND"
	CodeResourceNotDisclosed    = "RESOURCE_NOT_DISCLOSED"
	CodeTargetOutOfScope        = "TARGET_OUT_OF_SCOPE"
	CodeContentTooLarge         = "CONTENT_TOO_LARGE"
	CodeContentInvalid          = "CONTENT_INVALID"
	CodePatchInvalid            = "PATCH_INVALID"
	CodePatchPathDenied         = "PATCH_PATH_DENIED"
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

type ToolDecision struct {
	Outcome      string
	Mutates      bool
	CommandHash  string
	Command      string
	ReasonCode   string
	PublicReason string
	TargetPaths  []string
	PreimageHash string
	Prepared     any
}

type PreflightTool interface {
	Preflight(context.Context, DomainToolInput) ToolDecision
}

type DomainToolInput struct {
	Args         map[string]any
	CallID       string
	Context      contextengine.ContextPack
	ProjectDir   string
	RunID        string
	Session      *RunSession
	Scope        model.RunScope
	Phase        RunPhase
	Mode         model.RunMode
	Decision     *ToolDecision
	ActiveSkills *ActiveSkillSet
}

type ActiveSkillSet struct {
	Skills []model.RunSkill `json:"skills"`
}

func (s *ActiveSkillSet) Add(values []model.RunSkill) []model.RunSkill {
	if s == nil {
		return nil
	}
	seen := make(map[string]bool, len(s.Skills))
	for _, skill := range s.Skills {
		seen[skill.ID] = true
	}
	added := make([]model.RunSkill, 0, len(values))
	for _, skill := range values {
		if seen[skill.ID] {
			continue
		}
		seen[skill.ID] = true
		s.Skills = append(s.Skills, skill)
		added = append(added, skill)
	}
	return added
}

// ChangedTarget is deliberately domain-shaped. Model-visible results never
// expose artifact paths, runtime paths, database keys, or session details.
type ChangedTarget struct {
	Type        string   `json:"type"`
	SlideID     string   `json:"slide_id,omitempty"`
	Part        string   `json:"part"`
	Path        string   `json:"path,omitempty"`
	DisplayName string   `json:"display_name,omitempty"`
	Revision    int      `json:"revision,omitempty"`
	Hash        string   `json:"hash"`
	Fields      []string `json:"fields,omitempty"`
	Insertions  int      `json:"insertions,omitempty"`
	Deletions   int      `json:"deletions,omitempty"`
}

func (c ChangedTarget) Target() Resource {
	return Resource{Type: c.Type, SlideID: c.SlideID, Part: c.Part, Path: c.Path}
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
	Command          *CommandExecution `json:"-"`
	LoadedResources  []LoadedResource  `json:"-"`
	// Evidence and invalidation are runtime-internal. They are recorded in the
	// Evidence Ledger and SSE but are not duplicated in model observations.
	Evidence           []Evidence `json:"-"`
	InvalidatedTargets []Resource `json:"-"`
}

type LoadedResource struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	LocalPath string `json:"-"`
	OpenURL   string `json:"open_url,omitempty"`
}

type CommandExecution struct {
	Text            string
	Status          string
	Sensitive       bool
	ExitCode        int
	DurationMS      int64
	OutputTruncated bool
	Stdout          string
	Stderr          string
	Reason          string
}

func SuccessfulToolResult(summary string) ToolResult {
	return ToolResult{
		OK: true, Summary: summary, Data: map[string]any{}, ChangedTargets: []ChangedTarget{},
		Evidence: []Evidence{}, InvalidatedTargets: []Resource{}, Issues: []Issue{},
	}
}

type ToolDescriptor struct {
	Tool           DomainTool
	ReadOnly       bool
	Capability     ToolCapability
	Risk           RiskLevel
	Phases         []RunPhase
	Dynamic        bool
	EditCapability ToolCapability
}

func (r *ToolRegistry) RegisterDynamic(
	tool DomainTool,
	readCapability ToolCapability,
	editCapability ToolCapability,
	phases ...RunPhase,
) error {
	if err := r.Register(tool, true, readCapability, RiskLow, phases...); err != nil {
		return err
	}
	name := tool.Schema().Name
	desc := r.tools[name]
	if policy, ok := capabilityPolicies[editCapability]; !ok || policy.ReadOnly || policy.Risk != RiskHigh {
		delete(r.tools, name)
		r.order = r.order[:len(r.order)-1]
		return fmt.Errorf("invalid dynamic edit capability %q", editCapability)
	}
	desc.Dynamic = true
	desc.EditCapability = editCapability
	r.tools[name] = desc
	return nil
}

type ToolRegistry struct {
	tools map[string]ToolDescriptor
	order []string
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: map[string]ToolDescriptor{}, order: []string{}}
}

func (r *ToolRegistry) RegisterDomainTool(tool DomainTool, readOnly bool, phases ...RunPhase) error {
	capability := CapabilityRead
	risk := RiskLow
	if !readOnly {
		capability, risk = CapabilityWrite, RiskMedium
	}
	return r.Register(tool, readOnly, capability, risk, phases...)
}

func (r *ToolRegistry) Register(tool DomainTool, readOnly bool, capability ToolCapability, risk RiskLevel, phases ...RunPhase) error {
	if tool == nil || tool.Schema().Name == "" {
		return errors.New("invalid domain tool")
	}
	policy, ok := capabilityPolicies[capability]
	if !ok {
		return fmt.Errorf("unknown tool capability %q", capability)
	}
	if policy.ReadOnly != readOnly || policy.Risk != risk {
		return fmt.Errorf("tool capability %q requires read_only=%t risk=%q", capability, policy.ReadOnly, policy.Risk)
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
	if target.Type == "file" && target.Part == "content" {
		return true
	}
	if target.Type == "deck" {
		return scope.AllowsGlobal()
	}
	if target.Type != "slide" || !scope.ContainsSlide(target.SlideID) {
		return false
	}
	return (target.Part == "spec" && scope.AllowsSpec()) ||
		(target.Part == "html" && scope.AllowsHTML())
}

func AllowsRead(model.RunScope, Resource) bool { return true }

func AllowsArtifact(scope model.RunScope, ref ArtifactRef) bool {
	return AllowsWrite(scope, resourceForArtifact(ref))
}

func (r *ToolRegistry) Disclose(phase RunPhase, mode model.RunMode, scope model.RunScope) []ToolSchema {
	out := []ToolSchema{}
	for _, name := range r.order {
		desc := r.tools[name]
		if !toolAvailable(desc, phase, mode, scope) {
			continue
		}
		out = append(out, scopeToolSchema(desc.Tool.Schema(), scope, desc.ReadOnly))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func toolRelevantToRun(name string, mode model.RunMode, scope model.RunScope) bool {
	if name == "render_slide" {
		return mode == model.ModeExecute && scope.AllowsHTML()
	}
	return true
}

// toolAvailable is the single authority for domain-tool exposure and
// execution. Keeping these checks together prevents a function schema from
// being shown to the model while Runtime will deterministically deny it.
func toolAvailable(desc ToolDescriptor, phase RunPhase, mode model.RunMode, scope model.RunScope) bool {
	if !containsPhase(desc.Phases, phase) || !toolRelevantToRun(desc.Tool.Schema().Name, mode, scope) {
		return false
	}
	if mode != model.ModeExecute && !desc.ReadOnly && !desc.Dynamic {
		return false
	}
	if phase == PhasePlanning && !desc.ReadOnly {
		return false
	}
	policy, ok := capabilityPolicies[desc.Capability]
	if !ok || policy.ReadOnly != desc.ReadOnly || policy.Risk != desc.Risk {
		return false
	}
	if desc.ReadOnly {
		return true
	}
	return mode == model.ModeExecute && phase == PhaseExecuting
}

func scopeToolSchema(schema ToolSchema, scope model.RunScope, readOnly bool) ToolSchema {
	if schema.Name == "mutate_ppt" {
		variants, _ := schema.Parameters["oneOf"].([]any)
		filtered := []any{}
		for _, raw := range variants {
			variant, _ := raw.(map[string]any)
			props, _ := variant["properties"].(map[string]any)
			opSchema, _ := props["op"].(map[string]any)
			op, _ := opSchema["const"].(string)
			probeSlideID := ""
			if len(scope.SlideIDs) > 0 {
				probeSlideID = scope.SlideIDs[0]
			}
			allowed := mutationOperationAllowed(scope, op, probeSlideID)
			if allowed {
				if _, ok := props["slide_id"]; ok && !scope.AllowsGlobal() {
					props["slide_id"] = map[string]any{"type": "string", "enum": scope.SlideIDs}
				}
				filtered = append(filtered, raw)
			}
		}
		schema.Parameters["oneOf"] = filtered
		return schema
	}
	properties, _ := schema.Parameters["properties"].(map[string]any)
	if properties == nil {
		return schema
	}
	if _, ok := properties["resource"]; ok {
		properties["resource"] = resourceSchemaForScope(scope, !readOnly)
	}
	if schema.Name == "render_slide" && !scope.AllowsGlobal() {
		properties["slide_id"] = map[string]any{
			"type": "string", "enum": scope.SlideIDs,
			"description": "A slide authorized by the current run scope.",
		}
	}
	return schema
}

// mutationOperationAllowed is the single scope rule used both to disclose a
// mutate_ppt schema variant and to authorize the decoded mutation request.
// Page coverage and object capabilities are independent: spec-only runs must
// never receive or execute slide HTML mutations, even when every page is in scope.
func mutationOperationAllowed(scope model.RunScope, op, slideID string) bool {
	if strings.HasPrefix(op, "manifest.") || strings.HasPrefix(op, "outline.") || strings.HasPrefix(op, "design.") {
		return scope.AllowsGlobal()
	}
	if !scope.ContainsSlide(slideID) {
		return false
	}
	return (strings.HasPrefix(op, "slide.spec.") && scope.AllowsSpec()) ||
		(strings.HasPrefix(op, "slide.html.") && scope.AllowsHTML())
}

func (r *ToolRegistry) Execute(ctx context.Context, disclosed map[string]bool, name string, args map[string]any, input DomainToolInput) ToolResult {
	if err := input.Context.Command.Validate(); err != nil {
		return failedToolResult(ErrCapabilityDenied.Error(), "RunCommand is invalid: "+err.Error(), false)
	}
	if !input.Scope.Equal(input.Context.Command.Scope) || input.Mode != input.Context.Command.Mode {
		return failedToolResult(ErrCapabilityDenied.Error(), "tool input scope or mode diverges from the Runtime RunCommand", false)
	}
	if !disclosed[name] {
		return failedToolResult(ErrToolNotDisclosed.Error(), "tool was not disclosed in this turn", false)
	}
	desc, ok := r.tools[name]
	if !ok {
		return failedToolResult(ErrToolNotDisclosed.Error(), "tool is not registered", false)
	}
	if !toolAvailable(desc, input.Phase, input.Mode, input.Scope) {
		return failedToolResult(ErrCapabilityDenied.Error(), "tool is unavailable for the current Runtime mode, phase, scope, capability, or risk policy", false)
	}
	mutates := input.Decision != nil && input.Decision.Mutates
	if (!desc.ReadOnly || mutates) && input.Session == nil {
		return failedToolResult(CodeRunSessionRequired, "write tool requires an active run session", false)
	}
	if !desc.ReadOnly || mutates {
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
