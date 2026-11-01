package workflow

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/decision"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/prompt"
)

const toolDecisionPolicy = "global-tools-v5"

// Tool capability checks retain accepted input IDs and text independently of
// transcript compaction. Attachments and selections remain in the transcript.
type ReviewInstruction struct {
	InputID string `json:"input_id,omitempty"`
	Text    string `json:"text"`
}

type RunToolDecision struct {
	InputID        string                           `json:"input_id"`
	Mode           model.RunMode                    `json:"mode"`
	InputHash      string                           `json:"input_hash"`
	ConfigIdentity string                           `json:"config_identity"`
	Model          string                           `json:"model,omitempty"`
	Policy         string                           `json:"policy"`
	Status         string                           `json:"status"`
	Reason         string                           `json:"reason,omitempty"`
	Choices        map[string]decision.ChoiceAnswer `json:"choices,omitempty"`
	AllowedTools   []string                         `json:"allowed_tools"`
	Usage          decision.Usage                   `json:"usage"`
}

type toolDecisionHistoryStore interface {
	GetRun(context.Context, string) (model.Run, error)
	LoadThreadConversationContextBeforeInput(context.Context, string, string, string) (model.ThreadConversationContext, error)
}

func toolDecisionRequest(source model.ThreadConversationContext, text string) (decision.Request, string, error) {
	rules, err := prompt.Load("jev_prune")
	if err != nil {
		return decision.Request{}, "", err
	}
	history := contextengine.BuildConversationHistory(source.Rounds, contextengine.DecisionHistoryWindow())
	state := prompt.RenderGroup("system_prompt", rules) + "\n\n" + history.Text +
		"\n\n<current_user_input>\n" + contextengine.EscapeCommandContextText(text) + "\n</current_user_input>"
	questions := map[string]decision.Question{}
	for id, instructions := range map[string]string{
		"meta":        "Is creating or updating presentation basics needed to fulfill current_user_input under jev_prune?",
		"constraints": "Is creating or updating presentation-wide content or visual requirements and prohibitions needed to fulfill current_user_input under jev_prune?",
		"outline":     "Is creating or changing the presentation's sections, subsections, or slides needed to fulfill current_user_input under jev_prune?",
	} {
		questions[id] = decision.ChoiceQuestion{Instructions: instructions, Criteria: map[string]any{
			"needed":     "Required by the task.",
			"possible":   "May be required, or cannot be ruled out.",
			"not_needed": "Clearly unnecessary for the entire task.",
		}}
	}
	req := decision.Request{State: state, Questions: questions}
	// Request includes the exact rules, questions and material. Version and
	// policy metadata are hashed locally, never added to the model's state.
	fingerprint := hashCheckpointValue(struct {
		Request decision.Request
		Version string
		Policy  string
	}{req, rules.Version, toolDecisionPolicy})
	return req, fingerprint, nil
}

func currentToolDecisionInput(ctx context.Context, input RuntimeInput, state *RunState) (string, model.ThreadConversationContext, string, error) {
	reader, ok := input.Idempotency.(toolDecisionHistoryStore)
	if !ok {
		return "", model.ThreadConversationContext{}, "", errors.New("tool decision history store is required")
	}
	current, err := reader.GetRun(ctx, state.runID)
	if err != nil {
		return "", model.ThreadConversationContext{}, "", err
	}
	id, text := "initial:"+current.ClientRequestID, current.Command.Instruction
	// These original accepted inputs survive transcript compaction and are
	// checkpointed before their inbox receipts are consumed. Do not merge them.
	for i := len(state.reviewInstructions) - 1; i >= 0; i-- {
		item := state.reviewInstructions[i]
		if strings.HasPrefix(item.InputID, "steering:") {
			id, text = strings.TrimPrefix(item.InputID, "steering:"), item.Text
			break
		}
	}
	source, err := reader.LoadThreadConversationContextBeforeInput(ctx, current.ThreadID, current.ID, id)
	return id, source, text, err
}

