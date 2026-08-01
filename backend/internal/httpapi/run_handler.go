package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
)

const sseHeartbeatInterval = time.Second

// RunHandler 暴露 Run 的创建、SSE 订阅、HITL 输入、取消端点。
type RunHandler struct {
	svc *service.RunService
}

func NewRunHandler(svc *service.RunService) *RunHandler {
	return &RunHandler{svc: svc}
}

type createRunBody struct {
	Instruction string               `json:"instruction"`
	Target      model.RunTarget      `json:"target"`
	Interaction model.RunInteraction `json:"interaction"`
	Options     model.RunOptions     `json:"options"`
}

type runResponse struct {
	ID          string               `json:"id"`
	ThreadID    string               `json:"thread_id"`
	ProjectID   string               `json:"project_id"`
	Status      string               `json:"status"`
	EventsURL   string               `json:"events_url"`
	Target      model.RunTarget      `json:"target"`
	Interaction model.RunInteraction `json:"interaction"`
}

func toRunResponse(r model.Run) runResponse {
	return runResponse{
		ID: r.ID, ThreadID: r.ThreadID, ProjectID: r.ProjectID,
		Status: string(r.Status), EventsURL: "/api/v1/runs/" + r.ID + "/events",
		Target: r.WorkSpec.Target, Interaction: r.WorkSpec.Interaction,
	}
}

// CreateRun POST /threads/{id}/runs
func (h *RunHandler) CreateRun(c *gin.Context) {
	threadID := c.Param("id")
	var body createRunBody
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	if body.Target.Artifact == "" {
		AbortWithError(c, &APIError{HTTPStatus: http.StatusUnprocessableEntity, Code: "INVALID_TARGET", Message: "target is required"})
		return
	}
	params := model.CreateRunParams{
		Instruction: body.Instruction,
		WorkSpec: model.WorkSpec{
			Target: body.Target, Interaction: body.Interaction,
			Instruction: body.Instruction, Options: body.Options,
		},
	}
	r, err := h.svc.CreateRun(c.Request.Context(), threadID, params)
	if err != nil {
		handleCreateRunError(c, err)
		return
	}

	c.JSON(http.StatusCreated, toRunResponse(r))
}

// GetRun GET /runs/{id}
func (h *RunHandler) GetRun(c *gin.Context) {
	r, err := h.svc.GetRun(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, run.ErrRunNotFound) {
			AbortWithError(c, ErrNotFound("run not found"))
			return
		}
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	c.JSON(http.StatusOK, toRunResponse(r))
}

func handleCreateRunError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("thread not found"))
	case errors.Is(err, service.ErrSlideTargetNotFound):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusBadRequest, Code: "SLIDE_NOT_FOUND", Message: "slide_id is invalid or does not belong to the project"})
	case errors.Is(err, model.ErrInvalidWorkSpec):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusUnprocessableEntity, Code: "INVALID_TARGET", Message: err.Error()})
	case errors.Is(err, service.ErrRunTargetUnsupported):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusUnprocessableEntity, Code: "RUN_TARGET_UNSUPPORTED", Message: err.Error()})
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

// Events GET /runs/{id}/events (SSE，支持 Last-Event-ID 续传)
func (h *RunHandler) Events(c *gin.Context) {
	runID := c.Param("id")
	afterSeq := parseLastEventID(c)

	sw, err := newSSEWriter(c)
	if err != nil {
		AbortWithError(c, ErrInternal("streaming unsupported"))
		return
	}

	ctx := c.Request.Context()
	events, stop, err := h.svc.Subscribe(ctx, runID, afterSeq)
	if err != nil {
		AbortWithError(c, ErrNotFound("run not found"))
		return
	}
	defer stop()
	ticker := time.NewTicker(sseHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return
			}
			if werr := sw.event(strconv.FormatInt(ev.Seq, 10), string(ev.Type), ev.Payload); werr != nil {
				return
			}
			if ev.Type.Terminal() {
				return
			}
		case <-ticker.C:
			if werr := sw.heartbeat(); werr != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// Input POST /runs/{id}/input（HITL）
func (h *RunHandler) Input(c *gin.Context) {
	runID := c.Param("id")
	var body struct {
		Content string `json:"content"`
		ReplyTo string `json:"reply_to"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Content == "" {
		AbortWithError(c, ErrBadRequest("content is required"))
		return
	}
	err := h.svc.InjectInput(c.Request.Context(), runID, body.Content, body.ReplyTo)
	switch {
	case err == nil:
		c.JSON(http.StatusAccepted, gin.H{"status": "accepted"})
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("run not found"))
	case errors.Is(err, run.ErrRunNotRunning):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RUN_NOT_RUNNING", Message: "run is not running"})
	case errors.Is(err, run.ErrReplyMismatch):
		AbortWithError(c, ErrConflict("reply_to does not match a pending needs_input"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

// Cancel DELETE /runs/{id}
func (h *RunHandler) Cancel(c *gin.Context) {
	runID := c.Param("id")
	if err := h.svc.Cancel(c.Request.Context(), runID); err != nil {
		if errors.Is(err, run.ErrRunNotFound) {
			AbortWithError(c, ErrNotFound("run not found"))
			return
		}
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	c.Status(http.StatusNoContent)
}

func parseLastEventID(c *gin.Context) int64 {
	raw := c.GetHeader("Last-Event-ID")
	if raw == "" {
		raw = c.Query("last_event_id")
	}
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
