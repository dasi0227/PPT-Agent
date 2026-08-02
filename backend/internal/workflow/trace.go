package workflow

import "go.uber.org/zap"

type TraceEvent struct {
	Type    string         `json:"type"`
	RunID   string         `json:"run_id"`
	Payload map[string]any `json:"payload"`
}

type TraceRecorder interface {
	Record(TraceEvent)
}

type ZapTraceRecorder struct {
	Logger *zap.Logger
}

func (r ZapTraceRecorder) Record(event TraceEvent) {
	if r.Logger == nil {
		return
	}
	r.Logger.Debug(
		"runtime trace",
		zap.String("trace_type", event.Type),
		zap.String("run_id", event.RunID),
		zap.Any("payload", event.Payload),
	)
}

func recordTrace(recorder TraceRecorder, runID, eventType string, payload map[string]any) {
	if recorder == nil {
		return
	}
	recorder.Record(TraceEvent{Type: eventType, RunID: runID, Payload: payload})
}
