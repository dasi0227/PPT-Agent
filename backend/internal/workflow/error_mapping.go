package workflow

import (
	"context"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func classifyProviderError(ctx context.Context, err error) *model.AgentError {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(ctx.Err(), context.Canceled):
		cause := err
		if ctx.Err() != nil {
			cause = ctx.Err()
		}
		return model.NewAgentError(CodeCanceled, "provider_request", cause)
	case errors.Is(err, llm.ErrBadRequest):
		return model.NewAgentError("PROVIDER_BAD_REQUEST", "provider_request", err)
	case errors.Is(err, llm.ErrUnavailable), errors.Is(err, context.DeadlineExceeded):
		return model.NewAgentError("PROVIDER_UNAVAILABLE", "provider_request", err)
	case errors.Is(err, llm.ErrImageReference):
		return model.NewAgentError("IMAGE_REFERENCE_UNAVAILABLE", "resolve_image", err)
	case errors.Is(err, llm.ErrBadToolCall):
		return model.NewAgentError(model.CodeToolCallInvalid, "decode_tool_call", err)
	default:
		return model.NewAgentError(model.CodeInternal, "provider_request", err)
	}
}

func classifyPersistenceError(err error, operation string) *model.AgentError {
	if errors.Is(err, context.Canceled) {
		return model.NewAgentError(model.CodeRunCanceled, operation, err)
	}
	return model.AsAgentError(err, model.CodePersistenceFailed, operation)
}

func persistenceToolFailure(err error) ToolResult {
	failure := classifyPersistenceError(err, "tool_result_save")
	return failedToolResult(failure.Code, err.Error())
}

func internalToolFailure(err error) ToolResult {
	if errors.Is(err, context.Canceled) {
		return failedToolResult(model.CodeRunCanceled, err.Error())
	}
	if errors.Is(err, llm.ErrBadToolCall) {
		return failedToolResult(model.CodeToolCallInvalid, err.Error())
	}
	var submission *llm.SubmissionError
	if errors.As(err, &submission) {
		return detailedToolFailure(submission.Code, submission.Message, map[string]any{"field": submission.Field})
	}
	failure := model.AsAgentError(err, model.CodeInternal, "tool_call")
	details := make(map[string]any, len(failure.Details)+1)
	for key, value := range failure.Details {
		details[key] = value
	}
	if failure.Measure != "" {
		details["measure"] = failure.Measure
	}
	reason := err.Error()
	var classified *model.AgentError
	if errors.As(err, &classified) && classified.Reason != "" && classified.Reason != model.ErrorDefinitionFor(classified.Code).SafeMessage {
		reason = classified.Reason
	}
	return detailedToolFailure(failure.Code, reason, details)
}

func classifyRenderError(err error) *model.AgentError {
	if errors.Is(err, context.Canceled) {
		return model.NewAgentError(CodeCanceled, "render_slide", err)
	}
	var classified *model.AgentError
	if errors.As(err, &classified) {
		return classified
	}
	if errors.Is(err, spec.ErrSlideStageMissing) || errors.Is(err, spec.ErrStaticPageNumber) {
		return model.NewAgentError(CodeRenderFailed, "render_slide", err)
	}
	return model.NewAgentError(model.CodeInternal, "render_slide", err)
}
