package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/projecthistory"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/gin-gonic/gin"
)

func (r *Router) WithProjectHistory() (*Router, error) {
	manager, err := r.run.svc.EnableProjectHistory(r.cfg.WorkRoot)
	if err != nil {
		return nil, err
	}
	r.history = manager
	r.source = service.NewSlideSourceService(manager)
	sourceGroup := r.engine.Group("/api/v1/projects/:id/slides/:slide_id/source")
	sourceGroup.GET("", r.slideSourceGet)
	sourceGroup.PUT("", r.slideSourcePut)
	if r.export != nil {
		manager.ExportActive = r.export.svc.Manager().Active
	}
	group := r.engine.Group("/api/v1/projects/:id/history")
	group.GET("", r.historyState)
	group.GET("/preview", r.historyPreview)
	group.POST("/switch", r.historySwitch)
	return r, nil
}
func historyError(c *gin.Context, err error) {
	code := "HISTORY_UNAVAILABLE"
	status := http.StatusInternalServerError
	message := "项目历史操作失败，请重试"
	switch {
	case errors.Is(err, projecthistory.ErrConflict):
		code = "HISTORY_REVISION_CONFLICT"
		status = 409
		message = err.Error()
	case errors.Is(err, projecthistory.ErrBusy):
		code = "HISTORY_BUSY"
		status = 409
		message = err.Error()
	case errors.Is(err, projecthistory.ErrTarget):
		code = "HISTORY_TARGET_UNAVAILABLE"
		status = 409
		message = err.Error()
	}
	AbortWithError(c, &APIError{HTTPStatus: status, Code: code, Message: message})
}

