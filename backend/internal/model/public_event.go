package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"
)

const PublicEventSchemaVersion = 1

var PublicEventTypes = [...]EventType{
	EventRunStarted,
	EventRunProgress,
	EventRunFinished,
	EventPlanUpdated,
	EventMessageReasoning,
	EventMessageMilestone,
	EventMessageFinal,
	EventToolStarted,
	EventToolCompleted,
	EventQuestionAsked,
	EventQuestionAnswered,
}

var rawHTMLPattern = regexp.MustCompile(`(?i)<\s*/?\s*[a-z][a-z0-9-]*(?:\s+[^>]*)?/?\s*>`)

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
	Type    string `json:"type"`
	SlideID string `json:"slide_id,omitempty"`
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
	Target      RunTarget      `json:"target"`
	Interaction RunInteraction `json:"interaction"`
	UserInput   string         `json:"user_input"`
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

type RunFinishedPayload struct {
	PublicEventBase
	Status          string         `json:"status"`
	AffectedTargets []PublicTarget `json:"affected_targets,omitempty"`
	Error           *PublicError   `json:"error,omitempty"`
	DurationMS      int64          `json:"duration_ms"`
}

type PublicPlanStep struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type PublicPlan struct {
	PlanID      string           `json:"plan_id"`
	Revision    int              `json:"revision"`
	Explanation string           `json:"explanation,omitempty"`
	Steps       []PublicPlanStep `json:"steps"`
}

type PlanUpdatedPayload struct {
	PublicEventBase
	Plan PublicPlan `json:"plan"`
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
	CallID     string        `json:"call_id"`
	Tool       string        `json:"tool"`
	PlanStepID string        `json:"plan_step_id,omitempty"`
	Target     *PublicTarget `json:"target,omitempty"`
	Display    PublicDisplay `json:"display"`
}

type ToolPreview struct {
	SlideID  string   `json:"slide_id"`
	ImageURL string   `json:"image_url"`
	Warnings []string `json:"warnings"`
}

type ToolCompletedPayload struct {
	PublicEventBase
	CallID  string        `json:"call_id"`
	Tool    string        `json:"tool"`
	Status  string        `json:"status"`
	Display PublicDisplay `json:"display"`
	Preview *ToolPreview  `json:"preview,omitempty"`
	Error   *PublicError  `json:"error,omitempty"`
}

type QuestionOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type QuestionAskedPayload struct {
	PublicEventBase
	QuestionID  string           `json:"question_id"`
	Header      string           `json:"header,omitempty"`
	Prompt      string           `json:"prompt"`
	Selection   string           `json:"selection"`
	Options     []QuestionOption `json:"options"`
	AllowCustom bool             `json:"allow_custom"`
}

type QuestionAnswer struct {
	SelectedOptionIDs []string `json:"selected_option_ids"`
	CustomText        string   `json:"custom_text"`
}

