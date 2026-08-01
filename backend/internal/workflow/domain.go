// Package workflow defines the framework-independent Plan–Execute–Verify domain.
package workflow

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type Stage string

const (
	StageNormalize Stage = "normalize"
	StageContext   Stage = "context"
	StagePlan      Stage = "plan"
	StageExecute   Stage = "execute"
	StageVerify    Stage = "verify"
	StageRepair    Stage = "repair"
	StageCommit    Stage = "commit"
	StageDeliver   Stage = "deliver"
)

var orderedStages = []Stage{
	StageNormalize, StageContext, StagePlan, StageExecute,
	StageVerify, StageRepair, StageCommit, StageDeliver,
}

type WorkflowStatus string

const (
	StatusPending   WorkflowStatus = "pending"
	StatusRunning   WorkflowStatus = "running"
	StatusWaiting   WorkflowStatus = "waiting"
	StatusCompleted WorkflowStatus = "completed"
	StatusFailed    WorkflowStatus = "failed"
	StatusCanceled  WorkflowStatus = "canceled"
)

func (s WorkflowStatus) Terminal() bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusCanceled
}

type Operation string

const (
	OperationCreate      Operation = "create"
	OperationRevise      Operation = "revise"
	OperationMaterialize Operation = "materialize"
	OperationRebuild     Operation = "rebuild"
	OperationConsult     Operation = "consult"
)

type StepKind string

const (
	StepAnalyze          StepKind = "analyze"
	StepPatchBlueprint   StepKind = "patch_blueprint"
	StepReplaceDeck      StepKind = "replace_deck"
	StepApplyDesign      StepKind = "apply_design"
	StepMaterializeSlide StepKind = "materialize_slide"
	StepReviseSlide      StepKind = "revise_slide"
	StepValidate         StepKind = "validate"
	StepRepair           StepKind = "repair"
	StepCommit           StepKind = "commit"
	StepDeliver          StepKind = "deliver"
)

type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
)

type ArtifactKind string

const (
	ArtifactDeck         ArtifactKind = "blueprint_deck"
	ArtifactSlide        ArtifactKind = "blueprint_slide"
	ArtifactDesign       ArtifactKind = "design_spec"
	ArtifactPresentation ArtifactKind = "presentation_slide"
)

type ArtifactRef struct {
	Kind    ArtifactKind `json:"kind"`
	ID      string       `json:"id"`
	Path    string       `json:"path,omitempty"`
	Project string       `json:"project_id,omitempty"`
}

func (a ArtifactRef) Key() string { return string(a.Kind) + ":" + a.ID }

type Assumption struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Safe    bool   `json:"safe"`
}

type Criterion struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Verifiers   []string `json:"verifiers"`
	Required    bool     `json:"required"`
}

type Condition struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type Capability string

const (
	CapabilityReadContext          Capability = "read_context"
	CapabilityReadBlueprint        Capability = "read_blueprint"
	CapabilityWriteBlueprint       Capability = "write_blueprint"
	CapabilityReadDesignSpec       Capability = "read_design_spec"
	CapabilityWriteDesignSpec      Capability = "write_design_spec"
	CapabilityReadPresentation     Capability = "read_presentation"
	CapabilityWritePresentation    Capability = "write_presentation"
	CapabilitySearchAssets         Capability = "search_assets"
	CapabilityReadAssets           Capability = "read_assets"
	CapabilityMountAssets          Capability = "mount_assets"
	CapabilityRenderPreview        Capability = "render_preview"
	CapabilityValidateBlueprint    Capability = "validate_blueprint"
	CapabilityValidatePresentation Capability = "validate_presentation"
	CapabilityControl              Capability = "control"
)

type ExecutionBudget struct {
	MaxPlanAttempts int           `json:"max_plan_attempts"`
	MaxStepTurns    int           `json:"max_step_turns"`
	MaxRepairRounds int           `json:"max_repair_rounds"`
	MaxToolFailures int           `json:"max_tool_failures"`
	MaxDuration     time.Duration `json:"max_duration"`
}

func DefaultBudget() ExecutionBudget {
	return ExecutionBudget{
		MaxPlanAttempts: 2, MaxStepTurns: 12, MaxRepairRounds: 2,
		MaxToolFailures: 3, MaxDuration: 10 * time.Minute,
	}
}

type WorkflowStep struct {
	ID            string        `json:"id"`
	Kind          StepKind      `json:"kind"`
	Title         string        `json:"title"`
	Instruction   string        `json:"instruction"`
	Targets       []ArtifactRef `json:"targets"`
	DependsOn     []string      `json:"depends_on"`
	Preconditions []Condition   `json:"preconditions"`
	Capabilities  []Capability  `json:"capabilities"`
	Verifiers     []string      `json:"verifiers"`
	Status        StepStatus    `json:"status"`
}

