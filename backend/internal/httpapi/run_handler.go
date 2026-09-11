package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
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
	ClientRequestID   string                     `json:"client_request_id"`
	Model             string                     `json:"model"`
	Instruction       string                     `json:"instruction"`
	Scope             model.CreateRunScopeInput  `json:"scope"`
	Mode              model.RunMode              `json:"mode"`
	Options           model.RunOptions           `json:"options"`
	SkillIDs          []string                   `json:"skill_ids"`
	ComponentNames    []string                   `json:"component_names"`
	MentionedSlideIDs []string                   `json:"mentioned_slide_ids"`
	AttachmentIDs     []string                   `json:"attachment_ids"`
	DOMSelections     []model.DOMSelection       `json:"dom_selections"`
	ReferenceOrder    []model.ReferenceOrderItem `json:"reference_order"`
}

type runResponse struct {
	ID                       string                       `json:"id"`
	ThreadID                 string                       `json:"thread_id"`
	ProjectID                string                       `json:"project_id"`
	Status                   string                       `json:"status"`
	EventsURL                string                       `json:"events_url"`
	Scope                    model.RunScope               `json:"scope"`
	Mode                     model.RunMode                `json:"mode"`
	Model                    *string                      `json:"model"`
	Skills                   []model.PublicSkill          `json:"skills"`
	Components               []model.PublicLoadedResource `json:"components,omitempty"`
	DroppedMentionedSlideIDs []string                     `json:"dropped_mentioned_slide_ids,omitempty"`
	PauseReason              string                       `json:"pause_reason,omitempty"`
	PausedAt                 int64                        `json:"paused_at,omitempty"`
}