func (r *Runtime) decideTools(ctx context.Context, input RuntimeInput, state *RunState) error {
	result := &RunToolDecision{Mode: state.mode, ConfigIdentity: state.decisionIdentity, Policy: toolDecisionPolicy,
		Status: "skipped", Reason: "read_only_mode", Choices: map[string]decision.ChoiceAnswer{}}
	var req decision.Request
	var materialErr error
	var started time.Time
	if state.mode == model.ModeCreate {
		var source model.ThreadConversationContext
		var text string
		result.InputID, source, text, materialErr = currentToolDecisionInput(ctx, input, state)
		if materialErr == nil {
			req, result.InputHash, materialErr = toolDecisionRequest(source, text)
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if cached := state.toolDecision; cached != nil && cached.Mode == result.Mode && cached.InputID == result.InputID &&
		cached.InputHash == result.InputHash && cached.ConfigIdentity == result.ConfigIdentity && cached.Policy == result.Policy &&
		(materialErr == nil || cached.Reason == "context_unavailable") {
		applyToolDecision(state, cached)
		return nil
	}
	if state.mode != model.ModeCreate {
		// Read-only modes never invoke Jev, and entering Create invalidates this.
	} else if materialErr != nil {
		result.Status, result.Reason = "unavailable", "context_unavailable"
	} else if !decision.Fits(req) {
		result.Status, result.Reason = "unavailable", "capacity"
	} else if r.Decisions.Provider == nil {
		result.Reason = "disabled"
	} else if r.Decisions.Identity != state.decisionIdentity {
		result.Status, result.Reason = "unavailable", "configuration_changed"
	} else {
		callCtx, cancel := r.decisionContext(ctx, state, 3*time.Second)
		started = time.Now()
		response, err := r.Decisions.Provider.Evaluate(callCtx, req)
		cancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			result.Status, result.Reason = "unavailable", decision.Failure(err)
		} else {
			result.Status, result.Reason = "completed", ""
			result.Model, result.Usage = response.Model, response.Usage
			for _, id := range []string{"meta", "constraints", "outline"} {
				answer, err := response.Choice(id)
				if err != nil {
					result.Reason = "partial_response_kept"
					continue
				}
				result.Choices[id] = answer
			}
		}
	}
	applyToolDecision(state, result)
	if !started.IsZero() {
		recordTrace(input.Trace, state.runID, "decision.tools", map[string]any{"result": result, "elapsed_ms": time.Since(started).Milliseconds()})
	}
	state.toolDecision = result
	return r.saveCheckpoint(ctx, input, state, checkpointBoundary("tools_decided"))
}

func applyToolDecision(state *RunState, result *RunToolDecision) {
	// Reset the prior limit before deriving today's mode/phase/scope capabilities.
	// Reusing a decision must not prevent registry rebuilds or hide newly allowed
	// capabilities. Only these three explicit global capabilities can be pruned.
	state.tools.allowed = nil
	removed := map[string]bool{}
	if result != nil && result.Mode == state.mode && result.Status == "completed" && result.Policy == toolDecisionPolicy {
		for id, name := range map[string]string{"meta": "edit_meta", "constraints": "edit_constraints", "outline": "edit_outline"} {
			answer, ok := result.Choices[id]
			if ok && answer.Choice == "not_needed" && answer.Probabilities["not_needed"] >= 0.95 && answer.Confidence >= 0.85 {
				removed[name] = true
			}
		}
	}
	allowed := []string{}
	for _, schema := range state.tools.Disclose(state.phase, state.mode, state.scope) {
		if !removed[schema.Name] {
			allowed = append(allowed, schema.Name)
		}
	}
	sort.Strings(allowed)
	state.tools.Limit(allowed)
	if result != nil {
		result.AllowedTools = allowed
	}
}

func (r *ToolRegistry) Limit(names []string) {
	r.allowed = map[string]bool{}
	for _, name := range names {
		r.allowed[name] = true
	}
}

func (r *Runtime) decisionContext(ctx context.Context, state *RunState, limit time.Duration) (context.Context, context.CancelFunc) {
	if state.budget.MaxDuration > 0 {
		remaining := state.budget.MaxDuration - (state.activeDurationAt(r.clockNow()) - time.Duration(state.budgetBaseDurationMS)*time.Millisecond)
		if remaining < limit {
			limit = remaining
		}
	}
	return context.WithTimeout(ctx, limit)
}
