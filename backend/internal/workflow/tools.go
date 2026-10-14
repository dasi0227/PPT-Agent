package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

var (
	ErrToolNotDisclosed = errors.New(model.CodeResourceNotDisclosed)
	ErrTargetOutOfScope = errors.New(model.CodeTargetOutOfScope)
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
	CapabilityImageRead          ToolCapability = "image.read"
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
	CapabilityImageRead:          {ReadOnly: true, Risk: RiskLow},
	CapabilityProjectCommandRead: {ReadOnly: true, Risk: RiskLow},
	CapabilityProjectCommandEdit: {ReadOnly: false, Risk: RiskHigh},
}

const (
	CodeTargetOutOfScope = model.CodeTargetOutOfScope
	CodeContentTooLarge  = "CONTENT_TOO_LARGE"
	CodeRenderFailed     = "RENDER_FAILED"
)

type ToolSchema = llm.ToolSchema

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
	Measure      string
	TargetPaths  []string
	PreimageHash string
	Prepared     any
}

type PreflightTool interface {
	Preflight(context.Context, DomainToolInput) ToolDecision
}

type DomainToolInput struct {
	SeenVersions map[string]string
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
	Messages     []llm.Message
}

type ActiveSkillSet struct {
	Skills     []model.RunSkill     `json:"skills"`
	Components []model.RunComponent `json:"components,omitempty"`
	mu         sync.Mutex
}

// ChangedTarget is deliberately domain-shaped. Model-visible results never
// expose artifact paths, runtime paths, database keys, or session details.
type ChangedTarget struct {
	Type        string   `json:"type"`
	SlideID     string   `json:"slide_id,omitempty"`
	Part        string   `json:"part"`
	Path        string   `json:"path,omitempty"`
	DisplayName string   `json:"display_name,omitempty"`
	Hash        string   `json:"hash"`
	Fields      []string `json:"fields,omitempty"`
	Insertions  int      `json:"insertions,omitempty"`
	Deletions   int      `json:"deletions,omitempty"`
}

func (c ChangedTarget) Target() Resource {
	return Resource{Type: c.Type, SlideID: c.SlideID, Part: c.Part, Path: c.Path}
}

type ToolResult struct {
	ContentPrecheck []model.ContentPrecheck `json:"content_precheck,omitempty"`
	OK              bool                    `json:"ok"`
	// Applied means the requested operation completed, including a valid no-op.
	// An answered refusal is OK but does not apply the operation.
	Applied             bool                 `json:"applied"`
	Summary             string               `json:"summary"`
	Data                map[string]any       `json:"data,omitempty"`
	ChangedTargets      []ChangedTarget      `json:"changed_targets"`
	Issues              []Issue              `json:"issues"`
	Retryable           bool                 `json:"retryable"`
	Code                string               `json:"code,omitempty"`
	Observation         string               `json:"-"`
	ObservationParts    []llm.ContentPart    `json:"-"`
	ObservationMetadata *llm.MessageMetadata `json:"-"`
	Command             *CommandExecution    `json:"-"`
	LoadedResources     []LoadedResource     `json:"-"`
	OperationTargets    []model.PublicTarget `json:"-"`
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
		OK: true, Applied: true, Summary: summary, Data: map[string]any{}, ChangedTargets: []ChangedTarget{},
		Evidence: []Evidence{}, InvalidatedTargets: []Resource{}, Issues: []Issue{},
	}
}

// RefusedToolResult is an answered user decision, not a tool failure.
func RefusedToolResult(summary string) ToolResult {
	result := SuccessfulToolResult(summary)
	result.Applied = false
	result.Data = map[string]any{"decision": "refuse", "summary": summary}
	return result
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
	allowed map[string]bool
	tools   map[string]ToolDescriptor
	order   []string
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: map[string]ToolDescriptor{}, order: []string{}}
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
		return true
	}
	if target.Type != "slide" || !scope.ContainsSlide(target.SlideID) {
		return false
	}
	return target.Part == "spec" || target.Part == "html"
}

func AllowsArtifact(scope model.RunScope, ref ArtifactRef) bool {
	return AllowsWrite(scope, resourceForArtifact(ref))
}

