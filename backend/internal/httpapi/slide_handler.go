package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

// SlideHandler 暴露单页读取、版本列表与回滚端点。
type SlideHandler struct {
	svc *service.SlideService
}

func NewSlideHandler(svc *service.SlideService) *SlideHandler {
	return &SlideHandler{svc: svc}
}

type slideResponse struct {
	ID             string `json:"id"`
	ProjectID      string `json:"project_id"`
	Idx            int    `json:"idx"`
	Layout         string `json:"layout"`
	Title          string `json:"title"`
	HTMLPath       string `json:"html_path"`
	JSONPath       string `json:"json_path"`
	CurrentVersion int    `json:"current_version"`
	Order          int    `json:"order"`
	OutlineDirty   bool   `json:"outline_dirty"`
	Content        any    `json:"content,omitempty"`
}

type versionResponse struct {
	VersionNo    int    `json:"version_no"`
	TargetType   string `json:"target_type"`
	TargetID     string `json:"target_id"`
	SnapshotPath string `json:"snapshot_path"`
	RunID        string `json:"run_id,omitempty"`
	CreatedAt    int64  `json:"created_at"`
}

// GetSlide GET /slides/{id}
func (h *SlideHandler) GetSlide(c *gin.Context) {
	sl, err := h.svc.GetSlide(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			AbortWithError(c, ErrNotFound("slide not found"))
			return
		}
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	resp := toSlideResponse(sl)
	if content, cerr := h.svc.ReadContent(c.Request.Context(), sl.ID); cerr == nil {
		resp.Content = content
	}
	c.JSON(http.StatusOK, resp)
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
		c.JSON(http.StatusOK, toSlideResponse(sl))
	case errors.Is(err, gorm.ErrRecordNotFound):
		AbortWithError(c, ErrNotFound("slide not found"))
	case errors.Is(err, service.ErrVersionNotFound):
		AbortWithError(c, ErrNotFound("target version not found"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func toSlideResponse(sl model.Slide) slideResponse {
	return slideResponse{
		ID: sl.ID, ProjectID: sl.ProjectID, Idx: sl.Idx, Layout: sl.Layout, Title: sl.Title,
		HTMLPath: sl.HTMLPath, JSONPath: sl.JSONPath, CurrentVersion: sl.CurrentVersion,
		Order: sl.Order, OutlineDirty: sl.OutlineDirty,
	}
}

// PatchSlide PATCH /slides/{id}：字段级即时保存 slide.json（不产版本）。
// 活跃 run → 409 RUN_ACTIVE；校验失败 → 422；未找到 → 404。
func (h *SlideHandler) PatchSlide(c *gin.Context) {
	var body struct {
		Title         *string                `json:"title"`
		Subtitle      *string                `json:"subtitle"`
		ContentIntent *string                `json:"content_intent"`
		Layout        *string                `json:"layout"`
		Bullets       *[]string              `json:"bullets"`
		ChartIntent   *slidejson.ChartIntent `json:"chart_intent"`
		Steps         *int                   `json:"steps"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid body"))
		return
	}
	id := c.Param("id")
	sl, err := h.svc.PatchContent(c.Request.Context(), id, service.SlidePatch{
		Title: body.Title, Subtitle: body.Subtitle, ContentIntent: body.ContentIntent,
		Layout: body.Layout, Bullets: body.Bullets, ChartIntent: body.ChartIntent, Steps: body.Steps,
	})
	switch {
	case err == nil:
		meta, gerr := h.svc.GetSlide(c.Request.Context(), id)
		if gerr != nil {
			AbortWithError(c, ErrInternal(gerr.Error()))
			return
		}
		resp := toSlideResponse(meta)
		resp.Content = sl
		c.JSON(http.StatusOK, resp)
	case errors.Is(err, gorm.ErrRecordNotFound):
		AbortWithError(c, ErrNotFound("slide not found"))
	case errors.Is(err, service.ErrRunActive):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RUN_ACTIVE", Message: "project has an active run"})
	case service.IsValidationError(err):
		AbortWithError(c, ErrValidationFailed(err.Error()))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

// DeleteSlide DELETE /slides/:id：删除单页（无回收站；活跃 run → 409 RUN_ACTIVE）。
func (h *SlideHandler) DeleteSlide(c *gin.Context) {
	err := h.svc.DeleteSlide(c.Request.Context(), c.Param("id"))
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.Is(err, gorm.ErrRecordNotFound):
		AbortWithError(c, ErrNotFound("slide not found"))
	case errors.Is(err, service.ErrRunActive):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RUN_ACTIVE", Message: "project has an active run"})
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
