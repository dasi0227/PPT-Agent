package httpapi

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/gin-gonic/gin"
)

// APIError 对应 api-overview 的统一错误体与错误码表。
type APIError struct {
	HTTPStatus int
	Code       string
	Message    string
	Details    map[string]any
	Retryable  bool
}

func (e *APIError) Error() string { return e.Message }

// 错误码工厂（对应 api-overview 错误码表）。
func ErrBadRequest(msg string) *APIError {
	return ProjectAgentError(model.NewAgentError("BAD_REQUEST", "http_request", nil), "BAD_REQUEST", "http_request")
}
func ErrNotFound(msg string) *APIError {
	return ProjectAgentError(model.NewAgentError("NOT_FOUND", "http_request", nil), "NOT_FOUND", "http_request")
}
func ErrConflict(msg string) *APIError {
	return ProjectAgentError(model.NewAgentError("CONFLICT", "http_request", nil), "CONFLICT", "http_request")
}
func ErrValidationFailed(msg string) *APIError {
	return ProjectAgentError(model.NewAgentError("VALIDATION_FAILED", "http_request", nil), "VALIDATION_FAILED", "http_request")
}
func ErrInternal(msg string) *APIError {
	return ProjectAgentError(model.NewAgentError("INTERNAL", "http_request", nil), "INTERNAL", "http_request")
}

func ProjectAgentError(err error, fallbackCode, operation string) *APIError {
	agentErr := model.AsAgentError(err, fallbackCode, operation)
	return &APIError{
		HTTPStatus: agentErr.HTTPStatus(), Code: agentErr.Code,
		Message: agentErr.SafeMessage, Details: safeAPIDetails(agentErr.Details),
		Retryable: agentErr.Retryable,
	}
}

func safeAPIDetails(details map[string]any) map[string]any {
	if len(details) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	for key, value := range details {
		switch key {
		case "current_revision", "json_pointer", "rejection_code", "next_action", "required_capability":
			out[key] = value
		}
	}
	return out
}

// AbortWithError 以统一错误体写出并终止请求链。
func AbortWithError(c *gin.Context, err *APIError) {
	body := gin.H{"code": err.Code, "message": err.Message}
	if err.Details != nil {
		body["details"] = err.Details
	}
	body["retryable"] = err.Retryable
	c.AbortWithStatusJSON(err.HTTPStatus, gin.H{"error": body})
}
