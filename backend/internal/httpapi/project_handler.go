package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

type ProjectHandler struct {
	svc      *service.ProjectService
	slideSvc *service.SlideService
}

type patchProjectRequest struct {
	Title *string `json:"title"`
}

func (h *ProjectHandler) Patch(c *gin.Context) {
	var req patchProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	if req.Title == nil {
		AbortWithError(c, ErrBadRequest("no fields to update"))
		return
	}
	title := strings.TrimSpace(*req.Title)
	if title == "" || utf8.RuneCountInString(title) > 60 {
		AbortWithError(c, ErrBadRequest("title length must be 1..60"))
		return
	}
	p, err := h.svc.RenameProject(c.Request.Context(), c.Param("id"), title)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, toProjectResponse(p))
	// using run.ErrRunNotFound might be wrong for project not found, let's use strings.Contains or just default err handling
	// Wait, the spec says "case errors.Is(err, run.ErrRunNotFound):" but it's for project? Actually, run_store might return run.ErrRunNotFound.
	// Let's just return what the spec says or use ErrNotFound directly if err != nil and is not found.
	// Let's assume the spec code is literal.
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("project not found"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func NewProjectHandler(svc *service.ProjectService, slideSvc *service.SlideService) *ProjectHandler {
	return &ProjectHandler{svc: svc, slideSvc: slideSvc}
}

type projectResponse struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	WorkDir        string `json:"work_dir"`
	Theme          string `json:"theme"`
	Status         string `json:"status"`
	DesignPath     string `json:"design_path"`
	DeckPath       string `json:"deck_path"`
	DeckRevision   int    `json:"deck_revision"`
	DesignRevision int    `json:"design_revision"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

type createProjectRequest struct {
	Topic      string `json:"topic"`
	Brief      string `json:"brief"`
	SlideCount int    `json:"slide_count"`
	Language   string `json:"language"`
}

func (h *ProjectHandler) List(c *gin.Context) {
	projects, err := h.svc.ListProjects(c.Request.Context())
	if err != nil {
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	out := make([]projectResponse, len(projects))
	for i, p := range projects {
		out[i] = toProjectResponse(p)
	}
	c.JSON(http.StatusOK, out)
}

func (h *ProjectHandler) Create(c *gin.Context) {
	var req createProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	p, err := h.svc.CreateProject(c.Request.Context(), service.CreateProjectParams{
		Topic: req.Topic, Brief: req.Brief, SlideCount: req.SlideCount, Language: req.Language,
	})
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, toProjectResponse(p))
	case errors.Is(err, service.ErrInvalidProject):
		AbortWithError(c, ErrBadRequest("topic is required"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func (h *ProjectHandler) Get(c *gin.Context) {
	p, err := h.svc.GetProject(c.Request.Context(), c.Param("id"))
	switch {
	case err == nil:
		c.JSON(http.StatusOK, toProjectResponse(p))
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("project not found"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func (h *ProjectHandler) Delete(c *gin.Context) {
	err := h.svc.DeleteProject(c.Request.Context(), c.Param("id"))
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("project not found"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func (h *ProjectHandler) ListSlides(c *gin.Context) {
	slides, err := h.svc.ListSlides(c.Request.Context(), c.Param("id"))
	switch {
	case err == nil:
		out := make([]slideResponse, len(slides))
		for i, sl := range slides {
			resp := toSlideResponse(sl)
			if content, materialization, readErr := h.slideSvc.ReadBlueprint(c.Request.Context(), sl.ID); readErr == nil {
				resp.Blueprint, resp.Materialization = &content, &materialization
			}
			out[i] = resp
		}
		c.JSON(http.StatusOK, out)
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("project not found"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func toProjectResponse(p model.Project) projectResponse {
	return projectResponse{
		ID: p.ID, Title: p.Title, WorkDir: p.WorkDir, Theme: p.Theme, Status: p.Status,
		DesignPath: p.DesignPath, DeckPath: p.DeckPath, DeckRevision: p.DeckRevision,
		DesignRevision: p.DesignRevision, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// CreateSlide POST /projects/:id/slides：在锚点后插入空白页（活跃 run → 409 RUN_ACTIVE）。
func (h *ProjectHandler) CreateSlide(c *gin.Context) {
	var body struct {
		AfterSlideID string `json:"after_slide_id"`
		Layout       string `json:"layout"`
	}
	_ = c.ShouldBindJSON(&body)
	sl, err := h.slideSvc.AddSlide(c.Request.Context(), c.Param("id"), body.AfterSlideID, body.Layout)
	switch {
	case err == nil:
		resp := toSlideResponse(sl)
		if content, materialization, readErr := h.slideSvc.ReadBlueprint(c.Request.Context(), sl.ID); readErr == nil {
			resp.Blueprint, resp.Materialization = &content, &materialization
		}
		c.JSON(http.StatusCreated, resp)
	case errors.Is(err, service.ErrRunActive):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RUN_ACTIVE", Message: "project has an active run"})
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

// ReorderSlides POST /projects/:id/slides/reorder：按 ordered_ids 重排（活跃 run → 409 RUN_ACTIVE）。
func (h *ProjectHandler) ReorderSlides(c *gin.Context) {
	var body struct {
		OrderedIDs []string `json:"ordered_ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.OrderedIDs) == 0 {
		AbortWithError(c, ErrBadRequest("ordered_ids is required"))
		return
	}
	err := h.slideSvc.ReorderSlides(c.Request.Context(), c.Param("id"), body.OrderedIDs)
	switch {
	case err == nil:
		c.Status(http.StatusOK)
	case errors.Is(err, service.ErrRunActive):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RUN_ACTIVE", Message: "project has an active run"})
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}
