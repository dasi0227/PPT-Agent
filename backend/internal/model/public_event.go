package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"
)

const PublicEventSchemaVersion = 3

var PublicEventTypes = [...]EventType{
	EventRunStarted,
	EventRunProgress,
	EventRunCompleted,
	EventRunFailed,
	EventRunError,
	EventRunCanceled,
	EventRunResumed,
	EventPlanUpdated,
	EventPlanApprovalRequested,
	EventPlanApprovalAnswered,
	EventCommandPermissionRequested,
	EventCommandPermissionAnswered,
	EventScopeExpansionRequested,
	EventScopeExpansionAnswered,
	EventScopeUpdated,
	EventRunModeChanged,
	EventMessageReasoning,
	EventMessageMilestone,
	EventMessageFinal,
	EventToolStarted,
	EventToolCompleted,
	EventQuestionAsked,
	EventQuestionAnswered,
	EventContextWindowUpdated,
	EventContextCompacted,
}

type PublicEventBase struct {
	SchemaVersion int    `json:"schema_version"`
	RunID         string `json:"run_id"`
	OccurredAt    string `json:"occurred_at"`
}

func NewPublicEventBase(runID string) PublicEventBase {
	return PublicEventBase{
		SchemaVersion: PublicEventSchemaVersion,
		RunID:         runID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}
}

type PublicTarget struct {
	Type        string `json:"type"`
	SlideID     string `json:"slide_id,omitempty"`
	Part        string `json:"part"`
	DisplayName string `json:"display_name,omitempty"`
	Insertions  int    `json:"insertions,omitempty"`
	Deletions   int    `json:"deletions,omitempty"`
	LocalPath   string `json:"local_path,omitempty"`
	OpenURL     string `json:"open_url,omitempty"`
}

type PublicDisplay struct {
	Label  string `json:"label"`
	Detail string `json:"detail,omitempty"`
}

type PublicError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type RunStartedPayload struct {
	PublicEventBase
	Scope     RunScope               `json:"scope"`
	Mode      RunMode                `json:"mode"`
	UserInput string                 `json:"user_input"`
	Skills    []PublicSkill          `json:"skills,omitempty"`
	Resources []PublicLoadedResource `json:"resources,omitempty"`
}

type ProgressValue struct {
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Unit    string `json:"unit"`
}

type RunProgressPayload struct {
	PublicEventBase
	Stage    string         `json:"stage"`
	Text     string         `json:"text"`
	Target   *PublicTarget  `json:"target,omitempty"`
	Progress *ProgressValue `json:"progress,omitempty"`
}

type RunTerminalPayload struct {
	PublicEventBase
	DurationMS      int64           `json:"duration_ms"`
	AffectedTargets []PublicTarget  `json:"affected_targets"`
	Error           *PublicError    `json:"error"`
	TraceID         string          `json:"trace_id"`
	Reason          RunCancelReason `json:"reason,omitempty"`
}

func NewRunTerminalPayload(runID string, durationMS int64, affectedTargets []PublicTarget, publicError *PublicError) RunTerminalPayload {
	return NewRunTerminalPayloadFromBase(NewPublicEventBase(runID), durationMS, affectedTargets, publicError)
}

type RunResumedPayload struct {
	PublicEventBase
}

func NewRunTerminalPayloadFromBase(base PublicEventBase, durationMS int64, affectedTargets []PublicTarget, publicError *PublicError) RunTerminalPayload {
	if affectedTargets == nil {
		affectedTargets = []PublicTarget{}
	}
	return RunTerminalPayload{
		PublicEventBase: base,
		DurationMS:      durationMS,
		AffectedTargets: affectedTargets,
		Error:           publicError,
		TraceID:         base.RunID,
	}
}

type PublicPlanStep struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type PublicPlan struct {
	PlanID           string           `json:"plan_id"`
	Revision         int              `json:"revision"`
	ApprovedRevision int              `json:"approved_revision,omitempty"`
	Status           string           `json:"status"`
	Title            string           `json:"title"`
	Content          string           `json:"content"`
	Steps            []PublicPlanStep `json:"steps"`
}

type PlanUpdatedPayload struct {
	PublicEventBase
	Plan PublicPlan `json:"plan"`
}

