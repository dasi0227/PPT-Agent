package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIError 对应 api-overview 的统一错误体与错误码表。
type APIError struct {
	HTTPStatus int
	Code       string
	Message    string
	Details    map[string]any
}

func (e *APIError) Error() string { return e.Message }

// 错误码工厂（对应 api-overview 错误码表）。
func ErrBadRequest(msg string) *APIError {
	return &APIError{http.StatusBadRequest, "BAD_REQUEST", msg, nil}
}
func ErrNotFound(msg string) *APIError { return &APIError{http.StatusNotFound, "NOT_FOUND", msg, nil} }
func ErrConflict(msg string) *APIError { return &APIError{http.StatusConflict, "CONFLICT", msg, nil} }
func ErrInternal(msg string) *APIError {
	return &APIError{http.StatusInternalServerError, "INTERNAL", msg, nil}
}

// AbortWithError 以统一错误体写出并终止请求链。
func AbortWithError(c *gin.Context, err *APIError) {
	body := gin.H{"code": err.Code, "message": err.Message}
	if err.Details != nil {
		body["details"] = err.Details
	}
	c.AbortWithStatusJSON(err.HTTPStatus, gin.H{"error": body})
}
