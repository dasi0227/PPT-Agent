package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/gin-gonic/gin"
)

type interactionWrite struct {
	SceneRevision   int64 `json:"scene_revision,string"`
	RequestRevision int64 `json:"request_revision,string"`
}

func (w interactionWrite) context(c *gin.Context) (context.Context, bool) {
	if w.SceneRevision < 1 || w.RequestRevision < 1 {
		AbortWithError(c, ErrBadRequest("scene_revision and request_revision are required decimal strings"))
		return nil, false
	}
	return sessionstate.WithInteractionRevision(sessionstate.WithRequestScene(c.Request.Context(), w.SceneRevision), w.RequestRevision), true
}

func abortSessionError(c *gin.Context, err error) bool {
	status, code := http.StatusConflict, ""
	switch {
	case errors.Is(err, sessionstate.ErrSceneChanged):
		code = "SCENE_CHANGED"
	case errors.Is(err, sessionstate.ErrVersionConflict):
		code = "VERSION_CONFLICT"
	case errors.Is(err, sessionstate.ErrIdempotencyConflict):
		code = "IDEMPOTENCY_CONFLICT"
	case errors.Is(err, sessionstate.ErrCursorReset):
		code = "CURSOR_RESET_REQUIRED"
	case errors.Is(err, sessionstate.ErrContentExpired):
		status, code = http.StatusGone, "CONTENT_REF_EXPIRED"
	case errors.Is(err, sessionstate.ErrProjectRestoring):
		status, code = http.StatusServiceUnavailable, "PROJECT_RESTORING"
	default:
		return false
	}
	AbortWithError(c, &APIError{HTTPStatus: status, Code: code, Message: code})
	return true
}
