package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func deletionResponse(target store.Deletion) gin.H {
	return gin.H{"deletion": target, "server_now": time.Now().UnixMilli()}
}

func (r *Router) pendingDeletions(c *gin.Context) {
	targets, err := r.deletions.Pending(c.Request.Context())
	if err != nil {
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"deletions": targets, "server_now": time.Now().UnixMilli()})
}

func (r *Router) undoDeletion(c *gin.Context) {
	kind := c.Param("kind")
	var body struct {
		Token string `json:"token"`
	}
	if (kind != "project" && kind != "thread") || c.ShouldBindJSON(&body) != nil || body.Token == "" {
		AbortWithError(c, ErrBadRequest("invalid deletion restore request"))
		return
	}
	target, err := r.deletions.Restore(c.Request.Context(), kind, c.Param("id"), body.Token)
	if err != nil {
		if errors.Is(err, store.ErrDeletionExpired) || errors.Is(err, run.ErrRunNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "DELETION_EXPIRED", Message: "无法撤销，内容已删除或所属项目正在删除"})
		} else {
			AbortWithError(c, ErrInternal(err.Error()))
		}
		return
	}
	if r.thread.naming != nil {
		r.thread.naming.Events().ResetProject(c.Request.Context(), target.ProjectID)
	}
	c.Status(http.StatusNoContent)
}