type PlanApprovalRequestedPayload struct {
	PublicEventBase
	InteractionID string     `json:"interaction_id"`
	Plan          PublicPlan `json:"plan"`
}

type PlanApprovalAnswer struct {
	InteractionID    string `json:"interaction_id"`
	PlanID           string `json:"plan_id"`
	ExpectedRevision int    `json:"expected_revision"`
	Decision         string `json:"decision"`
	Feedback         string `json:"feedback,omitempty"`
	IdempotencyKey   string `json:"idempotency_key,omitempty"`
}

type PlanApprovalAnsweredPayload struct {
	PublicEventBase
	InteractionID string `json:"interaction_id"`
	PlanID        string `json:"plan_id"`
	Revision      int    `json:"revision"`
	Decision      string `json:"decision"`
	Feedback      string `json:"feedback,omitempty"`
}

type CommandPermissionAnswer struct {
	InteractionID string `json:"interaction_id"`
	CallID        string `json:"call_id"`
	CommandHash   string `json:"command_hash"`
	Decision      string `json:"decision"`
}

type CommandPermissionRequestedPayload struct {
	PublicEventBase
	InteractionID string `json:"interaction_id"`
	CallID        string `json:"call_id"`
	Command       string `json:"command"`
	CommandHash   string `json:"command_hash"`
	ReasonCode    string `json:"reason_code"`
	Reason        string `json:"reason"`
}

type CommandPermissionAnsweredPayload struct {
	PublicEventBase
	InteractionID string `json:"interaction_id"`
	CallID        string `json:"call_id"`
	CommandHash   string `json:"command_hash"`
	Decision      string `json:"decision"`
}

type ScopeExpansionAddition struct {
	SlideIDs []string    `json:"slide_ids,omitempty"`
	Object   ScopeObject `json:"object,omitempty"`
}

type ScopeExpansionRequestedPayload struct {
	PublicEventBase
	InteractionID     string                 `json:"interaction_id"`
	CallID            string                 `json:"call_id"`
	BaseRevision      int64                  `json:"base_revision"`
	CurrentScope      RunScope               `json:"current_scope"`
	RequestedAddition ScopeExpansionAddition `json:"requested_addition"`
	ProposedScope     RunScope               `json:"proposed_scope"`
	AffectedPageCount int                    `json:"affected_page_count"`
	Reason            string                 `json:"reason"`
}

type ScopeExpansionAnswer struct {
	InteractionID string               `json:"interaction_id"`
	CallID        string               `json:"call_id"`
	BaseRevision  int64                `json:"base_revision"`
	Decision      string               `json:"decision"`
	AdjustedScope *CreateRunScopeInput `json:"adjusted_scope,omitempty"`
}

type ScopeExpansionAnsweredPayload struct {
	PublicEventBase
	InteractionID string    `json:"interaction_id"`
	CallID        string    `json:"call_id"`
	BaseRevision  int64     `json:"base_revision"`
	Decision      string    `json:"decision"`
	AppliedScope  *RunScope `json:"applied_scope,omitempty"`
}

type ScopeUpdatedPayload struct {
	PublicEventBase
	PreviousScope RunScope `json:"previous_scope"`
	Scope         RunScope `json:"scope"`
	Cause         string   `json:"cause"`
	InteractionID string   `json:"interaction_id,omitempty"`
}

type RunModeChangedPayload struct {
	PublicEventBase
	PreviousMode RunMode `json:"previous_mode"`
	Mode         RunMode `json:"mode"`
}

type MessageReasoningPayload struct {
	PublicEventBase
	MessageID string `json:"message_id"`
	Text      string `json:"text"`
}

type MessageMilestonePayload struct {
	PublicEventBase
	MessageID        string   `json:"message_id"`
	Text             string   `json:"text"`
	CompletedStepIDs []string `json:"completed_step_ids"`
}

type MessageFinalPayload struct {
	PublicEventBase
	MessageID       string         `json:"message_id"`
	Text            string         `json:"text"`
	AffectedTargets []PublicTarget `json:"affected_targets,omitempty"`
}

type ToolStartedPayload struct {
	PublicEventBase
	CallID     string             `json:"call_id"`
	Tool       string             `json:"tool"`
	PlanStepID string             `json:"plan_step_id,omitempty"`
	Target     *PublicTarget      `json:"target,omitempty"`
	Display    PublicDisplay      `json:"display"`
	Command    *CommandProjection `json:"command,omitempty"`
}

