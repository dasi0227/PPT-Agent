package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

// SlideHandler 暴露当前单页预览端点。
type SlideHandler struct {
	svc *service.SlideService
}

func NewSlideHandler(svc *service.SlideService) *SlideHandler {
	return &SlideHandler{svc: svc}
}

// RenderSlide GET /slides/:id/render：返回单页 index.html 原始字节，供预览 iframe 加载。
// 使用稳定的 slide_id 作为唯一入参，避免暴露任意文件路径。
// 未找到 → 404；产物未生成 → 404 HTML_NOT_READY。
func (h *SlideHandler) RenderSlide(c *gin.Context) {
	raw, err := h.svc.ReadHTML(c.Request.Context(), c.Param("id"))
	switch {
	case err == nil:
		c.Header("Cache-Control", "no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Data(http.StatusOK, "text/html; charset=utf-8", raw)
	case errors.Is(err, gorm.ErrRecordNotFound):
		AbortWithError(c, ErrNotFound("slide not found"))
	case errors.Is(err, service.ErrSlideHTMLMissing):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusNotFound, Code: "HTML_NOT_READY", Message: "slide html not rendered yet"})
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}
