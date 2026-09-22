package httpapi

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

type PolishHandler struct {
	svc *service.PolishService
}

const maxPolishRequestBytes = 64 << 10

func NewPolishHandler(svc *service.PolishService) *PolishHandler {
	return &PolishHandler{svc: svc}
}

type polishRequest struct {
	Instruction string                    `json:"instruction"`
	Feedback    string                    `json:"feedback"`
	ThreadID    string                    `json:"thread_id"`
	Scope       model.CreateRunScopeInput `json:"scope"`
	Mode        model.RunMode             `json:"mode"`
}

func (h *PolishHandler) Polish(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPolishRequestBytes)
	var body polishRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	commandStream(c, "polish", func(ctx context.Context) (any, error) {
		result, err := h.svc.Polish(ctx, c.Param("id"), service.PolishParams{Instruction: body.Instruction, Feedback: body.Feedback, ThreadID: body.ThreadID, ScopeInput: body.Scope, Mode: body.Mode})
		if err != nil {
			return nil, err
		}
		return gin.H{"title": result.Title, "content": result.Content, "changed": result.Changed, "model_execution": result.ModelExecution, "prompt_version": result.PromptVersion}, nil
	})
}