type ToolPreview struct {
	SlideID  string   `json:"slide_id"`
	ImageURL string   `json:"image_url"`
	Warnings []string `json:"warnings"`
}

type PublicLoadedResource struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	OpenURL string `json:"open_url,omitempty"`
}

type ToolCompletedPayload struct {
	PublicEventBase
	CallID    string                 `json:"call_id"`
	Tool      string                 `json:"tool"`
	Status    string                 `json:"status"`
	Target    *PublicTarget          `json:"target,omitempty"`
	Display   PublicDisplay          `json:"display"`
	Preview   *ToolPreview           `json:"preview,omitempty"`
	Error     *PublicError           `json:"error,omitempty"`
	Command   *CommandProjection     `json:"command,omitempty"`
	Resources []PublicLoadedResource `json:"resources,omitempty"`
}

type CommandProjection struct {
	Text            string `json:"text"`
	Status          string `json:"status,omitempty"`
	ExitCode        *int   `json:"exit_code,omitempty"`
	DurationMS      *int64 `json:"duration_ms,omitempty"`
	OutputTruncated bool   `json:"output_truncated,omitempty"`
	StdoutPreview   string `json:"stdout_preview,omitempty"`
	StderrPreview   string `json:"stderr_preview,omitempty"`
}

type QuestionOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type QuestionField struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description,omitempty"`
	Options     []QuestionOption `json:"options"`
	AllowCustom bool             `json:"allow_custom"`
}

type QuestionAskedPayload struct {
	PublicEventBase
	QuestionID string          `json:"question_id"`
	Header     string          `json:"header,omitempty"`
	Questions  []QuestionField `json:"questions"`
}

type QuestionFieldAnswer struct {
	QuestionID       string `json:"question_id"`
	SelectedOptionID string `json:"selected_option_id,omitempty"`
	CustomText       string `json:"custom_text,omitempty"`
}

type QuestionAnswer struct {
	Answers []QuestionFieldAnswer `json:"answers"`
}

type QuestionAnsweredPayload struct {
	PublicEventBase
	QuestionID  string         `json:"question_id"`
	Answer      QuestionAnswer `json:"answer"`
	DisplayText string         `json:"display_text"`
}

type ContextWindowBucketDetail struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Layer  string `json:"layer"`
	Tokens int    `json:"tokens"`
}

type ContextWindowUpdatedPayload struct {
	PublicEventBase
	Total   int                                    `json:"total"`
	Max     int                                    `json:"max"`
	Ratio   float64                                `json:"ratio"`
	Status  string                                 `json:"status"`
	Buckets map[string]int                         `json:"buckets"`
	Details map[string][]ContextWindowBucketDetail `json:"details"`
}

type ContextCompactedPayload struct {
	PublicEventBase
	Compaction ContextCompaction `json:"compaction"`
}

