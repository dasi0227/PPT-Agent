package run

import (
	"context"
	"encoding/json"
	"errors"
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

	mu              sync.Mutex
	seq             int64
	subscribers     map[int]chan model.Event
	nextSubID       int
	closed          bool
	terminated      bool // 已发过终态事件，防止重复终态
	terminalStatus  string
	cancelRequested bool
	started         bool
	finalCount      int
	planID          string
	planRevision    int
	planCompleted   map[string]bool
	toolCalls       map[string]bool
	toolNames       map[string]string
	questions       map[string]bool
}

func NewBus(runID string, threadID string, store Store, hw HistoryWriter) *Bus {
	return &Bus{
		runID: runID, threadID: threadID, store: store, hw: hw,
		subscribers:   map[int]chan model.Event{},
		planCompleted: map[string]bool{}, toolCalls: map[string]bool{},
		toolNames: map[string]string{}, questions: map[string]bool{},
	}
}

// isWhitelistedForHistory persists only product history. Progress remains in the
// run event store for Last-Event-ID replay but is intentionally transient here.
func isWhitelistedForHistory(evt model.EventType) bool {
	switch evt {
	case model.EventRunStarted, model.EventPlanUpdated,
		model.EventMessageReasoning, model.EventMessageMilestone, model.EventMessageFinal,
		model.EventToolStarted, model.EventToolCompleted,
		model.EventQuestionAsked, model.EventQuestionAnswered, model.EventRunFinished:
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
	default:
		entry.Turn = "agent"
		entry.Type = string(e.Type)
	}
	return entry, true
}

