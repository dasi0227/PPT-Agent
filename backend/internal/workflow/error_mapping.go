package workflow

import (
	"context"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func classifyProviderError(ctx context.Context, err error) *model.AgentError {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(ctx.Err(), context.Canceled):
		cause := err
		if ctx.Err() != nil {
			cause = ctx.Err()
		}
		return model.NewAgentError(CodeCanceled, "provider_request", cause)
	case errors.Is(err, llm.ErrUnavailable):
		return model.NewAgentError("PROVIDER_UNAVAILABLE", "provider_request", err)
	case errors.Is(err, llm.ErrBadRequest):
		return model.NewAgentError("PROVIDER_BAD_REQUEST", "provider_request", err)
	default:
		return model.NewAgentError(CodeAgentFailed, "provider_request", err)
	}
}

func classifyRenderError(err error) *model.AgentError {
	switch {
	case errors.Is(err, context.Canceled):
		return model.NewAgentError(CodeCanceled, "render_slide", err)
	case errors.Is(err, ErrRenderWorkerUnavailable):
		return model.NewAgentError(CodeRenderWorkerUnavailable, "render_slide", err)
	default:
		return model.NewAgentError(CodeRenderFailed, "render_slide", err)
	}
}
