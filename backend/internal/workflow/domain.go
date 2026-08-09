// Package workflow implements the intent-bounded, evidence-gated ReAct runtime.
package workflow

import (
	"context"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type RuntimePhase string

const (
	PhaseChat            RuntimePhase = "chat"
	PhasePlanning        RuntimePhase = "planning"
	PhaseExecuting       RuntimePhase = "executing"
	PhaseWaitingInput    RuntimePhase = "waiting_input"
	PhaseCompletionCheck RuntimePhase = "completion_check"
	PhaseCommitting      RuntimePhase = "committing"
	PhaseTerminal        RuntimePhase = "terminal"
)

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

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
	SeverityFatal   Severity = "fatal"
)

func (s Severity) BlocksCompletion() bool {
	return s == SeverityError || s == SeverityFatal
}

type Resource struct {
	Type    string `json:"type"`
	SlideID string `json:"slide_id,omitempty"`
	Part    string `json:"part"`
}

func (r Resource) Key() string {
	if r.Type == "slide" {
		return "slide:" + r.SlideID + ":" + r.Part
	}
	return "deck:" + r.Part
}

type Issue struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Resource Resource `json:"resource,omitempty"`
	Summary  string   `json:"summary"`
	Action   string   `json:"action,omitempty"`
}

type ArtifactKind string

const (
	ArtifactOutline   ArtifactKind = "outline"
	ArtifactDesign    ArtifactKind = "design"
	ArtifactSlideSpec ArtifactKind = "slide_spec"
	ArtifactSlideHTML ArtifactKind = "slide_html"
	ArtifactDerived   ArtifactKind = "derived"
)

type ArtifactRef struct {
	Kind    ArtifactKind `json:"kind"`
	ID      string       `json:"id"`
	Path    string       `json:"path,omitempty"`
	Project string       `json:"project_id,omitempty"`
}

func (a ArtifactRef) Key() string { return a.Resource().Key() }

func (a ArtifactRef) Resource() Resource {
	return resourceForArtifact(a)
}

func resourceForArtifact(ref ArtifactRef) Resource {
	switch ref.Kind {
	case ArtifactSlideSpec:
		return Resource{Type: "slide", SlideID: ref.ID, Part: "spec"}
	case ArtifactSlideHTML:
		return Resource{Type: "slide", SlideID: ref.ID, Part: "html"}
	case ArtifactDesign:
		return Resource{Type: "deck", Part: "design"}
	default:
		return Resource{Type: "deck", Part: "outline"}
	}
}

type ArtifactChange struct {
	Artifact   ArtifactRef `json:"artifact"`
	BeforeHash string      `json:"before_hash,omitempty"`
	AfterHash  string      `json:"after_hash"`
	Source     string      `json:"source"`
	Tentative  bool        `json:"tentative,omitempty"`
	Insertions int         `json:"insertions,omitempty"`
	Deletions  int         `json:"deletions,omitempty"`
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

func (c ChangeSet) Count() int {
	return len(c.Created) + len(c.Updated) + len(c.Deleted)
}

func (c ChangeSet) All() []ArtifactChange {
	out := make([]ArtifactChange, 0, c.Count())
	out = append(out, c.Created...)
	out = append(out, c.Updated...)
	out = append(out, c.Deleted...)
	return out
}

type RuntimeBudget struct {
	MaxTurns                   int
	MaxDuration                time.Duration
	MaxConsecutiveToolFailures int
	MaxIdenticalGateRejections int
	ContextCompactionThreshold int
}

func DefaultRuntimeBudget() RuntimeBudget {
	return RuntimeBudget{
		MaxTurns: 128, MaxDuration: time.Hour,
		MaxConsecutiveToolFailures: 5, MaxIdenticalGateRejections: 3,
		ContextCompactionThreshold: 24000,
	}
}

type StructuredOutcome struct {
	LoopID  string         `json:"loop_id"`
	Phase   RuntimePhase   `json:"phase"`
	Status  WorkflowStatus `json:"status"`
	Scope   model.RunScope `json:"scope"`
	Changes ChangeSet      `json:"changes"`
	Issues  []Issue        `json:"issues"`
	Summary string         `json:"summary"`
	Code    string         `json:"code,omitempty"`
	Message string         `json:"message,omitempty"`
}

type CommitContext struct {
	Changes               ChangeSet
	MaterializationProofs []MaterializationProof
}

type CommitMetadata func(context.Context, CommitContext) error

const (
	CodeCanceled                = "RUN_CANCELED"
	CodeRunAlreadyCanceled      = "RUN_ALREADY_CANCELED"
	CodeBudgetExceeded          = "RUNTIME_BUDGET_EXCEEDED"
	CodeConsecutiveErrors       = "CONSECUTIVE_TOOL_ERRORS"
	CodeDependencyFailed        = "DEPENDENCY_FAILED"
	CodeGateRejectedRepeated    = "COMPLETION_BLOCK_REPEAT"
	CodeCommitFailed            = "COMMIT_FAILED"
	CodeAgentFailed             = "AGENT_FAILED"
	CodeInvalidControlCall      = "INVALID_CONTROL_CALL"
	CodeScopeExpansion          = "SCOPE_EXPANSION_REQUIRED"
	CodeRevisionConflict        = "RUN_REVISION_CONFLICT"
	CodeCompletionGateBlocked   = "COMPLETION_GATE_BLOCK"
	CodeCompletionReviewBlocked = "COMPLETION_REVIEW_BLOCK"
	CodeFinishMessageEmpty      = "FINISH_MESSAGE_EMPTY"
)