func (r *ToolRegistry) Disclose(phase RunPhase, mode model.RunMode, scope model.RunScope) []ToolSchema {
	out := []ToolSchema{}
	for _, name := range r.order {
		if r.allowed != nil && !r.allowed[name] {
			continue
		}
		desc := r.tools[name]
		if !toolAvailable(desc, phase, mode, scope) {
			continue
		}
		out = append(out, desc.Tool.Schema())
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func toolRelevantToRun(name string, mode model.RunMode, scope model.RunScope) bool {
	if name == "render_slide" {
		// Rendering capability is stable; actual page authorization is checked at execution.
		return mode == model.ModeExecute
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

func (r *ToolRegistry) Execute(ctx context.Context, disclosed map[string]bool, name string, args map[string]any, input DomainToolInput) ToolResult {
	if args == nil {
		args = map[string]any{}
		input.Args = args
	}
	if err := input.Context.Command.Validate(); err != nil {
		return failedToolResult(model.CodeInternal, "RunCommand is invalid: "+err.Error())
	}
	if !input.Scope.Equal(input.Context.Command.Scope) || input.Mode != input.Context.Command.Mode {
		return failedToolResult(model.CodeInternal, "tool input scope or mode diverges from the Runtime RunCommand")
	}
	if !disclosed[name] || (r.allowed != nil && !r.allowed[name]) {
		return failedToolResult(ErrToolNotDisclosed.Error(), "tool was not disclosed in this turn")
	}
	desc, ok := r.tools[name]
	if !ok {
		return failedToolResult(ErrToolNotDisclosed.Error(), "tool is not registered")
	}
	if !toolAvailable(desc, input.Phase, input.Mode, input.Scope) {
		return failedToolResult(model.CodeInternal, "tool is unavailable for the current Runtime mode, phase, scope, capability, or risk policy")
	}
	normalized, _, err := prepareToolArguments(desc.Tool.Schema(), args)
	if err != nil {
		return argumentFailure(err)
	}
	args, input.Args = normalized, normalized
	mutates := input.Decision != nil && input.Decision.Mutates
	if (!desc.ReadOnly || mutates) && input.Session == nil {
		return failedToolResult(model.CodeInternal, "write tool requires an active run session")
	}
	result := desc.Tool.Execute(ctx, input)
	for _, target := range result.ChangedTargets {
		if !AllowsWrite(input.Scope, target.Target()) {
			return failedToolResult(ErrTargetOutOfScope.Error(), "tool attempted to write a target outside the current run scope")
		}
	}
	return result
}

func validateToolArguments(schema ToolSchema, args map[string]any) error {
	compiled, err := compileToolArguments(schema)
	if err != nil {
		return err
	}
	return checkToolArguments(compiled, args)
}

func mostSpecificValidationError(root *jsonschema.ValidationError) *jsonschema.ValidationError {
	best := root
	for _, cause := range root.Causes {
		candidate := mostSpecificValidationError(cause)
		if len(candidate.InstanceLocation) > len(best.InstanceLocation) || len(best.Causes) > 0 {
			best = candidate
		}
	}
	return best
}

func failedToolResult(code, summary string) ToolResult {
	return detailedToolFailure(code, summary, nil)
}

func detailedToolFailure(code, summary string, details map[string]any) ToolResult {
	if details == nil {
		details = map[string]any{}
	}
	result := ToolResult{
		OK: false, Summary: summary, Data: details, ChangedTargets: []ChangedTarget{},
		Evidence: []Evidence{}, InvalidatedTargets: []Resource{},
		Issues: []Issue{{Code: code, Severity: SeverityError, Summary: summary}},
		Code:   code,
	}
	agentErr := toolResultError(result)
	result.Retryable = agentErr.ShouldAutoRetry()
	observation, _ := json.Marshal(contextengine.ModelValue(agentErr.ModelObservation()))
	result.Observation = string(observation)
	return result
}

// Preserve domain diagnostics through binding, persistence and replay without
// allowing result data to replace the registered error identity or category.
func toolResultError(result ToolResult) *model.AgentError {
	agentErr := model.NewAgentError(result.Code, "tool_call", errors.New(result.Summary))
	agentErr.Reason = result.Summary
	details := result.Data
	// Render storage and UI diagnostics include image paths and proofs. Only the
	// screenshot's model diagnostics belong in the failed tool reply.
	if result.Code == CodeRenderFailed && result.Data["model_diagnostics"] != nil {
		details = map[string]any{"slide_id": result.Data["slide_id"], "diagnostics": result.Data["model_diagnostics"]}
	}
	for key, value := range details {
		switch key {
		case "ok", "code", "category", "reason":
			continue
		case "measure":
			if measure, ok := value.(string); ok && measure != "" {
				agentErr.Measure = measure
			}
		default:
			agentErr.Details[key] = value
		}
	}
	return agentErr
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
