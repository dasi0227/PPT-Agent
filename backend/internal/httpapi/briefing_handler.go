package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

const maxBriefingRequestBytes = 64 << 10

type BriefingHandler struct {
	kickoff *service.KickoffService
	handoff *service.HandoffService
}

func NewBriefingHandler(kickoff *service.KickoffService, handoff *service.HandoffService) *BriefingHandler {
	return &BriefingHandler{kickoff: kickoff, handoff: handoff}
}

type briefingRequest struct {
	ThreadID         string `json:"thread_id"`
	ModelProfileName string `json:"model_profile_name"`
	BriefingID       string `json:"briefing_id"`
	Feedback         string `json:"feedback"`
}

func (h *BriefingHandler) Kickoff(c *gin.Context) {
	h.generate(c, "kickoff", h.kickoff.Generate)
}

func (h *BriefingHandler) Handoff(c *gin.Context) {
	h.generate(c, "handoff", h.handoff.Generate)
}

func (h *BriefingHandler) generate(
	c *gin.Context,
	operation string,
	generate func(context.Context, string, service.BriefingParams) (service.BriefingResult, error),
) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBriefingRequestBytes)
	var body briefingRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	result, err := generate(c.Request.Context(), c.Param("id"), service.BriefingParams{
		ThreadID: body.ThreadID, Model: body.ModelProfileName,
		BriefingID: body.BriefingID, Feedback: body.Feedback,
	})
	switch {
	case err == nil:
		c.JSON(http.StatusOK, result)
	case errors.Is(err, context.Canceled):
		return
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("project or thread not found"))
	default:
		AbortWithError(c, ProjectAgentError(err, "INTERNAL", operation))
	}
}