func ValidatePublicEvent(event EventType, payload any) error {
	if !isPublicEventType(event) {
		return fmt.Errorf("event %q is not public", event)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	if intValue(data["schema_version"]) != PublicEventSchemaVersion {
		return errors.New("schema_version must be 3")
	}
	if strings.TrimSpace(stringValue(data["run_id"])) == "" {
		return errors.New("run_id is required")
	}
	occurredAt := stringValue(data["occurred_at"])
	parsed, err := time.Parse(time.RFC3339Nano, occurredAt)
	if err != nil || parsed.Location() != time.UTC {
		return errors.New("occurred_at must be RFC3339 UTC")
	}
	if forbiddenPublicField(data) {
		return errors.New("public payload contains a forbidden field")
	}

	switch event {
	case EventRunStarted:
		scope, ok := data["scope"].(map[string]any)
		if !ok {
			return errors.New("run scope is required")
		}
		rawScope, err := json.Marshal(scope)
		if err != nil {
			return err
		}
		var runScope RunScope
		if err := json.Unmarshal(rawScope, &runScope); err != nil {
			return errors.New("run scope is invalid")
		}
		command := RunCommand{
			Scope:       runScope,
			Mode:        RunMode(stringValue(data["mode"])),
			Instruction: stringValue(data["user_input"]),
		}
		if err := command.Validate(); err != nil {
			return err
		}
		if err := validatePublicSkills(data["skills"]); err != nil {
			return err
		}
		return validateLoadedResources(data["resources"])
	case EventRunProgress:
		if !oneOf(stringValue(data["stage"]), "thinking", "planning", "reading", "writing", "rendering", "finalizing") {
			return errors.New("invalid progress stage")
		}
		if err := requireString(data, "text"); err != nil {
			return err
		}
		if rawProgress, exists := data["progress"]; exists {
			progress, ok := rawProgress.(map[string]any)
			if !ok {
				return errors.New("progress must be an object")
			}
			current, total := intValue(progress["current"]), intValue(progress["total"])
			if !isInteger(progress["current"]) || !isInteger(progress["total"]) ||
				current < 0 || total <= 0 || current > total || strings.TrimSpace(stringValue(progress["unit"])) == "" {
				return errors.New("invalid progress value")
			}
		}
		if err := validateOptionalTarget(data["target"]); err != nil {
			return err
		}
	case EventRunCompleted, EventRunFailed, EventRunError, EventRunCanceled:
		if int64Value(data["duration_ms"]) < 0 || !isInteger(data["duration_ms"]) {
			return errors.New("duration_ms must be a non-negative integer")
		}
		if err := requireString(data, "trace_id"); err != nil {
			return err
		}
		if _, exists := data["affected_targets"]; !exists {
			return errors.New("affected_targets is required")
		}
		if _, exists := data["error"]; !exists {
			return errors.New("error is required")
		}
		if (event == EventRunFailed || event == EventRunError) && data["error"] == nil {
			return errors.New("failed/error run requires error")
		}
		if err := validateOptionalError(data["error"]); err != nil {
			return err
		}
		if err := validateTargets(data["affected_targets"]); err != nil {
			return err
		}
		if reason := stringValue(data["reason"]); reason != "" &&
			!oneOf(reason, string(RunCancelUserRequested), string(RunCancelSuperseded)) {
			return errors.New("invalid run cancellation reason")
		}
	case EventRunResumed:
	case EventPlanUpdated:
		return validatePlan(data["plan"])
	case EventPlanApprovalRequested:
		if strings.TrimSpace(stringValue(data["interaction_id"])) == "" {
			return errors.New("interaction_id is required")
		}
		return validatePlan(data["plan"])
	case EventPlanApprovalAnswered:
		if strings.TrimSpace(stringValue(data["interaction_id"])) == "" || strings.TrimSpace(stringValue(data["plan_id"])) == "" || intValue(data["revision"]) < 1 || !oneOf(stringValue(data["decision"]), "approve", "revise", "cancel") {
			return errors.New("invalid plan approval answer")
		}
		if stringValue(data["decision"]) == "revise" && strings.TrimSpace(stringValue(data["feedback"])) == "" {
			return errors.New("revision feedback is required")
		}
	case EventCommandPermissionRequested:
		if err := requireString(data, "interaction_id", "call_id", "command", "command_hash", "reason_code", "reason"); err != nil {
			return err
		}
	case EventCommandPermissionAnswered:
		if err := requireString(data, "interaction_id", "call_id", "command_hash", "decision"); err != nil {
			return err
		}
		if !oneOf(stringValue(data["decision"]), "allow_once", "deny") {
			return errors.New("invalid command permission decision")
		}
	case EventScopeExpansionRequested:
		if err := requireString(data, "interaction_id", "call_id", "reason"); err != nil || intValue(data["base_revision"]) < 1 {
			return errors.New("invalid scope expansion request")
		}
	case EventScopeExpansionAnswered:
		if err := requireString(data, "interaction_id", "call_id", "decision"); err != nil || intValue(data["base_revision"]) < 1 || !oneOf(stringValue(data["decision"]), "approve", "reject", "adjust") {
			return errors.New("invalid scope expansion answer")
		}
	case EventScopeUpdated:
		if err := requireString(data, "cause"); err != nil {
			return err
		}
	case EventRunModeChanged:
		if !oneOf(stringValue(data["previous_mode"]), "chat", "grill", "plan", "execute") || !oneOf(stringValue(data["mode"]), "chat", "grill", "plan", "execute") {
			return errors.New("invalid run mode transition")
		}
	case EventMessageReasoning, EventMessageMilestone, EventMessageFinal:
		if err := requireString(data, "message_id", "text"); err != nil {
			return err
		}
		if event == EventMessageMilestone {
			ids, ok := data["completed_step_ids"].([]any)
			if !ok || len(ids) == 0 {
				return errors.New("milestone requires completed_step_ids")
			}
			if err := validateStringIDs(ids, "completed_step_ids"); err != nil {
				return err
			}
		}
		if event == EventMessageFinal {
			if err := validateTargets(data["affected_targets"]); err != nil {
				return err
			}
		}
	case EventToolStarted:
		if err := requireString(data, "call_id", "tool"); err != nil {
			return err
		}
		if !isBusinessTool(stringValue(data["tool"])) {
			return errors.New("control or unknown tool cannot be public")
		}
		if err := validateOptionalTarget(data["target"]); err != nil {
			return err
		}
		if err := validateDisplay(data["display"]); err != nil {
			return err
		}
		if (stringValue(data["tool"]) == "run_command") != (data["command"] != nil) {
			return errors.New("command projection must appear only on run_command events")
		}
		return validateCommandProjection(data["command"], false)
	case EventToolCompleted:
		if err := requireString(data, "call_id", "tool", "status"); err != nil {
			return err
		}
		if !isBusinessTool(stringValue(data["tool"])) || !oneOf(stringValue(data["status"]), "completed", "blocked", "failed") {
			return errors.New("invalid tool completion")
		}
		if stringValue(data["status"]) == "failed" && data["error"] == nil {
			return errors.New("failed tool requires error")
		}
		if err := validateOptionalError(data["error"]); err != nil {
			return err
		}
		if err := validateOptionalTarget(data["target"]); err != nil {
			return err
		}
		if err := validateDisplay(data["display"]); err != nil {
			return err
		}
		if (stringValue(data["tool"]) == "run_command") != (data["command"] != nil) {
			return errors.New("command projection must appear only on run_command events")
		}
		if err := validateCommandProjection(data["command"], true); err != nil {
			return err
		}
		if err := validateLoadedResources(data["resources"]); err != nil {
			return err
		}
		if rawPreview, exists := data["preview"]; exists {
			preview, ok := rawPreview.(map[string]any)
			if !ok {
				return errors.New("preview must be an object")
			}
			if err := requireString(preview, "slide_id", "image_url"); err != nil {
				return err
			}
			imageURL := stringValue(preview["image_url"])
			parsedURL, parseErr := url.Parse(imageURL)
			expectedPrefix := "/api/v1/runs/" + stringValue(data["run_id"]) + "/"
			if parseErr != nil || !strings.HasPrefix(parsedURL.Path, expectedPrefix) || parsedURL.IsAbs() {
				return errors.New("preview image_url must be a controlled HTTP path")
			}
			if warnings, ok := preview["warnings"].([]any); !ok {
				return errors.New("preview warnings must be an array")
			} else if err := validateStringList(warnings, "preview warnings"); err != nil {
				return err
			}
		}
	case EventQuestionAsked:
		if err := requireString(data, "question_id"); err != nil {
			return err
		}
		for _, field := range []string{"prompt", "selection", "options", "allow_custom"} {
			if _, exists := data[field]; exists {
				return fmt.Errorf("legacy question field %s is not allowed", field)
			}
		}
		questions, ok := data["questions"].([]any)
		if !ok || len(questions) == 0 {
			return errors.New("questions are required")
		}
		if err := validateQuestionFields(questions); err != nil {
			return err
		}
	case EventQuestionAnswered:
		if err := requireString(data, "question_id", "display_text"); err != nil {
			return err
		}
		if _, ok := data["answer"].(map[string]any); !ok {
			return errors.New("answer is required")
		}
		answer := data["answer"].(map[string]any)
		for _, field := range []string{"selected_option_ids", "custom_text"} {
			if _, exists := answer[field]; exists {
				return fmt.Errorf("legacy answer field %s is not allowed", field)
			}
		}
		answers, ok := answer["answers"].([]any)
		if !ok || len(answers) == 0 {
			return errors.New("answer.answers are required")
		}
		if err := validateQuestionAnswerFields(answers); err != nil {
			return err
		}
	case EventContextWindowUpdated:
		if !isInteger(data["total"]) || !isInteger(data["max"]) ||
			intValue(data["total"]) < 0 || intValue(data["max"]) <= 0 {
			return errors.New("invalid context window totals")
		}
		ratio, ok := data["ratio"].(float64)
		if !ok || ratio < 0 {
			return errors.New("invalid context window ratio")
		}
		if !oneOf(stringValue(data["status"]), "running", "warning", "compacting", "idle") {
			return errors.New("invalid context window status")
		}
		buckets, ok := data["buckets"].(map[string]any)
		if !ok {
			return errors.New("context window buckets are required")
		}
		for _, key := range []string{"read_ppt", "run_command", "system_prompt", "user_prompt", "chat_history", "other"} {
			if !isInteger(buckets[key]) || intValue(buckets[key]) < 0 {
				return errors.New("invalid context window bucket " + key)
			}
		}
	case EventContextCompacted:
		compaction, ok := data["compaction"].(map[string]any)
		if !ok || strings.TrimSpace(stringValue(compaction["id"])) == "" ||
			!oneOf(stringValue(compaction["trigger"]), "auto", "manual") ||
			strings.TrimSpace(stringValue(compaction["summary"])) == "" {
			return errors.New("invalid context compaction")
		}
	}
	return nil
}

func validatePublicSkills(value any) error {
	if value == nil {
		return nil
	}
	skills, ok := value.([]any)
	if !ok || len(skills) > MaxRunSkills {
		return errors.New("skills must be an array with at most three items")
	}
	seen := map[string]bool{}
	for _, raw := range skills {
		skill, ok := raw.(map[string]any)
		if !ok {
			return errors.New("skill must be an object")
		}
		id := strings.TrimSpace(stringValue(skill["id"]))
		if id == "" || seen[id] {
			return errors.New("skill ids must be present and unique")
		}
		if err := requireString(skill, "name", "description"); err != nil {
			return err
		}
		seen[id] = true
	}
	return nil
}

func validateLoadedResources(value any) error {
	if value == nil {
		return nil
	}
	values, ok := value.([]any)
	if !ok {
		return errors.New("resources must be an array")
	}
	for _, raw := range values {
		resource, ok := raw.(map[string]any)
		if !ok || !oneOf(stringValue(resource["kind"]), "component", "skill") {
			return errors.New("invalid loaded resource")
		}
		if err := requireString(resource, "id", "name"); err != nil {
			return err
		}
	}
	return nil
}

func validateQuestionOptions(data map[string]any) error {
	options, ok := data["options"].([]any)
	if !ok {
		return errors.New("question options are required")
	}
	if len(options) > 3 {
		return errors.New("question options cannot exceed 3")
	}
	allowCustom, ok := data["allow_custom"].(bool)
	if !ok {
		return errors.New("allow_custom is required")
	}
	if len(options) == 0 && !allowCustom {
		return errors.New("question requires options or a custom answer")
	}
	seen := map[string]bool{}
	for _, raw := range options {
		option, ok := raw.(map[string]any)
		if !ok {
			return errors.New("invalid question option")
		}
		if err := requireString(option, "id", "label"); err != nil {
			return err
		}
		id := stringValue(option["id"])
		if seen[id] {
			return errors.New("question option ids must be unique")
		}
		seen[id] = true
	}
	return nil
}

func validateQuestionFields(questions []any) error {
	seenQuestions := map[string]bool{}
	for _, raw := range questions {
		question, ok := raw.(map[string]any)
		if !ok {
			return errors.New("invalid question item")
		}
		if err := requireString(question, "id", "title"); err != nil {
			return err
		}
		id := stringValue(question["id"])
		if seenQuestions[id] {
			return errors.New("question ids must be unique")
		}
		seenQuestions[id] = true
		allowCustom, ok := question["allow_custom"].(bool)
		if !ok {
			return errors.New("question allow_custom is required")
		}
		options, ok := question["options"].([]any)
		if !ok {
			return errors.New("question options are required")
		}
		if len(options) > 3 {
			return errors.New("question options cannot exceed 3")
		}
		if len(options) == 0 && !allowCustom {
			return errors.New("fill-in question requires allow_custom")
		}
		seenOptions := map[string]bool{}
		for _, rawOption := range options {
			option, ok := rawOption.(map[string]any)
			if !ok {
				return errors.New("invalid question option")
			}
			if err := requireString(option, "id", "label"); err != nil {
				return err
			}
			optionID := stringValue(option["id"])
			if seenOptions[optionID] {
				return errors.New("question option ids must be unique")
			}
			seenOptions[optionID] = true
		}
	}
	return nil
}

func validateQuestionAnswerFields(answers []any) error {
	seen := map[string]bool{}
	for _, raw := range answers {
		answer, ok := raw.(map[string]any)
		if !ok {
			return errors.New("invalid question answer")
		}
		if err := requireString(answer, "question_id"); err != nil {
			return err
		}
		id := stringValue(answer["question_id"])
		if seen[id] {
			return errors.New("question answer ids must be unique")
		}
		seen[id] = true
		selected, hasSelected := answer["selected_option_id"].(string)
		custom, hasCustom := answer["custom_text"].(string)
		if (hasSelected && strings.TrimSpace(selected) != "") == (hasCustom && strings.TrimSpace(custom) != "") {
			return errors.New("question answer requires exactly one selected option or custom text")
		}
	}
	return nil
}

func isPublicEventType(event EventType) bool {
	for _, candidate := range PublicEventTypes {
		if event == candidate {
			return true
		}
	}
	return false
}

func isBusinessTool(name string) bool {
	return oneOf(name, "read_ppt", "mutate_ppt", "render_slide", "run_command", "load_component", "load_skill")
}

func forbiddenPublicField(value any) bool {
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			switch strings.ToLower(key) {
			case "args", "arguments", "html", "observation", "result", "path",
				"screenshot_path", "reasoning_content", "provider_reasoning":
				return true
			}
			if forbiddenPublicField(child) {
				return true
			}
		}
	case []any:
		for _, child := range current {
			if forbiddenPublicField(child) {
				return true
			}
		}
	}
	return false
}

