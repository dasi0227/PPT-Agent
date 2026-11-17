package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/projecthistory"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
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
	SceneRevision      int64                      `json:"scene_revision,string"`
	RestoredCheckpoint bool                       `json:"restored_checkpoint"`
	ClientRequestID    string                     `json:"client_request_id"`
	Model              string                     `json:"model"`
	Instruction        string                     `json:"instruction"`
	Scope              model.CreateRunScopeInput  `json:"scope"`
	Mode               model.RunMode              `json:"mode"`
	SkillIDs           []string                   `json:"skill_ids"`
	ComponentNames     []string                   `json:"component_names"`
	MentionedSlideIDs  []string                   `json:"mentioned_slide_ids"`
	AttachmentIDs      []string                   `json:"attachment_ids"`
	DOMSelections      []model.DOMSelection       `json:"dom_selections"`
	ReferenceOrder     []model.ReferenceOrderItem `json:"reference_order"`
}

type runResponse struct {
	UpdatedAt                int64                        `json:"updated_at"`
	ActiveDurationMS         *int64                       `json:"active_duration_ms,omitempty"`
	CanContinue              bool                         `json:"can_continue"`
	ExecutionModel           *model.ActiveModelSelection  `json:"model_execution,omitempty"`
	ID                       string                       `json:"id"`
	ThreadID                 string                       `json:"thread_id"`
	ProjectID                string                       `json:"project_id"`
	Status                   string                       `json:"status"`
	Scope                    model.RunScope               `json:"scope"`
	Mode                     model.RunMode                `json:"mode"`
	Model                    *string                      `json:"model"`
	Skills                   []model.PublicSkill          `json:"skills"`
	Components               []model.PublicLoadedResource `json:"components,omitempty"`
	DroppedMentionedSlideIDs []string                     `json:"dropped_mentioned_slide_ids,omitempty"`
	PauseReason              string                       `json:"pause_reason,omitempty"`
	PausedAt                 int64                        `json:"paused_at,omitempty"`
	CancelRequestedAt        int64                        `json:"cancel_requested_at,omitempty"`
}

func toRunResponse(r model.Run) runResponse {
	var profileName *string
	if r.Model.ProfileName != "" {
		value := r.Model.ProfileName
		profileName = &value
	}
	return runResponse{
		UpdatedAt:        r.UpdatedAt,
		ActiveDurationMS: r.ActiveDurationMS,
		CanContinue:      r.CanContinue, ExecutionModel: r.ExecutionModel, ID: r.ID, ThreadID: r.ThreadID, ProjectID: r.ProjectID,
		Status: string(r.Status),
		Scope:  r.Command.Scope, Mode: r.Command.Mode,
		Model: profileName, Skills: r.Command.PublicSkills(), Components: r.Command.PublicComponents(),
		DroppedMentionedSlideIDs: r.Command.DroppedMentionedSlideIDs,
		PauseReason:              r.PauseReason,
		PausedAt:                 r.PausedAt,
		CancelRequestedAt:        r.CancelRequestedAt,
	}
}

