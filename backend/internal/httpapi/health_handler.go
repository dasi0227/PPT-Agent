package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

// HealthHandler 暴露 liveness/readiness 探针。
type HealthHandler struct {
	health *service.HealthService
}

func NewHealthHandler(h *service.HealthService) *HealthHandler {
	return &HealthHandler{health: h}
}

func (h *HealthHandler) Healthz(c *gin.Context) {
	if err := h.health.Check(c.Request.Context()); err != nil {
		AbortWithError(c, ErrInternal("dependency not ready"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
