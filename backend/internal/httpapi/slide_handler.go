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

// RenderSlide GET /slides/:id/render：返回规范化单页 HTML，供预览 iframe 加载。
// 使用稳定的 slide_id 定位页面，避免暴露任意文件路径。
// 可选 expected_hash 校验源文件；未找到 → 404；内容已变 → 409。
func (h *SlideHandler) RenderSlide(c *gin.Context) {
	raw, err := h.svc.ReadHTML(c.Request.Context(), c.Param("id"), c.Query("expected_hash"))
	switch {
	case err == nil:
		c.Header("Cache-Control", "no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Data(http.StatusOK, "text/html; charset=utf-8", raw)
	case errors.Is(err, gorm.ErrRecordNotFound):
		AbortWithError(c, ErrNotFound("slide not found"))
	case errors.Is(err, service.ErrSlideHTMLMissing):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusNotFound, Code: "HTML_NOT_READY", Message: "slide html not rendered yet"})
	case errors.Is(err, service.ErrSlideHTMLChanged):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "CONTENT_CONFLICT", Message: "页面内容已更新，请重新加载。"})
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}
