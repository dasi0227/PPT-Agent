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
	runID string
	store Store

	mu          sync.Mutex
	seq         int64
	subscribers map[int]chan model.Event
	nextSubID   int
	closed      bool
	terminated  bool // 已发过终态事件，防止重复终态
}

func NewBus(runID string, store Store) *Bus {
	return &Bus{runID: runID, store: store, subscribers: map[int]chan model.Event{}}
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
