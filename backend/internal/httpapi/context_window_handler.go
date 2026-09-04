package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

type ContextWindowHandler struct {
	svc *service.ContextWindowService
}

func NewContextWindowHandler(svc *service.ContextWindowService) *ContextWindowHandler {
	return &ContextWindowHandler{svc: svc}
}

type compactContextRequest struct {
	ModelProfileName string `json:"model_profile_name"`
}

func (h *ContextWindowHandler) Get(c *gin.Context) {
	snapshot, err := h.svc.Snapshot(
		c.Request.Context(), c.Param("id"), c.Query("model_profile_name"),
	)
	h.respond(c, snapshot, err)
}

func (h *ContextWindowHandler) Compact(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var body compactContextRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	result, err := h.svc.Compact(c.Request.Context(), c.Param("id"), body.ModelProfileName)
	h.respond(c, result, err)
}

func (h *ContextWindowHandler) respond(c *gin.Context, result any, err error) {
	switch {
	case err == nil:
		c.JSON(http.StatusOK, result)
	case errors.Is(err, context.Canceled):
		return
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("thread not found"))
	default:
		AbortWithError(c, ProjectAgentError(err, "INTERNAL", "compact"))
	}
}