func historyResponseState(s projecthistory.State) gin.H {
	checkpoints := make([]gin.H, 0, len(s.Checkpoints))
	for _, cp := range s.Checkpoints {
		checkpoints = append(checkpoints, gin.H{"run_id": cp.RunID, "thread_id": cp.ThreadID, "time": cp.Time, "sequence": cp.Sequence})
	}
	return gin.H{"revision": s.Revision, "scene_revision": s.SceneRevision, "checkpoints": checkpoints,
		"latest": s.Latest, "latest_time": s.LatestTime, "scene": s.Scene}
}
func (r *Router) historyState(c *gin.Context) {
	s, err := r.history.State(c.Param("id"))
	if err != nil {
		historyError(c, err)
		return
	}
	visible := []projecthistory.Checkpoint{}
	for _, cp := range s.Checkpoints {
		run, err := r.history.Store.GetRun(c.Request.Context(), cp.RunID)
		if err == nil && run.Status.Terminal() {
			visible = append(visible, cp)
		}
	}
	s.Checkpoints = visible
	c.JSON(200, historyResponseState(s))
}
func (r *Router) historyPreview(c *gin.Context) {
	v, err := r.history.Preview(c.Request.Context(), c.Param("id"), c.Query("run_id"))
	if err != nil {
		historyError(c, err)
		return
	}
	c.JSON(200, v)
}
func (r *Router) historySwitch(c *gin.Context) {
	var body struct {
		RunID     string          `json:"run_id"`
		Revision  int64           `json:"revision"`
		Operation string          `json:"operation_id"`
		Scene     json.RawMessage `json:"scene"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<20)
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid history command"))
		return
	}
	var releaseNaming func()
	if r.thread != nil && r.thread.naming != nil {
		releaseNaming = r.thread.naming.BeginProjectReset(c.Request.Context(), c.Param("id"))
		defer releaseNaming()
	}
	s, err := r.history.Switch(c.Request.Context(), c.Param("id"), body.RunID, body.Revision, body.Operation, body.Scene)
	if err != nil {
		historyError(c, err)
		return
	}
	if r.thread != nil && r.thread.naming != nil {
		r.thread.naming.Events().ResetProject(c.Request.Context(), c.Param("id"))
	}
	c.Header("X-Project-History-Revision", strconv.FormatInt(s.Revision, 10))
	if err := r.history.Collect(c.Param("id")); err != nil {
		r.log.Warn("project checkpoint cleanup deferred")
	}
	c.JSON(200, historyResponseState(s))
}

type historyResponse struct {
	gin.ResponseWriter
	body     bytes.Buffer
	status   int
	streamed bool
	finish   func(bool) error
}

func (w *historyResponse) WriteHeader(status int) { w.status = status }
func (w *historyResponse) WriteHeaderNow()        {}
func (w *historyResponse) Write(raw []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	return w.body.Write(raw)
}
func (w *historyResponse) WriteString(raw string) (int, error) { return w.Write([]byte(raw)) }
func (w *historyResponse) Status() int {
	if w.status == 0 {
		return 200
	}
	return w.status
}
func (w *historyResponse) Size() int     { return w.body.Len() }
func (w *historyResponse) Written() bool { return w.status != 0 }

// Only requests that can mutate project history, plus history previews that
// snapshot project bytes, cross this gate. Ordinary reads observe files and
// state published through durable atomic replacement, so making them exclusive
// would let routine UI hydration reject an unrelated write such as Run creation.
func (r *Router) projectHistoryGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		if r.history == nil || c.FullPath() == "" {
			c.Next()
			return
		}
		path := strings.TrimPrefix(c.Request.URL.Path, "/api/v1/")
		readOnly := c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS"
		// Preview computes a diff from a stable project snapshot, so it remains
		// serialized with writers. All other reads must never take the exclusive
		// gate: active-session hydration runs concurrently with a newly submitted
		// command in normal UI flow.
		if readOnly && !strings.HasSuffix(path, "/history/preview") {
			c.Header("Cache-Control", "no-store")
			c.Next()
			return
		}
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			c.Next()
			return
		}
		ctx := c.Request.Context()
		id := ""
		switch parts[0] {
		case "projects":
			id = parts[1]
		case "threads":
			v, err := r.history.Store.GetThread(ctx, parts[1])
			if err == nil {
				id = v.ProjectID
			}
		case "runs":
			v, err := r.history.Store.GetRun(ctx, parts[1])
			if err == nil {
				id = v.ProjectID
			}
		case "slides":
			v, err := r.history.Store.GetSlide(ctx, parts[1])
			if err == nil {
				id = v.ProjectID
			}
		case "git-commits":
			v, err := r.history.Store.GetGitCommitOperation(ctx, parts[1])
			if err == nil {
				id = v.ProjectID
			}
		}
		if id == "" {
			c.Next()
			return
		}
		var release func()
		if c.Request.Method != "GET" || strings.Contains(path, "/history/preview") {
			var acquired bool
			release, acquired = r.history.TryGate(id)
			if !acquired {
				historyError(c, projecthistory.ErrBusy)
				return
			}
		} else {
			release = r.history.Gate(id)
		}
		defer func() {
			if release != nil {
				release()
			}
		}()
		if err := r.history.Recover(ctx, id); err != nil {
			historyError(c, err)
			return
		}
		s, err := r.history.State(id)
		if err != nil {
			historyError(c, err)
			return
		}
		c.Header("X-Project-ID", id)
		c.Header("X-Project-History-Revision", strconv.FormatInt(s.Revision, 10))
		c.Header("Cache-Control", "no-store")
		isHistory := len(parts) > 2 && parts[0] == "projects" && parts[2] == "history"
		if strings.HasSuffix(path, "/events") {
			release()
			release = nil
			c.Next()
			return
		}
		if readOnly || isHistory {
			c.Next()
			return
		}
		// Source PUT decides whether bytes actually change before creating a
		// history mutation. The project gate remains held through that decision.
		if c.Request.Method == http.MethodPut && len(parts) == 5 && parts[0] == "projects" && parts[2] == "slides" && parts[4] == "source" {
			c.Next()
			return
		}
		// Run controls operate on an existing task; polish and export do not mutate authoring history.
		if parts[0] == "runs" || (parts[0] == "git-commits" && strings.HasSuffix(path, "/cancel")) || strings.HasSuffix(path, "/polish") || strings.HasSuffix(path, "/exports") {
			c.Next()
			return
		}
		revision, _ := strconv.ParseInt(c.GetHeader("X-Discard-Future-Revision"), 10, 64)
		if s.Latest != "" && revision == 0 {
			AbortWithError(c, &APIError{HTTPStatus: 409, Code: "HISTORY_CONFIRM_REQUIRED", Message: "继续创作将丢弃原来的后续历史，无法再恢复到最新现场。", Details: map[string]any{"project_id": id, "revision": s.Revision}})
			return
		}
		finish, err := r.history.BeginMutation(ctx, id, revision)
		if err != nil {
			historyError(c, err)
			return
		}
		isProjectDelete := c.Request.Method == http.MethodDelete && len(parts) == 2 && parts[0] == "projects"
		var releaseNaming func()
		if isProjectDelete && r.thread != nil && r.thread.naming != nil {
			releaseNaming = r.thread.naming.BeginProjectReset(ctx, id)
			defer releaseNaming()
		}
		requestContext := c.Request.Context()
		if isProjectDelete {
			requestContext = service.WithDeferredProjectCleanup(requestContext)
		}
		c.Request = c.Request.WithContext(run.WithStartBarrier(requestContext, func() error { return finish(true) }))
		original := c.Writer
		buffer := &historyResponse{ResponseWriter: original, finish: finish}
		c.Writer = buffer
		defer func() { c.Writer = original }()
		c.Next()
		c.Writer = original
		if buffer.streamed {
			// commandStream settles the history transaction before its terminal frame.
			if cleanupErr := r.history.Collect(id); cleanupErr != nil {
				r.log.Warn("project checkpoint cleanup deferred")
			}
			return
		}
		if err = finish(buffer.Status() < 400); err != nil {
			historyError(c, err)
			return
		}
		if next, err := r.history.State(id); err == nil {
			c.Header("X-Project-History-Revision", strconv.FormatInt(next.Revision, 10))
		}
		if cleanupErr := r.history.Collect(id); cleanupErr != nil {
			r.log.Warn("project checkpoint cleanup deferred")
		}
		if isProjectDelete && buffer.Status() < 400 {
			if r.thread != nil && r.thread.naming != nil {
				r.thread.naming.Events().ResetProject(context.Background(), id)
			}
			if cleanupErr := r.history.Purge(id); cleanupErr != nil {
				r.log.Warn("deleted project container cleanup deferred")
			}
		}
		original.WriteHeader(buffer.Status())
		_, _ = original.Write(buffer.body.Bytes())
	}
}