func toRunResponse(r model.Run) runResponse {
	var profileName *string
	if r.Model.ProfileName != "" {
		value := r.Model.ProfileName
		profileName = &value
	}
	return runResponse{
		ID: r.ID, ThreadID: r.ThreadID, ProjectID: r.ProjectID,
		Status: string(r.Status), EventsURL: "/api/v1/runs/" + r.ID + "/events",
		Scope: r.Command.Scope, Mode: r.Command.Mode,
		Model: profileName, Skills: r.Command.PublicSkills(), Components: r.Command.PublicComponents(),
		DroppedMentionedSlideIDs: r.Command.DroppedMentionedSlideIDs,
		PauseReason:              r.PauseReason, PausedAt: r.PausedAt,
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
	if body.Scope.Object == "" {
		AbortWithError(c, &APIError{HTTPStatus: http.StatusUnprocessableEntity, Code: "INVALID_SCOPE", Message: "scope is required"})
		return
	}
	if strings.TrimSpace(body.ClientRequestID) == "" {
		AbortWithError(c, ProjectAgentError(model.NewAgentError("BAD_REQUEST", "create_run", nil), "BAD_REQUEST", "create_run"))
		return
	}
	params := model.CreateRunParams{
		ClientRequestID:   body.ClientRequestID,
		Model:             body.Model,
		SkillIDs:          body.SkillIDs,
		ComponentNames:    body.ComponentNames,
		MentionedSlideIDs: body.MentionedSlideIDs,
		AttachmentIDs:     body.AttachmentIDs,
		DOMSelections:     body.DOMSelections,
		ReferenceOrder:    body.ReferenceOrder,
		Instruction:       body.Instruction,
		ScopeInput:        &body.Scope,
		Command: model.RunCommand{
			Mode: body.Mode, Instruction: body.Instruction, Options: body.Options,
		},
	}
	r, err := h.svc.CreateRun(c.Request.Context(), threadID, params)
	if err != nil {
		handleCreateRunError(c, err)
		return
	}

	c.JSON(http.StatusCreated, toRunResponse(r))
}

func (h *RunHandler) ListSkills(c *gin.Context) {
	skills, err := h.svc.ListSkills()
	if err != nil {
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"skills": skills})
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

func (h *RunHandler) GetActiveRunForThread(c *gin.Context) {
	runModel, err := h.svc.GetActiveRunForThread(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, run.ErrRunNotFound) {
			AbortWithError(c, ErrNotFound("active run not found"))
			return
		}
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	c.JSON(http.StatusOK, toRunResponse(runModel))
}

func (h *RunHandler) Screenshot(c *gin.Context) {
	raw, err := h.svc.GetRenderScreenshot(c.Request.Context(), c.Param("id"), c.Param("screenshot_id"))
	if err != nil {
		if errors.Is(err, service.ErrScreenshotNotFound) {
			AbortWithError(c, ErrNotFound("render screenshot not found"))
			return
		}
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	c.Header("Cache-Control", "private, max-age=31536000, immutable")
	c.Data(http.StatusOK, "image/png", raw)
}

func handleCreateRunError(c *gin.Context, err error) {
	var agentErr *model.AgentError
	if errors.As(err, &agentErr) {
		AbortWithError(c, ProjectAgentError(agentErr, "INTERNAL", "create_run"))
		return
	}
	switch {
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("thread not found"))
	case errors.Is(err, service.ErrRunActive):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RUN_ACTIVE", Message: "project has an active run"})
	case errors.Is(err, service.ErrGitCommitActive):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "GIT_COMMIT_ACTIVE", Message: "project has an active Git commit"})
	case errors.Is(err, run.ErrEngineStopping):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusServiceUnavailable, Code: "SERVER_STOPPING", Message: "server is stopping"})
	case errors.Is(err, service.ErrSlideTargetNotFound):
		AbortWithError(c, ProjectAgentError(model.NewAgentError("SLIDE_NOT_FOUND", "create_run", err), "SLIDE_NOT_FOUND", "create_run"))
	case errors.Is(err, model.ErrInvalidRunCommand):
		code := "INVALID_SCOPE"
		for _, item := range []struct {
			target error
			code   string
		}{{model.ErrDOMSelectionInvalid, "DOM_SELECTION_INVALID"}, {model.ErrDOMSelectionLimit, "DOM_SELECTION_LIMIT"}, {model.ErrDOMSelectionTooLarge, "DOM_SELECTION_TOO_LARGE"}, {model.ErrDOMSelectionCommentTooLong, "DOM_SELECTION_COMMENT_TOO_LONG"}, {model.ErrReferenceOrderInvalid, "REFERENCE_ORDER_INVALID"}} {
			if errors.Is(err, item.target) {
				code = item.code
				break
			}
		}
		AbortWithError(c, ProjectAgentError(model.NewAgentError(code, "create_run", err), code, "create_run"))
	case errors.Is(err, service.ErrRunScopeUnsupported):
		AbortWithError(c, ProjectAgentError(model.NewAgentError("RUN_SCOPE_UNSUPPORTED", "create_run", err), "RUN_SCOPE_UNSUPPORTED", "create_run"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

// Resume POST /runs/{id}/resume. Only a durable paused Run may be claimed.
func (h *RunHandler) Resume(c *gin.Context) {
	resumed, err := h.svc.ResumeRun(c.Request.Context(), c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, run.ErrRunNotFound):
			AbortWithError(c, ErrNotFound("run not found"))
		case errors.Is(err, run.ErrRunNotRunning):
			AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RUN_NOT_PAUSED", Message: "run is not paused"})
		case errors.Is(err, run.ErrEngineStopping):
			AbortWithError(c, &APIError{HTTPStatus: http.StatusServiceUnavailable, Code: "SERVER_STOPPING", Message: "server is stopping"})
		default:
			AbortWithError(c, ErrInternal(err.Error()))
		}
		return
	}
	c.JSON(http.StatusAccepted, toRunResponse(resumed))
}

