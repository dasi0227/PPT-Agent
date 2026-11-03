package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/gin-gonic/gin"
)

func sessionReadError(c *gin.Context, err error) {
	if abortSessionError(c, err) {
		return
	}
	if errors.Is(err, run.ErrRunNotFound) {
		AbortWithError(c, ErrNotFound("session entity not found"))
		return
	}
	AbortWithError(c, ProjectAgentError(err, "INTERNAL", "session"))
}

// An execution can finish while its terminal write remains unavailable. Check
// that process-local failure at existing read boundaries, including after reload.
func (r *Router) sessionPersistence(ctx context.Context, threadID string, scene int64) error {
	if r.commands == nil {
		return nil
	}
	return r.commands.CheckThreadPersistence(ctx, threadID, scene)
}

func sessionLimit(c *gin.Context, name string, fallback, maximum int) (int, bool) {
	raw := c.Query(name)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		AbortWithError(c, ErrBadRequest("invalid page limit"))
		return 0, false
	}
	return min(value, maximum), true
}

func sessionPageRequest(c *gin.Context) (context.Context, *sessionstate.PageCursor, bool) {
	scene, err := strconv.ParseInt(c.Query("scene_revision"), 10, 64)
	if err != nil || scene < 1 {
		AbortWithError(c, ErrBadRequest("scene_revision is required"))
		return nil, nil, false
	}
	ctx := sessionstate.WithRequestScene(c.Request.Context(), scene)
	raw := c.Query("before")
	if raw == "" {
		return ctx, nil, true
	}
	cursor, err := sessionstate.DecodePageCursor(raw)
	if err != nil {
		sessionReadError(c, err)
		return nil, nil, false
	}
	if cursor.ThreadID != c.Param("id") || cursor.SceneRevision != scene {
		sessionReadError(c, sessionstate.ErrSceneChanged)
		return nil, nil, false
	}
	return ctx, &cursor, true
}

func (r *Router) sessionView(c *gin.Context) {
	limit, ok := sessionLimit(c, "group_limit", 20, 50)
	if !ok {
		return
	}
	value, err := r.thread.svc.ReadState().SessionView(c.Request.Context(), c.Param("id"), limit)
	if err == nil {
		err = r.sessionPersistence(c.Request.Context(), c.Param("id"), value.SceneRevision)
	}
	if err != nil {
		sessionReadError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, value)
}

func (r *Router) sessionTimeline(c *gin.Context) {
	ctx, before, ok := sessionPageRequest(c)
	if !ok {
		return
	}
	limit, ok := sessionLimit(c, "limit", 20, 50)
	if !ok {
		return
	}
	value, err := r.thread.svc.ReadState().Timeline(ctx, c.Param("id"), before, limit)
	if err != nil {
		sessionReadError(c, err)
		return
	}
	c.JSON(http.StatusOK, value)
}

func (r *Router) sessionItems(c *gin.Context) {
	ctx, before, ok := sessionPageRequest(c)
	if !ok {
		return
	}
	limit, ok := sessionLimit(c, "limit", 50, 100)
	if !ok {
		return
	}
	value, err := r.thread.svc.ReadState().TimelineItems(ctx, c.Param("id"), c.Param("groupId"), before, limit)
	if err != nil {
		sessionReadError(c, err)
		return
	}
	c.JSON(http.StatusOK, value)
}

