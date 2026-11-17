package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

type ThreadHandler struct {
	svc     *service.ThreadService
	naming  *service.NamingService
	suggest *service.SuggestService
}

type patchThreadRequest struct {
	AutoSuggestEnabled *bool `json:"auto_suggest_enabled"`
	AutoRenameEnabled  *bool `json:"auto_rename_enabled"`
}

func (h *ThreadHandler) Patch(c *gin.Context) {
	var input patchThreadRequest
	if c.ShouldBindJSON(&input) != nil || (input.AutoRenameEnabled == nil) == (input.AutoSuggestEnabled == nil) {
		AbortWithError(c, ErrBadRequest("exactly one automatic preference is required"))
		return
	}
	var thread model.Thread
	var err error
	if input.AutoSuggestEnabled != nil {
		if h.suggest == nil {
			AbortWithError(c, ErrInternal("suggestions unavailable"))
			return
		}
		thread, err = h.suggest.SetAutomatic(c.Request.Context(), c.Param("id"), *input.AutoSuggestEnabled)
	} else {
		thread, err = h.naming.SetAutomatic(c.Request.Context(), c.Param("id"), *input.AutoRenameEnabled)
	}
	if err != nil {
		AbortWithError(c, ProjectAgentError(err, "INTERNAL", "naming"))
		return
	}
	h.setEpochHeader(c, thread.ProjectID)
	c.JSON(http.StatusOK, toThreadResponse(thread))
}

func NewThreadHandler(svc *service.ThreadService, naming ...*service.NamingService) *ThreadHandler {
	handler := &ThreadHandler{svc: svc}
	if len(naming) > 0 {
		handler.naming = naming[0]
	}
	return handler
}

type threadResponse struct {
	ID                 string `json:"id"`
	ProjectID          string `json:"project_id"`
	Title              string `json:"title"`
	CreatedAt          int64  `json:"created_at"`
	UpdatedAt          int64  `json:"updated_at"`
	AutoSuggestEnabled bool   `json:"auto_suggest_enabled"`
	AutoRenameEnabled  bool   `json:"auto_rename_enabled"`
	NamingRevision     int64  `json:"naming_revision,string"`
}

type createThreadRequest struct {
	Title string `json:"title"`
}

func (h *ThreadHandler) List(c *gin.Context) {
	threads, err := h.svc.ListThreads(c.Request.Context(), c.Param("id"))
	switch {
	case err == nil:
		h.setEpochHeader(c, c.Param("id"))
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
	if strings.TrimSpace(req.Title) != "" {
		if _, err := service.ValidateThreadTitle(req.Title); err != nil {
			AbortWithError(c, ErrBadRequest(err.Error()))
			return
		}
	}
	th, err := h.svc.CreateThread(c.Request.Context(), c.Param("id"), service.CreateThreadParams{Title: req.Title})
	switch {
	case err == nil:
		h.setEpochHeader(c, th.ProjectID)
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
		h.setEpochHeader(c, th.ProjectID)
		c.JSON(http.StatusOK, toThreadResponse(th))
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("thread not found"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func (h *ThreadHandler) ClearPlan(c *gin.Context) {
	var input struct {
		SceneRevision int64  `json:"scene_revision,string"`
		PlanRevision  *int64 `json:"plan_revision,string"`
		RequestKey    string `json:"request_key"`
	}
	if c.ShouldBindJSON(&input) != nil || input.SceneRevision < 1 || input.PlanRevision == nil || *input.PlanRevision < 0 || strings.TrimSpace(input.RequestKey) == "" || len(input.RequestKey) > 128 {
		AbortWithError(c, ErrBadRequest("scene_revision, plan_revision and request_key are required"))
		return
	}
	receipt, err := h.svc.ClearPlan(sessionstate.WithRequestScene(c.Request.Context(), input.SceneRevision), c.Param("id"), *input.PlanRevision, input.RequestKey)
	if abortSessionError(c, err) {
		return
	}
	switch {
	case err == nil:
		c.JSON(http.StatusOK, receipt)
	case errors.Is(err, model.ErrThreadPlanBusy):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "PLAN_BUSY", Message: "任务执行中，无法清除计划。"})
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("thread not found"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func (h *ThreadHandler) Delete(c *gin.Context) {
	target, err := h.svc.DeleteThread(c.Request.Context(), c.Param("id"))
	switch {
	case err == nil:
		c.JSON(http.StatusOK, deletionResponse(target))
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("thread not found"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func toThreadResponse(t model.Thread) threadResponse {
	return threadResponse{
		ID: t.ID, ProjectID: t.ProjectID, Title: t.Title, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
		AutoSuggestEnabled: t.AutoSuggestEnabled, AutoRenameEnabled: t.AutoRenameEnabled, NamingRevision: t.NamingRevision,
	}
}

func (h *ThreadHandler) Events(c *gin.Context) {
	if h.naming == nil {
		AbortWithError(c, ErrInternal("naming service is unavailable"))
		return
	}
	if _, err := h.svc.ListThreads(c.Request.Context(), c.Param("id")); err != nil {
		AbortWithError(c, ErrNotFound("project not found"))
		return
	}
	writer, err := newSSEWriter(c)
	if err != nil {
		AbortWithError(c, ErrInternal("streaming unsupported"))
		return
	}
	events, stop, err := h.naming.Events().Subscribe(c.Request.Context(), c.Param("id"))
	if err != nil {
		AbortWithError(c, ErrNotFound("project not found"))
		return
	}
	defer stop()
	ticker := time.NewTicker(sseHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			raw, marshalErr := json.Marshal(event.Payload)
			if marshalErr != nil || writer.event(event.ID, event.Event, string(raw)) != nil {
				return
			}
		case <-ticker.C:
			if writer.heartbeat() != nil {
				return
			}
		case <-c.Request.Context().Done():
			return
		}
	}
}

func (h *ThreadHandler) setEpochHeader(c *gin.Context, projectID string) {
	if h.naming != nil && projectID != "" {
		c.Header("X-Thread-Stream-Epoch", h.naming.Events().Epoch(projectID))
	}
}