func validatePlan(value any) error {
	plan, ok := value.(map[string]any)
	if !ok {
		return errors.New("plan is required")
	}
	if err := requireString(plan, "plan_id"); err != nil {
		return err
	}
	if intValue(plan["revision"]) < 1 {
		return errors.New("plan revision must be positive")
	}
	if !isInteger(plan["revision"]) {
		return errors.New("plan revision must be an integer")
	}
	if err := requireString(plan, "title", "content", "status"); err != nil {
		return err
	}
	if !oneOf(stringValue(plan["status"]), "awaiting_approval", "active", "completed", "canceled") {
		return errors.New("invalid plan status")
	}
	steps, ok := plan["steps"].([]any)
	if !ok || len(steps) == 0 {
		return errors.New("plan steps are required")
	}
	inProgress := 0
	seen := map[string]bool{}
	for _, raw := range steps {
		step, ok := raw.(map[string]any)
		if !ok {
			return errors.New("invalid plan step")
		}
		if err := requireString(step, "id", "title", "status"); err != nil {
			return err
		}
		id, status := stringValue(step["id"]), stringValue(step["status"])
		if seen[id] || !oneOf(status, "pending", "in_progress", "completed", "failed") {
			return errors.New("invalid plan step")
		}
		seen[id] = true
		if status == "in_progress" {
			inProgress++
		}
	}
	if inProgress > 1 {
		return errors.New("at most one plan step may be in progress")
	}
	return nil
}

