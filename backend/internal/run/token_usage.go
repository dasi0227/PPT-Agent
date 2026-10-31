package run

import (
	"context"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/modelusage"
	"go.uber.org/zap"
)

func (e *Engine) withUsageRecorder(ctx context.Context, a *active) context.Context {
	return modelusage.WithRecorder(ctx, func(_ context.Context, call model.ModelCallUsage) error {
		if a.isDeleted() {
			return nil
		}
		// Consumption is an immutable fact about a finished request. Persist it
		// even after cancellation or a newer worker claims the Run; it carries no
		// execution authority and never updates checkpoints or Run lifecycle.
		writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := a.bus.Emit(writeCtx, model.EventRunUsageUpdated, model.RunUsageUpdatedPayload{
			PublicEventBase: model.NewPublicEventBase(a.run.ID), Call: call,
		})
		if err != nil && !a.isDeleted() && e.log != nil {
			e.log.Warn("model usage recording failed", zap.String("run_id", a.run.ID), zap.String("call_id", call.CallID), zap.Error(err))
		}
		return err
	})
}
