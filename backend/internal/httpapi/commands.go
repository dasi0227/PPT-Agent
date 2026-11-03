package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (r *Router) executeCommand(ctx context.Context, e model.CommandExecution) (any, error) {
	switch e.Kind {
	case "suggest":
		if r.suggest == nil {
			return nil, errors.New("suggestions unavailable")
		}
		return r.suggest.Generate(ctx, e)
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
		if input.Mode == "manual" {
			return r.thread.naming.ManualRename(ctx, e.ThreadID, input.Title)
		} else if input.Mode == "automatic" {
			return r.thread.naming.GenerateNow(ctx, e.ThreadID)
		}
		return nil, errors.New("rename mode must be manual or automatic")
	case "polish":
		if r.polish == nil {
			return nil, errors.New("polish unavailable")
		}
		var input polishRequest
		if err := json.Unmarshal(e.Input, &input); err != nil {
			return nil, err
		}
		result, err := r.polish.svc.Polish(ctx, e.ProjectID, service.PolishParams{Instruction: input.Instruction, PolishContent: input.PolishContent, Feedback: e.Feedback, ThreadID: e.ThreadID})
		return gin.H{"title": result.Title, "content": result.Content, "changed": result.Changed, "model_execution": result.ModelExecution, "prompt_version": result.PromptVersion}, err
	case "handoff":
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
	var input struct {
		model.CommandRequest
		SceneRevision int64  `json:"scene_revision,string"`
		Revision      *int64 `json:"entity_revision,string"`
	}
	if c.ShouldBindJSON(&input) != nil || input.SceneRevision < 1 || input.Revision == nil || *input.Revision < 0 {
		AbortWithError(c, ErrBadRequest("scene_revision and entity_revision are required decimal strings"))
		return
	}
	input.CommandRequest.ExpectedRevision = *input.Revision
	ctx := sessionstate.WithRequestScene(c.Request.Context(), input.SceneRevision)
	accepted, err := r.commands.Accept(ctx, c.Param("id"), input.CommandRequest, input.SceneRevision)
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
		SceneRevision  int64  `json:"scene_revision,string"`
		EntityRevision int64  `json:"entity_revision,string"`
		AttemptID      string `json:"attempt_id"`
		RequestKey     string `json:"request_key"`
	}
	if c.ShouldBindJSON(&input) != nil || input.SceneRevision < 1 || input.EntityRevision < 1 {
		AbortWithError(c, ErrBadRequest("invalid cancellation"))
		return
	}
	ctx := sessionstate.WithEntityRevision(sessionstate.WithRequestScene(c.Request.Context(), input.SceneRevision), input.EntityRevision)
	value, err := r.commands.Cancel(ctx, c.Param("id"), input.AttemptID, input.RequestKey)
	if err != nil {
		abortCommandError(c, err)
		return
	}
	c.JSON(http.StatusOK, value)
}
func abortCommandError(c *gin.Context, err error) {
	if abortSessionError(c, err) {
		return
	}
	switch {
	case errors.Is(err, run.ErrRunNotFound):
		AbortWithError(c, ErrNotFound("command not found"))
	case errors.Is(err, store.ErrCommandConflict), errors.Is(err, store.ErrRunActive), errors.Is(err, store.ErrGitCommitActive):
		AbortWithError(c, ErrConflict(err.Error()))
	default:
		AbortWithError(c, ProjectAgentError(err, "INTERNAL", "command"))
	}
}

// Automatic suggestions use the same durable command path as manual requests.
func (r *Router) WithSuggestions(svc *service.SuggestService) *Router {
	r.suggest = svc
	r.thread.suggest = svc
	r.run.svc.WithCompletedRunObserver(func(current model.Run) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		thread, err := r.thread.svc.GetThread(ctx, current.ThreadID)
		if err != nil || !thread.AutoSuggestEnabled {
			return
		}
		scene := current.SceneRevision
		_, err = r.commands.Accept(ctx, current.ThreadID, model.CommandRequest{
			Kind: "suggest", Source: "automatic", RunID: current.ID,
			RequestKey: "suggest-" + current.ID, Input: json.RawMessage(`{}`),
		}, scene)
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, store.ErrRunActive) {
			r.log.Warn("start automatic suggestions failed", zap.String("run_id", current.ID), zap.Error(err))
		}
	})
	return r
}
