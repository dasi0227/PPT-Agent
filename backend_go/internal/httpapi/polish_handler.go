package httpapi

import "github.com/dasi0227/PPT-Agent/backend/internal/service"

type PolishHandler struct {
	svc *service.PolishService
}

func NewPolishHandler(svc *service.PolishService) *PolishHandler {
	return &PolishHandler{svc: svc}
}

type polishRequest struct {
	Instruction   string `json:"instruction"`
	PolishContent string `json:"polish_content"`
}
