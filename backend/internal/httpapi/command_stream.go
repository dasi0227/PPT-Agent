package httpapi

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/gin-gonic/gin"
)

// Each request owns its execution context. Disconnecting cancels the provider and
// prevents a late result from being written or persisted by the service.
func commandStream(c *gin.Context, operation string, execute func(context.Context) (any, error)) {
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	finish := func(bool) error { return nil }
	if buffer, ok := c.Writer.(*historyResponse); ok {
		// Progress must bypass the normal mutation response buffer. The terminal
		// frame remains gated by durable history settlement.
		buffer.streamed = true
		c.Writer = buffer.ResponseWriter
		defer func() { c.Writer = buffer }()
		if buffer.finish != nil {
			finish = buffer.finish
		}
	}
	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	encoder := json.NewEncoder(c.Writer)
	send := func(value any) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := encoder.Encode(value); err != nil {
			cancel()
			return err
		}
		c.Writer.Flush()
		return nil
	}
	ctx = service.WithCommandProgress(ctx, func(phase int) error {
		if record := commandRecord(c); record != nil {
			record.activity.Phase = phase
			if err := record.save(); err != nil {
				return err
			}
		}
		return send(gin.H{"type": "phase", "phase": phase})
	})
	result, err := execute(ctx)
	if record := commandRecord(c); record != nil {
		if persistErr := record.settle(result, err); persistErr != nil {
			err = persistErr
		}
	}
	if finishErr := finish(err == nil); finishErr != nil {
		err = finishErr
	}
	if err != nil {
		if record := commandRecord(c); record != nil {
			_ = record.settle(nil, err)
		}
	}
	if ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return
	}
	if err != nil {
		public := ProjectAgentError(err, "INTERNAL", operation)
		_ = send(gin.H{"type": "error", "error": gin.H{"code": public.Code, "message": public.Message, "retryable": public.Retryable}})
		return
	}
	_ = send(gin.H{"type": "result", "result": result})
}
