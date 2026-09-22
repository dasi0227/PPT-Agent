package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const commandHistoryKey = "command-history"

type commandHistoryRecord struct {
	activity       model.CommandActivity
	service        *service.ThreadService
	previousResult json.RawMessage
}

func commandRecord(c *gin.Context) *commandHistoryRecord {
	value, _ := c.Get(commandHistoryKey)
	record, _ := value.(*commandHistoryRecord)
	return record
}

func (record *commandHistoryRecord) save() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return record.service.SaveCommandActivity(ctx, record.activity)
}

func (record *commandHistoryRecord) settle(result any, err error) error {
	if err != nil {
		record.activity.Result = record.previousResult
		record.activity.Status = "failed"
		if errors.Is(err, context.Canceled) {
			record.activity.Status = "canceled"
		}
	} else {
		raw, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return marshalErr
		}
		record.activity.Result = raw
		record.activity.Status = "completed"
	}
	return record.save()
}

type commandCaptureWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *commandCaptureWriter) Write(raw []byte) (int, error) {
	_, _ = w.body.Write(raw)
	return w.ResponseWriter.Write(raw)
}
func (w *commandCaptureWriter) WriteString(raw string) (int, error) { return w.Write([]byte(raw)) }

// Runs outside the authoring-history gate so failed/canceled attempts survive
// its rollback. Successful stream results are also saved before its checkpoint.
func (r *Router) commandHistory() gin.HandlerFunc {
	return func(c *gin.Context) {
		if r.thread == nil || r.thread.svc == nil {
			c.Next()
			return
		}
		parts := strings.Split(strings.Trim(c.Request.URL.Path, "/"), "/")
		if len(parts) < 4 || parts[0] != "api" || parts[1] != "v1" {
			c.Next()
			return
		}
		kind, method, threadID := "", "auto", ""
		if len(parts) == 5 && c.Request.Method == http.MethodPost {
			if parts[2] == "projects" && (parts[4] == "kickoff" || parts[4] == "handoff" || parts[4] == "polish") {
				kind = parts[4]
			}
			if parts[2] == "threads" && (parts[4] == "rename" || parts[4] == "compact" || parts[4] == "naming") {
				kind = parts[4]
				threadID = parts[3]
			}
		} else if len(parts) == 4 && parts[2] == "threads" && c.Request.Method == http.MethodPatch {
			kind = "rename"
			method = "manual"
			threadID = parts[3]
		}
		if kind == "" {
			c.Next()
			return
		}
		raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
		if err != nil {
			AbortWithError(c, ErrBadRequest("invalid command body"))
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(raw))
		var body map[string]json.RawMessage
		if json.Unmarshal(raw, &body) != nil {
			c.Next()
			return
		}
		if kind == "naming" {
			var action string
			_ = json.Unmarshal(body["action"], &action)
			if action != "manual" {
				c.Next()
				return
			}
			kind = "rename"
			method = "manual"
		}
		if kind == "compact" {
			method = "manual"
		}
		if threadID == "" {
			_ = json.Unmarshal(body["thread_id"], &threadID)
		}
		thread, err := r.thread.svc.GetThread(c.Request.Context(), threadID)
		if err != nil {
			AbortWithError(c, ErrBadRequest("command requires an existing thread"))
			return
		}
		if parts[2] == "projects" && thread.ProjectID != parts[3] {
			AbortWithError(c, ErrBadRequest("thread does not belong to project"))
			return
		}
		activity, err := r.thread.svc.BeginCommandActivity(c.Request.Context(), c.GetHeader("X-Command-ID"), threadID, kind, method, raw)
		if err != nil {
			if errors.Is(err, store.ErrCommandActivityConflict) {
				AbortWithError(c, ErrConflict(err.Error()))
			} else {
				AbortWithError(c, ProjectAgentError(err, "INTERNAL", kind))
			}
			return
		}
		record := &commandHistoryRecord{activity: activity, service: r.thread.svc, previousResult: activity.Result}
		c.Set(commandHistoryKey, record)
		c.Header("X-Command-ID", activity.ID)
		original := c.Writer
		capture := &commandCaptureWriter{ResponseWriter: original}
		c.Writer = capture
		defer func() {
			c.Writer = original
			if capture.Status() >= 400 {
				record.activity.Status = "failed"
				record.activity.Result = record.previousResult
			}
			if record.activity.Status == "loading" {
				record.activity.Status = "failed"
				if c.Request.Context().Err() != nil {
					record.activity.Status = "canceled"
				} else if capture.Status() < 400 && json.Valid(capture.body.Bytes()) {
					var result map[string]json.RawMessage
					_ = json.Unmarshal(capture.body.Bytes(), &result)
					data := json.RawMessage(capture.body.Bytes())
					if thread, ok := result["thread"]; ok {
						data = thread
					}
					record.activity.Result = data
					record.activity.Status = "completed"
				}
			}
			if err := record.save(); err != nil {
				r.log.Error("persist command activity", zap.String("command_id", activity.ID), zap.Error(err))
			}
		}()
		c.Next()
	}
}