func (r *Router) sessionDetail(c *gin.Context) {
	ref, err := sessionstate.DecodeContentRef(c.Query("ref"))
	if err != nil {
		sessionReadError(c, err)
		return
	}
	value, err := r.thread.svc.ReadState().TimelineContent(c.Request.Context(), c.Param("id"), c.Param("itemId"), ref)
	if err != nil {
		sessionReadError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json", value)
}

func (r *Router) sessionPreview(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	scene, err := strconv.ParseInt(c.Query("scene_revision"), 10, 64)
	if err != nil || scene < 1 || strconv.FormatInt(scene, 10) != c.Query("scene_revision") {
		AbortWithError(c, ErrBadRequest("scene_revision is required"))
		return
	}
	value, err := r.thread.svc.TimelinePreview(sessionstate.WithRequestScene(c.Request.Context(), scene), c.Param("id"), c.Param("itemId"))
	if err != nil {
		sessionReadError(c, err)
		return
	}
	c.JSON(http.StatusOK, value)
}

func sessionCursor(c *gin.Context) (sessionstate.Cursor, bool) {
	query := c.Request.URL.Query()
	raw := query.Get("cursor")
	if !query.Has("cursor") {
		raw = c.GetHeader("Last-Event-ID")
	}
	cursor, err := sessionstate.DecodeCursor(raw)
	if err != nil {
		sessionReadError(c, err)
		return cursor, false
	}
	if cursor.ThreadID != c.Param("id") {
		sessionReadError(c, sessionstate.ErrCursorReset)
		return cursor, false
	}
	return cursor, true
}

func (r *Router) sessionChanges(c *gin.Context) {
	cursor, ok := sessionCursor(c)
	if !ok {
		return
	}
	limit, ok := sessionLimit(c, "limit", 100, 500)
	if !ok {
		return
	}
	value, err := r.thread.svc.ReadState().SessionChanges(c.Request.Context(), cursor, limit)
	if err == nil {
		err = r.sessionPersistence(c.Request.Context(), cursor.ThreadID, cursor.SceneRevision)
	}
	if err != nil {
		sessionReadError(c, err)
		return
	}
	c.JSON(http.StatusOK, value)
}

func (r *Router) sessionEvents(c *gin.Context) {
	cursor, ok := sessionCursor(c)
	if !ok {
		return
	}
	reader := r.thread.svc.ReadState()
	// Register first: changes committed after the snapshot and before handshake
	// are either in this range read or represented by a pending wakeup.
	wake, stop := reader.SubscribeSession(cursor.ThreadID)
	defer stop()
	page, err := reader.SessionChanges(c.Request.Context(), cursor, 100)
	if err == nil {
		err = r.sessionPersistence(c.Request.Context(), cursor.ThreadID, cursor.SceneRevision)
	}
	if err != nil {
		sessionReadError(c, err)
		return
	}
	writer, err := newSSEWriter(c)
	if err != nil {
		AbortWithError(c, ErrInternal("streaming unsupported"))
		return
	}
	if writer.heartbeat() != nil {
		return
	}
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		for _, batch := range page.Batches {
			raw, err := json.Marshal(batch)
			if err != nil {
				return
			}
			next := batch.Cursor()
			if writer.event(next.Encode(), "thread.change", string(raw)) != nil {
				return
			}
			cursor = next
		}
		if !page.HasMore {
			select {
			case <-c.Request.Context().Done():
				return
			case <-wake:
			case <-heartbeat.C:
				if writer.heartbeat() != nil {
					return
				}
			}
		}
		page, err = reader.SessionChanges(c.Request.Context(), cursor, 100)
		if err == nil {
			err = r.sessionPersistence(c.Request.Context(), cursor.ThreadID, cursor.SceneRevision)
		}
		if err != nil {
			reason := "SYNC_FAILED"
			switch {
			case errors.Is(err, sessionstate.ErrSceneChanged):
				reason = "SCENE_CHANGED"
			case errors.Is(err, sessionstate.ErrCursorReset):
				reason = "CURSOR_RESET_REQUIRED"
			case errors.Is(err, sessionstate.ErrProjectRestoring):
				reason = "PROJECT_RESTORING"
			case errors.Is(err, run.ErrRunNotFound):
				reason = "NOT_FOUND"
			case model.AsAgentError(err, "INTERNAL", "session").Code == model.CodePersistenceFailed:
				reason = model.CodePersistenceFailed
			}
			raw, _ := json.Marshal(map[string]string{"reason": reason})
			_ = writer.event("", "thread.reset", string(raw))
			return
		}
	}
}
