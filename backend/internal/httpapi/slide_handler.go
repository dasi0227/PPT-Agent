package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

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
	c.JSON(http.StatusOK, toSlideResponse(sl))
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
	}
}
