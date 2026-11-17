package httpapi

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

type BriefingHandler struct {
	handoff *service.HandoffService
}

func NewBriefingHandler(handoff *service.HandoffService) *BriefingHandler {
	return &BriefingHandler{handoff: handoff}
}
