package llm

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

const MaxSubmissionAdditionalRequests = 2

type submissionIdentityKey struct{}

func WithSubmissionIdentity(ctx context.Context, commandID, attemptID string) context.Context {
	return context.WithValue(ctx, submissionIdentityKey{}, [2]string{commandID, attemptID})
}

// SubmissionSession owns the entire command's correction and output budgets.
// Compact reuses the same session for its necessary input batches. Validators
// must be pure: no business effect is applied until Generate returns success.
type SubmissionSession struct {
	Command        string
	Diagnose       func(map[string]any)
	Check          func(context.Context) error
	policy         ToolConstraintPolicy
	id             string
	started        time.Time
	requests       int
	additional     int
	corrections    int
	unsupported    int
	networkRetries int
	httpRequests   int
	modelRequests  int
	outputBudget   int
	outputUsed     int
}

func NewSubmissionSession(command string, outputBudget int) *SubmissionSession {
	return &SubmissionSession{Command: command, id: uuid.NewString(), started: time.Now(), outputBudget: outputBudget}
}

func (s *SubmissionSession) report(ctx context.Context, event string, fields map[string]any) {
	if fields == nil {
		fields = map[string]any{}
	}
	fields["event"], fields["command"], fields["submission_id"] = event, s.Command, s.id
	fields["generate_requests"], fields["model_requests"], fields["http_requests"] = s.requests, s.modelRequests, s.httpRequests
	fields["network_retries"], fields["corrections"], fields["unsupported_retries"] = s.networkRetries, s.corrections, s.unsupported
	fields["additional_requests"], fields["requests_remaining"] = s.additional, MaxSubmissionAdditionalRequests-s.additional
	fields["output_tokens_used"], fields["output_tokens_remaining"] = s.outputUsed, s.outputBudget-s.outputUsed
	fields["elapsed_ms"] = time.Since(s.started).Milliseconds()
	if deadline, ok := ctx.Deadline(); ok {
		fields["deadline_remaining_ms"] = max(int64(0), time.Until(deadline).Milliseconds())
	}
	if identity, ok := ctx.Value(submissionIdentityKey{}).([2]string); ok {
		fields["command_id"], fields["attempt_id"] = identity[0], identity[1]
	}
	if s.Diagnose != nil {
		s.Diagnose(fields)
	} else {
		slog.Info("command model submission", "diagnostic", fields)
	}
}

func (s *SubmissionSession) check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.Check != nil {
		return s.Check(ctx)
	}
	return nil
}

