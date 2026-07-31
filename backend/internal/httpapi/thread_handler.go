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

type ThreadHandler struct {
	svc *service.ThreadService
}

type patchThreadRequest struct {
	Title *string `json:"title"`
}

func (h *ThreadHandler) Patch(c *gin.Context) {
	var req patchThreadRequest
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
	t, err := h.svc.RenameThread(c.Request.Context(), c.Param("id"), title)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, toThreadResponse(t))
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("thread not found"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func NewThreadHandler(svc *service.ThreadService) *ThreadHandler {
	return &ThreadHandler{svc: svc}
}

type threadResponse struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Title       string `json:"title"`
	HistoryPath string `json:"history_path"`
	Status      string `json:"status"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type createThreadRequest struct {
	Title string `json:"title"`
}

func (h *ThreadHandler) List(c *gin.Context) {
	threads, err := h.svc.ListThreads(c.Request.Context(), c.Param("id"))
	switch {
	case err == nil:
		out := make([]threadResponse, len(threads))
		for i, th := range threads {
			out[i] = toThreadResponse(th)
		}
		c.JSON(http.StatusOK, out)
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("project not found"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func (h *ThreadHandler) Create(c *gin.Context) {
	var req createThreadRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			AbortWithError(c, ErrBadRequest("invalid request body"))
			return
		}
	}
	th, err := h.svc.CreateThread(c.Request.Context(), c.Param("id"), service.CreateThreadParams{Title: req.Title})
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, toThreadResponse(th))
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("project not found"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func (h *ThreadHandler) Get(c *gin.Context) {
	th, err := h.svc.GetThread(c.Request.Context(), c.Param("id"))
	switch {
	case err == nil:
		c.JSON(http.StatusOK, toThreadResponse(th))
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("thread not found"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func (h *ThreadHandler) History(c *gin.Context) {
	history, err := h.svc.History(c.Request.Context(), c.Param("id"))
	switch {
	case err == nil:
		c.JSON(http.StatusOK, history)
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("thread not found"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func (h *ThreadHandler) Delete(c *gin.Context) {
	err := h.svc.DeleteThread(c.Request.Context(), c.Param("id"))
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("thread not found"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func toThreadResponse(t model.Thread) threadResponse {
	return threadResponse{
		ID: t.ID, ProjectID: t.ProjectID, Title: t.Title, HistoryPath: t.HistoryPath,
		Status: t.Status, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}
