package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
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
	ThreadID    string                    `json:"thread_id"`
	Scope       model.CreateRunScopeInput `json:"scope"`
	Mode        model.RunMode             `json:"mode"`
	Model       string                    `json:"model"`
}

func (h *PolishHandler) Polish(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPolishRequestBytes)
	var body polishRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	result, err := h.svc.Polish(c.Request.Context(), c.Param("id"), service.PolishParams{
		Instruction: body.Instruction, ThreadID: body.ThreadID,
		ScopeInput: body.Scope, Mode: body.Mode, Model: body.Model,
	})
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"polished_instruction": result.Instruction,
			"changed":              result.Changed,
			"prompt_version":       result.PromptVersion,
		})
	case errors.Is(err, context.Canceled):
		return
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("project or thread not found"))
	default:
		AbortWithError(c, ProjectAgentError(err, "INTERNAL", "polish_prompt"))
	}
}