func (s *SubmissionSession) Generate(ctx context.Context, provider Provider, req GenerateRequest, validate func(GenerateResponse) error, guidance string) (result GenerateResponse, resultErr error) {
	if provider == nil {
		return result, errors.New("submission provider is unavailable")
	}
	strategy := ToolStrategy{Provider: provider.Name(), Model: provider.Model()}
	termination := "accepted"
	defer func() {
		if resultErr != nil && termination == "accepted" {
			termination = "failed"
		}
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(resultErr, context.Canceled) {
			termination = "canceled_or_superseded"
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(resultErr, context.DeadlineExceeded) {
			termination = "timed_out"
		}
		s.report(ctx, "finished", map[string]any{"disposition": termination, "strategy": strategy, "execution": ExecutionOf(provider)})
	}()
	if len(req.Tools) != 1 {
		return result, errors.New("a command submission must disclose exactly one tool")
	}
	parallel := false
	req.RequiredTool, req.ParallelToolCalls, req.ToolConstraintPolicy = req.Tools[0].Name, &parallel, &s.policy
	onStrategy, onRetry, onRequest := req.OnToolStrategy, req.OnRetry, req.OnRequest
	req.OnToolStrategy = func(value ToolStrategy) {
		strategy = value
		if onStrategy != nil {
			onStrategy(value)
		}
	}
	req.OnRetry = func(attempt int) {
		s.networkRetries++
		s.report(ctx, "network_retry", map[string]any{"retry_attempt": attempt, "strategy": strategy})
		if onRetry != nil {
			onRetry(attempt)
		}
	}
	req.OnRequest = func(d RequestDiagnostic) {
		if d.Phase == "started" {
			s.httpRequests++
			s.modelRequests++
		}
		s.report(ctx, "http_request", map[string]any{"request": d, "strategy": strategy})
		if onRequest != nil {
			onRequest(d)
		}
	}
	onReset := req.OnContinuationReset
	req.OnContinuationReset = func(reason string) {
		s.report(ctx, "continuation_reset", map[string]any{"reason": reason, "strategy": strategy})
		if onReset != nil {
			onReset(reason)
		}
	}
	for {
		if err := s.check(ctx); err != nil {
			termination = "invalidated"
			return result, err
		}
		remaining := s.outputBudget - s.outputUsed
		if remaining <= 0 {
			termination = "output_budget_exhausted"
			return result, errors.New("command submission output budget exhausted")
		}
		req.MaxOutputTokens = min(req.MaxOutputTokens, remaining)
		if req.MaxOutputTokens <= 0 {
			req.MaxOutputTokens = remaining
		}
		if maximum := provider.Capabilities().ContextWindowTokens; maximum > 0 && EstimateRequestTokens(req)+req.MaxOutputTokens >= maximum {
			termination = "context_limit"
			return result, errors.New("command evidence and correction context exceed model context window; no required input was omitted")
		}
		s.requests++
		started := time.Now()
		s.report(ctx, "request_started", map[string]any{"execution": ExecutionOf(provider)})
		response, err := provider.Generate(ctx, req)
		s.report(ctx, "request_completed", map[string]any{"request_elapsed_ms": time.Since(started).Milliseconds(), "execution": ExecutionOf(provider), "strategy": strategy, "provider_error": ProviderFailureDiagnostic(err)})
		if checkErr := s.check(ctx); checkErr != nil {
			termination = "invalidated"
			return result, checkErr
		}
		if err != nil {
			canRetry := s.additional < MaxSubmissionAdditionalRequests && s.policy.Learn(err)
			s.report(ctx, "provider_failure", map[string]any{"will_retry": canRetry, "strategy": strategy, "provider_error": ProviderFailureDiagnostic(err)})
			if !canRetry {
				termination = "provider_failure"
				return result, err
			}
			s.additional++
			s.unsupported++
			continue
		}
		used := response.Usage.OutputTokens
		if used <= 0 {
			used = EstimateTextTokens(response.Text()) + EstimateValueTokens(response.ToolCalls)
		}
		s.outputUsed += used
		if s.outputUsed > s.outputBudget {
			termination = "output_budget_exhausted"
			return result, errors.New("provider exceeded command submission output budget")
		}
		err = validate(response)
		if err == nil {
			if checkErr := s.check(ctx); checkErr != nil {
				termination = "invalidated"
				return result, checkErr
			}
			return response, nil
		}
		var invalid *SubmissionError
		repairable := errors.As(err, &invalid) && canReplaySubmissionAgainst(req.Messages, response)
		canCorrect := repairable && s.additional < MaxSubmissionAdditionalRequests && s.outputUsed < s.outputBudget
		d := SubmissionDiagnostic(response, err)
		d["will_correct"], d["strategy"] = canCorrect, strategy
		s.report(ctx, "protocol_violation", d)
		if !canCorrect {
			termination = "correction_budget_exhausted"
			if !repairable {
				termination = "unsafe_or_unrepairable_response"
			} else if s.outputUsed >= s.outputBudget {
				termination = "output_budget_exhausted"
			}
			return result, err
		}
		req.Messages = AppendSubmissionCorrection(req.Messages, response, err, guidance)
		req.Continuation = response.Continuation
		s.additional++
		s.corrections++
	}
}

func canReplaySubmissionAgainst(messages []Message, response GenerateResponse) bool {
	if !CanReplaySubmission(response) {
		return false
	}
	previous := map[string]bool{}
	for _, message := range messages {
		for _, call := range message.ToolCalls {
			previous[call.ID] = true
		}
	}
	for _, call := range response.ToolCalls {
		if previous[call.ID] {
			return false
		}
	}
	return true
}

func AppendSubmissionCorrection(messages []Message, response GenerateResponse, err error, guidance string) []Message {
	// Allocate anew: provider request snapshots and continuation fingerprints must
	// keep the original input, assistant parts and call IDs without rewriting.
	out := append([]Message{}, messages...)
	out = append(out, Message{Role: RoleAssistant, Content: response.Content, ToolCalls: response.ToolCalls})
	for _, call := range response.ToolCalls {
		out = append(out, Message{Role: RoleTool, ToolCallID: call.ID, Content: TextContent(RejectedSubmissionOutput(err, guidance)), Metadata: &MessageMetadata{Origin: "runtime", Kind: "submission_error"}})
	}
	return append(out, Message{Role: RoleUser, Content: TextContent(err.Error() + "\n" + guidance), Metadata: &MessageMetadata{Origin: "runtime", Kind: "guidance"}})
}

func ValidateSubmissionEnvelope(response GenerateResponse, tool string, noText bool) error {
	if len(response.ToolCalls) != 1 {
		return SubmissionFailure("TOOL_COUNT", "/tool_calls", "Call "+tool+" exactly once in this response.")
	}
	if response.ToolCalls[0].Name != tool {
		return SubmissionFailure("TOOL_NAME", "/tool_calls/name", "The only allowed tool is "+tool+".")
	}
	if noText && strings.TrimSpace(response.Text()) != "" {
		return SubmissionFailure("HAS_TEXT", "/content", "Return the result only through "+tool+", without ordinary text.")
	}
	return nil
}

// Allowlisted metadata only: provider messages can echo private input.
func ProviderFailureDiagnostic(err error) map[string]any {
	if err == nil {
		return nil
	}
	d := map[string]any{"class": "provider_error"}
	var invalid *SubmissionError
	if errors.As(err, &invalid) {
		return map[string]any{"class": "submission_invalid", "code": invalid.Code, "field": invalid.Field}
	}
	for _, item := range []struct {
		err  error
		name string
	}{{context.Canceled, "canceled"}, {context.DeadlineExceeded, "timed_out"}, {ErrUnavailable, "unavailable"}, {ErrBadRequest, "bad_request"}, {ErrBadToolCall, "unsafe_tool_response"}, {ErrImageReference, "image_reference"}} {
		if errors.Is(err, item.err) {
			d["class"] = item.name
			break
		}
	}
	var upstream *ProviderError
	if errors.As(err, &upstream) {
		d["status"], d["code"], d["type"], d["param"], d["request_id"] = upstream.StatusCode, upstream.Code, upstream.Type, upstream.Param, upstream.RequestID
	}
	var unsupported *UnsupportedToolConstraintError
	if errors.As(err, &unsupported) {
		d["class"], d["unsupported_field"] = "unsupported_tool_constraint", unsupported.Field
	}
	return d
}
