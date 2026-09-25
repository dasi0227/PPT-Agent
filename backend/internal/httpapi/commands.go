package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/gin-gonic/gin"
)

func (r *Router) executeCommand(ctx context.Context, e model.CommandExecution) (any, error) {
	switch e.Kind {
	case "rename":
		var input struct {
			Mode  string `json:"mode"`
			Title string `json:"title"`
		}
		if err := json.Unmarshal(e.Input, &input); err != nil {
			return nil, err
		}
		if r.thread.naming == nil {
			return nil, errors.New("naming unavailable")
		}
		var thread model.Thread
		var err error
		if input.Mode == "manual" {
			thread, err = r.thread.naming.ManualRename(ctx, e.ThreadID, input.Title)
		} else if input.Mode == "automatic" {
			thread, err = r.thread.naming.GenerateNow(ctx, e.ThreadID)
		} else {
			return nil, errors.New("rename mode must be manual or automatic")
		}
		return toThreadResponse(thread), err
	case "polish":
		if r.polish == nil {
			return nil, errors.New("polish unavailable")
		}
		var input polishRequest
		if err := json.Unmarshal(e.Input, &input); err != nil {
			return nil, err
		}
		result, err := r.polish.svc.Polish(ctx, e.ProjectID, service.PolishParams{Instruction: input.Instruction, Feedback: e.Feedback, ThreadID: e.ThreadID, ScopeInput: input.Scope, Mode: input.Mode})
		return gin.H{"title": result.Title, "content": result.Content, "changed": result.Changed, "model_execution": result.ModelExecution, "prompt_version": result.PromptVersion}, err
	case "kickoff", "handoff":
		if r.briefing == nil {
			return nil, errors.New("briefing unavailable")
		}
		params := service.BriefingParams{ThreadID: e.ThreadID, Feedback: e.Feedback}
		if e.BaseAttemptID != "" {
			previous, err := r.commands.Get(ctx, e.CommandID)
			if err != nil {
				return nil, err
			}
			if previous.LatestSuccess == nil || previous.LatestSuccess.AttemptID != e.BaseAttemptID {
				return nil, errors.New("base attempt is not the last successful briefing")
			}
			var result struct {
				Briefing struct {
					ID string `json:"briefing_id"`
				} `json:"briefing"`
			}
			if err := json.Unmarshal(previous.LatestSuccess.Result, &result); err != nil {
				return nil, err
			}
			params.BriefingID = result.Briefing.ID
		}
		if e.Kind == "kickoff" {
			return r.briefing.kickoff.Generate(ctx, e.ProjectID, params)
		}
		return r.briefing.handoff.Generate(ctx, e.ProjectID, params)
	case "compact":
		if r.contextWindow == nil {
			return nil, errors.New("compaction unavailable")
		}
		return r.contextWindow.svc.Compact(ctx, e.ThreadID, "")
	case "commit":
		if r.gitCommit == nil {
			return nil, errors.New("Git unavailable")
		}
		return r.gitCommit.svc.ExecuteCommand(ctx, e)
	}
	return nil, errors.New("unsupported command")
}
func (r *Router) createCommand(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	var request model.CommandRequest
	if c.ShouldBindJSON(&request) != nil {
		AbortWithError(c, ErrBadRequest("invalid command"))
		return
	}
	scene := int64(0)
	if r.history != nil {
		thread, err := r.thread.svc.GetThread(c.Request.Context(), c.Param("id"))
		if err != nil {
			AbortWithError(c, ErrNotFound("thread not found"))
			return
		}
		state, err := r.history.State(thread.ProjectID)
		if err != nil {
			historyError(c, err)
			return
		}
		scene = state.SceneRevision
	}
	accepted, err := r.commands.Accept(c.Request.Context(), c.Param("id"), request, scene)
	if err != nil {
		abortCommandError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, accepted)
}
func (r *Router) getCommand(c *gin.Context) {
	value, err := r.commands.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		abortCommandError(c, err)
		return
	}
	c.JSON(http.StatusOK, value)
}
func (r *Router) cancelCommand(c *gin.Context) {
	var input struct {
		AttemptID  string `json:"attempt_id"`
		RequestKey string `json:"request_key"`
	}
	if c.ShouldBindJSON(&input) != nil {
		AbortWithError(c, ErrBadRequest("invalid cancellation"))
		return
	}
	value, err := r.commands.Cancel(c.Request.Context(), c.Param("id"), input.AttemptID, input.RequestKey)
	if err != nil {
		abortCommandError(c, err)
		return
	}
	c.JSON(http.StatusOK, value)
}
func threadCursor(scene int64, threadID string, seq int64) string {
	return strconv.FormatInt(scene, 10) + ":" + threadID + ":" + strconv.FormatInt(seq, 10)
}
func (r *Router) threadJournalEvents(c *gin.Context) {
	thread, err := r.thread.svc.GetThread(c.Request.Context(), c.Param("id"))
	if err != nil {
		AbortWithError(c, ErrNotFound("thread not found"))
		return
	}
	scene := int64(0)
	if r.history != nil {
		state, err := r.history.State(thread.ProjectID)
		if err != nil {
			historyError(c, err)
			return
		}
		scene = state.SceneRevision
	}
	after := int64(0)
	cursor := c.GetHeader("Last-Event-ID")
	if cursor == "" {
		cursor = c.Query("cursor")
	}
	if cursor != "" {
		parts := strings.Split(cursor, ":")
		if len(parts) != 3 || parts[0] != strconv.FormatInt(scene, 10) || parts[1] != thread.ID {
			AbortWithError(c, ErrConflict("history cursor expired; reload history"))
			return
		}
		after, err = strconv.ParseInt(parts[2], 10, 64)
		if err != nil || after < 0 {
			AbortWithError(c, ErrBadRequest("invalid history cursor"))
			return
		}
	}
	writer, err := newSSEWriter(c)
	if err != nil {
		AbortWithError(c, ErrInternal("streaming unsupported"))
		return
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	heartbeat := time.NewTicker(sseHeartbeatInterval)
	defer heartbeat.Stop()
	for {
		if r.history != nil {
			state, err := r.history.State(thread.ProjectID)
			if err != nil || state.SceneRevision != scene {
				_ = writer.event("", "history.reset", `{}`)
				return
			}
		}
		if r.history != nil && r.history.Switching(thread.ProjectID) {
			_ = writer.event("", "history.reset", `{}`)
			return
		}
		events, err := r.thread.svc.JournalEvents(c.Request.Context(), thread.ID, after)
		if err != nil {
			return
		}
		if r.history != nil {
			state, stateErr := r.history.State(thread.ProjectID)
			if stateErr != nil || r.history.Switching(thread.ProjectID) || state.SceneRevision != scene {
				_ = writer.event("", "history.reset", `{}`)
				return
			}
		}
		for _, event := range events {
			entry, visible, err := service.PublicThreadEvent(event)
			if err != nil {
				return
			}
			after = event.Seq
			if !visible {
				continue
			}
			raw, err := json.Marshal(entry)
			if err != nil {
				return
			}
			if writer.event(threadCursor(scene, thread.ID, event.Seq), "thread.event", string(raw)) != nil {
				return
			}
		}
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
		case <-heartbeat.C:
			if writer.heartbeat() != nil {
				return
			}
		}
	}
}

