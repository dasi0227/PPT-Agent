package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

// SlideHandler 暴露单页读取、版本列表与回滚端点。
type SlideHandler struct {
	svc *service.SlideService
}

func NewSlideHandler(svc *service.SlideService) *SlideHandler {
	return &SlideHandler{svc: svc}
}

type versionResponse struct {
	VersionNo    int    `json:"version_no"`
	TargetType   string `json:"target_type"`
	TargetID     string `json:"target_id"`
	SnapshotPath string `json:"snapshot_path"`
	RunID        string `json:"run_id,omitempty"`
	CreatedAt    int64  `json:"created_at"`
}

// ListVersions GET /slides/{id}/versions
func (h *SlideHandler) ListVersions(c *gin.Context) {
	vs, err := h.svc.ListVersions(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			AbortWithError(c, ErrNotFound("slide not found"))
			return
		}
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	out := make([]versionResponse, len(vs))
	for i, v := range vs {
		out[i] = versionResponse{
			VersionNo: v.VersionNo, TargetType: v.TargetType, TargetID: v.TargetID,
			SnapshotPath: v.SnapshotPath, RunID: v.RunID, CreatedAt: v.CreatedAt,
		}
	}
	c.JSON(http.StatusOK, out)
}

// Rollback POST /slides/{id}/rollback
func (h *SlideHandler) Rollback(c *gin.Context) {
	var body struct {
		VersionNo *int `json:"version_no"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.VersionNo == nil {
		AbortWithError(c, ErrBadRequest("version_no is required"))
		return
	}
	sl, err := h.svc.RollbackSlide(c.Request.Context(), c.Param("id"), *body.VersionNo)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"slide_id": sl.ID, "current_version": sl.CurrentVersion})
	case errors.Is(err, gorm.ErrRecordNotFound):
		AbortWithError(c, ErrNotFound("slide not found"))
	case errors.Is(err, service.ErrVersionNotFound):
		AbortWithError(c, ErrNotFound("target version not found"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
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