// Emit 分配下一个 seq，持久化后扇出。终态事件（done/error）只允许发一次。
func (b *Bus) Emit(ctx context.Context, evt model.EventType, payload any) error {
	if err := model.ValidatePublicEvent(evt, payload); err != nil {
		return err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	if data["run_id"] != b.runID {
		return errors.New("public event run_id does not match bus")
	}

	b.mu.Lock()
	if b.terminated {
		b.mu.Unlock()
		return nil
	}
	if !b.started && evt != model.EventRunStarted {
		b.mu.Unlock()
		return errors.New("run.started must be the first public event")
	}
	if b.started && evt == model.EventRunStarted {
		b.mu.Unlock()
		return errors.New("run.started may only be emitted once")
	}
	if err := b.validateSequence(evt, data); err != nil {
		b.mu.Unlock()
		return err
	}
	nextSeq := b.seq + 1
	e := model.Event{
		RunID:     b.runID,
		Seq:       nextSeq,
		Type:      evt,
		Payload:   string(raw),
		CreatedAt: time.Now().Unix(),
	}
	subs := make([]chan model.Event, 0, len(b.subscribers))
	for _, ch := range b.subscribers {
		subs = append(subs, ch)
	}

	// 先持久化再扇出：保证断线重连能从 store 补齐（ARCH-RUN-005）。
	if err := b.store.AppendEvent(ctx, e); err != nil {
		b.mu.Unlock()
		return err
	}
	b.seq = nextSeq
	b.recordSequence(evt, data)
	// history.jsonl 副作用：仅白名单事件，失败吞掉不阻塞 SSE 主流程。
	if b.hw != nil && isWhitelistedForHistory(evt) && b.threadID != "" {
		if entry, ok := buildHistoryEntry(e); ok {
			_ = b.hw.Append(ctx, b.threadID, entry)
		}
	}
	b.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- e:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (b *Bus) validateSequence(evt model.EventType, data map[string]any) error {
	switch evt {
	case model.EventPlanUpdated:
		plan, _ := data["plan"].(map[string]any)
		revision, _ := plan["revision"].(float64)
		planID, _ := plan["plan_id"].(string)
		if b.planID != "" && planID != b.planID {
			return errors.New("plan_id cannot change")
		}
		if int(revision) <= b.planRevision {
			return errors.New("plan revision must increase monotonically")
		}
		nextCompleted := map[string]bool{}
		steps, _ := plan["steps"].([]any)
		for _, raw := range steps {
			step, _ := raw.(map[string]any)
			id, _ := step["id"].(string)
			completed := step["status"] == "completed"
			nextCompleted[id] = completed
		}
		for id := range b.planCompleted {
			if !nextCompleted[id] {
				return errors.New("completed plan steps cannot regress or disappear")
			}
		}
	case model.EventToolStarted:
		callID, _ := data["call_id"].(string)
		if _, exists := b.toolCalls[callID]; exists {
			return errors.New("tool call_id must be unique")
		}
	case model.EventToolCompleted:
		callID, _ := data["call_id"].(string)
		completed, exists := b.toolCalls[callID]
		if !exists || completed {
			return errors.New("tool.completed must match tool.started")
		}
		if b.toolNames[callID] != data["tool"] {
			return errors.New("tool.completed tool must match tool.started")
		}
	case model.EventQuestionAsked:
		for _, answered := range b.questions {
			if !answered {
				return errors.New("only one question may be pending")
			}
		}
	case model.EventQuestionAnswered:
		questionID, _ := data["question_id"].(string)
		answered, exists := b.questions[questionID]
		if !exists || answered {
			return errors.New("question.answered must match a pending question")
		}
	case model.EventMessageFinal:
		if b.cancelRequested {
			return errors.New("canceled run cannot emit message.final")
		}
		if b.finalCount != 0 {
			return errors.New("message.final may only be emitted once")
		}
	case model.EventRunFinished:
		status, _ := data["status"].(string)
		if b.cancelRequested && status == "completed" {
			return errors.New("cancel-requested run cannot complete")
		}
		if status == "completed" && b.finalCount != 1 {
			return errors.New("completed run requires exactly one prior message.final")
		}
		for _, completed := range b.toolCalls {
			if !completed {
				return errors.New("run.finished requires every started tool to complete")
			}
		}
	}
	return nil
}

func (b *Bus) recordSequence(evt model.EventType, data map[string]any) {
	switch evt {
	case model.EventRunStarted:
		b.started = true
	case model.EventPlanUpdated:
		plan, _ := data["plan"].(map[string]any)
		revision, _ := plan["revision"].(float64)
		b.planID, _ = plan["plan_id"].(string)
		b.planRevision = int(revision)
		steps, _ := plan["steps"].([]any)
		for _, raw := range steps {
			step, _ := raw.(map[string]any)
			if step["status"] == "completed" {
				id, _ := step["id"].(string)
				b.planCompleted[id] = true
			}
		}
	case model.EventToolStarted:
		callID, _ := data["call_id"].(string)
		b.toolCalls[callID] = false
		b.toolNames[callID], _ = data["tool"].(string)
	case model.EventToolCompleted:
		callID, _ := data["call_id"].(string)
		b.toolCalls[callID] = true
	case model.EventQuestionAsked:
		questionID, _ := data["question_id"].(string)
		b.questions[questionID] = false
	case model.EventQuestionAnswered:
		questionID, _ := data["question_id"].(string)
		b.questions[questionID] = true
	case model.EventMessageFinal:
		b.finalCount++
	case model.EventRunFinished:
		b.terminated = true
		b.terminalStatus, _ = data["status"].(string)
	}
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

// RequestCancelAuthority serializes cancel against the terminal event. When it
// returns false, the terminal event won the race and its status is authoritative.
func (b *Bus) RequestCancelAuthority() (bool, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.terminated {
		return false, b.terminalStatus
	}
	b.cancelRequested = true
	return true, ""
}

func (b *Bus) AppendSteeringHistory(ctx context.Context, message model.SteeringMessage) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.hw == nil || b.threadID == "" {
		return nil
	}
	return b.hw.Append(ctx, b.threadID, HistoryEntry{
		Seq: b.seq + 1, TS: message.AcceptedAt / int64(time.Second), RunID: message.RunID,
		Turn: "user", Type: "steering",
		Data: map[string]any{
			"client_message_id": message.ClientMessageID, "text": message.Content,
			"status": message.Status, "rejection_code": message.RejectionCode,
		},
	})
}