// CreateRun POST /threads/{id}/runs
func (h *RunHandler) CreateRun(c *gin.Context) {
	threadID := c.Param("id")
	var body createRunBody
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	if body.SceneRevision < 1 {
		AbortWithError(c, ErrBadRequest("scene_revision is required"))
		return
	}
	if body.Scope.Selection.Kind == "" {
		AbortWithError(c, &APIError{HTTPStatus: http.StatusUnprocessableEntity, Code: "INVALID_SCOPE", Message: "scope is required"})
		return
	}
	if strings.TrimSpace(body.ClientRequestID) == "" {
		AbortWithError(c, ProjectAgentError(model.NewAgentError("BAD_REQUEST", "create_run", nil), "BAD_REQUEST", "create_run"))
		return
	}
	params := model.CreateRunParams{
		SceneRevision:      body.SceneRevision,
		RestoredCheckpoint: body.RestoredCheckpoint,
		ClientRequestID:    body.ClientRequestID,
		Model:              body.Model,
		SkillIDs:           body.SkillIDs,
		ComponentNames:     body.ComponentNames,
		MentionedSlideIDs:  body.MentionedSlideIDs,
		AttachmentIDs:      body.AttachmentIDs,
		DOMSelections:      body.DOMSelections,
		ReferenceOrder:     body.ReferenceOrder,
		Instruction:        body.Instruction,
		ScopeInput:         &body.Scope,
		Command: model.RunCommand{
			Mode: body.Mode, Instruction: body.Instruction,
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
	if abortSessionError(c, err) {
		return
	}
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
	case errors.Is(err, projecthistory.ErrBusy):
		historyError(c, err)
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
		AbortWithError(c, ProjectAgentError(err, "INTERNAL", "create_run"))
	}
}

// Resume POST /runs/{id}/resume. Continue the latest eligible failed or user-interrupted run.
func (h *RunHandler) Resume(c *gin.Context) {
	var action sessionstate.RunAction
	if err := c.ShouldBindJSON(&action); err != nil || !action.Valid() {
		AbortWithError(c, ErrBadRequest("scene_revision, entity_revision and request_key are required"))
		return
	}
	resumed, err := h.svc.ResumeRun(sessionstate.WithRequestScene(c.Request.Context(), action.SceneRevision), c.Param("id"), action)
	if err != nil {
		if abortSessionError(c, err) {
			return
		}
		switch {
		case errors.Is(err, run.ErrRunNotFound):
			AbortWithError(c, ErrNotFound("run not found"))
		case errors.Is(err, run.ErrRunNotRunning), errors.Is(err, run.ErrRunNotContinuable):
			AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RUN_NOT_CONTINUABLE", Message: "任务已被替代或没有有效恢复点，无法继续执行。"})
		case errors.Is(err, run.ErrContinuationConflict):
			AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RUN_CONTINUATION_CONFLICT", Message: "项目内容已变化，无法安全衔接原任务，请发送新的要求。"})
		case errors.Is(err, run.ErrEngineStopping):
			AbortWithError(c, &APIError{HTTPStatus: http.StatusServiceUnavailable, Code: "SERVER_STOPPING", Message: "server is stopping"})
		default:
			AbortWithError(c, ErrInternal(err.Error()))
		}
		return
	}
	c.JSON(http.StatusAccepted, resumed)
}

func (h *RunHandler) Steer(c *gin.Context) {
	runID := c.Param("id")
	var body struct {
		SceneRevision   int64                      `json:"scene_revision,string"`
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
	message, err := h.svc.Steer(sessionstate.WithRequestScene(c.Request.Context(), body.SceneRevision), runID, body.ExpectedRunID, body.ClientMessageID, body.Content, body.AttachmentIDs, body.DOMSelections, body.ReferenceOrder)
	if err != nil {
		if abortSessionError(c, err) {
			return
		}
		AbortWithError(c, ProjectAgentError(err, "INTERNAL", "steer_run"))
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"status": "accepted", "run_id": message.RunID, "client_message_id": message.ClientMessageID,
	})
}

// Input POST /runs/{id}/input（HITL）
func (h *RunHandler) Input(c *gin.Context) {
	runID := c.Param("id")
	var body struct {
		interactionWrite
		Content string `json:"content"`
		ReplyTo string `json:"reply_to"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Content == "" {
		AbortWithError(c, ErrBadRequest("content is required"))
		return
	}
	ctx, ok := body.interactionWrite.context(c)
	if !ok {
		return
	}
	receipt, err := h.svc.InjectInput(ctx, runID, body.Content, body.ReplyTo)
	if err != nil && abortSessionError(c, err) {
		return
	}
	switch {
	case err == nil:
		c.JSON(http.StatusAccepted, receipt)
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
	var body struct {
		interactionWrite
		model.PlanApprovalAnswer
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid plan approval"))
		return
	}
	ctx, ok := body.interactionWrite.context(c)
	if !ok {
		return
	}
	receipt, err := h.svc.SubmitPlanApproval(ctx, c.Param("id"), body.PlanApprovalAnswer)
	if err != nil {
		if abortSessionError(c, err) {
			return
		}
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "PLAN_APPROVAL_REJECTED", Message: "计划审批已过期或不匹配"})
		return
	}
	c.JSON(http.StatusAccepted, receipt)
}

func (h *RunHandler) CommandPermission(c *gin.Context) {
	var body struct {
		interactionWrite
		model.CommandPermissionAnswer
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid command permission answer"))
		return
	}
	ctx, ok := body.interactionWrite.context(c)
	if !ok {
		return
	}
	receipt, err := h.svc.SubmitCommandPermission(ctx, c.Param("id"), body.CommandPermissionAnswer)
	if err != nil {
		if abortSessionError(c, err) {
			return
		}
		AbortWithError(c, &APIError{
			HTTPStatus: http.StatusConflict,
			Code:       "COMMAND_PERMISSION_REJECTED",
			Message:    "命令授权已过期或不匹配",
		})
		return
	}
	c.JSON(http.StatusAccepted, receipt)
}

func (h *RunHandler) ScopeExpansion(c *gin.Context) {
	var body struct {
		interactionWrite
		InteractionID string `json:"interaction_id"`
		CallID        string `json:"call_id"`
		BaseRevision  int64  `json:"base_revision,string"`
		Decision      string `json:"decision"`
		Feedback      string `json:"feedback"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid scope expansion answer"))
		return
	}
	ctx, ok := body.interactionWrite.context(c)
	if !ok {
		return
	}
	answer := model.ScopeExpansionAnswer{InteractionID: body.InteractionID, CallID: body.CallID, BaseRevision: body.BaseRevision, Decision: body.Decision, Feedback: body.Feedback}
	receipt, err := h.svc.SubmitScopeExpansion(ctx, c.Param("id"), answer)
	if err != nil {
		if abortSessionError(c, err) {
			return
		}
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "SCOPE_EXPANSION_REJECTED", Message: "范围扩权审批已过期或不匹配"})
		return
	}
	c.JSON(http.StatusAccepted, receipt)
}

