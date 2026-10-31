package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// RecordedToolResults projects durable outcomes that may have been saved before
// event publication failed. Reading a stopped run never re-executes its tools.
func RecordedToolResults(ctx context.Context, cp RuntimeCheckpoint, projectDir string, receipts IdempotencyStore) ([]model.ToolCompletedPayload, error) {
	projector := ToolPublicProjector{ProjectDir: projectDir, TextContext: model.PublicTextContext{
		HiddenValues: []string{projectDir, cp.RunID, cp.LoopID},
	}}
	results := []model.ToolCompletedPayload{}
	appendResult := func(call llm.ToolCall, result ToolResult) {
		payload, ok := projector.Completed(cp.RunID, call.ID, call.Name, call.Args, result)
		payload.OccurredAt = time.UnixMilli(cp.CreatedAt).UTC().Format(time.RFC3339Nano)
		if ok && model.ValidatePublicEvent(model.EventToolCompleted, payload) == nil {
			results = append(results, payload)
		}
	}
	if pending := cp.PendingReview; pending != nil && pending.Result != nil {
		appendResult(pending.Call, *pending.Result)
	}
	if cp.PendingToolBatch != nil && receipts != nil {
		for _, call := range cp.PendingToolBatch.Calls {
			record, err := receipts.GetIdempotency(ctx, "tool_call", cp.RunID, call.ID)
			if errors.Is(err, model.ErrRunNotFound) {
				continue
			}
			if err != nil {
				return results, err
			}
			if record.Status != "completed" || !validToolReceipt(record.ResultJSON) {
				continue
			}
			var stored persistedToolResult
			if json.Unmarshal([]byte(record.ResultJSON), &stored) == nil {
				appendResult(call, stored.restoreResult())
			}
		}
	}
	return results, nil
}
