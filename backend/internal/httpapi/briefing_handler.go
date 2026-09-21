package httpapi

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

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
	ThreadID   string `json:"thread_id"`
	BriefingID string `json:"briefing_id"`
	Feedback   string `json:"feedback"`
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
	commandStream(c, operation, func(ctx context.Context) (any, error) {
		return generate(ctx, c.Param("id"), service.BriefingParams{ThreadID: body.ThreadID, BriefingID: body.BriefingID, Feedback: body.Feedback})
	})
}