func validateDisplay(value any) error {
	display, ok := value.(map[string]any)
	if !ok {
		return errors.New("display is required")
	}
	if err := requireString(display, "label"); err != nil {
		return err
	}
	for _, key := range []string{"label", "detail"} {
		if text, exists := display[key]; exists {
			if _, ok := text.(string); !ok {
				return errors.New("display text must be a string")
			}
		}
	}
	return nil
}

func validateCommandProjection(value any, terminal bool) error {
	if value == nil {
		return nil
	}
	command, ok := value.(map[string]any)
	if !ok {
		return errors.New("command projection must be an object")
	}
	if err := requireString(command, "text"); err != nil {
		return err
	}
	if !terminal {
		for _, key := range []string{"status", "exit_code", "duration_ms", "stdout_preview", "stderr_preview"} {
			if _, exists := command[key]; exists {
				return errors.New("started command projection contains terminal fields")
			}
		}
		return nil
	}
	if !oneOf(stringValue(command["status"]), "completed", "blocked", "failed") {
		return errors.New("invalid command projection status")
	}
	if value, exists := command["exit_code"]; exists && (!isInteger(value) || intValue(value) < -1) {
		return errors.New("invalid command exit_code")
	}
	if value, exists := command["duration_ms"]; exists && (!isInteger(value) || int64Value(value) < 0) {
		return errors.New("invalid command duration_ms")
	}
	return nil
}

