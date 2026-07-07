package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

type ProjectHandler struct {
	svc      *service.ProjectService
	slideSvc *service.SlideService
}

func NewProjectHandler(svc *service.ProjectService, slideSvc *service.SlideService) *ProjectHandler {
	return &ProjectHandler{svc: svc, slideSvc: slideSvc}
}

type projectResponse struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	WorkDir    string `json:"work_dir"`
	Theme      string `json:"theme"`
	Status     string `json:"status"`
	DesignPath string `json:"design_path"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
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
			if content, cerr := h.slideSvc.ReadContent(c.Request.Context(), sl.ID); cerr == nil {
				resp.Content = content
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
		DesignPath: p.DesignPath, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}