func (h *RunHandler) Steer(c *gin.Context) {
	runID := c.Param("id")
	var body struct {
		ExpectedRunID   string                     `json:"expected_run_id"`
		ClientMessageID string                     `json:"client_message_id"`
		Content         string                     `json:"content"`
		AttachmentIDs   []string                   `json:"attachment_ids"`
		DOMSelections   []model.DOMSelection       `json:"dom_selections"`
		ReferenceOrder  []model.ReferenceOrderItem `json:"reference_order"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ProjectAgentError(model.NewAgentError("BAD_REQUEST", "steer_run", err), "BAD_REQUEST", "steer_run"))
		return
	}
	message, err := h.svc.Steer(c.Request.Context(), runID, body.ExpectedRunID, body.ClientMessageID, body.Content, body.AttachmentIDs, body.DOMSelections, body.ReferenceOrder)
	if err != nil {
		AbortWithError(c, ProjectAgentError(err, "INTERNAL", "steer_run"))
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"status": "accepted", "run_id": message.RunID, "client_message_id": message.ClientMessageID,
	})
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
		AbortWithError(c, ErrConflict("reply_to or answer does not match the pending question"))
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}

func (h *RunHandler) PlanApproval(c *gin.Context) {
	var answer model.PlanApprovalAnswer
	if err := c.ShouldBindJSON(&answer); err != nil {
		AbortWithError(c, ErrBadRequest("invalid plan approval"))
		return
	}
	if err := h.svc.SubmitPlanApproval(c.Request.Context(), c.Param("id"), answer); err != nil {
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "PLAN_APPROVAL_REJECTED", Message: "计划审批已过期或不匹配"})
		return
	}
	c.Status(http.StatusAccepted)
}

func (h *RunHandler) CommandPermission(c *gin.Context) {
	var answer model.CommandPermissionAnswer
	if err := c.ShouldBindJSON(&answer); err != nil {
		AbortWithError(c, ErrBadRequest("invalid command permission answer"))
		return
	}
	if err := h.svc.SubmitCommandPermission(c.Request.Context(), c.Param("id"), answer); err != nil {
		AbortWithError(c, &APIError{
			HTTPStatus: http.StatusConflict,
			Code:       "COMMAND_PERMISSION_REJECTED",
			Message:    "命令授权已过期或不匹配",
		})
		return
	}
	c.Status(http.StatusAccepted)
}

func (h *RunHandler) ScopeExpansion(c *gin.Context) {
	var answer model.ScopeExpansionAnswer
	if err := c.ShouldBindJSON(&answer); err != nil {
		AbortWithError(c, ErrBadRequest("invalid scope expansion answer"))
		return
	}
	if err := h.svc.SubmitScopeExpansion(c.Request.Context(), c.Param("id"), answer); err != nil {
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "SCOPE_EXPANSION_REJECTED", Message: "范围扩权审批已过期或不匹配"})
		return
	}
	c.Status(http.StatusAccepted)
}

// Cancel DELETE /runs/{id}
func (h *RunHandler) Cancel(c *gin.Context) {
	runID := c.Param("id")
	reason := model.RunCancelReason(c.DefaultQuery("reason", string(model.RunCancelUserRequested)))
	if reason != model.RunCancelUserRequested && reason != model.RunCancelSuperseded {
		AbortWithError(c, ErrBadRequest("invalid cancellation reason"))
		return
	}
	current, err := h.svc.RequestCancel(c.Request.Context(), runID, reason)
	if err != nil {
		if errors.Is(err, run.ErrRunNotFound) {
			AbortWithError(c, ProjectAgentError(model.NewAgentError("RUN_NOT_FOUND", "cancel_run", err), "RUN_NOT_FOUND", "cancel_run"))
			return
		}
		AbortWithError(c, ProjectAgentError(err, "INTERNAL", "cancel_run"))
		return
	}
	if current.Status.Terminal() {
		c.JSON(http.StatusOK, gin.H{"status": string(current.Status), "run_id": runID})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "cancel_requested", "run_id": runID})
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