func validateTargets(value any) error {
	if value == nil {
		return nil
	}
	targets, ok := value.([]any)
	if !ok {
		return errors.New("affected_targets must be an array")
	}
	for _, target := range targets {
		if err := validatePublicTarget(target); err != nil {
			return err
		}
	}
	return nil
}

func validateOptionalTarget(value any) error {
	if value == nil {
		return nil
	}
	return validatePublicTarget(value)
}

func validatePublicTarget(value any) error {
	target, ok := value.(map[string]any)
	if !ok {
		return errors.New("target must be an object")
	}
	part := stringValue(target["part"])
	if value, ok := target["insertions"]; ok && (!isInteger(value) || intValue(value) < 0) {
		return errors.New("target insertions must be a non-negative integer")
	}
	if value, ok := target["deletions"]; ok && (!isInteger(value) || intValue(value) < 0) {
		return errors.New("target deletions must be a non-negative integer")
	}
	if strings.TrimSpace(stringValue(target["local_path"])) != "" &&
		strings.Contains(stringValue(target["local_path"]), "\x00") {
		return errors.New("target local_path is invalid")
	}
	switch stringValue(target["type"]) {
	case "deck":
		if strings.TrimSpace(stringValue(target["slide_id"])) != "" {
			return errors.New("deck target must not contain slide_id")
		}
		if !oneOf(part, "manifest", "outline", "design") {
			return errors.New("deck target part must be manifest, outline, or design")
		}
	case "slide":
		if strings.TrimSpace(stringValue(target["slide_id"])) == "" {
			return errors.New("slide target requires slide_id")
		}
		if !oneOf(part, "spec", "html") {
			return errors.New("slide target part must be spec or html")
		}
	case "file":
		if strings.TrimSpace(stringValue(target["slide_id"])) != "" || part != "content" ||
			strings.TrimSpace(stringValue(target["display_name"])) == "" {
			return errors.New("file target requires content part and display_name")
		}
	default:
		return errors.New("invalid public target type")
	}
	return nil
}