func (h *RunHandler) GetResourceEditApproval(c *gin.Context) {
	scene, err := strconv.ParseInt(c.Query("scene_revision"), 10, 64)
	if err != nil || scene < 1 {
		AbortWithError(c, ErrBadRequest("scene_revision is required"))
		return
	}
	record, err := h.svc.GetResourceEditApproval(sessionstate.WithRequestScene(c.Request.Context(), scene), c.Param("id"), c.Param("interaction_id"))
	if err != nil {
		if abortSessionError(c, err) {
			return
		}
		AbortWithError(c, ErrNotFound("resource approval not found"))
		return
	}
	c.JSON(http.StatusOK, record)
}

func (h *RunHandler) UpdateResourceEditApproval(c *gin.Context) {
	var body struct {
		interactionWrite
		Revision int64           `json:"revision,string"`
		Draft    json.RawMessage `json:"draft"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Revision < 1 || len(body.Draft) == 0 {
		AbortWithError(c, ErrBadRequest("invalid resource approval draft"))
		return
	}
	ctx, ok := body.interactionWrite.context(c)
	if !ok {
		return
	}
	if body.Revision != body.RequestRevision {
		AbortWithError(c, ErrBadRequest("resource revision must match request_revision"))
		return
	}
	record, err := h.svc.UpdateResourceEditApproval(ctx, c.Param("id"), c.Param("interaction_id"), body.Revision, body.Draft)
	if err != nil {
		if abortSessionError(c, err) {
			return
		}
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RESOURCE_APPROVAL_CONFLICT", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, record)
}

func (h *RunHandler) ResourceEditApprovalDecision(c *gin.Context) {
	var body struct {
		interactionWrite
		InteractionID string `json:"interaction_id"`
		CallID        string `json:"call_id"`
		Revision      int64  `json:"revision,string"`
		Decision      string `json:"decision"`
		Feedback      string `json:"feedback"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.InteractionID != c.Param("interaction_id") || body.Revision < 1 {
		AbortWithError(c, ErrBadRequest("invalid resource approval decision"))
		return
	}
	ctx, ok := body.interactionWrite.context(c)
	if !ok {
		return
	}
	answer := model.ResourceEditApprovalAnswer{InteractionID: body.InteractionID, CallID: body.CallID, Revision: body.Revision, Decision: body.Decision, Feedback: body.Feedback}
	receipt, err := h.svc.SubmitResourceEditApproval(ctx, c.Param("id"), c.Param("interaction_id"), answer)
	if err != nil {
		if abortSessionError(c, err) {
			return
		}
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RESOURCE_APPROVAL_CONFLICT", Message: err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, receipt)
}

// Cancel DELETE /runs/{id}
func (h *RunHandler) Cancel(c *gin.Context) {
	runID := c.Param("id")
	reason := model.RunCancelReason(c.DefaultQuery("reason", string(model.RunCancelUserRequested)))
	if reason != model.RunCancelUserRequested && reason != model.RunCancelSuperseded {
		AbortWithError(c, ErrBadRequest("invalid cancellation reason"))
		return
	}
	var action sessionstate.RunAction
	if err := c.ShouldBindJSON(&action); err != nil || !action.Valid() {
		AbortWithError(c, ErrBadRequest("scene_revision, entity_revision and request_key are required"))
		return
	}
	mutation, err := h.svc.RequestCancel(sessionstate.WithRequestScene(c.Request.Context(), action.SceneRevision), runID, reason, action)
	if err != nil {
		if abortSessionError(c, err) {
			return
		}
		if errors.Is(err, run.ErrRunNotFound) {
			AbortWithError(c, ProjectAgentError(model.NewAgentError("RUN_NOT_FOUND", "cancel_run", err), "RUN_NOT_FOUND", "cancel_run"))
			return
		}
		AbortWithError(c, ProjectAgentError(err, "INTERNAL", "cancel_run"))
		return
	}
	if mutation.Current.Status.Terminal() {
		c.JSON(http.StatusOK, mutation)
		return
	}
	c.JSON(http.StatusAccepted, mutation)
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
