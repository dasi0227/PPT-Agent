package run

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Bus 是单个 Run 的事件总线：分配连续 seq、持久化、扇出给订阅者（SSE）。
// seq 单调递增且连续（API-SSE-001）；终态事件恰好一个（API-SSE-002）由 engine 保证。
type Bus struct {
	runID    string
	threadID string
	store    Store
	hw       HistoryWriter

	mu          sync.Mutex
	seq         int64
	subscribers map[int]chan model.Event
	nextSubID   int
	closed      bool
	terminated  bool // 已发过终态事件，防止重复终态
}

func NewBus(runID string, threadID string, store Store, hw HistoryWriter) *Bus {
	return &Bus{runID: runID, threadID: threadID, store: store, hw: hw, subscribers: map[int]chan model.Event{}}
}

// isWhitelistedForHistory keeps replayable Adaptive Runtime events while never
// persisting hidden model reasoning.
func isWhitelistedForHistory(evt model.EventType) bool {
	switch evt {
	case model.EventRunStarted, model.EventContextAssembled, model.EventStrategySelected,
		model.EventPlanCreated, model.EventStageStarted, model.EventStageCompleted,
		model.EventStepStarted, model.EventStepCompleted, model.EventStepFailed,
		model.EventToolCalled, model.EventToolCompleted, model.EventVerificationCompleted,
		model.EventRepairStarted, model.EventRepairCompleted, model.EventArtifactStaged,
		model.EventArtifactCommitted, model.EventStatusSummary, model.EventNeedsInput,
		model.EventRunCompleted, model.EventRunFailed, model.EventRunCanceled:
		return true
	}
	return false
}

// buildHistoryEntry 把 SSE 事件映射成 history schema（UX §2.3）。
// 返回 (entry, ok)；ok=false 时该事件不落盘（例如 run.started 无 user_input）。
func buildHistoryEntry(e model.Event) (HistoryEntry, bool) {
	var data map[string]any
	_ = json.Unmarshal([]byte(e.Payload), &data)
	entry := HistoryEntry{Seq: e.Seq, TS: e.CreatedAt, RunID: e.RunID, Data: data}
	switch e.Type {
	case model.EventRunStarted:
		text, _ := data["user_input"].(string)
		if text == "" {
			return HistoryEntry{}, false
		}
		entry.Turn = "user"
		entry.Type = "user_turn"
		entry.Data = map[string]any{
			"text": text, "target": data["target"], "interaction": data["interaction"],
		}
	case model.EventContextAssembled:
		entry.Turn = "agent"
		entry.Type = "context_assembled"
	case model.EventStatusSummary:
		entry.Turn = "agent"
		entry.Type = "markdown"
		entry.Data = map[string]any{"text": data["summary"]}
	case model.EventNeedsInput:
		entry.Turn = "agent"
		entry.Type = "needs_input"
	case model.EventRunCompleted:
		entry.Turn = "agent"
		entry.Type = "final_result"
		entry.Data = map[string]any{"result": data["outcome"]}
	case model.EventRunFailed, model.EventRunCanceled:
		entry.Turn = "agent"
		entry.Type = "error"
		if outcome, ok := data["outcome"].(map[string]any); ok {
			entry.Data = map[string]any{
				"code": outcome["code"], "message": outcome["message"],
				"status": outcome["status"],
			}
		}
	default:
		entry.Turn = "agent"
		entry.Type = string(e.Type)
	}
	return entry, true
}

// Emit 分配下一个 seq，持久化后扇出。终态事件（done/error）只允许发一次。
func (b *Bus) Emit(ctx context.Context, evt model.EventType, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		raw = []byte("{}")
	}
	b.mu.Lock()
	if b.terminated {
		b.mu.Unlock()
		return nil
	}
	if evt.Terminal() {
		b.terminated = true
	}
	b.seq++
	e := model.Event{
		RunID:     b.runID,
		Seq:       b.seq,
		Type:      evt,
		Payload:   string(raw),
		CreatedAt: time.Now().Unix(),
	}
	subs := make([]chan model.Event, 0, len(b.subscribers))
	for _, ch := range b.subscribers {
		subs = append(subs, ch)
	}
	b.mu.Unlock()

	// 先持久化再扇出：保证断线重连能从 store 补齐（ARCH-RUN-005）。
	if err := b.store.AppendEvent(ctx, e); err != nil {
		return err
	}
	// history.jsonl 副作用：仅白名单事件，失败吞掉不阻塞 SSE 主流程。
	if b.hw != nil && isWhitelistedForHistory(evt) && b.threadID != "" {
		if entry, ok := buildHistoryEntry(e); ok {
			_ = b.hw.Append(ctx, b.threadID, entry)
		}
	}
	for _, ch := range subs {
		select {
		case ch <- e:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Subscribe 注册一个实时订阅者，返回事件 channel 与取消函数。
func (b *Bus) Subscribe() (<-chan model.Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.nextSubID
	b.nextSubID++
	ch := make(chan model.Event, 64)
	b.subscribers[id] = ch
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if sub, ok := b.subscribers[id]; ok {
			delete(b.subscribers, id)
			close(sub)
		}
	}
}

// Close 关闭总线并释放所有订阅者。
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for id, ch := range b.subscribers {
		delete(b.subscribers, id)
		close(ch)
	}
}

func (b *Bus) Terminated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.terminated
}
