package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ProjectHandler struct {
	svc       *service.ProjectService
	mutations *service.PPTMutationService
}

func NewProjectHandler(svc *service.ProjectService, mutations *service.PPTMutationService) *ProjectHandler {
	return &ProjectHandler{svc: svc, mutations: mutations}
}

type patchProjectRequest struct {
	Title *string `json:"title"`
}
type createProjectRequest struct {
	Topic      string `json:"topic"`
	Brief      string `json:"brief"`
	SlideCount int    `json:"slide_count"`
	Language   string `json:"language"`
}
type projectResponse struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	WorkDir         string `json:"work_dir"`
	Theme           string `json:"theme"`
	Status          string `json:"status"`
	DesignPath      string `json:"design_path"`
	OutlinePath     string `json:"outline_path"`
	OutlineRevision int    `json:"outline_revision"`
	DesignRevision  int    `json:"design_revision"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
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
	if c.ShouldBindJSON(&req) != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	p, err := h.svc.CreateProject(c.Request.Context(), service.CreateProjectParams{Topic: req.Topic, Brief: req.Brief, SlideCount: req.SlideCount, Language: req.Language})
	if err == nil {
		c.JSON(http.StatusCreated, toProjectResponse(p))
		return
	}
	if errors.Is(err, service.ErrInvalidProject) {
		AbortWithError(c, ErrBadRequest("topic is required"))
		return
	}
	AbortWithError(c, ErrInternal(err.Error()))
}
func (h *ProjectHandler) Get(c *gin.Context) {
	p, err := h.svc.GetProject(c.Request.Context(), c.Param("id"))
	if err == nil {
		c.JSON(http.StatusOK, toProjectResponse(p))
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, run.ErrRunNotFound) {
		AbortWithError(c, ErrNotFound("project not found"))
		return
	}
	AbortWithError(c, ErrInternal(err.Error()))
}
func (h *ProjectHandler) Patch(c *gin.Context) {
	var req patchProjectRequest
	if c.ShouldBindJSON(&req) != nil || req.Title == nil {
		AbortWithError(c, ErrBadRequest("title is required"))
		return
	}
	title := strings.TrimSpace(*req.Title)
	if title == "" || utf8.RuneCountInString(title) > 60 {
		AbortWithError(c, ErrBadRequest("title length must be 1..60"))
		return
	}
	p, err := h.svc.RenameProject(c.Request.Context(), c.Param("id"), title)
	if err != nil {
		if errors.Is(err, service.ErrRunActive) {
			AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RUN_ACTIVE", Message: "project is currently locked"})
			return
		} else if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, run.ErrRunNotFound) {
			AbortWithError(c, ErrNotFound("project not found"))
			return
		}
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	c.JSON(http.StatusOK, toProjectResponse(p))
}
func (h *ProjectHandler) Delete(c *gin.Context) {
	if err := h.svc.DeleteProject(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, service.ErrRunActive) {
			AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RUN_ACTIVE", Message: "project is currently locked"})
			return
		} else if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, run.ErrRunNotFound) {
			AbortWithError(c, ErrNotFound("project not found"))
			return
		}
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *ProjectHandler) Content(c *gin.Context) {
	snapshot, err := h.mutations.Snapshot(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, run.ErrRunNotFound) {
			AbortWithError(c, ErrNotFound("project content not found"))
		} else {
			AbortWithError(c, ErrInternal(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, snapshot)
}
func (h *ProjectHandler) Mutate(c *gin.Context) {
	var request pptmutation.Request
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil {
		AbortWithError(c, ErrBadRequest("invalid mutation request"))
		return
	}
	snapshot, result, err := h.mutations.Apply(c.Request.Context(), c.Param("id"), request)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRunActive):
			AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RUN_ACTIVE", Message: "project has an active run"})
		case errors.Is(err, service.ErrGitCommitActive):
			AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "GIT_COMMIT_ACTIVE", Message: "project has an active Git commit"})
		case errors.Is(err, pptmutation.ErrRevisionConflict):
			AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "REVISION_CONFLICT", Message: err.Error()})
		case errors.Is(err, pptmutation.ErrInvalid):
			AbortWithError(c, &APIError{HTTPStatus: http.StatusUnprocessableEntity, Code: "MUTATION_INVALID", Message: err.Error()})
		default:
			AbortWithError(c, ErrInternal(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"mutation": result, "content": snapshot})
}

func (h *ProjectHandler) SetTheme(c *gin.Context) {
	var request struct {
		Theme string `json:"theme"`
	}
	if c.ShouldBindJSON(&request) != nil || strings.TrimSpace(request.Theme) == "" {
		AbortWithError(c, ErrBadRequest("theme is required"))
		return
	}
	project, err := h.svc.SetTheme(c.Request.Context(), c.Param("id"), request.Theme)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrThemeNotFound):
			AbortWithError(c, ErrNotFound("theme not found"))
		case errors.Is(err, service.ErrRunActive):
			AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RUN_ACTIVE", Message: "project is currently locked"})
		default:
			AbortWithError(c, ErrInternal(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, toProjectResponse(project))
}

func toProjectResponse(p model.Project) projectResponse {
	return projectResponse{ID: p.ID, Title: p.Title, WorkDir: p.WorkDir, Theme: p.Theme, Status: p.Status, DesignPath: "design.json", OutlinePath: "outline.json", OutlineRevision: p.OutlineRevision, DesignRevision: p.DesignRevision, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}
