package httpapi

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

const maxBriefingRequestBytes = 64 << 10

type BriefingHandler struct {
	handoff *service.HandoffService
}

func NewBriefingHandler(handoff *service.HandoffService) *BriefingHandler {
	return &BriefingHandler{handoff: handoff}
}

type briefingRequest struct {
	ThreadID   string `json:"thread_id"`
	BriefingID string `json:"briefing_id"`
	Feedback   string `json:"feedback"`
}
