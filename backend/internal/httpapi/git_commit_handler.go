package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type GitCommitHandler struct {
	svc *service.GitCommitService
}

func NewGitCommitHandler(svc *service.GitCommitService) *GitCommitHandler {
	return &GitCommitHandler{svc: svc}
}

type createGitCommitRequest struct {
	ThreadID        string `json:"thread_id"`
	Model           string `json:"model"`
	ClientRequestID string `json:"client_request_id"`
}

type gitCommitOperationResponse struct {
	ID        string                      `json:"id"`
	ProjectID string                      `json:"project_id"`
	ThreadID  string                      `json:"thread_id"`
	Status    model.GitCommitStatus       `json:"status"`
	Phase     model.GitCommitPhase        `json:"phase,omitempty"`
	EventsURL string                      `json:"events_url"`
	Result    *model.GitCommitResult      `json:"result,omitempty"`
	Error     *model.GitCommitPublicError `json:"error,omitempty"`
}

func toGitCommitOperationResponse(operation model.GitCommitOperation) gitCommitOperationResponse {
	response := gitCommitOperationResponse{
		ID: operation.ID, ProjectID: operation.ProjectID, ThreadID: operation.ThreadID,
		Status: operation.Status, Phase: operation.Phase,
		EventsURL: "/api/v1/git-commits/" + operation.ID + "/events",
	}
	if operation.ResultJSON != "" {
		var result model.GitCommitResult
		if json.Unmarshal([]byte(operation.ResultJSON), &result) == nil {
			response.Result = &result
		}
	}
	if operation.ErrorJSON != "" {
		var publicError model.GitCommitPublicError
		if json.Unmarshal([]byte(operation.ErrorJSON), &publicError) == nil {
			response.Error = &publicError
		}
	}
	return response
}

func (h *GitCommitHandler) Create(c *gin.Context) {
	var request createGitCommitRequest
	if c.ShouldBindJSON(&request) != nil {
		AbortWithError(c, ErrBadRequest("invalid request body"))
		return
	}
	operation, err := h.svc.Start(c.Request.Context(), c.Param("id"), service.GitCommitParams{
		ThreadID: strings.TrimSpace(request.ThreadID), Model: strings.TrimSpace(request.Model),
		ClientRequestID: strings.TrimSpace(request.ClientRequestID),
	})
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, toGitCommitOperationResponse(operation))
}

func (h *GitCommitHandler) Get(c *gin.Context) {
	operation, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, toGitCommitOperationResponse(operation))
}

func (h *GitCommitHandler) Events(c *gin.Context) {
	afterSeq := parseLastEventID(c)
	writer, err := newSSEWriter(c)
	if err != nil {
		AbortWithError(c, ErrInternal("streaming unsupported"))
		return
	}
	events, stop, err := h.svc.Subscribe(c.Request.Context(), c.Param("id"), afterSeq)
	if err != nil {
		h.handleError(c, err)
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
			if err := writer.event(strconv.FormatInt(event.Seq, 10), string(event.Type), event.Payload); err != nil {
				return
			}
			if event.Type.Terminal() {
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

func (h *GitCommitHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("Git commit operation, project, or thread not found"))
	case errors.Is(err, service.ErrRunActive):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RUN_ACTIVE", Message: "project has an active run"})
	case errors.Is(err, service.ErrGitCommitActive):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "GIT_COMMIT_ACTIVE", Message: "project has an active Git commit"})
	case errors.Is(err, service.ErrGitCommitToolUnsupported):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusUnprocessableEntity, Code: "MODEL_TOOL_CALL_UNSUPPORTED", Message: "selected model does not support tool calls"})
	case errors.Is(err, service.ErrGitCommitInvalid):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusUnprocessableEntity, Code: "GIT_COMMIT_INVALID", Message: "invalid Git commit request"})
	default:
		var agentError *model.AgentError
		if errors.As(err, &agentError) {
			AbortWithError(c, ProjectAgentError(agentError, "INTERNAL", "git_commit"))
			return
		}
		AbortWithError(c, ErrInternal(err.Error()))
	}
}
