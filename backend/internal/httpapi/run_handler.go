package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/command"
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
	Kind        string `json:"kind"`
	Scope       string `json:"scope"`
	PageIndex   *int   `json:"page_index"`
	Mode        string `json:"mode"`
	Command     string `json:"command"`
	Instruction string `json:"instruction"`
	Brief       string `json:"brief"`
	SlideCount  int    `json:"slide_count"`
	Language    string `json:"language"`
	Theme       string `json:"theme"`
}

type runResponse struct {
	ID        string `json:"id"`
	ThreadID  string `json:"thread_id"`
	ProjectID string `json:"project_id"`
	Kind      string `json:"kind"`
	Scope     string `json:"scope"`
	Mode      string `json:"mode"`
	Status    string `json:"status"`
	EventsURL string `json:"events_url"`
}

// CreateRun POST /threads/{id}/runs
func (h *RunHandler) CreateRun(c *gin.Context) {
	threadID := c.Param("id")
	var body createRunBody
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	if err := applyRawCommand(&body); err != nil {
		AbortWithError(c, ErrBadRequest(err.Error()))
		return
	}
	if body.Kind == "" {
		AbortWithError(c, ErrBadRequest("kind is required"))
		return
	}
	scope := model.Scope(body.Scope)
	if scope == "" {
		scope = model.ScopeCurrent
	}
	mode := model.Mode(body.Mode)
	if mode == "" {
		mode = model.ModeNormal
	}

	r, err := h.svc.CreateRun(c.Request.Context(), threadID, model.CreateRunParams{
		Kind:        model.Kind(body.Kind),
		Scope:       scope,
		PageIndex:   body.PageIndex,
		Mode:        mode,
		Command:     body.Command,
		Instruction: body.Instruction,
		Brief:       body.Brief,
		SlideCount:  body.SlideCount,
		Language:    body.Language,
		Theme:       body.Theme,
	})
	if err != nil {
		if errors.Is(err, run.ErrRunNotFound) {
			AbortWithError(c, ErrNotFound("thread not found"))
			return
		}
		if errors.Is(err, service.ErrInvalidPageIndex) {
			AbortWithError(c, ErrBadRequest("invalid or missing page_index"))
			return
		}
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}

	c.JSON(http.StatusCreated, runResponse{
		ID: r.ID, ThreadID: r.ThreadID, ProjectID: r.ProjectID,
		Kind: string(r.Kind), Scope: string(r.Scope), Mode: string(r.Mode),
		Status: string(r.Status), EventsURL: "/api/v1/runs/" + r.ID + "/events",
	})
}

func applyRawCommand(body *createRunBody) error {
	raw := strings.TrimSpace(body.Instruction)
	if !strings.HasPrefix(raw, "/") {
		return nil
	}
	parsed, err := command.Parse(raw)
	if err != nil {
		return err
	}
	if body.Scope != "" && model.Scope(body.Scope) != parsed.Scope {
		return fmt.Errorf("scope conflicts with raw command: %s vs %s", body.Scope, parsed.Scope)
	}
	if body.Mode != "" && model.Mode(body.Mode) != parsed.Mode {
		return fmt.Errorf("mode conflicts with raw command: %s vs %s", body.Mode, parsed.Mode)
	}
	if body.Command != "" && body.Command != parsed.Command {
		return fmt.Errorf("command conflicts with raw command: %s vs %s", body.Command, parsed.Command)
	}
	if body.PageIndex != nil {
		if parsed.PageIndex != nil && *body.PageIndex != *parsed.PageIndex {
			return fmt.Errorf("page_index conflicts with raw command")
		}
		if parsed.PageIndex == nil && parsed.Scope != model.ScopeCurrent {
			return fmt.Errorf("page_index conflicts with raw command scope %s", parsed.Scope)
		}
	}
	body.Scope = string(parsed.Scope)
	body.Mode = string(parsed.Mode)
	body.Command = parsed.Command
	body.Instruction = parsed.Instruction
	if parsed.PageIndex != nil {
		body.PageIndex = parsed.PageIndex
	} else if parsed.Scope != model.ScopeCurrent {
		body.PageIndex = nil
	}
	return nil
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
