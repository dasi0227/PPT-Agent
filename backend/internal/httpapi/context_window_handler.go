package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

type ContextWindowHandler struct {
	svc *service.ContextWindowService
}

func NewContextWindowHandler(svc *service.ContextWindowService) *ContextWindowHandler {
	return &ContextWindowHandler{svc: svc}
}

func (h *ContextWindowHandler) Get(c *gin.Context) {
	scene, err := strconv.ParseInt(c.Query("scene_revision"), 10, 64)
	if err != nil || scene < 1 || strconv.FormatInt(scene, 10) != c.Query("scene_revision") {
		AbortWithError(c, ErrBadRequest("scene_revision is required"))
		return
	}
	snapshot, err := h.svc.Snapshot(
		sessionstate.WithRequestScene(c.Request.Context(), scene), c.Param("id"), c.Query("model_profile_name"),
	)
	h.respond(c, snapshot, err)
}

func (h *ContextWindowHandler) respond(c *gin.Context, result any, err error) {
	if abortSessionError(c, err) {
		return
	}
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