func (r *Router) threadHistory(c *gin.Context) {
	thread, err := r.thread.svc.GetThread(c.Request.Context(), c.Param("id"))
	if err != nil {
		AbortWithError(c, ErrNotFound("thread not found"))
		return
	}
	scene := int64(0)
	if r.history != nil {
		state, err := r.history.State(thread.ProjectID)
		if err != nil {
			historyError(c, err)
			return
		}
		scene = state.SceneRevision
	}
	if r.history != nil && r.history.Switching(thread.ProjectID) {
		AbortWithError(c, ErrConflict("project history is switching; retry"))
		return
	}
	events, err := r.thread.svc.JournalEvents(c.Request.Context(), thread.ID, 0)
	if err != nil {
		AbortWithError(c, ErrInternal(err.Error()))
		return
	}
	if r.history != nil {
		state, stateErr := r.history.State(thread.ProjectID)
		if stateErr != nil || r.history.Switching(thread.ProjectID) || state.SceneRevision != scene {
			AbortWithError(c, ErrConflict("project history changed; reload"))
			return
		}
	}
	out := []map[string]any{}
	seq := int64(0)
	for _, event := range events {
		entry, visible, err := service.PublicThreadEvent(event)
		if err != nil {
			AbortWithError(c, ErrInternal(err.Error()))
			return
		}
		seq = event.Seq
		if visible {
			out = append(out, entry)
		}
	}
	c.JSON(http.StatusOK, gin.H{"events": out, "cursor": threadCursor(scene, thread.ID, seq), "scene_revision": scene, "thread_id": thread.ID, "seq": seq})
}

func abortCommandError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("command not found"))
	case errors.Is(err, store.ErrCommandConflict), errors.Is(err, store.ErrRunActive), errors.Is(err, store.ErrGitCommitActive):
		AbortWithError(c, ErrConflict(err.Error()))
	default:
		AbortWithError(c, ProjectAgentError(err, "INTERNAL", "command"))
	}
}