type QuestionAnsweredPayload struct {
	PublicEventBase
	QuestionID  string         `json:"question_id"`
	Answer      QuestionAnswer `json:"answer"`
	DisplayText string         `json:"display_text"`
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
		return errors.New("schema_version must be 1")
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
		target, ok := data["target"].(map[string]any)
		if !ok {
			return errors.New("run target is required")
		}
		interaction, ok := data["interaction"].(map[string]any)
		if !ok {
			return errors.New("run interaction is required")
		}
		spec := WorkSpec{
			Target: RunTarget{
				Artifact: Artifact(stringValue(target["artifact"])),
				Level:    TargetLevel(stringValue(target["level"])),
				SlideID:  stringValue(target["slide_id"]),
			},
			Interaction: RunInteraction{Intent: InteractionIntent(stringValue(interaction["intent"]))},
			Instruction: stringValue(data["user_input"]),
		}
		return spec.Validate()
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
	case EventRunFinished:
		if !oneOf(stringValue(data["status"]), "completed", "failed", "canceled") {
			return errors.New("invalid run status")
		}
		if stringValue(data["status"]) == "failed" && data["error"] == nil {
			return errors.New("failed run requires error")
		}
		if int64Value(data["duration_ms"]) < 0 || !isInteger(data["duration_ms"]) {
			return errors.New("duration_ms must be a non-negative integer")
		}
		if err := validateOptionalError(data["error"]); err != nil {
			return err
		}
		if err := validateTargets(data["affected_targets"]); err != nil {
			return err
		}
	case EventPlanUpdated:
		return validatePlan(data["plan"])
	case EventMessageReasoning, EventMessageMilestone, EventMessageFinal:
		if err := requireString(data, "message_id", "text"); err != nil {
			return err
		}
		if rawHTMLPattern.MatchString(stringValue(data["text"])) {
			return errors.New("message text must not contain raw HTML")
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
		return validateDisplay(data["display"])
	case EventToolCompleted:
		if err := requireString(data, "call_id", "tool", "status"); err != nil {
			return err
		}
		if !isBusinessTool(stringValue(data["tool"])) || !oneOf(stringValue(data["status"]), "completed", "failed") {
			return errors.New("invalid tool completion")
		}
		if stringValue(data["status"]) == "failed" && data["error"] == nil {
			return errors.New("failed tool requires error")
		}
		if err := validateOptionalError(data["error"]); err != nil {
			return err
		}
		if err := validateDisplay(data["display"]); err != nil {
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
		if err := requireString(data, "question_id", "prompt", "selection"); err != nil {
			return err
		}
		if rawHTMLPattern.MatchString(stringValue(data["prompt"])) ||
			rawHTMLPattern.MatchString(stringValue(data["header"])) {
			return errors.New("question text must not contain raw HTML")
		}
		if !oneOf(stringValue(data["selection"]), "single", "multiple") {
			return errors.New("invalid question selection")
		}
		if err := validateQuestionOptions(data); err != nil {
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
		selected, ok := answer["selected_option_ids"].([]any)
		if !ok {
			return errors.New("selected_option_ids must be an array")
		}
		if err := validateStringIDs(selected, "selected_option_ids"); err != nil {
			return err
		}
		if _, ok := answer["custom_text"].(string); !ok {
			return errors.New("custom_text must be a string")
		}
	}
	return nil
}

func validateQuestionOptions(data map[string]any) error {
	options, ok := data["options"].([]any)
	if !ok {
		return errors.New("question options are required")
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
		if rawHTMLPattern.MatchString(stringValue(option["label"])) ||
			rawHTMLPattern.MatchString(stringValue(option["description"])) {
			return errors.New("question options must not contain raw HTML")
		}
		id := stringValue(option["id"])
		if seen[id] {
			return errors.New("question option ids must be unique")
		}
		seen[id] = true
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
	return oneOf(name, "read_ppt", "write_ppt", "edit_ppt", "search_refs", "render_slide")
}

func forbiddenPublicField(value any) bool {
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			switch strings.ToLower(key) {
			case "args", "arguments", "html", "observation", "result", "path", "local_path",
				"screenshot_path", "hash", "reasoning_content", "provider_reasoning":
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
	if rawHTMLPattern.MatchString(stringValue(plan["explanation"])) {
		return errors.New("plan explanation must not contain raw HTML")
	}
	if !isInteger(plan["revision"]) {
		return errors.New("plan revision must be an integer")
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
		if rawHTMLPattern.MatchString(stringValue(step["title"])) {
			return errors.New("plan step title must not contain raw HTML")
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
			value, ok := text.(string)
			if !ok || rawHTMLPattern.MatchString(value) {
				return errors.New("display text must be a plain string without raw HTML")
			}
		}
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
	switch stringValue(target["type"]) {
	case "global":
		if strings.TrimSpace(stringValue(target["slide_id"])) != "" {
			return errors.New("global target must not contain slide_id")
		}
	case "slide":
		if strings.TrimSpace(stringValue(target["slide_id"])) == "" {
			return errors.New("slide target requires slide_id")
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
	if rawHTMLPattern.MatchString(stringValue(publicError["message"])) {
		return errors.New("error message must not contain raw HTML")
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
		text, ok := value.(string)
		if !ok || rawHTMLPattern.MatchString(text) {
			return fmt.Errorf("%s must contain safe strings", field)
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