type WorkflowPlan struct {
	ID              string          `json:"id"`
	Version         int             `json:"version"`
	Goal            string          `json:"goal"`
	Target          model.RunTarget `json:"target"`
	Operation       Operation       `json:"operation"`
	Assumptions     []Assumption    `json:"assumptions"`
	Affected        []ArtifactRef   `json:"affected"`
	Steps           []WorkflowStep  `json:"steps"`
	SuccessCriteria []Criterion     `json:"success_criteria"`
	Budget          ExecutionBudget `json:"budget"`
}

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
	SeverityFatal   Severity = "fatal"
)

func (s Severity) BlocksCommit() bool { return s == SeverityError || s == SeverityFatal }

type EvidenceRef struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Summary  string `json:"summary"`
	Location string `json:"location,omitempty"`
}

type Issue struct {
	Code        string       `json:"code"`
	Severity    Severity     `json:"severity"`
	Artifact    ArtifactRef  `json:"artifact"`
	Location    string       `json:"location,omitempty"`
	Evidence    string       `json:"evidence,omitempty"`
	EvidenceRef *EvidenceRef `json:"evidence_ref,omitempty"`
	RepairHint  string       `json:"repair_hint,omitempty"`
	Verifier    string       `json:"verifier"`
}

type ArtifactChange struct {
	Artifact   ArtifactRef `json:"artifact"`
	BeforeHash string      `json:"before_hash,omitempty"`
	AfterHash  string      `json:"after_hash"`
	StepID     string      `json:"step_id"`
}

type ChangeSet struct {
	Created  []ArtifactChange `json:"created"`
	Updated  []ArtifactChange `json:"updated"`
	Deleted  []ArtifactChange `json:"deleted"`
	Warnings []Issue          `json:"warnings"`
}

func EmptyChangeSet() ChangeSet {
	return ChangeSet{
		Created: []ArtifactChange{}, Updated: []ArtifactChange{},
		Deleted: []ArtifactChange{}, Warnings: []Issue{},
	}
}

type VerifyResult struct {
	Passed   bool               `json:"passed"`
	Issues   []Issue            `json:"issues"`
	Evidence []EvidenceRef      `json:"evidence"`
	Metrics  map[string]float64 `json:"metrics"`
}

type StructuredOutcome struct {
	WorkflowID   string            `json:"workflow_id"`
	Strategy     ExecutionStrategy `json:"strategy"`
	Status       WorkflowStatus    `json:"status"`
	Operation    Operation         `json:"operation"`
	Target       model.RunTarget   `json:"target"`
	Affected     []ArtifactRef     `json:"affected"`
	Changes      ChangeSet         `json:"changes"`
	Issues       []Issue           `json:"issues"`
	RepairRounds int               `json:"repair_rounds"`
	Summary      string            `json:"summary"`
	Code         string            `json:"code,omitempty"`
	Message      string            `json:"message,omitempty"`
}

type WorkflowState struct {
	WorkflowID    string            `json:"workflow_id"`
	RunID         string            `json:"run_id"`
	WorkSpec      model.WorkSpec    `json:"work_spec"`
	ContextID     string            `json:"context_id"`
	Stage         Stage             `json:"stage"`
	Plan          WorkflowPlan      `json:"plan"`
	CurrentStepID string            `json:"current_step_id,omitempty"`
	Attempt       int               `json:"attempt"`
	RepairRound   int               `json:"repair_round"`
	Issues        []Issue           `json:"issues"`
	Changes       ChangeSet         `json:"changes"`
	Status        WorkflowStatus    `json:"status"`
	Strategy      ExecutionStrategy `json:"strategy"`
	StartedAt     time.Time         `json:"started_at"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
}

func (s WorkflowState) Marshal() ([]byte, error) { return json.Marshal(s) }

func CanTransition(from, to Stage) bool {
	if from == StageVerify && to == StageCommit {
		return true
	}
	if from == StageVerify && to == StageRepair {
		return true
	}
	if from == StageRepair && to == StageVerify {
		return true
	}
	for i := range orderedStages {
		if orderedStages[i] != from {
			continue
		}
		return i+1 < len(orderedStages) && orderedStages[i+1] == to
	}
	return false
}

func Transition(state *WorkflowState, to Stage) error {
	if state.Stage == "" && to == StageNormalize {
		state.Stage = to
		return nil
	}
	if !CanTransition(state.Stage, to) {
		return fmt.Errorf("invalid workflow transition %s -> %s", state.Stage, to)
	}
	state.Stage = to
	return nil
}
