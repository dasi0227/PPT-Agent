package httpapi

import (
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
