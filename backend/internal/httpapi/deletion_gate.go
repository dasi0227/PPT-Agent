package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Reject new work against removed targets even while their background cleanup
// is waiting for an old request or worker to leave its critical section.
func (r *Router) deletedTargetGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.FullPath() == "" {
			c.Next()
			return
		}
		parts := strings.Split(strings.TrimPrefix(c.Request.URL.Path, "/api/v1/"), "/")
		if len(parts) < 2 || (c.Request.Method == http.MethodDelete && len(parts) == 2 && (parts[0] == "projects" || parts[0] == "threads")) {
			c.Next()
			return
		}
		var err error
		switch parts[0] {
		case "projects":
			_, err = r.project.svc.GetProject(c.Request.Context(), parts[1])
		case "threads":
			_, err = r.thread.svc.GetThread(c.Request.Context(), parts[1])
		case "exports":
			if r.export != nil {
				if operation, getErr := r.export.svc.Get(parts[1]); getErr == nil {
					_, err = r.project.svc.GetProject(c.Request.Context(), operation.ProjectID)
				}
			}
		}
		if err != nil {
			if errors.Is(err, run.ErrRunNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
				AbortWithError(c, ErrNotFound("project or thread not found"))
			} else {
				AbortWithError(c, ErrInternal(err.Error()))
			}
			return
		}
		c.Next()
	}
}