func validateOptionalError(value any) error {
	if value == nil {
		return nil
	}
	publicError, ok := value.(map[string]any)
	if !ok {
		return errors.New("error must be an object")
	}
	if err := requireString(publicError, "code", "message"); err != nil {
		return err
	}
	if _, ok := publicError["retryable"].(bool); !ok {
		return errors.New("error.retryable must be a boolean")
	}
	return nil
}

func validateStringIDs(values []any, field string) error {
	seen := map[string]bool{}
	for _, value := range values {
		id, ok := value.(string)
		id = strings.TrimSpace(id)
		if !ok || id == "" || seen[id] {
			return fmt.Errorf("%s must contain unique non-empty strings", field)
		}
		seen[id] = true
	}
	return nil
}

func validateStringList(values []any, field string) error {
	for _, value := range values {
		_, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s must contain strings", field)
		}
	}
	return nil
}

func require(data map[string]any, keys ...string) error {
	for _, key := range keys {
		if value, ok := data[key]; !ok || value == nil {
			return fmt.Errorf("%s is required", key)
		}
	}
	return nil
}

func requireString(data map[string]any, keys ...string) error {
	for _, key := range keys {
		if strings.TrimSpace(stringValue(data[key])) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	return nil
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func intValue(value any) int {
	switch number := value.(type) {
	case float64:
		return int(number)
	case int:
		return number
	case int64:
		return int(number)
	default:
		reflected := reflect.ValueOf(value)
		if reflected.IsValid() && reflected.Kind() >= reflect.Int && reflected.Kind() <= reflect.Int64 {
			return int(reflected.Int())
		}
		return 0
	}
}

func int64Value(value any) int64 {
	switch number := value.(type) {
	case float64:
		return int64(number)
	case int:
		return int64(number)
	case int64:
		return number
	default:
		return -1
	}
}

func isInteger(value any) bool {
	switch number := value.(type) {
	case float64:
		return number == float64(int64(number))
	case int, int64:
		return true
	default:
		return false
	}
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
