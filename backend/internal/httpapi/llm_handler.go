package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

// LLMHandler exposes only the safe, user-selectable profile projection.
type LLMHandler struct {
	registry *llm.Registry
}

func NewLLMHandler(registry *llm.Registry) *LLMHandler {
	return &LLMHandler{registry: registry}
}

func (h *LLMHandler) Profiles(c *gin.Context) {
	c.JSON(http.StatusOK, h.registry.Public())
}
